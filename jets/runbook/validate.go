package runbook

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/sqlscript"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

var (
	namePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	columnPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
)

// ReservedID is the one name a step may not take: refs to parameters begin
// with it.
const ReservedID = "params"

// ValidateFile is the save-time check of a `.rb.json` file, the row of the
// per-suffix validator table in jets/datatable. It has the file alone, so it
// checks everything but what needs the file's name or its SQL files: Decode,
// then Validate with no name and no SQL files.
func ValidateFile(content string) []wsvalidate.Finding {
	rb, fs := Decode([]byte(content))
	if rb == nil {
		return fs
	}
	return append(fs, Validate(rb, "", nil)...)
}

// Load decodes and validates a runbook as the executor does at a run's start
// and when it lists runbooks: Decode, then Validate with the file name and
// the SQL files. The runbook is nil when the content is not a JSON object.
func Load(fileName string, content []byte, sqlFiles map[string]string) (*Runbook, []wsvalidate.Finding) {
	rb, fs := Decode(content)
	if rb == nil {
		return nil, fs
	}
	return rb, append(fs, Validate(rb, fileName, sqlFiles)...)
}

// Validate checks the rules of the format.
//
// fileName is the runbook's file name, `<name>.rb.json`, with or without a
// directory; when it is not empty, the `runbook` field must equal the name.
// The save-time check passes "" because a validator is handed content only.
//
// sqlFiles maps each SQL path relative to the runbooks directory
// (Runbook.SQLPaths, e.g. "example/check.sql") to its text. When it is nil the
// SQL checks are skipped; when it is not nil, a missing file is an error.
//
// Every finding carries a JSON Pointer into the runbook; a finding about a
// SQL file also carries the line in that file and names the file in its
// message.
func Validate(rb *Runbook, fileName string, sqlFiles map[string]string) []wsvalidate.Finding {
	v := &validator{
		rb:       rb,
		sqlFiles: sqlFiles,
		params:   map[string]Type{},
		outputs:  map[string]Type{},
		declared: map[string]string{},
		done:     map[string]bool{},
		allIDs:   map[string]bool{},
	}
	v.run(fileName)
	return v.fs
}

type validator struct {
	rb       *Runbook
	sqlFiles map[string]string
	fs       findings

	params   map[string]Type // parameter name to type
	outputs  map[string]Type // "<step>.<column>" to its declared type, for steps already walked
	declared map[string]string
	done     map[string]bool // step ids already walked
	allIDs   map[string]bool // every step id in the file
}

func (v *validator) run(fileName string) {
	rb := v.rb
	if rb.Format != FormatVersion {
		v.fs.errorf(CodeFormat, "/format", fmt.Sprintf("\"format\" must be %d", FormatVersion))
	}
	switch {
	case rb.Runbook == "":
		v.fs.errorf(CodeRequired, "/runbook", "\"runbook\" is required: the file name without .rb.json")
	case !namePattern.MatchString(rb.Runbook):
		v.fs.errorf(CodeName, "/runbook", fmt.Sprintf("runbook name %q must match %s", rb.Runbook, namePattern))
	}
	if fileName != "" {
		base := path.Base(strings.ReplaceAll(fileName, "\\", "/"))
		if !strings.HasSuffix(base, FileSuffix) {
			v.fs.errorf(CodeName, "", fmt.Sprintf("a runbook file is named <name>%s, not %s", FileSuffix, base))
		} else if name := strings.TrimSuffix(base, FileSuffix); name != rb.Runbook {
			v.fs.errorf(CodeName, "/runbook", fmt.Sprintf("\"runbook\" is %q and the file is %s: they must agree", rb.Runbook, base))
		}
	}

	for _, name := range sortedKeys(rb.Parameters) {
		p := pointer("/parameters", name)
		par := rb.Parameters[name]
		if !namePattern.MatchString(name) {
			v.fs.errorf(CodeName, p, fmt.Sprintf("parameter name %q must match %s", name, namePattern))
		}
		if par.Type == "" {
			v.fs.errorf(CodeRequired, p, "a parameter has a \"type\"")
			continue
		}
		t, err := ParseType(par.Type)
		if err != nil {
			v.fs.errorf(CodeType, pointer(p, "type"), err.Error())
			continue
		}
		v.params[name] = t
		if len(par.Default) > 0 {
			if _, err := ParseValue(t, par.Default); err != nil {
				v.fs.errorf(CodeValue, pointer(p, "default"), fmt.Sprintf("the default is not a %s: %v", t, err))
			}
		}
	}

	if len(rb.Steps) == 0 {
		v.fs.errorf(CodeRequired, "/steps", "a runbook has at least one step")
	}
	rb.Walk(func(s, _ *Step) bool {
		if s.ID != "" {
			v.allIDs[s.ID] = true
		}
		return true
	})
	for i, s := range rb.Steps {
		v.step(s, fmt.Sprintf("/steps/%d", i), nil)
	}
}

