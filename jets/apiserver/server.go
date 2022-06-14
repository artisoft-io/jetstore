package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Server struct {
	dbpool *pgxpool.Pool
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
// Middleware Function for allowing selected cors client
func corsh(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("*** Origin Header:", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		next(w, r)
	}
}

// Options ------------------------------------------------------------
type OptionConfig struct {
	Origin string
	AllowedMethods string
	AllowedHeaders string
}
func (optionConfig OptionConfig)options(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Options Called:", r.URL, "method:",r.Method)
	for k,v := range r.Header {
		fmt.Println("    header:",k,"values:",v)
	}
	// write cors headers
	// write cors headers
	//* check that origin is what we expect
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	if len(optionConfig.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", optionConfig.AllowedMethods)
	}
	if len(optionConfig.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", optionConfig.AllowedHeaders)
	}
	w.WriteHeader(http.StatusOK)
}

// processFile
// --------------------------------------------------------------------------------------
func listenAndServe() error {
	// Open db connection
	var err error
	server.dbpool, err = pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer server.dbpool.Close()	

	// prepare users table
	err = updateUserSchema(server.dbpool, *dropTable)
	if err != nil {
		return fmt.Errorf("while updating users table: %v", err)
	}

	// setup the http routes
	server.Router = mux.NewRouter()
	// server.Initialize(os.Getenv("DB_DRIVER"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_PORT"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))

	// Set Routes
	// Home Route
	server.Router.HandleFunc("/", jsonh(server.Home)).Methods("GET")

	// Login Route
	loginOptions := OptionConfig{	Origin: "", 
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type"	}
	server.Router.HandleFunc("/login", loginOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/login", jsonh(corsh(server.Login))).Methods("POST")

	//Register route
	registerOptions := OptionConfig{	Origin: "", 
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type"	}
	server.Router.HandleFunc("/register", registerOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/register", jsonh(server.CreateUser)).Methods("POST")

	//Users routes
	server.Router.HandleFunc("/register", jsonh(server.CreateUser)).Methods("POST")
	server.Router.HandleFunc("/users", jsonh(authh(server.GetUsers))).Methods("GET")
	server.Router.HandleFunc("/users/info", jsonh(authh(server.GetUserDetails))).Methods("GET")
	server.Router.HandleFunc("/users/{id}", jsonh(authh(server.GetUser))).Methods("GET")
	server.Router.HandleFunc("/users/{id}", jsonh(authh(server.UpdateUser))).Methods("PUT")
	server.Router.HandleFunc("/users/{id}", authh(server.DeleteUser)).Methods("DELETE")

	// //Posts routes
	// server.Router.HandleFunc("/posts", jsonh(server.CreatePost)).Methods("POST")
	// server.Router.HandleFunc("/posts", jsonh(server.GetPosts)).Methods("GET")
	// server.Router.HandleFunc("/posts/{id}", jsonh(server.GetPost)).Methods("GET")
	// server.Router.HandleFunc("/posts/{id}", jsonh(authh(server.UpdatePost))).Methods("PUT")
	// server.Router.HandleFunc("/posts/{id}", authh(server.DeletePost)).Methods("DELETE")

	log.Println("Listening to address ",*serverAddr)
	return http.ListenAndServe(*serverAddr, server.Router)
}
