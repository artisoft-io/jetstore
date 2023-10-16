package datatable

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/git"
	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Commit changes in local workspace and push to repository:
//	- Compile workspace
//	- Commit and Push to repository
//	- Delete changes in db (except for workspace.db and lookup.db)
func commitWorkspaceAction(dbpool *pgxpool.Pool, dataTableAction *DataTableAction)  {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName
		wsUri := getWorkspaceUri(dataTableAction, irow)
		gitUser := dataTableAction.Data[irow]["git.user"]
		gitToken := dataTableAction.Data[irow]["git.token"]
		wsCM := dataTableAction.Data[irow]["git.commit.message"]
		var wsCommitMessage string
		if(wsCM != nil) {
			wsCommitMessage = wsCM.(string)
		}
		workspaceGit := git.NewWorkspaceGit(workspaceName, wsUri)
		var buf strings.Builder

		// Compile workspace
		gitLog, err = workspace.CompileWorkspace(dbpool, workspaceName, strconv.FormatInt(time.Now().Unix(), 10))
		buf.WriteString(gitLog)
		buf.WriteString("\n")
		if err != nil {
			status = "error"
			goto setCommitGitLog
		}
		
		// Commit and push workspace changes and update workspace_registry table
		gitLog, err = workspaceGit.CommitLocalWorkspace(
			gitUser.(string),
			gitToken.(string),
			wsCommitMessage,
		)
		buf.WriteString(gitLog)
		buf.WriteString("\n")
		if err != nil {
			status = "error"
			goto setCommitGitLog
		}

		// Delete workspace overrides (except for workspace.db and lookup.db)
		// Note, do not restaure files from stash
		err = wsfile.DeleteAllFileChanges(dbpool, workspaceName, false, true)
		if err != nil {
			buf.WriteString(fmt.Sprintf("Error while deleting all file changes from db: %v\n", err))
			status = "error"
			goto setCommitGitLog
		}

		setCommitGitLog:
		dataTableAction.Data[irow]["last_git_log"] = buf.String()
		dataTableAction.Data[irow]["status"] = status

		// Perform the Insert Rows
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
		// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
		// Executing the InserRow Stmt
		_, err = dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	}
}

// Pull workspace changes, update workspace_registry table and delete overrides in workspace_changes
func pullWorkspaceAction(dbpool *pgxpool.Pool, dataTableAction *DataTableAction)  {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName
		wsUri := getWorkspaceUri(dataTableAction, irow)
		gitUser := dataTableAction.Data[irow]["git.user"]
		gitToken := dataTableAction.Data[irow]["git.token"]

		workspaceGit := git.NewWorkspaceGit(workspaceName, wsUri)
		var buf strings.Builder

		// Pull changes from repository
		gitLog, err = workspaceGit.PullRemoteWorkspace(
			gitUser.(string),
			gitToken.(string),
		)
		buf.WriteString(gitLog)
		buf.WriteString("\n")
		if err != nil {
			status = "error"
			goto setPullGitLog
		}

		// Clear existing stash
		err = wsfile.ClearStash(workspaceName)
		if err != nil {
			buf.WriteString(fmt.Sprintf("Error while clearing stash for workspace %s, ignored\n", workspaceName))
			log.Printf("Error while clearing stash for workspace %s, ignored", workspaceName)
			err = nil
		}
		// Create new stash corresponding to this pulled workspace
		err = wsfile.StashFiles(workspaceName)
		if err != nil {
			buf.WriteString(fmt.Sprintf("Error while stashing workspace %s, ignored\n", workspaceName))
			log.Printf("Error while stashing workspace %s, ignored", workspaceName)
			err = nil
		}
		// Delete all workspace overrides
		// Note, do not restaure files from stash
		err = wsfile.DeleteAllFileChanges(dbpool, workspaceName, false, false)
		if err != nil {
			buf.WriteString(fmt.Sprintf("Error while deleting all file changes from db: %v\n", err))
			status = "error"
			goto setPullGitLog
		}
		
		// Compile workspace
		gitLog, err = workspace.CompileWorkspace(dbpool, workspaceName, strconv.FormatInt(time.Now().Unix(), 10))
		if err != nil {
			status = "error"
		}
		buf.WriteString(gitLog)
		buf.WriteString("\n")

		setPullGitLog:
		dataTableAction.Data[irow]["last_git_log"] = buf.String()
		dataTableAction.Data[irow]["status"] = status

		// Perform the Insert Rows
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		_, err = dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	}
}


