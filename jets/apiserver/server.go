package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
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
		user_id, err := user.TokenValid(r)
		if err != nil {
			// //*
			// log.Println("*** authh for",r.URL.Path,", Unauthorized")
			ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
			return
		}
		// //*
		// log.Println("* authh for",r.URL.Path,", Authorized for user ID", user_id)
		// Get a refresh token
		token, err := user.CreateToken(user_id)
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
	Origin         string
	AllowedMethods string
	AllowedHeaders string
}

func (optionConfig OptionConfig) options(w http.ResponseWriter, r *http.Request) {
	// // for devel
	// log.Println("* Options for", r.URL, "method:",r.Method)

	// write cors headers
	//* TODO check that origin is what we expect
	//
	// for key, value := range r.Header {
	// 	log.Println("OptionConfig: ",key,value)
	// }
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	if len(optionConfig.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", optionConfig.AllowedMethods)
	}
	if len(optionConfig.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", optionConfig.AllowedHeaders)
	}
	w.WriteHeader(http.StatusOK)
	// // for devel
	// for key, value := range w.Header() {
	// 	log.Println("Output Header: ", key, value)
	// }
}

func (server *Server) addVersionToDb(jetstoreVersion string) (err error) {
	// Add version to db
	stmt := "INSERT INTO jetsapi.jetstore_release (version) VALUES ($1)"
	_, err = server.dbpool.Exec(context.Background(), stmt, jetstoreVersion)
	if err != nil {
		return fmt.Errorf("while inserting jetstore version into jetstore_release table: %v", err)
	}
	return nil
}

// Validate the user table exists and create admin if not already created
func (server *Server) checkJetStoreDbVersion() error {
	tableExists, err := schema.DoesTableExists(server.dbpool, "jetsapi", "jetstore_release")
	if err != nil {
		return fmt.Errorf("while verifying that the jetstore_release table exists: %v", err)
	}
	var serverArgs []string
	var version string
	jetstoreVersion := os.Getenv("JETS_VERSION")
	log.Println("JetStore version JETS_VERSION is", jetstoreVersion)
	if !tableExists {
		// run update db with workspace init script
		log.Println("JetStore version table does not exist, initializing the db")
		// Cleanup any remaining
		_, _, err = server.ResetDomainTables(&PurgeDataAction{
			Action: "reset_domain_tables",
			RunUiDbInitScript: true,
			Data: []map[string]interface{}{},
		})
		if err != nil {
			return fmt.Errorf("while calling ResetDomainTables to initialize db: %v", err)
		}
		err = server.addVersionToDb(jetstoreVersion)
		if err != nil {
			return fmt.Errorf("while calling saving jetstoreVersion to database: %v", err)
		}
	} else {

		// Check the release in database vs current release
		stmt := "SELECT MAX(version) FROM jetsapi.jetstore_release"
		
		err = server.dbpool.QueryRow(context.Background(), stmt).Scan(&version)
		switch {
		case err != nil:
			log.Println("JetStore version is not defined in jetstore_release table, rebuilding all tables and running workspace db init script")
			_, _, err = server.ResetDomainTables(&PurgeDataAction{
				Action: "reset_domain_tables",
				RunUiDbInitScript: true,
				Data: []map[string]interface{}{},
			})
			if err != nil {
				return fmt.Errorf("while calling ResetDomainTables to initialize db (no version exist in db): %v", err)
			}
			err = server.addVersionToDb(jetstoreVersion)
			if err != nil {
				return fmt.Errorf("while calling saving jetstoreVersion to database: %v", err)
			}
	
		case jetstoreVersion > version:
			if strings.Contains(os.Getenv("JETS_RESET_DOMAIN_TABLE_ON_STARTUP"), "yes") {
				log.Println("New JetStore Release deployed, rebuilding all tables")
				_, _, err = server.ResetDomainTables(&PurgeDataAction{
					Action: "reset_domain_tables",
					RunUiDbInitScript: false,
					Data: []map[string]interface{}{},
				})
				if err != nil {
					return fmt.Errorf("while calling ResetDomainTables for new release: %v", err)
				}
				err = server.addVersionToDb(jetstoreVersion)
				if err != nil {
					return fmt.Errorf("while calling saving jetstoreVersion to database: %v", err)
				}
			} else {
				log.Println("New JetStore Release deployed, migrating tables to latest schema")
				serverArgs = []string{ "-migrateDb" }
			}

		default:
			log.Println("JetStore version in database", version, ">=", "deployed version", jetstoreVersion)
		}
	}

	if len(serverArgs) > 0 {
		if *usingSshTunnel {
			serverArgs = append(serverArgs, "-usingSshTunnel")
		}
		log.Printf("Run update_db: %s", serverArgs)
		cmd := exec.Command("/usr/local/bin/update_db", serverArgs...)
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b
		err := cmd.Run()
		if err != nil {
			log.Printf("while executing update_db command '%v': %v", serverArgs, err)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			b.WriteTo(os.Stdout)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("UPDATE_DB CAPTURED OUTPUT END")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			return err
		}
		log.Println("============================")
		log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
		log.Println("============================")
		b.WriteTo(os.Stdout)
		log.Println("============================")
		log.Println("UPDATE_DB CAPTURED OUTPUT END")
		log.Println("============================")

		err = server.addVersionToDb(jetstoreVersion)
		if err != nil {
			return fmt.Errorf("while calling saving jetstoreVersion to database: %v", err)
		}
}
	return nil
}

