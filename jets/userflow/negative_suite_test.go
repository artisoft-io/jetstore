package userflow

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

const negativeSuitePath = "../../jetsclient_ide/src/userflow/negative_suite.json"

type negativeCase struct {
	Name     string          `json:"name"`
	Class    string          `json:"class"`
	Document string          `json:"document"`
	Expect   string          `json:"expect"`
	By       string          `json:"by"`
	Content  json.RawMessage `json:"content"`
}

type negativeSuite struct {
	Comment string         `json:"comment"`
	Cases   []negativeCase `json:"cases"`
}

func loadNegativeSuite(t *testing.T) negativeSuite {
	t.Helper()
	raw, err := os.ReadFile(negativeSuitePath)
	if err != nil {
		t.Fatalf("reading the negative suite: %v", err)
	}
	var suite negativeSuite
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatalf("parsing the negative suite: %v", err)
	}
	return suite
}

// validatorsByDocument maps a case's document type to the validator the save
// path would dispatch to for it. Keeping this here rather than switching on a
// suffix keeps the suite readable — a case says "flow", not "x.uf.json".
var validatorsByDocument = map[string]func(string) []wsvalidate.Finding{
	"flow":   ValidateFlowDocument,
	"action": ValidateActionDocument,
	"form":   ValidateFormDocument,
}

// TestNegativeSuite is S.6, run where it matters: **in Go, against the emitted
// schemas, through the same validators the save path calls.**
//
// The rule the plan takes from the cpipes side: *a negative that validates is a
// hole, not a pass.* The three `valid` cases are the other half — a suite whose
// base does not pass is a suite that proves nothing, because everything would
// fail for the same uninteresting reason.
func TestNegativeSuite(t *testing.T) {
	suite := loadNegativeSuite(t)
	if len(suite.Cases) < 20 {
		t.Fatalf("suite has shrunk to %d cases; that is a deletion, not a pass", len(suite.Cases))
	}
	for _, tc := range suite.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			validate, ok := validatorsByDocument[tc.Document]
			if !ok {
				t.Fatalf("unknown document type %q", tc.Document)
			}
			findings := wsvalidate.ErrorsOnly(validate(string(tc.Content)))

			switch tc.Expect {
			case "valid":
				if len(findings) != 0 {
					t.Errorf("the %s base must validate, got %v", tc.Document, findings)
				}
			case "invalid":
				if len(findings) == 0 {
					t.Errorf("accepted, and it must not be — this is a hole in the %s schema",
						tc.Document)
				}
			default:
				t.Fatalf("unknown expect %q", tc.Expect)
			}
		})
	}
}

// TestNegativeSuiteLayers checks the `by` column rather than trusting it.
//
// **This is the addition to the cpipes pattern and the reason it is worth
// having.** This side validates in two layers, and "the document is rejected" is
// a weaker claim than "the layer that should catch it does". A case marked
// `schema` that only the reference checks catch means the schema has a hole that
// the reference layer happens to cover — which would hold here and not in the
// browser, where a document is parsed before anything walks it.
//
// The reverse is fine and stronger: a `reference` case the schema also rejects
// simply means the schema is tighter than the suite assumed.
func TestNegativeSuiteLayers(t *testing.T) {
	suite := loadNegativeSuite(t)
	for _, tc := range suite.Cases {
		if tc.Expect != "invalid" || tc.By != "schema" {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			// Schema only: no reference checks, no policy.
			var file string
			switch tc.Document {
			case "flow":
				file = flowSchemaFile
			case "action":
				file = actionSchemaFile
			case "form":
				file = formSchemaFile
			}
			if len(validateAgainst(file, string(tc.Content))) == 0 {
				t.Errorf("the schema accepts this; the suite says the schema should catch it")
			}
		})
	}
}

// TestNegativeSuiteCoversEveryClass guards the suite's shape rather than its
// size. A suite that grows only in the class someone last thought about stops
// probing the others, and the classes are the interesting part.
func TestNegativeSuiteCoversEveryClass(t *testing.T) {
	suite := loadNegativeSuite(t)
	seen := map[string]int{}
	documents := map[string]int{}
	for _, tc := range suite.Cases {
		seen[tc.Class]++
		documents[tc.Document]++
	}
	for _, class := range []string{
		"sanity", "unknown-type", "missing-required", "inapplicable-field",
		"wrong-discriminator", "invented-field", "value-range", "reference",
	} {
		if seen[class] == 0 {
			t.Errorf("no case in class %q", class)
		}
	}
	// And every document type is probed, not just the one with the most fields.
	for _, doc := range []string{"flow", "action", "form"} {
		if documents[doc] < 3 {
			t.Errorf("document %q has only %d cases", doc, documents[doc])
		}
	}
}
