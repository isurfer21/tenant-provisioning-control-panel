package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

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

// ApplySchema runs CREATE TABLE statements against the given DB.
func ApplySchema(db *sql.DB) error {
	if _, err := db.Exec(Schema); err != nil {
		return fmt.Errorf("apply outbox schema: %w", err)
	}
	return nil
}

// OutboxEvent is a row in the outbox_events table.
type OutboxEvent struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	Payload   []byte    `json:"payload"`
	Attempts  int64     `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}

//// T18: CreateTenantTaskWithOutbox creates tenant + task + outbox event in one DB tx.
func CreateTenantTaskWithOutbox(
	ctx context.Context, db *sql.DB, tenant *models.Tenant, taskType string,
) (*models.Tenant, *OutboxEvent, error) {
	if err := tenant.Validate(); err != nil {
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

	// 1. Insert tenant as 'provisioning'.
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO tenants (id, slug, name, status, version, created_at, updated_at)
		 VALUES (?, ?, ?, 'provisioning', 1, ?, ?)`,
		tID, tenant.Slug, tenant.Name, now, now); err != nil {
		return nil, nil, fmt.Errorf("insert tenant: %w", err)
	}

	// 2. Insert task as 'accepted'.
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO tasks (id, tenant_id, type, status, created_at, updated_at)
		 VALUES (?, ?, ?, 'accepted', ?, ?)`,
		taskID, tID, taskType, now, now); err != nil {
		return nil, nil, fmt.Errorf("insert task: %w", err)
	}

	// 3. Insert outbox event (not yet published).
	eventSubject := "tasks.create." + tID
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO outbox_events (id, subject, payload, attempts, created_at)
		 VALUES (?, ?, ?, 0, ?)`,
		eventID, eventSubject, string(payloadBytes), now); err != nil {
		return nil, nil, fmt.Errorf("insert outbox: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}

	tenant.ID = tID
	tenant.Status = "provisioning"
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	tenant.Version = 1

	outEv := &OutboxEvent{
		ID:      eventID,
		Subject: eventSubject,
		Payload: payloadBytes,
	}

	return tenant, outEv, nil
}

//// T22: Idempotent Message Processing - IsProcessed / Mark
func IsProcessed(db *sql.DB, eventID string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM processed_events WHERE event_id = ?", eventID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check processed: %w", err)
	}
	return count > 0, nil
}

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
		`SELECT id, subject, payload, attempts, created_at
		 FROM outbox_events WHERE attempts < 10 ORDER BY rowid LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var ev OutboxEvent
		var ct string
		if err := rows.Scan(&ev.ID, &ev.Subject, &ev.Payload, &ev.Attempts, &ct); err != nil {
			return nil, err
		}
		ev.CreatedAt, _ = time.Parse(time.RFC3339, ct)
		events = append(events, ev)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("outbox rows: %w", err)
	}
	return events, nil
}

// IncrementAttempts bumps the publish counter for an outbox event.
func IncrementAttempts(db *sql.DB, id string) (bool, error) {
	res, err := db.Exec(
		"UPDATE outbox_events SET attempts = attempts + 1 WHERE id = ? AND attempts < 10", id)
	if err != nil {
		return false, fmt.Errorf("increment attempts: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	return rowsAffected > 0, nil
}
