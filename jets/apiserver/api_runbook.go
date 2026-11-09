package main

// The workspace runbook endpoint, /runbook (project runbooks, Phase 1, track
// AB; plan §3.4). One POST route with an action name, shaped as /agentic is and
// for the same reasons (the client's POST helper reads the refreshed token out
// of every response body).
//
// Every action needs the runbook_execute capability, checked here. That is the
// authority to see and control runs, not to do what a step does: each step
// checks its own capability, as its person, when it runs (the executor,
// jets/datatable/runbook_executor.go). The actor of every decision is the
// authenticated user; no request field names one.
//
// Field names in requests and responses are snake_case, as the two tables name
// their columns. A refusal is an HTTP status with {"error": message}; a refusal
// because the runbook does not validate also carries "findings".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

// RunbookAction is the request envelope; fields are read per action.
type RunbookAction struct {
	Action        string                     `json:"action"`
	WorkspaceName string                     `json:"workspace_name"`
	Runbook       string                     `json:"runbook"`
	Parameters    map[string]json.RawMessage `json:"parameters"`
	Limit         int                        `json:"limit"`
	RunKey        int64                      `json:"run_key"`
	StepKey       *int64                     `json:"step_key"`
	Comment       string                     `json:"comment"`
	Values        map[string]json.RawMessage `json:"values"`
}

// runbookOps is what the endpoint needs of the executor, as an interface so
// the dispatch is testable with a fake.
type runbookOps interface {
	ListRunbooks(ctx context.Context, workspaceName string) (*map[string]any, error)
	GetRunbook(ctx context.Context, workspaceName, name string) (*map[string]any, error)
	StartRun(ctx context.Context, actor, workspaceName, name string, params map[string]json.RawMessage) (*map[string]any, error)
	ListRuns(ctx context.Context, workspaceName, name string, limit int) (*map[string]any, error)
	GetRun(ctx context.Context, key int64) (*map[string]any, error)
	Accept(ctx context.Context, actor string, key, stepKey int64, comment string) (*map[string]any, error)
	Deny(ctx context.Context, actor string, key, stepKey int64, comment string) (*map[string]any, error)
	Continue(ctx context.Context, actor string, key int64, stepKey *int64) (*map[string]any, error)
	Skip(ctx context.Context, actor string, key int64, stepKey *int64) (*map[string]any, error)
	Retry(ctx context.Context, actor string, key int64, stepKey *int64) (*map[string]any, error)
	Abort(ctx context.Context, actor string, key int64) (*map[string]any, error)
	Record(ctx context.Context, actor string, key, stepKey int64, values map[string]json.RawMessage) (*map[string]any, error)
	Release(ctx context.Context, actor string, key int64) (*map[string]any, error)
	Export(ctx context.Context, key int64) (*map[string]any, error)
}

