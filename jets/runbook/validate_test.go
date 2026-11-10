package runbook

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// A minimal valid runbook, as a Go value the tests mutate before encoding.
func base() map[string]any {
	return map[string]any{
		"format":  1,
		"runbook": "rb",
		"parameters": map[string]any{
			"since_utc": map[string]any{"type": "timestamp"},
			"ids":       map[string]any{"type": "list<session_id>", "default": []any{"1", "2"}},
		},
		"steps": []any{
			map[string]any{"id": "check", "type": "query", "returns": map[string]any{"state": "text", "n": "integer"}},
		},
	}
}

func withSteps(steps ...map[string]any) map[string]any {
	doc := base()
	list := doc["steps"].([]any)
	for _, s := range steps {
		list = append(list, s)
	}
	doc["steps"] = list
	return doc
}

func encode(t *testing.T, doc any) string {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func errorsOf(fs []wsvalidate.Finding) []wsvalidate.Finding { return wsvalidate.ErrorsOnly(fs) }

// wantFinding asserts a finding of this code and severity at this path.
func wantFinding(t *testing.T, fs []wsvalidate.Finding, sev wsvalidate.Severity, code, path string) {
	t.Helper()
	for _, f := range fs {
		if f.Code == code && f.Path == path && f.Severity == sev {
			return
		}
	}
	t.Errorf("want %s %s at %q; got %s", sev, code, path, describe(fs))
}

func wantClean(t *testing.T, fs []wsvalidate.Finding) {
	t.Helper()
	if e := errorsOf(fs); len(e) > 0 {
		t.Errorf("want no errors; got %s", describe(e))
	}
}

func describe(fs []wsvalidate.Finding) string {
	if len(fs) == 0 {
		return "none"
	}
	var b strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&b, "\n  %s %s %s:%d %s", f.Severity, f.Code, f.Path, f.Line, f.Message)
	}
	return b.String()
}

func TestTheBaseRunbookIsValid(t *testing.T) {
	wantClean(t, ValidateFile(encode(t, base())))
}

func TestDecodeIsStrict(t *testing.T) {
	t.Run("not JSON", func(t *testing.T) {
		rb, fs := Decode([]byte("{"))
		if rb != nil {
			t.Error("want nil runbook")
		}
		wantFinding(t, fs, wsvalidate.Error, CodeJSON, "")
	})
	t.Run("an array", func(t *testing.T) {
		_, fs := Decode([]byte("[]"))
		wantFinding(t, fs, wsvalidate.Error, CodeJSON, "")
	})
	t.Run("unknown top-level field", func(t *testing.T) {
		doc := base()
		doc["steps_"] = []any{}
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeUnknownField, "/steps_")
	})
	t.Run("misspelt confirm", func(t *testing.T) {
		doc := withSteps(map[string]any{"id": "d", "type": "ddl", "confim": false})
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeUnknownField, "/steps/1/confim")
	})
	t.Run("a field of another step type", func(t *testing.T) {
		doc := withSteps(map[string]any{"id": "q", "type": "query", "returns": map[string]any{"a": "text"}, "max_count": 3})
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeUnknownField, "/steps/1/max_count")
	})
	t.Run("unknown parameter field", func(t *testing.T) {
		doc := base()
		doc["parameters"] = map[string]any{"x": map[string]any{"type": "text", "required": true}}
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeUnknownField, "/parameters/x/required")
	})
	t.Run("wrong JSON type", func(t *testing.T) {
		doc := withSteps(map[string]any{"id": "q", "type": "query", "returns": map[string]any{"a": "text"}, "max_rows": "many"})
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeJSON, "/steps/1/max_rows")
	})
	t.Run("an unknown type still checks fields", func(t *testing.T) {
		doc := withSteps(map[string]any{"id": "s", "type": "shell", "command": "rm"})
		fs := ValidateFile(encode(t, doc))
		wantFinding(t, fs, wsvalidate.Error, CodeType, "/steps/1/type")
		wantFinding(t, fs, wsvalidate.Error, CodeUnknownField, "/steps/1/command")
	})
	t.Run("a pointer segment is escaped", func(t *testing.T) {
		doc := base()
		doc["a/b~c"] = 1
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeUnknownField, "/a~1b~0c")
	})
}

