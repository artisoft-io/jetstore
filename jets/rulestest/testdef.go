// Package rulestest holds the contract of a rules unit test: the .tst.json test
// definition, its validation, and the result file the harness writes.
//
// **It imports only the standard library, pipesmodel and wsvalidate, and that is
// a constraint rather than a preference.** The workspace compile records every
// test definition it finds (rules_unit_test_harness AD.1), so the compile must
// import this package's validator, and compute_pipes imports jets/workspace
// (actions_start_common.go, actions_coordinate_cp.go). A validator that imported
// compute_pipes would close the cycle. The code that runs a test — the engine,
// the session, the pattern matcher — lives in jets/rulestest/harness, which may
// import compute_pipes because nothing below it imports the harness.
//
// That is also why jetrules_config decodes into JetrulesConfig rather than into
// compute_pipes.JetrulesSpec: JetrulesConfig carries, under the same JSON names,
// the JetrulesSpec fields a test uses, and refuses the ones it does not. The
// harness converts one into the other, and a test there fails when JetrulesSpec
// gains a field this package has not classified as used or refused.
package rulestest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/pipesmodel"
)

// TestDefinition is one .tst.json file. Its identity is its path in the
// workspace; TestName is a label (project definition Q-6).
type TestDefinition struct {
	TestName string `json:"test_name"`
	Comment  string `json:"comment,omitempty"`
	// Legacy selects the engine's retraction behaviour for this test. Absent
	// means legacy, field by field. Top level only (definition §8.5.2).
	Legacy *LegacyConfig `json:"legacy,omitempty"`
	// CompareEngines runs the test on both engines, one after the other, and
	// fails when the checks or the status differ (definition Q-11). Top level only.
	CompareEngines bool `json:"compare_engines,omitzero"`
	// Expect turns a run ending in an error into the expected outcome
	// (definition §6). Top level only.
	Expect         *Expectation   `json:"expect,omitempty"`
	JetrulesConfig JetrulesConfig `json:"jetrules_config"`
	InputSessions  []InputSession `json:"input_sessions"`
	// ExpectedTriples and ExcludedTriples apply to every input session
	// (definition Q-2); a session may add its own.
	ExpectedTriples []Check `json:"expected_triples,omitempty"`
	ExcludedTriples []Check `json:"excluded_triples,omitempty"`
}

// LegacyConfig carries the two LEGACY switches, one per retraction defect kept
// on 2026-10-09 (jets/jetrules/rete/README.md). nil means legacy; false turns
// the defect off for this test. The names are the project definition's (§8.5.1).
type LegacyConfig struct {
	// Defect 1: a consequent that reads the graph is recomputed on retraction.
	RecomputeOnRetraction *bool `json:"recompute_on_retraction,omitempty"`
	// Defect 2: a withdrawn row stays in its relation and never fires again.
	WithdrawnRowStays *bool `json:"withdrawn_row_stays,omitempty"`
}

// Legacy-mode labels recorded on every result (definition §8.5.6).
const (
	LegacyModeLegacy = "legacy"  // both defects kept, as production runs
	LegacyModeFixed  = "fixed"   // both defects off
	LegacyModeFixed1 = "fixed-1" // defect 1 off, defect 2 kept
	LegacyModeFixed2 = "fixed-2" // defect 2 off, defect 1 kept
)

// RecomputeOnRetractionOn reports defect 1's switch, true (legacy) when absent.
func (l *LegacyConfig) RecomputeOnRetractionOn() bool {
	return l == nil || l.RecomputeOnRetraction == nil || *l.RecomputeOnRetraction
}

// WithdrawnRowStaysOn reports defect 2's switch, true (legacy) when absent.
func (l *LegacyConfig) WithdrawnRowStaysOn() bool {
	return l == nil || l.WithdrawnRowStays == nil || *l.WithdrawnRowStays
}

// ModeLabel is the legacy_mode recorded on the result. A nil receiver is legacy.
func (l *LegacyConfig) ModeLabel() string {
	d1, d2 := l.RecomputeOnRetractionOn(), l.WithdrawnRowStaysOn()
	switch {
	case d1 && d2:
		return LegacyModeLegacy
	case !d1 && !d2:
		return LegacyModeFixed
	case !d1:
		return LegacyModeFixed1
	default:
		return LegacyModeFixed2
	}
}

// LegacyMode is the test's legacy_mode label.
func (td *TestDefinition) LegacyMode() string {
	if td == nil {
		return LegacyModeLegacy
	}
	return td.Legacy.ModeLabel()
}

// Expectation names the outcome a test expects when it is not a pass.
// Status must be "error"; Reason, when given, must be contained in the run's
// reason (compared case-insensitively). An empty Reason accepts any error.
type Expectation struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// JetrulesConfig is the part of compute_pipes.JetrulesSpec a test uses, under
// the same JSON names (definition §3.2). The pipeline-only fields are refused
// by name; see refusedFields.
type JetrulesConfig struct {
	Comment                 string           `json:"comment,omitempty"`
	ProcessName             string           `json:"process_name"`
	UseJetRulesNative       bool             `json:"use_jet_rules_native,omitzero"`
	UseJetRulesGo           bool             `json:"use_jet_rules_go,omitzero"`
	InputRdfType            string           `json:"input_rdf_type,omitempty"`
	MaxLooping              int              `json:"max_looping,omitzero"`
	CurrentSourcePeriod     int              `json:"current_source_period,omitzero"`
	CurrentSourcePeriodDate string           `json:"current_source_period_date,omitempty"`
	CurrentSourcePeriodType string           `json:"current_source_period_type,omitempty"`
	RuleConfig              []map[string]any `json:"rule_config,omitempty"`
	IsDebug                 bool             `json:"is_debug,omitzero"`
}

