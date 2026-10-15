package service

import (
	"context"
	"fmt"

	lifecycle "tenant-provisioning-control-plane/internal/lifecycle"
	models "tenant-provisioning-control-plane/internal/model"
	stores "tenant-provisioning-control-plane/internal/store"
)
type TenantService struct {
	repo *stores.TenantRepository
}

func NewTenantService(repo *stores.TenantRepository) *TenantService {
	return &TenantService{repo: repo}
}

// CreateTenant validates slug, creates in 'provisioning', returns the new tenant.
func (s *TenantService) CreateTenant(ctx context.Context, t *models.Tenant) (*models.Tenant, error) {
	if err := t.Validate(); err != nil {
		return nil, &stores.ErrValidation{Message: err.Error()}
	}
	if t.Status != "provisioning" {
		t.Status = "provisioning"
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil  // FIX: repo populates ID/timestamps in-place; return them.
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

	// Compute target values: keep current if not explicitly changed.
	updateName := name
	if updateName == "" {
		updateName = tenant.Name
	}
	updateStatus := tenant.Status  // default to existing
	if newStatus != "" && newStatus != tenant.Status {
		if err := lifecycle.ValidateTenantTransition(tenant.Status, newStatus); err != nil {
			return nil, err
		}
		updateStatus = newStatus
	}

	// Only call repo if something actually changed.
	if updateName == tenant.Name && updateStatus == tenant.Status {
		return tenant, nil  // no-op update already persisted
	}

	err = s.repo.Update(ctx, id, updateName, updateStatus, oldVersion)
	if err != nil {
		return nil, err
	}

	tenant.Name = updateName
	tenant.Status = updateStatus
	return tenant, nil
}
