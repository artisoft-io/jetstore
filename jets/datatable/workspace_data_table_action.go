package datatable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
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

		case dataTableAction.FromClauses[0].Table == "compile_workspace":
			workspaceName := dataTableAction.Data[irow]["workspace_name"]
			fmt.Println("Compiling workspace", workspaceName)
			err = workspace.CompileWorkspace(ctx.Dbpool, workspaceName.(string), strconv.FormatInt(time.Now().Unix(), 10))
			if err != nil {
				log.Printf("While compiling workspace %s: %v", workspaceName, err)
				httpStatus = http.StatusBadRequest
				err = errors.New("error compiling workspace")
				return
			}
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

// This struct correspond to MenuEntry for the ui
type WorkspaceStructure struct {
	Key                string              `json:"key"`
	WorkspaceName      string              `json:"workspace_name"`
	ResultType         string              `json:"result_type"`
	ResultData         *[]*WorkspaceNode   `json:"result_data"`
}
type WorkspaceNode struct {
	Key           string                   `json:"key"`
	Type          string                   `json:"type"`
	Label         string                   `json:"label"`
	RoutePath     string                   `json:"route_path"`
	Children      *[]*WorkspaceNode        `json:"children"`
}
// WorkspaceQueryStructure ------------------------------------------------------
// Function to query the workspace structure, it returns a hierarchical structure
// modeled based on ui MenuEntry class.
// It uses a virtual table name to indicate the level of granularity of the structure
// dataTableAction.FromClauses[0].Table:
//		case "workspace_file_structure": structure based on files of the workspace
//		case "workspace_object_structure": structure based on object (rule, lookup, class, etc) of the workspace
// Initial implementation use workspace_file_structure
// Input dataTableAction.Data:
//      [
//      	{
//      		"key": "123",
//      		"workspace_name": "jets_ws",
//      		"workspace_uri": "uri here",
//      		"user_email": "email here"
//      	}
//      ]
// Output results:
//			{
//				"key": "123",
//				"workspace_name": "jets_ws",
//			  "result_type": "workspace_file_structure",
//				"result_data": [
//					{
//						"key": "a1",
//            "type": "dir",
//						"label": "Jet Rules",
//						"route_path": "/workspace/jets_ws/jetRules",
//						"children": [
//							{
//								"key": "a1.1",
//                "type": "dir",
//								"label": "folder name",
//								"children": [
//									{
//										"key": "a1.1.1",
//                    "type": "file",
//										"label": "mapping_rules.jr",
//										"route_path": "/workspace/jets_ws/wsFile/jet_rules%03mapping_rules.jr"
//									}
//								]
//							}		
//						]
//					}
//				]
//			}
func (ctx *Context) WorkspaceQueryStructure(dataTableAction *DataTableAction, token string) (results *[]byte, httpStatus int, err error) {
	// Validate the arguments
	if len(dataTableAction.Data) == 0 || len(dataTableAction.FromClauses) == 0 {
		httpStatus = http.StatusBadRequest
		err = errors.New("incomplete request")
		return
	}
	wskey := dataTableAction.Data[0]["key"]
	wsn := dataTableAction.Data[0]["workspace_name"]
	wsuri := dataTableAction.Data[0]["workspace_uri"]
	if wskey == nil || wsn == nil || wsuri == nil {
		httpStatus = http.StatusBadRequest
		err = errors.New("incomplete request")
		return
	}

	// Request type indicates the granularity of the result (file or object)
	requestType := dataTableAction.FromClauses[0].Table
	workspaceName := wsn.(string)

	// Prepare the return object
	httpStatus = http.StatusOK
	resultData := make([]*WorkspaceNode, 0)
	root := os.Getenv("WORKSPACES_HOME") + "/" + workspaceName
	var workspaceNode *WorkspaceNode

	switch requestType {
	case "workspace_file_structure":
		// Data Model (.jr)
		fmt.Println("** Visiting data_model:")
		workspaceNode, err = visitDirWrapper(root, "data_model", "Data Model", &[]string{".jr",".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:",err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return	
		}
		resultData = append(resultData, workspaceNode)

		// Jets Rules (.jr, .jr.sql)
		fmt.Println("** Visiting jet_rules:")
		workspaceNode, err = visitDirWrapper(root, "jet_rules", "Jets Rules", &[]string{".jr",".jr.sql"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:",err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return	
		}
		resultData = append(resultData, workspaceNode)

		// Lookups (.jr)
		fmt.Println("** Visiting lookups:")
		workspaceNode, err = visitDirWrapper(root, "lookups", "Lookups", &[]string{".jr",".csv"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:",err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return	
		}
		resultData = append(resultData, workspaceNode)

		// Process Configurations (workspace_init_db.sql)
		fmt.Println("** Visiting process_config:")
		workspaceNode, err = visitDirWrapper(root, "process_config", "Process Configuration", &[]string{"workspace_init_db.sql"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:",err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return	
		}
		resultData = append(resultData, workspaceNode)

		// Process Sequences (.jr)
		fmt.Println("** Visiting process_sequence:")
		workspaceNode, err = visitDirWrapper(root, "process_sequence", "Process Sequences", &[]string{".jr"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:",err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return	
		}
		resultData = append(resultData, workspaceNode)

		// Reports (.sql, .json)
		fmt.Println("** Visiting reports:")
		workspaceNode, err = visitDirWrapper(root, "reports", "Reports", &[]string{".sql", ".json"}, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:",err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return	
		}
		resultData = append(resultData, workspaceNode)

		// compile_workspace.sh
		resultData = append(resultData, &WorkspaceNode{
			Key: "compile_workspace",
			Label: "Compile Workspace Script",
			RoutePath: fmt.Sprintf("/workspace/%s/wsFile/%s", workspaceName, url.QueryEscape("compile_workspace.sh")),	
		})
	default:
		httpStatus = http.StatusBadRequest
		err = errors.New("invalid workspace request type")
		return
	}

	var v []byte
	v, err = json.Marshal(WorkspaceStructure{
		Key: wskey.(string),
		WorkspaceName: workspaceName,
		ResultType: requestType,
		ResultData: &resultData,
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

func visitDirWrapper(root, dir, dirLabel string, filters *[]string, workspaceName string) ( *WorkspaceNode, error) {
	var children *[]*WorkspaceNode
	var err error
	children, err = visitDir(root, dir, dir, filters, workspaceName)
	if err != nil {
		return	nil, err
	}

	for _,c := range *children {
		if c.Type == "dir" {
			c.Children, err = visitChildren(root+"/"+dir, dir+"/"+c.Label, c.Label, filters, workspaceName)
			if err != nil {
				return nil, err
			}
		}
	}

	results := &WorkspaceNode{
		Key: dir,
		Label: dirLabel,
		RoutePath: fmt.Sprintf("/workspace/%s/%s", workspaceName, dir),
		Children: children,
	}

	return results, nil
}

func visitChildren(root, relativeRoot, dir string, filters *[]string, workspaceName string) (*[]*WorkspaceNode, error) {
	var children *[]*WorkspaceNode
	var err error
	children, err = visitDir(root, relativeRoot, dir, filters, workspaceName)
	if err != nil {
		return	nil, err
	}

	for _,c := range *children {
		if c.Type == "dir" {
			c.Children, err = visitChildren(root+"/"+dir, relativeRoot+"/"+c.Label, c.Label, filters, workspaceName)
			if err != nil {
				return	nil, err
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
func visitDir(root, relativeRoot, dir string, filters *[]string, workspaceName string) ( *[]*WorkspaceNode, error) {

	fmt.Println("*visitDir called for dir:",dir)
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
			fmt.Println("visiting directory:", subdir)
			children = append(children, &WorkspaceNode{
				Key: path,
				Type: "dir",
				Label: subdir,
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
				fmt.Println("visiting file:", filename)
				children = append(children, &WorkspaceNode{
					Key: path,
					Type: "file",
					Label: filename,
					RoutePath: fmt.Sprintf("/workspace/%s/wsFile/%s", workspaceName, 
							url.QueryEscape(fmt.Sprintf("%s/%s", relativeRoot, filename))),
				})
			}
		}
		return nil
	})

	if err != nil {
		log.Println("while walking workspace dir:",err)
		return nil, err
	}
	return &children, nil
}