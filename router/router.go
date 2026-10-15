package router

import (
	"database/sql"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	middlewarepkg "tenant-provisioning-control-plane/internal/middleware"
	errpkg    "tenant-provisioning-control-plane/internal/error"
	handlerpkg "tenant-provisioning-control-plane/internal/handler"
	svcpkg     "tenant-provisioning-control-plane/internal/service"
	stores    "tenant-provisioning-control-plane/internal/store"
	lifecycle "tenant-provisioning-control-plane/internal/lifecycle"
)

// New creates a Gin router, wiring up middleware → services → handlers.
func New(db *sql.DB, logger io.Writer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// T17 Middleware chain: PanicRecovery → RequestID → Logger
	r.Use(middlewarepkg.PanicRecovery())
	r.Use(middlewarepkg.RequestID())
	if logger != nil {
		r.Use(middlewarepkg.Logger(logger))
	}

	// Wire service layer (repos are the leaf dependencies)
	tenantRepo := stores.NewTenantRepository(db)
	taskRepo := stores.NewTaskRepository(db)
	tenantSvc  := svcpkg.NewTenantService(tenantRepo)
	taskSvc    := svcpkg.NewTaskService(tenantRepo, taskRepo)

	// Health endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Register API groups under /v1
	v1 := r.Group("/v1")
	{
		handlerpkg.NewTenantHandler(tenantSvc).Register(v1) // tenant CRUD T14
		handlerpkg.NewTaskHandler(taskSvc).Register(v1)     // task queries  T15
	}

	return r
}

// Ensure lifecycle and error packages are accessible (side-effect deps).
var _ = (*lifecycle.ErrInvalidTransition)(nil)
var _ = (*errpkg.Error)(nil)
