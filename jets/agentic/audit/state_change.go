package audit

import "fmt"

// The shape a recorded act of responsibility takes (task AJ.2, Q-52).
//
// **Two things in this package are the same act and the claim was only ever in
// prose.** Q-52 was resolved on the argument that a human verdict on an
// incident and a transition of a run's autonomy tier are *a person taking
// responsibility for a state change, recorded with an actor* — and that
// argument was a sentence in a plan. This file is the sentence made
// checkable: one interface both writers satisfy, one attribution rule both
// enforce, and one test that runs the same table of refusals against both.
//
// # What the two acts share
//
// Each of them reads and locks the subject row, guards it against the state the
// caller expected to find, refuses an edge the policy does not draw, requires an
// actor and an actor *kind*, writes the new state and appends a chain event
// carrying both operands — all in one transaction. `RecordIncidentTransition`
// did all seven before this file existed and `RecordTierTransition` does them
// because it was written against the first.
//
// # What they do not share, which is why this is an interface and not a base
//
// The differences are decisions rather than variations, and flattening them
// would be the wrong trade:
//
//   - **The chain event is unconditional for a tier transition and conditional
//     for an incident's.** `agent_audit.run_id` is an AgentRun's key (P4 F254),
//     an Incident may name no run (R-44), and a *run* is the chain's own key by
//     construction. So the one act that can never be outside the hash chain is
//     the one about authority, which is the right way round and is a property of
//     the subject rather than a choice either function made.
//   - **The permitted edges come from different places.** An incident's are
//     Appendix A.5's graph; a tier's are an ordered scale, so *which* edges exist
//     is a question the tier vocabulary answers differently — see
//     tier_transition.go's note on why no per-step graph is invented here.
//   - **Who may act differs, and only one of them has a rule.** Either kind of
//     actor may move an incident; only a human may *raise* a tier, because a
//     gate the gated party can open is not a gate.
//
// # What this interface is for
//
// It is not a dispatch point — nothing ranges over a []StateChange, and if
// something ever does, that is a third instance and a reason to look again. It
// exists so that the sentence *these are the same shape of act* has a compiler
// behind it: adding a field to one writer that the other has no answer for now
// fails to build rather than being noticed by a reader.

// StateChange is the shape both recorded acts take: a subject, an expected
// state, a new one, and a party who takes responsibility for the move.
//
// Both *IncidentTransition and *TierTransition satisfy it, and
// TestBothRecordedActsAreTheSameShape is what holds them to it.
type StateChange interface {
	// Subject is the row whose state moves — an incident id, a run id.
	Subject() string
	// From is the state the caller expects to find. It is the guard, not a
	// description: a state change recorded against a subject that has moved
	// underneath the caller is refused rather than applied.
	From() string
	// To is the state the subject moves to.
	To() string
	// Attribution is who acted and of which kind. The kind is a column in both
	// records rather than something inferred from the actor's spelling, on
	// I-292's argument: a denominator decided by a regular expression over an
	// email address is not a denominator.
	Attribution() (actor, kind string)
	// Reason is the actor's own account of the move, in their words. It is
	// required for some moves and not for others, and which is a property of
	// the act rather than of this interface.
	Reason() string
}

// validateAttribution is the attribution rule both writers apply.
//
// **It refuses rather than defaulting, and the actor kind is why.** A
// transition whose kind is unknown counts for nothing in a metric that
// partitions on it — `HumanVerdicts` is exactly such a query — so guessing a
// kind would produce a row that reads as a label and is not one. `what` names
// the record in the message, since a caller holding several has to be told
// which.
func validateAttribution(what, actor, kind string) error {
	if actor == "" {
		return fmt.Errorf("%s names no actor; an unattributable transition is the one thing this record exists to prevent", what)
	}
	if !KnownActorKind(kind) {
		return fmt.Errorf("%s carries actor kind %q; it must be %q or %q, because a label a metric cannot attribute is not a label",
			what, kind, ActorHuman, ActorAgent)
	}
	return nil
}

// Subject, From, To, Attribution and Reason put *IncidentTransition into the
// shape above. They are accessors over fields that were already there — this
// task added no column to jetsapi.incident_event and changed no behaviour of
// RecordIncidentTransition.
func (t *IncidentTransition) Subject() string { return t.IncidentRef }

// From is the status the incident is expected to be in.
func (t *IncidentTransition) From() string { return t.FromStatus }

// To is the status it moves to.
func (t *IncidentTransition) To() string { return t.ToStatus }

// Attribution is the actor and the actor kind I-276 asked for.
func (t *IncidentTransition) Attribution() (string, string) { return t.Actor, t.ActorKind }

// Reason is the transition rationale — required for a reclassification by
// incident_event_reclassified_ck, optional otherwise.
func (t *IncidentTransition) Reason() string { return t.Rationale }
