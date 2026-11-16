package main

import (
	"time"

	"github.com/gin-gonic/gin"
)

func logRequests(c *gin.Context) {
	started := time.Now()
	defer func() {
		if c.Request.URL.Path != "/api/health" {
			requestLog(c.Request.Context()).Info("HTTP", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration", time.Since(started).Round(time.Millisecond))
		}
	}()
	c.Next()
}
