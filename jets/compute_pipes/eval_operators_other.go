package compute_pipes

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type opDMonths struct{}

func (op *opDMonths) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			v := (lhsv.Year()-rhsv.Year())*12 + int(lhsv.Month()) - int(rhsv.Month())
			if v > 0 {
				return v, nil
			}
			return -v, nil
		}
	}
	return nil, fmt.Errorf("opDMonths incompatible types, rejected")
}

type opApplyFormat struct{}

func (op *opApplyFormat) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case time.Time:
		switch rhsv := rhs.(type) {
		case string:
			switch strings.Count(rhsv, "%") {
			case 3:
				return fmt.Sprintf(rhsv, lhsv.Year(), int(lhsv.Month()), lhsv.Day()), nil
			default:
				return fmt.Sprintf(rhsv, lhsv.Year(), int(lhsv.Month()), lhsv.Day(),
					lhsv.Hour(), lhsv.Minute(), lhsv.Second(), lhsv.Nanosecond()), nil
			}
		default:
			return nil, fmt.Errorf("error: apply_format rhs argument must be a format string")
		}
	default:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf(rhsv, lhsv), nil
		default:
			return nil, fmt.Errorf("error: apply_format rhs argument must be a format string")
		}
	}
}

type opApplyRegex struct {
	re *regexp.Regexp
}

func (op *opApplyRegex) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	var err error
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			if op.re == nil {
				// fmt.Println("Compiling:", rhsv)
				op.re, err = regexp.Compile(rhsv)
				if err != nil {
					return nil, fmt.Errorf("while compiling regex '%s': %v", rhsv, err)
				}
			}
			// fmt.Println("apply regex on:", lhsv)
			vv := op.re.FindString(lhsv)
			if len(vv) == 0 {
				return nil, nil
			} else {
				return vv, nil
			}
		default:
			return nil, fmt.Errorf("error: apply_regex rhs argument must be a regular expression as a string")
		}
	default:
		return nil, fmt.Errorf("error: apply_format lhs argument must be a string")
	}
}

type opFindAndReplace struct {
}

// opFindAndReplace implements a find and replace operator that replaces all occurrences of a substring with another substring.
// The lhs argumwent is the input string, the rhs is expected to be []any with 2 elements: the first element is the substring to find, 
// the second element is the substring to replace with.
func (op *opFindAndReplace) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	rhsArr, ok := rhs.([]any)
	if !ok || len(rhsArr) != 2 {
		return nil, fmt.Errorf("error: find_and_replace rhs argument must be an array of 2 elements: [find, replace]")
	}
	find, ok1 := rhsArr[0].(string)
	replace, ok2 := rhsArr[1].(string)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("error: find_and_replace rhs argument must be an array of 2 strings: [find, replace]")
	}
	value, ok := lhs.(string)
	if !ok {
		return nil, fmt.Errorf("error: find_and_replace lhs argument must be a string got %T", lhs)
	}
	return strings.ReplaceAll(value, find, replace), nil
}

type opToArray struct {
}

// opToArray is a binary operator that retuns []any being the slice of lhs with rhs.
// This can be approximate as []any{lha, rhs}, however is lhs and/or rhs are themselves []any, we want to flatten them into a single array. 
// For example, if lhs is []any{1, 2} and rhs is 3, we want to return []any{1, 2, 3}. 
func (op *opToArray) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	var result []any
	switch lhsv := lhs.(type) {
	case []any:
		result = lhsv
	default:
		result = append(result, lhsv)
	}
	switch rhsv := rhs.(type) {
	case []any:
		result = append(result, rhsv...)
	default:
		result = append(result, rhsv)
	}
	return result, nil
}

// Operator length() -- unary operator
type opLength struct{}

func (op *opLength) Eval(lhs any, _ any) (any, error) {
	if lhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		return len(lhsv), nil
	}
	return nil, fmt.Errorf("opLength expecting string argument, rejected")
}

// Operator NEW_UUID -- unary operator
type opNewUUID struct{}

func (op *opNewUUID) Eval(_ any, _ any) (any, error) {
	return uuid.NewString(), nil
}

// Operator IN
type opIn struct{}

func (op *opIn) Eval(lhs any, rhs any) (any, error) {
	values, ok := rhs.(map[any]bool)
	if !ok {
		return 0, fmt.Errorf("error: operator IN is expecting static_list as rhs argument")
	}
	v := 0
	if values[lhs] {
		v = 1
	}
	return v, nil
}
