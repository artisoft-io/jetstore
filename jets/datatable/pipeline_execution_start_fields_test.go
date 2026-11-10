package datatable

// A submitted start names the fields the start itself reads from the request:
// file_key, client and user_email. A request missing one, or carrying one that is
// not a string, is refused with 400 before anything is written, rather than
// inserting the run and then panicking the handler when the start reads it.
// Uses the start fixture of session_delete_test.go, so it needs JETS_TEST_DSN and
// is skipped otherwise; see that file's header.

import (
	"net/http"
	"strings"
	"testing"
)

// submit starts a run as the UI does, with the row edited, and reports a panic
// of the handler rather than dying of it.
func (f *startFixture) submit(sessionId string, edit func(row map[string]any)) (code int, err error, panicked any) {
	row := map[string]any{
		"pipeline_config_key": 1, "main_input_registry_key": f.registryKey, "main_input_file_key": "k",
		"merged_input_registry_keys": "{}", "file_key": "k",
		"client": "c1", "process_name": "proc", "main_object_type": "Claim",
		"session_id": sessionId, "source_period_key": 1, "status": "submitted", "user_email": "u@example.com",
	}
	edit(row)
	action := &DataTableAction{
		Action:      "insert_rows",
		FromClauses: []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
		Data:        []map[string]any{row},
	}
	results := map[string]any{}
	defer func() {
		panicked = recover()
	}()
	_, code, err = f.ctx.InsertPipelineExecutionStatus(action, 0, &results, "")
	return
}

func TestSubmittedStartMissingAFieldIsRefused(t *testing.T) {
	f := newStartFixture(t)
	// DevMode skips the throttling, which is not this test's: the guard must hold
	// on either path, and it is checked before either.
	f.ctx.DevMode = true
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"no file_key", func(r map[string]any) { delete(r, "file_key") }},
		{"nil file_key", func(r map[string]any) { r["file_key"] = nil }},
		{"no client", func(r map[string]any) { delete(r, "client") }},
		{"numeric client", func(r map[string]any) { r["client"] = 7 }},
		{"no user_email", func(r map[string]any) { delete(r, "user_email") }},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sessionId := "run" + string(rune('a'+i))
			code, err, panicked := f.submit(sessionId, c.edit)
			if panicked != nil {
				t.Fatalf("the handler panicked: %v", panicked)
			}
			if code != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), "missing") {
				t.Errorf("answered %d, %v; want 400 naming the missing field", code, err)
			}
			if n := f.runs(t, sessionId); n != 0 {
				t.Errorf("%d runs inserted for a refused start, want 0", n)
			}
		})
	}
}

// A pending row is not started by the request, so it does not need the fields
// the start reads: the guard leaves it alone.
func TestPendingStartWithoutFileKeyIsUnchanged(t *testing.T) {
	f := newStartFixture(t)
	code, err, panicked := f.submit("runp", func(r map[string]any) {
		r["status"] = "pending"
		delete(r, "file_key")
	})
	if code != http.StatusOK || err != nil {
		t.Fatalf("a pending start without file_key answered %d, %v (panic: %v)", code, err, panicked)
	}
	if n := f.runs(t, "runp"); n != 1 {
		t.Errorf("%d runs inserted, want 1", n)
	}
}
