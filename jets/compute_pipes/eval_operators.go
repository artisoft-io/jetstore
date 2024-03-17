package compute_pipes

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

// build the runtime evaluator for the column transformation
func (ctx *BuilderContext) buildEvalOperator(op string) (evalOperator, error) {

	switch strings.ToUpper(op) {
	// Boolean operators
	case "==":
		return opEqual{}, nil
	case "!=":
		return opNotEqual{}, nil
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
	case "NOT":	// unary op
		return opNot{}, nil
	// Arithemtic operators
	case "/":
		return opDIV{}, nil
	case "+":
		return opADD{}, nil
	case "-":
		return opSUB{}, nil
	case "*":
		return opMUL{}, nil
	case "ABS":
		return opABS{}, nil
	// Special Operators
	case "DISTANCE_MONTHS":
		return opDMonths{}, nil
	case "APPLY_FORMAT":
		return opApplyFormat{}, nil
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
	}
	return 0, fmt.Errorf("invalid data: not a double: %v", d)
}

type opEqual struct {}
func (op opEqual) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
			if nearlyEqual(v, rhsv) {
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
			if nearlyEqual(float64(lhsv), rhsv) {
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
			if nearlyEqual(float64(lhsv), rhsv) {
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
			if nearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int:
			if nearlyEqual(lhsv, float64(rhsv)) {
				return 1, nil
			}
			return 0, nil
		case int64:
			if nearlyEqual(lhsv, float64(rhsv)) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if nearlyEqual(lhsv, rhsv) {
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
type opNotEqual struct {}
func (op opNotEqual) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return 0, nil
	}
	v, err := opEqual{}.eval(lhs, rhs)
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


// Boolean not
type opNot struct {}
func (op opNot) eval(lhs interface{}, _ interface{}) (interface{}, error) {
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


type opIS struct {}
func (op opIS) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil && rhs == nil {
		return 1, nil
	}
	switch lhsv := lhs.(type) {
	case float64:
		switch rhsv := rhs.(type) {
		case float64:
			if(math.IsNaN(lhsv) && math.IsNaN(rhsv)) {
				return 1, nil
			}
		}
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


type opLE struct {}
func (op opLE) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
			if v < rhsv || nearlyEqual(v, rhsv) {
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
			if v < rhsv || nearlyEqual(v, rhsv) {
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
			if v < rhsv || nearlyEqual(v, rhsv) {
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
			if v < lhsv || nearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int:
			v := float64(rhsv)
			if lhsv < v || nearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int64:
			v := float64(rhsv)
			if lhsv < v || nearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv <= rhsv || nearlyEqual(lhsv, rhsv) {
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


type opGT struct {}
func (op opGT) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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


type opGE struct {}
func (op opGE) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
			if v > rhsv || nearlyEqual(v, rhsv) {
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
			if v > rhsv || nearlyEqual(v, rhsv) {
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
			if v > rhsv || nearlyEqual(v, rhsv) {
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
			if lhsv > v || nearlyEqual(v, lhsv) {
				return 1, nil
			}
			return 0, nil
		case int:
			v := float64(rhsv)
			if lhsv > v || nearlyEqual(lhsv, v) {
				return 1, nil
			}
			return 0, nil
		case int64:
			v := float64(rhsv)
			if lhsv > v || nearlyEqual(lhsv, v) {
				return 1, nil
			}
			return 0, nil

		case float64:
			if lhsv > rhsv || nearlyEqual(lhsv, rhsv) {
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


type opDIV struct {}
func (op opDIV) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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


type opADD struct {}
func (op opADD) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	switch lhsv := lhs.(type) {
	case string:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%s%s", lhsv, rhsv), nil
		case int:
			return fmt.Sprintf("%s%v",lhsv,rhsv), nil
		case int64:
			return fmt.Sprintf("%s%v",lhsv,rhsv), nil
		case float64:
			return fmt.Sprintf("%s%v",lhsv,rhsv), nil
		}
	
	case int:
		switch rhsv := rhs.(type) {
		case string:
			return fmt.Sprintf("%v%v",lhsv,rhsv), nil
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
			return fmt.Sprintf("%v%v",lhsv,rhsv), nil
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
			return fmt.Sprintf("%v%v",lhsv,rhsv), nil
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
			d, err := time.ParseDuration(fmt.Sprintf("%dh", rhsv * 24))
			if err != nil {
				log.Printf("opADD: while adding time with int (assuming adding days to a date): %v", err)
			}
			return lhsv.Add(d), nil
		}
	}
	return nil, fmt.Errorf("opADD incompatible types: '%v' and '%v', rejected", lhs, rhs)
}


type opSUB struct {}
func (op opSUB) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
			d, err := time.ParseDuration(fmt.Sprintf("%dh", rhsv * 24))
			if err != nil {
				log.Printf("opSUB: while substracting time with int (assuming subtracting days to a date): %v", err)
			}
			return lhsv.Add(-d), nil

		case time.Time:
			// Assuming Substracting 2 dates, return the number of days as duration
			return int(lhsv.Sub(rhsv).Hours() / 24), nil
		}

	}
	return nil, fmt.Errorf("opSUB incompatible types, rejected")
}


type opMUL struct {}
func (op opMUL) eval(lhs interface{}, rhs interface{}) (interface{}, error) {
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
type opABS struct {}
func (op opABS) eval(lhs interface{}, _ interface{}) (interface{}, error) {
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

func nearlyEqual(a, b float64) bool {

	// already equal?
	if(a == b) {
			return true
	}

	diff := math.Abs(a - b)
	if a == 0.0 || b == 0.0 || diff < math.SmallestNonzeroFloat64 {
			return diff < 1e-10 * math.SmallestNonzeroFloat64
	}

	return diff / (math.Abs(a) + math.Abs(b)) < 1e-10
}
