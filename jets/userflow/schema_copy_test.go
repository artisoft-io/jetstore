package userflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The TypeScript sources the copies under schema/ are made from.
var schemaSources = map[string]string{
	flowSchemaFile:   "../../jetsclient_ide/src/userflow/userflow.schema.json",
	actionSchemaFile: "../../jetsclient_ide/src/actions/action.schema.json",
	formSchemaFile:   "../../jetsclient_ide/src/userflow/form.schema.json",
}

// expectedCopy is the source with its owned-asset `$comment` injected.
//
// Deterministic on purpose: the test regenerates it and compares bytes, so
// "matches the source" is a claim someone can check rather than trust. The
// comment is injected here rather than emitted by the TypeScript because the
// *source* is edited — putting a DO-NOT-EDIT notice on it would be false.
func expectedCopy(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(schemaSources[name])
	if err != nil {
		t.Fatalf("reading the source schema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the source schema: %v", err)
	}
	doc["$comment"] = ownedAssetComment(schemaSources[name])
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	return append(out, '\n')
}

// TestSchemaCopiesMatchSource is I-16's guard, and it is a test rather than a
// command because this repository has no CI and `go test ./...` is habitual.
//
//	UPDATE_SCHEMA=1 go test ./jets/userflow/
func TestSchemaCopiesMatchSource(t *testing.T) {
	for name := range schemaSources {
		t.Run(filepath.Base(name), func(t *testing.T) {
			want := expectedCopy(t, name)
			path := filepath.Join("schema", filepath.Base(name))
			if os.Getenv("UPDATE_SCHEMA") == "1" {
				if err := os.WriteFile(path, want, 0o644); err != nil {
					t.Fatalf("writing: %v", err)
				}
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the committed copy: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("%s has drifted from %s.\nRegenerate with:\n"+
					"  UPDATE_SCHEMA=1 go test ./jets/userflow/", path, schemaSources[name])
			}
		})
	}
}

// TestEmbeddedSchemasCompile checks the copies are usable, not merely present —
// a file can match its source and still be something v6 refuses.
func TestEmbeddedSchemasCompile(t *testing.T) {
	for _, name := range []string{flowSchemaFile, actionSchemaFile, formSchemaFile} {
		if _, err := compile(name); err != nil {
			t.Errorf("%s does not compile: %v", name, err)
		}
	}
}

// TestOwnedAssetHeaderIsPresent pins the header itself. The copy matching its
// source is one claim; the file saying what it is when somebody opens it in the
// IDE is another, and the IDE is the only place a reader will meet it.
func TestOwnedAssetHeaderIsPresent(t *testing.T) {
	for name := range schemaSources {
		raw, err := schemaFS.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		comment, _ := doc["$comment"].(string)
		if comment == "" || !contains(comment, "DO NOT EDIT") {
			t.Errorf("%s is missing its jetstore-owned-asset header", name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// TestSchemaFindingsEscapeTheirPointer covers the segment escaping.
//
// **Unexercised by anything realistic**, which is the same reason S.3 needed a
// written case: a *valid* state key cannot contain "/" or "~" because the schema
// forbids it, and this code path exists for invalid documents. A key of `a/b`
// would otherwise emit a pointer that reads as two segments.
func TestSchemaFindingsEscapeTheirPointer(t *testing.T) {
	const doc = `{"schemaVersion":1,"startAtKey":"a","states":{"a/b~c":{"description":"","formConfig":"f","isEnd":true}}}`
	findings := ValidateFlowDocument(doc)
	if len(findings) == 0 {
		t.Fatal("accepted a state with an empty description and an illegal key")
	}
	found := false
	for _, f := range findings {
		if f.Path == "/states/a~1b~0c" {
			found = true
		}
		if strings.Contains(f.Path, "a/b") {
			t.Errorf("pointer segment not escaped: %q", f.Path)
		}
	}
	if !found {
		t.Errorf("expected a finding at /states/a~1b~0c, got %v", findings)
	}
}
