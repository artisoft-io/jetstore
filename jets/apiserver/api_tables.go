package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

var stagePrefix string = os.Getenv("JETS_s3_STAGE_PREFIX")

// DoDataTableAction ------------------------------------------------------
// Entry point function
func (server *Server) DoDataTableAction(w http.ResponseWriter, r *http.Request) {
	var results *map[string]any
	var code int
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	token := user.ExtractToken(r)
	user, _ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", user), zap.String("time", time.Now().Format(time.RFC3339)))
	dataTableAction := datatable.DataTableAction{Limit: 200}
	err = json.Unmarshal(body, &dataTableAction)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	ctx := datatable.NewDataTableContext(server.dbpool, globalDevMode, *usingSshTunnel, unitTestDir, adminEmail)
	// Intercept specific dataTable action
	switch dataTableAction.Action {
	case "raw_query":
		// Queries the app composed, issued on a user's behalf.
		results, code, err = ctx.ExecRawQuery(&dataTableAction, token, datatable.CapabilityReadData)
	case "raw_query_tool":
		// The IDE's free-SQL box: the statement is whatever the user typed, so it
		// wants the capability the seed file already says covers the query tool.
		results, code, err = ctx.ExecRawQuery(&dataTableAction, token, datatable.CapabilityQueryTool)
	case "exec_ddl":
		results, code, err = ctx.ExecDataManagementStatement(&dataTableAction, token)
	case "raw_query_map":
		results, code, err = ctx.ExecRawQueryMap(&dataTableAction, token)
	case "insert_raw_rows":
		results, code, err = ctx.InsertRawRows(&dataTableAction, token)
	case "insert_rows":
		results, code, err = ctx.InsertRows(&dataTableAction, token)

		// fetch file from stage
	case "fetch_file_from_stage":
		// **This arm asked for nothing until 2026-08-25** (ui_refresh I-136's sweep).
		// It is handled here rather than by a method on DataTableContext, so the
		// per-statement Capability in sql_stmts.go never applied to it and no test
		// over that type's methods could see it. It reads an object out of the S3
		// stage at a path the request chooses; jetstore_read is what
		// DoPreviewFileAction requires for the nearest equivalent.
		if code, err = ctx.RequireCapability(datatable.CapabilityReadData, token); err != nil {
			ERROR(w, code, err)
			return
		}
		results = &map[string]any{}
		code = 200
		filePath, ok := dataTableAction.Data[0]["stage_file_path"].(string)
		if !ok {
			err = fmt.Errorf("error: stage_file_path must be string in fetch_file_from_stage action")
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}

		obj, err := awsi.DownloadBufFromS3(fmt.Sprintf("%s/%s", stagePrefix, filePath))
		if err != nil {
			code := stageFetchStatus(err)
			err = fmt.Errorf("error: failed to fetch file from stage: %v", err)
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		(*results)["file_content"] = string(obj)

		// resubmit pipeline
	case "resubmit_pipeline":
		// **A second route to a gated table, and it was the ungated one** (ui_refresh
		// I-136's sweep, 2026-08-25). insert_rows on jetsapi.pipeline_execution_status
		// resolves sqlInsertStmts and is refused without run_pipelines; this arm
		// hand-writes the same INSERT, resolves no statement, and inherited no
		// capability -- then calls StartPendingTasks, so it does not merely record a
		// run, it starts one. It stamps the token's user onto the row, which is the
		// same pairing I-124 named: careful about attribution, silent about
		// authorisation. datatable.TestResubmitPipelineUsesTheSameCapabilityAsItsInsert
		// ties the constant below to the statement's own Capability so the two routes
		// to this table cannot drift apart.
		if code, err = ctx.RequireCapability(datatable.CapabilityRunPipelines, token); err != nil {
			ERROR(w, code, err)
			return
		}
		results = &map[string]any{}
		code = 200
		sid, ok := dataTableAction.Data[0]["session_id"].(string)
		if !ok {
			err = fmt.Errorf("error: session_id must be string in resubmit_pipeline")
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		if sid == "F000000000000" {
			err = fmt.Errorf("error: invalid session_id in resubmit_pipeline")
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		// Refuse a run whose input was purged, rather than start one that fails at
		// its first step
		if err = datatable.CheckResubmittable(context.TODO(), server.dbpool, sid); err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		newSessionId, err := datatable.ReserveSessionId(server.dbpool)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		// datatable.ResubmitPipelineStmt says what is copied and what is not.
		stmt := datatable.ResubmitPipelineStmt
		// The copy reads the same input sessions as the runs it copies, so it is
		// guarded as InsertPipelineExecutionStatus guards a start: in one
		// transaction, each input session locked and refused with 409 when it is
		// queued for deletion or deleted (session_registry Q-16).
		tx, err := server.dbpool.Begin(context.TODO())
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, http.StatusInternalServerError, err)
			return
		}
		defer tx.Rollback(context.Background())
		if code, err = datatable.GuardResubmission(context.TODO(), tx, sid); err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		_, err = tx.Exec(context.TODO(), stmt, newSessionId, user, sid, os.Getenv("WORKSPACE"))
		if err == nil {
			err = tx.Commit(context.TODO())
		}
		if err != nil {
			err = fmt.Errorf("error: failed resubmit to database: %v", err)
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		// Start the pending task and check for timeouts
		err = ctx.StartPendingTasks()
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}

		// start pending tasks
	case "start_pending_tasks":
		// The Pipeline Status screen's *Check Pending* button (2026-11-05). A row
		// inserted into jetsapi.pipeline_execution_status with status 'pending' by
		// SQL, rather than through the UI, waits for the next StartPendingTasks:
		// after a resubmit_pipeline, a status update or a domain-table registration,
		// or the apiserver's hourly sweep (Start, server.go), which does not run in
		// dev mode. This arm is that call, on demand, and nothing else. It starts
		// pipelines, so it asks for the capability resubmit_pipeline asks for,
		// before it does anything.
		if code, err = ctx.RequireCapability(datatable.CapabilityRunPipelines, token); err != nil {
			ERROR(w, code, err)
			return
		}
		// Also marks submitted runs past their timeout as timed_out, as every
		// caller of StartPendingTasks does.
		if err = ctx.StartPendingTasks(); err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		// What is still pending afterwards, so the caller can tell a run held
		// back by throttling from one that started. Counted the way
		// StartPendingTasks selects them: a staged file key and a process_config row.
		var pending int64
		if pending, _, err = ctx.GetTaskThrottlingInfo("pending"); err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, 400, err)
			return
		}
		results = &map[string]any{"pending_count": pending}
		code = 200

	case "resubmit_sessions_as_pending":
		// Session Registry's Resubmit as Pending (JM-185, 2026-11-12): copies the
		// selected sessions' runs into new sessions with status pending and starts
		// nothing, so the sessions they replace can be deleted before Check Pending
		// starts them (datatable.ResubmitSessionsAsPending). It inserts what
		// resubmit_pipeline inserts, so it asks for the capability that arm asks for.
		if code, err = ctx.RequireCapability(datatable.CapabilityRunPipelines, token); err != nil {
			ERROR(w, code, err)
			return
		}
		results, code, err = ctx.ResubmitSessionsAsPending(&dataTableAction, user, os.Getenv("WORKSPACE"))

	case "preview_session_deletion":
		// The sessions a Delete of the selected ones would delete: each selected
		// session and, transitively, every session whose run read one of them
		// (session_registry Q-4). It writes nothing, and answers
		// {"rows": [...]} (datatable.PreviewSessionDeletion). Asked for
		// delete_session because what it shows is the confirmation of a deletion.
		if code, err = ctx.RequireCapability(datatable.CapabilityDeleteSession, token); err != nil {
			ERROR(w, code, err)
			return
		}
		results, code, err = ctx.PreviewSessionDeletion(&dataTableAction)

	case "delete_sessions":
		// Queues the closure of the selected sessions for deletion, when it is the
		// one the caller previewed, and answers {"queued": n, "waiting": [...]}
		// (datatable.DeleteSessions); then starts this process's executor, which
		// deletes after the answer. requested_by is the token's user.
		if code, err = ctx.RequireCapability(datatable.CapabilityDeleteSession, token); err != nil {
			ERROR(w, code, err)
			return
		}
		results, code, err = ctx.DeleteSessions(&dataTableAction, user)
		if err == nil {
			server.runUserDeletions()
		}

	case "workspace_insert_rows":
		results, code, err = ctx.WorkspaceInsertRows(&dataTableAction, token)
	case "workspace_query_structure":
		// This function returns encoded json ready to return to client
		resultsB, code, err := ctx.WorkspaceQueryStructure(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "add_workspace_file":
		resultsB, code, err := ctx.AddWorkspaceFile(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "delete_workspace_files":
		resultsB, code, err := ctx.DeleteWorkspaceFile(&dataTableAction, token)
		if err != nil {
			log.Printf("Error: %v", err)
			ERROR(w, code, err)
			return
		}
		JSONB(w, http.StatusOK, *resultsB)
		return

	case "get_workspace_document":
		// The runtime read: a flow's documents and the tables they draw, at
		// jetstore_read. See GetWorkspaceDocument for why it is not the case below.
		results, code, err = ctx.GetWorkspaceDocument(&dataTableAction, token)
	case "get_workspace_file_content":
		results, code, err = ctx.GetWorkspaceFileContent(&dataTableAction, token)
	case "save_workspace_file_content":
		results, code, err = ctx.SaveWorkspaceFileContent(&dataTableAction, token)
	case "delete_workspace_changes":
		results, code, err = ctx.DeleteWorkspaceChanges(&dataTableAction, token)
	case "delete_all_workspace_changes":
		results, code, err = ctx.DeleteAllWorkspaceChanges(&dataTableAction, token)

	case "workspace_read":
		results, code, err = ctx.DoWorkspaceReadAction(&dataTableAction, token)

	case "save_workspace_client_config":
		results, code, err = ctx.SaveWorkspaceClientConfig(&dataTableAction, token)

	case "read":
		results, code, err = ctx.DoReadAction(&dataTableAction, token)
	case "preview_file":
		results, code, err = ctx.DoPreviewFileAction(&dataTableAction, token)
	case "drop_table":
		results, code, err = ctx.DropTable(&dataTableAction, token)
	case "refresh_token":
		results = &map[string]any{}
		code = http.StatusOK
		err = nil
	case "get_workspace_uri":
		results = &map[string]any{
			"workspace_uri":               os.Getenv("WORKSPACE_URI"),
			"workspace_name":              os.Getenv("WORKSPACE"),
			"workspace_branch":            os.Getenv("WORKSPACE_BRANCH"),
			"workspace_file_key_label_re": os.Getenv("WORKSPACE_FILE_KEY_LABEL_RE"),
		}
		code = http.StatusOK
		err = nil
	default:
		code = http.StatusUnprocessableEntity
		err = fmt.Errorf("DoDataTableAction: unknown action: %v", dataTableAction.Action)
	}
	if err != nil {
		log.Printf("Error: %v", err)
		ERROR(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}

func addToken(r *http.Request, results *map[string]any) {
	token, ok := r.Header["Token"]
	if ok {
		(*results)["token"] = token[0]
	}
}

// stageFetchStatus is the status fetch_file_from_stage answers a failed download
// with: 404 when there is no such object, 400 for anything else.
//
// jetstore_maintenance_02 I-23, 2026-10-01. Every failure was a 400, so a client
// could tell "no such object" from "the read failed" only by finding NoSuchKey in
// the message text. The difference matters to the Pipeline Status screen's Get
// Run Manifest: a manifest is written for completed cpipes runs only, so its
// absence is an answer to report plainly rather than an error. The client
// (jetsclient_ide/src/actions/stageClipboard.ts, isMissingStageObject) reads a
// 404 as missing and still accepts the old 400-with-NoSuchKey form.
func stageFetchStatus(err error) int {
	if awsi.IsNoSuchKey(err) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}