func (v *validator) step(s *Step, ptr string, parent *Step) {
	if s == nil {
		v.fs.errorf(CodeRequired, ptr, "a step is an object")
		return
	}
	// The id.
	switch {
	case s.ID == "":
		v.fs.errorf(CodeRequired, pointer(ptr, "id"), "a step has an \"id\"")
	case s.ID == ReservedID:
		v.fs.errorf(CodeID, pointer(ptr, "id"), "\"params\" is reserved: refs to parameters begin with it")
	case !namePattern.MatchString(s.ID):
		v.fs.errorf(CodeID, pointer(ptr, "id"), fmt.Sprintf("step id %q must match %s", s.ID, namePattern))
	default:
		if first, dup := v.declared[s.ID]; dup {
			v.fs.errorf(CodeID, pointer(ptr, "id"), fmt.Sprintf("step id %q is already used at %s; ids are unique in the file, nested steps included", s.ID, first))
		} else {
			v.declared[s.ID] = ptr
		}
	}

	if _, known := stepFields[s.Type]; !known {
		if s.Type == "" {
			v.fs.errorf(CodeRequired, pointer(ptr, "type"), "a step has a \"type\"")
		} else {
			v.fs.errorf(CodeType, pointer(ptr, "type"), fmt.Sprintf("unknown step type %q (the vocabulary is closed: %s)", s.Type, stepTypeList()))
		}
		return
	}
	if s.When != nil {
		v.condition(s.When, pointer(ptr, "when"))
	}
	if s.OnFail != "" && s.OnFail != OnFailStop && s.OnFail != OnFailPause {
		v.fs.errorf(CodeValue, pointer(ptr, "on_fail"), fmt.Sprintf("\"on_fail\" is stop or pause, not %q", s.OnFail))
	}

	switch s.Type {
	case StepQuery, StepWait:
		v.sqlStep(s, ptr)
		if len(s.Returns) == 0 {
			v.fs.errorf(CodeRequired, pointer(ptr, "returns"), "a "+string(s.Type)+" step declares its columns in \"returns\"")
		}
		v.typedNames(s.Returns, pointer(ptr, "returns"), "column")
		if s.MaxRows != nil && (*s.MaxRows < 1 || *s.MaxRows > MaxMaxRows) {
			v.fs.errorf(CodeLimit, pointer(ptr, "max_rows"), fmt.Sprintf("\"max_rows\" is 1 to %d", MaxMaxRows))
		}
		if s.Type == StepWait {
			v.outputsOf(s) // until reads the step's own outputs
			v.required(s.Until != nil, ptr, "until", "a wait has an \"until\" condition")
			if s.Until != nil {
				v.condition(s.Until, pointer(ptr, "until"))
			}
			v.required(s.Interval != "", ptr, "interval", "a wait has an \"interval\"")
			v.duration(s.Interval, pointer(ptr, "interval"), MinWaitInterval, MaxDeadline)
			v.required(s.Deadline != "", ptr, "deadline", "a wait has a \"deadline\"")
			v.duration(s.Deadline, pointer(ptr, "deadline"), 0, MaxDeadline)
			v.count(s.MaxCount, ptr, MaxWaitCount, "a wait")
		}
	case StepDDL:
		v.sqlStep(s, ptr)
		if s.Confirm != nil && !*s.Confirm {
			v.fs.errorf(CodeConfirm, pointer(ptr, "confirm"),
				"a ddl step always pauses for a person to accept or deny it; \"confirm\": false is not allowed")
		}
	case StepAssert:
		v.required(s.Condition != nil, ptr, "condition", "an assert has a \"condition\"")
		if s.Condition != nil {
			v.condition(s.Condition, pointer(ptr, "condition"))
		}
	case StepRepeat:
		if parent != nil {
			v.fs.errorf(CodeNesting, ptr, "a repeat cannot be inside a repeat")
		}
		if len(s.Steps) == 0 {
			v.fs.errorf(CodeRequired, pointer(ptr, "steps"), "a repeat has at least one step")
		}
		for i, n := range s.Steps {
			if parent != nil {
				// Report the nesting once, and do not walk deeper.
				break
			}
			v.step(n, fmt.Sprintf("%s/steps/%d", ptr, i), s)
		}
		v.required(s.Until != nil, ptr, "until", "a repeat has an \"until\" condition")
		if s.Until != nil {
			v.condition(s.Until, pointer(ptr, "until"))
		}
		v.count(s.MaxCount, ptr, MaxRepeatCount, "a repeat")
		if s.Interval != "" {
			v.duration(s.Interval, pointer(ptr, "interval"), 0, MaxDeadline)
		}
	case StepPause:
		v.required(strings.TrimSpace(s.Message) != "", ptr, "message", "a pause has a \"message\"")
	case StepManual:
		v.required(strings.TrimSpace(s.Instruction) != "", ptr, "instruction", "a manual step has an \"instruction\"")
		v.typedNames(s.Records, pointer(ptr, "records"), "record")
	case StepStartPipeline:
		v.required(len(s.Row) > 0, ptr, "row", "a start_pipeline step has a \"row\"")
		for _, col := range sortedKeys(s.Row) {
			p := pointer(pointer(ptr, "row"), col)
			if !columnPattern.MatchString(col) {
				v.fs.errorf(CodeName, p, fmt.Sprintf("column %q must match %s", col, columnPattern))
			}
			if rv := s.Row[col]; rv.From != "" {
				v.ref(rv.From, p+"/from")
			}
		}
	case StepPutSchemaEvent:
		v.required(s.Event != nil && s.Event.From != "", ptr, "event", "a put_schema_event step has an \"event\": {\"from\": ref}")
		if s.Event != nil && s.Event.From != "" {
			v.ref(s.Event.From, pointer(ptr, "event")+"/from")
		}
	case StepCompile:
		if s.Deadline != "" {
			v.duration(s.Deadline, pointer(ptr, "deadline"), 0, MaxDeadline)
		}
	}
	if s.Type != StepWait {
		v.outputsOf(s)
	}
	v.done[s.ID] = true
}

