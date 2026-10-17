package router

import (
	"database/sql"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	middlewarepkg "tenant-provisioning-control-plane/internal/middleware"
	errpkg    "tenant-provisioning-control-plane/internal/error"
	handlerpkg "tenant-provisioning-control-plane/internal/handler"
	svcpkg     "tenant-provisioning-control-plane/internal/service"
	stores     "tenant-provisioning-control-plane/internal/store"
	lifecycle  "tenant-provisioning-control-plane/internal/lifecycle"
)

var startTime = time.Now()

// New creates a Gin router, wiring up middleware → services → handlers. GAP 5: tasks are read-only only.
func New(db *sql.DB, logger io.Writer) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// T17 Middleware chain: PanicRecovery → RequestID → Logger
	r.Use(middlewarepkg.PanicRecovery())
	r.Use(middlewarepkg.RequestID())
	if logger != nil {
		r.Use(middlewarepkg.Logger(logger))
	}

	// Wire service layer. TenantService now needs *sql.DB for outbox writes + consumer reads.
	tenantRepo := stores.NewTenantRepository(db)
	taskRepo   := stores.NewTaskRepository(db)
	tenantSvc  := svcpkg.NewTenantService(db, tenantRepo)
	taskSvc    := svcpkg.NewTaskService(tenantRepo, taskRepo)

	// Health endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Status endpoint (public-facing summary)
	r.GET("/v1/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":  "tenant-provisioning-control-plane",
			"status":   "ok",
			"uptime":   time.Since(startTime).Round(time.Second).String(),
			"nats_url": os.Getenv("NATS_URL"),
		})
	})

	// Register API groups under /v1. GAP 5: tasks group uses Register (GET-only), not RegisterAll.
	v1 := r.Group("/v1")
	{
		handlerpkg.NewTenantHandler(tenantSvc).Register(v1) // tenant CRUD T14 + GAP 4 (full DELETE).
		handlerpkg.NewTaskHandler(taskSvc).Register(v1)     // read-only tasks per spec.
	}

	return r
}

// Ensure lifecycle and error packages are accessible (side-effect deps).
var _ = (*lifecycle.ErrInvalidTransition)(nil)
var _ = (*errpkg.Error)(nil)
