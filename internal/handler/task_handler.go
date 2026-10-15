package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	errpkg "tenant-provisioning-control-plane/internal/error"
	models "tenant-provisioning-control-plane/internal/model"
	pkgmiddleware "tenant-provisioning-control-plane/internal/middleware"
	svc "tenant-provisioning-control-plane/internal/service"
)

// TaskHandler handles HTTP requests for task lifecycle endpoints.
type TaskHandler struct {
	service *svc.TaskService
}

func NewTaskHandler(service *svc.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

// Register registers task routes at /v1/tasks prefix.
func (h *TaskHandler) Register(r *gin.RouterGroup) {
	t := r.Group("/tasks")
	{
		t.POST("", h.Create)      // POST  /v1/tasks — create new task
		t.GET("/:id", h.GetByID) // GET   /v1/tasks/:id
	}
}

type createTaskRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Type     string `json:"type" binding:"required"`
}

func (h *TaskHandler) Create(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqID := pkgmiddleware.GetReqID(c)
		errpkg.SendError(c, http.StatusBadRequest, "invalid request body", reqID)
		return
	}
	t := &models.Task{TenantID: req.TenantID, Type: req.Type, Status: models.TaskAccepted}
	result, err := h.service.CreateTask(c.Request.Context(), t)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *TaskHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	t, err := h.service.GetTaskByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}
