package userflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	actionSchemaPath = "../../jetsclient_ide/src/actions/action.schema.json"
	actionsDir       = "../../jetsclient_ide/src/actions/flows"
)

// The action grammar's Go half, on the same argument as the flow schema's: the
// schema is authored in TypeScript and the claim that Go can enforce it is worth
// nothing until Go has. S.4 wires both checks into SaveWorkspaceFileContent.
func compileActionSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	fh, err := os.Open(actionSchemaPath)
	if err != nil {
		t.Fatalf("opening the emitted action schema: %v", err)
	}
	defer fh.Close()
	doc, err := jsonschema.UnmarshalJSON(fh)
	if err != nil {
		t.Fatalf("parsing the emitted action schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("action.schema.json", doc); err != nil {
		t.Fatalf("adding the schema resource: %v", err)
	}
	schema, err := compiler.Compile("action.schema.json")
	if err != nil {
		t.Fatalf("compiling the emitted action schema: %v", err)
	}
	return schema
}

func TestProofFlowActionsValidate(t *testing.T) {
	schema := compileActionSchema(t)
	names, err := filepath.Glob(filepath.Join(actionsDir, "*.ua.json"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no action documents at %s (err %v)", actionsDir, err)
	}
	// The two proof flows the plan nominates. A third appearing here without the
	// count moving would mean somebody added a document and not a decision.
	if len(names) != 2 {
		t.Fatalf("expected the two proof flows, found %d: %v", len(names), names)
	}
	for _, path := range names {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".ua.json"), func(t *testing.T) {
			fh, err := os.Open(path)
			if err != nil {
				t.Fatalf("opening: %v", err)
			}
			defer fh.Close()
			inst, err := jsonschema.UnmarshalJSON(fh)
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if err := schema.Validate(inst); err != nil {
				t.Errorf("%s does not validate:\n%v", path, err)
			}
		})
	}
}

// TestActionSchemaRejects probes the grammar from the other side. The negative
// suite proper is S.6's; these five are the minimum that makes the positive test
// above mean anything.
func TestActionSchemaRejects(t *testing.T) {
	schema := compileActionSchema(t)
	base, err := os.ReadFile(filepath.Join(actionsDir, "loadFilesUF.ua.json"))
	if err != nil {
		t.Fatalf("reading the base document: %v", err)
	}

	steps := func(doc map[string]any) []any {
		action := doc["actions"].(map[string]any)["lfSyncFileKey"].(map[string]any)
		return action["steps"].([]any)
	}

	cases := map[string]func(doc map[string]any){
		"an unknown step verb": func(doc map[string]any) {
			steps(doc)[0].(map[string]any)["do"] = "launchMissiles"
		},
		"a misspelt field on a step, which must not be ignored": func(doc map[string]any) {
			step := steps(doc)[0].(map[string]any)
			delete(step, "spinner")
			step["spinnner"] = true
		},
		"an endpoint outside ServerEPs": func(doc map[string]any) {
			steps(doc)[0].(map[string]any)["endpoint"] = "/somewhereElse"
		},
		"a value form that does not exist": func(doc map[string]any) {
			steps(doc)[0].(map[string]any)["data"] = map[string]any{
				"rows":   "fields",
				"fields": map[string]any{"x": map[string]any{"fromEnvironment": "HOME"}},
			}
		},
		"an action with no steps at all": func(doc map[string]any) {
			doc["actions"].(map[string]any)["lfSyncFileKey"].(map[string]any)["steps"] = []any{}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(base, &doc); err != nil {
				t.Fatalf("unmarshalling: %v", err)
			}
			mutate(doc)
			encoded, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("marshalling: %v", err)
			}
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(encoded)))
			if err != nil {
				t.Fatalf("re-parsing: %v", err)
			}
			if err := schema.Validate(inst); err == nil {
				t.Error("accepted, and it must not be")
			}
		})
	}
}
