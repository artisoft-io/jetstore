package compute_pipes

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// build the runtime evaluator for the column transformation
func BuildEvalOperator(op string) (evalOperator, error) {

	switch strings.ToUpper(op) {
	// Boolean operators
	case "==":
		return &opEqual{}, nil
	case "!=":
		return &opNotEqual{}, nil
	case "IS":
		return &opIS{}, nil
	case "<":
		return &opLT{}, nil
	case "<=":
		return &opLE{}, nil
	case ">":
		return &opGT{}, nil
	case ">=":
		return &opGE{}, nil
	case "AND":
		return &opAND{}, nil
	case "OR":
		return &opOR{}, nil
	case "NOT": // unary op
		return &opNot{}, nil
	// Arithemtic operators
	case "/":
		return &opDIV{}, nil
	case "+":
		return &opADD{}, nil
	case "-":
		return &opSUB{}, nil
	case "*":
		return &opMUL{}, nil
	case "ABS":
		return &opABS{}, nil
		// Special Operators
	case "IN":
		return &opIn{}, nil
	case "LENGTH":
		return &opLength{}, nil
	case "DISTANCE_MONTHS":
		return &opDMonths{}, nil
	case "APPLY_FORMAT":
		return &opApplyFormat{}, nil
	case "APPLY_REGEX":
		return &opApplyRegex{}, nil
	}
	return nil, fmt.Errorf("error: unknown operator: %v", op)
}

func ToBool(b interface{}) bool {
	switch v := b.(type) {
	case string:
		if strings.ToUpper(v) == "TRUE" {
			return true
		}
	case int:
		return v > 0
	case int64:
		return v > 0
	case float64:
		return v > 0
	case float32:
		return v > 0
	}
	return false
}

func ToDouble(d interface{}) (float64, error) {
	switch v := d.(type) {
	case string:
		return strconv.ParseFloat(v, 64)
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	}
	return 0, fmt.Errorf("invalid data: not a double: %v", d)
}

// Operator ==
type opEqual struct{}

func (op *opEqual) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv == strconv.Itoa(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv == fmt.Sprintf("%d", rhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opEqual string and double, string not a double")
			}
			if rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual string and datetime, rejected")
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			if strconv.Itoa(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if rdf.NearlyEqual(float64(lhsv), rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual int and datetime, rejected")
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			if fmt.Sprintf("%d", lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv == int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if rdf.NearlyEqual(float64(lhsv), rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opEqual double and string, string not a double")
			}
			if rdf.NearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int:
			if rdf.NearlyEqual(lhsv, float64(rhsv)) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if rdf.NearlyEqual(lhsv, float64(rhsv)) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if rdf.NearlyEqual(lhsv, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opEqual incompatible types, rejected")
}

// Operator !=
type opNotEqual struct{}

func (op *opNotEqual) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	v, err := (&opEqual{}).eval(lhs, rhs)
	if err != nil {
		return nil, fmt.Errorf("opNotEqual eval using opEqual: %v", err)
	}
	switch vv := v.(type) {
	case int:
		if vv == 0 {
			return 1, nil
		} else {
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opNotEqual incompatible types, rejected")
}

// Operator AND
type opAND struct{}

func (op *opAND) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			if lhsv == rhsv && lhsv == 1 {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opAND incompatible types, rejected")
}

// Operator OR
type opOR struct{}

func (op *opOR) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			if lhsv == 1 || rhsv == 1 {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opOR incompatible types, rejected")
}

// Boolean not
type opNot struct{}

func (op *opNot) eval(lhs interface{}, _ interface{}) (interface{}, error) {
	if lhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		if lhsv > 0 {
			return 0, nil
		}
		return 1, nil

	case int64:
		if lhsv > 0 {
			return 0, nil
		}
		return 1, nil

	case float64:
		if lhsv > 0 {
			return 0, nil
		}
		return 1, nil
	}

	return nil, fmt.Errorf("opNot incompatible types, rejected")
}

type opIS struct{}

func (op *opIS) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil && rhs == nil {
		return 1, nil
	}
	switch lhsv := lhs.(type) {
	case float64:
		switch rhsv := rhs.(type) {
		case float64:
			if math.IsNaN(lhsv) && math.IsNaN(rhsv) {
				return 1, nil
			}
		}
	}
	return 0, nil
}

// This cmpInt64 is not guaranteed to be stable.
// cmp(a, b) should return a negative number when a < b,
// a positive number when a > b and
// zero when a == b or
// a and b are incomparable in the sense of a strict weak ordering.
func cmpInt64(l, r int64) int {
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	default:
		return 0
	}
}
// This cmpFloat64 is not guaranteed to be stable.
// cmp(a, b) should return a negative number when a < b,
// a positive number when a > b and
// zero when a == b or
// a and b are incomparable in the sense of a strict weak ordering.
func cmpFloat64(l, r float64) int {
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	default:
		return 0
	}
}

// *TODO Migrate to this cmp function
// satisfy sort.SortFunc:
// SortFunc sorts the slice x in ascending order as determined by the cmp function.
// This sort is not guaranteed to be stable.
// cmp(a, b) should return a negative number when a < b,
// a positive number when a > b and
// zero when a == b or
// a and b are incomparable in the sense of a strict weak ordering.
// SortFunc requires that cmp is a strict weak ordering.
// See https://en.wikipedia.org/wiki/Weak_ordering#Strict_weak_orderings.
// The function should return 0 for incomparable items.
func CmpRecord(lhs any, rhs any) int {
	var err error
	if lhs == nil || rhs == nil {
		return 0
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			return strings.Compare(lhsv, rhsv)
		default:
			vv := fmt.Sprintf("%v", rhsv)
			return strings.Compare(lhsv, vv)
		}

	case int:
		var vv int64
		switch rhsv := rhs.(type) {
		case string:
			vv, err = strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return 0
			}
		case int:
			vv = int64(rhsv)
		case int64:
			vv = rhsv
		case float64:
			vv = int64(rhsv)
		case time.Time:
			vv = rhsv.Unix()
		}
		return cmpInt64(int64(lhsv), vv)

	case int64:
		var vv int64
		switch rhsv := rhs.(type) {
		case string:
			vv, err = strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return 0
			}
		case int:
			vv = int64(rhsv)
		case int64:
			vv = rhsv
		case float64:
			vv = int64(rhsv)
		case time.Time:
			vv = rhsv.Unix()
		}
		return cmpInt64(lhsv, vv)

	case float64:
		var vv float64
		switch rhsv := rhs.(type) {
		case string:
			vv, err = strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return 0
			}
		case int:
			vv = float64(rhsv)
		case int64:
			vv = float64(rhsv)
		case float64:
			vv =rhsv 
		case time.Time:
			vv = float64(rhsv.Unix())
		}
		return cmpFloat64(lhsv, vv)

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			return lhsv.Compare(rhsv)
		case int:
			return cmpInt64(lhsv.Unix(), int64(rhsv))
		case int64:
			return cmpInt64(lhsv.Unix(), rhsv)
		case float64:
			return cmpFloat64(float64(lhsv.Unix()), rhsv)
		}
	}
	return 0
}