// outputsOf puts the step's declared outputs in scope.
func (v *validator) outputsOf(s *Step) {
	for col, t := range s.OutputTypes() {
		v.outputs[OutputRef(s.ID, col)] = t
	}
	v.done[s.ID] = true
}

func (v *validator) required(ok bool, ptr, field, msg string) {
	if !ok {
		v.fs.errorf(CodeRequired, pointer(ptr, field), msg)
	}
}

func (v *validator) count(n *int, ptr string, max int, what string) {
	switch {
	case n == nil:
		v.fs.errorf(CodeRequired, pointer(ptr, "max_count"), what+" has a \"max_count\"")
	case *n < 1 || *n > max:
		v.fs.errorf(CodeLimit, pointer(ptr, "max_count"), fmt.Sprintf("\"max_count\" is 1 to %d for %s", max, what))
	}
}

func (v *validator) duration(s, ptr string, min, max time.Duration) {
	if s == "" {
		return
	}
	d, err := time.ParseDuration(s)
	switch {
	case err != nil:
		v.fs.errorf(CodeValue, ptr, fmt.Sprintf("%q is not a duration such as 30s, 5m or 1h", s))
	case d <= 0:
		v.fs.errorf(CodeLimit, ptr, "a duration is positive")
	case min > 0 && d < min:
		v.fs.errorf(CodeLimit, ptr, fmt.Sprintf("%s is less than the minimum, %s", s, min))
	case d > max:
		v.fs.errorf(CodeLimit, ptr, fmt.Sprintf("%s is more than the cap, %s", s, max))
	}
}

func (v *validator) typedNames(m map[string]string, ptr, what string) {
	for _, name := range sortedKeys(m) {
		p := pointer(ptr, name)
		if !columnPattern.MatchString(name) {
			v.fs.errorf(CodeName, p, fmt.Sprintf("%s name %q must match %s", what, name, columnPattern))
		}
		if _, err := ParseType(m[name]); err != nil {
			v.fs.errorf(CodeType, p, err.Error())
		}
	}
}

