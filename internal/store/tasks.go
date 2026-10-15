package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	model "tenant-provisioning-control-plane/internal/model"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Insert(ctx context.Context, t *model.Task) error {
	t.ID = uuid.New().String()
	now := model.NowUTC()
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO tasks (id, tenant_id, type, status, created_at, updated_at) VALUES (?, ?, ?, 'accepted', ?, ?)",
		t.ID, t.TenantID, t.Type, now, now)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*model.Task, error) {
	var t model.Task
	err := r.db.QueryRowContext(ctx,
		"SELECT id, tenant_id, type, status, created_at, updated_at FROM tasks WHERE id = ? LIMIT 1",
		id).Scan(&t.ID, &t.TenantID, &t.Type, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

func (r *TaskRepository) ListByTenantID(ctx context.Context, tenantID string, statusFilter string, page int, perPage int) ([]model.Task, error) {
	conditions := []string{}
	args := []interface{}{tenantID}
	if statusFilter != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, statusFilter)
	}
	offset, limit := 0, 20
	if page > 0 {
		offset = (page - 1) * limit
	}
	if perPage > 0 && perPage < 100 {
		limit = perPage
	}
	conditions = append(conditions, "LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, tenant_id, type, status, created_at, updated_at FROM tasks WHERE tenant_id = ? "+strings.Join(conditions, " ")+" ORDER BY created_at DESC",
		args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Type, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id string, newStatus string) error {
	now := model.NowUTC()
	res, err := r.db.ExecContext(ctx,
		"UPDATE tasks SET status = ?, updated_at = ? WHERE id = ? AND status IN ('accepted', 'in_progress')",
		newStatus, now, id)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("task already terminal or not found")
	}
	return nil
}

func (r *TaskRepository) CountOpenTasks(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE tenant_id = ? AND status IN ('accepted', 'in_progress')`, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count open tasks: %w", err)
	}
	return count, nil
}
