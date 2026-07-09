package compute_pipes

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/date_utils"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
)

// Operator distance_months() -- binary operator that calculates the distance in months between two dates.
// semantics: (from_date distance_months to_date) returns the number of months between from_date and to_date.
// May return negative value if from_date is after to_date. If either argument is null, returns null.
// Accept string or date or int (seconds since epoch) as arguments. If string, will attempt to parse as date, if fails, will return error.
type opDMonths struct{}

func (op *opDMonths) Eval(lhs any, rhs any) (any, error) {
	if lhs == nil || rhs == nil {
		return nil, nil
	}
	var lhsDate, rhsDate time.Time

	switch lhsv := lhs.(type) {
	case string:
		// convert to date if possible
		lhsv = strings.Trim(lhsv, "\"")
		d, err := rdf.ParseDate(lhsv)
		if err != nil {
			return nil, fmt.Errorf("opDMonths lhs string, string not a date: %v", err)
		}
		lhsDate = *d
	case time.Time:
		lhsDate = lhsv
	case int:
		lhsDate = time.Unix(int64(lhsv), 0)
	default:
		return nil, fmt.Errorf("opDMonths incompatible types for lhs: '%T', rejected", lhs)
	}

	switch rhsv := rhs.(type) {
	case string:
		// convert to date if possible
		rhsv = strings.Trim(rhsv, "\"")
		d, err := rdf.ParseDate(rhsv)
		if err != nil {
			return nil, fmt.Errorf("opDMonths rhs string, string not a date: %v", err)
		}
		rhsDate = *d
	case time.Time:
		rhsDate = rhsv
	case int:
		rhsDate = time.Unix(int64(rhsv), 0)
	default:
		return nil, fmt.Errorf("opDMonths incompatible types for rhs: '%T', rejected", rhs)
	}

	v := (rhsDate.Year()-lhsDate.Year())*12 + int(rhsDate.Month()) - int(lhsDate.Month())
	return v, nil
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
			case 0:
				// Convert java date format if used
				writeFormat := date_utils.FromJavaDateFormat(rhsv, false)
				return lhsv.Format(writeFormat), nil
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
var regexCache *sync.Map = new(sync.Map)

func GetCompiledRegex(pattern string) (*regexp.Regexp, error) {
	value, ok := regexCache.Load(pattern)
	if ok {
		return value.(*regexp.Regexp), nil
	}
	fmt.Println("Compiling:", pattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Printf("while compiling regex '%s': %v", pattern, err)
		return nil, fmt.Errorf("while compiling regex '%s': %v", pattern, err)
	}
	regexCache.Store(pattern, re)
	return re, nil
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
				op.re, err = GetCompiledRegex(rhsv)
				if err != nil {
					return nil, err
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
		return nil, fmt.Errorf("error: apply_regex lhs argument must be a string")
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
type opIn struct{
	noCase bool
}

func (op *opIn) Eval(lhs any, rhs any) (any, error) {
	values, ok := rhs.(map[any]bool)
	if !ok {
		return 0, fmt.Errorf("error: operator IN / IN_NO_CASE is expecting static_list as rhs argument")
	}
	if op.noCase {
		switch lhsv := lhs.(type) {
		case string:
			lhs = strings.ToUpper(lhsv)
		}
	}
	v := 0
	if values[lhs] {
		v = 1
	}
	return v, nil
}
