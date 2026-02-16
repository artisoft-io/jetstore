package utils

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

// Create test to validate ReplaceEnvVars
func TestReplaceEnvVars(t *testing.T) {
	env := map[string]any{
		"$VAR1": "value1",
		"$VAR2": "value2",
		"$VAR3": "$VAR1_value3",
	}
	assert := assert.New(t)
	result := ReplaceEnvVars("This is $VAR1 and $VAR2 and $VAR3", env)
	assert.Equal("This is value1 and value2 and value1_value3", result)

	// Test with no env vars
	result = ReplaceEnvVars("This is a string with no env vars", env)
	assert.Equal("This is a string with no env vars", result)

	// Test with nil env
	result = ReplaceEnvVars("This is $VAR1 and $VAR2 and $VAR3", nil)
	assert.Equal("This is $VAR1 and $VAR2 and $VAR3", result)

	// Test with env vars that are not in the string
	result = ReplaceEnvVars("This is a string with no matching env vars", env)
	assert.Equal("This is a string with no matching env vars", result)

	// Test with nested env vars
	env["$VAR4"] = "$VAR3_value4"
	result = ReplaceEnvVars("This is $VAR4", env)
	assert.Equal("This is value1_value3_value4", result)

	// Test non string values in env
	env["$VAR5"] = 123
	result = ReplaceEnvVars("This is $VAR5", env)
	assert.Equal("This is 123", result)

	// Test with env vars that reference each other in a loop, should not cause infinite loop
	env["$VAR4"] = "$VAR3_value4"
	result = ReplaceEnvVars("This is $VAR4", env)
	assert.Equal("This is value1_value3_value4", result)
}

// Create test to validate ParseLookbackPeriod
func TestParseLookbackPeriod(t *testing.T) {
	env := map[string]any{
		"${PERIOD_ID_TYPE}": "${MONTH_PERIOD}",
		"${MONTH_PERIOD}":   100,
	}
	assert := assert.New(t)
	firstPeriod, numPeriods, err := ParseLookbackPeriod("6", env)
	assert.NoError(err)
	assert.Equal(100, firstPeriod)
	assert.Equal(6, numPeriods)

	firstPeriod, numPeriods, err = ParseLookbackPeriod("0:6", env)
	assert.NoError(err)
	assert.Equal(100, firstPeriod)
	assert.Equal(6, numPeriods)

	firstPeriod, numPeriods, err = ParseLookbackPeriod("1:1", env)
	assert.NoError(err)
	assert.Equal(99, firstPeriod)
	assert.Equal(0, numPeriods)

	firstPeriod, numPeriods, err = ParseLookbackPeriod("1:6", env)
	assert.NoError(err)
	assert.Equal(99, firstPeriod)
	assert.Equal(5, numPeriods)

	firstPeriod, numPeriods, err = ParseLookbackPeriod("0", env)
	assert.NoError(err)
	assert.Equal(100, firstPeriod)
	assert.Equal(0, numPeriods)

	// Test with invalid lookbackStr
	_, _, err = ParseLookbackPeriod("invalid", env)
	assert.Error(err)

	// Test with invalid lookbackStr
	_, _, err = ParseLookbackPeriod("2:1", env)
	assert.Error(err)

	// Test with missing ${PERIOD_ID_TYPE} in env
	_, _, err = ParseLookbackPeriod("6", map[string]any{})
	assert.Error(err)
}