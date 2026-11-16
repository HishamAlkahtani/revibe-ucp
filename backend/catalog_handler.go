package main

import "github.com/gin-gonic/gin"

func (s *Server) searchHandler(c *gin.Context) {
	var input SearchInput
	if !bindJSON(c, &input) {
		return
	}
	session := currentSession(c)
	session.Steps = []string{"search_products"}
	_, err := s.commerce.search(c.Request.Context(), session, input)
	respond(c, session, "Choose an available variant below.", err)
}
