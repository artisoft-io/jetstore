package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/artisoft-io/jetstore/jets/schema"
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
		user_id, err := TokenValid(r)
		if err != nil {
			// //*
			// log.Println("*** authh for",r.URL.Path,", Unauthorized")
			ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
			return
		}
		// //*
		// log.Println("* authh for",r.URL.Path,", Authorized for user ID", user_id)
		// Get a refresh token
		token, err := CreateToken(user_id)
		if err != nil {
			ERROR(w, http.StatusInternalServerError, errors.New("TokenGenError"))
			return
		}
		r.Header["Token"] = []string{token}
		next(w, r)
	}
}
// Middleware Function for allowing selected cors client
func corsh(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// //*
		// log.Println("* cors for",r.URL.Path,", Origin Header:", r.Header.Get("Origin"))
		//* check that origin is what we expect
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
	//*
	log.Println("* Options for", r.URL, "method:",r.Method)

	// write cors headers
	//* TODO check that origin is what we expect
	//*
	for key, value := range r.Header {
		log.Println("OptionConfig: ",key,value)
	}
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	if len(optionConfig.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", optionConfig.AllowedMethods)
	}
	if len(optionConfig.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", optionConfig.AllowedHeaders)
	}
	w.WriteHeader(http.StatusOK)
	// //*
	// for key, value := range w.Header() {
	// 	log.Println("Output Header: ",key,value)
	// }
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

	// Check that the users table exists
	usersTableExists, err := schema.DoesTableExists(server.dbpool, "jetsapi", "users")
	if err != nil {
		return fmt.Errorf("while verifying that the users table exists: %v", err)
	}
	if !usersTableExists {
		return fmt.Errorf("error: user table does not exist, please update database schema")
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

	// Register route
	registerOptions := OptionConfig{	Origin: "", 
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type"	}
		server.Router.HandleFunc("/register", registerOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/register", jsonh(corsh(server.CreateUser))).Methods("POST")

	// DataTable route
	dataTableOptions := OptionConfig{	Origin: "", 
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type, Authorization"	}
	server.Router.HandleFunc("/dataTable", dataTableOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/dataTable", jsonh(corsh(authh(server.DataTableAction)))).Methods("POST")

	//* TODO add options and corrs check - Users routes
	// server.Router.HandleFunc("/register", jsonh(server.CreateUser)).Methods("POST")
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
