package audit

import "fmt"

// Making the domain model's data_classification marker load-bearing (task AE.2,
// I-311).
//
// **The defect this closes is not a missing control; it is a marker that read
// like one.** `Evidence.statement` has carried `data_classification = "PHI"`
// since Phase 0. The marker survived the whole toolchain — stripped from the
// JSON Schema, kept in the sidecar, emitted into every client workspace as a
// rule-visible triple — and was read by nothing: no rule matched on it, no Go
// code consulted it, and AE.1's detail screen rendered the field it marks to any
// holder of `agent_supervision`. A reader auditing the model would have counted
// it as a handling control. The user's decision of 2026-09-04 was to make it one
// rather than to remove it, the modelling intent being real.
//
// # The floor, and what this file adds above it
//
// **The settled floor is that a PHI-marked field is not covered by
// `agent_supervision` alone.** That capability is a governance authority: it is
// deliberately not `jetstore_read` because a transcript is a record of what an
// agent did rather than client data (`AgentSupervisionCapability`,
// `jets/apiserver/api_agentic.go`). A PHI-marked property is the exception that
// argument does not cover — it is client data sitting inside a governance
// record, and the two authorities are different for the same reason K.3 said
// they were.
//
// **So the shape is both halves rather than either**, and the reason each is
// insufficient alone is worth stating:
//
//   - **Redaction by default, and server-side.** A client-side hide is not a
//     control, because the value is already in the response body — the browser
//     would be deciding not to paint something it holds. So the withholding
//     happens here, in the read path, before an HTTP handler ever sees the row.
//   - **A distinct capability that lifts it.** Redaction alone would make the
//     field unreachable, which is not a control either: it is a deletion with
//     extra steps, and the evidence statement is the basis of a hypothesis that
//     a supervisor is being asked to judge. The capability is what makes
//     disclosure a decision somebody took rather than a side effect of holding
//     a supervision role.
//
// **Who holds that capability is not decided here** (**Q-42**). It is a policy
// about client data in a healthcare deployment, which I-311 itself says is the
// user's; `jets_init_db.sql` grants it to no role, so the default is redacted
// for everybody and granting it is a deliberate act on a screen that exists.
//
// # Why the answer is a parameter rather than a package default
//
// `ReadIncident` and `HypothesesFor` take a `PHIAccess` rather than defaulting
// to redaction, and that is the one design choice here that costs a caller
// something. A default would let a new caller inherit disclosure by writing
// nothing, and *the caller that forgets* is exactly the failure this entry is
// about — AE.1's screen did not forget so much as never have the question put to
// it. A parameter makes the compiler put it to every call site, present and
// future. It is `event_actor_kind`'s argument one package over: where there is no
// honest default, make the caller say.
//
// # The limit, recorded rather than papered over
//
// `user.HasCapability` returns true unconditionally for the admin identity
// (`HasCapability`, `jets/user/user.go:74`), so an admin sees the statement
// whatever is granted. That is `PgGuard`'s recorded limit arriving at a second
// control, and it is a deployment rule this code cannot enforce.

// ClassifiedProperty is one property of the domain model carrying a
// data_classification marker. The values are generated — see
// `DataClassifiedProperties` in data_classification.go — and the type is here
// because a generated file should hold data and not declarations that outlive
// the data.
type ClassifiedProperty struct {
	Entity         string
	Property       string
	Classification string
}

// Key is the "<Entity>.<property>" spelling used to match a generated entry
// against a redactor.
func (c ClassifiedProperty) Key() string { return c.Entity + "." + c.Property }

// ClassificationPHI is the only marker value the model uses today. Named rather
// than spelled at each site, so the day a second classification arrives the
// comparison sites are greppable.
const ClassificationPHI = "PHI"

// PHIAccess is a caller's answer to "may this read disclose PHI-classified
// fields?". A bool with two named values rather than a bare bool: `RedactPHI`
// at a call site says what `false` does not.
type PHIAccess bool

const (
	// RedactPHI withholds every PHI-classified field. This is what a caller
	// holding only `agent_supervision` passes.
	RedactPHI PHIAccess = false
	// DisclosePHI returns them. Only a caller that has checked the PHI
	// capability may pass it.
	DisclosePHI PHIAccess = true
)

// phiRedactors names the classified properties this package knows how to
// withhold, keyed as ClassifiedProperty.Key spells them.
//
// **This map is the hand-written half and the generated list is the other**, and
// nothing but a test connects them — which is the same arrangement, and the same
// hazard, as `jr_as_table` against the emitted `CREATE TABLE` before AB.1 gave
// it `_assert_tables_agree`. Here the assertion is
// TestEveryClassifiedPropertyHasARedactor, in both directions: a marked property
// with no redactor is a marker that means nothing again, and a redactor for a
// property nothing marks is code defending a field the model does not classify.
var phiRedactors = map[string]func(*Evidence){
	"Evidence.statement": func(e *Evidence) {
		// Blanked and flagged rather than replaced with a placeholder string.
		// A placeholder in the value is indistinguishable from an agent that
		// wrote those words, and the screen has to be able to tell "withheld"
		// from "the evidence says [redacted]".
		e.Statement = ""
		e.StatementRedacted = true
	},
}

// redactEvidence applies every registered redactor to a slice read from the
// database, in place. It is called by the read path rather than by the handler
// so that no caller can reach an unredacted row without having said
// DisclosePHI.
func redactEvidence(items []Evidence) {
	for i := range items {
		for _, c := range DataClassifiedProperties {
			if c.Entity != "Evidence" {
				continue
			}
			if redact, ok := phiRedactors[c.Key()]; ok {
				redact(&items[i])
			}
		}
	}
}

// applyPHIPolicy redacts unless the caller asked for disclosure. A single
// function so the decision reads the same at both call sites and there is one
// place to change if a third classification arrives.
func applyPHIPolicy(phi PHIAccess, groups ...[]Evidence) {
	if phi == DisclosePHI {
		return
	}
	for _, g := range groups {
		redactEvidence(g)
	}
}

// ClassifiedPropertiesOf returns the marked properties of one entity, for a
// caller that needs to report what it withheld. Exported because the manifest is
// the only statement of the answer and copying it into a handler would be the
// second source of truth this file exists to avoid.
func ClassifiedPropertiesOf(entity string) []ClassifiedProperty {
	var out []ClassifiedProperty
	for _, c := range DataClassifiedProperties {
		if c.Entity == entity {
			out = append(out, c)
		}
	}
	return out
}

// PHIRedactorCoverage reports which generated entries have no redactor and which
// redactors name nothing generated. It is exported so the assertion can live in
// the test file while the comparison stays beside the map it is about.
func PHIRedactorCoverage() (unhandled []string, unknown []string) {
	marked := make(map[string]struct{}, len(DataClassifiedProperties))
	for _, c := range DataClassifiedProperties {
		marked[c.Key()] = struct{}{}
		if _, ok := phiRedactors[c.Key()]; !ok {
			unhandled = append(unhandled, fmt.Sprintf("%s (%s)", c.Key(), c.Classification))
		}
	}
	for key := range phiRedactors {
		if _, ok := marked[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	return unhandled, unknown
}
