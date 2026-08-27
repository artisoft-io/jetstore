// Package userflow holds the Go side of the UserFlow configuration contract.
//
// Task S.1 puts only a test here, and that is the whole point of the package
// existing this early: the schema is authored in TypeScript
// (jetsclient_ide/src/userflow/schema.ts) and emitted as JSON Schema, and the
// claim that Go can enforce it is worth nothing until Go has actually done so.
// If zod emits a construct santhosh-tekuri/jsonschema/v6 cannot compile, that is
// a fact about the schema now, not a surprise waiting at S.4.
//
// S.4 adds the production half — the check in SaveWorkspaceFileContent, beside
// the existing well-formed-JSON check — and ports the reference rules from
// jetsclient_ide/src/userflow/validate.ts, which are the ones JSON Schema cannot
// express.
//
// This test reads the emitted artifact where it is written rather than through a
// copy, so the two languages check one file rather than two readings of it. That
// is deliberate and it does have a cost: //go:embed cannot reach outside its own
// directory, so S.4 will need either a copy made by the build or a schema moved
// under jets/. That choice is S.4's; recording it here so it is a decision and
// not a discovery.
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
	schemaPath = "../../jetsclient_ide/src/userflow/userflow.schema.json"
	// The eleven converted flows are JetStore-owned workspace assets, installed
	// into a workspace by install_workspace_assets. The directory also holds the
	// projections `cpipes-contract templates` writes, and those are UserFlow
	// documents too — validating them here is coverage rather than a mismatch.
	flowsDir = "../workspace_assets/user_flows"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	fh, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("opening the emitted schema: %v", err)
	}
	defer fh.Close()
	doc, err := jsonschema.UnmarshalJSON(fh)
	if err != nil {
		t.Fatalf("parsing the emitted schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("userflow.schema.json", doc); err != nil {
		t.Fatalf("adding the schema resource: %v", err)
	}
	schema, err := compiler.Compile("userflow.schema.json")
	if err != nil {
		t.Fatalf("compiling the emitted schema: %v", err)
	}
	return schema
}

func flowFiles(t *testing.T) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(flowsDir, "*.uf.json"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no flow documents at %s (err %v)", flowsDir, err)
	}
	return names
}

func instance(t *testing.T, path string) any {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer fh.Close()
	inst, err := jsonschema.UnmarshalJSON(fh)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return inst
}

// TestShippingFlowsValidate is the rule the plan states for this phase: a real
// configuration that fails means the schema is wrong, not the configuration.
// These eleven are the flows the Flutter app compiles in today, converted from a
// corpus generated out of the running app rather than transcribed.
func TestShippingFlowsValidate(t *testing.T) {
	schema := compileSchema(t)
	files := flowFiles(t)
	// The app's eleven, plus the three projected templates that share the
	// directory — see flowsDir. A projection is a UserFlow document and passes the
	// same schema; the count is of documents, not of migrations.
	if len(files) != 14 {
		t.Fatalf("expected the app's eleven flows and three projections, found %d", len(files))
	}
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".uf.json"), func(t *testing.T) {
			if err := schema.Validate(instance(t, path)); err != nil {
				t.Errorf("%s does not validate:\n%v", path, err)
			}
		})
	}
}

// TestSchemaRejects probes the schema from the other side. A schema exercised
// only by documents that pass is one nobody has probed; the negative suite
// proper is S.6's, and these four are the minimum that makes this test mean
// something.
func TestSchemaRejects(t *testing.T) {
	schema := compileSchema(t)
	base, err := os.ReadFile(filepath.Join(flowsDir, "loadFilesUF.uf.json"))
	if err != nil {
		t.Fatalf("reading the base document: %v", err)
	}

	cases := map[string]func(doc map[string]any){
		"a misspelt field, which must not be ignored": func(doc map[string]any) {
			states := doc["states"].(map[string]any)
			state := states["select_source_config"].(map[string]any)
			delete(state, "defaultNextState")
			state["defaultNexState"] = "select_file_keys"
		},
		"an end state that also transitions": func(doc map[string]any) {
			states := doc["states"].(map[string]any)
			states["select_file_keys"].(map[string]any)["defaultNextState"] = "select_source_config"
		},
		"a state with nowhere to go": func(doc map[string]any) {
			states := doc["states"].(map[string]any)
			delete(states["select_source_config"].(map[string]any), "defaultNextState")
		},
		"a state key with a space in it": func(doc map[string]any) {
			states := doc["states"].(map[string]any)
			states["not a key"] = states["select_file_keys"]
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(base, &doc); err != nil {
				t.Fatalf("unmarshalling: %v", err)
			}
			mutate(doc)
			// Round-tripping through the library's own reader keeps the number
			// and boolean types identical to the passing case, so a rejection is
			// the mutation's doing and not the decoder's.
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(mustJSON(t, doc)))
			if err != nil {
				t.Fatalf("re-parsing: %v", err)
			}
			if err := schema.Validate(inst); err == nil {
				t.Error("accepted, and it must not be")
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	return string(b)
}
