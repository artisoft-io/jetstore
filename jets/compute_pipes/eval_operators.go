package compute_pipes

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/utils"
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
		return &opIS{isNot: 0}, nil
	case "IS NOT":
		return &opIS{isNot: 1}, nil
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
	case "NEW_UUID":
		return &opNewUUID{}, nil
	case "DISTANCE_MONTHS":
		return &opDMonths{}, nil
	case "APPLY_FORMAT":
		return &opApplyFormat{}, nil
	case "APPLY_REGEX":
		return &opApplyRegex{}, nil
	}
	return nil, fmt.Errorf("error: unknown operator: %v", op)
}

func ToBool(b any) bool {
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

func ToDouble(d any) (float64, error) {
	switch v := d.(type) {
	case string:
		return strconv.ParseFloat(v, 64)
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint64:
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

func (op *opEqual) Eval(lhs any, rhs any) (any, error) {
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
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual string and int, string not an int")
			}
			if lv == rhsv {
				return 1, nil
			}
			return 0, nil
		case uint:
			lv, err := utils.String2UInt(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual string and uint, string not an uint")
			}
			if lv == rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual string and int64, string not an int64")
			}
			if lv == int(rhsv) {
				return 1, nil
			}
			return 0, nil
		case uint64:
			lv, err := utils.String2UInt(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual string and uint64, string not a uint64")
			}
			if lv == uint(rhsv) {
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

		default:
			return nil, fmt.Errorf("opEqual string and %T, rejected", rhs)
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual int and string, string not an int")
			}
			if lhsv == rv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		case uint:
			if lhsv > 0 && uint(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			if lhsv > 0 && uint64(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if rdf.NearlyEqual(float64(lhsv), rhsv) {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opEqual int and %T, rejected", rhs)
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual int64 and string, string not an int64")
			}
			if lhsv == int64(rv) {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv == int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case uint:
			if lhsv > 0 && uint64(lhsv) == uint64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			if lhsv > 0 && uint64(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if rdf.NearlyEqual(float64(lhsv), rhsv) {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opEqual int64 and %T, rejected", rhs)
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

		default:
			return nil, fmt.Errorf("opEqual float64 and %T, rejected", rhs)
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case string:
			v, err := rdf.ParseDate(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opEqual datetime and string, string not a datetime")
			}
			if lhsv.Equal(*v) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			if lhsv.Equal(rhsv) {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opEqual datetime and %T, rejected", rhs)
		}
	}
	return nil, fmt.Errorf("opEqual incompatible types: %T and %T, rejected", lhs, rhs)
}

// Operator !=
type opNotEqual struct{}

func (op *opNotEqual) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	v, err := (&opEqual{}).Eval(lhs, rhs)
	if err != nil {
		return nil, fmt.Errorf("opNotEqual Eval using opEqual: %v", err)
	}
	switch vv := v.(type) {
	case int:
		if vv == 0 {
			return 1, nil
		} else {
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opNotEqual incompatible types: %T and %T, rejected", lhs, rhs)
}

// Operator AND
type opAND struct{}

func (op *opAND) Eval(lhs any, rhs any) (any, error) {
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
	return nil, fmt.Errorf("opAND incompatible types: %T and %T, rejected", lhs, rhs)
}

// Operator OR
type opOR struct{}

func (op *opOR) Eval(lhs any, rhs any) (any, error) {
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
	return nil, fmt.Errorf("opOR incompatible types: %T and %T, rejected", lhs, rhs)
}

// Boolean not
type opNot struct{}

func (op *opNot) Eval(lhs any, _ any) (any, error) {
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

	return nil, fmt.Errorf("opNot incompatible types: %T, rejected", lhs)
}

type opIS struct {
	isNot int
}

func (op *opIS) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil && rhs == nil {
		return 1 - op.isNot, nil
	}
	switch lhsv := lhs.(type) {
	case float64:
		switch rhsv := rhs.(type) {
		case float64:
			if math.IsNaN(lhsv) && math.IsNaN(rhsv) {
				return 1 - op.isNot, nil
			}
		}
	}
	return op.isNot, nil
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
			v, err := utils.String2Int(rhsv)
			if err != nil {
				return 0
			}
			vv = int64(v)
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
			v, err := utils.String2Int(rhsv)
			if err != nil {
				return 0
			}
			vv = int64(v)
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
			vv = rhsv
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

func (op *opLT) Eval(lhs any, rhs any) (any, error) {
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
			v, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and int, string not a int")
			}
			if v < rhsv {
				return 1, nil
			}
			return 0, nil
		case uint:
			v, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and uint, string not a uint")
			}
			if uint(v) < rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			v, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and int64, string not a int64")
			}
			if int64(v) < rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			v, err := utils.String2UInt(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and uint64, string not a uint64")
			}
			if uint64(v) < rhsv {
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
			v, err := rdf.ParseDate(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and datetime, string not a datetime")
			}
			if (*v).Before(rhsv) {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLT string and %T, rejected", rhs)
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			v, err := utils.String2Int(rhsv)
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
		case uint:
			if lhsv > 0 && uint(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			if lhsv > 0 && uint64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLT int and %T, rejected", rhs)
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT string and int64, string not a int64")
			}
			if lhsv < int64(v) {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv < int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case uint:
			if lhsv > 0 && uint64(lhsv) < uint64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv < rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			if lhsv > 0 && uint64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) < rhsv {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLT int64 and %T, rejected", rhs)
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

		default:
			return nil, fmt.Errorf("opLT float64 and %T, rejected", rhs)
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case string:
			v, err := rdf.ParseDate(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opLT datetime and string, string not a datetime")
			}
			if lhsv.Before(*v) {
				return 1, nil
			}
			return 0, nil
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

func (op *opLE) Eval(lhs any, rhs any) (any, error) {
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
			v, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and int, string not a int")
			}
			if v <= rhsv {
				return 1, nil
			}
			return 0, nil
		case uint:
			v, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and uint, string not a uint")
			}
			if uint(v) <= rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			v, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and int64, string not a int64")
			}
			if int64(v) <= rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			v, err := utils.String2UInt(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and uint64, string not a uint64")
			}
			if uint64(v) <= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opLE string and double, string not a double")
			}
			if v <= rhsv {
				return 1, nil
			}
			return 0, nil

		case time.Time:
			v, err := rdf.ParseDate(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and datetime, string not a datetime")
			}
			if (*v).Before(rhsv) || (*v).Equal(rhsv) {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLE string and %T, rejected", rhs)
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			v, err := utils.String2Int(rhsv)
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
		case uint:
			if lhsv > 0 && uint(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil
		case int64:
			if int64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			if lhsv > 0 && uint64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLE int and %T, rejected", rhs)
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE string and int64, string not a int64")
			}
			if lhsv <= int64(v) {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv <= int64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case uint:
			if lhsv > 0 && uint64(lhsv) <= uint64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv <= rhsv {
				return 1, nil
			}
			return 0, nil
		case uint64:
			if lhsv > 0 && uint64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil

		case float64:
			if float64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLE int64 and %T, rejected", rhs)
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opLE double and string, string not a double")
			}
			if v <= lhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv <= float64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv <= float64(rhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv <= rhsv {
				return 1, nil
			}
			return 0, nil

		default:
			return nil, fmt.Errorf("opLE float64 and %T, rejected", rhs)
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case string:
			v, err := rdf.ParseDate(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opLE datetime and string, string not a datetime")
			}
			if lhsv.Before(*v) || lhsv.Equal(*v) {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			if lhsv.Before(rhsv) || lhsv.Equal(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opLE incompatible types, rejected")
}

type opGT struct{}

func (op *opGT) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	v, err := (&opLT{}).Eval(rhs, lhs)
	if err != nil {
		return nil, fmt.Errorf("opGT Eval using opLT: %v", err)
	}
	return v, nil
}

type opGE struct{}

func (op *opGE) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	v, err := (&opLE{}).Eval(rhs, lhs)
	if err != nil {
		return nil, fmt.Errorf("opGE Eval using opLE: %v", err)
	}
	return v, nil
}

type opDIV struct{}

func (op *opDIV) Eval(lhs any, rhs any) (any, error) {
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

func (op *opADD) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%s%s", lhsv, rhsv), nil
		case int:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD string and int, string not an int")
			}
			return lv + rhsv, nil
		case uint:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD string and int, string not an int")
			}
			return uint(lv) + rhsv, nil
		case int64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD string and int64, string not an int64")
			}
			return int64(lv) + rhsv, nil
		case uint64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD string and uint64, string not an uint64")
			}
			return uint64(lv) + rhsv, nil
		case float64:
			lv, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opADD string and float64, string not a float64")
			}
			return lv + rhsv, nil
		case time.Time:
			// Assuming lhs is days and rhs is date
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD string and time, string not an int (days)")
			}
			d := time.Duration(lv) * 24 * time.Hour
			return rhsv.Add(d), nil
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD int and string, string not an int")
			}
			return lhsv + rv, nil
		case int:
			return lhsv + rhsv, nil
		case int64:
			return int64(lhsv) + rhsv, nil
		case uint:
			return lhsv + int(rhsv), nil
		case uint64:
			return uint64(lhsv) + rhsv, nil
		case float64:
			return float64(lhsv) + rhsv, nil
		case time.Time:
			// Assuming lhs is days and rhs is date
			d := time.Duration(lhsv) * 24 * time.Hour
			return rhsv.Add(d), nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD int64 and string, string not an int64")
			}
			return lhsv + int64(rv), nil
		case int:
			return lhsv + int64(rhsv), nil
		case int64:
			return lhsv + rhsv, nil
		case uint:
			return lhsv + int64(rhsv), nil
		case uint64:
			return lhsv + int64(rhsv), nil
		case float64:
			return float64(lhsv) + rhsv, nil
		case time.Time:
			// Assuming lhs is days and rhs is date
			d := time.Duration(lhsv) * 24 * time.Hour
			return rhsv.Add(d), nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opADD float64 and string, string not a float64")
			}
			return lhsv + rv, nil
		case int:
			return lhsv + float64(rhsv), nil
		case int64:
			return lhsv + float64(rhsv), nil
		case uint:
			return lhsv + float64(rhsv), nil
		case uint64:
			return lhsv + float64(rhsv), nil

		case float64:
			return lhsv + rhsv, nil
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opADD time and string, string not an time duration in days")
			}
			d := time.Duration(rv) * 24 * time.Hour
			return lhsv.Add(d), nil
		case int:
			// Assuming lhs is date and rhs is days
			d := time.Duration(rhsv) * 24 * time.Hour
			return lhsv.Add(d), nil
		}
	}
	return nil, fmt.Errorf("opADD incompatible types: '%T' and '%T', rejected", lhs, rhs)
}

