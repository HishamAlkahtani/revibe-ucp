package main

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

func sameOrigin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	origin := c.GetHeader("Origin")
	if origin != "" && origin != "http://127.0.0.1:3000" && origin != "http://localhost:3000" {
		c.AbortWithStatusJSON(403, gin.H{"error": "invalid origin"})
		return
	}
	c.Next()
}

func bindJSON(c *gin.Context, out any) bool {
	mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if mediaType != "application/json" {
		c.JSON(415, gin.H{"error": "JSON required"})
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(out); err != nil {
		c.JSON(400, gin.H{"error": "invalid or oversized JSON"})
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		c.JSON(400, gin.H{"error": "expected a single JSON object"})
		return false
	}
	return true
}

// Return current state even when a model fails after a successful cart action.
func respond(c *gin.Context, session *Session, reply string, err error) {
	result := gin.H{"reply": reply, "products": session.Products, "cart": cartView(session.Cart), "checkoutUrl": session.CheckoutURL, "steps": session.Steps}
	if err != nil {
		requestLog(c.Request.Context()).Error("handler failed", "path", c.Request.URL.Path, "error", err.Error())
		delete(result, "reply")
		result["error"] = err.Error()
		c.JSON(502, result)
		return
	}
	c.JSON(200, result)
}
