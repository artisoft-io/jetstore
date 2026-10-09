package eval

import (
	"encoding/json"
	"strings"
	"testing"
)

// These cover the properties two harnesses now depend on: that the instruction
// names the operator and marks the hole, that it does not hand over the answer,
// and that an arm's own material is appended rather than mixed into the
// standing text — which is what makes arm 2 differ from arm 1 in one thing.

func TestInstruction_NamesTheHoleAndWithholdsTheAnswer(t *testing.T) {
	c := caseForTest(t)
	got, err := Instruction(c, 32768, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`Operator type required: "analyze"`,
		"Position in the apply array: 1",
		"MISSING — write this one",
		"t.pc.json",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the instruction does not contain %q:\n%s", want, got)
		}
	}
	// The removed instance is ground truth for a diff and is never sent.
	if strings.Contains(got, `"type": "analyze"`) {
		t.Errorf("the instruction hands over the answer:\n%s", got)
	}
}

func TestInstruction_AppendsAnArmsOwnMaterialLast(t *testing.T) {
	c := caseForTest(t)
	plain, err := Instruction(c, 32768, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	withExtra, err := Instruction(c, 32768, nil, "\n\nEXTRA MATERIAL")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(withExtra, plain) {
		t.Errorf("the standing instruction is not a prefix of the one with extra material;\n%s\n---\n%s",
			plain, withExtra)
	}
	if !strings.HasSuffix(withExtra, "EXTRA MATERIAL") {
		t.Errorf("the extra material is not last:\n%s", withExtra)
	}
}

func TestInstruction_CarriesTheSchemaOnlyWhenAsked(t *testing.T) {
	c := caseForTest(t)
	plain, err := Instruction(c, 32768, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "as a JSON Schema") {
		t.Errorf("arm 1 must not carry the schema in the prompt:\n%s", plain)
	}
	withSchema, err := Instruction(c, 32768, json.RawMessage(`{"type":"object"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withSchema, "as a JSON Schema") {
		t.Errorf("the schema was asked for and is not there:\n%s", withSchema)
	}
}

// A window with nothing left in it is refused rather than truncated: an
// over-budget prompt gets cut by the server, which changes what the model reads
// without saying so.
func TestInstruction_RefusesAWindowWithNoRoom(t *testing.T) {
	c := caseForTest(t)
	if _, err := Instruction(c, 1024, nil, ""); err == nil {
		t.Fatal("expected a refusal when the window leaves nothing for an instruction")
	}
}

func caseForTest(t *testing.T) *Case {
	t.Helper()
	c, err := MakeCase([]byte(twoPipes), Instance{
		File: "t.pc.json", Operator: "analyze",
		Path:  []Step{key("conditional_pipes_config"), at(0), key("apply")},
		Index: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
