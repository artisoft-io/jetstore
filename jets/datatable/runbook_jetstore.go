package datatable

// The JetStore step types of a runbook, through the functions the UI's buttons
// call (assessment §4.2: "a step calls the same Go function as the UI button
// it replaces"), so a runbook can do nothing the person could not do by hand,
// and the existing gates still apply: InsertRows' permission check and
// throttling, claimCompile's exclusion of a compile and rules unit tests.
// Project runbooks, Phase 1, track AB.
//
// **InsertRows takes a token, and the executor has none of its own.** A token
// for the step's person is minted in-process (user.CreateToken) and handed to
// InsertRows, which reads the person and their capability back from the
// database with it. It never leaves the process: it is the person's own
// identity in the form the function asks for, not a service identity.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/artisoft-io/jetstore/jets/user"
)

// RunbookDataTableOps is RunbookJetStore over a DataTableContext.
type RunbookDataTableOps struct {
	Ctx *DataTableContext
}

// StartPending is Check Pending.
func (o RunbookDataTableOps) StartPending(ctx context.Context, actor string) error {
	return o.Ctx.StartPendingTasks()
}

// StartPipeline inserts a submitted pipeline_execution_status row through
// InsertRows, as the Start Pipeline dialog does. The row's status is
// submitted and its user_email the person, whatever the runbook says; a row
// with no session_id gets a reserved one.
func (o RunbookDataTableOps) StartPipeline(ctx context.Context, actor string, row map[string]any) (string, error) {
	data := make(map[string]any, len(row)+3)
	for k, v := range row {
		data[k] = v
	}
	data["status"] = "submitted"
	data["user_email"] = actor
	sid, _ := data["session_id"].(string)
	if sid == "" {
		var err error
		if sid, err = ReserveSessionId(o.Ctx.Dbpool); err != nil {
			return "", fmt.Errorf("while reserving a session id: %v", err)
		}
		data["session_id"] = sid
	}
	token, err := user.CreateToken(actor)
	if err != nil {
		return "", fmt.Errorf("while identifying %s to InsertRows: %v", actor, err)
	}
	_, status, err := o.Ctx.InsertRows(&DataTableAction{
		Action:      "insert_rows",
		FromClauses: []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
		Data:        []map[string]any{data},
	}, token)
	if err != nil {
		return "", fmt.Errorf("InsertRows answered %d: %v", status, err)
	}
	return sid, nil
}

// PutSchemaEvent is Put Schema Event.
func (o RunbookDataTableOps) PutSchemaEvent(ctx context.Context, actor, event, key string) ([]string, error) {
	res, _, err := o.Ctx.PutSchemaEventToS3(&RegisterFileKeyAction{
		Action: "put_schema_event_to_s3",
		Data:   []map[string]any{{"event": event, "file_key": key}},
	}, "")
	if err != nil {
		return nil, err
	}
	keys, _ := (*res)["file_keys"].([]string)
	return keys, nil
}

// Compile is Compile Workspace: claimCompile's exclusion and claim, then the
// compile, waited for until the deadline. A compile still running at the
// deadline goes on and writes its own status; the step fails.
func (o RunbookDataTableOps) Compile(ctx context.Context, actor, workspaceName string, deadline time.Duration) error {
	if status, err := claimCompile(o.Ctx.Dbpool, workspaceName, nil); err != nil {
		if status == http.StatusConflict {
			return fmt.Errorf("refused: %v", err)
		}
		return err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		compileWorkspaceAction(o.Ctx, &DataTableAction{
			Action:        "workspace_insert_rows",
			FromClauses:   []FromClause{{Table: "compile_workspace_by_name"}},
			WorkspaceName: workspaceName,
			Data: []map[string]any{{
				"workspace_name": workspaceName, "status": statusCompileInProgress, "user_email": actor, "last_git_log": nil,
			}},
		})
	}()
	select {
	case <-done:
	case <-time.After(deadline):
		return fmt.Errorf("the compile was still running at the %s deadline", deadline)
	case <-ctx.Done():
		return errors.New("cancelled while the compile was running; it goes on")
	}
	var status string
	var gitLog *string
	err := o.Ctx.Dbpool.QueryRow(context.Background(), `SELECT coalesce(status, ''), last_git_log
		FROM jetsapi.workspace_registry WHERE workspace_name = $1`, workspaceName).Scan(&status, &gitLog)
	if err != nil {
		return fmt.Errorf("while reading the compile's status: %v", err)
	}
	if status == "error" {
		tail := ""
		if gitLog != nil {
			tail = *gitLog
			if len(tail) > 2000 {
				tail = "..." + tail[len(tail)-2000:]
			}
		}
		return fmt.Errorf("the compile failed: %s", tail)
	}
	return nil
}
