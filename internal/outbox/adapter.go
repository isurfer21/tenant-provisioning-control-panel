package outbox

import (
	"database/sql"

	natsp "tenant-provisioning-control-plane/internal/nats"
)

// fetcherAdapter wraps DB access to implement natspublisher.Fetcher. GAP 1 + 6: wires RetryPolicy into the dispatch loop.
type fetcherAdapter struct{ db *sql.DB }

// NewFetcher creates a Fetcher that queries the outbox_events table in the given DB.
func NewFetcher(db *sql.DB) natsp.Fetcher { return &fetcherAdapter{db: db} }

func (a *fetcherAdapter) FetchPending(limit int) ([]natsp.OutboxEvent, error) {
	rows, err := a.db.Query(
		`SELECT id, subject, payload, attempts FROM outbox_events WHERE attempts < 10 ORDER BY rowid LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []natsp.OutboxEvent
	for rows.Next() {
		e := natsp.OutboxEvent{}
		var payloadStr string
		if err := rows.Scan(&e.ID, &e.Subject, &payloadStr, &e.Attempts); err != nil {
			return nil, err
		}
		e.Payload = []byte(payloadStr)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (a *fetcherAdapter) MarkSuccessful(id string) error {
	_, err := a.db.Exec(`DELETE FROM outbox_events WHERE id = ? AND attempts < 10`, id)
	return err
}

func (a *fetcherAdapter) MarkFailed(id string) (bool, error) {
	res, err := a.db.Exec(
		`UPDATE outbox_events SET attempts = attempts + 2 WHERE id = ? AND attempts < 10`, id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}
