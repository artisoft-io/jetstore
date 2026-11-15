package audit

import "sort"

// The Incident lifecycle — which transitions the server
// will accept on an incident, and which a supervision screen may offer.
//
// **This is lifecycle.go's shape a second time, and the repetition is the
// point.** RecordApproval guards a transition on its expected
// from_state and says nothing about which state may follow which; a later change added the
// policy for ChangeProposal, in a table the handler enforces and the screen
// reads. Incident needs the same two things, and needs them
// because a transition on an incident was a mutable `status` column with no
// actor, no timestamp and no record — so a human's correction and the agent's
// own reclassification were the same row.
//
// # Where the graph comes from
//
// The design's *Lifecycle State Machines*, which draw the
// Incident machine first and whose preamble governs this whole file:
// "Transitions are enforced deterministically. An agent may propose a
// transition; it cannot effect an illegal one. Every transition records actor,
// agent version where applicable, and timestamp."
//
// **The design's Incident machine is fully expressible in the model's vocabulary, and its
// ChangeProposal machine was not.** lifecycle.go records that the design draws four
// states — `verified`, `closed`, `rolled_back` and the spelling `drafted` —
// that ApprovalState does not carry, so `deployed` there has no successor but
// `superseded`. IncidentStatus's eleven values cover every state drawn here,
// including `verified` and `closed`. That is worth stating rather than
// enjoying: the same appendix, read the same way, gives a complete answer for
// one entity and a truncated one for another, so *"the design is the source"* is not
// on its own a claim that the design was sufficient.
//
// **The diagram and the prose disagree about `reclassified`, and the prose is
// followed.** The prose is explicit — "`reclassified` returns to `triaged` with
// a new classification and a recorded reason" — while the ASCII routes
// `reclassified`'s line down the page and into `resolved`, which would make a
// reclassification a resolution. Read as a graph the second is also
// unreachable from anything the prose says. So `reclassified -> triaged` is
// the edge, and `reclassified -> resolved` is not. The rationale
// half of that sentence is enforced one layer down, by
// `incident_event_reclassified_ck` in the generated DDL, and not here: a
// transition table says which edges exist and a CHECK says what a row must
// carry to travel one.
//
// **`superseded` has no analogue here.** The design grants "reachable from any
// non-terminal state" to ChangeProposal alone, and IncidentStatus has no such
// member. Nothing is added by symmetry.

// The eleven IncidentStatus values, mirroring the model's IncidentStatus and
// the CHECK constraints the DDL generates from it. Named constants because a
// transition table written in string literals is a typo away from a state that
// silently never matches — lifecycle.go's argument, unchanged.
const (
	IncidentDetected            = "detected"
	IncidentTriaged             = "triaged"
	IncidentDiagnosed           = "diagnosed"
	IncidentRemediationProposed = "remediation_proposed"
	IncidentAwaitingApproval    = "awaiting_approval"
	IncidentRemediating         = "remediating"
	IncidentResolved            = "resolved"
	IncidentVerified            = "verified"
	IncidentClosed              = "closed"
	IncidentReclassified        = "reclassified"
	IncidentSuppressedAsBenign  = "suppressed_as_benign"
)

// **`IncidentStatuses` and `KnownIncidentStatus` are `incident.go`'s**, not this
// file's, and that is a merge rather than a design. The incident
// read path landed hours before this did, and both had written the eleven
// values out — the same vocabulary, from the same model, in two files in one
// package. Theirs is kept because it merged first and because its ordering
// carries an argument this file does not need: it puts the three adjudications
// last, so that a screen filtering on status can see it is filtering on two
// kinds of thing. **The constants below are still this file's**, because a
// transition table written in string literals is a typo away from a state that
// silently never matches, and `TestIncidentForwardTableUsesOnlyKnownStatuses`
// asserts every one of them against their list.

// The two states the design marks terminal in terms: "`suppressed_as_benign` and
// `closed` are terminal."
var incidentTerminal = map[string]bool{
	IncidentClosed:             true,
	IncidentSuppressedAsBenign: true,
}

// IncidentTerminal reports whether a status admits no further transition.
func IncidentTerminal(status string) bool { return incidentTerminal[status] }

// incidentForward is the design's Incident machine, edge for edge.
//
// Two edges are worth pointing at because a reader will check them:
//
//   - `remediation_proposed -> triaged` is the back edge the diagram draws
//     across the top of the figure. A proposal that turns out to address the
//     wrong thing sends the incident back to triage rather than forward.
//   - `triaged` is the only state with three successors, and all three of its
//     branches are adjudications: diagnose it, reclassify it, or call it
//     benign. That is not a coincidence — it is why the supervision design calls the
//     supervision screen the labelling instrument.
var incidentForward = map[string][]string{
	IncidentDetected:            {IncidentTriaged},
	IncidentTriaged:             {IncidentDiagnosed, IncidentReclassified, IncidentSuppressedAsBenign},
	IncidentDiagnosed:           {IncidentRemediationProposed},
	IncidentRemediationProposed: {IncidentAwaitingApproval, IncidentTriaged},
	IncidentAwaitingApproval:    {IncidentRemediating},
	IncidentRemediating:         {IncidentResolved},
	IncidentReclassified:        {IncidentTriaged},
	IncidentResolved:            {IncidentVerified},
	IncidentVerified:            {IncidentClosed},
}

// IncidentTransitions returns the statuses an incident in `from` may move to,
// sorted, or nil when it is terminal or unknown. The sort is not cosmetic: the
// result is rendered as a row of buttons and serialised into a response, and an
// unordered map range would reorder them between requests.
func IncidentTransitions(from string) []string {
	if !KnownIncidentStatus(from) || IncidentTerminal(from) {
		return nil
	}
	out := append([]string(nil), incidentForward[from]...)
	sort.Strings(out)
	return out
}

// IncidentTransitionAllowed reports whether `from -> to` is one of the lifecycle's edges.
// This is the check a handler makes before calling RecordIncidentTransition;
// that function's own guard is a different question — whether the incident is
// still in `from` — and both are needed. The design's own illegal example is
// `detected -> resolved`.
func IncidentTransitionAllowed(from, to string) bool {
	for _, s := range IncidentTransitions(from) {
		if s == to {
			return true
		}
	}
	return false
}
