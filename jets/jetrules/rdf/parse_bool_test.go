package rdf

import (
	"errors"
	"testing"
)

// The specification of rules_unit_test_harness I-45: the accepted spellings,
// trimmed and in any case; empty is null; anything else is an error. The C++
// counterpart, rdf::parse_bool, is pinned to the same table in
// jets/rdf/parse_bool_test.cc.
func TestParseBool(t *testing.T) {
	for _, value := range []string{"true", "TRUE", "True", "t", "T", "yes", "YES", "Yes", "y", "Y", "1", " true ", "\tY\n"} {
		if b, err := ParseBool(value); err != nil || !b {
			t.Errorf("ParseBool(%q) = %v, %v; want true", value, b, err)
		}
	}
	for _, value := range []string{"false", "FALSE", "f", "F", "no", "NO", "n", "N", "0", " no "} {
		if b, err := ParseBool(value); err != nil || b {
			t.Errorf("ParseBool(%q) = %v, %v; want false", value, b, err)
		}
	}
	for _, value := range []string{"", "  ", "\t"} {
		if _, err := ParseBool(value); !errors.Is(err, ErrNullBool) {
			t.Errorf("ParseBool(%q) error = %v, want ErrNullBool", value, err)
		}
	}
	for _, value := range []string{"maybe", "tomato", "nope", "yess", "2", "-1", "10", "on", "off", "\"true\""} {
		_, err := ParseBool(value)
		if err == nil || errors.Is(err, ErrNullBool) {
			t.Errorf("ParseBool(%q) error = %v, want an error naming the value", value, err)
		}
	}
}

// What a rule sees when it tests a node for truth.
func TestNodeTruth(t *testing.T) {
	cases := []struct {
		node    *Node
		want    bool
		wantErr bool
	}{
		{&Node{Value: "yes"}, true, false},
		{&Node{Value: "false"}, false, false},
		{&Node{Value: ""}, false, false}, // null
		{&Node{Value: "maybe"}, false, true},
		{&Node{Value: 1}, true, false},
		{&Node{Value: 0}, false, false},
		{Null(), false, false},
		{nil, false, false},
	}
	for _, c := range cases {
		got, err := c.node.Truth()
		if got != c.want || (err != nil) != c.wantErr {
			t.Errorf("Truth(%v) = %v, %v; want %v, error %v", c.node, got, err, c.want, c.wantErr)
		}
		if c.node != nil && got != c.node.Bool() {
			t.Errorf("Bool(%v) = %v, disagrees with Truth", c.node, c.node.Bool())
		}
	}
}
