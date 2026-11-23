package audit

import (
	"reflect"
	"strings"
	"testing"
)

// The design's Incident machine, asserted edge by edge. A transition table
// that is never tested against the diagram it transcribes is a diagram
// transcribed once.
func TestIncidentTransitionsFollowAppendixA5(t *testing.T) {
	for _, tc := range []struct {
		from string
		want []string
	}{
		{IncidentDetected, []string{IncidentTriaged}},
		// Three successors, all three adjudications — which is why the supervision design
		// calls the supervision screen the labelling instrument.
		{IncidentTriaged, []string{IncidentDiagnosed, IncidentReclassified, IncidentSuppressedAsBenign}},
		// Not the design's: a person's verdict on a diagnosis (incidentHumanOnly).
		{IncidentDiagnosed, []string{IncidentRemediationProposed, IncidentReclassified, IncidentSuppressedAsBenign}},
		// The back edge the design draws across the top of the figure.
		{IncidentRemediationProposed, []string{IncidentAwaitingApproval, IncidentTriaged}},
		{IncidentAwaitingApproval, []string{IncidentRemediating}},
		{IncidentRemediating, []string{IncidentResolved}},
		// The prose, not the ASCII: "reclassified returns to triaged with a new
		// classification and a recorded reason".
		{IncidentReclassified, []string{IncidentTriaged}},
		{IncidentResolved, []string{IncidentVerified}},
		{IncidentVerified, []string{IncidentClosed}},
		// "suppressed_as_benign and closed are terminal."
		{IncidentClosed, nil},
		{IncidentSuppressedAsBenign, nil},
		{"compacted", nil},
	} {
		got := IncidentTransitions(tc.from)
		want := append([]string(nil), tc.want...)
		if len(want) > 1 {
			// IncidentTransitions sorts; the table above is written in the design's
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

// The design names one illegal transition explicitly. It is the cheapest possible
// assertion that the table is a permitted-transitions-only graph rather than a
// list of suggestions.
func TestDetectedToResolvedIsIllegal(t *testing.T) {
	if IncidentTransitionAllowed(IncidentDetected, IncidentResolved) {
		t.Fatal("the lifecycle design says in terms that detected -> resolved is illegal")
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
		t.Errorf("the model lists eleven IncidentStatus values; this file has %d", len(IncidentStatuses))
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

// The two edges from a diagnosis to a verdict are a person's, and only these
// two: every other edge leaves the actor kind to the attribution rule.
func TestADiagnosisTakesOnlyAPersonsVerdict(t *testing.T) {
	for _, to := range []string{IncidentReclassified, IncidentSuppressedAsBenign} {
		if !IncidentTransitionHumanOnly(IncidentDiagnosed, to) {
			t.Errorf("diagnosed -> %s should be a person's alone", to)
		}
		for _, tc := range []struct {
			kind string
			ok   bool
		}{{ActorHuman, true}, {ActorAgent, false}} {
			err := (&IncidentTransition{
				IncidentEventId: "ie_v", IncidentRef: "inc_v",
				FromStatus: IncidentDiagnosed, ToStatus: to,
				Actor: "someone", ActorKind: tc.kind,
			}).validate()
			if tc.ok && err != nil {
				t.Errorf("diagnosed -> %s by a %s: %v", to, tc.kind, err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), "a person's verdict")) {
				t.Errorf("diagnosed -> %s by an %s: want a refusal, got %v", to, tc.kind, err)
			}
		}
	}
	for _, from := range IncidentStatuses {
		for _, to := range IncidentTransitions(from) {
			want := from == IncidentDiagnosed && (to == IncidentReclassified || to == IncidentSuppressedAsBenign)
			if IncidentTransitionHumanOnly(from, to) != want {
				t.Errorf("%s -> %s: human only = %v, want %v", from, to, !want, want)
			}
		}
	}
}
