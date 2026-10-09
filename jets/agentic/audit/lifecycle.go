package audit

import "sort"

// The ChangeProposal approval lifecycle (task K.3, gap 11) — which transitions
// a decision screen may offer, and which the server will accept.
//
// **K.1 built the primitive and deliberately left the policy out.**
// RecordApproval guards on the expected from_state, so two approvers cannot
// both decide, and it constrains both ends against the nine-value vocabulary
// through the table's CHECK. What it does not do is say which of the nine may
// follow which — from its point of view `draft -> deployed` is a legal write.
// That was right for K.1: a record of decisions should not encode a workflow it
// was not asked about, and the guard it does carry is the one that makes the
// record trustworthy.
//
// **K.3 is what needs the policy, because a screen has to offer something.** A
// decision UI presents a set of buttons, and the set has to come from somewhere.
// The two ways to get it wrong are both worth naming: offering all nine
// (nonsense, and it invites `rejected -> approved`), or hard-coding a set in the
// React app (`shell/capabilities.tsx` says it in as many words — a client-side
// check is presentation, and the server is the enforcement point). So the table
// lives here, the handler enforces it, and the screen *reads* it. One source,
// and the screen cannot offer what the server would refuse.
//
// # Where the graph comes from
//
// Appendix A.5 of the proposal, *Lifecycle State Machines*, which draws the
// ChangeProposal machine explicitly and says of it: "Transitions are enforced
// deterministically. An agent may propose a transition; it cannot effect an
// illegal one." That sentence is this file. `superseded` being reachable from
// any non-terminal state is A.5's own sentence too.
//
// **The diagram names four states the vocabulary does not have, and this is the
// one place that has to notice.** A.5 draws `drafted -> validated`, and
// continues past `deployed` to `verified -> closed`, with `deployed ->
// rolled_back -> drafted`. A.4's ApprovalState vocabulary — the nine values in
// the CHECK, generated from `model.py:102` — contains `draft`, and contains
// none of `verified`, `closed` or `rolled_back`. So:
//
//   - `drafted` is read as `draft`, a spelling difference and nothing more;
//   - the three states after `deployed` are **not modelled**, so `deployed` has
//     no successor here except `superseded`. That is a truncation of A.5 rather
//     than a decision taken against it, and it is why `deployed` is not in
//     Terminal below: the vocabulary simply runs out.
//
// Recorded as a finding rather than resolved here, because widening a
// controlled vocabulary is a model change and belongs with whoever owns the
// post-deployment phase, not with an approval screen.

// The nine ApprovalState values, mirroring model.py:102 and the CHECK
// constraint the DDL generates from it. Named constants because a transition
// table written in string literals is a typo away from a state that silently
// never matches.
const (
	StateDraft                    = "draft"
	StateValidated                = "validated"
	StateAgentReviewed            = "agent_reviewed"
	StateAwaitingHumanApproval    = "awaiting_human_approval"
	StateApproved                 = "approved"
	StateApprovedWithModification = "approved_with_modification"
	StateRejected                 = "rejected"
	StateSuperseded               = "superseded"
	StateDeployed                 = "deployed"
)

// ApprovalStates is the vocabulary, in the order A.4 lists it — which is also
// roughly lifecycle order, and is the order a screen should render a legend in.
var ApprovalStates = []string{
	StateDraft, StateValidated, StateAgentReviewed, StateAwaitingHumanApproval,
	StateApproved, StateApprovedWithModification, StateRejected,
	StateSuperseded, StateDeployed,
}

// terminal states have no outgoing transition at all. A.5 marks `rejected`
// terminal in the diagram itself; `superseded` is terminal because a superseded
// proposal is one another proposal replaced, and moving it afterwards would
// rewrite a decision rather than record one.
var terminal = map[string]bool{
	StateRejected:   true,
	StateSuperseded: true,
}

// Terminal reports whether a state admits no further decision.
func Terminal(state string) bool { return terminal[state] }

// forward is A.5's ChangeProposal machine restricted to the modelled
// vocabulary. `superseded` is not listed on any row; Transitions adds it to
// every non-terminal state, which is A.5's "reachable from any non-terminal
// state" rather than nine hand-copied entries that can drift apart.
var forward = map[string][]string{
	StateDraft:                    {StateValidated},
	StateValidated:                {StateAgentReviewed},
	StateAgentReviewed:            {StateAwaitingHumanApproval},
	StateAwaitingHumanApproval:    {StateApproved, StateApprovedWithModification, StateRejected},
	StateApproved:                 {StateDeployed},
	StateApprovedWithModification: {StateDeployed},
	// StateDeployed: A.5 continues to verified/closed/rolled_back, none of
	// which the vocabulary has. Superseded only, supplied below.
	StateDeployed: nil,
}

// KnownState reports whether a string is one of the nine. A proposal row cannot
// hold anything else — the CHECK sees to that — but a request body can, and a
// caller naming a state that does not exist should be told so rather than
// silently offered no transitions.
func KnownState(state string) bool {
	for _, s := range ApprovalStates {
		if s == state {
			return true
		}
	}
	return false
}

// Transitions returns the states a proposal in `from` may move to, sorted, or
// nil when it is terminal or unknown.
//
// The sort is not cosmetic: the result is rendered as a row of buttons and
// serialised into a response, and an unordered map range would reorder them
// between requests.
func Transitions(from string) []string {
	if !KnownState(from) || Terminal(from) {
		return nil
	}
	out := append([]string(nil), forward[from]...)
	out = append(out, StateSuperseded)
	sort.Strings(out)
	return out
}

// TransitionAllowed reports whether `from -> to` is one of A.5's edges. This is
// the check the handler makes before calling RecordApproval; RecordApproval's
// own guard is a different question — whether the proposal is still in `from` —
// and both are needed.
func TransitionAllowed(from, to string) bool {
	for _, s := range Transitions(from) {
		if s == to {
			return true
		}
	}
	return false
}