// Compile workspace changes, update workspace_registry table and delete overrides in workspace_changes
func compileWorkspaceAction(dbpool *pgxpool.Pool, dataTableAction *DataTableAction)  {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName
	
		// Compile workspace
		fmt.Println("Compiling workspace", workspaceName)
		gitLog, err = workspace.CompileWorkspace(dbpool, workspaceName, strconv.FormatInt(time.Now().Unix(), 10))
		if err != nil {
			status = "error"
		}
		dataTableAction.Data[irow]["last_git_log"] = gitLog
		dataTableAction.Data[irow]["status"] = status

		// Perform the Insert Rows
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		_, err = dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	}
}

// Execute pipeline in unit test mode
func unitTestWorkspaceAction(ctx *Context, dataTableAction *DataTableAction, token string)  {

	dataTableAction.Action = "insert_rows"
	dataTableAction.FromClauses[0].Table = "pipeline_execution_status"
	ctx.DevMode = true
	results, _, err := ctx.InsertRows(dataTableAction, token)

	dataTableAction.Data[0]["last_git_log"] = (*results)["log"]
	dataTableAction.Data[0]["status"] = ""
	if err != nil {
		dataTableAction.Data[0]["status"] = "error"
	}

	// Perform the Insert Rows
	sqlStmt := sqlInsertStmts["unit_test"]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for jcol, colKey := range sqlStmt.ColumnKeys {
		row[jcol] = dataTableAction.Data[0][colKey]
	}

	_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
	if err != nil {
		log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
	}
}

// Load workspace config
func loadWorkspaceConfigAction(ctx *Context, dataTableAction *DataTableAction)  {

	// using update_db script
	log.Printf("Loading Workspace Config for workspace: %s\n", dataTableAction.WorkspaceName)
	serverArgs := []string{ "-initWorkspaceDb", "-migrateDb" }
	if ctx.UsingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}
	results, err := RunUpdateDb(dataTableAction.WorkspaceName, &serverArgs)

	dataTableAction.Data[0]["last_git_log"] = results
	dataTableAction.Data[0]["status"] = ""
	if err != nil {
		dataTableAction.Data[0]["status"] = "error"
	}

	// Perform the Insert Rows
	sqlStmt := sqlInsertStmts["unit_test"]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for jcol, colKey := range sqlStmt.ColumnKeys {
		row[jcol] = dataTableAction.Data[0][colKey]
	}

	_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
	if err != nil {
		log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
	}
}

// Run update_db
func RunUpdateDb(workspaceName string, serverArgs *[]string) (string, error) {
	log.Printf("Run update_db: %s", *serverArgs)
	cmd := exec.Command("/usr/local/bin/update_db", *serverArgs...)
	cmd.Env = append(os.Environ(),
	fmt.Sprintf("WORKSPACE=%s", workspaceName),
	)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	result := buf.String()
	if err != nil {
		log.Printf("while executing update_db command '%v': %v", serverArgs, err)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println(result)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("UPDATE_DB CAPTURED OUTPUT END")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return result, err
	}
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT BEGIN")
	log.Println("============================")
	log.Println(result)
	log.Println("============================")
	log.Println("UPDATE_DB CAPTURED OUTPUT END")
	log.Println("============================")
	return result, nil
}
