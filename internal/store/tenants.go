package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	model "tenant-provisioning-control-plane/internal/model"
)

type TenantRepository struct {
	db *sql.DB
}

func NewTenantRepository(db *sql.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) Create(ctx context.Context, t *model.Tenant) error {
	t.ID = uuid.New().String()
	now := model.NowUTC()
	t.CreatedAt = now
	t.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenants (id, slug, name, status, version, created_at, updated_at)
		VALUES (?, ?, ?, 'provisioning', 1, ?, ?)
	`, t.ID, t.Slug, t.Name, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("create tenant: %w", err)
	}

	t.Version = 1
	t.Status = "provisioning"
	return nil
}

func (r *TenantRepository) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := r.db.QueryRowContext(ctx, `
		SELECT id, slug, name, status, version, created_at, updated_at
		FROM tenants WHERE id = ? LIMIT 1
	`, id).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Status, &t.Version, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) GetBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	var t model.Tenant
	err := r.db.QueryRowContext(ctx, `
		SELECT id, slug, name, status, version, created_at, updated_at
		FROM tenants WHERE slug = ? LIMIT 1
	`, slug).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Status, &t.Version, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) Update(ctx context.Context, id string, name string, status string, oldVersion int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE tenants SET name = ?, status = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ?`, name, status, model.NowUTC(), id, oldVersion)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrConflict
	}
	return nil
}

func (r *TenantRepository) List(ctx context.Context, statusFilter string, page int, perPage int) ([]model.Tenant, error) {
	var where string
	var args []interface{}
	if statusFilter != "" {
		where = "WHERE status = ?"
		args = append(args, statusFilter)
	}

	offset := 0
	limit := 20
	if page > 0 {
		offset = (page - 1) * limit
	}
	if perPage > 0 && perPage < 100 {
		limit = perPage
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, name, status, version, created_at, updated_at
		FROM tenants `+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Status, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}
