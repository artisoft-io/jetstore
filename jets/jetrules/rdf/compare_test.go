package rdf

import (
	"math"
	"math/rand"
	"testing"
)

// ComparisonCases is the shared table of rules_benchmark Q-68: how both engines
// compare two numbers in ==, !=, <, <=, > and >=. The C++ engine's test copies
// it. The double rows were checked against C++'s is_eq, is_gt and is_lt
// (round_to_digits, then fcmp), compiled, on 2026-10-22.
var ComparisonCases = []struct {
	Name                   string
	Lhs, Rhs               *Node
	EQ, NE, LT, LE, GT, GE bool
}{
	{"int 5, double 5", I(5), F(5), true, false, false, true, false, true},
	{"int 5, double 5.5", I(5), F(5.5), false, true, true, true, false, false},
	{"double 5.5, int 5", F(5.5), I(5), false, true, false, false, true, true},
	{"int 5, uint 5", I(5), UI(5), true, false, false, true, false, true},
	{"int -1, uint 1", I(-1), UI(1), false, true, true, true, false, false},
	{"uint max, int max", UI(math.MaxUint64), I(math.MaxInt64), false, true, false, false, true, true},
	{"int 3, int 7", I(3), I(7), false, true, true, true, false, false},
	{"0.1 + 0.2, 0.3: equal to 15 digits", F(0.30000000000000004), F(0.3), true, false, false, true, false, true},
	{"1, 1.000000000000001: equal to 15 digits", F(1), F(1.000000000000001), true, false, false, true, false, true},
	{"1, 1.000000000001", F(1), F(1.000000000001), false, true, true, true, false, false},
	{"NaN, NaN", F(math.NaN()), F(math.NaN()), false, true, false, false, false, false},
	{"NaN, int 5", F(math.NaN()), I(5), false, true, false, false, false, false},
	{"inf, inf", F(math.Inf(1)), F(math.Inf(1)), false, true, false, false, false, false},
	{"inf, int 5", F(math.Inf(1)), I(5), false, true, false, false, false, false},
	{"int 5, text 5", I(5), S("5"), false, true, false, false, false, false},
	{"int 5, double 1e12", I(5), F(1e12), false, true, true, true, false, false},
	{"uint 5, double 5", UI(5), F(5), true, false, false, true, false, true},
	// Tiny doubles: round_to_digits used to give NaN below about 1e-294, which
	// fcmp calls equal to anything
	{"1e-300, int 5", F(1e-300), I(5), false, true, true, true, false, false},
	{"5e-324, 1e300", F(5e-324), F(1e300), false, true, true, true, false, false},
	{"1e-300, int -7", F(1e-300), I(-7), false, true, false, false, true, true},
}

func TestComparisonCases(t *testing.T) {
	for _, c := range ComparisonCases {
		for _, r := range []struct {
			op   string
			got  *Node
			want bool
		}{
			{"==", c.Lhs.EQ(c.Rhs), c.EQ},
			{"!=", c.Lhs.NE(c.Rhs), c.NE},
			{"<", c.Lhs.LT(c.Rhs), c.LT},
			{"<=", c.Lhs.LE(c.Rhs), c.LE},
			{">", c.Lhs.GT(c.Rhs), c.GT},
			{">=", c.Lhs.GE(c.Rhs), c.GE},
		} {
			if r.got.Bool() != r.want {
				t.Errorf("%s: %v %s %v = %v, want %v", c.Name, c.Lhs, r.op, c.Rhs, r.got.Bool(), r.want)
			}
		}
	}
}

// The tiny doubles through the rounding itself, which the fast path skips.
func TestRoundToDigitsTiny(t *testing.T) {
	for _, v := range []float64{1e-300, -1e-300, 5e-324, 1e-294, 1e-290} {
		if r := roundToDigits(v, 15); math.IsNaN(r) || r == 0 {
			t.Errorf("roundToDigits(%v, 15) = %v", v, r)
		}
	}
	for _, c := range []struct {
		a, b float64
		want int
	}{{1e-300, 5, -1}, {5e-324, 1e300, -1}, {1e-300, -7, 1}, {1e-300, 1e-300, 0}} {
		if got := compareDoublesRounded(c.a, c.b); got != c.want {
			t.Errorf("compareDoublesRounded(%v, %v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// The fast paths of compareDoubles give what the 15-digit rule gives, on a
// million pairs: random doubles, money-like values, powers of ten, tiny values,
// and neighbours from an ulp to past the fast path's 2e-14 bound.
func TestCompareDoublesFastPath(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 500000; i++ {
		var a float64
		switch i % 6 {
		case 0:
			a = math.Float64frombits(r.Uint64())
		case 1:
			a = r.Float64() * math.Pow(10, float64(r.Intn(40)-20))
		case 2:
			a = math.Round(r.Float64()*1e8) / 100
		case 3:
			a = math.Pow(10, float64(r.Intn(600)-300))
		case 4:
			a = r.Float64() * math.Pow(10, float64(-r.Intn(30)-290))
		case 5:
			a = float64(r.Intn(2000000) - 1000000)
		}
		if math.IsNaN(a) || math.IsInf(a, 0) {
			continue
		}
		if r.Intn(2) == 0 {
			a = -a
		}
		var b float64
		switch r.Intn(4) {
		case 0:
			b = a
			for j := r.Intn(40); j > 0; j-- {
				b = math.Nextafter(b, math.Inf(1))
			}
		case 1:
			b = a * (1 + (r.Float64()*2-1)*5e-14) // around the bound
		case 2:
			b = a * (1 + (r.Float64()*2-1)*2.1e-14)
		case 3:
			b = math.Float64frombits(r.Uint64())
		}
		if math.IsNaN(b) || math.IsInf(b, 0) {
			continue
		}
		for _, p := range [][2]float64{{a, b}, {b, a}} {
			got, ordered := compareDoubles(p[0], p[1])
			if want := compareDoublesRounded(p[0], p[1]); !ordered || got != want {
				t.Fatalf("compareDoubles(%x, %x) = %d, the rule gives %d", p[0], p[1], got, want)
			}
		}
	}
}
