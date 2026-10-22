package rdf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Basic math operators

func (rhs *Node) ABS() *Node {
	if rhs == nil {
		return nil
	}
	switch rhsv := rhs.Value.(type) {
	case int:
		if rhsv < 0 {
			return I(-rhsv)
		}
		return rhs
	case float64:
		return F(math.Abs(rhsv))
	default:
		return nil
	}
}

// arith applies an arithmetic operator to two numbers, typed as both engines
// type it (decided 2026-10-22 in rules_benchmark Phase 4): a double on either
// side makes the result a double, both operands converted; two integers give
// the left operand's type, the right converted to it, as C++'s numeric_cast
// does, and nil when it does not fit there (a negative into a uint, a uint
// past the int range), where C++ throws. Until then int + double and
// int - double truncated the double to an int on both engines, and int * double
// on Go. ok is false when either operand is not a number.
func arith(lhs, rhs *Node, fi func(a, b int) *Node, fu func(a, b uint) *Node,
	ff func(a, b float64) *Node) (r *Node, ok bool) {
	var lf, rf float64
	lDouble, rDouble := false, false
	switch lv := lhs.Value.(type) {
	case int:
		lf = float64(lv)
	case uint:
		lf = float64(lv)
	case float64:
		lf, lDouble = lv, true
	default:
		return nil, false
	}
	switch rv := rhs.Value.(type) {
	case int:
		rf = float64(rv)
	case uint:
		rf = float64(rv)
	case float64:
		rf, rDouble = rv, true
	default:
		return nil, false
	}
	if lDouble || rDouble {
		return ff(lf, rf), true
	}
	switch lv := lhs.Value.(type) {
	case int:
		switch rv := rhs.Value.(type) {
		case int:
			return fi(lv, rv), true
		case uint:
			if rv > math.MaxInt {
				return nil, true
			}
			return fi(lv, int(rv)), true
		}
	case uint:
		switch rv := rhs.Value.(type) {
		case uint:
			return fu(lv, rv), true
		case int:
			if rv < 0 {
				return nil, true
			}
			return fu(lv, uint(rv)), true
		}
	}
	return nil, true
}

func (lhs *Node) ADD(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	if r, ok := arith(lhs, rhs,
		func(a, b int) *Node { return I(a + b) },
		func(a, b uint) *Node { return UI(a + b) },
		func(a, b float64) *Node { return F(a + b) }); ok {
		return r
	}

	switch lhsv := lhs.Value.(type) {
	case LDate:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &Node{Value: lhsv.Add(rhsv)}
		case float64:
			return &Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}

	case LDatetime:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &Node{Value: lhsv.Add(rhsv)}
		case float64:
			return &Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}

	case string:
		switch rhsv := rhs.Value.(type) {
		case string:
			return S(lhsv + rhsv)
		case int:
			return S(fmt.Sprintf("%v%v", lhsv, rhsv))
		case float64, LDate, LDatetime, RdfNull:
			// As C++ writes them: the shortest digits with no exponent
			// (rules_benchmark I-16), YYYY-MM-DD, the ISO datetime, "null" (I-32).
			return S(lhsv + TextOf(rhs))
		default:
			return S(fmt.Sprintf("%v%v", lhs, rhs))
		}
	default:
		return nil
	}
}

func (lhs *Node) DIV(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	r, _ := arith(lhs, rhs,
		func(a, b int) *Node {
			if b == 0 {
				return F(math.NaN())
			}
			return I(a / b)
		},
		func(a, b uint) *Node {
			if b == 0 {
				return F(math.NaN())
			}
			return UI(a / b)
		},
		func(a, b float64) *Node {
			if NearlyEqual(b, 0) {
				return F(math.NaN())
			}
			return F(a / b)
		})
	return r
}

func (lhs *Node) MUL(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	if r, ok := arith(lhs, rhs,
		func(a, b int) *Node { return I(a * b) },
		func(a, b uint) *Node { return UI(a * b) },
		func(a, b float64) *Node { return F(a * b) }); ok {
		return r
	}
	switch lhsv := lhs.Value.(type) {
	case string:
		switch rhsv := rhs.Value.(type) {
		case int:
			if rhsv > 0 && rhsv < 1000000 {
				return S(strings.Repeat(lhsv, rhsv))
			}
			return nil
		case float64:
			if rhsv > 0 && rhsv < 1000000 {
				return S(strings.Repeat(lhsv, int(rhsv)))
			}
			return nil
		default:
			return nil
		}
	default:
		return nil
	}
}

func (lhs *Node) SUB(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	if r, ok := arith(lhs, rhs,
		func(a, b int) *Node { return I(a - b) },
		func(a, b uint) *Node { return UI(a - b) },
		func(a, b float64) *Node { return F(a - b) }); ok {
		return r
	}
	switch lhsv := lhs.Value.(type) {
	case LDate:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &Node{Value: lhsv.Add(-rhsv)}
		case LDate:
			return I(int((*lhsv.Date).Sub(*rhsv.Date).Hours())/24)
		case float64:
			return &Node{Value: lhsv.Add(-int(rhsv))}
		default:
			return nil
		}
	
	case LDatetime:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &Node{Value: lhsv.Add(-rhsv)}
		case LDatetime:
			return I(int((*lhsv.Datetime).Sub(*rhsv.Datetime).Hours()/24))
		case float64:
			return &Node{Value: lhsv.Add(-int(rhsv))}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int:
			s, _ := strings.CutSuffix(lhsv, strconv.Itoa(rhsv))
			return S(s)
		case string:
			s, _ := strings.CutSuffix(lhsv, rhsv)
			return S(s)
		default:
			return nil
		}
	default:
		return nil
	}
}
