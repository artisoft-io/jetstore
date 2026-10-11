package rulestest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The result file is the whole contract between the harness process and the
// apiserver that launched it (definition §5.4): the harness writes one, opens no
// database connection, and the apiserver reads it into
// jetsapi.rules_unit_test_execution and jetsapi.rules_unit_test_check.
//
// JSON names equal the column names wherever a field has a column. The log is
// not in the file: it is the process's standard output and error, captured by
// the apiserver.

// Test statuses. queued and executing are the apiserver's; a result file
// carries only the three terminal ones.
const (
	StatusPass  = "pass"
	StatusFail  = "fail"
	StatusError = "error"
)

// Check kinds (rules_unit_test_check.kind).
const (
	KindExpected = "expected"
	KindExcluded = "excluded"
	KindCount    = "count"
)

// Check outcomes (rules_unit_test_check.outcome).
const (
	OutcomePass = "pass"
	OutcomeFail = "fail"
)

// Engines (rules_unit_test_execution.engine). A compare_engines run records
// EngineBoth.
const (
	EngineNative = "native"
	EngineGo     = "go"
	EngineBoth   = "native+go"
)

// Result is the harness's result file.
type Result struct {
	TestPath string `json:"test_path"`
	TestName string `json:"test_name"`
	// Status is pass, fail or error; Reason says why when it is not pass.
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	// Engine is native, go, or native+go for compare_engines.
	Engine string `json:"engine"`
	// LegacyMode is legacy, fixed, fixed-1 or fixed-2 (definition §8.5.6).
	LegacyMode string `json:"legacy_mode"`
	// LoadMs is building the engine (summed over engines for compare_engines),
	// RunMs running the sessions and evaluating the checks, and DurationMs the
	// harness's wall time from start to writing this file.
	LoadMs     int64 `json:"load_ms"`
	RunMs      int64 `json:"run_ms"`
	DurationMs int64 `json:"duration_ms"`
	// Sessions is the number of input sessions run; Loops the rule loop
	// iterations summed over sessions and rule sets.
	Sessions int `json:"sessions"`
	Loops    int `json:"loops"`
	// Checks has one entry per check per session (rules_unit_test_check rows).
	Checks []CheckResult `json:"checks"`
	// SessionResults is the per-session detail behind Status, Reason and Loops.
	SessionResults []SessionResult `json:"session_results,omitempty"`
	// Findings are the validation findings of an invalid test definition.
	Findings []wsvalidate.Finding `json:"findings,omitempty"`
	// EngineDifferences lists what differed between the engines, compare_engines only.
	EngineDifferences []string `json:"engine_differences,omitempty"`
}

// CheckResult is one rules_unit_test_check row.
type CheckResult struct {
	SessionName string `json:"session_name"`
	Kind        string `json:"kind"`
	// Check is the check as written, compact JSON.
	Check   string `json:"check"`
	Outcome string `json:"outcome"`
	// Found is what the check saw: the binding that matched, or the count.
	Found string `json:"found"`
	// Engine is set only on a compare_engines run, where each check appears
	// once per engine.
	Engine string `json:"engine,omitempty"`
}

// SessionResult is how one input session ran on one engine.
type SessionResult struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	// Status is error when the rules did not run to completion, else pass or
	// fail from the session's checks.
	Status         string   `json:"status"`
	Reason         string   `json:"reason,omitempty"`
	Loops          int      `json:"loops"`
	MaxLoopReached bool     `json:"max_loop_reached,omitzero"`
	Exceptions     []string `json:"exceptions,omitempty"`
}

// WriteResultFile writes r to path atomically: a reader never sees a partial file.
func WriteResultFile(path string, r *Result) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("while encoding the result: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("while writing the result: %w", err)
	}
	_, err = tmp.Write(append(b, '\n'))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("while writing the result: %w", err)
	}
	return nil
}

// ReadResultFile reads a result file. Unknown fields are ignored, so a newer
// harness's file is readable by an older apiserver; a status that is not one of
// the three terminal statuses is an error.
func ReadResultFile(path string) (*Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("result file %s is not valid JSON: %w", path, err)
	}
	switch r.Status {
	case StatusPass, StatusFail, StatusError:
	default:
		return nil, fmt.Errorf("result file %s has status %q, want pass, fail or error", path, r.Status)
	}
	return &r, nil
}
