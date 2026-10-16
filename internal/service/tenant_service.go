package service

import (
	"context"
	"database/sql"
	"fmt"

	lifecycle "tenant-provisioning-control-plane/internal/lifecycle"
	models "tenant-provisioning-control-plane/internal/model"
	outboxpkg "tenant-provisioning-control-plane/internal/outbox"
	stores "tenant-provisioning-control-plane/internal/store"
)

// TenantService holds tenant business logic.
type TenantService struct {
	repo *stores.TenantRepository
	db   *sql.DB // shared DB for outbox writes and consumer reads.
}

func NewTenantService(db *sql.DB, repo *stores.TenantRepository) *TenantService {
	return &TenantService{db: db, repo: repo}
}

// CreateTenantWithOutbox creates tenant + task + outbox event in one DB tx (T18).
func (s *TenantService) CreateTenantWithOutbox(ctx context.Context, t *models.Tenant) (*models.Tenant, error) {
	tenant, _, err := outboxpkg.CreateTenantTaskWithOutbox(ctx, s.db, t, models.TaskTypeDeploy)
	if err != nil {
		return nil, fmt.Errorf("create tenant with outbox: %w", err)
	}
	return tenant, nil
}

// CreateTenant validates slug, inserts directly (no outbox). For tests / legacy code.
func (s *TenantService) CreateTenant(ctx context.Context, t *models.Tenant) (*models.Tenant, error) {
	if err := t.Validate(); err != nil {
		return nil, &stores.ErrValidation{Message: err.Error()}
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTenantByID returns tenant or ErrNotFound.
func (s *TenantService) GetTenantByID(ctx context.Context, id string) (*models.Tenant, error) {
	return s.repo.GetByID(ctx, id)
}

// GetTenantBySlug returns tenant or ErrNotFound.
func (s *TenantService) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// ListTenants returns paginated tenants with optional status filter.
func (s *TenantService) ListTenants(ctx context.Context, statusFilter string, page int, perPage int) ([]models.Tenant, error) {
	return s.repo.List(ctx, statusFilter, page, perPage)
}

// UpdateTenant updates name/status with optimistic concurrency + FSM guard.
func (s *TenantService) UpdateTenant(
	ctx context.Context, id string, name string, newStatus string, oldVersion int64,
) (*models.Tenant, error) {
	tenant, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant.Version != oldVersion {
		return nil, stores.ErrConflict
	}

	updateName := name
	if updateName == "" {
		updateName = tenant.Name
	}
	updateStatus := tenant.Status
	if newStatus != "" && newStatus != tenant.Status {
		if err := lifecycle.ValidateTenantTransition(tenant.Status, newStatus); err != nil {
			return nil, err
		}
		updateStatus = newStatus
	}
	if updateName == tenant.Name && updateStatus == tenant.Status {
		return tenant, nil
	}

	err = s.repo.Update(ctx, id, updateName, updateStatus, oldVersion)
	if err != nil {
		return nil, err
	}

	tenant.Name = updateName
	tenant.Status = updateStatus
	return tenant, nil
}

// StartDestroy creates a destroy task row + outbox event and transitions tenant to 'destroying'. GAP 4.
func (s *TenantService) StartDestroy(ctx context.Context, id string) (*models.Tenant, error) {
	tenant, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if tenant.Status != "active" && tenant.Status != "failed" {
		return nil, &lifecycle.ErrInvalidTransition{From: tenant.Status, To: "destroying"}
	}
	if err := lifecycle.ValidateTenantTransition(tenant.Status, "destroying"); err != nil {
		return nil, err
	}

	if _, err := outboxpkg.CreateDestroyTaskWithOutbox(ctx, s.db, tenant.ID); err != nil {
		return nil, fmt.Errorf("create destroy-outbox event: %w", err)
	}

	err = s.repo.Update(ctx, tenant.ID, tenant.Name, "destroying", tenant.Version)
	if err != nil {
		return nil, fmt.Errorf("transition tenant to destroying: %w", err)
	}

	tenant.Status = "destroying"
	tenant.Version++
	return tenant, nil
}