func TestFormatAndName(t *testing.T) {
	doc := base()
	doc["format"] = 2
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeFormat, "/format")

	doc = base()
	doc["runbook"] = "Bad-Name"
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeName, "/runbook")

	doc = base()
	delete(doc, "runbook")
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeRequired, "/runbook")

	// The name against the file: only Load and Validate know the file.
	content := []byte(encode(t, base()))
	if _, fs := Load("runbooks/rb.rb.json", content, nil); len(errorsOf(fs)) > 0 {
		t.Errorf("rb.rb.json: %s", describe(fs))
	}
	_, fs := Load("runbooks/other.rb.json", content, nil)
	wantFinding(t, fs, wsvalidate.Error, CodeName, "/runbook")
	_, fs = Load("rb.json", content, nil)
	wantFinding(t, fs, wsvalidate.Error, CodeName, "")
}

func TestStepIds(t *testing.T) {
	cases := map[string]struct {
		step map[string]any
		code string
		path string
	}{
		"pattern":   {map[string]any{"id": "Bad", "type": "pause", "message": "m"}, CodeID, "/steps/1/id"},
		"too long":  {map[string]any{"id": "a" + strings.Repeat("b", 63), "type": "pause", "message": "m"}, CodeID, "/steps/1/id"},
		"reserved":  {map[string]any{"id": "params", "type": "pause", "message": "m"}, CodeID, "/steps/1/id"},
		"duplicate": {map[string]any{"id": "check", "type": "pause", "message": "m"}, CodeID, "/steps/1/id"},
		"missing":   {map[string]any{"type": "pause", "message": "m"}, CodeRequired, "/steps/1/id"},
		"nested duplicate": {map[string]any{"id": "r", "type": "repeat", "max_count": 2,
			"until": map[string]any{"output": "check.n", "eq": 0},
			"steps": []any{map[string]any{"id": "check", "type": "pause", "message": "m"}}}, CodeID, "/steps/1/steps/0/id"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			wantFinding(t, ValidateFile(encode(t, withSteps(c.step))), wsvalidate.Error, c.code, c.path)
		})
	}
	t.Run("no steps", func(t *testing.T) {
		doc := base()
		doc["steps"] = []any{}
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeRequired, "/steps")
	})
}

func TestTypes(t *testing.T) {
	for _, ok := range []string{"text", "integer", "numeric", "boolean", "timestamp", "session_id", "list<text>", "list<session_id>"} {
		if _, err := ParseType(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "int", "list<list<text>>", "list<>", "TEXT"} {
		if _, err := ParseType(bad); err == nil {
			t.Errorf("%q: accepted", bad)
		}
	}
	doc := base()
	doc["parameters"] = map[string]any{"x": map[string]any{"type": "varchar"}}
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeType, "/parameters/x/type")

	doc = base()
	doc["parameters"] = map[string]any{"x": map[string]any{"type": "integer", "default": "seven"}}
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeValue, "/parameters/x/default")

	doc = base()
	doc["parameters"] = map[string]any{"x": map[string]any{"type": "session_id", "default": "12a"}}
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeValue, "/parameters/x/default")

	doc = withSteps(map[string]any{"id": "q", "type": "query", "returns": map[string]any{"a": "money"}})
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeType, "/steps/1/returns/a")

	doc = withSteps(map[string]any{"id": "m", "type": "manual", "instruction": "i", "records": map[string]any{"At": "timestamp"}})
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeName, "/steps/1/records/At")
}

func waitStep() map[string]any {
	return map[string]any{"id": "w", "type": "wait", "returns": map[string]any{"state": "text"},
		"until":    map[string]any{"all_of": "w.state", "eq": "completed"},
		"interval": "30s", "deadline": "3h", "max_count": 90}
}

