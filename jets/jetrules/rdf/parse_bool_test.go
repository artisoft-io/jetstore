package rdf

import "testing"

func TestParseBool(t *testing.T) {
	for value, want := range map[string]int{
		"true": 1, "TRUE": 1, "t": 1, "T": 1, "1": 1, "yes": 1, "YES": 1, "Yes": 1,
		"false": 0, "f": 0, "0": 0, "no": 0, "NO": 0, "": 0, "y": 0, "maybe": 0,
	} {
		if got := ParseBool(value); got != want {
			t.Errorf("ParseBool(%q) = %d, want %d", value, got, want)
		}
	}
}
