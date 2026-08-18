// Package wsvalidate holds the contract between the workspace save path and the
// validators that check what is being saved.
//
// **It imports nothing, and that is its whole design.** The save hook lives in
// jets/datatable; the validators live wherever their file type does —
// jets/userflow for .uf.json and .ua.json, jets/agentic/tools for .pc.json. A
// shared Finding type had to live somewhere none of them would have to import
// another, and putting it in the caller would have inverted the dependency: a
// validator would import the HTTP handler package to describe its own findings.
//
// Settled 2026-08-18 with the agentic_ai stream as Q-3 in the ui_refresh
// project's tracking. Their request was for a per-suffix validator table
// returning findings with a severity rather than a bare error; the package name
// and the leaf-package shape were this side's addition, because where the type
// lives is what actually decides the packaging.
//
// # Severity is the point of the interface
//
// A bare `error` would collapse the tier. jets/userflow already reports an
// unreachable state as a warning and a dangling transition as an error, and the
// deployment decides whether the first blocks a save
// (JETS_USERFLOW_STRICT_REACHABILITY). The cpipes validator has no warning tier
// today, so every diagnostic it produces is an Error — the tier costs it nothing
// and is there when it wants one.
package wsvalidate

// Severity of a finding. Any Error blocks the save; Warnings are reported and
// do not.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Finding is one thing wrong with a file, and where.
//
// Path is a JSON Pointer (RFC 6901) into the document — "/states/x/choices/0" —
// or empty for a finding about the document as a whole. It exists because a
// message is not something an editor can jump to or a repair prompt can use;
// see the ui_refresh project's I-21 for the argument, which came from the
// agentic_ai side.
//
// **Segments are not escaped by this package.** A validator whose path segments
// can contain "/" or "~" must escape them per RFC 6901 §3 at the point of
// construction, because only the validator knows whether its own identifiers
// can.
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
}

// Validator checks the content of one workspace file.
//
// It takes the raw text rather than a parsed document on purpose: the schema
// library wants its own reader, and the cpipes validator unmarshals into a typed
// config and mutates it while applying defaults. One shared parsed value would
// have to be re-marshalled or would be quietly mutated.
//
// A validator may assume the content is well-formed JSON — the save path checks
// that first, for every file ending .json, because it is a precondition for
// every structured check and doing it once keeps one bad file from producing two
// different complaints.
type Validator func(content string) []Finding

// ErrorsOnly is what a save-time check acts on; warnings travel alongside.
func ErrorsOnly(findings []Finding) []Finding {
	errs := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if f.Severity == Error {
			errs = append(errs, f)
		}
	}
	return errs
}
