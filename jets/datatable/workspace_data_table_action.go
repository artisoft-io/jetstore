package datatable

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"time"

	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/git"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

// WorkspaceInsertRows ------------------------------------------------------
// Main insert row function with pre processing hooks for validating/authorizing the request
// Main insert row function with post processing hooks for starting pipelines
// Inserting rows using pre-defined sql statements, keyed by table name provided in dataTableAction
func (ctx *Context) WorkspaceInsertRows(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	returnedKey := make([]int, len(dataTableAction.Data))
	httpStatus = http.StatusOK
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: unknown table")
		return
	}
	// Check if stmt is reserved for admin only
	if sqlStmt.AdminOnly {
		userEmail, err2 := user.ExtractTokenID(token)
		if err2 != nil || userEmail != *ctx.AdminEmail {
			httpStatus = http.StatusUnauthorized
			err = errors.New("error: unauthorized, only admin can delete users")
			return
		}
	}
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		// Pre-Processing hook
		// -----------------------------------------------------------------------
		switch {
		case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "WORKSPACE/"):
			sqlStmt.Stmt = strings.ReplaceAll(sqlStmt.Stmt, "$SCHEMA", dataTableAction.FromClauses[0].Schema)

		case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "workspace_registry"):
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for update workspace_registry, missing workspace_name")
			}
			// Insert or update workspace entry in workspace_registry table:
			//	- If folder workspace_name in workspaces root does not exists, chechout workspace_uri in workspace_name
			//  - If user is renaming workspace_name, delete the old workspace folder under workspaces root
			//    Note: UI must provide old workspace name as 'previous.workspace_name' virtual column
			wsName := dataTableAction.Data[irow]["workspace_name"]
			wsUri := dataTableAction.Data[irow]["workspace_uri"]
			gitUser := dataTableAction.Data[irow]["git.user"]
			gitToken := dataTableAction.Data[irow]["git.token"]
			gitUserName := dataTableAction.Data[irow]["git.user.name"]
			gitUserEmail := dataTableAction.Data[irow]["git.user.email"]
			wsPN := dataTableAction.Data[irow]["previous.workspace_name"]
			if(wsName == nil || wsUri == nil || gitUser == nil || gitToken == nil || 
				gitUserName == nil || gitUserEmail == nil) {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for update workspace_registry, missing git information")
			}
			var wsPreviousName string
			if(wsPN != nil) {
				wsPreviousName = wsPN.(string)
			}

			workspaceGit := git.NewWorkspaceGit(wsName.(string), wsUri.(string))
			gitLog, err := workspaceGit.UpdateLocalWorkspace(
				gitUserName.(string),
				gitUserEmail.(string),
				gitUser.(string),
				gitToken.(string),
				wsPreviousName,
			)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog

		case dataTableAction.FromClauses[0].Table == "commit_workspace":
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for commit_workspace, missing workspace_name")
			}
			// Commit and push workspace changes and update workspace_registry table
			wsName := dataTableAction.Data[irow]["workspace_name"]
			wsUri := dataTableAction.Data[irow]["workspace_uri"]
			gitUser := dataTableAction.Data[irow]["git.user"]
			gitToken := dataTableAction.Data[irow]["git.token"]
			if(wsName == nil || wsUri == nil || gitUser == nil || gitToken == nil) {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for commit_workspace, missing git information")
			}

			workspaceGit := git.NewWorkspaceGit(wsName.(string), wsUri.(string))
			gitLog, err := workspaceGit.CommitLocalWorkspace(
				gitUser.(string),
				gitToken.(string),
			)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog

		case dataTableAction.FromClauses[0].Table == "pull_workspace":
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for pull_workspace, missing workspace_name")
			}
			// Pull workspace changes, update workspace_registry table and delete overrides in workspace_changes
			wsName := dataTableAction.Data[irow]["workspace_name"]
			wsUri := dataTableAction.Data[irow]["workspace_uri"]
			gitUser := dataTableAction.Data[irow]["git.user"]
			gitToken := dataTableAction.Data[irow]["git.token"]
			if(wsName == nil || wsUri == nil || gitUser == nil || gitToken == nil) {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for pull_workspace, missing git information")
			}

			workspaceGit := git.NewWorkspaceGit(wsName.(string), wsUri.(string))
			gitLog, err := workspaceGit.PullRemoteWorkspace(
				gitUser.(string),
				gitToken.(string),
			)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog

			//* TODO: Delete workspace overrides (except for workspace.db and lookup.db)
			// Note, do not restaure files from stash
			// Create new stash corresponding to this pulled workspace


		case dataTableAction.FromClauses[0].Table == "compile_workspace":
			workspaceName := dataTableAction.WorkspaceName
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invaid request for compile_workspace, missing workspace_name")
			}
		
			fmt.Println("Compiling workspace", workspaceName)
			err = workspace.CompileWorkspace(ctx.Dbpool, workspaceName, strconv.FormatInt(time.Now().Unix(), 10))
			if err != nil {
				log.Printf("While compiling workspace %s: %v", workspaceName, err)
				httpStatus = http.StatusBadRequest
				err = errors.New("error compiling workspace")
				return
			}

		case dataTableAction.FromClauses[0].Table == "delete/workspace_registry":
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invaid request for delete/workspace_registry, missing workspace_name")
			}
			//* TODO: Delete entry in workspace_registry table:
			//	- Delete in workspace_registry table by key
			//	- Delete folder with workspace_name under workspaces root

		}

		// Perform the Insert Rows
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
		// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
		// Executing the InserRow Stmt
		if strings.Contains(sqlStmt.Stmt, "RETURNING key") {
			err = ctx.Dbpool.QueryRow(context.Background(), sqlStmt.Stmt, row...).Scan(&returnedKey[irow])
		} else {
			_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		}
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
			if strings.Contains(err.Error(), "duplicate key value") {
				httpStatus = http.StatusConflict
				err = errors.New("duplicate key value")
				return
			} else {
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while inserting into a table")
			}
		}
	}

	// Post Processing Hook
	// -----------------------------------------------------------------------
	switch {

	}
	results = &map[string]interface{}{
		"returned_keys": &returnedKey,
	}
	return
}

