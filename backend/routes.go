package main

import "github.com/gin-gonic/gin"

type Server struct {
	agent    *Agent
	sessions *SessionStore
}

func (s *Server) routes() *gin.Engine {
	router := gin.New()
	router.Use(logRequests, gin.Recovery(), sameOrigin)
	router.GET("/api/health", s.healthHandler)
	api := router.Group("/api", s.sessionMiddleware)
	api.POST("/chat", s.chatHandler)
	return router
}

func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
