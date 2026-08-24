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
	coverageDir      = "../../jetsclient_ide/src/actions/coverage"
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
	// The migrated flows: Phase 2's two proof flows plus each one track F lands.
	// A document appearing here without this count moving would mean somebody
	// added a document and not a decision — which is why the number is written
	// down rather than derived from the glob.
	//
	// 3 as of F.1 (mapFileUF, 2026-08-23); 5 as of F.2 (loadConfigUF and
	// workspacePullUF, 2026-08-23 — one task, because they are one delegate file);
	// 6 as of F.3 (clientRegistryUF, 2026-08-23); 7 as of F.4 (startPipelineUF,
	// 2026-08-24); 8 as of F.5 (homeFiltersUF, 2026-08-24); 9 as of F.6
	// (pipelineConfigUF, 2026-08-24).
	const migratedFlows = 9
	if len(names) != migratedFlows {
		t.Fatalf("expected %d migrated flows, found %d: %v", migratedFlows, len(names), names)
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

// TestCoverageActionsValidate is the other half of S.2a's exit condition: not
// "the grammar can express the five arms we built" but "the grammar can express
// all of them". The documents here carry the arms of the flows track F has not
// reached, wired to no flow and no screen — see coverage.test.ts for what that
// does and does not prove.
//
// **The count falls as track F lands flows, and it falling is the point.** Seven
// until F.2 re-partitioned workspacePullUF.ua.json into two runtime documents
// and deleted it; six then, five since F.3 did the same for clientRegistryUF,
// four since F.4 did it for startPipelineUF, three since F.5 did it for
// homeFiltersUF, and two since F.6 did it for pipelineConfigUF. F.9 retires
// whatever is left — `fileMappingUF` and `sourceConfigUF`, which are F.8 and
// F.7.
func TestCoverageActionsValidate(t *testing.T) {
	schema := compileActionSchema(t)
	names, err := filepath.Glob(filepath.Join(coverageDir, "*.ua.json"))
	if err != nil || len(names) != 2 {
		t.Fatalf("expected two coverage documents at %s, found %d (err %v)",
			coverageDir, len(names), err)
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
