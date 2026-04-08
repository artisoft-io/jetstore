package utils

import (
	"fmt"
	"math"
	"strconv"
)

// Utility function for extended type conversions

func String2Int(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		// parse as float and convert to int, to handle the case where the input is "1.0" for an integer type
		fv, err2 := strconv.ParseFloat(s, 64)
		if err2 != nil {
			return 0, fmt.Errorf("error parsing %s as int value: %v, also error parsing as float: %v", s, err, err2)
		}
		// Let's makes sure that fv is a number, i.e. is not NaN or Inf, before converting to int
		if math.IsNaN(fv) || math.IsInf(fv, 0) {
			return 0, fmt.Errorf("error parsing %s as int value: %v, also error parsing as float: value is NaN or Inf",
				s, err)
		}
		return int(fv), nil
	}
	return v, nil
}

func String2UInt(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		// parse as float and convert to uint, to handle the case where the input is "1.0" for an integer type
		fv, err2 := strconv.ParseFloat(s, 64)
		if err2 != nil {
			return 0, fmt.Errorf("error parsing %s as uint value: %v, also error parsing as float: %v", s, err, err2)
		}
		// Let's makes sure that fv is a number, i.e. is not NaN or Inf, before converting to uint
		if math.IsNaN(fv) || math.IsInf(fv, 0) {
			return 0, fmt.Errorf("error parsing %s as uint value: %v, also error parsing as float: value is NaN or Inf",
				s, err)
		}
		return uint(fv), nil
	}
	return uint(v), err
}
