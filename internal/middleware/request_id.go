package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ctxKey string

const reqIDKey ctxKey = "request_id"

// RequestID generates a UUID for each request and stores it in context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.New().String()
		c.Set(reqIDKey, id)
		c.Header("X-Request-ID", id)
		ctx := context.WithValue(c.Request.Context(), reqIDKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetReqID retrieves the request ID from context or env.
func GetReqID(c *gin.Context) string {
	if id, ok := c.Get(reqIDKey); ok {
		return id.(string)
	}
	return ""
}
