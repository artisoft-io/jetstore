package rdf

// Defines basic operators using *Node as arguments
// Semantic is lhs.op(rhs) returns *Node
// returns nil if lhs and rhs are not a valid combination for op

func (lhs *Node) AND(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return FALSE()
	}
	if lhs.Bool() && rhs.Bool() {
		return TRUE()
	}
	return FALSE()
}

func (lhs *Node) OR(rhs *Node) *Node {
	if lhs.Bool() || rhs.Bool() {
		return TRUE()
	}
	return FALSE()
}

func (lhs *Node) NOT() *Node {
	if lhs == nil {
		return FALSE()
	}
  return B(!lhs.Bool())
}

func (lhs *Node) EQ(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return FALSE()
	}
	// Numbers: by value, a double operand as doubles (compareNumbers, Q-68)
	if c, ordered, numeric := compareNumbers(lhs, rhs); numeric {
		return B(ordered && c == 0)
	}
  if lhs == rhs {
    return TRUE()
  }
	switch lhsv := lhs.Value.(type) {
	case string:
		rhsv, ok := rhs.Value.(string)
		if ok && lhsv == rhsv {
			return TRUE()
		}
		return FALSE()
	case BlankNode:
		rhsv, ok := rhs.Value.(BlankNode)
		if ok && lhsv.Key == rhsv.Key {
			return TRUE()
		}
		return FALSE()
	case NamedResource:
		rhsv, ok := rhs.Value.(NamedResource)
		if ok && lhsv.Name == rhsv.Name {
			return TRUE()
		}
		return FALSE()
	case LDate:
		rhsv, ok := rhs.Value.(LDate)
		if ok && lhsv.Date.Equal(*rhsv.Date) {
			return TRUE()
		}
		return FALSE()
	case LDatetime:
		rhsv, ok := rhs.Value.(LDatetime)
		if ok && lhsv.Datetime.Equal(*rhsv.Datetime) {
			return TRUE()
		}
		return FALSE()
	default:
		return FALSE()
	}
}

func (lhs *Node) NE(rhs *Node) *Node {
  return lhs.EQ(rhs).NOT()
}

func (lhs *Node) GE(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return FALSE()
	}
	// Numbers: by value, a double operand as doubles (compareNumbers, Q-68)
	if c, ordered, numeric := compareNumbers(lhs, rhs); numeric {
		return B(ordered && c >= 0)
	}
	switch lhsv := lhs.Value.(type) {
	case string:
		rhsv, ok := rhs.Value.(string)
		if ok && lhsv >= rhsv {
			return TRUE()
		}
		return FALSE()
	case BlankNode:
		rhsv, ok := rhs.Value.(BlankNode)
		if ok && lhsv.Key >= rhsv.Key {
			return TRUE()
		}
		return FALSE()
	case NamedResource:
		rhsv, ok := rhs.Value.(NamedResource)
		if ok && lhsv.Name >= rhsv.Name {
			return TRUE()
		}
		return FALSE()
	case LDate:
		rhsv, ok := rhs.Value.(LDate)
		if ok && !lhsv.Date.Before(*rhsv.Date) {
			return TRUE()
		}
		return FALSE()
	case LDatetime:
		rhsv, ok := rhs.Value.(LDatetime)
		if ok && !lhsv.Datetime.Before(*rhsv.Datetime) {
			return TRUE()
		}
		return FALSE()
	default:
		return FALSE()
	}
}

func (lhs *Node) GT(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return FALSE()
	}
	// Numbers: by value, a double operand as doubles (compareNumbers, Q-68)
	if c, ordered, numeric := compareNumbers(lhs, rhs); numeric {
		return B(ordered && c > 0)
	}
	switch lhsv := lhs.Value.(type) {
	case string:
		rhsv, ok := rhs.Value.(string)
		if ok && lhsv > rhsv {
			return TRUE()
		}
		return FALSE()
	case BlankNode:
		rhsv, ok := rhs.Value.(BlankNode)
		if ok && lhsv.Key > rhsv.Key {
			return TRUE()
		}
		return FALSE()
	case NamedResource:
		rhsv, ok := rhs.Value.(NamedResource)
		if ok && lhsv.Name > rhsv.Name {
			return TRUE()
		}
		return FALSE()
	case LDate:
		rhsv, ok := rhs.Value.(LDate)
		if ok && lhsv.Date.After(*rhsv.Date) {
			return TRUE()
		}
		return FALSE()
	case LDatetime:
		rhsv, ok := rhs.Value.(LDatetime)
		if ok && lhsv.Datetime.After(*rhsv.Datetime) {
			return TRUE()
		}
		return FALSE()
	default:
		return FALSE()
	}
}

func (lhs *Node) LE(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return FALSE()
	}
	// Numbers: by value, a double operand as doubles (compareNumbers, Q-68)
	if c, ordered, numeric := compareNumbers(lhs, rhs); numeric {
		return B(ordered && c <= 0)
	}
	switch lhsv := lhs.Value.(type) {
	case string:
		rhsv, ok := rhs.Value.(string)
		if ok && lhsv <= rhsv {
			return TRUE()
		}
		return FALSE()
	case BlankNode:
		rhsv, ok := rhs.Value.(BlankNode)
		if ok && lhsv.Key <= rhsv.Key {
			return TRUE()
		}
		return FALSE()
	case NamedResource:
		rhsv, ok := rhs.Value.(NamedResource)
		if ok && lhsv.Name <= rhsv.Name {
			return TRUE()
		}
		return FALSE()
	case LDate:
		rhsv, ok := rhs.Value.(LDate)
		if ok && !lhsv.Date.After(*rhsv.Date) {
			return TRUE()
		}
		return FALSE()
	case LDatetime:
		rhsv, ok := rhs.Value.(LDatetime)
		if ok && !lhsv.Datetime.After(*rhsv.Datetime) {
			return TRUE()
		}
		return FALSE()
	default:
		return FALSE()
	}
}

func (lhs *Node) LT(rhs *Node) *Node {
	if lhs == nil || rhs == nil {
		return FALSE()
	}
	// Numbers: by value, a double operand as doubles (compareNumbers, Q-68)
	if c, ordered, numeric := compareNumbers(lhs, rhs); numeric {
		return B(ordered && c < 0)
	}
	switch lhsv := lhs.Value.(type) {
	case string:
		rhsv, ok := rhs.Value.(string)
		if ok && lhsv < rhsv {
			return TRUE()
		}
		return FALSE()
	case BlankNode:
		rhsv, ok := rhs.Value.(BlankNode)
		if ok && lhsv.Key < rhsv.Key {
			return TRUE()
		}
		return FALSE()
	case NamedResource:
		rhsv, ok := rhs.Value.(NamedResource)
		if ok && lhsv.Name < rhsv.Name {
			return TRUE()
		}
		return FALSE()
	case LDate:
		rhsv, ok := rhs.Value.(LDate)
		if ok && lhsv.Date.Before(*rhsv.Date) {
			return TRUE()
		}
		return FALSE()
	case LDatetime:
		rhsv, ok := rhs.Value.(LDatetime)
		if ok && lhsv.Datetime.Before(*rhsv.Datetime) {
			return TRUE()
		}
		return FALSE()
	default:
		return FALSE()
	}
}