type opLT struct{}

func (op *opLT) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			if lhsv < rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			v, err := strconv.Atoi(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and int, string not a int")
			}
			if v < rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			v, err := strconv.ParseInt(lhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opLT string and int64, string not a int64")
			}
			if v < rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opLT string and double, string not a double")
			}
			if v < rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLT string and datetime, rejected")
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.Atoi(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT int and string, string not a int")
			}
			if lhsv < v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv < rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLT int and datetime, rejected")
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opLT string and int64, string not a int64")
			}
			if lhsv < v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv < int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv < rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLT int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opLT double and string, string not a double")
			}
			if v < lhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv < float64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv < float64(rhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv < rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLT int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if lhsv.Before(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opLT incompatible types, rejected")
}

type opLE struct{}

func (op *opLE) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			if lhsv <= rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			v, err := strconv.Atoi(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and int, string not a int")
			}
			if v <= rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			v, err := strconv.ParseInt(lhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opLE string and int64, string not a int64")
			}
			if v <= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opLE string and double, string not a double")
			}
			if v < rhsv || rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE string and datetime, rejected")
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.Atoi(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE int and string, string not a int")
			}
			if lhsv <= v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv <= rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v := float64(lhsv)
			if v < rhsv || rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE int and datetime, rejected")
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opLE string and int64, string not a int64")
			}
			if lhsv <= v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv <= int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv <= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v := float64(lhsv)
			if v < rhsv || rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opLE double and string, string not a double")
			}
			if v < lhsv || rdf.NearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int:
			v := float64(rhsv)
			if lhsv < v || rdf.NearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			v := float64(rhsv)
			if lhsv < v || rdf.NearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv <= rhsv || rdf.NearlyEqual(lhsv, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if lhsv == rhsv || lhsv.Before(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opLE incompatible types, rejected")
}

type opGT struct{}

func (op *opGT) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			if lhsv > rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			v, err := strconv.Atoi(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opGT string and int, string not a int")
			}
			if v > rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			v, err := strconv.ParseInt(lhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opGT string and int64, string not a int64")
			}
			if v > rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opGT string and double, string not a double")
			}
			if v > rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGT string and datetime, rejected")
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.Atoi(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opGT int and string, string not a int")
			}
			if lhsv > v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv > rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) > rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) > rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGT int and datetime, rejected")
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opGT string and int64, string not a int64")
			}
			if lhsv > v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv > int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv > rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) > rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGT int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opGT double and string, string not a double")
			}
			if v > lhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv > float64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv > float64(rhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv > rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGT int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if lhsv.After(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opGT incompatible types, rejected")
}

type opGE struct{}

func (op *opGE) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			if lhsv >= rhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			v, err := strconv.Atoi(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opGE string and int, string not a int")
			}
			if v >= rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			v, err := strconv.ParseInt(lhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opGE string and int64, string not a int64")
			}
			if v >= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opGE string and double, string not a double")
			}
			if v > rhsv || rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE string and datetime, rejected")
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.Atoi(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opGE int and string, string not a int")
			}
			if lhsv >= v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv >= rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) >= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v := float64(lhsv)
			if v > rhsv || rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE int and datetime, rejected")
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opGE string and int64, string not a int64")
			}
			if lhsv >= v {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv >= int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv >= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v := float64(lhsv)
			if v > rhsv || rdf.NearlyEqual(v, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opGE double and string, string not a double")
			}
			if lhsv > v || rdf.NearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int:
			v := float64(rhsv)
			if lhsv > v || rdf.NearlyEqual(lhsv, v) {
				return 1, nil
			}
			return 0, nil
		case int64:
			v := float64(rhsv)
			if lhsv > v || rdf.NearlyEqual(lhsv, v) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv > rhsv || rdf.NearlyEqual(lhsv, rhsv) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			if lhsv == rhsv || lhsv.After(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opGE incompatible types, rejected")
}

type opDIV struct{}

func (op *opDIV) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	lhsv, err := ToDouble(lhs)
	if err != nil {
		return nil, err
	}
	rhsv, err := ToDouble(rhs)
	if err != nil {
		return nil, err
	}
	if math.Abs(rhsv) < 1e-10 {
		return math.NaN(), nil
	}
	return lhsv / rhsv, nil
}