func repeatStep() map[string]any {
	return map[string]any{"id": "r", "type": "repeat", "max_count": 5,
		"until": map[string]any{"output": "inner.n", "eq": 0},
		"steps": []any{map[string]any{"id": "inner", "type": "query", "returns": map[string]any{"n": "integer"}}}}
}

func TestLimits(t *testing.T) {
	query := func(k string, v any) map[string]any {
		return map[string]any{"id": "q", "type": "query", "returns": map[string]any{"a": "text"}, k: v}
	}
	mod := func(s map[string]any, k string, v any) map[string]any { s[k] = v; return s }
	cases := map[string]struct {
		step map[string]any
		code string
		path string
	}{
		"timeout over the cap":    {query("timeout", "31m"), CodeLimit, "/steps/1/timeout"},
		"timeout not a duration":  {query("timeout", "5 minutes"), CodeValue, "/steps/1/timeout"},
		"timeout negative":        {query("timeout", "-1m"), CodeLimit, "/steps/1/timeout"},
		"max_rows over the cap":   {query("max_rows", 10001), CodeLimit, "/steps/1/max_rows"},
		"max_rows zero":           {query("max_rows", 0), CodeLimit, "/steps/1/max_rows"},
		"wait interval too short": {mod(waitStep(), "interval", "5s"), CodeLimit, "/steps/1/interval"},
		"wait deadline over 24h":  {mod(waitStep(), "deadline", "25h"), CodeLimit, "/steps/1/deadline"},
		"wait max_count over":     {mod(waitStep(), "max_count", 1001), CodeLimit, "/steps/1/max_count"},
		"repeat max_count over":   {mod(repeatStep(), "max_count", 101), CodeLimit, "/steps/1/max_count"},
		"compile deadline over":   {map[string]any{"id": "c", "type": "compile", "deadline": "48h"}, CodeLimit, "/steps/1/deadline"},
		"ddl timeout over":        {map[string]any{"id": "d", "type": "ddl", "timeout": "1h"}, CodeLimit, "/steps/1/timeout"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			wantFinding(t, ValidateFile(encode(t, withSteps(c.step))), wsvalidate.Error, c.code, c.path)
		})
	}
	t.Run("at the caps", func(t *testing.T) {
		w := waitStep()
		w["interval"], w["deadline"], w["max_count"], w["timeout"], w["max_rows"] = "10s", "24h", 1000, "30m", 10000
		r := repeatStep()
		r["max_count"] = 100
		wantClean(t, ValidateFile(encode(t, withSteps(w, r))))
	})
}

