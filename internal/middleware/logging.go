package middleware

import (
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns a Gin middleware that logs each request to writer with structured format:
// [time] method=/path status=200 latency=1ms client_ip=127.0.0.1 req_id=uuid
func Logger(w io.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		fields := []any{
			time.Now().UTC().Format(time.RFC3339Nano),
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).Truncate(time.Millisecond),
			"client_ip", c.ClientIP(),
			"req_id", GetReqID(c),
		}
		fmt.Fprintf(w, "%s %v\n", "[log]", fields)
	}
}
