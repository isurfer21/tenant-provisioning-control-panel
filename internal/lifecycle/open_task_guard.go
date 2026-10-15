package lifecycle

import (
	"context"
	"fmt"

	stores "tenant-provisioning-control-plane/internal/store"
)

// ErrOpenTaskGuard is returned when a new task creation violates the max-1 open guard.
type ErrOpenTaskGuard struct {
	TenantID  string
	OpenCount int
}

func (e *ErrOpenTaskGuard) Error() string {
	return fmt.Sprintf("open task guard violated for tenant %s: %d active tasks", e.TenantID, e.OpenCount)
}

// BeforeCreateOpenTask checks the open-task guard (max 1 accepted/in_progress per tenant).
func BeforeCreateOpenTask(ctx context.Context, repo *stores.TaskRepository, tenantID string) error {
	count, err := repo.CountOpenTasks(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("open task guard: %w", err)
	}
	if count > 0 {
		return &ErrOpenTaskGuard{TenantID: tenantID, OpenCount: count}
	}
	return nil
}
