package prompt

import (
	"encoding/json"
	"strings"
	"testing"
)

// The renderer's job is to say the same thing the JSON Schema says in a fifth
// of the tokens, so the tests are about what a model would read: which fields
// are required, what a discriminated union's legal tokens are, and that nothing
// is silently dropped.

func TestAsTypeScriptRendersRequiredAndOptional(t *testing.T) {
	doc := []byte(`{"$ref":"#/$defs/Step","$defs":{"Step":{"type":"object",
		"description":"One step.\nSecond line is dropped.",
		"properties":{"name":{"type":"string"},"count":{"type":"integer"},"on":{"type":"boolean"}},
		"required":["name"]}}}`)
	got, err := AsTypeScript(doc, "Step")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"// One step.",
		"type Step = { count?: number; name: string; on?: boolean };",
		"// Produce one value of type Step.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Second line") {
		t.Errorf("the description's second line should not be rendered:\n%s", got)
	}
}

func TestAsTypeScriptRendersUnionsAndEnums(t *testing.T) {
	doc := []byte(`{"$ref":"#/$defs/Spec","$defs":{
		"Spec":{"type":"object","properties":{
			"type":{"enum":["sort","filter"]},
			"mode":{"const":"strict"},
			"who":{"oneOf":[{"$ref":"#/$defs/Named"},{"type":"null"}]},
			"tags":{"type":"array","items":{"type":"string"}},
			"free":{"type":"object"}},
			"required":["type"]},
		"Named":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}}}`)
	got, err := AsTypeScript(doc, "Spec")
	if err != nil {
		t.Fatal(err)
	}
	// The discriminator's legal tokens are the whole reason a model can pick
	// one: I-41 measured a prompt that named class names instead, and the model
	// dutifully wrote a class name into `type`.
	for _, want := range []string{
		`type: "sort" | "filter"`,
		`mode?: "strict"`,
		"who?: Named | null",
		"tags?: string[]",
		"free?: Record<string, unknown>",
		"type Named = { id: string };",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q:\n%s", want, got)
		}
	}
}

// **The rendering must be stable across runs**, because the whole point of an
// arm is that it differs from the other arm in one thing. Go's map iteration
// order is random, so a renderer that walked a map without sorting would send a
// different prompt on every call and the comparison would be measuring that.
func TestAsTypeScriptIsDeterministic(t *testing.T) {
	doc := []byte(`{"$ref":"#/$defs/A","$defs":{
		"A":{"type":"object","properties":{"z":{"type":"string"},"m":{"type":"string"},"a":{"type":"string"}}},
		"C":{"type":"string"},"B":{"type":"string"}}}`)
	first, err := AsTypeScript(doc, "A")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := AsTypeScript(doc, "A")
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatalf("rendering %d differs:\n%s\n---\n%s", i, first, again)
		}
	}
	if got := strings.Index(first, "type B"); got < 0 || got > strings.Index(first, "type C") {
		t.Errorf("definitions are not in sorted order:\n%s", first)
	}
}

// A schema document with no $defs is a caller error rather than an empty
// rendering: an empty prompt region would look like a schema that says nothing.
func TestAsTypeScriptRefusesADocumentWithNoDefs(t *testing.T) {
	if _, err := AsTypeScript([]byte(`{"type":"object"}`), "X"); err == nil {
		t.Fatal("expected an error for a document with no $defs")
	}
	if _, err := AsTypeScript([]byte(`not json`), "X"); err == nil {
		t.Fatal("expected an error for a document that is not JSON")
	}
}

// **The rendering is far smaller than the JSON Schema and that is the reason it
// exists** (I-41 measured 18 to 20% on two real bundles). This asserts the
// direction on a real sub-schema rather than the ratio, which is a property of
// the contract and would make the test a tripwire on unrelated edits.
func TestAsTypeScriptIsSmallerThanTheSchema(t *testing.T) {
	doc := []byte(`{"$ref":"#/$defs/Outer","$defs":{
		"Outer":{"type":"object","description":"An outer thing with a long description that goes on.",
			"properties":{"inner":{"$ref":"#/$defs/Inner"},"name":{"type":"string"}},"required":["inner"]},
		"Inner":{"type":"object","description":"An inner thing.",
			"properties":{"a":{"type":"string"},"b":{"type":"integer"},"c":{"type":"boolean"}},
			"required":["a","b"]}}}`)
	got, err := AsTypeScript(doc, "Outer")
	if err != nil {
		t.Fatal(err)
	}
	var compact json.RawMessage
	if err := json.Unmarshal(doc, &compact); err != nil {
		t.Fatal(err)
	}
	if len(got) >= len(compact) {
		t.Errorf("the TypeScript rendering is %d bytes against %d of JSON Schema; it is meant to be smaller",
			len(got), len(compact))
	}
}