func TestRequiredFields(t *testing.T) {
	drop := func(s map[string]any, k string) map[string]any { delete(s, k); return s }
	cases := map[string]struct {
		step map[string]any
		path string
	}{
		"query returns":          {map[string]any{"id": "q", "type": "query"}, "/steps/1/returns"},
		"wait until":             {drop(waitStep(), "until"), "/steps/1/until"},
		"wait interval":          {drop(waitStep(), "interval"), "/steps/1/interval"},
		"wait deadline":          {drop(waitStep(), "deadline"), "/steps/1/deadline"},
		"wait max_count":         {drop(waitStep(), "max_count"), "/steps/1/max_count"},
		"wait returns":           {drop(waitStep(), "returns"), "/steps/1/returns"},
		"repeat until":           {drop(repeatStep(), "until"), "/steps/1/until"},
		"repeat max_count":       {drop(repeatStep(), "max_count"), "/steps/1/max_count"},
		"repeat steps":           {drop(drop(repeatStep(), "steps"), "until"), "/steps/1/steps"},
		"assert condition":       {map[string]any{"id": "a", "type": "assert"}, "/steps/1/condition"},
		"pause message":          {map[string]any{"id": "p", "type": "pause"}, "/steps/1/message"},
		"manual instruction":     {map[string]any{"id": "m", "type": "manual"}, "/steps/1/instruction"},
		"start_pipeline row":     {map[string]any{"id": "s", "type": "start_pipeline"}, "/steps/1/row"},
		"put_schema_event event": {map[string]any{"id": "e", "type": "put_schema_event"}, "/steps/1/event"},
		"step type":              {map[string]any{"id": "x"}, "/steps/1/type"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			wantFinding(t, ValidateFile(encode(t, withSteps(c.step))), wsvalidate.Error, CodeRequired, c.path)
		})
	}
	t.Run("on_fail", func(t *testing.T) {
		doc := withSteps(map[string]any{"id": "p", "type": "pause", "message": "m", "on_fail": "retry"})
		wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeValue, "/steps/1/on_fail")
	})
	t.Run("every other type with its fields", func(t *testing.T) {
		doc := withSteps(
			map[string]any{"id": "d", "type": "ddl", "preview": true, "confirm": true, "on_fail": "pause"},
			map[string]any{"id": "a", "type": "assert", "condition": map[string]any{"output": "check.n", "ge": 0}, "message": "m"},
			waitStep(), repeatStep(),
			map[string]any{"id": "p", "type": "pause", "message": "m", "description": "d"},
			map[string]any{"id": "m", "type": "manual", "instruction": "i", "records": map[string]any{"at": "timestamp", "sid": "session_id"}},
			map[string]any{"id": "sp", "type": "start_pending"},
			map[string]any{"id": "pipe", "type": "start_pipeline", "confirm": false,
				"row": map[string]any{"client": "acme", "pipeline_config_key": 3, "input_session_id": map[string]any{"from": "m.sid"}}},
			map[string]any{"id": "ev", "type": "put_schema_event", "event": map[string]any{"from": "check.state"}, "key": "k"},
			map[string]any{"id": "c", "type": "compile"},
			map[string]any{"id": "after", "type": "assert", "condition": map[string]any{"output": "pipe.session_id", "ne": "0"}},
		)
		wantClean(t, ValidateFile(encode(t, doc)))
	})
}

func TestEveryDdlPausesForAPerson(t *testing.T) {
	doc := withSteps(map[string]any{"id": "d", "type": "ddl", "confirm": false})
	wantFinding(t, ValidateFile(encode(t, doc)), wsvalidate.Error, CodeConfirm, "/steps/1/confirm")

	for _, confirm := range []any{nil, true} {
		s := map[string]any{"id": "d", "type": "ddl"}
		if confirm != nil {
			s["confirm"] = confirm
		}
		wantClean(t, ValidateFile(encode(t, withSteps(s))))
	}
	// And the accessor ignores the field for a ddl, so an executor built on it
	// cannot be talked out of the pause by a runbook that skipped validation.
	f := false
	if !(&Step{Type: StepDDL, Confirm: &f}).ConfirmRequired() {
		t.Error("a ddl with confirm false: ConfirmRequired is false")
	}
	if (&Step{Type: StepStartPipeline, Confirm: &f}).ConfirmRequired() {
		t.Error("start_pipeline with confirm false: ConfirmRequired is true")
	}
	if !(&Step{Type: StepCompile}).ConfirmRequired() || (&Step{Type: StepQuery}).ConfirmRequired() {
		t.Error("defaults: compile confirms, query does not")
	}
}

func TestNoRepeatInsideARepeat(t *testing.T) {
	r := repeatStep()
	r["steps"] = append(r["steps"].([]any), map[string]any{"id": "r2", "type": "repeat", "max_count": 2,
		"until": map[string]any{"output": "inner.n", "eq": 0},
		"steps": []any{map[string]any{"id": "p", "type": "pause", "message": "m"}}})
	wantFinding(t, ValidateFile(encode(t, withSteps(r))), wsvalidate.Error, CodeNesting, "/steps/1/steps/1")
}

