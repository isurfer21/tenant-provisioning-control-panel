package model

import "fmt"

const (
	TaskTypeDeploy  = "deploy"
	TaskTypeUpdate  = "update"
	TaskTypeDestroy = "destroy"
)

func IsValidTaskType(s string) bool {
	return s == TaskTypeDeploy || s == TaskTypeUpdate || s == TaskTypeDestroy
}

const (
	TaskAccepted   = "accepted"
	TaskInProgress = "in_progress"
	Completed      = "done"
	Failed         = "failed"
)

func IsValidStatus(s string) bool {
	return s == TaskAccepted || s == TaskInProgress || s == Completed || s == Failed
}

func IsTerminal(s string) bool {
	return s == Completed || s == Failed
}

func isOpenStatus(s string) bool {
	return s == TaskAccepted || s == TaskInProgress
}

type Task struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (t *Task) Validate() error {
	if !IsValidTaskType(t.Type) {
		return fmt.Errorf("invalid task type: %q", t.Type)
	}
	if !IsValidStatus(t.Status) {
		return fmt.Errorf("invalid task status: %q", t.Status)
	}
	return nil
}
