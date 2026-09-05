// Package rca is the advisory half of Phase 4's agent track (task AC.2): it
// ranks causal hypotheses for one run, and every hypothesis it emits carries
// the evidence against itself as well as the evidence for.
//
// # What this package claims and what AC.1 claims
//
// jets/agentic/triage emits a *locus* — where in the execution record a failure
// left its evidence — and never a cause. The gate's reading (phase-4 plan §9.5)
// is that the record supports the first and does not support the second: three
// of the imported ten IncidentClassification values have no substrate in
// JetStore at all, four are evidenced only at a grain coarser than the class
// name implies, and one — parse_failure — the record does well. So the step
// from a locus to a cause is the step the substrate does not determine, and a
// ranked hypothesis with contradicting evidence is the honest shape for it.
//
// **R-27 is the risk this package creates rather than finds**: a locus
// taxonomy is close enough to a cause taxonomy in shape that *classified* will
// be read as *diagnosed*. AC.1's structural answer was that a Finding has no
// classification field. This package cannot take that answer, because naming a
// cause is what it is for. **Its structural answer is that a hypothesis cannot
// exist without the evidence against it**: ContradictingEvidence is a required
// field that a nil value fails Validate on, so an assertion with nothing
// against it is not a value this package can produce. Appendix A.2.8 calls that
// column a calibration control; here it is also the thing that keeps a ranking
// advisory in the type system rather than in the prose beside it.
//
// # Two vocabularies, and a hypothesis says which one it is speaking
//
// Q-27 was answered on 2026-09-04: *per class* means both vocabularies,
// reported separately. So every Hypothesis carries Locus — the AC.1 verdict it
// was derived from, in the nine-member locus vocabulary — and CauseCategory,
// the claim, in the imported ten. AD.1 scores per locus and reports the
// cause-class denominators beside it, and neither number is computable if a
// hypothesis does not say which vocabulary its claim is in.
//
// **CauseCategory may be empty and Locus may not.** A hypothesis derived from a
// locus the gate's §9.5 maps to no cause class makes no claim in the cause
// vocabulary at all; the floor emits none for such a locus and reports the
// locus in Ranking.UnmappedLoci instead, so that a locus which fired and
// produced no hypothesis is visible rather than silently absent.
//
// # The deterministic floor comes first, and that is §9.8's instruction
//
// The contradicting side has a substrate that was already built and populated
// before this task started: AnomalyConfounder's fourteen members are the
// record's own statement of what a detector could not rule out, they are
// written on every anomaly N.4's detectors emit and on every finding AC.1
// emits, and EvidenceSource gained detector_confounder at AB.1 so a hypothesis
// can name one. §9.8 says to populate the contradicting side from that
// substrate *before* any model is asked for one, so that a model's contribution
// is measurable against a floor rather than against nothing. Rank is that
// floor. It reads no model, holds no threshold, and is a transcription of the
// gate's own table plus arithmetic over counted evidence items.
//
// # Nothing here writes a row
//
// AC.3 is the shadow-mode wiring. This package returns a Ranking; it does not
// insert into jetsapi.hypothesis, does not create an incident, and does not
// supply the severity that jetsapi.incident requires and no locus determines
// (I-306).
package rca

import (
	"fmt"
	"slices"
)

// The ten IncidentClassification values, which are the source proposal's own
// (§A.2.7, §A.2.8's cause_category). They are the members of
// jetsapi.hypothesis.hypothesis_cause_category_ck and of
// jetsapi.incident.incident_classification_ck; TestCauseCategoriesMatchDDL
// asserts them against the generated DDL so a regeneration that adds or removes
// one fails the suite rather than an insert, on triage.TestLociMatchDDL's model
// and observe.TestVocabulariesMatchDDL's before it.
//
// The order is §9.5's table order, which is the order the ranker breaks ties
// in. It is not a priority: it is the order the gate wrote them in, kept so
// that two runs of the ranker over the same evidence agree.
const (
	CauseSourceDeliveryFailure   = "source_delivery_failure"
	CauseSourceContentChange     = "source_content_change"
	CauseTransportFailure        = "transport_failure"
	CauseParseFailure            = "parse_failure"
	CauseValidationBreach        = "validation_breach"
	CauseTransformationDefect    = "transformation_defect"
	CauseInfrastructureFailure   = "infrastructure_failure"
	CauseDependencyFailure       = "dependency_failure"
	CauseCapacityOrCostDeviation = "capacity_or_cost_deviation"
	CauseBenignVariation         = "benign_variation"
)

// CauseCategories is the vocabulary in §9.5's order.
var CauseCategories = []string{
	CauseSourceDeliveryFailure, CauseSourceContentChange, CauseTransportFailure,
	CauseParseFailure, CauseValidationBreach, CauseTransformationDefect,
	CauseInfrastructureFailure, CauseDependencyFailure,
	CauseCapacityOrCostDeviation, CauseBenignVariation,
}

