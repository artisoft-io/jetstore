package audit

import (
	"fmt"
	"strings"
)

// The autonomy tier vocabulary, its ordering, and the comparison (task AJ.1,
// gap 7b, criterion 51).
//
// **Three parties record a tier and none compared one until this file.**
// Loop.Tier is written onto every audit event, agent_run.tier persists the
// authority a run was conducted under, approval_event.tier_at_event records the
// authority a decision was taken at, and Remediation.autonomy_tier_required
// names the authority an action needs. Phase 1 said in terms that recording was
// all it was doing — "Tier is recorded and not enforced in Phase 1 —
// enforcement is gap 7" — and Phase 4 gave the required tier a home while
// tabling Remediation for a different reason. What was missing was one
// comparison, and this is it.
//
// # The vocabulary had no Go definition, and that is why it gets one here
//
// AutonomyTier lives in Python (`AutonomyTier`,
// tools/jets_agentic/jets_agentic/model.py:74) and reaches Go only as SQL: the
// DDL emitter joins the enum's members into a string (`tiers`,
// tools/jets_agentic/jets_agentic/ddl.py:342) and the generated
// agent_audit.sql carries it as four CHECK constraints. Go has been writing
// tier values as bare string literals with nothing asserting the set — and the
// one non-test caller of the loop was writing `"assisted"`, which is not a
// member and never failed because that harness persists nothing.
//
// So this file is the vocabulary, and TestTiersMatchTheGeneratedCheck holds it
// against the generated SQL rather than against a comment.
//
// # The ordering is asserted, not assumed
//
// AutonomyTier's own docstring says "Lexicographic order is tier order — the
// ceiling comparisons of gap 7 work on the text directly, so do not relabel to
// the §9.2 names". That is true today and is a property of the *labels*: it
// holds because the members are spelled T0…T4 and would stop holding the day
// somebody renamed them to §9.2's Observe/Advise/Propose/Act/Bounded, which is
// exactly what the docstring is warning against.
//
// **A comparison that relies on it silently inverts when that happens.**
// `"Advise" < "Observe"` is true and means the opposite of what a tier
// comparison intends, and nothing in a string comparison can notice. So Reaches
// works over the explicit order of Tiers below, and TestTierOrderIsLexicographic
// asserts that the declared order and the lexicographic one still agree — which
// turns the docstring's instruction into a failing test rather than a request.

// Tier is one member of the AutonomyTier vocabulary. It is a distinct type so
// that a required tier and a current tier cannot be swapped for a plain string
// without the compiler having something to say.
type Tier string

// The five members, mirroring model.py:74 and the CHECK constraints the DDL
// generates from it.
const (
	// TierT0 is §9.2's Observe: read metadata and code, record analysis.
	TierT0 Tier = "T0"
	// TierT1 is Advise: advisory hypotheses and narratives, no modification.
	TierT1 Tier = "T1"
	// TierT2 is Propose: staged artifacts in a non-executing state.
	TierT2 Tier = "T2"
	// TierT3 is Act with approval: execute after a recorded human authorisation.
	TierT3 Tier = "T3"
	// TierT4 is Bounded autonomy: reversible action classes inside an envelope.
	TierT4 Tier = "T4"
)

// Tiers is the vocabulary in tier order, least authority first. The order here
// is the authority order; Reaches reads it and nothing else does.
var Tiers = []Tier{TierT0, TierT1, TierT2, TierT3, TierT4}

// Rank returns a tier's position in Tiers, and false when it is not a member.
// The boolean is the whole point: a rank of zero for an unrecognised value
// would make every unknown tier look like T0, which is the lowest authority and
// therefore the value least likely to be noticed.
func (t Tier) Rank() (int, bool) {
	for i, m := range Tiers {
		if m == t {
			return i, true
		}
	}
	return 0, false
}

func (t Tier) String() string { return string(t) }

// ErrUnknownTier is what refusing to guess looks like. A tier that is not in
// the vocabulary has no honest comparison available, so neither ParseTier nor
// Reaches invents one.
type ErrUnknownTier struct{ Value string }

func (e *ErrUnknownTier) Error() string {
	return fmt.Sprintf("%q is not an autonomy tier; the vocabulary is %s", e.Value, TierList())
}

// TierList renders the vocabulary for an error message, in tier order.
func TierList() string {
	parts := make([]string, 0, len(Tiers))
	for _, t := range Tiers {
		parts = append(parts, string(t))
	}
	return strings.Join(parts, ", ")
}

// ParseTier turns a recorded string into a Tier, refusing anything outside the
// vocabulary rather than defaulting.
//
// **It refuses the empty string too**, which is worth saying because "" is what
// a struct field carries when nobody set it. agent_audit.tier is nullable and
// its CHECK admits NULL, so an event may legitimately carry no tier; a
// *comparison* may not, and a caller holding "" is a caller who does not know
// what authority it is acting under.
func ParseTier(s string) (Tier, error) {
	t := Tier(strings.TrimSpace(s))
	if _, ok := t.Rank(); !ok {
		return "", &ErrUnknownTier{Value: s}
	}
	return t, nil
}

// Reaches reports whether a run operating at `current` may take an action
// requiring `required`.
//
// It returns an error rather than false when either operand is outside the
// vocabulary, so a caller can tell "this run is not authorised" from "this
// build cannot read one of these values" — two refusals that should read
// differently in an audit record, and which a bare boolean would flatten into
// one.
func Reaches(current, required Tier) (bool, error) {
	c, ok := current.Rank()
	if !ok {
		return false, fmt.Errorf("the current tier is unusable: %w", &ErrUnknownTier{Value: string(current)})
	}
	r, ok := required.Rank()
	if !ok {
		return false, fmt.Errorf("the required tier is unusable: %w", &ErrUnknownTier{Value: string(required)})
	}
	return c >= r, nil
}
