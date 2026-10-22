package rdf

import (
	"testing"
	"time"
)

// rules_benchmark I-32: a date, a datetime or a null written into text reads as
// the C++ engine writes it (rdf::to_string, boost's ISO extended forms), not as
// Go's struct form {2025-06-05 00:00:00 +0000 UTC}. The datetime forms were
// read off boost by compiling to_iso_extended_string on 2026-10-22.
func TestTextPlusDateIsIso(t *testing.T) {
	dt := func(us int) *Node {
		v := time.Date(2025, 6, 5, 13, 4, 5, us*1000, time.UTC)
		return &Node{Value: LDatetime{Datetime: &v}}
	}
	cases := []struct {
		rhs  *Node
		want string
	}{
		{DD("2025-06-05"), "key:2025-06-05"},
		{dt(0), "key:2025-06-05T13:04:05"},
		{dt(500000), "key:2025-06-05T13:04:05.500000"},
		{dt(123), "key:2025-06-05T13:04:05.000123"},
		{DDT("2025-06-05"), "key:2025-06-05T00:00:00"},
		{Null(), "key:null"},
	}
	for _, c := range cases {
		got := S("key:").ADD(c.rhs)
		if got == nil || got.Value != c.want {
			t.Errorf("\"key:\" + %v = %v, want %q", c.rhs.Value, got, c.want)
		}
		if got := "key:" + TextOf(c.rhs); got != c.want {
			t.Errorf("TextOf(%v) gives %q, want %q", c.rhs.Value, got, c.want)
		}
	}
}
