package rulestest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Finding codes.
const (
	CodeInvalidJSON  = "invalid_json"
	CodeUnknownField = "unknown_field"
	CodeRefused      = "refused_field"   // a JetrulesSpec field a test does not use
	CodeMisplaced    = "misplaced_field" // a top-level field written somewhere else
	CodeRequired     = "required"
	CodeInvalidValue = "invalid_value"
	CodeConflict     = "conflict"
	CodeMissingFile  = "missing_file"
)

// topLevelOnly are the fields of TestDefinition that are refused anywhere else
// with a message naming where they belong (definition §8.5.2).
var topLevelOnly = map[string]bool{"legacy": true, "compare_engines": true, "expect": true}

// RdfTypes are the type names ParseRdfNodeValue reads.
var RdfTypes = []string{"null", "bn", "resource", "int", "bool", "long", "uint", "ulong", "double", "text", "date", "datetime"}

// NoTruthMainOnExist is the C++-only rule_config switch (definition F14).
const NoTruthMainOnExist = "_0:no_truth_main_on_exist"

var variableRe = regexp.MustCompile(`^\?[A-Za-z_][A-Za-z0-9_]*$`)

// LoadTestDefinition reads and validates the .tst.json at path. The error is
// for a file that cannot be read; everything wrong with its content is a
// finding. A definition with an Error finding must not be run; it is returned
// whenever the JSON decodes, so a caller recording it (the workspace compile)
// has its name and process. Input CSVs are checked to exist beside the file.
func LoadTestDefinition(path string) (*TestDefinition, []wsvalidate.Finding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	td, findings := ParseTestDefinition(content)
	if td != nil {
		findings = append(findings, checkInputFiles(td, filepath.Dir(path))...)
	}
	return td, findings, nil
}

// ValidateContent is a wsvalidate.Validator for .tst.json: everything
// LoadTestDefinition checks except that the input files exist.
func ValidateContent(content string) []wsvalidate.Finding {
	_, findings := ParseTestDefinition([]byte(content))
	return findings
}

var _ wsvalidate.Validator = ValidateContent

// HasErrors reports whether any finding is an Error.
func HasErrors(findings []wsvalidate.Finding) bool {
	return len(wsvalidate.ErrorsOnly(findings)) > 0
}

// ParseTestDefinition decodes and validates a test definition. It returns nil
// when the content is not JSON of the right shape.
func ParseTestDefinition(content []byte) (*TestDefinition, []wsvalidate.Finding) {
	var raw any
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, []wsvalidate.Finding{errorf(CodeInvalidJSON, "", "not valid JSON: %v", err)}
	}
	if _, ok := raw.(map[string]any); !ok {
		return nil, []wsvalidate.Finding{errorf(CodeInvalidJSON, "", "a test definition is a JSON object")}
	}
	// Keys first, exactly and everywhere, so every unknown key is reported with
	// its path. encoding/json matches field names case-insensitively and names
	// no path in its unknown-field error, so the strict decode below is the
	// backstop, not the report.
	findings := checkKeys(raw, reflect.TypeFor[TestDefinition](), "", true)
	if len(findings) > 0 {
		return nil, findings
	}
	var td TestDefinition
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&td); err != nil {
		return nil, []wsvalidate.Finding{decodeFinding(err)}
	}
	if dec.More() {
		return nil, []wsvalidate.Finding{errorf(CodeInvalidJSON, "", "content after the test definition's closing brace")}
	}
	findings = append(findings, validateDefinition(&td, raw.(map[string]any))...)
	return &td, findings
}

func decodeFinding(err error) wsvalidate.Finding {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) {
		path := ""
		if te.Field != "" {
			path = "/" + strings.ReplaceAll(te.Field, ".", "/")
		}
		return errorf(CodeInvalidValue, path, "%s must be a JSON %s, not %s", fieldLabel(path), jsonKind(te.Type), te.Value)
	}
	return errorf(CodeInvalidJSON, "", "%v", err)
}

