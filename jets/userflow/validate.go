// Reference checks over a user flow document, and the deployment-time switch
// that decides how severely one of them is taken.
//
// This is the port of jetsclient_ide/src/userflow/validate.ts, kept literally
// parallel to it: same finding codes, same order, same messages. Two
// implementations of one rule stay in agreement only if a reader can put them
// side by side, so structural improvements to one belong in both or in neither.
//
// **What is here and what is still S.4's.** These are the checks a JSON Schema
// cannot express — schema.ts absorbs everything expressible in the document's
// shape. Wiring them into SaveWorkspaceFileContent, beside the existing
// well-formed-JSON check, is S.4's task and is deliberately not done here.
package userflow

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Severity of a finding. A document with any error must not be saved; warnings
// are reported and do not block.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Finding codes, matching validate.ts.
const (
	CodeUnknownStartState = "unknownStartState"
	CodeUnknownTarget     = "unknownTarget"
	CodeUnreachableState  = "unreachableState"
)

type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	// Path locates the offending place as a JSON Pointer (RFC 6901) into the
	// document — "/states/select_client/choices/0/nextState". Empty means the
	// finding is about the document as a whole.
	//
	// **Added at the agentic_ai stream's request, and it was right that it helps
	// this side sooner.** The structure was already here and was being
	// stringified into Message: an unknownTarget knows which state and which
	// transition offended, and putting that only in prose means a UI cannot jump
	// to it and a repair prompt cannot say where. One string rather than a
	// typed location keeps Finding comparable and JSON-friendly, and keeps the
	// two file types from having to agree on a schema for locations.
	//
	// No escaping is applied and none is needed: every segment that varies is an
	// Identifier, whose pattern excludes both "/" and "~" (schema.ts). If a
	// future segment can contain either, it must be escaped per RFC 6901 §3
	// rather than interpolated.
	Path string `json:"path,omitempty"`
}

// Policy says how severely each configurable check is taken. A struct rather
// than a bool so that a second configurable check does not change every
// signature.
type Policy struct {
	UnreachableState Severity
}

// StrictReachabilityEnvVar is the deployment-time switch.
//
// **The trade this was introduced with turned out not to exist, and the entry
// that describes it is I-18.** It was added believing that pipelineConfigUF
// shipped two dead states, so that a strict deployment could not save a flow it
// already runs. Those two states are reached by a button — an action that jumps
// the flow — and the walk could not see the edge because the document did not
// declare it. It does now, via State.GoToStates, and **no flow in the corpus is
// refused under the strict policy**.
//
// The default stays a warning regardless, because the two questions are
// different: whether the shipping flows pass is about today, and whether a
// workspace wants an unreached state to block a save is about that workspace.
// A deployment that wants the stricter reading can now take it without also
// taking a false positive.
const StrictReachabilityEnvVar = "JETS_USERFLOW_STRICT_REACHABILITY"

// DefaultPolicy reports unreachable states without blocking the save.
func DefaultPolicy() Policy { return Policy{UnreachableState: Warning} }

// StrictPolicy refuses a document with a state nothing transitions to.
func StrictPolicy() Policy { return Policy{UnreachableState: Error} }

// PolicyFromEnv resolves the policy from the process environment.
//
// Truthy is "1", "true", "yes" or "on", case-insensitive and trimmed. Anything
// else, including an unset variable, leaves the warning a warning — so a typo
// in the value fails safe rather than silently turning enforcement on.
func PolicyFromEnv() Policy {
	if IsTruthy(os.Getenv(StrictReachabilityEnvVar)) {
		return StrictPolicy()
	}
	return DefaultPolicy()
}

// IsTruthy matches isTruthy in validate.ts. Kept exported and separate for that
// reason: it is the one place the two implementations must agree character for
// character, and a shared test table is cheaper than a shared reading.
func IsTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Choice is a guarded transition. Only the target is read here; the condition is
// the schema's business and the interpreter's.
type Choice struct {
	NextState string `json:"nextState"`
}

