package error

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	lifecycle "tenant-provisioning-control-plane/internal/lifecycle"
	stores "tenant-provisioning-control-plane/internal/store"
)

// Error represents a standard JSON error contract: code/message/request_id.
type Error struct {
	Code     int    `json:"error_code"`
	Message  string `json:"error_message"`
	ReqID    string `json:"request_id"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("HTTP %d: %s (req=%s)", e.Code, e.Message, e.ReqID)
}

// MapError maps internal sentinel/errors to HTTP-aware Error for Gin context.
func MapError(err error, reqID string) *Error {
	if err == nil {
		return nil
	}

	var code int
	var message string

	switch {
	case errors.Is(err, stores.ErrNotFound):
		code = http.StatusNotFound
		message = "resource not found"
	case errors.Is(err, stores.ErrAlreadyExists):
		code = http.StatusConflict
		message = "tenant already exists with this slug"
	case errors.Is(err, stores.ErrConflict):
		code = http.StatusConflict
		message = "conflict: concurrent update detected"
	case err.Error() == "task already terminal or not found":
		code = http.StatusConflict
		message = "task is in a terminal state or not found"
	default:
		var valErr *stores.ErrValidation
		if errors.As(err, &valErr) {
			code = http.StatusBadRequest
			message = fmt.Sprintf("invalid request body: %s", valErr.Message)
		} else {
			var transErr *lifecycle.ErrInvalidTransition
			if errors.As(err, &transErr) {
				code = http.StatusUnprocessableEntity
				message = fmt.Sprintf("invalid state transition: %s -> %s", transErr.From, transErr.To)
			} else {
				var termErr *lifecycle.ErrTerminalTask
				if errors.As(err, &termErr) {
					code = http.StatusForbidden
					message = fmt.Sprintf("task already in terminal state: %s", termErr.Status)
				} else {
					var openTaskErr *lifecycle.ErrOpenTaskGuard
					if errors.As(err, &openTaskErr) {
						code = http.StatusConflict
						message = fmt.Sprintf("tenant %s has an active task — complete it before creating another", openTaskErr.TenantID)
					} else {
						code = http.StatusInternalServerError
						message = "internal server error"
					}
				}
			}
		}
	}

	return &Error{Code: code, Message: message, ReqID: reqID}
}

// SendError helper writes a JSON error response for Gin.
func SendError(c *gin.Context, code int, msg string, reqID string) {
	c.JSON(code, gin.H{"error_code": code, "error_message": msg, "request_id": reqID})
}
