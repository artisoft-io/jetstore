package compute_pipes

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dateRe *regexp.Regexp
var datetimeRe *regexp.Regexp

func init() {
	dateRe = regexp.MustCompile(`(\d{1,4})-?\/?(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|\d{1,2})-?\/?(\d{1,4})`)
	datetimeRe = regexp.MustCompile(`(\d{1,4})-?\/?(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|\d{1,2})-?\/?(\d{1,4})[T-]?\s?(\d{1,2})?[:.]?(\d{1,2})?[:.]?(\d{1,2})?[.,]?(\d+)?\s?([+-])?(\d{1,2})?[:.]?(\d{1,2})?`)
}

func ParseDate(date string) (*time.Time, error) {
	ntok := dateRe.FindStringSubmatch(strings.ToUpper(date))
	if len(ntok) < 4 {
		return nil, fmt.Errorf("ParseDate: Argument is not a date: %s", date)
	}
	token1 := ntok[1]
	token2 := ntok[2]
	token3 := ntok[3]
	var y, m, d int
	var err error
	if len(token1) == 4 {
		if y, err = strconv.Atoi(token1); err != nil {
			return nil, fmt.Errorf("ParseDate: error parsing the year %s: %v", token1, err)
		}
		if m, err = month2Int(token2); err != nil {
			return nil, fmt.Errorf("ParseDate: error parsing the month %s: %v", token2, err)
		}
		if d, err = strconv.Atoi(token3); err != nil {
			return nil, fmt.Errorf("ParseDate: error parsing the day %s: %v", token3, err)
		}
	} else {
		if y, err = strconv.Atoi(token3); err != nil {
			return nil, fmt.Errorf("ParseDate: error parsing the year %s: %v", token3, err)
		}
		if m, err = month2Int(token1); err != nil {
			return nil, fmt.Errorf("ParseDate: error parsing the month %s: %v", token1, err)
		}
		if d, err = strconv.Atoi(token2); err != nil {
			return nil, fmt.Errorf("ParseDate: error parsing the day %s: %v", token2, err)
		}
	}
	result := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return &result, nil
}

func ParseDatetime(datetime string) (*time.Time, error) {
	ntok := datetimeRe.FindStringSubmatch(strings.ToUpper(datetime))
	if len(ntok) < 4 {
		return nil, fmt.Errorf("ParseDatetime: Argument is not a date: %s", datetime)
	}
	// Get the date portion
	token1 := ntok[1]
	token2 := ntok[2]
	token3 := ntok[3]
	var y, m, d int
	var err error
	if len(token1) == 4 {
		if y, err = strconv.Atoi(token1); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the year %s: %v", token1, err)
		}
		if m, err = month2Int(token2); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the month %s: %v", token2, err)
		}
		if d, err = strconv.Atoi(token3); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the day %s: %v", token3, err)
		}
	} else {
		if y, err = strconv.Atoi(token3); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the year %s: %v", token3, err)
		}
		if m, err = month2Int(token1); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the month %s: %v", token1, err)
		}
		if d, err = strconv.Atoi(token2); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the day %s: %v", token2, err)
		}
	}
	if len(ntok) < 5 || len(ntok[4]) == 0 {
		result := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return &result, nil	
	}

	// Get the duration portion
	var hr, min, sec, frac int
	precision := 9 // time library precision in go is nanosecond
	if len(ntok) > 4 && len(ntok[4]) > 0 {
		if hr, err = strconv.Atoi(ntok[4]); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the hours %s: %v", ntok[4], err)
		}	
	}
	if len(ntok) > 5 && len(ntok[5]) > 0 {
		if min, err = strconv.Atoi(ntok[5]); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the minutes %s: %v", ntok[5], err)
		}	
	}
	if len(ntok) > 6 && len(ntok[6]) > 0 {
		if sec, err = strconv.Atoi(ntok[6]); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the seconds %s: %v", ntok[6], err)
		}	
	}
	if len(ntok) > 7 && len(ntok[7]) > 0 {
		fracStr := ntok[7]
		if(len(fracStr) >= precision) {
			// Drop excess digits
			fracStr = fracStr[0:precision]
		}
		if frac, err = strconv.Atoi(fracStr); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the fraction %s: %v", fracStr, err)
		}	
		if len(fracStr) < precision {
			// trailing zeros get dropped from the string,
			// "1:01:01.1" would yield .000001 instead of .100000
			// the power() compensates for the missing decimal
			// places
			frac *= int(math.Pow10(precision - len(fracStr)));
		}
	}
	// timezone
	if len(ntok) > 9 && len(ntok[9]) > 0 {
		var hr_offset, min_offset int
		sign := ntok[8]
		if hr_offset, err = strconv.Atoi(ntok[9]); err != nil {
			return nil, fmt.Errorf("ParseDatetime: error parsing the timezone hours %s: %v", ntok[9], err)
		}
		if len(ntok) > 10 && len(ntok[10]) > 0 {
			if min_offset, err = strconv.Atoi(ntok[10]); err != nil {
				return nil, fmt.Errorf("ParseDatetime: error parsing the timezone minutes %s: %v", ntok[10], err)
			}
		}
		if sign == "-" {
			hr += hr_offset
			min += min_offset
		} else {
			hr -= hr_offset
			min -= min_offset
		}
	}
	// That's it
	result := time.Date(y, time.Month(m), d, hr, min, sec, frac, time.UTC)
	return &result, nil
}

func month2Int(month string) (int, error) {
	if len(month) == 3 {
		if month == "JAN" {
			return 1, nil
		}
		if month == "FEB" {
			return 2, nil
		}
		if month == "MAR" {
			return 3, nil
		}
		if month == "APR" {
			return 4, nil
		}
		if month == "MAY" {
			return 5, nil
		}
		if month == "JUN" {
			return 6, nil
		}
		if month == "JUL" {
			return 7, nil
		}
		if month == "AUG" {
			return 8, nil
		}
		if month == "SEP" {
			return 9, nil
		}
		if month == "OCT" {
			return 10, nil
		}
		if month == "NOV" {
			return 11, nil
		}
		if month == "DEC" {
			return 12, nil
		}
		return 0, fmt.Errorf("error: %s is not a month", month)
	} else {
		return strconv.Atoi(month)
	}
}