// refusedFields are the JetrulesSpec fields that mean something only in a
// pipeline, with the reason a test refuses each (definition §3.2, Q-9).
func (JetrulesConfig) refusedFields() map[string]string {
	const throughput = "is a pipeline throughput setting; a test runs one engine on its own input sessions"
	const reporting = "is pipeline output; a test reports through its result file"
	return map[string]string{
		"metadata_input_sources":  "is not supported in a rules unit test (it reads from S3, Q-9); assert reference data through rule_config or an input session",
		"pool_size":               throughput,
		"max_input_count":         throughput,
		"max_rete_sessions_saved": throughput,
		"output_channels":         reporting,
		"error_channel":           reporting,
		"on_error":                reporting,
		"max_error_count":         reporting,
	}
}

// RefusedJetrulesSpecFields lists the JetrulesSpec JSON names a test refuses.
// The harness's drift test uses it with UsedJetrulesSpecFields to require that
// every JetrulesSpec field is classified.
func RefusedJetrulesSpecFields() []string {
	return sortedKeys(JetrulesConfig{}.refusedFields())
}

// UsedJetrulesSpecFields lists the JetrulesSpec JSON names a test uses.
func UsedJetrulesSpecFields() []string {
	return jsonFieldNames(JetrulesConfig{})
}

// InputSession is one RDF session: its CSVs are asserted together, the rules
// run, and the checks are evaluated against the session.
type InputSession struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
	// Input lists CSV files relative to the test definition's directory.
	Input           []string `json:"input"`
	ExpectedTriples []Check  `json:"expected_triples,omitempty"`
	ExcludedTriples []Check  `json:"excluded_triples,omitempty"`
}

// Check is one entry of expected_triples or excluded_triples, in one of three forms:
//
//   - a triple: {"s", "p", "o"} — a pattern of one;
//   - a pattern: {"pattern": [triples]} — satisfied when one binding of its
//     ?variables matches every triple;
//   - a count: {"s", "p", "count": ExpressionNode}, optionally with a "pattern"
//     binding the ?variable in "s" — the expression is evaluated against
//     {"count": n}, and with a pattern every binding must satisfy it.
//
// A subject or predicate is a resource name or a ?variable; an object is an
// ObjectTerm. A count is allowed in expected_triples only.
type Check struct {
	Comment string                     `json:"comment,omitempty"`
	S       string                     `json:"s,omitempty"`
	P       string                     `json:"p,omitempty"`
	O       *ObjectTerm                `json:"o,omitempty"`
	Pattern []Triple                   `json:"pattern,omitempty"`
	Count   *pipesmodel.ExpressionNode `json:"count,omitempty"`
}

// Kind of a check as recorded on the result: "count" for a count, otherwise
// the list it came from ("expected" or "excluded").
func (c *Check) Kind(list string) string {
	if c.Count != nil {
		return KindCount
	}
	return list
}

// Triples returns the check's triples to match: its pattern, or its single
// triple. A count's s and p are not a triple to match.
func (c *Check) Triples() []Triple {
	if len(c.Pattern) > 0 {
		return c.Pattern
	}
	if c.Count != nil {
		return nil
	}
	return []Triple{{S: c.S, P: c.P, O: c.O}}
}

// String is the check as written, compact JSON, for the result's check column.
func (c Check) String() string {
	c.Comment = ""
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<unencodable check: %v>", err)
	}
	return string(b)
}

// Triple is one triple of a pattern.
type Triple struct {
	S string      `json:"s"`
	P string      `json:"p"`
	O *ObjectTerm `json:"o"`
}

// ObjectTerm is the object of a triple: a ?variable, a typed value
// {"value": "...", "type": "..."} as rule_config writes it (definition F30), or a
// bare string, which is text — ExtractRdfNodeInfoJson's rule. A text value that
// begins with "?" is written in the typed form.
type ObjectTerm struct {
	Variable string // "?x" when the term is a variable
	Value    string
	Type     string // rdf type name as ParseRdfNodeValue reads it
	typed    bool   // written as {"value", "type"}
}

// IsVariable reports whether the term is a ?variable.
func (o *ObjectTerm) IsVariable() bool { return o != nil && o.Variable != "" }

func (o *ObjectTerm) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*o = ObjectTerm{}
		if strings.HasPrefix(s, "?") {
			o.Variable = s
		} else {
			o.Value, o.Type = s, "text"
		}
		return nil
	}
	var typed struct {
		Value string `json:"value"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(b, &typed); err != nil {
		return err
	}
	*o = ObjectTerm{Value: typed.Value, Type: typed.Type, typed: true}
	return nil
}

func (o ObjectTerm) MarshalJSON() ([]byte, error) {
	switch {
	case o.Variable != "":
		return json.Marshal(o.Variable)
	case !o.typed && o.Type == "text":
		return json.Marshal(o.Value)
	default:
		return json.Marshal(struct {
			Value string `json:"value"`
			Type  string `json:"type"`
		}{o.Value, o.Type})
	}
}

// String is the term as a reader would write it in a found column.
func (o *ObjectTerm) String() string {
	switch {
	case o == nil:
		return "<nil>"
	case o.Variable != "":
		return o.Variable
	case o.Type == "text":
		return fmt.Sprintf("%q", o.Value)
	default:
		return fmt.Sprintf("%s^^%s", o.Value, o.Type)
	}
}

// IsVariableName reports whether a subject, predicate or object string is a
// ?variable.
func IsVariableName(s string) bool { return strings.HasPrefix(s, "?") }
