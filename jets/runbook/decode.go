package runbook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The fields each step type may carry, besides the common ones. Decoding is
// strict against this table: a field a type does not have is an error, so a
// misspelt `confirm` or a `max_count` on a query is never silently ignored.
var commonStepFields = []string{"id", "type", "description", "when", "on_fail"}

var stepFields = map[StepType][]string{
	StepQuery:          {"sql", "returns", "bind", "timeout", "max_rows"},
	StepDDL:            {"sql", "bind", "timeout", "preview", "confirm"},
	StepAssert:         {"condition", "message"},
	StepWait:           {"sql", "returns", "bind", "timeout", "max_rows", "until", "interval", "deadline", "max_count"},
	StepRepeat:         {"steps", "until", "max_count", "interval"},
	StepPause:          {"message"},
	StepManual:         {"instruction", "records"},
	StepStartPending:   {},
	StepStartPipeline:  {"row", "confirm"},
	StepPutSchemaEvent: {"event", "key", "confirm"},
	StepCompile:        {"confirm", "deadline"},
}

// StepTypes lists the step vocabulary in a stable order.
func StepTypes() []StepType {
	return []StepType{StepQuery, StepDDL, StepAssert, StepWait, StepRepeat, StepPause,
		StepManual, StepStartPending, StepStartPipeline, StepPutSchemaEvent, StepCompile}
}

var runbookFields = []string{"format", "runbook", "description", "parameters", "steps"}
var parameterFields = []string{"type", "description", "default"}

// Decode reads a runbook file strictly. It returns the runbook — as much of it
// as could be read — and the findings of decoding: content that is not a JSON
// object, an unknown field, a field of the wrong JSON type, a condition
// outside the grammar. It never panics. The runbook is nil only when the
// content is not a JSON object at all.
//
// Decode does not check the rules of the format (ids, refs, caps, the ddl
// confirmation); Validate does, and Load does both.
func Decode(content []byte) (*Runbook, []wsvalidate.Finding) {
	var fs findings
	top := objectOf(content, "", "a runbook is a JSON object", &fs)
	if top == nil {
		return nil, fs
	}
	rb := &Runbook{}
	checkKeys(top, runbookFields, "", "the runbook", &fs)
	decodeInto(top, "format", "", &rb.Format, "an integer", &fs)
	decodeInto(top, "runbook", "", &rb.Runbook, "a string", &fs)
	decodeInto(top, "description", "", &rb.Description, "a string", &fs)

	if raw, ok := top["parameters"]; ok {
		pp := pointer("", "parameters")
		params := objectOf(raw, pp, "\"parameters\" is an object of name to {\"type\": ...}", &fs)
		if params != nil {
			rb.Parameters = make(map[string]Parameter, len(params))
			for _, name := range sortedKeys(params) {
				p := pointer(pp, name)
				obj := objectOf(params[name], p, "a parameter is an object such as {\"type\": \"timestamp\"}", &fs)
				var par Parameter
				if obj != nil {
					checkKeys(obj, parameterFields, p, "a parameter", &fs)
					decodeInto(obj, "type", p, &par.Type, "a string", &fs)
					decodeInto(obj, "description", p, &par.Description, "a string", &fs)
					if d, ok := obj["default"]; ok {
						par.Default = append(json.RawMessage(nil), d...)
					}
				}
				rb.Parameters[name] = par
			}
		}
	}

	if raw, ok := top["steps"]; ok {
		rb.Steps = decodeSteps(raw, pointer("", "steps"), &fs)
	}
	return rb, fs
}

func decodeSteps(raw json.RawMessage, ptr string, fs *findings) []*Step {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		fs.errorf(CodeJSON, ptr, "\"steps\" is an array of steps")
		return nil
	}
	steps := make([]*Step, 0, len(list))
	for i, r := range list {
		steps = append(steps, decodeStep(r, fmt.Sprintf("%s/%d", ptr, i), fs))
	}
	return steps
}

