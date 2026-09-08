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
	// "github.com/jackc/pgx/v5/pgxpool"
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
func (ctx *DataTableContext) WorkspaceInsertRows(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	httpStatus = http.StatusOK
	returnedKey := make([]int, len(dataTableAction.Data))
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		return nil, http.StatusBadRequest, errors.New("error: unknown table")
	}
	userProfile, err2 := ctx.VerifyUserPermission(sqlStmt, token)
	if err2 != nil {
		httpStatus, err = RefusalFor(err2)
		return
	}
	var gitProfile user.GitProfile
	gitProfile, gitProfileErr := user.GetGitProfile(ctx.Dbpool, userProfile.Email)

	row := make([]any, len(sqlStmt.ColumnKeys))
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
			// # Why the git-off branch is here and not only in the git package
			//
			// The two refusals below run *before* `git.InitWorkspaceGit` is reached, so
			// the switch that lives inside that package cannot be seen from this side of
			// the call. Left alone, a deployment with JETS_NO_GIT_ACCESS set would refuse
			// this request with 400 and "missing git information" -- an accurate sentence
			// about a requirement the deployment no longer has -- and would never reach
			// the code that knows the requirement is gone. That is the whole reason the
			// change is two-layered rather than one function in one package.
			//
			// **A branch and not a deletion.** Both refusals are right when git is on: a
			// deployment with a repository cannot clone, switch a branch or push without
			// a uri and a git identity, and the 400 is what says so. Nothing about a
			// site with no source-control host makes that guard less true for the sites
			// that have one.
			//
			// **The notice falls through to the SQL update rather than being returned.**
			// Each of these pseudo-tables is an UPDATE on workspace_registry that writes
			// last_git_log (jets/datatable/sql_stmts.go:236 onward), and that column is
			// what the registry screen's "View Last Log" dialog reads. Reporting the
			// skip through the column the log always travelled by means the UI is told
			// the usual thing, which happens to say that nothing was done -- no new
			// response shape, and no UI change owed by this phase.
			if git.NoGitAccess() {
				// Logged once per skipped action rather than once per git command that
				// would have run, so pressing a button produces one line and not five.
				log.Println(git.NoGitAccessNotice)
				gitLog = git.NoGitAccessNotice
			} else {
				wsUri := getWorkspaceUri(dataTableAction, irow)
				if gitProfileErr != nil {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
				}
				gitUser := gitProfile.Name
				gitToken := gitProfile.GitToken
				gitUserName := gitProfile.GitHandle
				gitUserEmail := gitProfile.Email
				wsPN := dataTableAction.Data[irow]["previous.workspace_name"]
				if wsUri == "" || gitUser == "" || gitToken == "" ||
					gitUserName == "" || gitUserEmail == "" {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for update workspace_registry, missing git information")
				}
				var wsPreviousName string
				if wsPN != nil {
					wsPreviousName = wsPN.(string)
				}

				workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
					WorkspaceName:   dataTableAction.WorkspaceName,
					WorkspaceUri:    wsUri,
					WorkspaceBranch: dataTableAction.WorkspaceBranch,
					FeatureBranch:   dataTableAction.FeatureBranch,
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
			}
			// status is left as the success path leaves it -- the empty string -- so the
			// registry screen computes this row's status from GetStatus on the next read
			// rather than from a value invented here.
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = status

		case dataTableAction.FromClauses[0].Table == "commit_workspace":
			// Validating request
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for commit_workspace, missing workspace_name")
			}
			// # Why the delete is skipped as well as the commit, and why that is the
			// # part of this change worth reading twice
			//
			// On the success path this case does two things that look like one: it
			// commits and pushes, and it then drops the workspace's rows from
			// jetsapi.workspace_changes (DeleteAllFileChanges below, called with
			// restoreFromStash false). With a repository that deletion is safe because
			// it is redundant -- the edits it removes have just been pushed, so the
			// remote holds them and a later pull brings them back into the tree.
			//
			// **With no repository those rows are the only copy of the user's edits.**
			// The container's workspace tree is re-copied from the image whenever the
			// task starts (cbooter's cp -r of WORKSPACES_REPO into WORKSPACES_HOME),
			// and it is the override rows that are re-applied over it -- so a workspace
			// edit survives a task rotation because it is in the database, and for no
			// other reason. Skipping the push while still running the delete would
			// therefore destroy the user's work at the next rotation and report success
			// in the same breath, with nothing on the screen or in the log to say a
			// deletion had happened. That is the specific bug this branch exists to
			// avoid; it is not a hypothetical ordering worry.
			//
			// The requirement's word for the skipped git operations is *silently*, and
			// the asymmetry is deliberate: silence about a push that did not happen is
			// honest, because the notice below says so and nothing was lost. Silence
			// about a deletion is not.
			if git.NoGitAccess() {
				log.Println(git.NoGitAccessNotice)
				gitLog = git.NoGitAccessNotice
			} else {
				wsUri := getWorkspaceUri(dataTableAction, irow)
				if gitProfileErr != nil {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
				}
				gitUser := gitProfile.Name
				gitToken := gitProfile.GitToken
				if wsUri == "" || gitUser == "" || gitToken == "" {
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
				if wsCM != nil {
					wsCommitMessage = wsCM.(string)
				}
				workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
					WorkspaceName:   dataTableAction.WorkspaceName,
					WorkspaceUri:    wsUri,
					WorkspaceBranch: dataTableAction.WorkspaceBranch,
					FeatureBranch:   dataTableAction.FeatureBranch,
				})
				var buf strings.Builder
				// Commit and push workspace changes and update workspace_registry table
				gitLog, err = workspaceGit.CommitLocalWorkspace(&gitProfile, wsCommitMessage)
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
				gitLog = buf.String()
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = status

		case dataTableAction.FromClauses[0].Table == "git_command_workspace":
			// Execute git commands in workspace
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_command_workspace, missing workspace_name")
			}
			// Git off: the same branch as workspace_registry above, and the argument for
			// it is the one written out there. The refusal skipped here also covers a
			// missing git.command, which costs nothing to skip alongside the uri --
			// there is no command to run either way, and refusing on the shape of a
			// request that will not be executed tells the user about the wrong problem.
			if git.NoGitAccess() {
				log.Println(git.NoGitAccessNotice)
				gitLog = git.NoGitAccessNotice
			} else {
				wsUri := getWorkspaceUri(dataTableAction, irow)
				gitCommand := dataTableAction.Data[irow]["git.command"]
				if wsUri == "" || gitCommand == nil {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_command_workspace, missing git information")
				}
				workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
					WorkspaceName:   dataTableAction.WorkspaceName,
					WorkspaceUri:    wsUri,
					WorkspaceBranch: dataTableAction.WorkspaceBranch,
					FeatureBranch:   dataTableAction.FeatureBranch,
				})
				gitLog, err = workspaceGit.GitCommandWorkspace(gitCommand.(string))
				if err != nil {
					log.Printf("Error while git status workspace: %s\n", gitLog)
					httpStatus = http.StatusBadRequest
				}
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = ""

		case dataTableAction.FromClauses[0].Table == "git_status_workspace":
			// Execute git status commands in workspace
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_status_workspace, missing workspace_name")
			}
			// Git off: the branch of workspace_registry above. The registry screen leaves
			// this button enabled at every status, so it is a button a user in a no-git
			// deployment can press; answering it with the notice is what makes pressing
			// it informative rather than a 400 about a uri the deployment does not need.
			if git.NoGitAccess() {
				log.Println(git.NoGitAccessNotice)
				gitLog = git.NoGitAccessNotice
			} else {
				wsUri := getWorkspaceUri(dataTableAction, irow)
				if wsUri == "" {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for git_status_workspace, missing git information")
				}
				workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
					WorkspaceName:   dataTableAction.WorkspaceName,
					WorkspaceUri:    wsUri,
					WorkspaceBranch: dataTableAction.WorkspaceBranch,
					FeatureBranch:   dataTableAction.FeatureBranch,
				})
				gitLog, err = workspaceGit.GitCommandWorkspace("git status")
				if err != nil {
					log.Printf("Error while git status in workspace: %s\n", gitLog)
					httpStatus = http.StatusBadRequest
				}
			}
			dataTableAction.Data[irow]["last_git_log"] = gitLog
			dataTableAction.Data[irow]["status"] = ""

		case dataTableAction.FromClauses[0].Table == "push_only_workspace":
			// Push only workspace
			if dataTableAction.WorkspaceName == "" {
				return nil, http.StatusBadRequest, fmt.Errorf("invalid request for push_only_workspace, missing workspace_name")
			}
			// Git off: the branch of workspace_registry above. This case has nothing to
			// do but report, since a push with no remote is the one operation here that
			// has no non-git half -- unlike commit_workspace and pull_workspace, which
			// each carry work that is business logic rather than source control.
			if git.NoGitAccess() {
				log.Println(git.NoGitAccessNotice)
				gitLog = git.NoGitAccessNotice
			} else {
				wsUri := getWorkspaceUri(dataTableAction, irow)
				if gitProfileErr != nil {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
				}
				gitUser := gitProfile.Name
				gitToken := gitProfile.GitToken
				if wsUri == "" || gitUser == "" || gitToken == "" {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for push_only_workspace, missing git information")
				}
				workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
					WorkspaceName:   dataTableAction.WorkspaceName,
					WorkspaceUri:    wsUri,
					WorkspaceBranch: dataTableAction.WorkspaceBranch,
					FeatureBranch:   dataTableAction.FeatureBranch,
				})
				gitLog, err = workspaceGit.PushOnlyWorkspace(gitUser, gitToken)
				if err != nil {
					log.Printf("Error while push (only) workspace: %s\n", gitLog)
					httpStatus = http.StatusBadRequest
					status = "error"
				}
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
			// # Git off: the refusals go and the action stays, which is not the shape of
			// # the five cases above
			//
			// pullWorkspaceAction does four things and only the first is git: it pulls,
			// then clears the stash, re-stashes the pulled tree, and re-applies the
			// database overrides through workspace.SyncWorkspaceFiles. The last three
			// are the mechanism by which a workspace edit held in workspace_changes
			// reaches the file system, which is precisely what a deployment with no
			// repository needs this button for -- and the compile / load-client-config
			// options below are the other half of the same journey. So this case skips
			// its refusals and calls the action; it does not skip the action.
			//
			// **The pull inside it needs nothing from here**, and that is deliberate
			// rather than an omission. PullRemoteWorkspace returns the notice with no
			// error when git is off, so pullWorkspaceAction's `goto` past the remaining
			// three steps is not taken and they run as they do today. Doing the skip at
			// this call site instead would mean either duplicating those three steps or
			// editing workspace_helper_functions.go to teach it a second entry point,
			// and the git package already owns the decision (see
			// jets/datatable/git/no_git_access.go for why it lives there).
			//
			// gitProfile may be a zero value on this path, since the gitProfileErr
			// refusal is one of the two skipped; the handle and token it carries are
			// passed to a pull that will not run.
			if !git.NoGitAccess() {
				wsUri := getWorkspaceUri(dataTableAction, irow)
				if gitProfileErr != nil {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid git profile, cannot obtain git token")
				}
				gitUser := gitProfile.Name
				gitToken := gitProfile.GitToken
				if wsUri == "" || gitUser == "" || gitToken == "" {
					return nil, http.StatusBadRequest, fmt.Errorf("invalid request for pull_workspace, missing git information")
				}
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
				if otherActions != nil {
					l := otherActions.([]any)
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
				WorkspaceName:   dataTableAction.WorkspaceName,
				WorkspaceUri:    "",
				WorkspaceBranch: dataTableAction.WorkspaceBranch,
				FeatureBranch:   dataTableAction.FeatureBranch,
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

		// log.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
		// log.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
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

	case dataTableAction.FromClauses[0].Table == "load_workspace_config":
		// Load workspace config
		loadWorkspaceConfigAction(ctx, dataTableAction)

	}
returnResults:
	results = &map[string]any{
		"returned_keys": &returnedKey,
	}
	return
}

