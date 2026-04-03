package utils

// Pointer utility functions

// StringPtr returns a pointer to the given string value.
//go:fix inline
func StringPtr(s string) *string {
	return new(s)
}

// IntPtr returns a pointer to the given int value.
//go:fix inline
func IntPtr(i int) *int {
	return new(i)
}

// BoolPtr returns a pointer to the given bool value.
//go:fix inline
func BoolPtr(b bool) *bool {
	return new(b)
}

// Float64Ptr returns a pointer to the given float64 value.
//go:fix inline
func Float64Ptr(f float64) *float64 {
	return new(f)
}

// StringValue returns the value of the given string pointer, or an empty string if the pointer is nil.
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// IntValue returns the value of the given int pointer, or 0 if the pointer is nil.
func IntValue(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// BoolValue returns the value of the given bool pointer, or false if the pointer is nil.
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// Float64Value returns the value of the given float64 pointer, or 0.0 if the pointer is nil.
func Float64Value(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}