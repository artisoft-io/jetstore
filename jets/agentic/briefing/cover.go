package briefing

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The finding codes Cover emits. They are about the *contract* rather than about
// a record, which is why they are separate from the ones in briefing.go: these
// are read by whoever writes the briefing, before any model has answered.
const (
	// CodeResponseUnreadable is a response_format that is not a json schema this
	// package can walk.
	CodeResponseUnreadable = "briefing_response_schema_not_readable"
	// CodeResponseUnsupported is a json schema construct that makes the set of
	// fields the briefing can carry unknowable here.
	CodeResponseUnsupported = "briefing_response_schema_unsupported"
	// CodeResponseOpen is an object that admits properties it does not name.
	CodeResponseOpen = "briefing_response_object_is_open"
	// CodeResponseFieldHasNoRule is a field the briefing can carry and the
	// provenance schema says nothing about. It is CodeNoRule established before
	// the run rather than during it.
	CodeResponseFieldHasNoRule = "briefing_response_field_has_no_rule"
	// CodeRuleFieldNotInResponse is a rule over a field the briefing cannot
	// carry - a guardrail that can never fire.
	CodeRuleFieldNotInResponse = "briefing_rule_field_not_in_response"
)

// Cover checks the provenance schema against the shape of the briefing it is
// for, and is `AK.2`'s addition to `AK.1`'s checker.
//
// # What it adds, and why the closure was not already enough
//
// `Check`'s closure is total over the document in front of it: a populated field
// no rule covers is a finding. That makes *no briefing field asserts anything
// without a corresponding event* a property of every **run**. It does not make
// it a property of the **briefing**, and the difference is the whole of what a
// supervisor is being asked to trust: under the runtime closure alone, a field
// added to the response schema is caught the first time a model happens to
// populate it, by whoever is reading that record's findings. A field the model
// populates rarely is a guardrail that reports late, on a record nobody chose.
//
// Cover asks the same question of the contract: **every leaf the response schema
// can produce has a rule, and every rule names a leaf the schema can produce.**
// The second half is not symmetry for its own sake - a rule over a field that no
// longer exists is a guardrail that has stopped firing and says nothing about
// having stopped, which is this repository's most-recorded failure shape.
//
// # Why it is not in Check
//
// `Check` runs per record and this runs per document. Putting it in the
// per-record path would walk a json schema for every member in the population to
// re-establish something that cannot have changed since the file was read.
// `ParseSchema` runs it, so the save path and every loader get it; a schema built
// in Go calls it directly.
//
// # What it cannot check
//
// A rule's `Sources` point into the input **entity**, and an entity has no schema
// - it is whatever the rule session asserted, serialised by `EncodeColumnData`.
// So a source selector that names nothing is caught at runtime (the rule fails
// its field) and not here. That asymmetry is a property of the substrate rather
// than a gap in this function: the briefing's shape is declared and the entity's
// is emergent.
func (s *Schema) Cover() []wsvalidate.Finding {
	if len(s.Response) == 0 {
		return nil
	}
	var out []wsvalidate.Finding
	leaves, findings := responseLeaves(s.Response)
	out = append(out, findings...)
	if len(wsvalidate.ErrorsOnly(findings)) > 0 {
		// A schema that could not be walked yields an incomplete leaf set, and
		// comparing rules against it would report fields as uncovered that the
		// walk simply never reached.
		return out
	}

	ruled := make(map[string]int, len(s.Rules))
	for i := range s.Rules {
		sel, err := ParseSelector(s.Rules[i].Field)
		if err != nil {
			// compile() reports the parse failure; nothing to add here.
			continue
		}
		ruled[sel.Canonical()] = i
	}
	for _, leaf := range leaves {
		if _, ok := ruled[leaf]; !ok {
			out = append(out, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     CodeResponseFieldHasNoRule,
				Path:     "/rules",
				Message: fmt.Sprintf(
					"the briefing can carry %q and the provenance schema has no rule for it; "+
						"declare it, or declare it ungrounded with a reason", leaf),
			})
		}
	}
	inResponse := make(map[string]bool, len(leaves))
	for _, leaf := range leaves {
		inResponse[leaf] = true
	}
	for canonical, i := range ruled {
		if inResponse[canonical] {
			continue
		}
		out = append(out, wsvalidate.Finding{
			Severity: wsvalidate.Error,
			Code:     CodeRuleFieldNotInResponse,
			Path:     fmt.Sprintf("/rules/%d/field", i),
			Message: fmt.Sprintf(
				"rule for %q names a field the briefing cannot carry; a rule that can never fire "+
					"is a guardrail that reports nothing and says nothing about reporting nothing",
				s.Rules[i].Field),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// Canonical is the selector as this package understands it, which is what two
// selectors are compared on. `care_gaps[].code` written twice the same way is
// the easy case; the point is that it is the *parse* that is compared, so a
// future spelling of the same address still matches.
func (s Selector) Canonical() string {
	var b strings.Builder
	for i := range s.segments {
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s.segments[i].name)
		if s.segments[i].each {
			b.WriteString("[]")
		}
	}
	return b.String()
}

// responseLeaves walks a json schema and returns a selector for every scalar the
// briefing can carry.
//
// **The subset it accepts is deliberately small and everything outside it is an
// error.** A json schema is a large language and this function's whole value is
// that the leaf set it returns is complete; a construct it skipped would silently
// shrink that set, and a field missing from the set is a field with no rule that
// nobody is told about. So `$ref`, `oneOf`, `anyOf`, `allOf` and a missing `type`
// are refused rather than ignored.
func responseLeaves(raw json.RawMessage) ([]string, []wsvalidate.Finding) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     CodeResponseUnreadable,
			Path:     "/response_format",
			Message:  fmt.Sprintf("the briefing's response_format is not readable as json: %v", err),
		}}
	}
	w := &leafWalk{}
	w.walk(doc, "", "/response_format", false)
	sort.Strings(w.leaves)
	return w.leaves, w.findings
}

