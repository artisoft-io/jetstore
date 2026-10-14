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
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pullWorkspace and compileWorkspace are the pull and the compile the
// workspace_registry actions run: variables so that a test of the requests can
// stand in for git and for the compiler, as lookupRulesUnitTestCases stands in
// for workspace.db.
var (
	pullWorkspace    = pullWorkspaceAction
	compileWorkspace = compileWorkspaceAction
)

// syncWorkspaceOverrides is workspace.SyncWorkspaceFiles, a variable so that a test can
// stand in for the database.
var syncWorkspaceOverrides = workspace.SyncWorkspaceFiles

// syncOverridesToLocal writes the database's workspace overrides over the apiserver's
// local workspace, compiled files excepted, before an action that reads the local tree.
// It returns a line for the action's log, empty when the sync succeeded.
//
// **The local tree is not the latest copy of an override.** An IDE save writes both the
// database and the local file (`SaveContent`, jets/datatable/wsfile/file_content.go),
// but a report with updateLookupTables runs in its own task: it writes the lookup it
// computed to that task's workspace and to jetsapi.workspace_changes, and compiles
// there (`SyncS3Files`, jets/awsi/files.go, called from
// jets/run_reports/delegate/run_reports.go). The apiserver's copy catches up only at
// its next start (`checkWorkspaceVersion`). Until then a compile from the UI compiled
// the old file, and a commit from the UI pushed it and then deleted every override,
// the fresh one included. On a deployment on 2026-10-14 that committed September's
// lookup and discarded October's, which then existed only in S3 and in the compiled
// lookup.db.
//
// The pull already does this after merging (pullWorkspaceAction, above); the compile
// and the commit now do it first.
func syncOverridesToLocal(dbpool *pgxpool.Pool, workspaceName string) string {
	if _, err := syncWorkspaceOverrides(dbpool, workspaceName, "", true, true); err != nil {
		log.Println("Error while synching workspace overrides from database:", err)
		return fmt.Sprintf("Error while synching workspace overrides from database: %v\n", err)
	}
	return ""
}

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

	// Re-stash the pulled tree, so that "delete my changes" has a pristine
	// baseline to restore from.
	//
	// **Skipped when git is off, and this is the one place in that mode where
	// running the ordinary path would destroy something.** The pair is a clear
	// followed by a re-stash, and the clear is what does the damage: `StashFiles`
	// on its own refuses to overwrite an existing stash
	// (`StashFiles`, `jets/datatable/wsfile/file_stash.go:21`, the
	// already-stashed branch), so without the `ClearStash` above it the sequence
	// is inert. With it, the stash is re-taken from the tree as it stands now.
	//
	// With a repository that is correct: the pull has just put the tree at the
	// remote's content, so a snapshot of it *is* the pristine baseline. With git
	// off there was no pull, so the tree still carries whatever the database
	// overrides last wrote onto it -- and re-stashing captures the user's own
	// edits as the thing to restore *to*. Nothing fails, and every later revert
	// silently returns the edited file. The pristine copy that was taken at
	// startup from the image (`checkWorkspaceVersion`,
	// `jets/apiserver/server.go`) would already have been thrown away by the
	// clear.
	//
	// So the honest behaviour with no repository is to leave the stash alone.
	// The startup snapshot stays pristine, revert keeps working, and the two
	// steps this skips had nothing to contribute without a pull in front of them.
	if git.NoGitAccess() {
		buf.WriteString(git.NoGitAccessNotice)
		buf.WriteString("\nThe workspace file stash is left as it is.\n")
		log.Printf("pullWorkspaceAction: %s Stash untouched for workspace %s.",
			git.NoGitAccessNotice, workspaceName)
	} else {
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
	}

	// Apply workspace overrides from database, skipping compiled files
	_, err = workspace.SyncWorkspaceFiles(dbpool, workspaceName, "", true, true)
	if err != nil {
		buf.WriteString(fmt.Sprintf("Error while synching workspace file from database: %v (ignored)\n", err))
		log.Println("Error while synching workspace file from database:", err, "(ignored)")
		err = nil
	}

setPullGitLog:
	return buf.String(), err
}

// Compile workspace changes, update workspace_registry table and delete overrides in workspace_changes
func compileWorkspaceAction(ctx *DataTableContext, dataTableAction *DataTableAction) {

	var err error
	sqlStmt := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		var gitLog string
		status := ""
		workspaceName := dataTableAction.WorkspaceName

		// Bring the local tree up to the database's overrides first, so a lookup a
		// refresh wrote since the last start is the one compiled (syncOverridesToLocal).
		// A failed sync is reported and the compile goes ahead: it leaves the overrides
		// in place, so compiling again after a restart repairs it.
		syncLog := syncOverridesToLocal(ctx.Dbpool, workspaceName)

		// Compile workspace
		log.Println("Compiling workspace", workspaceName)
		gitLog, err = workspace.CompileWorkspace(ctx.Dbpool, workspaceName, strconv.FormatInt(time.Now().Unix(), 10))
		gitLog = syncLog + gitLog
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
			if otherActions != nil {
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
func loadWorkspaceConfigAction(ctx *DataTableContext, dataTableAction *DataTableAction) {
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

// Run update_db - function used by apiserver and server
func RunUpdateDb(workspaceName string, serverArgs *[]string) (string, error) {
	// Sanitize the arguments to prevent injection of options/flags
	*serverArgs = utils.SanitizeArgs(*serverArgs)
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
