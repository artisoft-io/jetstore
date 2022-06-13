package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
)

var apiSecret          = flag.String("API_SECRET", "", "Secret used for signing jwt tokens (required)")
var dropTable          = flag.Bool  ("d", false, "drop users table if it exists, default is false")
var dsn                = flag.String("dsn", "", "primary database connection string (required)")
var serverAddr         = flag.String("serverAddr", ":8080", "server address to ListenAndServe (required)")

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
func options(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Options Called:", r.URL, "method:",r.Method)
	for k,v := range r.Header {
		fmt.Println("    header:",k,"values:",v)
	}
	// write cors headers
	//* check that origin is what we expect
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	//
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
	server.Router.HandleFunc("/login", jsonh(corsh(server.Login))).Methods("POST")
	server.Router.HandleFunc("/login", options).Methods("OPTIONS")

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

	fmt.Println("Listening to address ",*serverAddr)
	return http.ListenAndServe(*serverAddr, server.Router)
}

func main() {
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *apiSecret == "" {
		hasErr = true
		errMsg = append(errMsg, "API_SECRET must be provided.")
	}
	if *dsn == "" {
		hasErr = true
		errMsg = append(errMsg, "dsn for primary database node (-dsn) must be provided.")
	}
	if *serverAddr == "" {
		hasErr = true
		errMsg = append(errMsg, "Server address (-serverAddr) must be provided.")
	}
	if hasErr {
		flag.Usage()
		for _, msg := range errMsg {
			fmt.Println("**",msg)
		}
		os.Exit((1))
	}

	fmt.Println("apiserver argument:")
	fmt.Println("-------------------")
	fmt.Println("Got argument: apiSecret",*apiSecret)
	fmt.Println("Got argument: dropTable",*dropTable)
	fmt.Println("Got argument: dsn",*dsn)
	fmt.Println("Got argument: serverAddr",*serverAddr)

	log.Fatal(listenAndServe())
}
