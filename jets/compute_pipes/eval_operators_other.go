package compute_pipes

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type opDMonths struct{}

func (op *opDMonths) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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

func (op *opApplyFormat) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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

func (op *opApplyRegex) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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

// Operator length() -- unary operator
type opLength struct{}

func (op *opLength) eval(lhs interface{}, _ interface{}) (interface{}, error) {
	if lhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		return len(lhsv), nil
	}
	return nil, fmt.Errorf("opLength expecting string argument, rejected")
}

// Operator IN
type opIn struct{}

func (op *opIn) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
