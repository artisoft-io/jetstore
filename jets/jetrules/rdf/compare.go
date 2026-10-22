package rdf

import (
	"math"
	"strconv"
)

// How ==, !=, <, <=, > and >= compare two numbers, the same in both engines
// (rules_benchmark Q-68, decided 2026-10-22):
//   - integers with integers, int against uint included, by value, as C++'s
//     std::cmp_equal and std::cmp_less do: int 5 equals uint 5;
//   - any double operand compares as doubles, both operands converted, by the
//     C++ engine's double comparison: each value rounded to 15 significant
//     digits (round_to_digits), then Knuth's fcmp at machine epsilon
//     (jets/rdf/rdf_ast.h, jets/rdf/other/fcmp.c). Go used a relative 1e-10
//     tolerance, and C++ cast the double to the integer's type, so 5 == 5.5
//     was true there;
//   - a NaN or an infinity is never equal and never ordered, as C++'s is_eq
//     and is_gt have it.

// compareNumbers compares two numbers. numeric is false when either is not a
// number; ordered is false when a double operand is a NaN or an infinity.
func compareNumbers(lhs, rhs *Node) (c int, ordered, numeric bool) {
	switch lv := lhs.Value.(type) {
	case int:
		switch rv := rhs.Value.(type) {
		case int:
			return cmpInt(lv, rv), true, true
		case uint:
			if lv < 0 {
				return -1, true, true
			}
			return cmpUint(uint(lv), rv), true, true
		case float64:
			c, ordered = compareDoubles(float64(lv), rv)
			return c, ordered, true
		}
	case uint:
		switch rv := rhs.Value.(type) {
		case int:
			if rv < 0 {
				return 1, true, true
			}
			return cmpUint(lv, uint(rv)), true, true
		case uint:
			return cmpUint(lv, rv), true, true
		case float64:
			c, ordered = compareDoubles(float64(lv), rv)
			return c, ordered, true
		}
	case float64:
		switch rv := rhs.Value.(type) {
		case int:
			c, ordered = compareDoubles(lv, float64(rv))
			return c, ordered, true
		case uint:
			c, ordered = compareDoubles(lv, float64(rv))
			return c, ordered, true
		case float64:
			c, ordered = compareDoubles(lv, rv)
			return c, ordered, true
		}
	}
	return 0, false, false
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpUint(a, b uint) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compareDoubles is C++'s is_eq / is_gt / is_lt: fcmp of the two values
// rounded to 15 significant digits, unordered when either is not finite. Two
// fast paths give the same answer without rounding, the same in both engines:
// equal doubles are equal, and doubles further apart than 2e-14 of the larger
// keep their plain order. Rounding to 15 digits moves a value by at most half
// a step of 1e-14 of it and is monotonic, so two values that far apart round
// to values still more than fcmp's epsilon apart, in the same order.
// TestCompareDoublesFastPath checks it against compareDoublesRounded.
func compareDoubles(a, b float64) (c int, ordered bool) {
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return 0, false
	}
	if a == b {
		return 0, true
	}
	if math.Abs(a-b) > 2e-14*math.Max(math.Abs(a), math.Abs(b)) {
		if a < b {
			return -1, true
		}
		return 1, true
	}
	return compareDoublesRounded(a, b), true
}

// compareDoublesRounded is the rule itself, for finite a and b.
func compareDoublesRounded(a, b float64) int {
	return fcmp(roundToDigits(a, 15), roundToDigits(b, 15), dblEpsilon)
}

// dblEpsilon is C's DBL_EPSILON.
var dblEpsilon = math.Nextafter(1, 2) - 1

// roundToDigits is C++'s round_to_digits (jets/rdf/rdf_ast.h):
// round(value * factor) / factor with factor = pow(10, digits -
// ceil(log10(fabs(value)))), with glibc's log10 (glibcLog10) and glibc pow's
// powers of ten (pow10Table). Go's math.Log10, and a correctly rounded log10,
// both differ from glibc's by an ulp near a power of ten, and an ulp above an
// integer moves the ceiling, so factor, by ten.
func roundToDigits(value float64, digits int) float64 {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	factor := pow10(digits - int(math.Ceil(glibcLog10(math.Abs(value)))))
	if math.IsInf(factor, 0) {
		// Below about 1e-294 the factor overflows, and the rounded value would be
		// NaN, which fcmp calls equal to anything: the value is kept unrounded,
		// in both engines (until rules_benchmark Phase 4, 1e-300 == 5 was true)
		return value
	}
	return math.Round(value*factor) / factor
}

// glibcLog10 is glibc's __ieee754_log10 (sysdeps/ieee754/dbl-64/e_log10.c,
// from fdlibm), for x > 0 finite: log10(2^k * m) as k * log10(2), in a high
// and a low part, plus log(m) / ln(10). It mirrors x86-64 glibc 2.39, the
// production image's; an aarch64 build of glibc may fuse the products into an
// FMA and round differently. Within a few ulps of a power of ten glibc's log
// and Go's math.Log can still differ, a known limit.
func glibcLog10(x float64) float64 {
	const (
		two54     = 1.80143985094819840000e+16
		ivln10    = 4.34294481903251816668e-01
		log10_2hi = 3.01029995663611771306e-01
		log10_2lo = 3.69423907715893078616e-13
	)
	bits := math.Float64bits(x)
	hx := int32(bits >> 32)
	k := int32(0)
	if hx < 0x00100000 { // subnormal
		k -= 54
		x *= two54
		bits = math.Float64bits(x)
		hx = int32(bits >> 32)
	}
	k += (hx >> 20) - 1023
	i := int32(uint32(k) & 0x80000000 >> 31)
	hx = (hx & 0x000fffff) | ((0x3ff - i) << 20)
	y := float64(k + i)
	x = math.Float64frombits(uint64(uint32(hx))<<32 | bits&0xffffffff)
	// float64() on each product stops the compiler fusing it into an FMA,
	// which glibc's x86-64 build does not do
	z := float64(y*log10_2lo) + float64(ivln10*math.Log(x))
	return z + float64(y*log10_2hi)
}

// pow10Table holds glibc's pow(10, n) for n in [-400, 400]: the correctly
// rounded 10^n, parsed, except for the two exponents where x86-64 glibc 2.39's
// pow is an ulp above it (10^23 is exactly halfway between two doubles).
// Another glibc, or another architecture's, may differ.
var pow10Table = func() []float64 {
	t := make([]float64, 2*pow10Bias+1)
	for i := range t {
		t[i], _ = strconv.ParseFloat("1e"+strconv.Itoa(i-pow10Bias), 64)
	}
	t[23+pow10Bias] = math.Nextafter(t[23+pow10Bias], math.Inf(1))
	t[210+pow10Bias] = math.Nextafter(t[210+pow10Bias], math.Inf(1))
	return t
}()

const pow10Bias = 400

func pow10(n int) float64 {
	if n < -pow10Bias || n > pow10Bias {
		return math.Pow(10, float64(n))
	}
	return pow10Table[n+pow10Bias]
}

// fcmp is Knuth's floating-point comparison, as jets/rdf/other/fcmp.c has it:
// -1, 0 or 1 as x1 is below, within epsilon * 2^exponent of, or above x2.
func fcmp(x1, x2, epsilon float64) int {
	m := x2
	if math.Abs(x1) > math.Abs(x2) {
		m = x1
	}
	_, exponent := math.Frexp(m)
	delta := math.Ldexp(epsilon, exponent)
	difference := x1 - x2
	switch {
	case difference > delta:
		return 1
	case difference < -delta:
		return -1
	}
	return 0
}
