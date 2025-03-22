package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

type Server struct {
	dbpool             *pgxpool.Pool
	Router             *mux.Router
	AuditLogger        *zap.Logger
	LastSecretRotation *time.Time
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
			// DEV
			// log.Println("*** authh for",r.URL.Path,", Unauthorized")
			ERROR(w, http.StatusUnauthorized, errors.New("Unauthorized"))
			return
		}
		// DEV
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
		// DEV
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
	if jetstoreVersion == "" {
		log.Println("Error, attempting to save empty jetstoreVersion in table jetstore_release, skipping")
		return nil
	}
	// Add version to db
	stmt := "INSERT INTO jetsapi.jetstore_release (version) VALUES ($1)"
	_, err = server.dbpool.Exec(context.Background(), stmt, jetstoreVersion)
	if err != nil {
		return fmt.Errorf("while inserting jetstore version into jetstore_release table: %v", err)
	}
	return nil
}

// Check JetStore DB schema exist, create if not
func (server *Server) checkJetStoreSchema() error {
	tableExists, err := schema.DoesTableExists(server.dbpool, "jetsapi", "jetstore_release")
	if err != nil {
		return fmt.Errorf("while verifying that the jetstore_release table exists: %v", err)
	}
	jetstoreVersion := os.Getenv("JETS_VERSION")
	log.Println("JetStore image version JETS_VERSION is", jetstoreVersion)
	if !tableExists {
		// run update db with workspace init script
		log.Println("JetStore version table does not exist, initializing the db schema")
		// Cleanup any remaining
		_, _, err = server.ResetDomainTables(&PurgeDataAction{
			Action:            "reset_domain_tables",
			RunUiDbInitScript: true,
			Data:              []map[string]interface{}{},
		})
		if err != nil {
			return fmt.Errorf("while calling ResetDomainTables to initialize db schema: %v", err)
		}
		// Updating JetStore version in database
		err = server.addVersionToDb(jetstoreVersion)
		if err != nil {
			return fmt.Errorf("while calling saving jetstoreVersion to database: %v", err)
		}

	}
	return nil
}

// Update JetStore Db
// Precondition: db schema exist
func (server *Server) checkJetStoreDbVersion() error {
	var serverArgs []string
	var version sql.NullString
	jetstoreVersion := os.Getenv("JETS_VERSION")
	log.Println("JetStore image version JETS_VERSION is", jetstoreVersion)

	// Check the release in database vs current release
	stmt := "SELECT MAX(version) FROM jetsapi.jetstore_release"

	err := server.dbpool.QueryRow(context.Background(), stmt).Scan(&version)
	switch {
	case err != nil:
		log.Println("JetStore version is not defined in jetstore_release table, rebuilding all tables and running workspace db init script")
		_, _, err = server.ResetDomainTables(&PurgeDataAction{
			Action:            "reset_domain_tables",
			RunUiDbInitScript: true,
			Data:              []map[string]interface{}{},
		})
		if err != nil {
			return fmt.Errorf("while calling ResetDomainTables to initialize db (no version exist in db): %v", err)
		}

	case !version.Valid || jetstoreVersion > version.String:
		log.Println("JetStore deployed version (in database) is", version.String)
		log.Println("New JetStore Release deployed, running workspace db init script and migrating tables to latest schema")
		serverArgs = []string{"-initBaseWorkspaceDb", "-migrateDb"}
		if *usingSshTunnel {
			serverArgs = append(serverArgs, "-usingSshTunnel")
		}
		_, err = datatable.RunUpdateDb(os.Getenv("WORKSPACE"), &serverArgs)
		if err != nil {
			return fmt.Errorf("while calling RunUpdateDb: %v", err)
		}

	default:
		log.Println("JetStore deployed version (in database) is", version)
		log.Println("JetStore version in database", version, ">=", "JetStore image version", jetstoreVersion)
		// DO NOT UPDATE version in database, hence return here
		return nil
	}

	// Updating JetStore version in database
	err = server.addVersionToDb(jetstoreVersion)
	if err != nil {
		return fmt.Errorf("while calling saving jetstoreVersion to database: %v", err)
	}
	return nil
}

