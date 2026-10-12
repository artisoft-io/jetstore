package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A probe that a faithful model fails is a probe that measures nothing, so
// every 'rules' case is run against the answer a perfect pass-through would
// give — the document from its own prompt, sent as an object and as a string —
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
		for _, args := range []string{`{"rules": ` + doc + `}`, `{"rules": ` + string(quoted) + `}`} {
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
