package main

// The /runbook endpoint's dispatch, driven through a fake executor: every
// action reaches its operation with the request's fields and the
// authenticated actor, a malformed request is refused 400 before anything
// runs, and the executor's refusals keep their status. The executor itself is
// tested against PostgreSQL in jets/datatable (runbook_executor_test.go).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

type fakeRunbookOps struct {
	calls []string
	err   error
}

func (f *fakeRunbookOps) note(format string, args ...any) (*map[string]any, error) {
	b, _ := json.Marshal(args)
	f.calls = append(f.calls, format+" "+string(b))
	if f.err != nil {
		return nil, f.err
	}
	return &map[string]any{"run": map[string]any{"run_key": 7}}, nil
}

func (f *fakeRunbookOps) ListRunbooks(_ context.Context, ws string) (*map[string]any, error) {
	return f.note("list_runbooks", ws)
}
func (f *fakeRunbookOps) GetRunbook(_ context.Context, ws, name string) (*map[string]any, error) {
	return f.note("get_runbook", ws, name)
}
func (f *fakeRunbookOps) StartRun(_ context.Context, actor, ws, name string, p map[string]json.RawMessage) (*map[string]any, error) {
	return f.note("start_run", actor, ws, name, p)
}
func (f *fakeRunbookOps) ListRuns(_ context.Context, ws, name string, limit int) (*map[string]any, error) {
	return f.note("list_runs", ws, name, limit)
}
func (f *fakeRunbookOps) GetRun(_ context.Context, key int64) (*map[string]any, error) {
	return f.note("get_run", key)
}
func (f *fakeRunbookOps) Accept(_ context.Context, actor string, key, step int64, c string) (*map[string]any, error) {
	return f.note("accept", actor, key, step, c)
}
func (f *fakeRunbookOps) Deny(_ context.Context, actor string, key, step int64, c string) (*map[string]any, error) {
	return f.note("deny", actor, key, step, c)
}
func (f *fakeRunbookOps) Continue(_ context.Context, actor string, key int64, step *int64) (*map[string]any, error) {
	return f.note("continue", actor, key, step)
}
func (f *fakeRunbookOps) Skip(_ context.Context, actor string, key int64, step *int64) (*map[string]any, error) {
	return f.note("skip", actor, key, step)
}
func (f *fakeRunbookOps) Retry(_ context.Context, actor string, key int64, step *int64) (*map[string]any, error) {
	return f.note("retry", actor, key, step)
}
func (f *fakeRunbookOps) Abort(_ context.Context, actor string, key int64) (*map[string]any, error) {
	return f.note("abort", actor, key)
}
func (f *fakeRunbookOps) Record(_ context.Context, actor string, key, step int64, v map[string]json.RawMessage) (*map[string]any, error) {
	return f.note("record", actor, key, step, v)
}
func (f *fakeRunbookOps) Release(_ context.Context, actor string, key int64) (*map[string]any, error) {
	return f.note("release", actor, key)
}
func (f *fakeRunbookOps) Export(_ context.Context, key int64) (*map[string]any, error) {
	return f.note("export", key)
}

