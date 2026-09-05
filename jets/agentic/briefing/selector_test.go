package briefing

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("test fixture is not json: %v", err)
	}
	return m
}

func TestParseSelectorRefusals(t *testing.T) {
	for _, tc := range []struct{ name, sel string }{
		{"empty", ""},
		{"blank", "   "},
		{"empty segment", "a..b"},
		{"trailing dot", "a."},
		// The one that matters: valid InferMappingSpec.Path notation, refused
		// here because selecting one event by position would let a membership
		// check pass on the wrong event.
		{"array index", "has_Medical_Events.0.Diagnosis"},
		{"index alone", "0"},
		{"bracket mid segment", "events[0].code"},
		{"unclosed bracket", "events[.code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseSelector(tc.sel); err == nil {
				t.Fatalf("ParseSelector(%q) was accepted and should not be", tc.sel)
			}
		})
	}
}

func TestParseSelectorAccepts(t *testing.T) {
	for _, sel := range []string{"a", "a.b", "a[]", "a[].b", "a[].b[].c", "jets:key"} {
		if _, err := ParseSelector(sel); err != nil {
			t.Errorf("ParseSelector(%q): %v", sel, err)
		}
	}
}

func TestResolve(t *testing.T) {
	doc := mustJSON(t, `{
	  "member": {"plan_id": "P1"},
	  "events": [
	    {"code": "B182", "date": "2025-08-14"},
	    {"code": "F1120", "date": "2025-11-13"}
	  ],
	  "one_event": {"code": "I330"},
	  "empty": []
	}`)
	cases := []struct {
		sel      string
		pointers []string
		values   []string
	}{
		{"member.plan_id", []string{"/member/plan_id"}, []string{"P1"}},
		{"events[].code", []string{"/events/0/code", "/events/1/code"}, []string{"B182", "F1120"}},
		{"empty[].code", nil, nil},
		{"absent[].code", nil, nil},
		{"member.absent", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.sel, func(t *testing.T) {
			s, err := ParseSelector(tc.sel)
			if err != nil {
				t.Fatal(err)
			}
			got := s.Resolve(doc)
			if len(got) != len(tc.pointers) {
				t.Fatalf("resolved %d values, want %d: %+v", len(got), len(tc.pointers), got)
			}
			for i := range got {
				if got[i].Pointer != tc.pointers[i] {
					t.Errorf("pointer %d: got %q want %q", i, got[i].Pointer, tc.pointers[i])
				}
				if got[i].Value != any(tc.values[i]) {
					t.Errorf("value %d: got %v want %v", i, got[i].Value, tc.values[i])
				}
			}
		})
	}
}

// A multi-valued property with one value is serialised as a bare scalar by
// addToEntityObj (jets/compute_pipes/jetrules_extract_entity.go:99), so `[]`
// over a scalar has to yield that scalar. Without this, every membership check
// fails for a patient with one event.
func TestResolveEachOverScalar(t *testing.T) {
	doc := mustJSON(t, `{"has_Medical_Events": {"Diagnosis": "B182"}}`)
	s, err := ParseSelector("has_Medical_Events[].Diagnosis")
	if err != nil {
		t.Fatal(err)
	}
	got := s.Resolve(doc)
	if len(got) != 1 || got[0].Value != any("B182") {
		t.Fatalf("got %+v, want the single scalar", got)
	}
	if got[0].Pointer != "/has_Medical_Events/Diagnosis" {
		t.Errorf("pointer %q", got[0].Pointer)
	}
}

func TestPointerEscaping(t *testing.T) {
	doc := mustJSON(t, `{"jets:key": "k", "a/b": {"c~d": 1}}`)
	s, _ := ParseSelector("a/b.c~d")
	got := s.Resolve(doc)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Pointer != "/a~1b/c~0d" {
		t.Errorf("RFC 6901 escaping: got %q want %q", got[0].Pointer, "/a~1b/c~0d")
	}
}

func TestCovers(t *testing.T) {
	doc := mustJSON(t, `{"events":[{"code":"a"},{"code":"b"}]}`)
	s, _ := ParseSelector("events[].code")
	if !s.Covers(doc, "/events/1/code") {
		t.Error("events[].code should cover /events/1/code")
	}
	if s.Covers(doc, "/events/2/code") {
		t.Error("events[].code should not cover an element that is not there")
	}
}

func TestLeaves(t *testing.T) {
	doc := mustJSON(t, `{"b": 1, "a": {"y": "v", "x": null}, "c": [true, "z"]}`)
	var got []Located
	leaves(doc, "", &got)
	want := []string{"/a/y", "/b", "/c/0", "/c/1"}
	if len(got) != len(want) {
		t.Fatalf("got %d leaves %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].Pointer != want[i] {
			t.Errorf("leaf %d: got %q want %q", i, got[i].Pointer, want[i])
		}
	}
}
