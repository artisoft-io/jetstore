package eval

// What a harness sends for one case: the system message, and the instruction
// that describes the hole.
//
// **Moved here from tools/compile_pass at AG.2, 2026-09-05, verbatim apart from
// the exports.** It lived in that tool's `main` while that tool was the only
// caller. The bake-off is a second caller, and its arm 1 is meant to be the
// shape P.1 measured — so a second rendering of the same prompt would make the
// two harnesses incomparable in exactly the way nobody would notice, since both
// would still be sending something reasonable. One rendering, two callers.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/prompt"
)

// SystemPrompt is the system message a config-authoring run sends.
const SystemPrompt = "You are a JetStore Compute Pipes configuration author. " +
	"You answer with one JSON transformation specification and nothing else — no prose, " +
	"no markdown fence, no explanation."

// instructionBudgetShare is how much of the reserved window the instruction may
// take. The rest of the reserve is the answer, and an answer that cannot be
// finished is a case lost to formatting rather than to authoring.
const instructionBudgetShare = 2

// Instruction describes the hole without sending the whole config through it.
//
// **The config does not fit beside the schema and that is not a tuning
// problem.** A held-out config runs to tens of kilobytes and the operator's
// schema already takes most of the window, so the instruction carries the
// enclosing step — its channels, and its siblings reduced to their type — with
// the hole marked. A run that sent the whole document would measure a different
// task, so what was sent is recorded rather than left to be inferred.
//
// extra is appended after the standing text, and is where a caller adds an arm's
// own material — I-180's TypeScript rendering and its worked examples are the
// case this parameter exists for. It is separate from inPrompt because the two
// answer different questions: inPrompt is the JSON Schema, which P.1 measured
// and which moved nothing, and extra is anything else.
func Instruction(c *Case, numCtx int, inPrompt json.RawMessage, extra string) (string, error) {
	// The served window is what the instruction competes for, and what else is
	// in it depends on where the schema goes: as `format` it costs nothing
	// (F113), in the instruction it costs its own size and the budget for the
	// context abstract shrinks by exactly that.
	budgetTokens := prompt.DefaultReserveTokens / instructionBudgetShare
	if numCtx > 0 {
		remaining := numCtx - prompt.EstimateTokens(inPrompt) -
			prompt.EstimateTokens([]byte(extra)) - prompt.DefaultReserveTokens
		if remaining < budgetTokens {
			budgetTokens = remaining
		}
	}
	if budgetTokens < 256 {
		return "", fmt.Errorf("eval: %s: nothing left of the window for an instruction", c.File)
	}

	sketch, err := stepSketch(c)
	if err != nil {
		return "", err
	}
	rendered, kind := sketch, "the enclosing step, with its siblings reduced to their type"
	if prompt.EstimateTokens(rendered) > budgetTokens {
		rendered, err = json.Marshal(SiblingTypes(c))
		if err != nil {
			return "", err
		}
		kind = "the sibling transformation types only — the enclosing step did not fit the window"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "A JetStore Compute Pipes configuration has one transformation removed from an "+
		"`apply` array. Write the transformation that belongs there.\n\n")
	fmt.Fprintf(&b, "File: %s\n", c.File)
	fmt.Fprintf(&b, "Operator type required: %q\n", c.Operator)
	fmt.Fprintf(&b, "Position in the apply array: %d\n\n", c.Hole.Index)
	fmt.Fprintf(&b, "Context (%s):\n%s\n\n", kind, rendered)
	if len(inPrompt) > 0 {
		fmt.Fprintf(&b, "This is the type you must produce, as a JSON Schema:\n\n```json\n%s\n```\n\n",
			inPrompt)
	}
	b.WriteString("Answer with the single transformation object. Its `type` must be exactly the " +
		"operator named above. Reference only channels and columns the context shows.")
	if extra != "" {
		b.WriteString(extra)
	}
	return b.String(), nil
}

// stepSketch is the object holding the `apply` array, with the array replaced
// by its siblings' types and a marker at the hole.
func stepSketch(c *Case) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(c.Context, &doc); err != nil {
		return nil, err
	}
	if len(c.Hole.Path) == 0 {
		return nil, errors.New("the case has no path to a hole")
	}
	node := doc
	for _, s := range c.Hole.Path[:len(c.Hole.Path)-1] {
		switch {
		case s.Key != "":
			m, ok := node.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected an object at %q", s.Key)
			}
			node = m[s.Key]
		default:
			arr, ok := node.([]any)
			if !ok || s.Index >= len(arr) {
				return nil, fmt.Errorf("no element %d on the path to the hole", s.Index)
			}
			node = arr[s.Index]
		}
	}
	step, ok := node.(map[string]any)
	if !ok {
		return nil, errors.New("the hole's enclosing node is not an object")
	}
	out := map[string]any{}
	for k, v := range step {
		if k == c.Hole.Path[len(c.Hole.Path)-1].Key {
			out[k] = SiblingTypes(c)
			continue
		}
		out[k] = v
	}
	return json.MarshalIndent(out, "", "  ")
}

// SiblingTypes is the apply array as a list of operator types with the hole
// marked — the smallest description of where the answer goes.
func SiblingTypes(c *Case) []any {
	var doc any
	if err := json.Unmarshal(c.Context, &doc); err != nil {
		return nil
	}
	holder, err := navigateTo(doc, c.Hole.Path)
	if err != nil {
		return nil
	}
	arr, ok := holder.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(arr)+1)
	for i := 0; i <= len(arr); i++ {
		if i == c.Hole.Index {
			out = append(out, map[string]any{"MISSING — write this one": c.Operator})
		}
		if i < len(arr) {
			t := "?"
			if m, ok := arr[i].(map[string]any); ok {
				if s, ok := m["type"].(string); ok {
					t = s
				}
			}
			out = append(out, map[string]any{"type": t})
		}
	}
	return out
}

func navigateTo(node any, path []Step) (any, error) {
	cur := node
	for _, s := range path {
		if s.Key != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected an object at %q", s.Key)
			}
			cur = m[s.Key]
			continue
		}
		arr, ok := cur.([]any)
		if !ok || s.Index >= len(arr) {
			return nil, fmt.Errorf("no element %d", s.Index)
		}
		cur = arr[s.Index]
	}
	return cur, nil
}