// Download overriten workspace files from s3
// Check the workspace version in db, if jetstore version is more recent, recompile workspace
func (server *Server) checkWorkspaceVersion() error {
	// Download overriten workspace files from s3 if any
	workspaceName := os.Getenv("WORKSPACE")
	err := workspace.SyncWorkspaceFiles(workspaceName, devMode)
	if err != nil {
		log.Println("Error while synching workspace file from s3:",err)
		return err
	}
	// Check if need to recompile workspace, skip if in dev mode
	if os.Getenv("JETSTORE_DEV_MODE") != "" {
		// We're in dev mode, the user is responsible to compile workspace when needed
		return nil
	}
	var version sql.NullString
	jetstoreVersion := os.Getenv("JETS_VERSION")
	// Check the release in database vs current release
	stmt := "SELECT MAX(version) FROM jetsapi.workspace_version"
	err = server.dbpool.QueryRow(context.Background(), stmt).Scan(&version)
	switch {
	case err != nil:
		if err.Error() == "no rows in result set" {
			log.Println("Workspace version is not defined in workspace_version table, no need to recompile workspace")
			return nil	
		}
		log.Println("Error while reading workspace version from workspace_version table:",err)
		return err

	case !version.Valid:
		log.Println("Workspace version is not defined in workspace_version table, no need to recompile workspace")
		return nil	

	case jetstoreVersion > version.String:
		// recompile workspace, set the workspace version to be same as jetstore version
		// Skip this if in DEV MODE
		if !devMode {
			err = workspace.CompileWorkspace(server.dbpool, workspaceName, jetstoreVersion)
			if err != nil {
				log.Println("Error while compiling workspace:",err)
				return err
			}	
		}

	default:
		log.Println("JetStore version in database", version, ">=", "workspace version", jetstoreVersion,", no need to recompile workspace")
	}
	return nil
}