func jsonKind(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "object"
	}
}

// ---------------------------------------------------------------------------
// Key checking

// keyChecker is implemented by a type whose JSON shape is not its struct fields.
type keyChecker interface {
	checkKeys(raw any, ptr string) []wsvalidate.Finding
}

// refuser is implemented by a struct some of whose JSON names are refused with
// a reason rather than reported as unknown.
type refuser interface {
	refusedFields() map[string]string
}

var (
	keyCheckerType = reflect.TypeFor[keyChecker]()
	refuserType    = reflect.TypeFor[refuser]()
)

// checkKeys walks raw against t and reports every object key that t does not
// declare, exactly, with its JSON Pointer. Type mismatches are left to the decoder.
func checkKeys(raw any, t reflect.Type, ptr string, root bool) []wsvalidate.Finding {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(keyCheckerType) {
		return reflect.New(t).Interface().(keyChecker).checkKeys(raw, ptr)
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		fields := structFields(t)
		var refused map[string]string
		if reflect.PointerTo(t).Implements(refuserType) {
			refused = reflect.New(t).Interface().(refuser).refusedFields()
		}
		var findings []wsvalidate.Finding
		for _, key := range sortedKeys(obj) {
			kptr := ptr + "/" + escapePointer(key)
			if ft, ok := fields[key]; ok {
				findings = append(findings, checkKeys(obj[key], ft, kptr, false)...)
				continue
			}
			switch {
			case refused[key] != "":
				findings = append(findings, errorf(CodeRefused, kptr, "%s %s", key, refused[key]))
			case !root && topLevelOnly[key]:
				findings = append(findings, errorf(CodeMisplaced, kptr,
					"%s is a top-level field of the test definition, beside jetrules_config; it is not allowed here", key))
			default:
				findings = append(findings, unknownField(key, kptr, fields, refused))
			}
		}
		return findings
	case reflect.Slice, reflect.Array:
		arr, ok := raw.([]any)
		if !ok {
			return nil
		}
		var findings []wsvalidate.Finding
		for i, v := range arr {
			findings = append(findings, checkKeys(v, t.Elem(), ptr+"/"+strconv.Itoa(i), false)...)
		}
		return findings
	case reflect.Map:
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		var findings []wsvalidate.Finding
		for _, key := range sortedKeys(obj) {
			findings = append(findings, checkKeys(obj[key], t.Elem(), ptr+"/"+escapePointer(key), false)...)
		}
		return findings
	}
	return nil
}

func unknownField(key, ptr string, fields map[string]reflect.Type, refused map[string]string) wsvalidate.Finding {
	names := make([]string, 0, len(fields)+len(refused))
	for n := range fields {
		names = append(names, n)
	}
	for n := range refused {
		names = append(names, n)
	}
	if best := ClosestName(key, names); best != "" {
		return errorf(CodeUnknownField, ptr, "unknown field %q; did you mean %q?", key, best)
	}
	return errorf(CodeUnknownField, ptr, "unknown field %q", key)
}

// structFields maps the JSON names of t's exported fields to their types,
// promoting the fields of embedded structs as encoding/json does.
func structFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				for n, nt := range structFields(ft) {
					if _, ok := fields[n]; !ok {
						fields[n] = nt
					}
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = f.Type
	}
	return fields
}

func jsonFieldNames(v any) []string {
	return sortedKeys(structFields(reflect.TypeOf(v)))
}

// checkKeys for an object term: a string, or exactly {"value", "type"}, both strings.
func (*ObjectTerm) checkKeys(raw any, ptr string) []wsvalidate.Finding {
	switch v := raw.(type) {
	case string:
		return nil
	case map[string]any:
		return checkTypedValue(v, ptr)
	default:
		return []wsvalidate.Finding{errorf(CodeInvalidValue, ptr,
			`an object is a string (text, or a ?variable) or {"value": "...", "type": "..."}`)}
	}
}

