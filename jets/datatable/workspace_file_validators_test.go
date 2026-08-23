package datatable

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

func TestValidatorForPicksTheMostSpecificSuffix(t *testing.T) {
	cases := map[string]bool{
		"user_flows/loadFilesUF.uf.json":            true,
		"user_flows/loadFilesUF.ua.json":            true,
		"user_flows/loadFilesUF.form.json":          true,
		"table_configs/lfSourceConfigTable.tc.json": true,
		// The plain-JSON files that have always been saved through this path keep
		// their existing behaviour: well-formedness only, no specific validator.
		"workspace_control.json":    false,
		"reports/something.json":    false,
		"pipes_config/x.pc.json":    false, // agentic_ai's row, not yet added
		"user_flows/notes.md":       false,
		"looks_like.uf.json.backup": false,
		"UPPER/LOADFILES.UF.JSON":   true, // case-insensitive, like the check it sits behind
	}
	for name, want := range cases {
		if got := validatorFor(name) != nil; got != want {
			t.Errorf("validatorFor(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestSuffixesAreDistinct guards the rule rather than today's table: longest
// match only means something if two suffixes can overlap, and it is the rule
// that keeps the *next* file type honest — `.tc.json` was the fourth and did not
// need it, which is exactly when a guard like this stops being watched.
func TestSuffixesAreDistinct(t *testing.T) {
	for i, a := range workspaceFileValidators {
		for j, b := range workspaceFileValidators {
			if i != j && strings.HasSuffix(a.suffix, b.suffix) {
				t.Errorf("%q ends with %q; longest-match decides, and that should be deliberate",
					a.suffix, b.suffix)
			}
		}
	}
}

func readGolden(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// TestShippingDocumentsPassTheSaveCheck is the phase's rule at the save path: a
// real configuration that fails means the check is wrong, not the configuration.
func TestShippingDocumentsPassTheSaveCheck(t *testing.T) {
	flows := "../../jetsclient_ide/src/userflow/flows"
	entries, err := os.ReadDir(flows)
	if err != nil {
		t.Fatalf("reading %s: %v", flows, err)
	}
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".uf.json") {
			continue
		}
		content := readGolden(t, flows+"/"+e.Name())
		if findings := validatorFor(e.Name())(content); len(findings) > 0 {
			t.Errorf("%s is refused by the save check: %v", e.Name(), findings)
		}
		checked++
	}
	if checked != 11 {
		t.Errorf("expected the app's eleven flows, checked %d", checked)
	}
}

// TestShippingTablesPassTheSaveCheck is the same rule for the fourth document
// type, and it goes through `validatorFor` rather than calling the validator
// directly — the dispatch is the part that was untested until the mutation pass
// found it (see the header of the file under test).
func TestShippingTablesPassTheSaveCheck(t *testing.T) {
	tables := "../../jetsclient_ide/src/datatable/tables"
	entries, err := os.ReadDir(tables)
	if err != nil {
		t.Fatalf("reading %s: %v", tables, err)
	}
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".tc.json") {
			continue
		}
		content := readGolden(t, tables+"/"+e.Name())
		if findings := validatorFor(e.Name())(content); len(findings) > 0 {
			t.Errorf("%s is refused by the save check: %v", e.Name(), findings)
		}
		checked++
	}
	if checked != 37 {
		t.Errorf("expected the flows' 37 table configurations, checked %d", checked)
	}
}

func TestSaveCheckRejects(t *testing.T) {
	base := readGolden(t, "../../jetsclient_ide/src/userflow/flows/loadFilesUF.uf.json")
	mutate := func(f func(doc map[string]any)) string {
		var doc map[string]any
		if err := json.Unmarshal([]byte(base), &doc); err != nil {
			t.Fatalf("unmarshalling: %v", err)
		}
		f(doc)
		out, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("marshalling: %v", err)
		}
		return string(out)
	}

	t.Run("a misspelt field", func(t *testing.T) {
		content := mutate(func(doc map[string]any) {
			s := doc["states"].(map[string]any)["select_source_config"].(map[string]any)
			delete(s, "defaultNextState")
			s["defaultNexState"] = "select_file_keys"
		})
		findings := ValidateFlowDocumentForTest(content)
		if len(findings) == 0 {
			t.Fatal("accepted, and it must not be")
		}
	})

	t.Run("a transition to a state that does not exist, with its pointer", func(t *testing.T) {
		content := mutate(func(doc map[string]any) {
			doc["states"].(map[string]any)["select_source_config"].(map[string]any)["defaultNextState"] = "typo"
		})
		// Two findings, and only one blocks: breaking the transition also strands
		// `select_file_keys`, which is a warning. The save path acts on
		// ErrorsOnly, so that is what this asserts.
		all := ValidateFlowDocumentForTest(content)
		errs := wsvalidate.ErrorsOnly(all)
		if len(all) != 2 || len(errs) != 1 {
			t.Fatalf("expected one error beside one warning, got %v", all)
		}
		if errs[0].Path != "/states/select_source_config/defaultNextState" {
			t.Errorf("unexpected path %q", errs[0].Path)
		}
	})

	t.Run("the message names the file and every finding", func(t *testing.T) {
		content := mutate(func(doc map[string]any) {
			doc["states"].(map[string]any)["select_source_config"].(map[string]any)["defaultNextState"] = "typo"
		})
		err := describeFindings("user_flows/x.uf.json",
			wsvalidate.ErrorsOnly(ValidateFlowDocumentForTest(content)))
		if !strings.Contains(err.Error(), "user_flows/x.uf.json") ||
			!strings.Contains(err.Error(), "/states/select_source_config/defaultNextState") {
			t.Errorf("message is not usable: %v", err)
		}
	})
}