// Validate the user table exists and create admin if not already created
func (server *Server) initUsers() error {
	usersTableExists, err := schema.DoesTableExists(server.dbpool, "jetsapi", "users")
	if err != nil {
		return fmt.Errorf("while verifying that the users table exists: %v", err)
	}
	if !usersTableExists {
		return fmt.Errorf("error: user table does not exist, please update database schema")
	}
	// Check the admin user exists
	stmt := "SELECT user_email FROM jetsapi.users WHERE user_email=$1"
	var v string
	err = server.dbpool.QueryRow(context.Background(), stmt, *adminEmail).Scan(&v)
	if err != nil {
		log.Println("Admin User is not defined in users table, creating it")
		if *awsAdminPwdSecret == "" && *adminPwd == "" {
			return fmt.Errorf("admin password not defined and database not initialized, must be defined")
		}
		var adminPassword string = *adminPwd
		var err error
		if *awsAdminPwdSecret != "" {
			adminPassword, err = awsi.GetSecretValue(*awsAdminPwdSecret, *awsRegion)
			if err != nil {
				return fmt.Errorf("while getting apiSecret from aws secret: %v", err)
			}
		}
		// hash the password
		hashedPassword, err := user.Hash(adminPassword)
		if err != nil {
			return fmt.Errorf("while hashing admin password: %v", err)
		}
		adminPassword = string(hashedPassword)
		stmt = "INSERT INTO jetsapi.users (user_email, name, password, is_active) VALUES ($1, 'Admin', $2, 1)"
		_, err = server.dbpool.Exec(context.Background(), stmt, *adminEmail, adminPassword)
		if err != nil {
			return fmt.Errorf("while inserting admin into users table: %v", err)
		}
	}
	// Initialize user package
	// Set the AdminEmail for the user package
	user.AdminEmail = *adminEmail
	user.ApiSecret = *apiSecret
	user.TokenExpiration = *tokenExpiration

	return nil
}

