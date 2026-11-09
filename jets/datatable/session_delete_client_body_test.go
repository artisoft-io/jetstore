package datatable

// The body the client's Delete posts, read by the server's own parsers.
// session_registry Phase 2 (JM-081), 2026-11-08: the client's half is
// jetsclient_ide/src/screens/documents/homeScreen.ua.json (deleteSessions) and
// jetsclient_ide/src/actions/sessionDeletion.ts (previewRequest); this checks
// the two halves agree without a browser.

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// The deleteSessions post step's data, as the action document authors it.
func deleteSessionsPostData(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../jetsclient_ide/src/screens/documents/homeScreen.ua.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Actions map[string]struct {
			Steps []map[string]any `json:"steps"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, step := range doc.Actions["deleteSessions"].Steps {
		if step["do"] == "post" && step["action"] == "delete_sessions" {
			return step["data"].(map[string]any)
		}
	}
	t.Fatal("homeScreen.ua.json: deleteSessions has no post of delete_sessions")
	return nil
}

func TestDeleteSessionsClientBodyParses(t *testing.T) {
	data := deleteSessionsPostData(t)
	// One row (rows "fields"), each field a fromKeyList, which the interpreter
	// evaluates to a JSON array of strings (interpret.ts, unpackToList).
	if data["rows"] != "fields" {
		t.Fatalf(`deleteSessions posts rows %q; this test builds rows "fields"`, data["rows"])
	}
	fields := data["fields"].(map[string]any)
	state := map[string][]any{
		"sessionRegistryTable":      {"1001", "1005"},
		"session_deletion_expected": {"1001", "1005", "2002"},
	}
	row := map[string]any{}
	for name, value := range fields {
		key, ok := value.(map[string]any)["fromKeyList"].(string)
		if !ok {
			t.Fatalf("field %s is %v; this test evaluates fromKeyList only", name, value)
		}
		row[name] = state[key]
	}
	// Through JSON, as the request arrives.
	encoded, _ := json.Marshal([]map[string]any{row})
	var rows []map[string]any
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatal(err)
	}
	ids, err := sessionIDsOf(rows)
	if err != nil || !slices.Equal(ids, []string{"1001", "1005"}) {
		t.Errorf("sessionIDsOf = %v, %v; want the selection", ids, err)
	}
	expected, err := expectedOf(rows)
	if err != nil || !slices.Equal(expected, []string{"1001", "1005", "2002"}) {
		t.Errorf("expectedOf = %v, %v; want the previewed ids", expected, err)
	}
}

// previewSessionDeletion's request: one row per selected session, {session_id}.
func TestPreviewClientBodyParses(t *testing.T) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(`[{"session_id":"1001"},{"session_id":"1005"}]`), &rows); err != nil {
		t.Fatal(err)
	}
	ids, err := sessionIDsOf(rows)
	if err != nil || !slices.Equal(ids, []string{"1001", "1005"}) {
		t.Errorf("sessionIDsOf = %v, %v", ids, err)
	}
}