// State carries only the fields the reference checks need. Unknown fields are
// ignored on purpose — the schema check runs first and is what rejects them, so
// duplicating that here would give one document two different complaints.
//
// GoToStates are transitions an action of this state makes rather than ones the
// Next button makes. They are edges of the state machine all the same, and
// omitting them is what made S.1 report two live states as dead — see I-18 and
// the comment on targetsOf.
type State struct {
	IsEnd            bool     `json:"isEnd"`
	Choices          []Choice `json:"choices"`
	DefaultNextState string   `json:"defaultNextState"`
	GoToStates       []string `json:"goToStates"`
}

type Flow struct {
	StartAtKey string           `json:"startAtKey"`
	States     map[string]State `json:"states"`
}

// ParseFlow reads a .uf.json document. It does not validate against the schema;
// that is the caller's first step and a separate library's job.
func ParseFlow(data []byte) (*Flow, error) {
	var flow Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("user flow is not valid JSON: %w", err)
	}
	return &flow, nil
}

// Transition is a target and where in the document it was declared.
type Transition struct {
	Target string
	Path   string
}

// targetsOf returns every state a state transitions to: choices, then the
// default, then the ones an action takes — the same order as validate.ts, so
// findings come out in the same order from both.
func targetsOf(flow *Flow, key string) []Transition {
	state, ok := flow.States[key]
	if !ok {
		return nil
	}
	base := fmt.Sprintf("/states/%s", key)
	targets := make([]Transition, 0, len(state.Choices)+len(state.GoToStates)+1)
	for i, choice := range state.Choices {
		targets = append(targets, Transition{
			Target: choice.NextState,
			Path:   fmt.Sprintf("%s/choices/%d/nextState", base, i),
		})
	}
	if state.DefaultNextState != "" {
		targets = append(targets, Transition{
			Target: state.DefaultNextState,
			Path:   base + "/defaultNextState",
		})
	}
	for i, target := range state.GoToStates {
		targets = append(targets, Transition{
			Target: target,
			Path:   fmt.Sprintf("%s/goToStates/%d", base, i),
		})
	}
	return targets
}

// ValidateFlow runs the reference checks over a document that has already passed
// the schema.
//
// Order is deliberate: an unknown start state suppresses the reachability walk,
// because a walk from nowhere reports every state as unreachable and buries the
// one finding that matters.
func ValidateFlow(flow *Flow, policy Policy) []Finding {
	findings := make([]Finding, 0)

	_, startExists := flow.States[flow.StartAtKey]
	if !startExists {
		findings = append(findings, Finding{
			Severity: Error,
			Code:     CodeUnknownStartState,
			Message:  fmt.Sprintf("startAtKey %q is not a state", flow.StartAtKey),
			Path:     "/startAtKey",
		})
	}

	// Sorted so that two runs over one document report in one order. Go's map
	// iteration is deliberately randomised, and a validator whose output moves
	// between runs is one nobody can diff.
	keys := sortedKeys(flow.States)
	for _, key := range keys {
		for _, transition := range targetsOf(flow, key) {
			if _, ok := flow.States[transition.Target]; !ok {
				findings = append(findings, Finding{
					Severity: Error,
					Code:     CodeUnknownTarget,
					Message: fmt.Sprintf(
						"state %q transitions to %q, which is not a state", key, transition.Target),
					Path: transition.Path,
				})
			}
		}
	}

	if startExists {
		reached := map[string]bool{flow.StartAtKey: true}
		frontier := []string{flow.StartAtKey}
		for len(frontier) > 0 {
			key := frontier[len(frontier)-1]
			frontier = frontier[:len(frontier)-1]
			for _, transition := range targetsOf(flow, key) {
				if _, ok := flow.States[transition.Target]; ok && !reached[transition.Target] {
					reached[transition.Target] = true
					frontier = append(frontier, transition.Target)
				}
			}
		}
		for _, key := range keys {
			if !reached[key] {
				findings = append(findings, Finding{
					Severity: policy.UnreachableState,
					Code:     CodeUnreachableState,
					Message: fmt.Sprintf(
						"state %q is not reachable from %q", key, flow.StartAtKey),
					Path: fmt.Sprintf("/states/%s", key),
				})
			}
		}
	}

	return findings
}

// ErrorsOnly is what a save-time check acts on; warnings are reported alongside.
func ErrorsOnly(findings []Finding) []Finding {
	errs := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Severity == Error {
			errs = append(errs, f)
		}
	}
	return errs
}

func sortedKeys(states map[string]State) []string {
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
