package utils

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

// Create test to validate pointers functions
func TestPointersFunctions(t *testing.T) {
	assert := assert.New(t)

	// Test StringPtr and StringValue
	str := "test"
	strPtr := StringPtr(str)
	assert.Equal(str, StringValue(strPtr))
	assert.Equal("", StringValue(nil))

	// Test IntPtr and IntValue
	i := 43
	intPtr := new(i)
	assert.Equal(i, IntValue(intPtr))
	assert.Equal(0, IntValue(nil))

	// Test BoolPtr and BoolValue
	b := true
	boolPtr := BoolPtr(b)
	assert.Equal(b, BoolValue(boolPtr))
	assert.Equal(false, BoolValue(nil))

	// Test Float64Ptr and Float64Value
	f := 3.14
	floatPtr := Float64Ptr(f)
	assert.Equal(f, Float64Value(floatPtr))
	assert.Equal(0.0, Float64Value(nil))

	// WOW!
	assert.Equal(true, *new(true))
}
