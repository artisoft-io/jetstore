package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A probe that a faithful model fails is a probe that measures nothing, so
// every 'rules' case is run against the answers a perfect pass-through would
// give — the document's fields as the arguments, the whole document, and the
// document as a string —
// and must score intact; and an answer with one term moved must not.
func TestRulesProbesScoreAFaithfulAnswerIntact(t *testing.T) {
	var cf caseFile
	if err := json.Unmarshal(casesJSON, &cf); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cf.Cases {
		if c.Arm != "rules" {
			continue
		}
		n++
		doc := c.Prompt[strings.Index(c.Prompt, "{"):]
		quoted, _ := json.Marshal(doc)
		var parsed struct {
			Imports json.RawMessage `json:"imports"`
			Rules   json.RawMessage `json:"rules"`
		}
		if err := json.Unmarshal([]byte(doc), &parsed); err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		flat := `{"rules": ` + string(parsed.Rules)
		if len(parsed.Imports) > 0 {
			flat += `, "imports": ` + string(parsed.Imports)
		}
		flat += `}`
		for _, args := range []string{
			flat, // the canonical form: the document's fields are the arguments
			`{"rules": ` + strings.TrimSuffix(strings.TrimSpace(string(parsed.Rules)), "]") + importsElement(parsed.Imports) + `]}`,
			`{"rules": ` + doc + `}`,
			`{"rules": ` + string(quoted) + `, "file_name": "x.jr"}`,
		} {
			if missing, present := payloadMissing(json.RawMessage(args), c.Fidelity); !present || len(missing) > 0 {
				t.Errorf("%s: a faithful answer scores as missing %v (present %t)", c.ID, missing, present)
			}
		}
		moved := strings.Replace(doc, `"subject": "?`, `"subject": "?moved_`, 1)
		if missing, _ := payloadMissing(json.RawMessage(`{"rules": `+moved+`}`), c.Fidelity); len(missing) == 0 {
			t.Errorf("%s: an answer with a renamed variable scores intact", c.ID)
		}
	}
	if n != 6 {
		t.Errorf("%d rules probes, want the six that mirror the content arm's", n)
	}
}

// importsElement is the array form's last-element reading of the imports.
func importsElement(imports json.RawMessage) string {
	if len(imports) == 0 {
		return ""
	}
	return `, {"imports": ` + string(imports) + `}`
}
