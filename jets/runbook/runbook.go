// Package runbook is the workspace runbook format: a `runbooks/<name>.rb.json`
// file listing steps that the apiserver executes on a person's behalf, with
// each SQL step's statement in a sibling `.sql` file.
//
// The package holds the format and nothing that runs it: the types, a strict
// decoder, the validator the save path and the executor share, condition
// evaluation, the binding of `:name` placeholders to PostgreSQL parameters, and
// the types of the `.rbr.json` result a run is exported as. The executor lives
// in jets/datatable and imports this package; this package imports no database
// driver, so a value reaches SQL only through the Args of a BoundStatement.
//
// README.md beside this file documents the format with an example that a test
// decodes and validates.
package runbook

import (
	"encoding/json"
	"path"
	"time"
)

// FormatVersion is the only `format` a runbook file may declare today.
const FormatVersion = 1

// File naming.
const (
	// Dir is the workspace directory runbooks live in. SQL paths in a runbook
	// are relative to it.
	Dir = "runbooks"
	// FileSuffix is a runbook's file suffix: `runbooks/<name>.rb.json`.
	FileSuffix = ".rb.json"
	// ResultSuffix is the suffix of an exported run result. Results are never
	// written into the workspace.
	ResultSuffix = ".rbr.json"
)

// Limits. A runbook may lower a default; it may not exceed a cap.
const (
	DefaultTimeout         = 5 * time.Minute
	MaxTimeout             = 30 * time.Minute
	DefaultMaxRows         = 1000
	MaxMaxRows             = 10000
	MinWaitInterval        = 10 * time.Second
	MaxDeadline            = 24 * time.Hour
	MaxWaitCount           = 1000
	MaxRepeatCount         = 100
	DefaultCompileDeadline = 15 * time.Minute
)

// StepType names a step's kind. The vocabulary is closed: it is the security
// boundary of the feature, and a new type needs its own assessment.
type StepType string

const (
	StepQuery          StepType = "query"
	StepDDL            StepType = "ddl"
	StepAssert         StepType = "assert"
	StepWait           StepType = "wait"
	StepRepeat         StepType = "repeat"
	StepPause          StepType = "pause"
	StepManual         StepType = "manual"
	StepStartPending   StepType = "start_pending"
	StepStartPipeline  StepType = "start_pipeline"
	StepPutSchemaEvent StepType = "put_schema_event"
	StepCompile        StepType = "compile"
)

// HasSQL reports whether a step of this type reads a `.sql` file.
func (t StepType) HasSQL() bool {
	return t == StepQuery || t == StepDDL || t == StepWait
}

// OnFail values.
const (
	OnFailStop  = "stop"
	OnFailPause = "pause"
)

// Runbook is one `.rb.json` file.
type Runbook struct {
	Format      int                  `json:"format"`
	Runbook     string               `json:"runbook"`
	Description string               `json:"description,omitempty"`
	Parameters  map[string]Parameter `json:"parameters,omitempty"`
	Steps       []*Step              `json:"steps"`
}

// Parameter is a value set when a run starts. Default, when present, is the
// JSON value as written; ParameterValues coerces it to Type.
type Parameter struct {
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
}

// Step is one step of any type. Which fields a type may carry is enforced by
// Decode (an unknown or foreign field is an error); a Step built in code is
// checked by Validate for the fields that matter.
type Step struct {
	ID          string     `json:"id"`
	Type        StepType   `json:"type"`
	Description string     `json:"description,omitempty"`
	When        *Condition `json:"when,omitempty"`
	OnFail      string     `json:"on_fail,omitempty"`

	// query, ddl, wait
	SQL     string            `json:"sql,omitempty"`
	Returns map[string]string `json:"returns,omitempty"` // query, wait
	Bind    map[string]string `json:"bind,omitempty"`
	Timeout string            `json:"timeout,omitempty"`
	MaxRows *int              `json:"max_rows,omitempty"` // query, wait

	// ddl
	Preview bool `json:"preview,omitempty"`

	// ddl, start_pipeline, put_schema_event, compile
	Confirm *bool `json:"confirm,omitempty"`

	// assert
	Condition *Condition `json:"condition,omitempty"`
	// assert, pause
	Message string `json:"message,omitempty"`

	// wait, repeat
	Until    *Condition `json:"until,omitempty"`
	Interval string     `json:"interval,omitempty"`
	MaxCount *int       `json:"max_count,omitempty"`
	// wait, compile
	Deadline string `json:"deadline,omitempty"`

	// repeat
	Steps []*Step `json:"steps,omitempty"`

	// manual
	Instruction string            `json:"instruction,omitempty"`
	Records     map[string]string `json:"records,omitempty"`

	// start_pipeline
	Row map[string]RowValue `json:"row,omitempty"`

	// put_schema_event
	Event *FromRef `json:"event,omitempty"`
	Key   string   `json:"key,omitempty"`
}

