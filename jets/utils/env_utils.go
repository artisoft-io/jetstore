package utils

import "strings"

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