// DoWorkspaceReadAction ------------------------------------------------------
func (ctx *DataTableContext) DoWorkspaceReadAction(dataTableAction *DataTableAction, token string) (*map[string]any, int, error) {

	// Replace table schema with value $SCHEMA with the workspace_name
	//* NOTE: Reading directly from sqlite, no schema needed (set $SCHEMA to empty)
	for i := range dataTableAction.FromClauses {
		if dataTableAction.FromClauses[i].Schema == "$SCHEMA" {
			dataTableAction.FromClauses[i].Schema = ""
		}
	}
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}

	// to package up the result
	results := make(map[string]any)
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
	var resultRows *[][]any
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
				log.Println("Oops expecting workspace_name, workspace_uri, workspace_branch, feature_branch and status columns")
			} else {
				// Get the status from git command
				for irow := range *resultRows {
					if (*resultRows)[irow][statusPos] == "" {
						workspaceGit := git.InitWorkspaceGit(&git.WorkspaceGit{
							WorkspaceName:   (*resultRows)[irow][workspaceNamePos].(string),
							WorkspaceUri:    (*resultRows)[irow][workspaceUriPos].(string),
							WorkspaceBranch: (*resultRows)[irow][workspaceBranchPos].(string),
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
func (ctx *DataTableContext) WorkspaceQueryStructure(dataTableAction *DataTableAction, token string) (results *[]byte, httpStatus int, err error) {
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
		httpStatus, err = RefusalFor(err2)
		return
	}

	// Request type indicates the granularity of the result (file or object)
	requestType := dataTableAction.FromClauses[0].Table

	// Prepare the return object
	httpStatus = http.StatusOK
	var resultData []*wsfile.WorkspaceNode
	root := os.Getenv("WORKSPACES_HOME") + "/" + workspaceName

	switch requestType {
	case "workspace_file_structure":
		// The section list, the file-suffix filters and — the part C.1 added —
		// which sections have a compiled view of `workspace.db` all live in one
		// table now, wsfile.WorkspaceSections. This used to be eight
		// near-identical blocks with copy-pasted error handling, and the client
		// worked out what a heading showed by composing a form key from the
		// directory name and looking it up in its own registry.
		resultData, err = wsfile.BuildWorkspaceFileStructure(root, workspaceName)
		if err != nil {
			log.Println("while walking workspace structure:", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while walking workspace folder")
			return
		}
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
	// log.Println("*** Workspace Structure ***")
	// log.Println(string(v))
	// log.Println("*** Workspace Structure ***")
	results = &v
	return
}

// AddWorkspaceFile --------------------------------------------------------------------------
// Function to add a workspace file
func (ctx *DataTableContext) addWorkspaceFile(dataTableAction *DataTableAction, _ string) (err error) {
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name")
		log.Println(err)
		return
	}
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsFileName := request["source_file_name"]
		if wsFileName == nil {
			err = fmt.Errorf("GetWorkspaceFileContent: missing file_name")
			log.Println(err)
			return
		}
		var fileName string
		fileName, err = url.QueryUnescape(wsFileName.(string))
		if err != nil {
			log.Println(err)
			return
		}
		// Confine the file path within the workspace directory (CWE-73)
		var fullFileName string
		fullFileName, err = wsfile.ResolveWorkspacePath(workspaceName, fileName)
		if err != nil {
			log.Println(err)
			return
		}

		// Create an empty file to local workspace
		var myfile *os.File
		fileDir := filepath.Dir(fullFileName)
		if err = os.MkdirAll(fileDir, 0770); err != nil {
			err = fmt.Errorf("while creating file directory structure: %v", err)
			log.Println(err)
			return
		}

		myfile, err = os.Create(fullFileName)
		if err != nil {
			err = fmt.Errorf("while creating workspace file: %v", err)
			log.Println(err)
			return
		}
		myfile.Close()
	}
	return
}

// AddWorkspaceFile
func (ctx *DataTableContext) AddWorkspaceFile(dataTableAction *DataTableAction, token string) (rb *[]byte, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		httpStatus, err = RefusalFor(err2)
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
func (ctx *DataTableContext) DeleteWorkspaceFile(dataTableAction *DataTableAction, token string) (rb *[]byte, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		httpStatus, err = RefusalFor(err2)
		return
	}
	httpStatus = http.StatusOK
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name")
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsFileName := request["source_file_name"]
		if wsFileName == nil {
			err = fmt.Errorf("GetWorkspaceFileContent: missing file_name")
			log.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		var fileName string
		if fileName, err = url.QueryUnescape(wsFileName.(string)); err != nil {
			log.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		// Confine the file path within the workspace directory (CWE-73)
		var fullFileName string
		if fullFileName, err = wsfile.ResolveWorkspacePath(workspaceName, fileName); err != nil {
			log.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		// Write empty file to local workspace & db
		if err = wsfile.SaveContent(ctx.Dbpool, workspaceName, fileName, ""); err != nil {
			log.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}

		// Delete the local file
		err = os.Remove(fullFileName)
		if err != nil {
			err = fmt.Errorf("while removing workspace file: %v", err)
			log.Println(err)
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
// documentDirs are the workspace directories GetWorkspaceDocument serves, and
// the suffixes each may serve from.
//
// **This map is the whole of the security argument**, so it is a whitelist of
// both halves rather than a prefix check: a directory this does not name is not
// readable through this path at any capability, and a suffix that directory does
// not list is not readable either. `.pc.json` beside a flow, a `.sql` under
// reports, `workspace_control.json` at the root — none of them reach here.
var documentDirs = map[string][]string{
	"user_flows":    {".uf.json", ".ua.json", ".form.json", ".apply.json"},
	"table_configs": {".tc.json"},
}

// documentPathOK reports whether fileName names a document this path may serve.
//
// Exactly one separator, so a nested path cannot walk out of the directory the
// map named — `wsfile.GetContent` already confines to the workspace (CWE-73) and
// this confines to the two directories inside it. The two checks are
// independent and both are wanted: the first stops an escape from the workspace,
// the second stops a jetstore_read user reading the workspace's rules, its
// pipeline configurations or its client config.
func documentPathOK(fileName string) bool {
	dir, name, found := strings.Cut(fileName, "/")
	if !found || name == "" || strings.Contains(name, "/") {
		return false
	}
	suffixes, ok := documentDirs[dir]
	if !ok {
		return false
	}
	for _, suffix := range suffixes {
		// A file that is *only* the suffix — ".tc.json" — names no document and
		// is refused, which also refuses the dotfile it would otherwise be.
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			return true
		}
	}
	return false
}

// GetWorkspaceDocument --------------------------------------------------------------------------
// Reads one user flow or table configuration document, for the *running* app
// rather than for the IDE.
//
// **Why this exists beside GetWorkspaceFileContent, which reads any workspace
// file.** That one gates on `workspace_ide`, and `jets_init_db.sql` grants
// `workspace_ide` to `knowledge_engineer` alone. Since the eleven flows became
// workspace assets (ui_refresh's X.5) the React app reads its flow documents at
// run time, so the IDE's capability had become the capability required to *use a
// flow* — and `ops_user` and `client_advocate`, who hold `jetstore_read` and
// `run_pipelines`, could no longer open one. They are the roles the flows are
// for.
//
// The alternative was granting them `workspace_ide`, which also carries the free
// SQL query tool, git push, file save and delete, and purge data. Widening a
// capability to fix a read is how a capability stops meaning anything.
//
// So: same content, same confinement, a *read-only* verb over a whitelist of two
// directories, at the capability an ordinary user already has to read data.
func (ctx *DataTableContext) GetWorkspaceDocument(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	if code, err2 := ctx.requireCapability(CapabilityReadData, token); err2 != nil {
		return nil, code, err2
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	wsFileName := request["file_name"]
	if workspaceName == "" || wsFileName == nil {
		err = fmt.Errorf("GetWorkspaceDocument: missing workspace_name or file_name")
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	if !documentPathOK(fileName) {
		// Refused by path rather than by existence, so this cannot be used to
		// discover what a workspace holds outside the two directories.
		err = fmt.Errorf("GetWorkspaceDocument: %s is not a user flow or table configuration document", fileName)
		log.Println(err)
		httpStatus = http.StatusForbidden
		return
	}

	content, err := wsfile.GetContent(workspaceName, fileName)
	results = &map[string]any{
		"file_name":    wsFileName,
		"file_content": content,
	}
	return
}

func (ctx *DataTableContext) GetWorkspaceFileContent(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	wsFileName := request["file_name"]
	if workspaceName == "" || wsFileName == nil {
		err = fmt.Errorf("GetWorkspaceFileContent: missing workspace_name or file_name")
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Read file from local workspace
	content, err := wsfile.GetContent(workspaceName, fileName)
	results = &map[string]any{
		"file_name":    wsFileName,
		"file_content": content,
	}
	return
}

// SaveWorkspaceFileContent --------------------------------------------------------------------------
// Function to save the workspace file content in local workspace file system and in database
func (ctx *DataTableContext) SaveWorkspaceFileContent(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	wsFileName := request["file_name"]
	wsFileContent := request["file_content"]
	if workspaceName == "" || wsFileName == nil || wsFileContent == nil {
		err = fmt.Errorf("SaveWorkspaceFileContent: missing workspace_name, file_content, or file_name")
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	fileName, err := url.QueryUnescape(wsFileName.(string))
	if err != nil {
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Validate structured files before writing them. Two steps, in this order,
	// and the order is the agreement with the agentic_ai stream (Q-3):
	//
	//  1. **Well-formedness, for anything ending .json.** It is a precondition for
	//     every structured check, so doing it once keeps one bad file from
	//     producing two different complaints.
	//  2. **At most one specific validator**, on the most specific suffix match —
	//     see validatorFor. Absent for a plain .json, which keeps the behaviour
	//     every existing file type has today.
	// Warnings travel with the findings and do not block; only errors do. Which
	// of the two an unreachable state is, is the deployment's call through
	// JETS_USERFLOW_STRICT_REACHABILITY.
	content := wsFileContent.(string)
	if err = checkWorkspaceFile(fileName, content); err != nil {
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	// Write file to local workspace
	err = wsfile.SaveContent(ctx.Dbpool, workspaceName, fileName, content)
	results = &map[string]any{
		"file_name": wsFileName,
	}
	return
}

// SaveWorkspaceClientConfig --------------------------------------------------------------------------
// Function to save the workspace file content in local workspace file system and in database
func (ctx *DataTableContext) SaveWorkspaceClientConfig(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	httpStatus = http.StatusOK
	request := dataTableAction.Data[0]
	workspaceName := dataTableAction.WorkspaceName
	clientName := request["client"]
	if workspaceName == "" || clientName == nil {
		err = fmt.Errorf("SaveWorkspaceClientConfig: missing workspace_name, or client")
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}

	// Save client config to local workspace
	err = wsfile.SaveClientConfig(ctx.Dbpool, workspaceName, clientName.(string))
	results = &map[string]any{}
	return
}

// DeleteWorkspaceChanges --------------------------------------------------------------------------
// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
// Restaure files from stash, except for .db and .tgz files
func (ctx *DataTableContext) DeleteWorkspaceChanges(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	httpStatus = http.StatusOK
	workspaceName := dataTableAction.WorkspaceName
	for ipos := range dataTableAction.Data {
		request := dataTableAction.Data[ipos]
		wsFileName := request["file_name"]
		if workspaceName == "" || wsFileName == nil {
			err = fmt.Errorf("DeleteWorkspaceChanges: missing workspace_name, oid, key, or file_name")
			log.Println(err)
			httpStatus = http.StatusBadRequest
			return
		}
		// **Reverting a file runs whether or not git is on, and the reason it is
		// safe with git off is a property of the stash rather than of this call.**
		//
		// `DeleteFileChange` deletes the file's row from `jetsapi.workspace_changes`
		// and copies the stashed version back over the working file. The deletion is
		// only a revert if the stash holds the *pristine* content; if it holds the
		// user's own edits the row is discarded and the file does not change, which
		// is a silent loss rather than a revert.
		//
		// With a repository the stash is re-taken after every pull, so it tracks the
		// remote. With no repository it is taken once at startup, from the tree
		// `cbooter` copied out of the image and before the database overrides are
		// applied -- so it is pristine by construction. **What makes that hold is
		// that `pullWorkspaceAction` no longer clears and re-takes it when git is
		// off** (see the comment there): the clear is what would have replaced the
		// pristine snapshot with an edited one.
		//
		// This was gated shut for one revision of Phase 6, before that fix existed,
		// on the ground that the row is the only copy of the edit. It is the only
		// copy, and with a pristine stash the revert is still what the user asked
		// for. Q-89.
		err = wsfile.DeleteFileChange(ctx.Dbpool, workspaceName, wsFileName.(string))
		if err != nil {
			httpStatus = http.StatusBadRequest
			return
		}
	}

	results = &map[string]any{}
	return
}

// DeleteAllWorkspaceChanges --------------------------------------------------------------------------
// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
func (ctx *DataTableContext) DeleteAllWorkspaceChanges(dataTableAction *DataTableAction, token string) (results *map[string]any, httpStatus int, err error) {
	_, err2 := ctx.VerifyUserPermission(&SqlInsertDefinition{Capability: "workspace_ide"}, token)
	if err2 != nil {
		status, refusal := RefusalFor(err2)
		return nil, status, refusal
	}
	httpStatus = http.StatusOK
	workspaceName := dataTableAction.WorkspaceName
	if workspaceName == "" {
		err = fmt.Errorf("DeleteAllWorkspaceChanges: missing workspace_name")
		log.Println(err)
		httpStatus = http.StatusBadRequest
		return
	}
	// Runs whether or not git is on; `DeleteWorkspaceChanges` above carries the
	// argument, which is that the stash a no-git deployment restores from is
	// pristine by construction and stays that way. Q-89.
	//
	// **Note the asymmetry with `commit_workspace`, which is still skipped when
	// git is off.** That call passes `restaureFromStash` false: it drops the rows
	// *without* putting anything back, because with a repository the content has
	// just been pushed. With no repository nothing was pushed and nothing is
	// restored, so it is a deletion with no counterpart -- which is a different
	// act from the revert here, and the reason one is gated and the other is not.
	//
	// Delete all workspace changes and restaure from stash
	err = wsfile.DeleteAllFileChanges(ctx.Dbpool, workspaceName, true, false)
	if err != nil {
		httpStatus = http.StatusBadRequest
		return
	}

	results = &map[string]any{}
	return
}