func TestRefs(t *testing.T) {
	assert := func(cond map[string]any) map[string]any {
		return map[string]any{"id": "a", "type": "assert", "condition": cond}
	}
	t.Run("resolve", func(t *testing.T) {
		doc := withSteps(
			assert(map[string]any{"all": []any{
				map[string]any{"output": "params.since_utc", "lt": "2030-01-01T00:00:00Z"},
				map[string]any{"any_of": "params.ids", "eq": "2"},
				map[string]any{"output": "check.state", "eq": "ready"},
			}}),
			waitStep(),   // until reads its own output
			repeatStep(), // until reads its nested step
			map[string]any{"id": "after", "type": "assert", // reads a nested step after the repeat
				"condition": map[string]any{"output": "inner.n", "eq": 0}},
		)
		wantClean(t, ValidateFile(encode(t, doc)))
	})
	refused := map[string]struct {
		doc  map[string]any
		path string
	}{
		"unknown parameter": {withSteps(assert(map[string]any{"output": "params.nope", "eq": 1})), "/steps/1/condition/output"},
		"unknown step":      {withSteps(assert(map[string]any{"output": "nope.n", "eq": 1})), "/steps/1/condition/output"},
		"unknown column":    {withSteps(assert(map[string]any{"output": "check.nope", "eq": 1})), "/steps/1/condition/output"},
		"not a ref":         {withSteps(assert(map[string]any{"output": "check", "eq": 1})), "/steps/1/condition/output"},
		"a later step": {withSteps(
			assert(map[string]any{"output": "later.n", "eq": 1}),
			map[string]any{"id": "later", "type": "query", "returns": map[string]any{"n": "integer"}}), "/steps/1/condition/output"},
		"a step's when reads itself": {withSteps(map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"},
			"when": map[string]any{"output": "q.n", "eq": 0}}), "/steps/1/when/output"},
		"a wait binds its own output": {withSteps(func() map[string]any {
			w := waitStep()
			w["bind"] = map[string]any{"s": "w.state"}
			return w
		}()), "/steps/1/bind/s"},
		"a nested step reads a later nested step": {withSteps(map[string]any{"id": "r", "type": "repeat", "max_count": 2,
			"until": map[string]any{"output": "b.n", "eq": 0},
			"steps": []any{
				map[string]any{"id": "a", "type": "assert", "condition": map[string]any{"output": "b.n", "eq": 0}},
				map[string]any{"id": "b", "type": "query", "returns": map[string]any{"n": "integer"}},
			}}), "/steps/1/steps/0/condition/output"},
		"a row from nowhere": {withSteps(map[string]any{"id": "s", "type": "start_pipeline",
			"row": map[string]any{"client": map[string]any{"from": "x.y"}}}), "/steps/1/row/client/from"},
		"an event from nowhere": {withSteps(map[string]any{"id": "e", "type": "put_schema_event",
			"event": map[string]any{"from": "params.event"}}), "/steps/1/event/from"},
		"a bind to nowhere": {withSteps(map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"},
			"bind": map[string]any{"x": "check.missing"}}), "/steps/1/bind/x"},
	}
	for name, c := range refused {
		t.Run(name, func(t *testing.T) {
			wantFinding(t, ValidateFile(encode(t, c.doc)), wsvalidate.Error, CodeRef, c.path)
		})
	}
}

