package datatable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	// "strconv"
	// "time"

	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/git"
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/user"

	// "github.com/artisoft-io/jetstore/jets/workspace"
	// "github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

func getWorkspaceUri(dataTableAction *DataTableAction, irow int) string {
	result := os.Getenv("WORKSPACE_URI")
	if result == "" {
		v := dataTableAction.Data[irow]["workspace_uri"]
		if v != nil {
			result = v.(string)
		}
	}
	return result
}

// WorkspaceInsertRows ------------------------------------------------------
// Main insert row function with pre processing hooks for validating/authorizing the request
// Main insert row function with post processing hooks to perform work async
// Inserting rows using pre-defined sql statements, keyed by table name provided in dataTableAction
func (ctx *Context) WorkspaceInsertRows(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	returnedKey := make([]int, len(dataTableAction.Data))
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		return nil, http.StatusBadRequest, errors.New("error: unknown table")
	}
	userProfile, err2 := ctx.VerifyUserPermission(sqlStmt, token)
	if err2 != nil {
		httpStatus = http.StatusUnauthorized
		err = errors.New("error: unauthorized, cannot get user info or does not have permission")
		return
	}
	var gitProfile user.GitProfile
	gitProfile, gitProfileErr := user.GetGitProfile(ctx.Dbpool, userProfile.Email)

	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		// Pre-Processing hook
		// -----------------------------------------------------------------------
		var gitLog, status string
		switch {
		case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "WORKSPACE/"):
			sqlStmt.Stmt = strings.ReplaceAll(sqlStmt.Stmt, "$SCHEMA", dataTableAction.FromClauses[0].Schema)

		case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "workspace_registry"):
			// Insert or update workspace entry in workspace_registry table:
			//	- If folder workspace_name in workspaces root does not exists, 
			//    chechout branch (workspace_branch) from workspace_uri in workspace_name, 
			//    switch to feature_branch
			//  - If user is renaming workspace_name, delete the old workspace folder under workspaces root
			//    Note: UI must provide old workspace name as 'previous.workspace_name' virtual column
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for update workspace_registry, missing workspace_name")
			}
			wsUri := getWorkspaceUri(dataTableAction, irow)
			if gitProfileErr != nil {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
			}
			gitUser :=      gitProfile.Name
			gitToken :=     gitProfile.GitToken
			gitUserName :=  gitProfile.GitHandle
			gitUserEmail := gitProfile.Email
			wsPN := dataTableAction.Data[irow]["previous.workspace_name"]
			if(wsUri == "" || gitUser == "" || gitToken == "" || 
				gitUserName == "" || gitUserEmail == "") {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for update workspace_registry, missing git information")
			}
			var wsPreviousName string
			if(wsPN != nil) {
				wsPreviousName = wsPN.(string)
			}

			workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
				WorkspaceName   : dataTableAction.WorkspaceName,
				WorkspaceUri    : wsUri,
				WorkspaceBranch : dataTableAction.WorkspaceBranch,
				FeatureBranch   : dataTableAction.FeatureBranch,
			})
			gitLog, err = workspaceGit.UpdateLocalWorkspace(
				gitProfile.Name,
				gitProfile.Email,
				gitProfile.GitHandle,
				gitProfile.GitToken,
				wsPreviousName,
			)
			if err != nil {
				log.Printf("Error while updating local workspace: %s\n", gitLog)
				httpStatus = http.StatusBadRequest
				status = "error"
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = status

		case dataTableAction.FromClauses[0].Table == "commit_workspace":
			// Validating request
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for commit_workspace, missing workspace_name")
			}
			wsUri := getWorkspaceUri(dataTableAction, irow)
			if gitProfileErr != nil {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
			}
			gitUser := gitProfile.Name
			gitToken := gitProfile.GitToken
			if(wsUri == "" || gitUser == "" || gitToken == "") {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for commit_workspace, missing git information")
			}
			// Commit changes in local workspace and push to repository:
			//	- Commit and Push to repository
			//  NOTE:
			//	- Delete workspace overrides
			//	  (except for workspace.db, workspace.tgz, lookup.db, and reports.tgz)
			//	- Compile workspace must be done manually
			wsCM := dataTableAction.Data[irow]["git.commit.message"]
			var wsCommitMessage string
			if(wsCM != nil) {
				// escape singe ' with ''
				wsCommitMessage = strings.ReplaceAll(wsCM.(string), "'", "''")
			}
			workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
				WorkspaceName   : dataTableAction.WorkspaceName,
				WorkspaceUri    : wsUri,
				WorkspaceBranch : dataTableAction.WorkspaceBranch,
				FeatureBranch   : dataTableAction.FeatureBranch,
			})
			var buf strings.Builder
			// Commit and push workspace changes and update workspace_registry table
			gitLog, err = workspaceGit.CommitLocalWorkspace(&gitProfile,	wsCommitMessage)
			buf.WriteString(gitLog)
			buf.WriteString("\n")
			if err != nil {
				status = "error"
			} else {
				// Delete all workspace overrides w/o restaure from stash
				err = wsfile.DeleteAllFileChanges(ctx.Dbpool, dataTableAction.WorkspaceName, false, true)
				if err != nil {
					status = "error"
				}
			}
			dataTableAction.Data[irow]["last_git_log"] = buf.String()
			dataTableAction.Data[irow]["status"] = status
	
		case dataTableAction.FromClauses[0].Table == "git_command_workspace":
			// Execute git commands in workspace
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_command_workspace, missing workspace_name")
			}
			wsUri := getWorkspaceUri(dataTableAction, irow)
			gitCommand := dataTableAction.Data[irow]["git.command"]
			if(wsUri == "" || gitCommand == nil) {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_command_workspace, missing git information")
			}
			workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
				WorkspaceName   : dataTableAction.WorkspaceName,
				WorkspaceUri    : wsUri,
				WorkspaceBranch : dataTableAction.WorkspaceBranch,
				FeatureBranch   : dataTableAction.FeatureBranch,
			})
			gitLog, err = workspaceGit.GitCommandWorkspace(gitCommand.(string))
			if err != nil {
				log.Printf("Error while git status workspace: %s\n", gitLog)
				httpStatus = http.StatusBadRequest
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = ""

		case dataTableAction.FromClauses[0].Table == "git_status_workspace":
			// Execute git status commands in workspace
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_status_workspace, missing workspace_name")
			}
			wsUri := getWorkspaceUri(dataTableAction, irow)
			if(wsUri == "") {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_status_workspace, missing git information")
			}
			workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
				WorkspaceName   : dataTableAction.WorkspaceName,
				WorkspaceUri    : wsUri,
				WorkspaceBranch : dataTableAction.WorkspaceBranch,
				FeatureBranch   : dataTableAction.FeatureBranch,
			})
			gitLog, err = workspaceGit.GitCommandWorkspace("git status")
			if err != nil {
				log.Printf("Error while git status in workspace: %s\n", gitLog)
				httpStatus = http.StatusBadRequest
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = ""

		case dataTableAction.FromClauses[0].Table == "push_only_workspace":
			// Push only workspace
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for push_only_workspace, missing workspace_name")
			}
			wsUri := getWorkspaceUri(dataTableAction, irow)
			if gitProfileErr != nil {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
			}
			gitUser := gitProfile.Name
			gitToken := gitProfile.GitToken
			if(wsUri == "" || gitUser == "" || gitToken == "") {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for push_only_workspace, missing git information")
			}
			workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
				WorkspaceName   : dataTableAction.WorkspaceName,
				WorkspaceUri    : wsUri,
				WorkspaceBranch : dataTableAction.WorkspaceBranch,
				FeatureBranch   : dataTableAction.FeatureBranch,
			})
			gitLog, err = workspaceGit.PushOnlyWorkspace(gitUser, gitToken)
			if err != nil {
				log.Printf("Error while push (only) workspace: %s\n", gitLog)
				httpStatus = http.StatusBadRequest
				status = "error"
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = status

		case dataTableAction.FromClauses[0].Table == "pull_workspace":
			// Pull changes by merging WorkspaceBranch into current branch
			// Apply workspace overrides (except for compiled files)
			// Optionally, compile workspace and load client config
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for pull_workspace, missing workspace_name")
			}
			wsUri := getWorkspaceUri(dataTableAction, irow)
			if gitProfileErr != nil {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
			}
			gitUser := gitProfile.Name
			gitToken := gitProfile.GitToken
			if(wsUri == "" || gitUser == "" || gitToken == "") {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for pull_workspace, missing git information")
			}
			gitLog, err = pullWorkspaceAction(ctx.Dbpool, irow, &gitProfile, dataTableAction)
			if err != nil {
				log.Printf("Error while pull workspace: %v\nLog: %s\n", err, gitLog)
				httpStatus = http.StatusBadRequest
				status = "error"
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			if status != "error" {
				// Check if compile_workspace is requested, if not check if load client config is requested
				otherActions := dataTableAction.Data[irow]["otherWorkspaceActionOptions"]
				if(otherActions != nil) {
					l := otherActions.([]interface{})
					compileWorkspaceStarted := false
					for i := range l {
						if l[i] != nil && l[i] == "wpCompileWorkspaceOption" {
							status = "Compiling workspace in progress"
							go compileWorkspaceAction(ctx, dataTableAction)
							compileWorkspaceStarted = true
						}
					}
					if !compileWorkspaceStarted {
						for i := range l {
							if l[i] != nil && (l[i] == "wpLoadClientConfgOption" || l[i] == "wpLoadSelectedClientConfgOption") {
								status = "Loading client config in progress"
								go loadWorkspaceConfigAction(ctx, dataTableAction)
							}
						}	
					}
				}
			}
			dataTableAction.Data[irow]["status"] = status

		case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "compile_workspace"):
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invaid request for compile_workspace, missing workspace_name")
			}
			dataTableAction.Data[irow]["status"] = "Compile in progress"

		case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "load_workspace_config"):
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invaid request for load_workspace_config, missing workspace_name")
			}
			dataTableAction.Data[irow]["status"] = ""


		case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "unit_test"):
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invaid request for unit_test, missing workspace_name")
			}
			dataTableAction.Data[irow]["status"] = "Unit Test in progress"

		case dataTableAction.FromClauses[0].Table == "delete_workspace":
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invaid request for delete/workspace_registry, missing workspace_name")
			}
			// Delete entry in workspace_registry table:
			//	- It is an error to delete the active workspace
			//	- Delete folder with workspace_name under workspaces root
			//	- Delete in workspace_registry table by key (done below by the main sqlStmt)
			dataTableAction.Data[irow]["status"] = ""
			workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
				WorkspaceName   : dataTableAction.WorkspaceName,
				WorkspaceUri    : "",
				WorkspaceBranch : dataTableAction.WorkspaceBranch,
				FeatureBranch   : dataTableAction.FeatureBranch,
			})
			err = workspaceGit.DeleteWorkspace()
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
		}

		// Perform the Insert Rows
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
		// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
		// Executing the InserRow Stmt
		var dbErr error
		if strings.Contains(sqlStmt.Stmt, "RETURNING key") {
			dbErr = ctx.Dbpool.QueryRow(context.Background(), sqlStmt.Stmt, row...).Scan(&returnedKey[irow])
		} else {
			_, dbErr = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		}
		if dbErr != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, dbErr)
			if err == nil {
				err = dbErr
				if strings.Contains(err.Error(), "duplicate key value") {
					httpStatus = http.StatusConflict
					err = errors.New("duplicate key value")
				} else {
					httpStatus = http.StatusInternalServerError
					err = fmt.Errorf("while inserting in table %s: %v", dataTableAction.FromClauses[0].Table, dbErr)
				}	
			}
		}
		if err != nil {
			// Break from the data loop
			goto returnResults
		}
	}

	// Post Processing Hook
	// -----------------------------------------------------------------------
	switch {
	case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "compile_workspace"):
		//	- Compile workspace (workspace.db, lookup.db, and reports.tgz)
		go compileWorkspaceAction(ctx, dataTableAction)

	case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "unit_test"):
		go UnitTestWorkspaceAction(ctx, dataTableAction, token)

	case dataTableAction.FromClauses[0].Table == "load_workspace_config":
		// Load workspace config
		loadWorkspaceConfigAction(ctx, dataTableAction)

	}
	returnResults:
	results = &map[string]interface{}{
		"returned_keys": &returnedKey,
	}
	return
}

