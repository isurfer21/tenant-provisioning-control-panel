package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	errpkg "tenant-provisioning-control-plane/internal/error"
	models "tenant-provisioning-control-plane/internal/model"
	pkgmiddleware "tenant-provisioning-control-plane/internal/middleware"
	svcpkg "tenant-provisioning-control-plane/internal/service"
)

// TenantHandler handles HTTP requests for tenant lifecycle endpoints.
type TenantHandler struct {
	service *svcpkg.TenantService
}

func NewTenantHandler(svc *svcpkg.TenantService) *TenantHandler {
	return &TenantHandler{service: svc}
}

func (h *TenantHandler) Register(r *gin.RouterGroup) {
	t := r.Group("/tenants")
	{
		t.POST("", h.Create)      // POST  /v1/tenants — create new tenant (body: slug, name); invokes outbox. T18
		t.GET("", h.List)         // GET   /v1/tenants?status=...&page=&limit=
		t.GET("/:id", h.GetByID) // GET   /v1/tenants/:id
		t.PATCH("/:id", h.Update)// PATCH /v1/tenants/:id (body: name?, status?, version)
		t.DELETE("/:id", h.Delete)// DELETE /v1/tenants/:id — full lifecycle destroy. T14 GAP 4.
	}
}

type createTenantRequest struct {
	Slug string `json:"slug" binding:"required"`
	Name string `json:"name" binding:"required"`
}

func (h *TenantHandler) Create(c *gin.Context) {
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqID := pkgmiddleware.GetReqID(c)
		errpkg.SendError(c, http.StatusBadRequest, "invalid request body", reqID)
		return
	}
	t := &models.Tenant{Slug: req.Slug, Name: req.Name}
	result, err := h.service.CreateTenantWithOutbox(c.Request.Context(), t)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *TenantHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	t, err := h.service.GetTenantByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": t})
}

type listQuery struct {
	Status string `form:"status"`
	Page   int    `form:"page" binding:"omitempty,min=1"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

func (h *TenantHandler) List(c *gin.Context) {
	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query params"})
		return
	}
	result, err := h.service.ListTenants(c.Request.Context(), q.Status, q.Page, q.Limit)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

type updateTenantRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Version int64 `json:"version"` // optional — 0 means "skip optimistic lock check"
}

func (h *TenantHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqID := pkgmiddleware.GetReqID(c)
		errpkg.SendError(c, http.StatusBadRequest, "invalid request body", reqID)
		return
	}
	result, err := h.service.UpdateTenant(c.Request.Context(), id, req.Name, req.Status, req.Version)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Delete implements full tenant-destroy lifecycle (GAP 4): validates status, creates destroy task + outbox event, transitions to 'destroying'.
func (h *TenantHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	tenant, err := h.service.GetTenantByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	result, err := h.service.StartDestroy(c.Request.Context(), tenant.ID)
	if err != nil {
		errpkg.SendError(c, http.StatusUnprocessableEntity, "cannot destroy: "+err.Error(), pkgmiddleware.GetReqID(c))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func handleError(c *gin.Context, err error) {
	reqID := pkgmiddleware.GetReqID(c)
	if appErr := errpkg.MapError(err, reqID); appErr != nil {
		errpkg.SendError(c, appErr.Code, appErr.Message, reqID)
		return
	}
	errpkg.SendError(c, http.StatusInternalServerError, "internal error", reqID)
}
