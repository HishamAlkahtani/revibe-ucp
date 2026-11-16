package main

import "github.com/gin-gonic/gin"

func (s *Server) checkoutHandler(c *gin.Context) {
	session := currentSession(c)
	session.Steps = []string{"prepare_checkout"}
	session.CheckoutURL = ""
	_, err := s.commerce.checkout(c.Request.Context(), session)
	respond(c, session, "Review and pay on Revibe. Your order has not been placed.", err)
}