// Get the file_key list of the unit test files
func (server *Server) getUnitTestFileKeys() ([]string, error) {
	workspaceName := os.Getenv("WORKSPACE")
	root := os.Getenv("WORKSPACES_HOME") + "/" + workspaceName
	// Check that the workspace contains a unit_test directory
	_, err := os.Open(root + "/data/test_data")
	if err != nil {
		log.Println("Folder 'data/test_data' does not exists in workspace, skipping copying unit test files")
		return nil, nil
	}
	workspaceNode, err := wsfile.VisitDirWrapper(root, "data/test_data", "Unit Test Data", &[]string{".txt", ".csv"}, workspaceName)
	if err != nil {
		log.Println("while walking workspace unit test folder structure:", err)
		return nil, err
	}
	stack := workspaceNode.Children
	fileKeys := make([]string, 0)
	for len(*stack) > 0 {
		item := (*stack)[0]
		*stack = (*stack)[1:]
		str, err := url.QueryUnescape(item.RouteParams["file_name"])
		if err != nil {
			log.Println("while walking workspace unit test folder structure:", err)
			return nil, err
		}
		if len(str) > 0 {
			fileKeys = append(fileKeys, str)
		}
		if item.Children != nil {
			*stack = append(*stack, *item.Children...)
		}
	}
	return fileKeys, nil
}

func (server *Server) syncUnitTestFiles() {
	// Collect files from local workspace
	log.Println("Copying unit_test files to s3:")
	fileKeys, err := server.getUnitTestFileKeys()
	if err != nil {
		//* TODO Log to a new workspace error table to report in UI
		log.Println("Error while getting unit test file keys:", err)
	} else {
		bucket := os.Getenv("JETS_BUCKET")
		region := os.Getenv("JETS_REGION")
		s3Prefix := os.Getenv("JETS_s3_INPUT_PREFIX")
		workspaceName := os.Getenv("WORKSPACE")
		root := os.Getenv("WORKSPACES_HOME") + "/" + workspaceName
		for i := range fileKeys {
			fileHd, err := os.Open(fmt.Sprintf("%s/%s", root, fileKeys[i]))
			if err != nil {
				log.Println("Error while opening file to copy to s3:", err)
			} else {
				if err = awsi.UploadToS3(bucket, region, strings.Replace(fileKeys[i], "data/test_data", s3Prefix, 1), fileHd); err != nil {
					log.Println("Error while copying to s3:", err)
				}
				fileHd.Close()
			}
		}
	}
}

