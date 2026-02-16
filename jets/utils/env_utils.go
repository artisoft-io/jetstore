package utils

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// Simple utilities for handling env var substitutions

// ReplaceEnvVars replaces occurrences of $VAR in the input value string
// with their corresponding values from the env map.
// It performs multiple passes (up to 5) to handle nested replacements.
func ReplaceEnvVars(value string, env map[string]any) string {
	if env == nil {
		return value
	}
	lc := 0
	for strings.Contains(value, "$") && lc < 5 {
		lc += 1
		for k, v := range env {
			switch vv := v.(type) {
			case string:
				value = strings.ReplaceAll(value, k, vv)
			case int:
				value = strings.ReplaceAll(value, k, strconv.Itoa(vv))
			case int64:
				value = strings.ReplaceAll(value, k, strconv.FormatInt(vv, 10))
			case uint64:
				value = strings.ReplaceAll(value, k, strconv.FormatUint(vv, 10))
			default:
				// for other types, use fmt.Sprintf to convert to string
				value = strings.ReplaceAll(value, k, fmt.Sprintf("%v", vv))
			}
		}
	}
	return value
}

// ParseLookbackPeriod parses a lookback period string (e.g., "6", "1:1") and returns
// the first ${PERIOD_ID} and the number of periods to look back.
// example: if lookbackStr is "6" and ${PERIOD_ID} is 100, it returns (100, 6)
// example: if lookbackStr is "0:6" and ${PERIOD_ID} is 100, it returns (100, 6)
// example: if lookbackStr is "1:1" and ${PERIOD_ID} is 100, it returns (99, 0)
// example: if lookbackStr is "1:6" and ${PERIOD_ID} is 100, it returns (99, 5)
// Note: ${PERIOD_ID_TYPE} must be provided in the env map for this function to work,
// and it should be one of: ${MONTH_PERIOD}, ${WEEK_PERIOD}, ${DAY_PERIOD}
// and these env vars should be set to the current period id for the corresponding period type
// as int values.
func ParseLookbackPeriod(lookbackStr string, env map[string]any) (int, int, error) {
	var offset, firstPeriod, numPeriods int
	var err error
	lb := strings.Split(lookbackStr, ":")
	l := len(lb)
	switch l {
	case 0:
		return 0, 0, fmt.Errorf("error: invalid lookback_period: %s", lookbackStr)

	case 1:
		// case of lookbackStr with only one value, the first period is the current period (offset of 0)
		// and the number of periods is the value in lookbackStr
		offset = 0
		numPeriods, err = strconv.Atoi(ReplaceEnvVars(lb[0], env))
		if err != nil {
			return 0, 0, fmt.Errorf("error: invalid num periods in lookback_period: %s", lookbackStr)
		}

	case 2:
		// case of lookbackStr with two values, the first value is the offset to apply to the current period to get the first period
		// and the second value is the number of periods to look back from the *current* period
		offset, err = strconv.Atoi(ReplaceEnvVars(lb[0], env))
		if err != nil {
			return 0, 0, fmt.Errorf("error: invalid offset in lookback_period: %s: %v", lookbackStr, err)
		}
		lastPeriods, err := strconv.Atoi(ReplaceEnvVars(lb[1], env))
		if err != nil {
			return 0, 0, fmt.Errorf("error: invalid num periods in lookback_period: %s: %v", lookbackStr, err)
		}
		numPeriods = lastPeriods - offset
		if numPeriods < 0 {
			return 0, 0, fmt.Errorf("error: num periods in lookback_period cannot be negative: %s", lookbackStr)
		}

	default:
		return 0, 0, fmt.Errorf("error: invalid lookback_period format (too many colons): %s", lookbackStr)
	}

	per0 := ReplaceEnvVars("${PERIOD_ID_TYPE}", env)
	if per0 == "" {
		return 0, 0, fmt.Errorf("error: missing ${PERIOD_ID_TYPE} in env for lookback_period: %s", lookbackStr)
	}
	firstPeriod, err = strconv.Atoi(per0)
	if err != nil {
		return 0, 0, fmt.Errorf("error: invalid ${PERIOD_ID_TYPE} value in env for lookback_period: %s", lookbackStr)
	}
	log.Printf("### ParseLookbackPeriod: lookbackStr=%s, offset=%d, numPeriods=%d, firstPeriod=%d", lookbackStr, offset, numPeriods, firstPeriod)
	return firstPeriod - offset, numPeriods, nil
}