// DoWorkspaceReadAction ------------------------------------------------------
func (ctx *Context) DoWorkspaceReadAction(dataTableAction *DataTableAction) (*map[string]interface{}, int, error) {

	// Replace table schema with value $SCHEMA with the workspace_name
	//* NOTE: Reading directly from sqlite, no schema needed (set $SCHEMA to empty)
	for i := range dataTableAction.FromClauses {
		if dataTableAction.FromClauses[i].Schema == "$SCHEMA" {
			dataTableAction.FromClauses[i].Schema = ""
		}
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
			//  - If workspace_name folder does not exist: status = removed
			//  - If workspace_name == os.Getenv("WORKSPACE"): status = active
			//  - If git status in workspace_name folder contains 'nothing to commit, working tree clean': status = no changes
			//  - else: status = modified
			// Get the column position for workspace_name and status
			workspaceNamePos := -1
			workspaceUriPos := -1
			statusPos := -1
			for i := range dataTableAction.Columns {
				switch dataTableAction.Columns[i].Column {
				case "workspace_name":
					workspaceNamePos = i
					if statusPos > -1 && workspaceUriPos > -1 {
						goto done
					}
				case "workspace_uri":
					workspaceUriPos = i
					if workspaceNamePos > -1 && statusPos > -1 {
						goto done
					}
				case "status":
					statusPos = i
					if workspaceNamePos > -1 && workspaceUriPos > -1 {
						goto done
					}
				}
			}
			done: 
			if workspaceNamePos < 0 || workspaceUriPos < 0 || statusPos < 0 {
				fmt.Println("Oops expecting workspace_name, workspace_uri and status columns")
			} else {
				// Get the status from git command
				for irow := range *resultRows {
					workspaceGit := git.NewWorkspaceGit(
						(*resultRows)[irow][workspaceNamePos].(string),
						(*resultRows)[irow][workspaceUriPos].(string))
					status, err := workspaceGit.GetStatus()
					if err != nil {
						return nil, http.StatusBadRequest, err
					}
					(*resultRows)[irow][statusPos] = status
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

// This struct correspond to MenuEntry for the ui
type WorkspaceStructure struct {
	Key           string            `json:"key"`
	WorkspaceName string            `json:"workspace_name"`
	ResultType    string            `json:"result_type"`
	ResultData    *[]*WorkspaceNode `json:"result_data"`
}
type WorkspaceNode struct {
	Key          string            `json:"key"`
	PageMatchKey string            `json:"pageMatchKey"`
	Type         string            `json:"type"`
	Size         int64             `json:"size"`
	Label        string            `json:"label"`
	RoutePath    string            `json:"route_path"`
	RouteParams  map[string]string `json:"route_params"`
	Children     *[]*WorkspaceNode `json:"children"`
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
//			"workspace_uri": "uri here",
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
	wskey := dataTableAction.Data[0]["key"]
	workspaceName := dataTableAction.WorkspaceName
	wsuri := dataTableAction.Data[0]["workspace_uri"]
	if wskey == nil || workspaceName == "" || wsuri == nil {
		httpStatus = http.StatusBadRequest
		err = errors.New("incomplete request")
		return
	}

	// Request type indicates the granularity of the result (file or object)
	requestType := dataTableAction.FromClauses[0].Table

	// Prepare the return object
	httpStatus = http.StatusOK
	resultData := make([]*WorkspaceNode, 0)
	root := os.Getenv("WORKSPACES_HOME") + "/" + workspaceName
	var workspaceNode *WorkspaceNode

	switch requestType {
	case "workspace_file_structure":
		// Data/test_data (.csv, .txt)
		// fmt.Println("** Visiting data/test_data:")
		workspaceNode, err = VisitDirWrapper(root, "data/test_data", "Unit Test Data", &[]string{".txt", ".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Data Model (.jr)
		// fmt.Println("** Visiting data_model:")
		workspaceNode, err = VisitDirWrapper(root, "data_model", "Data Model", &[]string{".jr", ".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Jets Rules (.jr, .jr.sql)
		// fmt.Println("** Visiting jet_rules:")
		workspaceNode, err = VisitDirWrapper(root, "jet_rules", "Jets Rules", &[]string{".jr", ".jr.sql"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Lookups (.jr)
		// fmt.Println("** Visiting lookups:")
		workspaceNode, err = VisitDirWrapper(root, "lookups", "Lookups", &[]string{".jr", ".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Process Configurations (workspace_init_db.sql)
		// fmt.Println("** Visiting process_config:")
		workspaceNode, err = VisitDirWrapper(root, "process_config", "Process Configuration", &[]string{"workspace_init_db.sql"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Process Sequences (.jr)
		// fmt.Println("** Visiting process_sequence:")
		workspaceNode, err = VisitDirWrapper(root, "process_sequence", "Process Sequences", &[]string{".jr"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// Reports (.sql, .json)
		// fmt.Println("** Visiting reports:")
		workspaceNode, err = VisitDirWrapper(root, "reports", "Reports", &[]string{".sql", ".json"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
		resultData = append(resultData, workspaceNode)

		// compile_workspace.sh
		resultData = append(resultData, &WorkspaceNode{
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
	default:
		httpStatus = http.StatusBadRequest
		err = errors.New("invalid workspace request type")
		return
	}

	var v []byte
	v, err = json.Marshal(WorkspaceStructure{
		Key:           wskey.(string),
		WorkspaceName: workspaceName,
		ResultType:    requestType,
		ResultData:    &resultData,
	})
	// v, err = json.MarshalIndent(WorkspaceStructure{
	// 	Key: wskey.(string),
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

func VisitDirWrapper(root, dir, dirLabel string, filters *[]string, workspaceName string) (*WorkspaceNode, error) {
	var children *[]*WorkspaceNode
	var err error
	children, err = visitDir(root, dir, dir, filters, workspaceName)
	if err != nil {
		return nil, err
	}

	for _, c := range *children {
		if c.Type == "dir" {
			c.Children, err = visitChildren(root+"/"+dir, dir+"/"+c.Label, c.Label, filters, workspaceName)
			if err != nil {
				return nil, err
			}
		}
	}

	results := &WorkspaceNode{
		Key:          dir,
		Type:         "section",
		PageMatchKey: dir,
		Label:        dirLabel,
		RoutePath:    "/workspace/:workspace_name/home",
		RouteParams: map[string]string{
			"workspace_name": workspaceName,
			"label":          dirLabel,
		},
		Children: children,
	}

	return results, nil
}

func visitChildren(root, relativeRoot, dir string, filters *[]string, workspaceName string) (*[]*WorkspaceNode, error) {
	var children *[]*WorkspaceNode
	var err error
	children, err = visitDir(root, relativeRoot, dir, filters, workspaceName)
	if err != nil {
		return nil, err
	}

	for _, c := range *children {
		if c.Type == "dir" {
			c.Children, err = visitChildren(root+"/"+dir, relativeRoot+"/"+c.Label, c.Label, filters, workspaceName)
			if err != nil {
				return nil, err
			}
		}
	}

	return children, nil
}

// Function that visit a directory path to collect the file structure
// This function returns the direct children of the directory
// root is workspace root path (full path)
// relativeRoot is file relative root with respect to workspace root (file path within workspace)
// relativeRoot includes dir as the last component of it
// Note: This function cannot be called recursively, otherwise it will interrupt WalDir
func visitDir(root, relativeRoot, dir string, filters *[]string, workspaceName string) (*[]*WorkspaceNode, error) {

	// fmt.Println("*visitDir called for dir:",dir)
	fileSystem := os.DirFS(fmt.Sprintf("%s/%s", root, dir))
	children := make([]*WorkspaceNode, 0)

	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
		// fmt.Println("*** WalkDir @",path, "err is",err)
		if err != nil {
			log.Printf("ERROR while walking workspace directory %q: %v", path, err)
			return err
		}

		if info.Name() == "." {
			return nil
		}

		if info.IsDir() {

			subdir := info.Name()
			// fmt.Println("visiting directory:", subdir)
			children = append(children, &WorkspaceNode{
				Key:          path,
				Type:         "dir",
				PageMatchKey: path,
				Label:        subdir,
				RouteParams: map[string]string{
					"workspace_name": workspaceName,
					"label":          subdir,
				},
			})
			return fs.SkipDir

		} else {

			filename := info.Name()
			keepEntry := false
			for i := range *filters {
				if strings.HasSuffix(filename, (*filters)[i]) {
					keepEntry = true
				}
			}
			if keepEntry {
				// fmt.Println("visiting file:", filename)
				fileInfo, err := info.Info()
				var size int64
				if err != nil {
					log.Println("while trying to get the file size:", err)
				} else {
					size = fileInfo.Size()
				}
				relativeFileName := url.QueryEscape(fmt.Sprintf("%s/%s", relativeRoot, filename))
				children = append(children, &WorkspaceNode{
					Key:          path,
					Type:         "file",
					PageMatchKey: relativeFileName,
					Label:        filename,
					Size:         size,
					RoutePath:    "/workspace/:workspace_name/home",
					RouteParams: map[string]string{
						"workspace_name": workspaceName,
						"file_name":      relativeFileName,
						"label":          filename,
					},
				})
			}
		}
		return nil
	})

	if err != nil {
		log.Println("while walking workspace dir:", err)
		return nil, err
	}
	return &children, nil
}

// GetWorkspaceFileContent --------------------------------------------------------------------------
// Function to get the workspace file content based on relative file name
// Read the file from the workspace on file system since it's already in sync with database
func (ctx *Context) GetWorkspaceFileContent(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	wsName := request["workspace_name"]
	wsFileName := request["file_name"]
	if wsName == nil || wsFileName == nil {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name or file_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	workspaceName := wsName.(string)
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Read file from local workspace
	var content []byte
	content, err = os.ReadFile(fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName, fileName))
	if err != nil {
		err = fmt.Errorf("failed to read local workspace file %s: %v", fileName, err)
		httpStatus = http.StatusBadRequest
		return
	}
	results = &map[string]interface{}{
		"file_name":    wsFileName,
		"file_content": string(content),
	}
	return
}

// SaveWorkspaceFileContent --------------------------------------------------------------------------
// Function to save the workspace file content in local workspace file system and in database
func (ctx *Context) SaveWorkspaceFileContent(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	wsName := request["workspace_name"]
	wsFileName := request["file_name"]
	wsFileContent := request["file_content"]
	if wsName == nil || wsFileName == nil || wsFileContent == nil {
		err = fmt.Errorf("SaveWorkspaceFileContent: missing workspace_name, file_content, or file_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	workspaceName := wsName.(string)
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Write file to local workspace
	data := []byte(wsFileContent.(string))
	path := fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName, fileName)
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		err = fmt.Errorf("failed to write local workspace file %s: %v", fileName, err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Write file and metadata to database
	var fileHd *os.File
	fileHd, err = os.Open(path)
	if err != nil {
		err = fmt.Errorf("(2) failed to open local workspace file %s: %v", fileName, err)
		httpStatus = http.StatusBadRequest
		return
	}
	defer fileHd.Close()
	p := strings.Index(fileName, "/")
	var contentType string
	if p > 0 {
		contentType = fileName[0:p]
	}
	if contentType == "" {
		err = fmt.Errorf("failed to find contentType in %s", fileName)
		httpStatus = http.StatusBadRequest
		return
	}
	fo := dbutils.FileDbObject{
		WorkspaceName: workspaceName,
		FileName:      fileName,
		ContentType:   contentType,
		Status:        dbutils.FO_Open,
		UserEmail:     "system",
	}
	n, err := fo.WriteObject(ctx.Dbpool, fileHd)
	if err != nil {
		err = fmt.Errorf("failed to save local workspace file %s in database: %v", fileName, err)
		httpStatus = http.StatusBadRequest
		return
	}
	fmt.Println("uploaded", fo.FileName, "size", n, "bytes to database")
	results = &map[string]interface{}{
		"file_name": wsFileName,
	}
	return
}

func stashPath() string {
	return fmt.Sprintf("%s/ws:stash", os.Getenv("WORKSPACES_HOME"))
}

// StashWorkspaceFiles --------------------------------------------------------------------------
// Function to copy all workspace files to a stash location
// The stash is used when deleting workspace changes to restore the file to original content
func StashWorkspaceFiles(workspaceName string) error {
	workspacePath := fmt.Sprintf("%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName)
	stashPath := stashPath()
	log.Printf("Stashing workspace files from %s to %s", workspacePath, stashPath)

	// make sure the stash directory exists
	var err error
	if err2 := os.Mkdir(stashPath, 0755); os.IsExist(err2) {
		log.Println("Workspace stash", stashPath, "exists")
	} else {
		log.Println("Workspace stash directory ", stashPath, "created")
	}

	// copy all files if targetDir does not exists
	if _, err2 := os.Stat(fmt.Sprintf("%s/%s", stashPath, workspaceName)); err2 != nil {
		log.Println("Stashing workspace files")
		targetDir := fmt.Sprintf("--target-directory=%s", stashPath)
		cmd := exec.Command("cp", "--recursive", "--no-dereference", targetDir, workspacePath)
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b
		err = cmd.Run()
		if err != nil {
			log.Printf("while executing cp to stash of the workspace files: %v", err)
		} else {
			log.Println("cp workspace files to stash output:")
		}
		b.WriteTo(os.Stdout)
		log.Println("============================")

		// Removing files that we don't want to be restaured
		targetDir = fmt.Sprintf("%s/%s", stashPath, workspaceName)
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/.git", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/.github", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/.gitignore", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/lookup.db", targetDir)).Run()
		exec.Command("rm", "--recursive", "--force", fmt.Sprintf("%s/workspace.db", targetDir)).Run()
	} else {
		log.Println("Workspace files already stashed, not overriting them")
	}

	return err
}

// Function to restore file from stash, it copy src file to dst
func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

// Restaure (copy dir recursively) srcDir to dstDir
func restaure(srcDir, dstDir string) error {
	targetDir := fmt.Sprintf("--target-directory=%s", dstDir)
	cmd := exec.Command("cp", "--recursive", "--no-dereference", targetDir, srcDir)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	if err != nil {
		log.Printf("while executing restaure from stash all the workspace files: %v", err)
	} else {
		log.Println("restaure all workspace files from stash output:")
	}
	b.WriteTo(os.Stdout)
	log.Println("============================")
	return err
}

// DeleteWorkspaceChanges --------------------------------------------------------------------------
// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
func (ctx *Context) DeleteWorkspaceChanges(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsName := request["workspace_name"]
		wsOid := request["oid"]
		wsFileName := request["file_name"]
		wsUserEmail := request["user_email"]
		wsKey := request["key"]
		if wsName == nil || wsOid == nil || wsFileName == nil || wsKey == nil || wsUserEmail == nil {
			err = fmt.Errorf("DeleteWorkspaceChanges: missing workspace_name, oid, key, user email, or file_name")
			fmt.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		fmt.Println("DeleteWorkspaceChanges: Deleting key", wsKey, "file name", wsFileName)
		stmt := fmt.Sprintf("SELECT lo_unlink(%s); DELETE FROM jetsapi.workspace_changes WHERE key = %s",
			wsOid.(string), wsKey.(string))
		fmt.Println("DELETE stmt:", stmt)
		_, err = ctx.Dbpool.Exec(context.Background(), stmt)
		if err != nil {
			log.Printf("While deleting row in workspace_changes table: %v", err)
			httpStatus = http.StatusBadRequest
			return
		}
		// restauring file from stash (if exists, do not report error if fails)
		stashPath := stashPath()
		source := fmt.Sprintf("%s/%s/%s", stashPath, wsName, wsFileName)
		destination := fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), wsName, wsFileName)
		log.Printf("Restauring file %s to %s", source, destination)
		if n, err2 := copy(source, destination); err2 != nil {
			log.Println("while restauring file:", err2)
		} else {
			log.Println("copied", n, "bytes")
		}
	}
	results = &map[string]interface{}{}
	return
}

// DeleteAllWorkspaceChanges --------------------------------------------------------------------------
// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
func (ctx *Context) DeleteAllWorkspaceChanges(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	wsName := request["workspace_name"]
	if wsName == nil {
		err = fmt.Errorf("DeleteAllWorkspaceChanges: missing workspace_name")
		fmt.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	fmt.Println("DeleteAllWorkspaceChanges: woarkspace_name", wsName)
	stmt := fmt.Sprintf(
		"SELECT lo_unlink(oid) FROM jetsapi.workspace_changes WHERE workspace_name = '%s'; DELETE FROM jetsapi.workspace_changes WHERE workspace_name = '%s'",
		wsName.(string), wsName.(string))
	fmt.Println("DELETE stmt:", stmt)
	_, err = ctx.Dbpool.Exec(context.Background(), stmt)
	if err != nil {
		log.Printf("While deleting row in workspace_changes table: %v", err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Restauring all workspace  files
	stashPath := stashPath()
	source := fmt.Sprintf("%s/%s", stashPath, wsName)
	log.Printf("Restauring all workspace files from %s", source)
	if err2 := restaure(source, os.Getenv("WORKSPACES_HOME")); err2 != nil {
		log.Println("while restauring all workspace files:", err2)
	}

	results = &map[string]interface{}{}
	return
}
