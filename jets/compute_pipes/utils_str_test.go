package compute_pipes

import "testing"

func TestIsOnlyNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"12345", true},
		{"123a45", false},
		{"", true},
		{"0000", true},
		{"12 34", false},
	}

	for _, test := range tests {
		result := IsOnlyNumeric(test.input)
		if result != test.expected {
			t.Errorf("IsOnlyNumeric(%q) = %v; want %v", test.input, result, test.expected)
		}
	}
}

func TestFilterNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"12345", "12345"},
		{"123a45", "12345"},
		{"", ""},
		{"0000", "0000"},
		{"12 34", "1234"},
	}

	for _, test := range tests {
		result := FilterNumeric(test.input)
		if result != test.expected {
			t.Errorf("FilterNumeric(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}
