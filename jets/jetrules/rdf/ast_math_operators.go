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

func (lhs *Node) ADD(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
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

	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			return I(lhsv + rhsv)
		case float64:
			return I(lhsv + int(rhsv))
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			return F(lhsv + float64(rhsv))
		case float64:
			return F(lhsv + rhsv)
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case string:
			return S(lhsv + rhsv)
		case int:
			return S(fmt.Sprintf("%v%v", lhsv, rhsv))
		case float64:
			return S(fmt.Sprintf("%v%v", lhsv, rhsv))
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
	switch lhsv := lhs.Value.(type) {
	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			if rhsv == 0 {
				return F(math.NaN())
			}
			return I(lhsv / rhsv)
		case float64:
			if NearlyEqual(rhsv, 0) {
				return F(math.NaN())
			}
			return F(float64(lhsv) / rhsv)
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			if rhsv == 0 {
				return F(math.NaN())
			}
			return F(lhsv / float64(rhsv))
		case float64:
			if NearlyEqual(rhsv, 0) {
				return F(math.NaN())
			}
			return F(lhsv / rhsv)
		default:
			return nil
		}
	default:
		return nil
	}
}

func (lhs *Node) MUL(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	switch lhsv := lhs.Value.(type) {
	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			return I(lhsv * rhsv)
		case float64:
			return I(lhsv * int(rhsv))
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			return F(lhsv * float64(rhsv))
		case float64:
			return F(lhsv * rhsv)
		default:
			return nil
		}
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

	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			return I(lhsv - rhsv)
		case float64:
			return I(lhsv - int(rhsv))
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			return F(lhsv - float64(rhsv))
		case float64:
			return F(lhsv - rhsv)
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
