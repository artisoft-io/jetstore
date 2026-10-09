package datatable

// Binding a run to the workspace it ran under.
//
// pipeline_execution_status had seventeen columns and none of them named a
// workspace, a branch or a version, so a regression detector could establish
// *this step used to work and now does not* and nothing in the record could answer
// *what changed*. jetsapi.workspace_version is written at compile time with no
// reference to any run, and workspace_registry.last_git_log describes the
// workspace's current state rather than a run's.
//
// The binding is stamped when the run's row is inserted, which is the moment the
// run is handed to a state machine, and it names two things:
//
//   - workspace_name, from the WORKSPACE environment of the process inserting the
//     row. That is the active workspace of the deployment by construction: it is
//     the same variable jets/workspace reads to decide what to sync.
//   - workspace_version, MAX(version) of jetsapi.workspace_version, which is what
//     GetWorkspaceVersion returns and what every sync compares itself against.
//
// **What this is not.** The version is a compile marker -- CompileWorkspace is
// called with a unix timestamp from the Workspace IDE and with the JetStore version
// at server start -- so it identifies the compiled artefact and not a git commit.
// Resolving a version to a commit would need workspace_version to record the
// workspace and its last_git_log at compile time, and that table is uniquely keyed
// on the version string alone across every workspace. That is a separate change
// with a separate migration and it is deliberately not made here.

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// setWorkspaceBinding stamps the workspace name and version into the row about to
// be inserted into pipeline_execution_status. Both are written as nil when they
// cannot be established, so the columns are null rather than empty: a run inserted
// before a workspace was ever compiled genuinely has no version, and an empty string would read
// as one.
func setWorkspaceBinding(dbpool *pgxpool.Pool, row map[string]any) {
	var name, version any
	if ws := os.Getenv("WORKSPACE"); len(ws) > 0 {
		name = ws
	}
	if v, ok := currentWorkspaceVersion(dbpool); ok {
		version = v
	}
	row["workspace_name"] = name
	row["workspace_version"] = version
}

// currentWorkspaceVersion returns the compiled workspace version in force. It
// reports not-ok rather than an error: a run must not fail to start because the
// record cannot say what it will run under, and the null column says so.
func currentWorkspaceVersion(dbpool *pgxpool.Pool) (string, bool) {
	if dbpool == nil {
		return "", false
	}
	var version *string
	err := dbpool.QueryRow(context.Background(),
		"SELECT MAX(version) FROM jetsapi.workspace_version").Scan(&version)
	if err != nil || version == nil || len(*version) == 0 {
		return "", false
	}
	return *version, true
}