type opSUB struct{}

func (op *opSUB) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case int:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB string and int, string not an int")
			}
			return lv - rhsv, nil
		case uint:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB string and uint, string not an uint")
			}
			return uint(lv) - rhsv, nil
		case int64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB string and int64, string not an int64")
			}
			return int64(lv) - rhsv, nil
		case uint64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB string and uint64, string not an uint64")
			}
			return uint64(lv) - rhsv, nil
		case float64:
			lv, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opSUB string and float64, string not a float64")
			}
			return lv - rhsv, nil
		case time.Time:
			// Assuming lhs is days and rhs is date
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB string and time, string not an int (days)")
			}
			d := time.Duration(lv) * 24 * time.Hour
			return rhsv.Add(-d), nil
		}
	case int:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB int and string, string not an int")
			}
			return lhsv - rv, nil
		case int:
			return lhsv - rhsv, nil
		case int64:
			return int64(lhsv) - rhsv, nil
		case float64:
			return float64(lhsv) - rhsv, nil
		case time.Time:
			// Assuming lhs is days and rhs is date
			d := time.Duration(lhsv) * 24 * time.Hour
			return rhsv.Add(-d), nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB int64 and string, string not an int64")
			}
			return lhsv - int64(rv), nil
		case int:
			return lhsv - int64(rhsv), nil
		case int64:
			return lhsv - rhsv, nil
		case float64:
			return float64(lhsv) - rhsv, nil
		case time.Time:
			// Assuming lhs is days and rhs is date
			d := time.Duration(lhsv) * 24 * time.Hour
			return rhsv.Add(-d), nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opSUB float64 and string, string not a float64")
			}
			return lhsv - rv, nil
		case int:
			return lhsv - float64(rhsv), nil
		case int64:
			return lhsv - float64(rhsv), nil

		case float64:
			return lhsv - rhsv, nil
		}

	case time.Time:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opSUB time and string, string not an time duration in days")
			}
			d := time.Duration(rv) * 24 * time.Hour
			return lhsv.Add(-d), nil
		case int:
			// Assuming lhs is date and rhs is days
			d := time.Duration(rhsv) * 24 * time.Hour
			return lhsv.Add(-d), nil

		case time.Time:
			// Assuming Substracting 2 dates, return the number of days as duration
			return int(lhsv.Sub(rhsv).Hours() / 24), nil
		}

	}
	return nil, fmt.Errorf("opSUB incompatible types: '%T' and '%T', rejected", lhs, rhs)
}