type opADD struct{}

func (op *opADD) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%s%s", lhsv, rhsv), nil
		case int:
			return fmt.Sprintf("%s%v", lhsv, rhsv), nil
		case int64:
			return fmt.Sprintf("%s%v", lhsv, rhsv), nil
		case float64:
			return fmt.Sprintf("%s%v", lhsv, rhsv), nil
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%v%v", lhsv, rhsv), nil
		case int:
			return lhsv + rhsv, nil
		case int64:
			return int64(lhsv) + rhsv, nil

		case float64:
			return float64(lhsv) + rhsv, nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%v%v", lhsv, rhsv), nil
		case int:
			return lhsv + int64(rhsv), nil
		case int64:
			return lhsv + rhsv, nil

		case float64:
			return float64(lhsv) + rhsv, nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%v%v", lhsv, rhsv), nil
		case int:
			return lhsv + float64(rhsv), nil
		case int64:
			return lhsv + float64(rhsv), nil

		case float64:
			return lhsv + rhsv, nil
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case int:
			// Assuming lhs is date and rhs is days
			d := time.Duration(rhsv) * 24 * time.Hour
			// d, err := time.ParseDuration(fmt.Sprintf("%dh", rhsv * 24))
			// if err != nil {
			// 	log.Printf("opADD: while adding time with int (assuming adding days to a date): %v", err)
			// }
			return lhsv.Add(d), nil
		}
	}
	return nil, fmt.Errorf("opADD incompatible types: '%v' and '%v', rejected", lhs, rhs)
}

type opSUB struct{}

func (op *opSUB) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv - rhsv, nil
		case int64:
			return int64(lhsv) - rhsv, nil

		case float64:
			return float64(lhsv) - rhsv, nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv - int64(rhsv), nil
		case int64:
			return lhsv - rhsv, nil

		case float64:
			return float64(lhsv) - rhsv, nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv - float64(rhsv), nil
		case int64:
			return lhsv - float64(rhsv), nil

		case float64:
			return lhsv - rhsv, nil
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case int:
			// Assuming lhs is date and rhs is days
			d := time.Duration(rhsv) * 24 * time.Hour
			// d, err := time.ParseDuration(fmt.Sprintf("%dh", rhsv * 24))
			// if err != nil {
			// 	log.Printf("opSUB: while substracting time with int (assuming subtracting days to a date): %v", err)
			// }
			return lhsv.Add(-d), nil

		case time.Time:
			// Assuming Substracting 2 dates, return the number of days as duration
			return int(lhsv.Sub(rhsv).Hours() / 24), nil
		}

	}
	return nil, fmt.Errorf("opSUB incompatible types, rejected")
}

type opMUL struct{}

func (op *opMUL) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv * rhsv, nil
		case int64:
			return int64(lhsv) * rhsv, nil
		case float64:
			return float64(lhsv) * rhsv, nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv * int64(rhsv), nil
		case int64:
			return lhsv * rhsv, nil
		case float64:
			return float64(lhsv) * rhsv, nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case int:
			return lhsv * float64(rhsv), nil
		case int64:
			return lhsv * float64(rhsv), nil
		case float64:
			return lhsv * rhsv, nil
		}
	}
	return nil, fmt.Errorf("opMUL incompatible types, rejected")
}

// Operator abs()
type opABS struct{}

func (op *opABS) eval(lhs interface{}, _ interface{}) (interface{}, error) {
	if lhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case int:
		if lhsv < 0 {
			return -lhsv, nil
		}
		return lhsv, nil

	case int64:
		if lhsv < 0 {
			return -lhsv, nil
		}
		return lhsv, nil

	case float64:
		return math.Abs(lhsv), nil
	}
	return nil, fmt.Errorf("opABS incompatible types, rejected")
}
