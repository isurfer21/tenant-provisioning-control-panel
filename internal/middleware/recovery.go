package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// PanicRecovery recovers from panics and returns a standard JSON error with HTTP 500.
func PanicRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error_code":    500,
					"error_message": "internal server error (panic recovered)",
					"request_id":    GetReqID(c),
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
