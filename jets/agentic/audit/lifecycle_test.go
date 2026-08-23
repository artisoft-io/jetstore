package audit

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The lifecycle table is pure, so these need no database. That is the point of
// putting the policy in a table rather than in SQL or in a screen.

// The vocabulary here and the CHECK constraint the DDL generates must agree. A
// state the table offers that the CHECK refuses fails at the INSERT, in
// production, after a human clicked a button — so this reads the embedded
// schema and compares the two sets rather than trusting that both were edited
// together.
func TestApprovalStatesMatchTheGeneratedCheck(t *testing.T) {
	sql, err := os.ReadFile("agent_audit.sql")
	if err != nil {
		t.Fatalf("reading agent_audit.sql: %v", err)
	}
	re := regexp.MustCompile(`change_proposal_approval_state_ck CHECK \(approval_state IN \(([^)]*)\)\)`)
	m := re.FindSubmatch(sql)
	if m == nil {
		t.Fatal("could not find the change_proposal approval_state CHECK; this test is stale")
	}
	inCheck := map[string]bool{}
	for _, lit := range strings.Split(string(m[1]), ",") {
		inCheck[strings.Trim(strings.TrimSpace(lit), "'")] = true
	}
	if len(inCheck) != len(ApprovalStates) {
		t.Errorf("the CHECK has %d states and ApprovalStates has %d", len(inCheck), len(ApprovalStates))
	}
	for _, s := range ApprovalStates {
		if !inCheck[s] {
			t.Errorf("%q is in ApprovalStates and not in the CHECK", s)
		}
		delete(inCheck, s)
	}
	for s := range inCheck {
		t.Errorf("%q is in the CHECK and not in ApprovalStates", s)
	}
}

// A.5's ChangeProposal machine, edge by edge, restricted to the modelled
// vocabulary. Written out rather than derived from `forward`, so that the test
// disagrees with the table when one of them changes.
func TestTransitionsFollowAppendixA5(t *testing.T) {
	want := map[string][]string{
		StateDraft:                    {StateSuperseded, StateValidated},
		StateValidated:                {StateAgentReviewed, StateSuperseded},
		StateAgentReviewed:            {StateAwaitingHumanApproval, StateSuperseded},
		StateAwaitingHumanApproval:    {StateApproved, StateApprovedWithModification, StateRejected, StateSuperseded},
		StateApproved:                 {StateDeployed, StateSuperseded},
		StateApprovedWithModification: {StateDeployed, StateSuperseded},
		StateDeployed:                 {StateSuperseded},
		StateRejected:                 nil,
		StateSuperseded:               nil,
	}
	for from, expect := range want {
		got := Transitions(from)
		if len(got) != len(expect) {
			t.Errorf("from %s: got %v, want %v", from, got, expect)
			continue
		}
		for i := range got {
			if got[i] != expect[i] {
				t.Errorf("from %s: got %v, want %v", from, got, expect)
				break
			}
		}
	}
}

// The two states A.5 marks as ends of the line admit nothing, including
// `superseded`. Superseding a rejected proposal would rewrite a decision.
func TestTerminalStatesOfferNothing(t *testing.T) {
	for _, s := range []string{StateRejected, StateSuperseded} {
		if !Terminal(s) {
			t.Errorf("%s should be terminal", s)
		}
		if got := Transitions(s); len(got) != 0 {
			t.Errorf("%s is terminal and offers %v", s, got)
		}
		if TransitionAllowed(s, StateApproved) {
			t.Errorf("%s -> approved was allowed", s)
		}
	}
}

// The two mistakes a decision screen invites: skipping the lifecycle, and
// reversing a decision that was already taken.
func TestTheEdgesThatMustNotExist(t *testing.T) {
	forbidden := [][2]string{
		{StateDraft, StateApproved},
		{StateDraft, StateDeployed},
		{StateValidated, StateApproved},
		{StateAwaitingHumanApproval, StateDeployed},
		{StateRejected, StateApproved},
		{StateSuperseded, StateDraft},
		{StateApproved, StateRejected},
		{StateDeployed, StateDraft},
	}
	for _, e := range forbidden {
		if TransitionAllowed(e[0], e[1]) {
			t.Errorf("%s -> %s is allowed and should not be", e[0], e[1])
		}
	}
}

// `superseded` is reachable from every non-terminal state — A.5's own sentence,
// and the one rule Transitions synthesises rather than reads off `forward`.
func TestSupersededIsReachableFromEveryNonTerminalState(t *testing.T) {
	for _, s := range ApprovalStates {
		if Terminal(s) {
			continue
		}
		if !TransitionAllowed(s, StateSuperseded) {
			t.Errorf("%s cannot be superseded", s)
		}
	}
}

// A state name that is not one of the nine yields no transitions rather than
// panicking or silently behaving like a terminal state that exists. The
// distinction matters at the endpoint, which turns "no transitions" into a
// refusal naming what is permitted.
func TestUnknownStateIsNotOfferedAnything(t *testing.T) {
	for _, s := range []string{"", "drafted", "verified", "closed", "rolled_back", "APPROVED"} {
		if KnownState(s) {
			t.Errorf("%q should not be a known state", s)
		}
		if got := Transitions(s); got != nil {
			t.Errorf("unknown state %q offered %v", s, got)
		}
	}
}

// Transitions must not hand out the slice the table holds: a caller that
// appends to the result would corrupt the policy for every later request.
func TestTransitionsDoesNotAliasTheTable(t *testing.T) {
	first := Transitions(StateAwaitingHumanApproval)
	first[0] = "clobbered"
	second := Transitions(StateAwaitingHumanApproval)
	if second[0] == "clobbered" {
		t.Error("Transitions returned the table's own slice")
	}
}