func decodeStep(raw json.RawMessage, ptr string, fs *findings) *Step {
	s := &Step{}
	obj := objectOf(raw, ptr, "a step is a JSON object", fs)
	if obj == nil {
		return s
	}
	decodeInto(obj, "id", ptr, &s.ID, "a string", fs)
	var typ string
	decodeInto(obj, "type", ptr, &typ, "a string", fs)
	s.Type = StepType(typ)

	allowed := append([]string(nil), commonStepFields...)
	what := fmt.Sprintf("a %s step", typ)
	if extra, ok := stepFields[s.Type]; ok {
		allowed = append(allowed, extra...)
	} else {
		// An unknown type is Validate's to report; meanwhile accept any field
		// some type has, so a misspelt field is still caught.
		seen := map[string]bool{}
		for _, fields := range stepFields {
			for _, f := range fields {
				if !seen[f] {
					seen[f] = true
					allowed = append(allowed, f)
				}
			}
		}
		what = "any step"
	}
	checkKeys(obj, allowed, ptr, what, fs)

	decodeInto(obj, "description", ptr, &s.Description, "a string", fs)
	decodeInto(obj, "on_fail", ptr, &s.OnFail, "a string", fs)
	decodeInto(obj, "sql", ptr, &s.SQL, "a string", fs)
	decodeInto(obj, "returns", ptr, &s.Returns, "an object of column to type", fs)
	decodeInto(obj, "bind", ptr, &s.Bind, "an object of name to ref", fs)
	decodeInto(obj, "timeout", ptr, &s.Timeout, "a duration string such as \"5m\"", fs)
	decodeInto(obj, "max_rows", ptr, &s.MaxRows, "an integer", fs)
	decodeInto(obj, "preview", ptr, &s.Preview, "a boolean", fs)
	decodeInto(obj, "confirm", ptr, &s.Confirm, "a boolean", fs)
	decodeInto(obj, "message", ptr, &s.Message, "a string", fs)
	decodeInto(obj, "interval", ptr, &s.Interval, "a duration string such as \"30s\"", fs)
	decodeInto(obj, "deadline", ptr, &s.Deadline, "a duration string such as \"1h\"", fs)
	decodeInto(obj, "max_count", ptr, &s.MaxCount, "an integer", fs)
	decodeInto(obj, "instruction", ptr, &s.Instruction, "a string", fs)
	decodeInto(obj, "records", ptr, &s.Records, "an object of name to type", fs)
	decodeInto(obj, "key", ptr, &s.Key, "a string", fs)

	for _, f := range []struct {
		key string
		dst **Condition
	}{{"when", &s.When}, {"condition", &s.Condition}, {"until", &s.Until}} {
		if r, ok := obj[f.key]; ok {
			*f.dst = parseCondition(r, pointer(ptr, f.key), fs)
		}
	}
	if r, ok := obj["steps"]; ok {
		s.Steps = decodeSteps(r, pointer(ptr, "steps"), fs)
	}
	if r, ok := obj["row"]; ok {
		p := pointer(ptr, "row")
		row := objectOf(r, p, "\"row\" is an object of column to a literal or {\"from\": ref}", fs)
		if row != nil {
			s.Row = make(map[string]RowValue, len(row))
			for _, col := range sortedKeys(row) {
				s.Row[col] = parseRowValue(row[col], pointer(p, col), fs)
			}
		}
	}
	if r, ok := obj["event"]; ok {
		p := pointer(ptr, "event")
		ev := objectOf(r, p, "\"event\" is {\"from\": ref}", fs)
		if ev != nil {
			s.Event = &FromRef{}
			checkKeys(ev, []string{"from"}, p, "\"event\"", fs)
			decodeInto(ev, "from", p, &s.Event.From, "a ref string", fs)
		}
	}
	return s
}

// parseRowValue reads a start_pipeline column value: a scalar literal, or
// {"from": ref}.
func parseRowValue(raw json.RawMessage, ptr string, fs *findings) RowValue {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			fs.errorf(CodeJSON, ptr, "not a JSON object")
			return RowValue{}
		}
		checkKeys(obj, []string{"from"}, ptr, "a row value", fs)
		var v RowValue
		decodeInto(obj, "from", ptr, &v.From, "a ref string", fs)
		if v.From == "" {
			fs.errorf(CodeRequired, ptr, "a row value object is {\"from\": ref}")
		}
		return v
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		fs.errorf(CodeValue, ptr, "a row value is a literal or {\"from\": ref}, not an array")
		return RowValue{}
	}
	var x any
	if err := json.Unmarshal(trimmed, &x); err != nil {
		fs.errorf(CodeJSON, ptr, "not a JSON value")
		return RowValue{}
	}
	return RowValue{Literal: append(json.RawMessage(nil), trimmed...)}
}

// objectOf decodes raw as a JSON object of raw members, or reports msg.
func objectOf(raw []byte, ptr, msg string, fs *findings) map[string]json.RawMessage {
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&obj); err != nil || obj == nil {
		if err != nil && ptr == "" {
			msg = fmt.Sprintf("%s: %v", msg, err)
		}
		fs.errorf(CodeJSON, ptr, msg)
		return nil
	}
	if dec.More() {
		fs.errorf(CodeJSON, ptr, "trailing content after the JSON value")
	}
	return obj
}

// checkKeys reports every key of obj not in allowed.
func checkKeys(obj map[string]json.RawMessage, allowed []string, ptr, what string, fs *findings) {
	for _, k := range sortedKeys(obj) {
		if !isOneOf(k, allowed) {
			fs.errorf(CodeUnknownField, pointer(ptr, k),
				fmt.Sprintf("unknown field %q for %s (allowed: %s)", k, what, strings.Join(allowed, ", ")))
		}
	}
}

// decodeInto decodes obj[key], when present, into dst, strictly.
func decodeInto(obj map[string]json.RawMessage, key, ptr string, dst any, want string, fs *findings) {
	raw, ok := obj[key]
	if !ok {
		return
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		fs.errorf(CodeJSON, pointer(ptr, key), fmt.Sprintf("%q must be %s", key, want))
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