// The ten EvidenceSource values. Nine are §A.2.8's and detector_confounder is
// this project's, added at AB.1 because §9.7 found the record's own statement
// of what could not be ruled out had no member to be filed under.
//
// **These are not a CHECK constraint anywhere**, and that is a property of the
// column rather than an oversight: supporting_evidence and contradicting_
// evidence are jsonb, because an Evidence is a value object with three fields
// and no table of its own. So the database will accept any source string, and
// Hypothesis.Validate is the only thing that will not — which makes it load
// bearing here in a way triage.Finding.Validate is not, where Postgres is the
// backstop. TestEvidenceSourcesMatchTheDataModel asserts them against the
// generated .jr, which is the nearest thing to an authority they have.
const (
	SourceRunTelemetry          = "run_telemetry"
	SourceLineage               = "lineage"
	SourceCommitHistory         = "commit_history"
	SourcePriorIncident         = "prior_incident"
	SourceInfrastructureLog     = "infrastructure_log"
	SourceSourceDeliveryHistory = "source_delivery_history"
	SourceDqResult              = "dq_result"
	SourceCodeInspection        = "code_inspection"
	SourceProfile               = "profile"
	SourceDetectorConfounder    = "detector_confounder"
)

// EvidenceSources is the vocabulary in the data model's order.
var EvidenceSources = []string{
	SourceRunTelemetry, SourceLineage, SourceCommitHistory, SourcePriorIncident,
	SourceInfrastructureLog, SourceSourceDeliveryHistory, SourceDqResult,
	SourceCodeInspection, SourceProfile, SourceDetectorConfounder,
}

// Evidence is one item of a hypothesis's case, for or against. It is §A.2.8's
// `$defs` entry and the model's Evidence value object, three fields and no
// more.
type Evidence struct {
	// Statement is the evidence in words. It is the model's one
	// data_classification = "PHI" property (AE.2), so a caller putting a
	// hypothesis on a screen goes through audit's redaction rather than
	// reading this field directly.
	Statement string
	// Source is one of the ten EvidenceSource members.
	Source string
	// SourceRef is a resolvable reference into the source: a session id and
	// locus, an anomaly id, a plan section.
	SourceRef string
}

// Hypothesis is one ranked causal claim about one incident. Its first eight
// fields are jetsapi.hypothesis's eight columns; the last three are not columns
// and are documented as not being columns below.
type Hypothesis struct {
	HypothesisId string
	// IncidentRef is jetsapi.hypothesis.hypothesis_incident_ref. The floor
	// leaves it empty: an incident is written by AC.3 and a ranking computed
	// before one exists cannot name it, which is why this package returns a
	// Ranking keyed on the session rather than a list keyed on an incident.
	IncidentRef string
	// Cause is the hypothesised cause in words (§A.2.8 caps it at 500 chars).
	Cause string
	// CauseCategory is the claim's entry in the ten-member cause vocabulary,
	// or empty where the hypothesis makes no claim in it. Empty is a value the
	// column holds — classification and cause_category are both nullable, on
	// I-289's argument that a required cause column forces a deterministic step
	// to invent one.
	CauseCategory string
	// Confidence is 0..1. The floor computes it rather than choosing it: see
	// Ranker.Rank, where it is the fraction of the hypothesis's evidence items
	// that are for it.
	Confidence float64
	// Rank is 1-based within the session's ranking.
	Rank int
	// SupportingEvidence is the case for. §A.2.8 requires at least one.
	SupportingEvidence []Evidence
	// ContradictingEvidence is the case against. **Required, and nil is not
	// empty**: an empty slice asserts that the ranker found nothing against
	// this hypothesis, and nil says it was never asked. §A.2.8 calls the column
	// a calibration control — an agent that can omit the evidence against its
	// own hypothesis will — and Validate is where that is enforced.
	ContradictingEvidence []Evidence

	// --- not columns -------------------------------------------------------

	// Locus is the AC.1 verdict this hypothesis was derived from, in the nine
	// member locus vocabulary. It has no column on jetsapi.hypothesis and it is
	// not redundant with jetsapi.incident.incident_locus: an incident is
	// written per (session, locus) and a hypothesis is written per (incident,
	// cause), so the join recovers it — but a Ranking is computed before any
	// incident exists, and Q-27's answer needs every hypothesis to say which
	// vocabulary its claim is in.
	Locus string
	// Basis is why this hypothesis ranks where it does, in the words an
	// operator reads. **It has no column either, and that is criterion 45's
	// last clause meeting a schema that has no place to keep it** — see
	// Ranking.Basis and I-361. It is required by Validate for the same reason
	// triage.Finding.Basis and observe.Anomaly.ExpectedBasis are: a claim that
	// cannot say what it rests on is the signal an operator learns to ignore.
	Basis string
	// RankerRef names the ranker and its generation, so two generations are
	// told apart the way two detector and two classifier generations are. A
	// ranker that consulted a model names the model here, because a figure
	// without its model is not a figure (R-23).
	RankerRef string
}

