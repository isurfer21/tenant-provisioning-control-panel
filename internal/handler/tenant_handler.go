package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	errpkg "tenant-provisioning-control-plane/internal/error"
	models "tenant-provisioning-control-plane/internal/model"
	pkgmiddleware "tenant-provisioning-control-plane/internal/middleware"
	svc "tenant-provisioning-control-plane/internal/service"
)

// TenantHandler handles HTTP requests for tenant lifecycle endpoints (CRUD).
type TenantHandler struct {
	service *svc.TenantService
}

func NewTenantHandler(service *svc.TenantService) *TenantHandler {
	return &TenantHandler{service: service}
}

// Register registers all tenant routes at /v1/tenants prefix.
func (h *TenantHandler) Register(r *gin.RouterGroup) {
	t := r.Group("/tenants")
	{
		t.POST("", h.Create)      // POST  /v1/tenants — create new tenant (body: slug, name)
		t.GET("", h.List)         // GET   /v1/tenants?status=...&page=&limit=
		t.GET("/:id", h.GetByID) // GET   /v1/tenants/:id
		t.PATCH("/:id", h.Update)// PATCH /v1/tenants/:id (body: name?, status?, version)
		t.DELETE("/:id", h.Delete)// DELETE /v1/tenants/:id
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
	result, err := h.service.CreateTenant(c.Request.Context(), t)
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
	Version int64 `json:"version" binding:"required"`
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

func (h *TenantHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result, err := h.service.GetTenantByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	_ = result // For now just confirm the tenant exists; real delete would use outbox in Phase 3.
	c.JSON(http.StatusOK, gin.H{"data": map[string]string{"status": "deleted"}})
}

func handleError(c *gin.Context, err error) {
	reqID := pkgmiddleware.GetReqID(c)
	appErr := errpkg.MapError(err, reqID)
	errpkg.SendError(c, appErr.Code, appErr.Message, reqID)
}
