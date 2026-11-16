package main

import "github.com/gin-gonic/gin"

func (s *Server) getCartHandler(c *gin.Context) {
	session := currentSession(c)
	session.Steps = []string{}
	var err error
	if session.Cart != nil {
		var cart Cart
		err = s.commerce.call(c.Request.Context(), "get_cart", object{"id": session.Cart["id"]}, &cart)
		if err == nil {
			session.Cart = cart
		}
	}
	respond(c, session, "Your current cart.", err)
}

func (s *Server) addCartItemHandler(c *gin.Context) {
	var input AddInput
	if !bindJSON(c, &input) {
		return
	}
	session := currentSession(c)
	session.Steps = []string{"add_to_cart"}
	_, err := s.commerce.add(c.Request.Context(), session, input)
	respond(c, session, "Added to your cart. Checkout when you’re ready.", err)
}