// TestCheckWorkspaceFileIsTheWiring covers the step SaveWorkspaceFileContent
// actually performs, rather than the validators in isolation.
//
// Mutation testing is why it exists: bypassing the dispatch inside the handler
// broke no test, because the handler needs a database and a token and nothing
// reached it. The check is now a function, and this is that function.
func TestCheckWorkspaceFileIsTheWiring(t *testing.T) {
	good := readGolden(t, "../../jetsclient_ide/src/userflow/flows/loadFilesUF.uf.json")

	t.Run("a valid flow is written", func(t *testing.T) {
		if err := checkWorkspaceFile("user_flows/loadFilesUF.uf.json", good); err != nil {
			t.Errorf("refused a shipping flow: %v", err)
		}
	})

	t.Run("an invalid flow is refused, and the message names the pointer", func(t *testing.T) {
		var doc map[string]any
		if err := json.Unmarshal([]byte(good), &doc); err != nil {
			t.Fatal(err)
		}
		doc["states"].(map[string]any)["select_source_config"].(map[string]any)["defaultNextState"] = "typo"
		out, _ := json.Marshal(doc)
		err := checkWorkspaceFile("user_flows/x.uf.json", string(out))
		if err == nil {
			t.Fatal("accepted, and it must not be")
		}
		if !strings.Contains(err.Error(), "/states/select_source_config/defaultNextState") {
			t.Errorf("message is not usable: %v", err)
		}
	})

	t.Run("the same content under a plain .json name is written", func(t *testing.T) {
		// The dispatch is by suffix, and a file that names no specific validator
		// keeps exactly the behaviour every existing file type has today.
		var doc map[string]any
		_ = json.Unmarshal([]byte(good), &doc)
		doc["states"].(map[string]any)["select_source_config"].(map[string]any)["defaultNextState"] = "typo"
		out, _ := json.Marshal(doc)
		if err := checkWorkspaceFile("reports/whatever.json", string(out)); err != nil {
			t.Errorf("a plain .json should not be flow-validated: %v", err)
		}
	})

	t.Run("malformed json is refused before any validator runs", func(t *testing.T) {
		if err := checkWorkspaceFile("user_flows/x.uf.json", "{ not json"); err == nil {
			t.Error("accepted malformed json")
		} else if !strings.Contains(err.Error(), "not a valid json file") {
			t.Errorf("the well-formedness check should report first: %v", err)
		}
	})

	t.Run("a non-json file is not touched", func(t *testing.T) {
		if err := checkWorkspaceFile("jet_rules/mapping.jr", "this is not json at all"); err != nil {
			t.Errorf("refused a non-json file: %v", err)
		}
	})
}