// DoWorkspaceReadAction ------------------------------------------------------
func (ctx *Context) DoWorkspaceReadAction(dataTableAction *DataTableAction, token string) (*map[string]interface{}, int, error) {

	// Replace table schema with value $SCHEMA with the workspace_name
	//* NOTE: Reading directly from sqlite, no schema needed (set $SCHEMA to empty)
	for i := range dataTableAction.FromClauses {
		if dataTableAction.FromClauses[i].Schema == "$SCHEMA" {
			dataTableAction.FromClauses[i].Schema = ""
		}
	}
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		return nil, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info or does not have permission")
	}

	// to package up the result
	results := make(map[string]interface{})
	var err error

	if len(dataTableAction.Columns) == 0 {
		return nil, http.StatusNotImplemented, fmt.Errorf("Column names must be provided")
	}

	// Pre Processing Hook
	// -----------------------------------------------------------------------
	switch {
	case dataTableAction.FromClauses[0].Table == "workspace_registry":
		// None for now
	default:
		if dataTableAction.WorkspaceName == "" {
			return nil, http.StatusBadRequest, fmt.Errorf("invaid request, missing workspace_name")
		}	
	}

	// Build the query
	query, nbrRowsQuery := dataTableAction.buildQuery()

	// Perform the query
	var resultRows *[][]interface{}
	var totalRowCount int
	if dataTableAction.FromClauses[0].Schema == "jetsapi" {
		resultRows, _, err = execQuery(ctx.Dbpool, dataTableAction, &query)
		if err != nil {
			return nil, http.StatusInternalServerError,
				fmt.Errorf("while executing query from tables %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
		// Post Processing Hook
		// -----------------------------------------------------------------------
		switch {
		case dataTableAction.FromClauses[0].Table == "workspace_registry":
			// Post processing for workspace_registry table to get status from file system:
			//	- If workspace_registry.status == 'error', then status = 'error'
			//  - If workspace_name folder does not exist: status = removed
			//  - If workspace_name == os.Getenv("WORKSPACE") && workspace_branch == os.Getenv("WORKSPACE_BRANCH"): 
			//			- status = 'active' if local branch set to feature_branch (i.e. != workspace_branch)
			//			- status = 'active, missing feature branch' if local branch == workspace_branch
			//  - If git status in workspace_name folder contains 'nothing to commit, working tree clean': status = no changes
			//  - else: status = modified
			// Get the column position for workspace_name, workspace_branch, feature_branch and status
			workspaceNamePos := -1
			workspaceBranchPos := -1
			featureBranchPos := -1
			workspaceUriPos := -1
			statusPos := -1
			missingColumns := true
			for i := range dataTableAction.Columns {
				switch dataTableAction.Columns[i].Column {
				case "workspace_name":
					workspaceNamePos = i
				case "workspace_uri":
					workspaceUriPos = i
				case "workspace_branch":
					workspaceBranchPos = i
				case "feature_branch":
					featureBranchPos = i
				case "status":
					statusPos = i
				}
				if workspaceNamePos > -1 &&
					workspaceBranchPos > -1 &&
					featureBranchPos > -1 &&
					workspaceUriPos > -1 &&
					statusPos > -1 {	
						missingColumns = false
						goto done
				}
		}
			done: 
			if missingColumns {
				fmt.Println("Oops expecting workspace_name, workspace_uri, workspace_branch, feature_branch and status columns")
			} else {
				// Get the status from git command
				for irow := range *resultRows {
					if (*resultRows)[irow][statusPos] == "" {
						workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
							WorkspaceName   : (*resultRows)[irow][workspaceNamePos].(string),
							WorkspaceUri    : (*resultRows)[irow][workspaceUriPos].(string),
							WorkspaceBranch : (*resultRows)[irow][workspaceBranchPos].(string),
						})
						status, err := workspaceGit.GetStatus()
						if err != nil {
							return nil, http.StatusBadRequest, err
						}
						(*resultRows)[irow][statusPos] = status	
						(*resultRows)[irow][featureBranchPos] = workspaceGit.FeatureBranch
					}
				}
			}
		}

		// get the total nbr of row
		err = ctx.Dbpool.QueryRow(context.Background(), nbrRowsQuery).Scan(&totalRowCount)
		if err != nil {
			return nil, http.StatusInternalServerError,
				fmt.Errorf("while getting total row count from tables %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	} else {
		// Query the workspace sqlite db
		workspaceDsn := fmt.Sprintf("%s/%s/workspace.db", os.Getenv("WORKSPACES_HOME"), dataTableAction.WorkspaceName)
		db, err := sql.Open("sqlite3", workspaceDsn) // Open the created SQLite File
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("while opening workspace db: %v", err)
		}
		resultRows, err = execWorkspaceQuery(db, dataTableAction, &query)
		if err != nil {
			return nil, http.StatusInternalServerError,
				fmt.Errorf("while executing workspace query from tables %s: %v", dataTableAction.FromClauses[0].Table, err)
		}

		// get the total nbr of row
		err = db.QueryRow(nbrRowsQuery).Scan(&totalRowCount)
		if err != nil {
			return nil, http.StatusInternalServerError,
				fmt.Errorf("while getting total row count from workspace tables %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	}

	results["totalRowCount"] = totalRowCount
	results["rows"] = resultRows
	return &results, http.StatusOK, nil
}

// WorkspaceQueryStructure ------------------------------------------------------
// Function to query the workspace structure, it returns a hierarchical structure
// modeled based on ui MenuEntry class.
// It uses a virtual table name to indicate the level of granularity of the structure
// dataTableAction.FromClauses[0].Table:
//
//	case "workspace_file_structure": structure based on files of the workspace
//	case "workspace_object_structure": structure based on object (rule, lookup, class, etc) of the workspace
//
// Initial implementation use workspace_file_structure
// NOTE: routePath must correspond to the parametrized url (needed by ui MenuEntry)
// NOTE: routeParam contains the routePath parameters (needed by ui MenuEntry)
// Input dataTableAction.Data:
//
//	[
//		{
//			"key": "123",
//			"workspace_name": "jets_ws",
//			"user_email": "email here"
//		}
//	]
//
// Output results:
//
//				{
//					"key": "123",
//					"workspace_name": "jets_ws",
//				  "result_type": "workspace_file_structure",
//					"result_data": [
//						{
//							"key": "a1",
//	           "type": "dir",
//							"label": "Jet Rules",
//							"route_path": "/workspace/:workspace_name/jetRules",
//							"route_params": {
//									"workspace_name": "jets_ws",
//							},
//							"children": [
//								{
//									"key": "a1.1",
//	               "type": "dir",
//									"label": "folder name",
//									"children": [
//										{
//											"key": "a1.1.1",
//	                   "type": "file",
//											"label": "mapping_rules.jr",
//											"route_path": "/workspace/:workspace_name/wsFile/:file_name",
//											"route_params": {
//												"workspace_name": "jets_ws",
//												"file_name": "jet_rules%03mapping_rules.jr",
//											}
//								 	  }
//									]
//								}
//							]
//						}
//					]
//				}
func (ctx *Context) WorkspaceQueryStructure(dataTableAction *DataTableAction, token string) (results *[]byte, httpStatus int, err error) {
	// Validate the arguments
	if len(dataTableAction.Data) == 0 || len(dataTableAction.FromClauses) == 0 {
		httpStatus = http.StatusBadRequest
		err = errors.New("incomplete request")
		return
	}
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		httpStatus = http.StatusBadRequest
		err = errors.New("incomplete request")
		return
	}
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		httpStatus = http.StatusUnauthorized
		err = errors.New("error: unauthorized, cannot get user info or does not have permission")
		return
	}

	// Request type indicates the granularity of the result (file or object)
	requestType := dataTableAction.FromClauses[0].Table

	// Prepare the return object
	httpStatus = http.StatusOK
	resultData := make([]*wsfile.WorkspaceNode, 0)
	root := os.Getenv("WORKSPACES_HOME") + "/" + workspaceName
	var workspaceNode *wsfile.WorkspaceNode

	switch requestType {
	case "workspace_file_structure":
		// Data Model (.jr)
		// fmt.Println("** Visiting data_model:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "data_model", "Data Model", &[]string{".jr", ".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Jets Rules (.jr, .jr.sql)
		// fmt.Println("** Visiting jet_rules:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "jet_rules", "Jets Rules", &[]string{".jr", ".jr.sql"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Lookups (.jr)
		// fmt.Println("** Visiting lookups:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "lookups", "Lookups", &[]string{".jr", ".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// cpipes config (.pc.json)
		fmt.Println("** Visiting pipes_config:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "pipes_config", "Pipes Config", &[]string{".pc.json"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Process Configurations (workspace_init_db.sql)
		// fmt.Println("** Visiting process_config:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "process_config", "Process Configuration", &[]string{"workspace_init_db.sql"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Process Sequences (.jr)
		// fmt.Println("** Visiting process_sequence:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "process_sequence", "Process Sequences", &[]string{".jr"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Reports (.sql, .json)
		// fmt.Println("** Visiting reports:")
		workspaceNode, err = wsfile.VisitDirWrapper(root, "reports", "Reports", &[]string{".sql", ".json"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// compile_workspace.sh
		resultData = append(resultData, &wsfile.WorkspaceNode{
			Key:          "compile_workspace",
			Type:         "file",
			PageMatchKey: "compile_workspace.sh",
			Label:        "Compile Workspace Script",
			RoutePath:    "/workspace/:workspace_name/home",
			RouteParams: map[string]string{
				"workspace_name": workspaceName,
				"file_name":      url.QueryEscape("compile_workspace.sh"),
				"label":          "compile_workspace.sh",
			},
		})

		// workspace_control.json
		resultData = append(resultData, &wsfile.WorkspaceNode{
			Key:          "workspace_control",
			Type:         "file",
			PageMatchKey: "workspace_control.json",
			Label:        "Workspace Control",
			RoutePath:    "/workspace/:workspace_name/home",
			RouteParams: map[string]string{
				"workspace_name": workspaceName,
				"file_name":      url.QueryEscape("workspace_control.json"),
				"label":          "workspace_control.json",
			},
		})
	default:
		httpStatus = http.StatusBadRequest
		err = errors.New("invalid workspace request type")
		return
	}

	var v []byte
	v, err = json.Marshal(wsfile.WorkspaceStructure{
		WorkspaceName: workspaceName,
		ResultType:    requestType,
		ResultData:    &resultData,
	})
	// v, err = json.MarshalIndent(WorkspaceStructure{
	// 	WorkspaceName: workspaceName,
	// 	ResultType: requestType,
	// 	ResultData: &resultData,
	// },"", "  ")
	// //*
	// fmt.Println("*** Workspace Structure ***")
	// fmt.Println(string(v))
	// fmt.Println("*** Workspace Structure ***")
	results = &v
	return
}

// AddWorkspaceFile --------------------------------------------------------------------------
// Function to add a workspace file
func (ctx *Context) addWorkspaceFile(dataTableAction *DataTableAction, _ string) (err error) {
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name")
		fmt.Println(err)
		return
	}
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsFileName := request["source_file_name"]
		if wsFileName == nil {
			err = fmt.Errorf("GetWorkspaceFileContent: missing file_name")
			fmt.Println(err)
			return
		}
		var fileName string
		fileName, err = url.QueryUnescape(wsFileName.(string))
		fullFileName := fmt.Sprintf("%s/%s/%s",os.Getenv("WORKSPACES_HOME"),workspaceName, fileName)
		if err != nil {
			fmt.Println(err)
			return
		}
	
		// Create an empty file to local workspace
		var myfile *os.File
		fileDir :=filepath.Dir(fullFileName)
    if err = os.MkdirAll(fileDir, 0770); err != nil {
			err = fmt.Errorf("while creating file directory structure: %v", err)
			fmt.Println(err)
			return
		}

		myfile, err = os.Create(fullFileName) 
    if err != nil { 
			err = fmt.Errorf("while creating workspace file: %v", err)
			fmt.Println(err)
			return
    } 
    myfile.Close() 		
	}
	return
}

// AddWorkspaceFile
func (ctx *Context) AddWorkspaceFile(dataTableAction *DataTableAction, token string) (rb *[]byte, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		httpStatus = http.StatusUnauthorized
		err = errors.New("error: unauthorized, cannot get user info or does not have permission")
		return
	}
	httpStatus = http.StatusOK
	err = ctx.addWorkspaceFile(dataTableAction, token)
	if err != nil {
		httpStatus = http.StatusBadRequest
		return
	}
	dataTableAction.Action = "workspace_query_structure"
	dataTableAction.FromClauses = []FromClause{{Table: "workspace_file_structure"}}
	return ctx.WorkspaceQueryStructure(dataTableAction, token)
}

// DeleteWorkspaceFile
func (ctx *Context) DeleteWorkspaceFile(dataTableAction *DataTableAction, token string) (rb *[]byte, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		httpStatus = http.StatusUnauthorized
		err = errors.New("error: unauthorized, cannot get user info or does not have permission")
		return
	}
	httpStatus = http.StatusOK
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsFileName := request["source_file_name"]
		if wsFileName == nil {
			err = fmt.Errorf("GetWorkspaceFileContent: missing file_name")
			fmt.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		var fileName string
		if fileName, err = url.QueryUnescape(wsFileName.(string)); err != nil {
			fmt.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		fullFileName := fmt.Sprintf("%s/%s/%s",os.Getenv("WORKSPACES_HOME"),workspaceName, fileName)
		// Write empty file to local workspace & db
		if err = wsfile.SaveContent(ctx.Dbpool, workspaceName, fileName, ""); err != nil {
			fmt.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}

		// Delete the local file
		err = os.Remove(fullFileName) 
    if err != nil { 
			err = fmt.Errorf("while removing workspace file: %v", err)
			fmt.Println(err)
			httpStatus = http.StatusBadRequest
			return
    } 		
	}
	dataTableAction.Action = "workspace_query_structure"
	dataTableAction.FromClauses = []FromClause{{Table: "workspace_file_structure"}}
	return ctx.WorkspaceQueryStructure(dataTableAction, token)
}


// GetWorkspaceFileContent --------------------------------------------------------------------------
// Function to get the workspace file content based on relative file name
// Read the file from the workspace on file system since it's already in sync with database
func (ctx *Context) GetWorkspaceFileContent(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		return nil, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info or does not have permission")
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	wsFileName := request["file_name"]
	if workspaceName == "" || wsFileName == nil {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name or file_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Read file from local workspace
	content, err := wsfile.GetContent(workspaceName,  fileName)
	results = &map[string]interface{}{
		"file_name":    wsFileName,
		"file_content": content,
	}
	return
}

// SaveWorkspaceFileContent --------------------------------------------------------------------------
// Function to save the workspace file content in local workspace file system and in database
func (ctx *Context) SaveWorkspaceFileContent(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		return nil, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info or does not have permission")
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	wsFileName := request["file_name"]
	wsFileContent := request["file_content"]
	if workspaceName == "" || wsFileName == nil || wsFileContent == nil {
		err = fmt.Errorf("SaveWorkspaceFileContent: missing workspace_name, file_content, or file_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Write file to local workspace
	err = wsfile.SaveContent(ctx.Dbpool, workspaceName, fileName, wsFileContent.(string))
	results = &map[string]interface{}{
		"file_name": wsFileName,
	}
	return
}

// SaveWorkspaceClientConfig --------------------------------------------------------------------------
// Function to save the workspace file content in local workspace file system and in database
func (ctx *Context) SaveWorkspaceClientConfig(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		return nil, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info or does not have permission")
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	clientName := request["client"]
	if workspaceName == "" || clientName == nil {
		err = fmt.Errorf("SaveWorkspaceClientConfig: missing workspace_name, or client")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Save client config to local workspace
	err = wsfile.SaveClientConfig(ctx.Dbpool, workspaceName, clientName.(string))
	results = &map[string]interface{}{}
	return
}

// DeleteWorkspaceChanges --------------------------------------------------------------------------
// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
// Restaure files from stash, except for .db and .tgz files
func (ctx *Context) DeleteWorkspaceChanges(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		return nil, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info or does not have permission")
	}
	httpStatus = http.StatusOK
	workspaceName := dataTableAction.WorkspaceName
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsOid := request["oid"]
		wsFileName := request["file_name"]
		wsKey := request["key"]
		if workspaceName == "" || wsOid == nil || wsFileName == nil || wsKey == nil {
			err = fmt.Errorf("DeleteWorkspaceChanges: missing workspace_name, oid, key, or file_name")
			fmt.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		err = wsfile.DeleteFileChange(ctx.Dbpool, wsKey.(string), workspaceName, wsFileName.(string), wsOid.(string))
		if err != nil {
			httpStatus = http.StatusBadRequest
			return
		}
	}

	results = &map[string]interface{}{}
	return
}

// DeleteAllWorkspaceChanges --------------------------------------------------------------------------
// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
func (ctx *Context) DeleteAllWorkspaceChanges(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		return nil, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info or does not have permission")
	}
	httpStatus = http.StatusOK
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		err = fmt.Errorf("DeleteAllWorkspaceChanges: missing workspace_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	// Delete all workspace changes and restaure from stash
	err = wsfile.DeleteAllFileChanges(ctx.Dbpool, workspaceName, true, false)
	if err != nil {
		httpStatus = http.StatusBadRequest
		return
	}

	results = &map[string]interface{}{}
	return
}
