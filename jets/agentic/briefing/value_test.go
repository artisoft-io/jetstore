package briefing

import (
	"encoding/json"
	"testing"
	"time"
)

// The readings that were three copies until `AY.5`. What is asserted here is
// the **union**, because that is the decision the merge made: the checker's
// date table was wider than the two renderers' and the wider one is what all
// three now read.

func TestADateIsReadByEveryLayoutAnyCopyAccepted(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2025-08-14", time.Date(2025, 8, 14, 0, 0, 0, 0, time.UTC)},
		{"2025-08-14T10:30:00", time.Date(2025, 8, 14, 10, 30, 0, 0, time.UTC)},
		{"2025-08-14T10:30:00Z", time.Date(2025, 8, 14, 10, 30, 0, 0, time.UTC)},
		// The sixth layout, and the reason this test exists. It was in this
		// package's table and in neither renderer's, so a value of this shape
		// was a date to Check and not to a briefing: the clause would be
		// omitted and nobody told. That is I-643's predicted divergence, found
		// in the tree rather than argued about.
		{"2025-08-14 10:30:00", time.Date(2025, 8, 14, 10, 30, 0, 0, time.UTC)},
		{"2025-8-14", time.Date(2025, 8, 14, 0, 0, 0, 0, time.UTC)},
		{"2025-8-14T10:30:00", time.Date(2025, 8, 14, 10, 30, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got, ok := AsDate(c.in)
		if !ok {
			t.Errorf("AsDate(%q) reports no date", c.in)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("AsDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	// A blank value is not a date, which is the rule `require` reads as *a
	// property that is there and says nothing is not a property that is there*.
	for _, blank := range []string{"", "   ", "\t"} {
		if _, ok := AsDate(blank); ok {
			t.Errorf("AsDate(%q) reports a date", blank)
		}
	}

	// The pointer arm is the renderers' and the checker did not have it.
	when := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	if got, ok := AsDate(&when); !ok || !got.Equal(when) {
		t.Errorf("AsDate(*time.Time) = %v %v", got, ok)
	}
	var nilTime *time.Time
	if _, ok := AsDate(nilTime); ok {
		t.Error("AsDate(nil *time.Time) reports a date")
	}
}

// TestANumberReadsTheSameWhicheverCopyWantedIt pins the two arms that were in
// one copy and not the other: `json.Number` was the checker's and the wider
// integer widths were the engine's.
func TestANumberReadsTheSameWhicheverCopyWantedIt(t *testing.T) {
	for _, v := range []any{3, int32(3), int64(3), uint(3), uint32(3), uint64(3),
		float32(3), float64(3), "3", json.Number("3")} {
		if n, ok := AsInt(v); !ok || n != 3 {
			t.Errorf("AsInt(%T %v) = %d %v", v, v, n, ok)
		}
		if f, ok := AsFloat(v); !ok || f != 3 {
			t.Errorf("AsFloat(%T %v) = %v %v", v, v, f, ok)
		}
	}
	// A count that arrives as 2.0 from json or toon is the integer 2, and a
	// ratio compared with 0.8 is not truncated to 0. The two readers exist for
	// that difference and this is it.
	if n, ok := AsInt(2.9); !ok || n != 2 {
		t.Errorf("AsInt(2.9) = %d %v, want 2", n, ok)
	}
	if f, ok := AsFloat("0.89"); !ok || f != 0.89 {
		t.Errorf("AsFloat(\"0.89\") = %v %v", f, ok)
	}
}

// TestAValueRendersWithoutATrailingZero is the property a briefing depends on:
// json decodes every number as a float64, and a fill count that printed as
// `3.0` would reach a member.
func TestAValueRendersWithoutATrailingZero(t *testing.T) {
	for _, c := range []struct{ in, want any }{
		{float64(3), "3"},
		{3, "3"},
		{nil, ""},
		{true, "true"},
		{time.Date(2025, 8, 14, 10, 30, 0, 0, time.UTC), "2025-08-14"},
	} {
		if got := ValueText(c.in); got != c.want {
			t.Errorf("ValueText(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