func TestConditionGrammar(t *testing.T) {
	assert := func(cond any) map[string]any {
		return withSteps(map[string]any{"id": "a", "type": "assert", "condition": cond})
	}
	refused := map[string]struct {
		cond any
		code string
		path string
	}{
		"not an object":                {"check.n == 0", CodeCondition, "/steps/1/condition"},
		"two refs":                     {map[string]any{"output": "check.n", "count": "check.n", "eq": 0}, CodeCondition, "/steps/1/condition/output"},
		"two operators":                {map[string]any{"output": "check.n", "eq": 0, "ne": 1}, CodeCondition, "/steps/1/condition/ne"},
		"no operator":                  {map[string]any{"output": "check.n"}, CodeCondition, "/steps/1/condition"},
		"no ref":                       {map[string]any{"eq": 0}, CodeCondition, "/steps/1/condition"},
		"unknown key":                  {map[string]any{"output": "check.n", "eq": 0, "matches": "x"}, CodeCondition, "/steps/1/condition/matches"},
		"unknown operator":             {map[string]any{"output": "check.n", "like": "x"}, CodeCondition, "/steps/1/condition/like"},
		"in takes an array":            {map[string]any{"output": "check.state", "in": "ready"}, CodeCondition, "/steps/1/condition/in"},
		"empty takes a bool":           {map[string]any{"output": "check.state", "empty": "yes"}, CodeCondition, "/steps/1/condition/empty"},
		"count and empty":              {map[string]any{"count": "check.state", "empty": true}, CodeCondition, "/steps/1/condition/empty"},
		"eq takes a scalar":            {map[string]any{"output": "check.state", "eq": []any{"a"}}, CodeCondition, "/steps/1/condition/eq"},
		"all is not empty":             {map[string]any{"all": []any{}}, CodeCondition, "/steps/1/condition/all"},
		"all beside a leaf":            {map[string]any{"all": []any{map[string]any{"output": "check.n", "eq": 0}}, "output": "check.n"}, CodeCondition, "/steps/1/condition"},
		"not of garbage":               {map[string]any{"not": 3}, CodeCondition, "/steps/1/condition/not"},
		"nested error":                 {map[string]any{"any": []any{map[string]any{"output": "check.n", "eq": 0}, map[string]any{"eq": 1}}}, CodeCondition, "/steps/1/condition/any/1"},
		"operand of the wrong type":    {map[string]any{"output": "check.n", "eq": "zero"}, CodeValue, "/steps/1/condition/eq"},
		"in element of the wrong type": {map[string]any{"output": "check.n", "in": []any{1, "two"}}, CodeValue, "/steps/1/condition/in"},
		"count against text":           {map[string]any{"count": "check.state", "gt": "many"}, CodeValue, "/steps/1/condition/gt"},
		"output of a declared list":    {map[string]any{"output": "params.ids", "eq": "1"}, CodeCondition, "/steps/1/condition"},
	}
	for name, c := range refused {
		t.Run(name, func(t *testing.T) {
			wantFinding(t, ValidateFile(encode(t, assert(c.cond))), wsvalidate.Error, c.code, c.path)
		})
	}
	t.Run("accepted", func(t *testing.T) {
		for _, cond := range []any{
			map[string]any{"output": "check.n", "eq": nil},
			map[string]any{"output": "params.ids", "empty": false},
			map[string]any{"count": "params.ids", "in": []any{1, 2}},
			map[string]any{"not": map[string]any{"any_of": "check.state", "in": []any{"failed", "error"}}},
			map[string]any{"output": "params.since_utc", "ge": "2026-11-06T17:00:00Z"},
		} {
			wantClean(t, ValidateFile(encode(t, assert(cond))))
		}
	})
}

// --- SQL files ---------------------------------------------------------------

func sqlDoc(steps ...map[string]any) *Runbook {
	doc := base()
	doc["steps"] = []any{}
	for _, s := range steps {
		doc["steps"] = append(doc["steps"].([]any), s)
	}
	b, _ := json.Marshal(doc)
	rb, _ := Decode(b)
	return rb
}

