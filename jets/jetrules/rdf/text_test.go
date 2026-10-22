package rdf

import (
	"math"
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

// DoubleTextCases is the shared table of rules_benchmark I-16 (Q-60): how both
// engines write a double into text. The C++ engine's test copies it, so a change
// here is a change there. Each text was checked against C++'s
// std::to_chars(..., std::chars_format::fixed) on 2026-10-22.
var DoubleTextCases = []struct {
	Value float64
	Text  string
}{
	{991, "991"},
	{-991, "-991"},
	{0, "0"},
	{math.Copysign(0, -1), "-0"},
	{1, "1"},
	{0.5, "0.5"},
	{-2.75, "-2.75"},
	{1.5, "1.5"},
	{100.1, "100.1"},
	{12.34, "12.34"},
	{0.891089, "0.891089"},
	{1234567.5, "1234567.5"},
	{1234567.89, "1234567.89"},
	{1e6, "1000000"},
	{0.00001, "0.00001"},
	{1e-7, "0.0000001"},
	{0.30000000000000004, "0.30000000000000004"}, // 0.1 + 0.2 at run time
	{0.3333333333333333, "0.3333333333333333"},
	{0.6666666666666666, "0.6666666666666666"},
	{123456789.123, "123456789.123"},
	{1 << 53, "9007199254740992"},
	{123456789012345678, "123456789012345680"},
	{1e21, "1000000000000000000000"},
	{1e23, "99999999999999991611392"},
	{-1e23, "-99999999999999991611392"},
	{math.NaN(), "nan"},
	{math.Inf(1), "inf"},
	{math.Inf(-1), "-inf"},
}

func TestFormatDoubleTable(t *testing.T) {
	for _, c := range DoubleTextCases {
		if got := FormatDouble(c.Value); got != c.Text {
			t.Errorf("FormatDouble(%v) = %q, want %q", c.Value, got, c.Text)
		}
		// Text plus a double, the I-16 case itself ("rate: 991" on both engines).
		if got := S("rate: ").ADD(F(c.Value)); got == nil || got.Value != "rate: "+c.Text {
			t.Errorf("\"rate: \" + %v = %v, want %q", c.Value, got, "rate: "+c.Text)
		}
		if got := TextOf(F(c.Value)); got != c.Text {
			t.Errorf("TextOf(%v) = %q, want %q", c.Value, got, c.Text)
		}
	}
}

// create_resource and create_entity name a resource from a double the same way.
func TestCreateResourceFromDouble(t *testing.T) {
	for _, c := range []struct {
		v    float64
		want string
	}{{1e6, "1000000"}, {12.5, "12.5"}, {0.00001, "0.00001"}} {
		if got := F(c.v).CreateResource(nil); got == nil || got.Name() != c.want {
			t.Errorf("create_resource(%v) = %v, want %q", c.v, got, c.want)
		}
		rm := NewResourceManager(nil)
		sess := NewRdfSession(rm, NewMetaRdfGraph(rm))
		if entity, key := F(c.v).CreateEntityKey(sess); entity == nil || entity.Name() != c.want || key.Value != c.want {
			t.Errorf("create_entity(%v) = %v, %v, want %q", c.v, entity, key, c.want)
		}
	}
}
