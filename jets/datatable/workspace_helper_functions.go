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

// Pull workspace changes in local repository:
//   - Pull changes by merging WorkspaceBranch into current branch
//   - Update the file stash with pulled version
//   - Apply workspace overrides (except for compiled files)
//     NOTE:
//   - Compile workspace must be done manually
func pullWorkspaceAction(dbpool *pgxpool.Pool, irow int, gitProfile *user.GitProfile, dataTableAction *DataTableAction) (string, error) {
	var err error
	var gitLog string
	workspaceName := dataTableAction.WorkspaceName
	wsUri := getWorkspaceUri(dataTableAction, irow)
	workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
		WorkspaceName:   dataTableAction.WorkspaceName,
		WorkspaceUri:    wsUri,
		WorkspaceBranch: dataTableAction.WorkspaceBranch,
		FeatureBranch:   dataTableAction.FeatureBranch,
	})
	var buf strings.Builder

	// Pull changes from repository
	gitLog, err = workspaceGit.PullRemoteWorkspace(
		gitProfile.GitHandle,
		gitProfile.GitToken,
	)
	buf.WriteString(gitLog)
	buf.WriteString("\n")
	if err != nil {
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
		buf.WriteString(fmt.Sprintf("Error while synching workspace file from database: %v (ignored)\n", err))
		log.Println("Error while synching workspace file from database:", err, "(ignored)")
		err = nil
	}

setPullGitLog:
	return buf.String(), err
}

// Compile workspace changes, update workspace_registry table and delete overrides in workspace_changes
func compileWorkspaceAction(ctx *Context, dataTableAction *DataTableAction) {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName

		// Compile workspace
		fmt.Println("Compiling workspace", workspaceName)
		gitLog, err = workspace.CompileWorkspace(ctx.Dbpool, workspaceName, strconv.FormatInt(time.Now().Unix(), 10))
		if err != nil {
			status = "error"
		}
		lastLog := dataTableAction.Data[irow]["last_git_log"]
		if lastLog != nil {
			dataTableAction.Data[irow]["last_git_log"] = fmt.Sprintf("%v\n%s", lastLog, gitLog)
		} else {
			dataTableAction.Data[irow]["last_git_log"] = gitLog
		}

		// Load base workspace config
		if status != "error" {
			// Load the base workspace config in case domain schema or process config changed
			log.Printf("Loading base Workspace Config for workspace: %s\n", dataTableAction.WorkspaceName)
			serverArgs := []string{"-initBaseWorkspaceDb"}
			if ctx.UsingSshTunnel {
				serverArgs = append(serverArgs, "-usingSshTunnel")
			}
			gitLog, err = RunUpdateDb(dataTableAction.WorkspaceName, &serverArgs)
			if err != nil {
				status = "error"
			}
			lastLog = dataTableAction.Data[irow]["last_git_log"]
			if lastLog != nil {
				dataTableAction.Data[irow]["last_git_log"] = fmt.Sprintf("%v\n%s", lastLog, gitLog)
			} else {
				dataTableAction.Data[irow]["last_git_log"] = gitLog
			}
		}

		// Check if load client-specific config
		if status != "error" {
			otherActions := dataTableAction.Data[irow]["otherWorkspaceActionOptions"]
			if(otherActions != nil) {
				l := otherActions.([]interface{})
				for i := range l {
					if l[i] != nil && (l[i] == "wpLoadClientConfgOption" || l[i] == "wpLoadSelectedClientConfgOption") {
						status = "Loading client config in progress"
						go loadWorkspaceConfigAction(ctx, dataTableAction)
					}
				}
			}
		}
		dataTableAction.Data[irow]["status"] = status

		// Perform the Insert Rows
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}
		_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
		}
	}
}

// LoadWorkspaceConfigAction to load client config into JetStore db
// Update the workspace_registry table with status
func loadWorkspaceConfigAction(ctx *Context, dataTableAction *DataTableAction) {
	// using update_db script
	log.Printf("Loading Workspace Config for workspace: %s\n", dataTableAction.WorkspaceName)
	serverArgs := make([]string, 0)
	if ctx.UsingSshTunnel {
		serverArgs = append(serverArgs, "-usingSshTunnel")
	}

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	irow := 0
	var gitLog string
	status := ""
	// update_db script
	clients := dataTableAction.Data[irow]["updateDbClients"]
	if clients != nil {
		// Load specific clients
		serverArgs = append(serverArgs, "-clients")
		serverArgs = append(serverArgs, clients.(string))
	} else {
		// Load all clients
		serverArgs = append(serverArgs, "-initWorkspaceDb")
	}
	gitLog, err = RunUpdateDb(dataTableAction.WorkspaceName, &serverArgs)
	if err != nil {
		status = "error"
	}
	lastLog := dataTableAction.Data[irow]["last_git_log"]
	if lastLog != nil {
		dataTableAction.Data[irow]["last_git_log"] = fmt.Sprintf("%v\n%s", lastLog, gitLog)
	} else {
		dataTableAction.Data[irow]["last_git_log"] = gitLog
	}
	dataTableAction.Data[irow]["status"] = status

	// Perform the Insert Rows
	for jcol, colKey := range sqlStmt.ColumnKeys {
		row[jcol] = dataTableAction.Data[irow][colKey]
	}
	_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
	if err != nil {
		log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
	}
}

// Execute pipeline in unit test mode
func UnitTestWorkspaceAction(ctx *Context, dataTableAction *DataTableAction, token string) {

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

// Run update_db - function used by apiserver and server
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