type opMUL struct{}

func (op *opMUL) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case int:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opMUL string and int, string not an int")
			}
			return lv * rhsv, nil
		case uint:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opMUL string and uint, string not an int")
			}
			return uint(lv) * rhsv, nil
		case int64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opMUL string and int64, string not an int64")
			}
			return int64(lv) * rhsv, nil
		case uint64:
			lv, err := utils.String2Int(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opMUL string and uint64, string not an uint64")
			}
			return uint64(lv) * rhsv, nil
		case float64:
			lv, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opMUL string and float64, string not a float64")
			}
			return lv * rhsv, nil
		}

	case int:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opMUL int and string, string not an int")
			}
			return lhsv * rv, nil
		case int:
			return lhsv * rhsv, nil
		case int64:
			return int64(lhsv) * rhsv, nil
		case float64:
			return float64(lhsv) * rhsv, nil
		}

	case int64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := utils.String2Int(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opMUL int64 and string, string not an int")
			}
			return lhsv * int64(rv), nil
		case int:
			return lhsv * int64(rhsv), nil
		case int64:
			return lhsv * rhsv, nil
		case float64:
			return float64(lhsv) * rhsv, nil
		}

	case float64:
		switch rhsv := rhs.(type) {
		case string:
			rv, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opMUL float64 and string, string not a float64")
			}
			return lhsv * rv, nil
		case int:
			return lhsv * float64(rhsv), nil
		case int64:
			return lhsv * float64(rhsv), nil
		case float64:
			return lhsv * rhsv, nil
		}
	}
	return nil, fmt.Errorf("opMUL incompatible types: '%T' and '%T', rejected", lhs, rhs)
}

// Operator abs()
type opABS struct{}

func (op *opABS) Eval(lhs any, _ any) (any, error) {
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
	return nil, fmt.Errorf("opABS incompatible types: '%T', rejected", lhs)
}
