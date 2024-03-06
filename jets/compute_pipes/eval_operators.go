package compute_pipes

import (
	"fmt"
	"strconv"
	"time"
)

// build the runtime evaluator for the column transformation
func (ctx *BuilderContext) buildEvalOperator(op string) (evalOperator, error) {

	switch op {
	// select, value, eval, map, count, distinct_count, sum, min
	case "==":
		return opEqual{}, nil
	case "IS":
		return opIS{}, nil
	case "<":
		return opLT{}, nil
	case "<=":
		return opLE{}, nil
	case ">":
		return opGT{}, nil
	case ">=":
		return opGE{}, nil
	case "/":
		return opDIV{}, nil
	case "distance_months":
		return opDMonths{}, nil

	}
	return nil, fmt.Errorf("error: unknown operator: %v", op)
}


type opEqual struct {}
func (op opEqual) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil && rhs == nil {
		return 1, nil
	}
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := lhs.(type) {
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
			if v == rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual string and datetime, rejected")
		}
	
	case int:
		switch rhsv := lhs.(type) {
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
			if float64(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual int and datetime, rejected")
		}

	case int64:
		switch rhsv := lhs.(type) {
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
			if float64(lhsv) == rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := lhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opEqual double and string, string not a double")
			}
			if v == lhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv == float64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv == float64(rhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opEqual int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := lhs.(type) {
		case time.Time:
			if lhsv == rhsv {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opEqual incompatible types, rejected")
}


type opIS struct {}
func (op opIS) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil && rhs == nil {
		return 1, nil
	}
	return 0, nil
}


type opLT struct {}
func (op opLT) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
		case time.Time:
			if lhsv.Before(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opLT incompatible types, rejected")
}


type opLE struct {}
func (op opLE) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil && rhs == nil {
		return 1, nil
	}
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := lhs.(type) {
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
			if v <= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE string and datetime, rejected")
		}
	
	case int:
		switch rhsv := lhs.(type) {
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
			if float64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE int and datetime, rejected")
		}

	case int64:
		switch rhsv := lhs.(type) {
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
			if float64(lhsv) <= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opLE int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := lhs.(type) {
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
		case time.Time:
			return nil, fmt.Errorf("opLE int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := lhs.(type) {
		case time.Time:
			if lhsv == rhsv || lhsv.Before(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opLE incompatible types, rejected")
}


type opGT struct {}
func (op opGT) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
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
		switch rhsv := lhs.(type) {
		case time.Time:
			if lhsv.After(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opGT incompatible types, rejected")
}


type opGE struct {}
func (op opGE) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil && rhs == nil {
		return 1, nil
	}
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := lhs.(type) {
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
			if v >= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE string and datetime, rejected")
		}
	
	case int:
		switch rhsv := lhs.(type) {
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
			if float64(lhsv) >= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE int and datetime, rejected")
		}

	case int64:
		switch rhsv := lhs.(type) {
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
			if float64(lhsv) >= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE int64 and datetime, rejected")
		}

	case float64:
		switch rhsv := lhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opGE double and string, string not a double")
			}
			if v >= lhsv {
				return 1, nil
			}
			return 0, nil
		case int:
			if lhsv >= float64(rhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if lhsv >= float64(rhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv >= rhsv {
				return 1, nil
			}
			return 0, nil
		case time.Time:
			return nil, fmt.Errorf("opGE int64 and datetime, rejected")
		}

	case time.Time:
		switch rhsv := lhs.(type) {
		case time.Time:
			if lhsv == rhsv || lhsv.After(rhsv) {
				return 1, nil
			}
			return 0, nil
		}
	}
	return nil, fmt.Errorf("opGE incompatible types, rejected")
}


type opDIV struct {}
func (op opDIV) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := lhs.(type) {
		case int:
			v, err := strconv.Atoi(lhsv)
			if err != nil {
				return nil, fmt.Errorf("opDIV string and int, string not a int")
			}
			return v / rhsv, nil
		case int64:
			v, err := strconv.ParseInt(lhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opDIV string and int64, string not a int64")
			}
			return v / rhsv, nil

		case float64:
			v, err := strconv.ParseFloat(lhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opDIV string and double, string not a double")
			}
			return v / rhsv, nil
		}
	
	case int:
		switch rhsv := lhs.(type) {
		case string:
			v, err := strconv.Atoi(rhsv)
			if err != nil {
				return nil, fmt.Errorf("opDIV int and string, string not a int")
			}
			return lhsv / v, nil
		case int:
			return lhsv / rhsv, nil
		case int64:
			return int64(lhsv) / rhsv, nil

		case float64:
			return float64(lhsv) / rhsv, nil
		}

	case int64:
		switch rhsv := lhs.(type) {
		case string:
			v, err := strconv.ParseInt(rhsv, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("opDIV string and int64, string not a int64")
			}
			return lhsv / v, nil
		case int:
			return lhsv / int64(rhsv), nil
		case int64:
			return lhsv / rhsv, nil

		case float64:
			return float64(lhsv) / rhsv, nil
		}

	case float64:
		switch rhsv := lhs.(type) {
		case string:
			v, err := strconv.ParseFloat(rhsv, 64)
			if err != nil {
				return nil, fmt.Errorf("opDIV double and string, string not a double")
			}
			return v / lhsv, nil
		case int:
			return lhsv / float64(rhsv), nil
		case int64:
			return lhsv / float64(rhsv), nil

		case float64:
			return lhsv / rhsv, nil
		}
	}
	return nil, fmt.Errorf("opDIV incompatible types, rejected")
}


type opDMonths struct {}
func (op opDMonths) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	switch lhsv := lhs.(type) {

	case time.Time:
		switch rhsv := lhs.(type) {
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