// ref checks that a ref resolves to a parameter or an output in scope, and
// returns its type.
func (v *validator) ref(ref, ptr string) (Type, bool) {
	if name, ok := strings.CutPrefix(ref, ReservedID+"."); ok {
		if t, ok := v.params[name]; ok {
			return t, true
		}
		v.fs.errorf(CodeRef, ptr, fmt.Sprintf("%s: no parameter %q is declared", ref, name))
		return Type{}, false
	}
	step, col, ok := strings.Cut(ref, ".")
	if !ok || !namePattern.MatchString(step) || !columnPattern.MatchString(col) {
		v.fs.errorf(CodeRef, ptr, fmt.Sprintf("%q is not a ref: write params.<name> or <step_id>.<column>", ref))
		return Type{}, false
	}
	if t, ok := v.outputs[ref]; ok {
		return t, true
	}
	switch {
	case v.done[step]:
		v.fs.errorf(CodeRef, ptr, fmt.Sprintf("%s: step %q has no output %q", ref, step, col))
	case v.allIDs[step]:
		v.fs.errorf(CodeRef, ptr, fmt.Sprintf("%s: step %q does not run before this point; a ref reads a parameter or an earlier step", ref, step))
	default:
		v.fs.errorf(CodeRef, ptr, fmt.Sprintf("%s: there is no step %q", ref, step))
	}
	return Type{}, false
}

// condition checks a condition's refs and that its operands fit their types.
func (v *validator) condition(c *Condition, ptr string) {
	switch {
	case c.All != nil:
		for i, s := range c.All {
			v.condition(s, fmt.Sprintf("%s/all/%d", ptr, i))
		}
		return
	case c.Any != nil:
		for i, s := range c.Any {
			v.condition(s, fmt.Sprintf("%s/any/%d", ptr, i))
		}
		return
	case c.Not != nil:
		v.condition(c.Not, ptr+"/not")
		return
	}
	if c.Ref == "" || c.Op == "" {
		return // the grammar finding is Decode's
	}
	t, ok := v.ref(c.Ref, pointer(ptr, c.Quantifier))
	if !ok {
		return
	}
	if c.Quantifier == QOutput && t.List && c.Op != OpEmpty {
		v.fs.errorf(CodeCondition, ptr, fmt.Sprintf("%s is a %s; read it with all_of, any_of or count", c.Ref, t))
	}
	if c.Op == OpEmpty {
		return
	}
	base := t.Base
	if c.Quantifier == QCount {
		base = TypeInteger
	}
	if _, err := c.operands(base); err != nil {
		v.fs.errorf(CodeValue, pointer(ptr, c.Op), fmt.Sprintf("%s is compared as a %s: %v", c.Ref, base, err))
	}
}

// sqlStep checks a query, ddl or wait step's SQL path, binds, and file.
func (v *validator) sqlStep(s *Step, ptr string) {
	if s.SQL != "" {
		clean := path.Clean(s.SQL)
		if path.IsAbs(s.SQL) || clean != s.SQL || strings.HasPrefix(clean, "../") || clean == ".." ||
			!strings.HasSuffix(s.SQL, ".sql") || strings.Contains(s.SQL, "\\") {
			v.fs.errorf(CodeSQLFile, pointer(ptr, "sql"),
				fmt.Sprintf("%q: \"sql\" is a clean relative path ending .sql under %s/, such as %s", s.SQL, Dir, path.Join(v.rb.Runbook, s.ID+".sql")))
			return
		}
	}
	for _, name := range sortedKeys(s.Bind) {
		p := pointer(pointer(ptr, "bind"), name)
		if !columnPattern.MatchString(name) {
			v.fs.errorf(CodeName, p, fmt.Sprintf("bind name %q must match %s", name, columnPattern))
		}
		v.ref(s.Bind[name], p)
	}
	if s.Timeout != "" {
		v.duration(s.Timeout, pointer(ptr, "timeout"), 0, MaxTimeout)
	}
	if v.sqlFiles == nil {
		return
	}

	file := s.SQLPath(v.rb.Runbook)
	where := pointer(ptr, "sql")
	text, ok := v.sqlFiles[file]
	if !ok {
		v.fs.errorf(CodeSQLFile, where, fmt.Sprintf("%s/%s does not exist", Dir, file))
		return
	}
	at := func(code string, sev wsvalidate.Severity, line int, msg string) {
		v.fs.add(wsvalidate.Finding{Severity: sev, Code: code, Path: where, Line: line,
			Message: fmt.Sprintf("%s/%s: %s", Dir, file, msg)})
	}
	if lex := sqlscript.ValidateScript(text); len(lex) > 0 {
		for _, f := range lex {
			at(f.Code, f.Severity, f.Line, f.Message)
		}
		return
	}
	stmts, _ := sqlscript.Split(text)
	n := 0
	for _, st := range stmts {
		if st.Body != "" {
			n++
		}
	}
	switch {
	case (s.Type == StepQuery || s.Type == StepWait) && n != 1:
		at(CodeSQLStatements, wsvalidate.Error, 0, fmt.Sprintf("a %s step's file holds exactly one statement; it holds %d", s.Type, n))
	case n == 0:
		at(CodeSQLStatements, wsvalidate.Error, 0, "the file holds no statement")
	}

	scan, err := scanSQL(text)
	if err != nil {
		at(sqlscript.UnterminatedCode, wsvalidate.Error, 0, err.Error())
		return
	}
	for _, sub := range scan.substs {
		at(CodeSQLSubst, wsvalidate.Error, sub.line, sub.message)
	}
	for _, sub := range scan.quoted {
		at(CodeSQLSubst, wsvalidate.Warning, sub.line, fmt.Sprintf("%s inside a string literal is sent as written: nothing substitutes it", sub.text))
	}
	used := map[string]bool{}
	for _, nm := range scan.names {
		if used[nm.name] {
			continue
		}
		used[nm.name] = true
		if _, ok := s.Bind[nm.name]; ok {
			continue // its ref was checked above
		}
		if _, ok := v.params[nm.name]; !ok {
			at(CodeSQLBind, wsvalidate.Error, nm.line, fmt.Sprintf(
				":%s resolves to nothing: declare a parameter %q or bind it to an earlier output in \"bind\"", nm.name, nm.name))
		}
	}
	for _, name := range sortedKeys(s.Bind) {
		if !used[name] {
			v.fs.warnf(CodeSQLBind, pointer(pointer(ptr, "bind"), name),
				fmt.Sprintf("%s/%s has no :%s", Dir, file, name))
		}
	}
}

