package rdf

import (
	"math"
	"testing"
)

// This file contains test cases for the Node's logic operators

func TestMathABS(t *testing.T) {

	if !I(1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !I(-1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if I(-1).ABS().Value.(int) != 1 {
		t.Error("operator failed")
	}

	if !F(1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !F(-1).ABS().EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if F(-1).ABS().Value.(float64) != 1 {
		t.Error("operator failed")
	}

	if DD("1960/10/10").ABS() != nil {
		t.Error("operator failed")
	}

	if DDT("1960/10/10").ABS() != nil {
		t.Error("operator failed")
	}

	if Null().ABS() != nil {
		t.Error("operator failed")
	}
}

func TestMathADD(t *testing.T) {
	if !I(1).ADD(I(1)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}
	if I(1).ADD(I(2)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}
	
	if !F(1).ADD(I(1)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}
	if !F(1).ADD(F(1)).EQ(F(2)).Bool() {
		t.Error("operator failed")
	}
	if F(1).ADD(I(2)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}

	if !DD("10/10/2010").ADD(I(1)).EQ(DD("10/11/2010")).Bool() {
		t.Error("operator failed")
	}

	if !DD("10/10/2010").ADD(F(1)).EQ(DD("10/11/2010")).Bool() {
		t.Error("operator failed")
	}

	if !DDT("10/10/2010").ADD(I(1)).EQ(DDT("10/11/2010")).Bool() {
		t.Error("operator failed")
	}

	if DD("10/10/2010").ADD(DD("10/11/2010")) != nil {
		t.Error("operator failed")
	}

	if DDT("10/10/2010").ADD(DDT("10/11/2010")) != nil {
		t.Error("operator failed")
	}

	if !S("Hello ").ADD(S("World")).EQ(S("Hello World")).Bool() {
		t.Error("operator failed")
	}

	if !S("Hello ").ADD(I(1)).EQ(S("Hello 1")).Bool() {
		t.Error("operator failed")
	}

	if !S("Hello ").ADD(F(1)).EQ(S("Hello 1")).Bool() {
		t.Error("operator failed")
	}
	if !S("Hello ").ADD(F(1.5)).EQ(S("Hello 1.5")).Bool() {
		t.Error("operator failed")
	}
}

func TestMathDIV(t *testing.T) {
	if !I(1).DIV(I(1)).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !I(4).DIV(I(2)).EQ(I(2)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).DIV(F(1)).EQ(F(1)).Bool() {
		t.Error("operator failed")
	}
	if !F(4).DIV(F(2)).EQ(F(2)).Bool() {
		t.Error("operator failed")
	}

	if I(1).DIV(S("1")) != nil {
		t.Error("operator failed")
	}
	if F(1).DIV(S("1")) != nil {
		t.Error("operator failed")
	}
}

func TestMathMUL(t *testing.T) {
	if !I(1).MUL(I(1)).EQ(I(1)).Bool() {
		t.Error("operator failed")
	}
	if !I(2).MUL(I(2)).EQ(I(4)).Bool() {
		t.Error("operator failed")
	}
	if !F(2).MUL(F(2)).EQ(F(4)).Bool() {
		t.Error("operator failed")
	}
	if !F(2).MUL(I(2)).EQ(F(4)).Bool() {
		t.Error("operator failed")
	}

	if S("1").MUL(S("2")) != nil {
		t.Error("operator failed")
	}

	if !S("1").MUL(I(2)).EQ(S("11")).Bool() {
		t.Error("operator failed")
	}
	if !S("F1").MUL(I(2)).EQ(S("F1F1")).Bool() {
		t.Error("operator failed")
	}
	if !S("F1").MUL(F(2)).EQ(S("F1F1")).Bool() {
		t.Error("operator failed")
	}
}

func TestMathSUB(t *testing.T) {
	if !I(1).SUB(I(1)).EQ(I(0)).Bool() {
		t.Error("operator failed")
	}

	if !F(1).SUB(I(1)).EQ(I(0)).Bool() {
		t.Error("operator failed")
	}
	if !F(1).SUB(F(1)).EQ(F(0)).Bool() {
		t.Error("operator failed")
	}

	if !S("S1TT").SUB(S("TT")).EQ(S("S1")).Bool() {
		t.Error("operator failed")
	}

	if S("S1TT").SUB(S("TTT")).EQ(S("S1")).Bool() {
		t.Error("operator failed")
	}

	if S("S1TT").SUB(DD("01/01/2000")) != nil {
		t.Error("operator failed")
	}

	if !DD("07/27/2000").SUB(DD("07/20/2000")).EQ(I(7)).Bool() {
		t.Error("operator failed")
	}
	if !DD("2000/7/27").SUB(DD("07/20/2000")).EQ(I(7)).Bool() {
		t.Error("operator failed")
	}
	if !DDT("2000/7/27").SUB(DDT("07/20/2000")).EQ(I(7)).Bool() {
		t.Error("operator failed")
	}

	if !DD("07/20/2000").SUB(DD("07/27/2000")).EQ(I(-7)).Bool() {
		t.Error("operator failed")
	}
}

// + - * /: a double on either side makes the result a double; two integers give
// the left operand's type, the right converted to it, and nil when it does not
// fit (C++'s numeric_cast throws there). Decided 2026-10-22, rules_benchmark
// Phase 4: int + double and int - double used to truncate to an int on both
// engines, int * double on Go, and Go had no uint case.
func TestArithmeticTyping(t *testing.T) {
	add := func(a, b *Node) *Node { return a.ADD(b) }
	sub := func(a, b *Node) *Node { return a.SUB(b) }
	mul := func(a, b *Node) *Node { return a.MUL(b) }
	div := func(a, b *Node) *Node { return a.DIV(b) }
	cases := []struct {
		name string
		op   func(a, b *Node) *Node
		a, b *Node
		want *Node // nil: no result
	}{
		{"int + double", add, I(5), F(0.5), F(5.5)},
		{"double + int", add, F(0.5), I(5), F(5.5)},
		{"int - double", sub, I(5), F(0.5), F(4.5)},
		{"double - int", sub, F(0.5), I(5), F(-4.5)},
		{"int * double", mul, I(5), F(0.5), F(2.5)},
		{"double * int", mul, F(0.5), I(5), F(2.5)},
		{"int / double", div, I(5), F(2), F(2.5)},
		{"int / int", div, I(5), I(2), I(2)},
		{"int + int", add, I(5), I(2), I(7)},
		{"int + uint", add, I(5), UI(3), I(8)},
		{"uint + int", add, UI(5), I(3), UI(8)},
		{"uint + uint", add, UI(5), UI(3), UI(8)},
		{"uint - int", sub, UI(5), I(3), UI(2)},
		{"uint * int", mul, UI(5), I(3), UI(15)},
		{"uint / int", div, UI(7), I(2), UI(3)},
		{"uint * double", mul, UI(5), F(0.5), F(2.5)},
		{"uint + negative int", add, UI(5), I(-1), nil},
		{"int + uint past int", add, I(5), UI(math.MaxUint64), nil},
		{"int + text", add, I(5), S("x"), nil},
		{"date + int", add, DD("2025-01-01"), I(1), DD("2025-01-02")},
	}
	for _, c := range cases {
		got := c.op(c.a, c.b)
		switch {
		case c.want == nil:
			if got != nil {
				t.Errorf("%s: %v %v gives %v (%s), want no result", c.name, c.a, c.b, got, got.GetTypeName())
			}
		case got == nil:
			t.Errorf("%s: %v %v gives no result, want %v", c.name, c.a, c.b, c.want)
		case got.GetType() != c.want.GetType() || TextOf(got) != TextOf(c.want):
			t.Errorf("%s: %v %v gives %v (%s), want %v (%s)", c.name, c.a, c.b,
				got, got.GetTypeName(), c.want, c.want.GetTypeName())
		}
	}
}

// DivisionByZeroCases is the shared table of rules_benchmark Q-69: integer
// division by zero is null; a double operand divides as IEEE doubles do. The
// C++ engine's test checks the same rows. Text is the result's TextOf, so
// "+Inf"-style spellings are the I-16 ones.
var DivisionByZeroCases = []struct {
	Name     string
	Lhs, Rhs *Node
	Want     string // "null", or the double's text
}{
	{"int 5 / int 0", I(5), I(0), "null"},
	{"uint 5 / uint 0", UI(5), UI(0), "null"},
	{"int 0 / int 0", I(0), I(0), "null"},
	{"double 5 / double 0", F(5), F(0), "inf"},
	{"double -5 / double 0", F(-5), F(0), "-inf"},
	{"double 0 / double 0", F(0), F(0), "nan"},
	{"int 5 / double 0", I(5), F(0), "inf"},
}

func TestDivisionByZero(t *testing.T) {
	for _, c := range DivisionByZeroCases {
		got := c.Lhs.DIV(c.Rhs)
		var text string
		switch {
		case got == nil:
			text = "<nil>"
		case got.IsNull():
			text = "null"
		default:
			if _, ok := got.Value.(float64); !ok {
				t.Errorf("%s = %v (%s), want a double", c.Name, got, got.GetTypeName())
				continue
			}
			text = TextOf(got)
		}
		if text != c.Want {
			t.Errorf("%s = %s, want %s", c.Name, text, c.Want)
		}
	}
}