func runbookRequest(t *testing.T, body string) *RunbookAction {
	t.Helper()
	a := &RunbookAction{}
	if err := json.Unmarshal([]byte(body), a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRunbookDispatchReachesEveryAction(t *testing.T) {
	cases := []struct{ body, want string }{
		{`{"action":"list_runbooks","workspace_name":"jets_ws"}`, `list_runbooks ["jets_ws"]`},
		{`{"action":"get_runbook","workspace_name":"jets_ws","runbook":"rb"}`, `get_runbook ["jets_ws","rb"]`},
		{`{"action":"start_run","workspace_name":"jets_ws","runbook":"rb","parameters":{"n":3}}`,
			`start_run ["ke@x","jets_ws","rb",{"n":3}]`},
		{`{"action":"list_runs","workspace_name":"jets_ws","limit":5}`, `list_runs ["jets_ws","",5]`},
		{`{"action":"get_run","run_key":7}`, `get_run [7]`},
		{`{"action":"accept","run_key":7,"step_key":9,"comment":"ok"}`, `accept ["ke@x",7,9,"ok"]`},
		{`{"action":"deny","run_key":7,"step_key":9}`, `deny ["ke@x",7,9,""]`},
		{`{"action":"continue","run_key":7,"step_key":9}`, `continue ["ke@x",7,9]`},
		{`{"action":"skip","run_key":7}`, `skip ["ke@x",7,null]`},
		{`{"action":"retry","run_key":7,"step_key":9}`, `retry ["ke@x",7,9]`},
		{`{"action":"abort","run_key":7}`, `abort ["ke@x",7]`},
		{`{"action":"record","run_key":7,"step_key":9,"values":{"at":"2026-11-06T17:00:00Z"}}`,
			`record ["ke@x",7,9,{"at":"2026-11-06T17:00:00Z"}]`},
		{`{"action":"release","run_key":7}`, `release ["ke@x",7]`},
		{`{"action":"export","run_key":7}`, `export [7]`},
	}
	for _, c := range cases {
		ops := &fakeRunbookOps{}
		res, code, err := runbookDispatch(context.Background(), ops, runbookRequest(t, c.body), "ke@x")
		if err != nil || code != http.StatusOK || res == nil {
			t.Errorf("%s: %d %v", c.body, code, err)
			continue
		}
		if len(ops.calls) != 1 || ops.calls[0] != c.want {
			t.Errorf("%s reached %v, want %s", c.body, ops.calls, c.want)
		}
	}
}

// The actor is the authenticated user: a request that names one is not
// believed (the envelope has no such field).
func TestRunbookDispatchTakesTheActorFromTheToken(t *testing.T) {
	ops := &fakeRunbookOps{}
	_, _, err := runbookDispatch(context.Background(), ops,
		runbookRequest(t, `{"action":"accept","run_key":7,"step_key":9,"actor":"someone@else","by":"x"}`), "ke@x")
	if err != nil || !strings.Contains(ops.calls[0], `"ke@x"`) || strings.Contains(ops.calls[0], "someone") {
		t.Fatalf("calls %v, %v", ops.calls, err)
	}
}

func TestRunbookDispatchRefusesMalformedRequests(t *testing.T) {
	for _, body := range []string{
		`{"action":"nope"}`,
		`{"action":"list_runbooks"}`,
		`{"action":"get_runbook","workspace_name":"jets_ws"}`,
		`{"action":"start_run","workspace_name":"jets_ws"}`,
		`{"action":"get_run"}`,
		`{"action":"accept","run_key":7}`,
		`{"action":"deny","step_key":9}`,
		`{"action":"record","run_key":7}`,
		`{"action":"release"}`,
	} {
		ops := &fakeRunbookOps{}
		_, code, err := runbookDispatch(context.Background(), ops, runbookRequest(t, body), "ke@x")
		if err == nil || code != http.StatusBadRequest || len(ops.calls) != 0 {
			t.Errorf("%s: %d %v, calls %v", body, code, err, ops.calls)
		}
	}
}

// A refusal keeps its status, and a runbook that does not validate carries its
// findings to the client.
func TestRunbookDispatchKeepsRefusals(t *testing.T) {
	refusal := &datatable.RunbookError{Status: http.StatusConflict, Message: "runbook rb does not validate: x",
		Findings: []wsvalidate.Finding{{Severity: wsvalidate.Error, Code: "runbookConfirm", Message: "x"}}}
	ops := &fakeRunbookOps{err: refusal}
	_, code, err := runbookDispatch(context.Background(), ops,
		runbookRequest(t, `{"action":"start_run","workspace_name":"jets_ws","runbook":"rb"}`), "ke@x")
	if code != http.StatusConflict {
		t.Fatalf("code %d", code)
	}
	w := httptest.NewRecorder()
	writeRunbookError(w, code, err)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusConflict || body["error"] == nil || body["findings"] == nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	ops = &fakeRunbookOps{err: context.Canceled}
	if _, code, _ = runbookDispatch(context.Background(), ops, runbookRequest(t, `{"action":"get_run","run_key":1}`), "ke@x"); code != http.StatusInternalServerError {
		t.Fatalf("code %d", code)
	}
}

// The capability the endpoint requires is seeded, for knowledge_engineer.
func TestRunbookExecuteCapabilityIsSeeded(t *testing.T) {
	sql, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sql), "('knowledge_engineer', '"+datatable.RunbookExecuteCapability+"')") {
		t.Fatalf("jets_init_db.sql does not grant %s to knowledge_engineer", datatable.RunbookExecuteCapability)
	}
}

// The executor's operations satisfy the endpoint's interface.
var _ runbookOps = (*datatable.RunbookExecutor)(nil)
