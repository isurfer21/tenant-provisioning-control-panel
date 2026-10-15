package service

import (
	"context"

	lifecycle "tenant-provisioning-control-plane/internal/lifecycle"
	models "tenant-provisioning-control-plane/internal/model"
	stores "tenant-provisioning-control-plane/internal/store"
)

// TaskService coordinates task operations across store repos and FSM.
type TaskService struct {
	tenantRepo *stores.TenantRepository
	taskRepo   *stores.TaskRepository
}

func NewTaskService(tenantRepo *stores.TenantRepository, taskRepo *stores.TaskRepository) *TaskService {
	return &TaskService{tenantRepo: tenantRepo, taskRepo: taskRepo}
}

// CreateTask validates task type, enforces open-task guard, and inserts.
func (s *TaskService) CreateTask(ctx context.Context, t *models.Task) (*models.Task, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if err := lifecycle.BeforeCreateOpenTask(ctx, s.taskRepo, t.TenantID); err != nil {
		return nil, err
	}
	if err := s.taskRepo.Insert(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTaskByID returns task or ErrNotFound.
func (s *TaskService) GetTaskByID(ctx context.Context, id string) (*models.Task, error) {
	return s.taskRepo.GetByID(ctx, id)
}

// ListTasksByTenant returns paginated tasks for a tenant.
func (s *TaskService) ListTasksByTenant(ctx context.Context, tenantID string, statusFilter string, page int, perPage int) ([]models.Task, error) {
	_, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return s.taskRepo.ListByTenantID(ctx, tenantID, statusFilter, page, perPage)
}

// UpdateTaskStatus updates task status with FSM validation.
func (s *TaskService) UpdateTaskStatus(ctx context.Context, id string, newStatus string) error {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := lifecycle.ValidateTaskTransition(task.Status, newStatus); err != nil {
		return err
	}
	return s.taskRepo.UpdateStatus(ctx, id, newStatus)
}