// processFile
// --------------------------------------------------------------------------------------
func listenAndServe() error {
	var err error
	// Get secret to sign jwt tokens
	if *awsApiSecret != "" {
		*apiSecret, err = awsi.GetSecretValue(*awsApiSecret, *awsRegion)
		if err != nil {
			return fmt.Errorf("while getting apiSecret from aws secret: %v", err)
		}
	}

	// Open db connection
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *awsRegion, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	server.dbpool, err = pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer server.dbpool.Close()

	// Check jetstore version, run update_db if needed
	err = server.checkJetStoreDbVersion()
	if err != nil {
		return fmt.Errorf("while calling checkJetStoreDbVersion: %v", err)
	}

	// Check workspace version, compile workspace if needed
	err = server.checkWorkspaceVersion()
	if err != nil {
		return fmt.Errorf("while calling checkWorkspaceVersion: %v", err)
	}

	// Check that the users table and admin user exists
	err = server.initUsers()
	if err != nil {
		return fmt.Errorf("while calling initUsers: %v", err)
	}

	// setup the http routes
	server.Router = mux.NewRouter()
	// server.Initialize(os.Getenv("DB_DRIVER"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_PORT"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))

	// Set Routes
	// Home Route
	// server.Router.HandleFunc("/", jsonh(server.Home)).Methods("GET")

	// Serve the jetsclient app
	fs := http.FileServer(http.Dir(*uiWebDir))
	server.Router.Handle("/", fs).Methods("GET")
	server.Router.Handle("/favicon.ico", fs).Methods("GET")
	server.Router.Handle("/flutter.js", fs).Methods("GET")
	server.Router.Handle("/version.json", fs).Methods("GET")
	server.Router.Handle("/main.dart.js.map", fs).Methods("GET")
	server.Router.Handle("/index.html", fs).Methods("GET")
	server.Router.Handle("/favicon.png", fs).Methods("GET")
	server.Router.Handle("/icons/Icon-192.png", fs).Methods("GET")
	server.Router.Handle("/icons/Icon-maskable-512.png", fs).Methods("GET")
	server.Router.Handle("/icons/Icon-maskable-192.png", fs).Methods("GET")
	server.Router.Handle("/icons/Icon-512.png", fs).Methods("GET")
	server.Router.Handle("/assets/NOTICES", fs).Methods("GET")
	server.Router.Handle("/assets/fonts/MaterialIcons-Regular.otf", fs).Methods("GET")
	server.Router.Handle("/assets/AssetManifest.json", fs).Methods("GET")
	server.Router.Handle("/assets/AssetManifest.bin", fs).Methods("GET")
	server.Router.Handle("/assets/AssetManifest.smcbin", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/RobotoCondensed-Bold.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/RobotoCondensed-Italic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Regular.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Light.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Thin.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/RobotoCondensed-Light.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/RobotoCondensed-Regular.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-MediumItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-ThinItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Italic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/RobotoCondensed-LightItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-BlackItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/RobotoCondensed-BoldItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Medium.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-BoldItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Bold.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-Black.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/Roboto-LightItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/images/logo.png", fs).Methods("GET")
	server.Router.Handle("/assets/FontManifest.json", fs).Methods("GET")
	server.Router.Handle("/assets/packages/cupertino_icons/assets/CupertinoIcons.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/shaders/ink_sparkle.frag", fs).Methods("GET")
	server.Router.Handle("/flutter_service_worker.js", fs).Methods("GET")
	server.Router.Handle("/canvaskit/canvaskit.js", fs).Methods("GET")
	server.Router.Handle("/canvaskit/canvaskit.wasm", fs).Methods("GET")
	server.Router.Handle("/canvaskit/profiling/canvaskit.js", fs).Methods("GET")
	server.Router.Handle("/canvaskit/profiling/canvaskit.wasm", fs).Methods("GET")
	server.Router.Handle("/main.dart.js", fs).Methods("GET")
	server.Router.Handle("/manifest.json", fs).Methods("GET")
	// server.Router.Handle("", fs).Methods("GET")
	
	// Login Route
	loginOptions := OptionConfig{Origin: "",
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type"}
	server.Router.HandleFunc("/login", loginOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/login", jsonh(corsh(server.Login))).Methods("POST")

	// Register route
	registerOptions := OptionConfig{Origin: "",
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type"}
	server.Router.HandleFunc("/register", registerOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/register", jsonh(corsh(server.CreateUser))).Methods("POST")

	// DataTable route
	dataTableOptions := OptionConfig{Origin: "",
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type, Authorization"}
	server.Router.HandleFunc("/dataTable", dataTableOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/dataTable", jsonh(corsh(authh(server.DoDataTableAction)))).Methods("POST")

	// RegisterFileKey route
	registerFileKeyOptions := OptionConfig{Origin: "",
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type, Authorization"}
	server.Router.HandleFunc("/registerFileKey", registerFileKeyOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/registerFileKey", jsonh(corsh(authh(server.DoRegisterFileKeyAction)))).Methods("POST")

	// PurgeData route
	purgeDataOptions := OptionConfig{Origin: "",
		AllowedMethods: "POST, OPTIONS",
		AllowedHeaders: "Content-Type, Authorization"}
	server.Router.HandleFunc("/purgeData", purgeDataOptions.options).Methods("OPTIONS")
	server.Router.HandleFunc("/purgeData", jsonh(corsh(authh(server.DoPurgeDataAction)))).Methods("POST")

	//* TODO add options and corrs check - Users routes
	// server.Router.HandleFunc("/register", jsonh(server.CreateUser)).Methods("POST")
	server.Router.HandleFunc("/users", jsonh(authh(server.GetUsers))).Methods("GET")
	server.Router.HandleFunc("/users/info", jsonh(authh(server.GetUserDetails))).Methods("GET")
	server.Router.HandleFunc("/users/{id}", jsonh(authh(server.GetUser))).Methods("GET")
	server.Router.HandleFunc("/users/{id}", jsonh(authh(server.UpdateUser))).Methods("PUT")
	server.Router.HandleFunc("/users/{id}", authh(server.DeleteUser)).Methods("DELETE")


	log.Println("Listening to address ", *serverAddr)
	return http.ListenAndServe(*serverAddr, server.Router)
}