type leafWalk struct {
	leaves   []string
	findings []wsvalidate.Finding
}

func (w *leafWalk) refuse(code, path, msg string) {
	w.findings = append(w.findings, wsvalidate.Finding{
		Severity: wsvalidate.Error, Code: code, Path: path, Message: msg,
	})
}

// walk descends one schema node. `selector` is the address of this node in the
// briefing, and `inArray` says that the segment it ends with already carries a
// `[]` - which is how an array of arrays is detected and refused.
func (w *leafWalk) walk(node any, selector, path string, inArray bool) {
	obj, ok := node.(map[string]any)
	if !ok {
		w.refuse(CodeResponseUnsupported, path, "a json schema node must be an object")
		return
	}
	for _, unsupported := range []string{"$ref", "oneOf", "anyOf", "allOf", "not"} {
		if _, present := obj[unsupported]; present {
			w.refuse(CodeResponseUnsupported, path, fmt.Sprintf(
				"%s makes the set of fields the briefing can carry depend on the instance; "+
					"a provenance schema is checked against a fixed set of fields", unsupported))
			return
		}
	}
	kind, _ := obj["type"].(string)
	switch kind {
	case "object":
		props, ok := obj["properties"].(map[string]any)
		if !ok || len(props) == 0 {
			w.refuse(CodeResponseUnsupported, path,
				"an object with no `properties` can carry anything, so nothing can be declared about it")
			return
		}
		if extra, present := obj["additionalProperties"]; !present || extra != false {
			w.refuse(CodeResponseOpen, path,
				"a briefing object must set \"additionalProperties\": false; an open object can carry "+
					"a field no rule covers, and the point of declaring the shape is that it cannot")
		}
		for _, name := range sortedKeys(props) {
			child := selector
			if child != "" {
				child += "."
			}
			child += name
			w.walk(props[name], child, path+"/properties/"+escapePointerToken(name), false)
		}
	case "array":
		if inArray {
			w.refuse(CodeResponseUnsupported, path,
				"an array of arrays has no selector: `[]` addresses every element of one property")
			return
		}
		if selector == "" {
			w.refuse(CodeResponseUnsupported, path,
				"the briefing's response_format must be an object at the root: its fields are what the "+
					"provenance schema names")
			return
		}
		items, present := obj["items"]
		if !present {
			w.refuse(CodeResponseUnsupported, path,
				"an array with no `items` can carry anything, so nothing can be declared about it")
			return
		}
		w.walk(items, selector+"[]", path+"/items", true)
	case "string", "number", "integer", "boolean":
		if selector == "" {
			w.refuse(CodeResponseUnsupported, path,
				"the briefing's response_format must be an object at the root: its fields are what the "+
					"provenance schema names")
			return
		}
		w.leaves = append(w.leaves, selector)
	case "":
		w.refuse(CodeResponseUnsupported, path,
			"a json schema node with no `type` can carry anything, so nothing can be declared about it")
	default:
		w.refuse(CodeResponseUnsupported, path, fmt.Sprintf(
			"type %q is not a shape a briefing field can take", kind))
	}
}
