package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) chatHandler(c *gin.Context) {
	var input struct {
		Message string `json:"message"`
	}
	if !bindJSON(c, &input) {
		return
	}
	if strings.TrimSpace(input.Message) == "" || len(input.Message) > 2000 {
		c.JSON(400, gin.H{"error": "enter a message of 1–2000 bytes"})
		return
	}
	session := currentSession(c)
	session.CheckoutURL, session.Steps = "", []string{}
	if s.agent.key == "" {
		respond(c, session, "", fmt.Errorf("add OPENAI_API_KEY to .env and restart the Go backend"))
		return
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("X-Accel-Buffering", "no")
	emit := func(event string, data any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
			cancel()
			return err
		}
		c.Writer.Flush()
		return nil
	}
	if err := emit("started", object{}); err != nil {
		return
	}
	_, err := s.agent.chat(ctx, session, input.Message, emit)
	if ctx.Err() != nil {
		requestLog(ctx).Info("chat canceled", "reason", ctx.Err().Error())
		return
	}
	// Preserve confirmed state even if a later model call fails.
	if emitErr := emit("state", object{"products": session.Products, "cart": cartView(session.Cart), "checkoutUrl": session.CheckoutURL}); emitErr != nil {
		return
	}
	if err != nil {
		requestLog(ctx).Error("chat failed", "error", err.Error())
		_ = emit("error", object{"message": err.Error()})
		return
	}
	requestLog(ctx).Info("chat completed", "tool_calls", len(session.Steps))
	_ = emit("done", object{})
}