func stepTypeList() string {
	var names []string
	for _, t := range StepTypes() {
		names = append(names, string(t))
	}
	return strings.Join(names, ", ")
}

// ParameterValues resolves a run's parameters: each given value coerced to
// its declared type, a default for one not given, and an error for one not
// given with no default, or given and not declared.
func ParameterValues(rb *Runbook, given map[string]json.RawMessage) (map[string]Value, error) {
	out := make(map[string]Value, len(rb.Parameters))
	for name := range given {
		if _, ok := rb.Parameters[name]; !ok {
			return nil, fmt.Errorf("parameter %q is not declared by runbook %s", name, rb.Runbook)
		}
	}
	for _, name := range sortedKeys(rb.Parameters) {
		par := rb.Parameters[name]
		t, err := ParseType(par.Type)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %v", name, err)
		}
		raw, ok := given[name]
		if !ok {
			raw = par.Default
		}
		if len(raw) == 0 {
			return nil, fmt.Errorf("parameter %q is required: it has no default", name)
		}
		val, err := ParseValue(t, raw)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %v", name, err)
		}
		out[name] = val
	}
	return out, nil
}

// QueryOutputs turns a query or wait result into the step's outputs, keyed
// by column (put them in an Env as OutputRef(step.ID, column)).
//
// The result's columns must equal `returns`'s keys, in any order. With exactly
// one row each column is a scalar, unless `returns` declares it `list<T>`;
// with any other number of rows each column is a list. Values are coerced to
// their declared types.
func QueryOutputs(step *Step, columns []string, rows [][]any) (map[string]Value, error) {
	want := map[string]Type{}
	for col, ts := range step.Returns {
		t, err := ParseType(ts)
		if err != nil {
			return nil, fmt.Errorf("returns %q: %v", col, err)
		}
		want[col] = t
	}
	seen := map[string]bool{}
	for _, c := range columns {
		if _, ok := want[c]; !ok {
			return nil, fmt.Errorf("the result has column %q, which \"returns\" does not declare", c)
		}
		if seen[c] {
			return nil, fmt.Errorf("the result has column %q twice", c)
		}
		seen[c] = true
	}
	for _, c := range sortedKeys(want) {
		if !seen[c] {
			return nil, fmt.Errorf("\"returns\" declares %q and the result has no such column", c)
		}
	}
	out := make(map[string]Value, len(columns))
	for i, c := range columns {
		t := want[c]
		if len(rows) == 1 && !t.List {
			if len(rows[0]) != len(columns) {
				return nil, fmt.Errorf("row 0 has %d values for %d columns", len(rows[0]), len(columns))
			}
			val, err := NewValue(t, rows[0][i])
			if err != nil {
				return nil, fmt.Errorf("column %q: %v", c, err)
			}
			out[c] = val
			continue
		}
		elems := make([]any, len(rows))
		for r, row := range rows {
			if len(row) != len(columns) {
				return nil, fmt.Errorf("row %d has %d values for %d columns", r, len(row), len(columns))
			}
			elems[r] = row[i]
		}
		val, err := ListValue(t.Base, elems)
		if err != nil {
			return nil, fmt.Errorf("column %q: %v", c, err)
		}
		out[c] = val
	}
	return out, nil
}
