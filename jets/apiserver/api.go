package main

import (
	"net/http"
)

// Home ------------------------------------------------------------
func (server *Server) Home(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, "Welcome To This Awesome API")
}