// FromRef is `{"from": "<ref>"}`: a value taken from a parameter or an earlier
// step's output.
type FromRef struct {
	From string `json:"from"`
}

// RowValue is one column of a start_pipeline row: a JSON literal (string,
// number, boolean or null) or a FromRef.
type RowValue struct {
	// Literal is the literal as written, when From is empty.
	Literal json.RawMessage
	// From is the ref, when the value is `{"from": ...}`.
	From string
}

// MarshalJSON writes the value back in the form it was written.
func (v RowValue) MarshalJSON() ([]byte, error) {
	if v.From != "" {
		return json.Marshal(FromRef{From: v.From})
	}
	if len(v.Literal) == 0 {
		return []byte("null"), nil
	}
	return v.Literal, nil
}

// UnmarshalJSON reads either form, strictly.
func (v *RowValue) UnmarshalJSON(b []byte) error {
	var fs findings
	*v = parseRowValue(b, "", &fs)
	return fs.err()
}

// SQLPath is the step's SQL file, relative to Dir: its `sql` field, or
// `<runbook>/<id>.sql`.
func (s *Step) SQLPath(runbook string) string {
	if s.SQL != "" {
		return s.SQL
	}
	return path.Join(runbook, s.ID+".sql")
}

// TimeoutOrDefault is the step's statement timeout. It assumes a validated
// step: an unparsable value yields the default.
func (s *Step) TimeoutOrDefault() time.Duration {
	if d, err := time.ParseDuration(s.Timeout); err == nil && s.Timeout != "" {
		return d
	}
	return DefaultTimeout
}

// MaxRowsOrDefault is the step's row cap.
func (s *Step) MaxRowsOrDefault() int {
	if s.MaxRows != nil {
		return *s.MaxRows
	}
	return DefaultMaxRows
}

// IntervalDuration is a wait's or repeat's interval, 0 when absent.
func (s *Step) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(s.Interval)
	return d
}

// DeadlineOrDefault is a wait's deadline, or a compile's (default 15m).
func (s *Step) DeadlineOrDefault() time.Duration {
	if d, err := time.ParseDuration(s.Deadline); err == nil && s.Deadline != "" {
		return d
	}
	if s.Type == StepCompile {
		return DefaultCompileDeadline
	}
	return 0
}

// OnFailOrDefault is `stop` unless the step says `pause`.
func (s *Step) OnFailOrDefault() string {
	if s.OnFail == OnFailPause {
		return OnFailPause
	}
	return OnFailStop
}

// ConfirmRequired reports whether the step waits for a person to accept it
// before it runs. A ddl always does: Validate refuses `confirm: false` on one,
// and this returns true regardless.
func (s *Step) ConfirmRequired() bool {
	switch s.Type {
	case StepDDL:
		return true
	case StepStartPipeline, StepPutSchemaEvent, StepCompile:
		return s.Confirm == nil || *s.Confirm
	}
	return false
}

// OutputTypes are the outputs the step declares, by column, with their
// declared types: `returns` for query and wait, `records` for manual,
// `session_id` for start_pipeline. Other types have none.
func (s *Step) OutputTypes() map[string]Type {
	out := map[string]Type{}
	var decl map[string]string
	switch s.Type {
	case StepQuery, StepWait:
		decl = s.Returns
	case StepManual:
		decl = s.Records
	case StepStartPipeline:
		out["session_id"] = Type{Base: TypeSessionID}
		return out
	}
	for name, t := range decl {
		if typ, err := ParseType(t); err == nil {
			out[name] = typ
		}
	}
	return out
}

// Step returns the step with this id, nested steps included, or nil.
func (rb *Runbook) Step(id string) *Step {
	var found *Step
	rb.Walk(func(s, _ *Step) bool {
		if s.ID == id {
			found = s
			return false
		}
		return true
	})
	return found
}

// Walk calls fn for every step in document order, a repeat before its nested
// steps; parent is the enclosing repeat or nil. It stops when fn returns false.
func (rb *Runbook) Walk(fn func(step, parent *Step) bool) {
	for _, s := range rb.Steps {
		if s == nil {
			continue
		}
		if !fn(s, nil) {
			return
		}
		for _, n := range s.Steps {
			if n == nil {
				continue
			}
			if !fn(n, s) {
				return
			}
		}
	}
}

// SQLPaths lists the SQL files the runbook reads, relative to Dir, in step
// order and without repeats: what a caller loads before calling Validate.
func (rb *Runbook) SQLPaths() []string {
	var out []string
	seen := map[string]bool{}
	rb.Walk(func(s, _ *Step) bool {
		if s.Type.HasSQL() {
			p := s.SQLPath(rb.Runbook)
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
		return true
	})
	return out
}
