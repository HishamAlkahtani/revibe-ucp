package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Session struct {
	mu          sync.Mutex
	History     []ChatMessage
	Products    []Product
	Cart        *Cart // UCP cart backing the UI basket.
	CheckoutURL string
	Steps       []string
	touchedAt   time.Time
}

type SessionStore struct {
	mu    sync.Mutex
	items map[string]*Session
}

func newSessionStore() *SessionStore { return &SessionStore{items: map[string]*Session{}} }

func (store *SessionStore) get(c *gin.Context) *Session {
	store.mu.Lock()
	defer store.mu.Unlock()
	for id, session := range store.items {
		if time.Since(session.touchedAt) > time.Hour {
			delete(store.items, id)
		}
	}
	id, _ := c.Cookie("session")
	if session := store.items[id]; session != nil {
		session.touchedAt = time.Now()
		return session
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	id = hex.EncodeToString(bytes[:])
	session := &Session{Products: []Product{}, Steps: []string{}, touchedAt: time.Now()}
	store.items[id] = session
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("session", id, 0, "/", "", false, true)
	return session
}

func (s *Server) sessionMiddleware(c *gin.Context) {
	session := s.sessions.get(c)
	if !session.mu.TryLock() {
		c.AbortWithStatusJSON(409, gin.H{"error": "wait for the current request"})
		return
	}
	defer session.mu.Unlock()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Minute)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	c.Set("session", session)
	c.Next()
}

func currentSession(c *gin.Context) *Session { return c.MustGet("session").(*Session) }