// DoRunbookAction is the /runbook handler. Authentication is authh's.
func (server *Server) DoRunbookAction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	token := user.ExtractToken(r)
	userEmail, _ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", userEmail),
		zap.String("time", time.Now().Format(time.RFC3339)))

	jetsUser, err := user.GetUserByToken(server.dbpool, token)
	if err != nil {
		log.Printf("while GetUserByToken: %v", err)
		ERROR(w, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info"))
		return
	}
	if !jetsUser.HasCapability(datatable.RunbookExecuteCapability) {
		log.Printf("user %s attempted a runbook action without the %s capability",
			userEmail, datatable.RunbookExecuteCapability)
		ERROR(w, http.StatusForbidden, errors.New("error: unauthorized, user do not have required capability"))
		return
	}
	action := RunbookAction{}
	if err = json.Unmarshal(body, &action); err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	exec := datatable.CurrentRunbookExecutor()
	if exec == nil {
		ERROR(w, http.StatusServiceUnavailable, errors.New("the runbook executor is not running"))
		return
	}
	results, code, err := runbookDispatch(r.Context(), exec, &action, jetsUser.Email)
	if err != nil {
		log.Printf("runbook action %s: %v", action.Action, err)
		writeRunbookError(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}

// writeRunbookError writes {"error"} and, for a runbook that does not
// validate, its findings.
func writeRunbookError(w http.ResponseWriter, code int, err error) {
	var re *datatable.RunbookError
	if errors.As(err, &re) && len(re.Findings) > 0 {
		JSON(w, code, map[string]any{"error": re.Message, "findings": re.Findings})
		return
	}
	ERROR(w, code, err)
}

// runbookDispatch is the endpoint's behaviour without the http plumbing.
func runbookDispatch(ctx context.Context, ops runbookOps, a *RunbookAction, actor string) (*map[string]any, int, error) {
	needRun := func() error {
		if a.RunKey <= 0 {
			return fmt.Errorf("%s needs run_key", a.Action)
		}
		return nil
	}
	needStep := func() error {
		if err := needRun(); err != nil {
			return err
		}
		if a.StepKey == nil {
			return fmt.Errorf("%s needs step_key", a.Action)
		}
		return nil
	}
	needWorkspace := func() error {
		if a.WorkspaceName == "" {
			return fmt.Errorf("%s needs workspace_name", a.Action)
		}
		return nil
	}
	var check func() error
	var call func() (*map[string]any, error)
	switch a.Action {
	case "list_runbooks":
		check, call = needWorkspace, func() (*map[string]any, error) { return ops.ListRunbooks(ctx, a.WorkspaceName) }
	case "get_runbook":
		check = func() error {
			if err := needWorkspace(); err != nil {
				return err
			}
			if a.Runbook == "" {
				return errors.New("get_runbook needs runbook")
			}
			return nil
		}
		call = func() (*map[string]any, error) { return ops.GetRunbook(ctx, a.WorkspaceName, a.Runbook) }
	case "start_run":
		check = func() error {
			if err := needWorkspace(); err != nil {
				return err
			}
			if a.Runbook == "" {
				return errors.New("start_run needs runbook")
			}
			return nil
		}
		call = func() (*map[string]any, error) {
			return ops.StartRun(ctx, actor, a.WorkspaceName, a.Runbook, a.Parameters)
		}
	case "list_runs":
		check, call = needWorkspace, func() (*map[string]any, error) {
			return ops.ListRuns(ctx, a.WorkspaceName, a.Runbook, a.Limit)
		}
	case "get_run":
		check, call = needRun, func() (*map[string]any, error) { return ops.GetRun(ctx, a.RunKey) }
	case "accept":
		check, call = needStep, func() (*map[string]any, error) { return ops.Accept(ctx, actor, a.RunKey, *a.StepKey, a.Comment) }
	case "deny":
		check, call = needStep, func() (*map[string]any, error) { return ops.Deny(ctx, actor, a.RunKey, *a.StepKey, a.Comment) }
	case "continue":
		check, call = needRun, func() (*map[string]any, error) { return ops.Continue(ctx, actor, a.RunKey, a.StepKey) }
	case "skip":
		check, call = needRun, func() (*map[string]any, error) { return ops.Skip(ctx, actor, a.RunKey, a.StepKey) }
	case "retry":
		check, call = needRun, func() (*map[string]any, error) { return ops.Retry(ctx, actor, a.RunKey, a.StepKey) }
	case "abort":
		check, call = needRun, func() (*map[string]any, error) { return ops.Abort(ctx, actor, a.RunKey) }
	case "record":
		check, call = needStep, func() (*map[string]any, error) {
			return ops.Record(ctx, actor, a.RunKey, *a.StepKey, a.Values)
		}
	case "release":
		check, call = needRun, func() (*map[string]any, error) { return ops.Release(ctx, actor, a.RunKey) }
	case "export":
		check, call = needRun, func() (*map[string]any, error) { return ops.Export(ctx, a.RunKey) }
	default:
		return nil, http.StatusBadRequest, fmt.Errorf("unknown runbook action %q", a.Action)
	}
	if err := check(); err != nil {
		return nil, http.StatusBadRequest, err
	}
	res, err := call()
	if err != nil {
		return nil, datatable.RunbookErrorStatus(err), err
	}
	return res, http.StatusOK, nil
}