func TestSqlFiles(t *testing.T) {
	q := map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"}}
	d := map[string]any{"id": "d", "type": "ddl"}
	cases := map[string]struct {
		step  map[string]any
		files map[string]string
		sev   wsvalidate.Severity
		code  string
		line  int
	}{
		"missing":         {q, map[string]string{}, wsvalidate.Error, CodeSQLFile, 0},
		"two statements":  {q, map[string]string{"rb/q.sql": "SELECT 1 AS n; SELECT 2 AS n;"}, wsvalidate.Error, CodeSQLStatements, 0},
		"no statement":    {d, map[string]string{"rb/d.sql": "-- nothing\n"}, wsvalidate.Error, CodeSQLStatements, 0},
		"unterminated":    {q, map[string]string{"rb/q.sql": "SELECT 'a AS n\n"}, wsvalidate.Error, "sqlUnterminated", 1},
		"unresolved name": {q, map[string]string{"rb/q.sql": "SELECT 1 AS n\nWHERE x = :nope"}, wsvalidate.Error, CodeSQLBind, 2},
		"$SESSIONID":      {q, map[string]string{"rb/q.sql": "SELECT count(*) AS n FROM t\nWHERE session_id = $SESSIONID"}, wsvalidate.Error, CodeSQLSubst, 2},
		"${...}":          {q, map[string]string{"rb/q.sql": "SELECT ${col} AS n"}, wsvalidate.Error, CodeSQLSubst, 1},
		"{{...}}":         {q, map[string]string{"rb/q.sql": "SELECT {{ col }} AS n"}, wsvalidate.Error, CodeSQLSubst, 1},
		"positional $1":   {q, map[string]string{"rb/q.sql": "SELECT $1 AS n"}, wsvalidate.Error, CodeSQLSubst, 1},
		"inside a string": {q, map[string]string{"rb/q.sql": "SELECT count(*) AS n FROM t WHERE a = '$SESSIONID'"}, wsvalidate.Warning, CodeSQLSubst, 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fs := Validate(sqlDoc(c.step), "", c.files)
			for _, f := range fs {
				if f.Code == c.code && f.Severity == c.sev && f.Path == "/steps/0/sql" && f.Line == c.line {
					if !strings.Contains(f.Message, "runbooks/rb/") {
						t.Errorf("the message does not name the file: %s", f.Message)
					}
					return
				}
			}
			t.Errorf("want %s %s at /steps/0/sql line %d; got %s", c.sev, c.code, c.line, describe(fs))
		})
	}
	accepted := map[string]struct {
		step  map[string]any
		files map[string]string
	}{
		"casts and comments": {q, map[string]string{"rb/q.sql": "-- :not_a_name $SESSIONID {{x}}\n/* :nor_this */\nSELECT :since_utc::date AS n, 'it''s :quoted' AS s, \"col:x\", $$ :body $SESSIONID $$"}},
		"a list":             {q, map[string]string{"rb/q.sql": "SELECT count(*) AS n FROM t WHERE session_id = ANY(:ids)"}},
		"a ddl script":       {d, map[string]string{"rb/d.sql": "CREATE TABLE s.t (a int);\nINSERT INTO s.t SELECT 1 WHERE now() > :since_utc;\n"}},
		"another path": {map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"}, "sql": "shared/count.sql"},
			map[string]string{"shared/count.sql": "SELECT 1 AS n"}},
		"a bound output": {map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"}, "bind": map[string]any{"x": "params.since_utc"}},
			map[string]string{"rb/q.sql": "SELECT 1 AS n WHERE now() > :x"}},
	}
	for name, c := range accepted {
		t.Run(name, func(t *testing.T) {
			fs := Validate(sqlDoc(c.step), "", c.files)
			if len(fs) > 0 {
				t.Errorf("want no findings; got %s", describe(fs))
			}
		})
	}
	t.Run("an unused bind is a warning", func(t *testing.T) {
		s := map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"}, "bind": map[string]any{"x": "params.since_utc"}}
		fs := Validate(sqlDoc(s), "", map[string]string{"rb/q.sql": "SELECT 1 AS n"})
		wantFinding(t, fs, wsvalidate.Warning, CodeSQLBind, "/steps/0/bind/x")
		wantClean(t, fs)
	})
	t.Run("a bad sql path", func(t *testing.T) {
		for _, p := range []string{"/etc/x.sql", "../x.sql", "a/../../x.sql", "x.txt", "a//b.sql", "a\\b.sql"} {
			s := map[string]any{"id": "q", "type": "query", "returns": map[string]any{"n": "integer"}, "sql": p}
			wantFinding(t, ValidateFile(encode(t, withSteps(s))), wsvalidate.Error, CodeSQLFile, "/steps/1/sql")
		}
	})
	t.Run("nil files skips the SQL checks", func(t *testing.T) {
		if fs := Validate(sqlDoc(q), "", nil); len(fs) > 0 {
			t.Errorf("got %s", describe(fs))
		}
	})
	t.Run("SQLPaths", func(t *testing.T) {
		r := repeatStep()
		rb := sqlDoc(q, d, r, map[string]any{"id": "q2", "type": "query", "returns": map[string]any{"n": "integer"}, "sql": "rb/q.sql"})
		got := rb.SQLPaths()
		want := []string{"rb/q.sql", "rb/d.sql", "rb/inner.sql"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

// --- the format page --------------------------------------------------------

// formatPage is the documentation page that specifies the format. Its example
// is tested, so the page cannot drift from what Validate accepts.
const formatPage = "../../docs/src/content/docs/developer/engine/runbook-format.md"

var fence = regexp.MustCompile("(?s)```(json|sql)\n(.*?)```")

func TestTheReadmeExampleValidates(t *testing.T) {
	readme, err := os.ReadFile(formatPage)
	if err != nil {
		t.Fatal(err)
	}
	var doc string
	files := map[string]string{}
	for _, m := range fence.FindAllStringSubmatch(string(readme), -1) {
		switch m[1] {
		case "json":
			if doc == "" {
				doc = m[2]
			}
		case "sql":
			first, rest, _ := strings.Cut(m[2], "\n")
			if name, ok := strings.CutPrefix(first, "-- file: "); ok {
				files[name] = rest
			}
		}
	}
	if doc == "" || len(files) == 0 {
		t.Fatalf("no example found: %d json, %d sql", len(doc), len(files))
	}
	rb, fs := Load("runbooks/pipeline_health_check.rb.json", []byte(doc), files)
	if len(fs) > 0 {
		t.Fatalf("the format page's example: %s", describe(fs))
	}
	if len(rb.SQLPaths()) != len(files) {
		t.Errorf("the example reads %v and the format page has %d files", rb.SQLPaths(), len(files))
	}
	// Every step type that runs without AWS appears.
	types := map[StepType]bool{}
	rb.Walk(func(s, _ *Step) bool { types[s.Type] = true; return true })
	for _, want := range []StepType{StepQuery, StepAssert, StepWait, StepDDL, StepPause, StepManual, StepRepeat} {
		if !types[want] {
			t.Errorf("the example has no %s step", want)
		}
	}
	// And it binds.
	params, err := ParameterValues(rb, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := MapEnv{}
	for k, v := range params {
		env[ParamRef(k)] = v
	}
	stmts, err := Bind(files["pipeline_health_check/scratch_counts.sql"], rb.Step("scratch_counts"), env)
	if err != nil || len(stmts) != 3 || len(stmts[2].Args) != 1 {
		t.Errorf("scratch_counts: %v, %+v", err, stmts)
	}
}

// --- round trip --------------------------------------------------------------

func TestARunbookRoundTrips(t *testing.T) {
	readme, _ := os.ReadFile(formatPage)
	m := fence.FindStringSubmatch(string(readme))
	rb, fs := Decode([]byte(m[2]))
	if len(fs) > 0 {
		t.Fatal(describe(fs))
	}
	b, err := json.Marshal(rb)
	if err != nil {
		t.Fatal(err)
	}
	again, fs := Decode(b)
	if len(fs) > 0 {
		t.Fatalf("re-encoded: %s\n%s", describe(fs), b)
	}
	b2, _ := json.Marshal(again)
	if string(b) != string(b2) {
		t.Errorf("not stable:\n%s\n%s", b, b2)
	}
	// And the plain decoder reads it too (Condition and RowValue unmarshal).
	var plain Runbook
	if err := json.Unmarshal(b, &plain); err != nil {
		t.Fatal(err)
	}
	b3, _ := json.Marshal(&plain)
	if string(b3) != string(b) {
		t.Errorf("json.Unmarshal differs:\n%s\n%s", b, b3)
	}
}
