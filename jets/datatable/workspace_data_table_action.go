package datatable

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"log"
	"net/http"
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
