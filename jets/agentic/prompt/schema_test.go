package prompt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const toyDoc = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$defs": {
    "Root":   {"type":"object","properties":{"ch":{"$ref":"#/$defs/Channel"}}},
    "Channel":{"type":"object","properties":{"name":{"$ref":"#/$defs/Name"}}},
    "Name":   {"type":"string"},
    "Unused": {"type":"object","properties":{"x":{"$ref":"#/$defs/AlsoUnused"}}},
    "AlsoUnused": {"type":"integer"}
  }
}`

func TestSubschema_KeepsTheClosureAndDropsTheRest(t *testing.T) {
	out, err := Subschema([]byte(toyDoc), "Root")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["$ref"] != "#/$defs/Root" {
		t.Errorf("root $ref = %v; the document must be rooted at the addressed entry, "+
			"because `format` takes a document and not a reference into one", got["$ref"])
	}
	defs := got["$defs"].(map[string]any)
	for _, want := range []string{"Root", "Channel", "Name"} {
		if _, ok := defs[want]; !ok {
			t.Errorf("%s is missing; the document would not compile", want)
		}
	}
	for _, unwanted := range []string{"Unused", "AlsoUnused"} {
		if _, ok := defs[unwanted]; ok {
			t.Errorf("%s was kept; the point of the closure is to leave it out", unwanted)
		}
	}
}

func TestSubschema_RefusesWhatItCannotProduce(t *testing.T) {
	t.Run("unknown entry names candidates", func(t *testing.T) {
		_, err := Subschema([]byte(toyDoc), "Chanel")
		if err == nil {
			t.Fatal("expected an unknown entry to be refused")
		}
		if !strings.Contains(err.Error(), "Channel") {
			t.Errorf("the error does not suggest the near miss: %v", err)
		}
	})
	t.Run("dangling ref", func(t *testing.T) {
		_, err := Subschema([]byte(`{"$defs":{"A":{"$ref":"#/$defs/Missing"}}}`), "A")
		if err == nil {
			t.Fatal("expected a dangling reference to be refused")
		}
		if !strings.Contains(err.Error(), "Missing") {
			t.Errorf("the error does not name the dangling reference: %v", err)
		}
	})
	t.Run("no defs at all", func(t *testing.T) {
		if _, err := Subschema([]byte(`{"type":"object"}`), "A"); err == nil {
			t.Fatal("expected a document with no $defs to be refused")
		}
	})
}

func TestFits_RefusesAnOverBudgetSchema(t *testing.T) {
	small := []byte(strings.Repeat("x", 4000)) // ~1k tokens
	if err := Fits(small, 0, 0); err != nil {
		t.Errorf("a 1k-token schema should fit in 32k: %v", err)
	}
	big := []byte(strings.Repeat("x", 4*30000)) // ~30k tokens
	err := Fits(big, 0, 0)
	if err == nil {
		t.Fatal("a 30k-token schema leaves 2k of a 32k window and must be refused")
	}
	// The error has to be actionable, not just a refusal.
	if !strings.Contains(err.Error(), "narrower") {
		t.Errorf("the error does not say what to do about it: %v", err)
	}
}

// The measurement the package exists because of, asserted against the real
// emitted contract rather than described in a comment. It is skipped when the
// schema is not present — tools/ is not copied into any image — so this is a
// source-tree check, which is where the claim needs to hold.
func TestAgainstTheRealContract_TransformationSpecDoesNotFit(t *testing.T) {
	doc := realSchema(t)

	for _, tc := range []struct {
		def      string
		wantFits bool
		why      string
	}{
		{"ChannelSpec", true, "a channel spec is small and comfortable"},
		{"TransformationColumnSpec", true, "decision 9's fragment granularity, and the unit that works today"},
		{"TransformationSpec", false, "§3.3 proposes constraining to this, and it does not fit the window"},
	} {
		t.Run(tc.def, func(t *testing.T) {
			sub, err := Subschema(doc, tc.def)
			if err != nil {
				t.Skipf("%s is not in this contract: %v", tc.def, err)
			}
			err = Fits(sub, 0, 0)
			gotFits := err == nil
			if gotFits != tc.wantFits {
				t.Errorf("%s: fits = %v, want %v (%s). ~%d tokens of %d, %d reserved",
					tc.def, gotFits, tc.wantFits, tc.why,
					EstimateTokens(sub), DefaultContextTokens, DefaultReserveTokens)
			}
			t.Logf("%s: %d bytes, ~%d tokens", tc.def, len(sub), EstimateTokens(sub))
		})
	}
}

// And the narrowing is real even where it is not sufficient: addressing one
// entry must always be smaller than the whole document.
func TestAgainstTheRealContract_ClosureIsSmallerThanTheWhole(t *testing.T) {
	doc := realSchema(t)
	sub, err := Subschema(doc, "TransformationSpec")
	if err != nil {
		t.Skipf("TransformationSpec absent: %v", err)
	}
	if len(sub) >= len(doc) {
		t.Errorf("the closure is %d bytes of a %d-byte document; it is not narrowing anything",
			len(sub), len(doc))
	}
}

func realSchema(t *testing.T) []byte {
	t.Helper()
	// From the package directory up to the repo root, then to the contract.
	path := filepath.Join("..", "..", "..", "tools", "cpipes_contract", "cpipes_schema.json")
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the emitted cpipes schema is not present at %s: %v", path, err)
	}
	return doc
}
