package audit

import (
	"reflect"
	"testing"
)

// Appendix A.5's Incident machine, asserted edge by edge. A transition table
// that is never tested against the diagram it transcribes is a diagram
// transcribed once.
func TestIncidentTransitionsFollowAppendixA5(t *testing.T) {
	for _, tc := range []struct {
		from string
		want []string
	}{
		{IncidentDetected, []string{IncidentTriaged}},
		// Three successors, all three adjudications — which is why plan §10.7
		// calls the supervision screen the labelling instrument.
		{IncidentTriaged, []string{IncidentDiagnosed, IncidentReclassified, IncidentSuppressedAsBenign}},
		{IncidentDiagnosed, []string{IncidentRemediationProposed}},
		// The back edge A.5 draws across the top of the figure.
		{IncidentRemediationProposed, []string{IncidentAwaitingApproval, IncidentTriaged}},
		{IncidentAwaitingApproval, []string{IncidentRemediating}},
		{IncidentRemediating, []string{IncidentResolved}},
		// The prose, not the ASCII: "reclassified returns to triaged with a new
		// classification and a recorded reason" (I-298).
		{IncidentReclassified, []string{IncidentTriaged}},
		{IncidentResolved, []string{IncidentVerified}},
		{IncidentVerified, []string{IncidentClosed}},
		// A.5: "suppressed_as_benign and closed are terminal."
		{IncidentClosed, nil},
		{IncidentSuppressedAsBenign, nil},
		{"compacted", nil},
	} {
		got := IncidentTransitions(tc.from)
		want := append([]string(nil), tc.want...)
		if len(want) > 1 {
			// IncidentTransitions sorts; the table above is written in A.5's
			// reading order, which is the order a human checks it in.
			want = sortedCopy(want)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("from %s: got %v, want %v", tc.from, got, want)
		}
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// A.5 names one illegal transition explicitly. It is the cheapest possible
// assertion that the table is a permitted-transitions-only graph rather than a
// list of suggestions.
func TestDetectedToResolvedIsIllegal(t *testing.T) {
	if IncidentTransitionAllowed(IncidentDetected, IncidentResolved) {
		t.Fatal("A.5 says in terms that detected -> resolved is illegal")
	}
}

// Terminality is a property of the state and not of the table's rows: a status
// with no forward edge is not thereby terminal, and reading it that way would
// make an unlisted state look closed.
func TestIncidentTerminalStates(t *testing.T) {
	for _, s := range IncidentStatuses {
		want := s == IncidentClosed || s == IncidentSuppressedAsBenign
		if IncidentTerminal(s) != want {
			t.Errorf("%s: terminal = %v, want %v", s, IncidentTerminal(s), want)
		}
	}
}

// Every status in the transition table has to be one of the eleven, in both
// directions. This is the guard against a typo in a map key — a state that
// silently never matches, which is lifecycle.go's own stated reason for named
// constants.
func TestIncidentForwardTableUsesOnlyKnownStatuses(t *testing.T) {
	for from, tos := range incidentForward {
		if !KnownIncidentStatus(from) {
			t.Errorf("transition table has key %q, which is not an IncidentStatus", from)
		}
		for _, to := range tos {
			if !KnownIncidentStatus(to) {
				t.Errorf("%s -> %q: target is not an IncidentStatus", from, to)
			}
		}
	}
	if len(IncidentStatuses) != 11 {
		t.Errorf("A.4 lists eleven IncidentStatus values; this file has %d", len(IncidentStatuses))
	}
}

// Every non-terminal status must be reachable, or the machine has a state
// nothing can enter — which is how a vocabulary member ends up looking
// supported while being unwritable.
func TestEveryStatusIsReachableExceptTheEntryState(t *testing.T) {
	reached := map[string]bool{IncidentDetected: true}
	for _, tos := range incidentForward {
		for _, to := range tos {
			reached[to] = true
		}
	}
	for _, s := range IncidentStatuses {
		if !reached[s] {
			t.Errorf("%s is in the vocabulary and no transition reaches it", s)
		}
	}
}
