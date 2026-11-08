package userflow

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The Home screen's bundled action document, against the copy of the action
// schema the server embeds. session_registry Phase 2 (JM-081), 2026-11-08: its
// deleteSessions action is the first to use confirm's detailFromKey and the two
// server actions preview_session_deletion and delete_sessions, so it is the
// document that shows the Go half of the grammar admits them.
const homeScreenActions = "../../jetsclient_ide/src/screens/documents/homeScreen.ua.json"

func validateActionDocument(t *testing.T, doc map[string]any) error {
	t.Helper()
	schema, err := compile(actionSchemaFile)
	if err != nil {
		t.Fatalf("compiling the embedded action schema: %v", err)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	return schema.Validate(inst)
}

func readHomeScreenActions(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(homeScreenActions)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestHomeScreenActionsValidate(t *testing.T) {
	if err := validateActionDocument(t, readHomeScreenActions(t)); err != nil {
		t.Fatalf("homeScreen.ua.json does not validate: %v", err)
	}
}

func TestConfirmDetailFromKeyIsAKey(t *testing.T) {
	doc := readHomeScreenActions(t)
	steps := doc["actions"].(map[string]any)["deleteSessions"].(map[string]any)["steps"].([]any)
	confirm := steps[1].(map[string]any)
	if confirm["do"] != "confirm" || confirm["detailFromKey"] == nil {
		t.Fatalf("deleteSessions' second step is not the confirm with a detail: %v", confirm)
	}
	confirm["detailFromKey"] = "not a key!"
	if err := validateActionDocument(t, doc); err == nil {
		t.Error("a detailFromKey that is not an identifier was accepted")
	}
}
