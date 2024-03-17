package compute_pipes

import (
	"fmt"
	"strings"
	"time"
)


type opDMonths struct {}
func (op opDMonths) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {

	case time.Time:
		switch rhsv := rhs.(type) {
		case time.Time:
			v := (lhsv.Year() - rhsv.Year()) * 12 + int(lhsv.Month()) - int(rhsv.Month())
			if v > 0 {
				return v, nil
			}
			return -v, nil
		}
	}
	return nil, fmt.Errorf("opDMonths incompatible types, rejected")
}

type opApplyFormat struct {}
func (op opApplyFormat) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
