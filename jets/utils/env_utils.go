package utils

import (
	"fmt"
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
			vv, ok := v.(string)
			if ok {
				value = strings.ReplaceAll(value, k, vv)
			}
		}
	}
	return value
}

// ParseLookbackPeriod parses a lookback period string (e.g., "6", "1:1") and returns
// the first ${PERIOD_ID} and the number of periods to look back.
func ParseLookbackPeriod(lookbackStr string, env map[string]any) (int, int, error) {
	lb := strings.Split(lookbackStr, ":")
	if len(lb) == 0 {
		return 0, 0, fmt.Errorf("error: invalid lookback_period: %s", lookbackStr)
	}
	firstPeriod, err := strconv.Atoi(ReplaceEnvVars(lb[0], env))
	if err != nil {
		return 0, 0, fmt.Errorf("error: invalid first period_id in lookback_period: %s", lookbackStr)
	}
	numPeriods := 0
	if len(lb) > 1 {
		numPeriods, err = strconv.Atoi(ReplaceEnvVars(lb[1], env))
		if err != nil {
			return 0, 0, fmt.Errorf("error: invalid num periods in lookback_period: %s", lookbackStr)
		}
	}
	var firstPeriodId int
	per0 := env["$PERIOD_ID"]
	if per0 == nil {
		return 0, 0, fmt.Errorf("error: missing $PERIOD_ID in env for lookback_period: %s", lookbackStr)
	}
	switch v := per0.(type) {
	case int:
		firstPeriodId = v
	case int32:
		firstPeriodId = int(v)
	case int64:
		firstPeriodId = int(v)
	case float32:
		firstPeriodId = int(v)
	case float64:
		firstPeriodId = int(v)
	case string:
		firstPeriodId, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, fmt.Errorf("error: invalid $PERIOD_ID value in env for lookback_period: %s", lookbackStr)
		}
	default:
		return 0, 0, fmt.Errorf("error: invalid $PERIOD_ID type in env for lookback_period: %s", lookbackStr)
	}
	return firstPeriodId - firstPeriod, numPeriods, nil
}
