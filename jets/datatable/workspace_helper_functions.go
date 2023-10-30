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
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Commit changes in local workspace and push to repository:
//	- Commit and Push to repository
//  NOTE:
//	- Delete workspace overrides
//	  (except for workspace.db, lookup.db, and reports.tgz)
//	  must be done manually
//	- Compile workspace must be done manually
func commitWorkspaceAction(dbpool *pgxpool.Pool,  gitProfile *user.GitProfile, dataTableAction *DataTableAction)  {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName
		wsUri := getWorkspaceUri(dataTableAction, irow)
		wsCM := dataTableAction.Data[irow]["git.commit.message"]
		var wsCommitMessage string
		if(wsCM != nil) {
			// escape singe ' with ''
			wsCommitMessage = strings.ReplaceAll(wsCM.(string), "'", "''")
		}
		workspaceGit := git.NewWorkspaceGit(workspaceName, wsUri)
		var buf strings.Builder
		
		// Commit and push workspace changes and update workspace_registry table
		gitLog, err = workspaceGit.CommitLocalWorkspace(gitProfile,	wsCommitMessage)
		buf.WriteString(gitLog)
		buf.WriteString("\n")
		if err != nil {
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

		// Executing the InserRow Stmt
		_, err = dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	}
}

// Pull workspace changes in local repository:
//	- Pull changes from orign repo
//	- Update the file stash with pulled version
//  - Apply workspace overrides
//  NOTE:
//	- Compile workspace must be done manually
func pullWorkspaceAction(dbpool *pgxpool.Pool,  gitProfile *user.GitProfile, dataTableAction *DataTableAction)  {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName
		wsUri := getWorkspaceUri(dataTableAction, irow)

		workspaceGit := git.NewWorkspaceGit(workspaceName, wsUri)
		var buf strings.Builder

		// Pull changes from repository
		gitLog, err = workspaceGit.PullRemoteWorkspace(
			gitProfile.GitHandle,
			gitProfile.GitToken,
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

		// Apply workspace overrides from database, skipping compiled files
		err = workspace.SyncWorkspaceFiles(dbpool, workspaceName, dbutils.FO_Open, "", true, true)
		if err != nil {
			//* TODO Log to a new workspace error table to report in UI
			log.Println("Error while synching workspace file from database:", err, "(ignored)")
			err = nil
		}

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