// Fired reports whether the hypothesis has more for it than against it, which
// is §B.3's escalation trigger read the other way up. It is a convenience for a
// caller and never a filter inside this package: a hypothesis whose
// contradicting evidence outweighs its supporting is still ranked and still
// emitted, because suppressing it would delete the calibration the column
// exists for.
func (h *Hypothesis) Fired() bool {
	return len(h.SupportingEvidence) > len(h.ContradictingEvidence)
}

// Validate checks the vocabularies, the required fields and the one rule that
// makes this package advisory rather than assertive.
//
// The database is the authority for cause_category and is not the authority for
// anything else here: the two evidence columns are jsonb and will accept an
// invented source, and no CHECK anywhere requires contradicting_evidence to
// have been considered. So this function is the whole of the guarantee for
// three of the four checks below, which is stated rather than assumed.
func (h *Hypothesis) Validate() error {
	if h.Cause == "" {
		return fmt.Errorf("cause is required: a hypothesis that does not say what it claims is not a hypothesis")
	}
	if h.CauseCategory != "" && !slices.Contains(CauseCategories, h.CauseCategory) {
		return fmt.Errorf("cause_category %q is not in the vocabulary", h.CauseCategory)
	}
	if h.Locus == "" {
		return fmt.Errorf("locus is required: Q-27's answer needs a hypothesis to say which vocabulary its claim is in, and the cause category alone cannot say which evidence position it came from")
	}
	if h.Confidence < 0 || h.Confidence > 1 {
		return fmt.Errorf("confidence is %v, which is outside 0..1", h.Confidence)
	}
	if h.Rank < 1 {
		return fmt.Errorf("rank is %d; §A.2.8's ranks are 1-based", h.Rank)
	}
	if h.Basis == "" {
		return fmt.Errorf("basis is required: a ranking a human cannot read the basis of is criterion 45's last clause unmet")
	}
	if h.RankerRef == "" {
		return fmt.Errorf("ranker_ref is required")
	}
	if len(h.SupportingEvidence) == 0 {
		return fmt.Errorf("supporting_evidence is empty; §A.2.8 requires at least one item")
	}
	if h.ContradictingEvidence == nil {
		return fmt.Errorf("contradicting_evidence is nil rather than empty: §A.2.8 makes it required as a calibration control, and an empty list asserting that nothing was found against this hypothesis is a claim the ranker has to make deliberately")
	}
	for i := range h.SupportingEvidence {
		if err := h.SupportingEvidence[i].validate(); err != nil {
			return fmt.Errorf("supporting_evidence[%d]: %w", i, err)
		}
	}
	for i := range h.ContradictingEvidence {
		if err := h.ContradictingEvidence[i].validate(); err != nil {
			return fmt.Errorf("contradicting_evidence[%d]: %w", i, err)
		}
	}
	return nil
}

func (e *Evidence) validate() error {
	if e.Statement == "" {
		return fmt.Errorf("statement is required")
	}
	if !slices.Contains(EvidenceSources, e.Source) {
		return fmt.Errorf("source %q is not in the vocabulary", e.Source)
	}
	return nil
}

// Ranking is one session's ranked hypotheses, with the denominators a per-class
// report needs and the account of how the order was arrived at.
//
// It is keyed on the session and not on an incident for the reason §9.6 gives
// for an incident's own grain: session_id is the only key all seven
// evidence-bearing tables carry (F202). A caller that has written incidents
// assigns IncidentRef afterwards.
type Ranking struct {
	SessionId string
	// RankerRef is the ranker and its generation, repeated on every hypothesis.
	RankerRef string
	// Hypotheses are in rank order, best first.
	Hypotheses []Hypothesis
	// UnmappedLoci are the loci that fired and that §9.5's table maps to no
	// cause class, so no hypothesis was emitted for them. **Two of the nine are
	// permanently in this state** — step_never_started and
	// per_record_failures_unreportable appear in no row of §9.5 (F393) — and a
	// caller counting hypotheses per locus needs to know the difference between
	// a locus that produced none and a locus that was never asked.
	UnmappedLoci []string
	// UnaskedLoci are the loci AC.1 could not evaluate. They are the other half
	// of the denominator: a cause class whose only evidence position was
	// not_evaluable has not been ruled out, it has not been looked at.
	UnaskedLoci []string
	// Basis says how the ranking as a whole was produced, including which
	// classes were considered and rejected before any hypothesis was emitted.
	Basis string
}

// Validate checks every hypothesis and the one invariant of the ranking itself.
func (r *Ranking) Validate() error {
	for i := range r.Hypotheses {
		if got := r.Hypotheses[i].Rank; got != i+1 {
			return fmt.Errorf("hypotheses[%d] has rank %d: ranks must be 1-based and dense in slice order", i, got)
		}
		if err := r.Hypotheses[i].Validate(); err != nil {
			return fmt.Errorf("hypotheses[%d] (%s at %s): %w", i,
				r.Hypotheses[i].CauseCategory, r.Hypotheses[i].Locus, err)
		}
	}
	if r.Basis == "" {
		return fmt.Errorf("basis is required on the ranking as well as on each hypothesis: the per-hypothesis basis says why one outranks another and does not say what was considered and dropped")
	}
	return nil
}