// Download overriten workspace files from jetstore database
// Check the workspace version in db, if jetstore image version is more recent, recompile workspace
func (server *Server) checkWorkspaceVersion() error {
	var err error
	workspaceName := os.Getenv("WORKSPACE")

	// Copy the workspace files to a stash location (needed when we delete/revert file changes)
	// Skip when in globalDevMode
	if !globalDevMode {
		err = wsfile.StashFiles(workspaceName)
		if err != nil {
			//* TODO Log to a new workspace error table to report in UI
			log.Printf("Error while stashing workspace file: %v", err)
		}
	}

	// Put the active workspace entry into workspace_registry table if ACTIVE_WORKSPACE_URI is set
	activeWorkspaceUri := os.Getenv("ACTIVE_WORKSPACE_URI")
	workspaceBranch := os.Getenv("WORKSPACE_BRANCH")
	if activeWorkspaceUri != "" && workspaceBranch != "" {
		stmt := fmt.Sprintf(`
			INSERT INTO jetsapi.workspace_registry 
				(workspace_name, workspace_uri, workspace_branch, user_email) VALUES 
				('%s', '%s', '%s', 'system')
				ON CONFLICT ON CONSTRAINT workspace_name_unique_cstraintv3
				DO UPDATE SET (workspace_uri, workspace_branch, user_email, last_update) =
				(EXCLUDED.workspace_uri, EXCLUDED.workspace_branch, EXCLUDED.user_email, DEFAULT)`,
			workspaceName, activeWorkspaceUri, workspaceBranch)
		_, err = server.dbpool.Exec(context.Background(), stmt)
		if err != nil {
			log.Printf("while inserting active workspace into workspace_registry table: %v, ignored", err)
		}
	}

	// Check if need to Download overriten workspace files from database & recompile workspace,
	// skip if in dev mode
	if globalDevMode {
		// Local development, do not sync and compile workspace
		return nil
	}

	var version sql.NullString
	jetstoreVersion := os.Getenv("JETS_VERSION")
	// Check the workspace release in database vs current release
	stmt := "SELECT MAX(version) FROM jetsapi.workspace_version"
	err = server.dbpool.QueryRow(context.Background(), stmt).Scan(&version)
	switch {
	case err != nil:
		if errors.Is(err, pgx.ErrNoRows) {
			log.Println("Workspace version is not defined (no rows returned) in workspace_version table, setting it to JetStore version")
			server.syncUnitTestFiles()
		} else {
			return fmt.Errorf("while reading workspace version from workspace_version table: %v", err)
		}

	case !version.Valid:
		log.Println("Workspace version is not defined (null version) in workspace_version table, setting it to JetStore version")
		server.syncUnitTestFiles()

	case jetstoreVersion > version.String:
		// Download overriten workspace files from database if any, skipping sqlite and tgz files since we will recompile workspace
		if err = workspace.SyncWorkspaceFiles(server.dbpool, workspaceName, dbutils.FO_Open, "", true, true); err != nil {
			log.Println("Error (ignored) while synching workspace file from database:", err)
		}
		log.Println("Workspace deployed version (in database) is", version.String)
		// Sync unit test files
		server.syncUnitTestFiles()

	default:
		log.Println("Workspace version in database", version, ">=", "JetStore image version", jetstoreVersion, ", no need to recompile workspace")
		// Download overriten workspace files from database if any, not skipping sqlite/tgz files to get latest in case it was recompiled
		if err = workspace.SyncWorkspaceFiles(server.dbpool, workspaceName, dbutils.FO_Open, "", false, false); err != nil {
			log.Println("Error (ignored) while synching workspace file from database:", err)
		}
		// NOT Recompiling workspace, hence return here
		return nil
	}
	log.Println("Recompiling workspace, set the workspace version to be same as jetstore version")
	_, err = workspace.CompileWorkspace(server.dbpool, workspaceName, jetstoreVersion)
	if err != nil {
		err = fmt.Errorf("error while compiling workspace: %v", err)
		log.Println(err)
		return err
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
			adminPassword, err = awsi.GetCurrentSecretValue(*awsAdminPwdSecret)
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
		*apiSecret, err = awsi.GetCurrentSecretValue(*awsApiSecret)
		if err != nil {
			return fmt.Errorf("while getting apiSecret from aws secret: %v", err)
		}
	}

	// Open db connection
	if *awsDsnSecret != "" {
		// Get the dsn from the aws secret
		*dsn, err = awsi.GetDsnFromSecret(*awsDsnSecret, *usingSshTunnel, *dbPoolSize)
		if err != nil {
			return fmt.Errorf("while getting dsn from aws secret: %v", err)
		}
	}
	server.dbpool, err = pgxpool.Connect(context.Background(), *dsn)
	if err != nil {
		return fmt.Errorf("while opening db connection: %v", err)
	}
	defer server.dbpool.Close()

	// Check that JetStore schema exist
	err = server.checkJetStoreSchema()
	if err != nil {
		return fmt.Errorf("while calling checkJetStoreSchema: %v", err)
	}

	// Check workspace version, compile workspace if needed
	err = server.checkWorkspaceVersion()
	if err != nil {
		return fmt.Errorf("while calling checkWorkspaceVersion: %v", err)
	}

	// Check jetstore version, run update_db if needed
	err = server.checkJetStoreDbVersion()
	if err != nil {
		return fmt.Errorf("while calling checkJetStoreDbVersion: %v", err)
	}

	// Check that the users table and admin user exists
	err = server.initUsers()
	if err != nil {
		return fmt.Errorf("while calling initUsers: %v", err)
	}

	// Create and configure the auditLogger
	// See the documentation for Config and zapcore.EncoderConfig for all the
	// available options.
	rawJSON := []byte(`{
	  "level": "info",
	  "encoding": "json",
	  "outputPaths": ["stdout", "/tmp/logs"],
	  "errorOutputPaths": ["stderr"],
	  "initialFields": {"logger_type": "audit_log"},
	  "encoderConfig": {
	    "messageKey": "message",
	    "levelKey": "level",
	    "levelEncoder": "lowercase"
	  }
	}`)

	var cfg zap.Config
	if err = json.Unmarshal(rawJSON, &cfg); err != nil {
		return fmt.Errorf("while unmarshalling audit logger config: %v", err)
	}
	server.AuditLogger = zap.Must(cfg.Build())
	defer server.AuditLogger.Sync()

	// setup the http routes
	server.Router = mux.NewRouter()
	// server.Initialize(os.Getenv("DB_DRIVER"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_PORT"), os.Getenv("DB_HOST"), os.Getenv("DB_NAME"))

	// Set Routes
	// Home Route
	// server.Router.HandleFunc("/", audit(jsonh(server.Home))).Methods("GET")

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
	server.Router.Handle("/assets/AssetManifest.bin.json", fs).Methods("GET")
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
	server.Router.Handle("/assets/assets/fonts/VictorMono-BoldItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Bold.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-ExtraLightItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-ExtraLight.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Italic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Italic-VariableFont_wght.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-LightItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Light.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-MediumItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Medium.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Regular.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-SemiBoldItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-SemiBold.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-ThinItalic.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-Thin.ttf", fs).Methods("GET")
	server.Router.Handle("/assets/assets/fonts/VictorMono-VariableFont_wght.ttf", fs).Methods("GET")
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

	// //* Currently not used
	// //* TODO add options and corrs check - Users routes
	// // server.Router.HandleFunc("/register", jsonh(server.CreateUser)).Methods("POST")
	// server.Router.HandleFunc("/users", jsonh(authh(server.GetUsers))).Methods("GET")
	// server.Router.HandleFunc("/users/info", jsonh(authh(server.GetUserDetails))).Methods("GET")
	// server.Router.HandleFunc("/users/{id}", jsonh(authh(server.GetUser))).Methods("GET")
	// server.Router.HandleFunc("/users/{id}", jsonh(authh(server.UpdateUser))).Methods("PUT")
	// server.Router.HandleFunc("/users/{id}", authh(server.DeleteUser)).Methods("DELETE")

	// Get the secret rotation version from db
	server.LastSecretRotation, err = server.GetLastSecretRotation()
	if err != nil {
		return fmt.Errorf("while getting last rotation from database: %v", err)
	}

	// Start a background tasks to identify timed out and pending tasks and watch for secret rotation
	if !globalDevMode {
		go func() {
			for {
				time.Sleep(1 * time.Hour)

				// Check if the secrets have rotated
				tm, err := server.GetLastSecretRotation()
				if err != nil {
					log.Println("Warning: while getting last time the secret were rotated from db:", err)
				}
				if tm != nil {
					if server.LastSecretRotation == nil || tm.After(*server.LastSecretRotation) {
						// The secrets have rotated, update the cached value of the secerts
						server.SecretsRotated()
						server.LastSecretRotation = tm
					}
				}

				// Start pending task and check for timeouts
				err = datatable.NewDataTableContext(server.dbpool, false, false, nil, nil).StartPendingTasks("cpipesSM")
				if err != nil {
					log.Println("Warning: while StartPendingTasks for cpipesSM:", err)
				}
			}
		}()
	}

	log.Println("Listening to address ", *serverAddr)
	return http.ListenAndServe(*serverAddr, server.Router)
}

func (server *Server) GetLastSecretRotation() (tm *time.Time, err error) {
	var sqltm sql.NullTime
	err = server.dbpool.QueryRow(context.Background(), "SELECT MAX(last_update) FROM jetsapi.secret_rotation").Scan(&sqltm)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "password authentication failed"):
			now := time.Now()
			return &now, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return nil, fmt.Errorf("while querying last_update from secret_rotation table: %v", err)
		default:
			return nil, nil
		}
	}
	if sqltm.Valid {
		return &sqltm.Time, nil
	}
	return nil, nil
}
