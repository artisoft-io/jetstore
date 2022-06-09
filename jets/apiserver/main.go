package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type Server struct {
	Router *mux.Router
}
var server = Server{}

// Middleware Function for json header
func jsonh(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}
// Middleware Function for validating jwt token
func authh(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := TokenValid(r)
		if err != nil {
			ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
			return
		}
		next(w, r)
	}
}

func main() {

	server.Router = mux.NewRouter()
	// server.Initialize(os.Getenv("DB_DRIVER"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_PORT"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))

	// Set Routes
	// Home Route
	server.Router.HandleFunc("/", jsonh(server.Home)).Methods("GET")

	// Login Route
	server.Router.HandleFunc("/login", jsonh(server.Login)).Methods("POST")

	//Users routes
	server.Router.HandleFunc("/users", jsonh(server.CreateUser)).Methods("POST")
	server.Router.HandleFunc("/users", jsonh(server.GetUsers)).Methods("GET")
	server.Router.HandleFunc("/users/{id}", jsonh(server.GetUser)).Methods("GET")
	server.Router.HandleFunc("/users/{id}", jsonh(authh(server.UpdateUser))).Methods("PUT")
	server.Router.HandleFunc("/users/{id}", authh(server.DeleteUser)).Methods("DELETE")

	// //Posts routes
	// server.Router.HandleFunc("/posts", jsonh(server.CreatePost)).Methods("POST")
	// server.Router.HandleFunc("/posts", jsonh(server.GetPosts)).Methods("GET")
	// server.Router.HandleFunc("/posts/{id}", jsonh(server.GetPost)).Methods("GET")
	// server.Router.HandleFunc("/posts/{id}", jsonh(authh(server.UpdatePost))).Methods("PUT")
	// server.Router.HandleFunc("/posts/{id}", authh(server.DeletePost)).Methods("DELETE")

	fmt.Println("Listening to port 8080")
	log.Fatal(http.ListenAndServe(":8080", server.Router))
}