// checkTypedValue checks {"value": "...", "type": "..."} as ParseRdfNodeValue
// reads it: both strings, the type one it knows.
func checkTypedValue(v map[string]any, ptr string) []wsvalidate.Finding {
	var findings []wsvalidate.Finding
	for _, key := range sortedKeys(v) {
		kptr := ptr + "/" + escapePointer(key)
		switch key {
		case "value", "type":
			if _, ok := v[key].(string); !ok {
				findings = append(findings, errorf(CodeInvalidValue, kptr,
					"%s must be a JSON string, as rule_config writes it (e.g. \"1\", not 1)", key))
			}
		default:
			findings = append(findings, unknownField(key, kptr,
				map[string]reflect.Type{"value": nil, "type": nil}, nil))
		}
	}
	if t, ok := v["type"].(string); !ok {
		if _, present := v["type"]; !present {
			findings = append(findings, errorf(CodeRequired, ptr+"/type", "type is required in {\"value\", \"type\"}"))
		}
	} else if !isRdfType(t) {
		findings = append(findings, errorf(CodeInvalidValue, ptr+"/type",
			"unknown type %q; one of %s", t, strings.Join(RdfTypes, ", ")))
	}
	return findings
}

func isRdfType(t string) bool {
	for _, k := range RdfTypes {
		if t == k {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Semantic validation

func validateDefinition(td *TestDefinition, raw map[string]any) []wsvalidate.Finding {
	var f []wsvalidate.Finding
	if strings.TrimSpace(td.TestName) == "" {
		f = append(f, errorf(CodeRequired, "/test_name", "test_name is required"))
	}
	if td.Expect != nil {
		if td.Expect.Status != StatusError {
			f = append(f, errorf(CodeInvalidValue, "/expect/status",
				"expect.status must be %q; a test that expects to pass needs no expect", StatusError))
		}
	}
	f = append(f, validateJetrulesConfig(td, raw)...)
	if len(td.InputSessions) == 0 {
		f = append(f, errorf(CodeRequired, "/input_sessions", "at least one input session is required"))
	}
	seen := map[string]int{}
	for i := range td.InputSessions {
		s := &td.InputSessions[i]
		sptr := fmt.Sprintf("/input_sessions/%d", i)
		switch {
		case strings.TrimSpace(s.Name) == "":
			f = append(f, errorf(CodeRequired, sptr+"/name", "the input session's name is required"))
		case seen[s.Name] > 0:
			f = append(f, errorf(CodeConflict, sptr+"/name",
				"input session name %q is already used by input session %d", s.Name, seen[s.Name]-1))
		default:
			seen[s.Name] = i + 1
		}
		if len(s.Input) == 0 {
			f = append(f, errorf(CodeRequired, sptr+"/input", "an input session needs at least one CSV file"))
		}
		for j, in := range s.Input {
			if msg := relativePathProblem(in); msg != "" {
				f = append(f, errorf(CodeInvalidValue, fmt.Sprintf("%s/input/%d", sptr, j), "%s", msg))
			}
		}
		f = append(f, validateChecks(s.ExpectedTriples, sptr+"/expected_triples", true)...)
		f = append(f, validateChecks(s.ExcludedTriples, sptr+"/excluded_triples", false)...)
	}
	f = append(f, validateChecks(td.ExpectedTriples, "/expected_triples", true)...)
	f = append(f, validateChecks(td.ExcludedTriples, "/excluded_triples", false)...)
	return f
}

func validateJetrulesConfig(td *TestDefinition, raw map[string]any) []wsvalidate.Finding {
	var f []wsvalidate.Finding
	jc := &td.JetrulesConfig
	if _, ok := raw["jetrules_config"]; !ok {
		return append(f, errorf(CodeRequired, "/jetrules_config", "jetrules_config is required"))
	}
	if strings.TrimSpace(jc.ProcessName) == "" {
		f = append(f, errorf(CodeRequired, "/jetrules_config/process_name",
			"process_name is required: a rule file listed in rule_sets, or a rule sequence name"))
	}
	if jc.UseJetRulesNative && jc.UseJetRulesGo {
		f = append(f, errorf(CodeConflict, "/jetrules_config/use_jet_rules_go",
			"use_jet_rules_go and use_jet_rules_native are both set; a test runs on one engine, or on both with compare_engines"))
	}
	if td.CompareEngines && (jc.UseJetRulesNative || jc.UseJetRulesGo) {
		f = append(f, errorf(CodeConflict, "/compare_engines",
			"compare_engines runs both engines; remove use_jet_rules_go and use_jet_rules_native"))
	}
	if jc.CurrentSourcePeriod != 0 && jc.CurrentSourcePeriodType == "" {
		// AssertSourcePeriodInfo replaces a period without a type by the
		// environment's, which a test does not have: it would assert 0.
		f = append(f, errorf(CodeRequired, "/jetrules_config/current_source_period_type",
			"current_source_period needs current_source_period_type (month_period, day_period or hour_period); without it the period is reset to 0"))
	}
	if jc.MaxLooping < 0 {
		f = append(f, errorf(CodeInvalidValue, "/jetrules_config/max_looping", "max_looping must not be negative"))
	}
	runsGo := jc.UseJetRulesGo || td.CompareEngines
	for i, rc := range jc.RuleConfig {
		rptr := fmt.Sprintf("/jetrules_config/rule_config/%d", i)
		for _, key := range sortedKeys(rc) {
			kptr := rptr + "/" + escapePointer(key)
			switch v := rc[key].(type) {
			case string:
			case map[string]any:
				f = append(f, checkTypedValue(v, kptr)...)
			default:
				f = append(f, errorf(CodeInvalidValue, kptr,
					`a rule_config value is a string (text) or {"value": "...", "type": "..."}`))
			}
			if key == NoTruthMainOnExist && runsGo {
				f = append(f, errorf(CodeRefused, kptr,
					"%s exists only in the C++ engine; the Go engine would ignore it (use_jet_rules_go or compare_engines is set)", key))
			}
		}
	}
	return f
}

func validateChecks(checks []Check, ptr string, expected bool) []wsvalidate.Finding {
	var f []wsvalidate.Finding
	for i := range checks {
		f = append(f, validateCheck(&checks[i], fmt.Sprintf("%s/%d", ptr, i), expected)...)
	}
	return f
}

func validateCheck(c *Check, ptr string, expected bool) []wsvalidate.Finding {
	var f []wsvalidate.Finding
	bound := map[string]bool{}
	for j := range c.Pattern {
		t := &c.Pattern[j]
		f = append(f, validateTriple(t.S, t.P, t.O, fmt.Sprintf("%s/pattern/%d", ptr, j), true)...)
		for _, v := range []string{t.S, t.P} {
			if IsVariableName(v) {
				bound[v] = true
			}
		}
		if t.O.IsVariable() {
			bound[t.O.Variable] = true
		}
	}
	switch {
	case c.Count != nil:
		if !expected {
			f = append(f, errorf(CodeInvalidValue, ptr+"/count",
				"a count belongs in expected_triples; state the count you expect instead of excluding one"))
		}
		if c.O != nil {
			f = append(f, errorf(CodeConflict, ptr+"/o", "a count check names s and p, not o"))
		}
		f = append(f, validateTriple(c.S, c.P, &ObjectTerm{Variable: "?count"}, ptr, false)...)
		for _, pos := range []struct{ name, v string }{{"s", c.S}, {"p", c.P}} {
			if IsVariableName(pos.v) && !bound[pos.v] {
				f = append(f, errorf(CodeInvalidValue, ptr+"/"+pos.name,
					"%s in a count must be bound by the check's pattern", pos.v))
			}
		}
	case len(c.Pattern) > 0:
		for _, pos := range []struct {
			name string
			set  bool
		}{{"s", c.S != ""}, {"p", c.P != ""}, {"o", c.O != nil}} {
			if pos.set {
				f = append(f, errorf(CodeConflict, ptr+"/"+pos.name,
					"a check is a triple (s, p, o), a pattern, or a count (s, p, count); %s does not go with pattern unless it is a count", pos.name))
			}
		}
	default:
		if c.S == "" && c.P == "" && c.O == nil {
			f = append(f, errorf(CodeRequired, ptr,
				"a check is a triple (s, p, o), a pattern, or a count (s, p, count)"))
			break
		}
		f = append(f, validateTriple(c.S, c.P, c.O, ptr, true)...)
	}
	return f
}

func validateTriple(s, p string, o *ObjectTerm, ptr string, needO bool) []wsvalidate.Finding {
	var f []wsvalidate.Finding
	for _, pos := range []struct{ name, v string }{{"s", s}, {"p", p}} {
		switch {
		case strings.TrimSpace(pos.v) == "":
			f = append(f, errorf(CodeRequired, ptr+"/"+pos.name, "%s is required", pos.name))
		case IsVariableName(pos.v) && !variableRe.MatchString(pos.v):
			f = append(f, errorf(CodeInvalidValue, ptr+"/"+pos.name,
				"%q is not a variable name: ? then a letter or _, then letters, digits or _", pos.v))
		}
	}
	switch {
	case o == nil:
		if needO {
			f = append(f, errorf(CodeRequired, ptr+"/o", "o is required"))
		}
	case o.Variable != "":
		if !variableRe.MatchString(o.Variable) {
			f = append(f, errorf(CodeInvalidValue, ptr+"/o",
				"%q is not a variable name: ? then a letter or _, then letters, digits or _", o.Variable))
		}
	}
	return f
}

func relativePathProblem(p string) string {
	switch {
	case strings.TrimSpace(p) == "":
		return "an input file name is empty"
	case filepath.IsAbs(p):
		return fmt.Sprintf("%q is absolute; input files are relative to the test definition's directory", p)
	case !filepath.IsLocal(p):
		return fmt.Sprintf("%q leaves the test definition's directory", p)
	}
	return ""
}

func checkInputFiles(td *TestDefinition, dir string) []wsvalidate.Finding {
	var f []wsvalidate.Finding
	for i := range td.InputSessions {
		for j, in := range td.InputSessions[i].Input {
			if relativePathProblem(in) != "" {
				continue
			}
			st, err := os.Stat(filepath.Join(dir, in))
			switch {
			case err != nil:
				f = append(f, errorf(CodeMissingFile, fmt.Sprintf("/input_sessions/%d/input/%d", i, j),
					"input file %q not found beside the test definition", in))
			case st.IsDir():
				f = append(f, errorf(CodeInvalidValue, fmt.Sprintf("/input_sessions/%d/input/%d", i, j),
					"input %q is a directory, not a CSV file", in))
			}
		}
	}
	return f
}

// ---------------------------------------------------------------------------
// Helpers

func errorf(code, path, format string, args ...any) wsvalidate.Finding {
	return wsvalidate.Finding{Severity: wsvalidate.Error, Code: code, Path: path, Message: fmt.Sprintf(format, args...)}
}

func fieldLabel(path string) string {
	if path == "" {
		return "the test definition"
	}
	return path
}

// escapePointer escapes a JSON Pointer segment (RFC 6901 §3).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ClosestName returns the candidate closest to name: an exact match ignoring
// case first, then the smallest edit distance within a third of name's length
// (at least 2), or "" when nothing is that close. Ties go to the first in sorted order.
func ClosestName(name string, candidates []string) string {
	sorted := append([]string(nil), candidates...)
	sort.Strings(sorted)
	for _, c := range sorted {
		if strings.EqualFold(c, name) {
			return c
		}
	}
	limit := max(2, len(name)/3)
	best, bestD := "", limit+1
	lname := strings.ToLower(name)
	for _, c := range sorted {
		if d := editDistance(lname, strings.ToLower(c)); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
