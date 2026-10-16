package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	models "tenant-provisioning-control-plane/internal/model"
)

// Schema is the SQL DDL for processed_events and outbox_events tables.
const Schema = `
CREATE TABLE IF NOT EXISTS processed_events (
	event_id TEXT PRIMARY KEY,
	subject TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS outbox_events (
	id TEXT PRIMARY KEY,
	subject TEXT NOT NULL,
	payload TEXT NOT NULL,
	attempts INTEGER DEFAULT 0,
	created_at TEXT NOT NULL
);
`

// OutboxEvent is a row in the outbox_events table.
type OutboxEvent struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Payload  []byte `json:"payload"`
	Attempts int64  `json:"attempts"`
}

// CreateTenantTaskWithOutbox creates tenant + task + outbox event in one DB tx. T18 pattern (commit before publish).
func CreateTenantTaskWithOutbox(
	ctx context.Context, db *sql.DB, t *models.Tenant, taskType string,
) (*models.Tenant, *OutboxEvent, error) {
	if err := t.Validate(); err != nil {
		return nil, nil, fmt.Errorf("tenant validate: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := models.NowUTC()

	tID := uuid.New().String()
	taskID := uuid.New().String()
	eventID := uuid.New().String()

	payload := map[string]string{
		"tenant_id": tID,
		"task_id":   taskID,
		"type":      taskType,
		"event_id":  eventID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal outbox payload: %w", err)
	}

	// Insert tenant as provisioning.
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO tenants (id, slug, name, status, version, created_at, updated_at) VALUES (?, ?, ?, 'provisioning', 1, ?, ?)`,
		tID, t.Slug, t.Name, now, now); err != nil {
		return nil, nil, fmt.Errorf("insert tenant: %w", err)
	}

	// Insert task as accepted.
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO tasks (id, tenant_id, type, status, created_at, updated_at) VALUES (?, ?, ?, 'accepted', ?, ?)`,
		taskID, tID, taskType, now, now); err != nil {
		return nil, nil, fmt.Errorf("insert task: %w", err)
	}

	// Insert outbox event (not yet published).
	eventSubject := "tasks.create." + tID
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (id, subject, payload, attempts, created_at) VALUES (?, ?, ?, 0, ?)`,
		eventID, eventSubject, string(payloadBytes), now); err != nil {
		return nil, nil, fmt.Errorf("insert outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}

	t.ID = tID
	t.Status = "provisioning"
	t.CreatedAt = now
	t.UpdatedAt = now
	t.Version = 1

	outEv := OutboxEvent{ID: eventID, Subject: eventSubject, Payload: payloadBytes}
	return t, &outEv, nil
}

// CreateDestroyTaskWithOutbox writes only a destroy task row + outbox event for an existing tenant. T14 full lifecycle / GAP 4.
func CreateDestroyTaskWithOutbox(ctx context.Context, db *sql.DB, tenantID string) (*OutboxEvent, error) {
	taskID := uuid.New().String()
	eventID := uuid.New().String()
	now := models.NowUTC()

	payload := map[string]string{
		"tenant_id": tenantID,
		"task_id":   taskID,
		"type":      models.TaskTypeDestroy,
		"event_id":  eventID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal destroy payload: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx,
		`INSERT INTO tasks (id, tenant_id, type, status, created_at, updated_at) VALUES (?, ?, ?, 'accepted', ?, ?)`,
		taskID, tenantID, models.TaskTypeDestroy, now, now); err != nil {
		return nil, fmt.Errorf("insert destroy task: %w", err)
	}

	eventSubject := "tasks.create." + tenantID
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (id, subject, payload, attempts, created_at) VALUES (?, ?, ?, 0, ?)`,
		eventID, eventSubject, string(payloadBytes), now); err != nil {
		return nil, fmt.Errorf("insert destroy outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit destroy tx: %w", err)
	}

	return &OutboxEvent{ID: eventID, Subject: eventSubject, Payload: payloadBytes}, nil
}

// IsProcessed checks whether an event has already been processed. GAP 2 / T22.
func IsProcessed(db *sql.DB, eventID string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM processed_events WHERE event_id = ?", eventID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check-processed: %w", err)
	}
	return count > 0, nil
}

// MarkProcessed writes an idempotency marker. GAP 2 / T22.
func MarkProcessed(db *sql.DB, eventID string, subject string) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO processed_events (event_id, subject, created_at) VALUES (?, ?, ?)",
		eventID, subject, models.NowUTC())
	return err
}

// FetchPendingOutbox retrieves up to `limit` outbox events with <10 attempts.
func FetchPendingOutbox(db *sql.DB, limit int) ([]OutboxEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.Query(
		`SELECT id, subject, payload, attempts FROM outbox_events WHERE attempts < 10 ORDER BY rowid LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch-pending: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var ev OutboxEvent
		var payloadStr string
		// Read the row and reconstruct. The attemtps column is int(11), created_at not needed in interface.
		if err := rows.Scan(&ev.ID, &ev.Subject, &payloadStr, &ev.Attempts); err != nil {
			return nil, err
		}
		ev.Payload = []byte(payloadStr)
		events = append(events, ev)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox-rows: %w", err)
	}
	return events, nil
}

// IncrementAttempts bumps the publish counter for an outbox event. Called on transient NATS failures.
func IncrementAttempts(db *sql.DB, id string) (bool, error) {
	res, err := db.Exec(
		"UPDATE outbox_events SET attempts = attempts + 1 WHERE id = ? AND attempts < 10", id)
	if err != nil {
		return false, fmt.Errorf("increment-attempts: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0, nil
}
