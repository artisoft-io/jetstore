package cleansing_functions

import (
	"strings"
	"testing"
)

// applySubstring runs the substring cleansing function the way a map column
// does, through ApplyCleasingFunction, and fails the test on a panic.
func applySubstring(t *testing.T, argument, input string) (obj any, errMsg string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("substring(%q) on %q panicked: %v", argument, input, r)
		}
	}()
	ctx := NewCleansingFunctionContext(&map[string]int{})
	return ctx.ApplyCleasingFunction("substring", argument, input, &[]any{input})
}

func TestSubstring(t *testing.T) {
	tests := []struct {
		name     string
		argument string
		input    string
		want     any // nil or a string
		wantErr  string
	}{
		// In range: what the function has always given
		{"prefix", "0,3", "abcdef", "abc", ""},
		{"middle", "2,5", "abcdef", "cde", ""},
		{"whole value", "0,6", "abcdef", "abcdef", ""},
		{"spaces around the numbers", " 1 , 3 ", "abcdef", "bc", ""},
		{"negative end counts from the end", "0,-2", "abcdef", "abcd", ""},
		{"negative end with a start", "1,-1", "abcdef", "bcde", ""},
		{"start past a negative end", "4,-3", "abcdef", nil, ""},

		// Out of range: an empty result (nil), never a panic
		{"end past the value", "0,10", "abcdef", nil, ""},
		{"end one past the value", "2,7", "abcdef", nil, ""},
		{"start past the value", "8,10", "abcdef", nil, ""},
		{"start at the end of the value", "6,7", "abcdef", nil, ""},
		{"start past the value, negative end", "8,-1", "abcdef", nil, ""},
		{"empty value", "0,3", "", nil, ""},
		{"empty value, negative end", "0,-1", "", nil, ""},
		{"negative end past the start of the value", "0,-10", "abcdef", nil, ""},
		{"negative end resolving to the start", "0,-6", "abcdef", nil, ""},
		{"end zero", "0,0", "abcdef", nil, ""},
		{"end zero with a start", "2,0", "abcdef", nil, ""},

		// Refused when the argument is parsed: an error, reported per record
		{"negative start", "-1,3", "abcdef", nil, "start must be 0 or more"},
		{"negative start, value shorter than end", "-1,10", "abc", nil, "start must be 0 or more"},
		{"negative start and end", "-3,-1", "abcdef", nil, "start must be 0 or more"},
		{"start after end", "5,3", "abcdef", nil, "end must be greater than start"},
		{"start equal to end", "3,3", "abcdef", nil, "end must be greater than start"},
		{"one number", "3", "abcdef", nil, "cannot be parsed as start,end"},
		{"three numbers", "1,2,3", "abcdef", nil, "cannot be parsed as start,end"},
		{"not a number", "a,3", "abcdef", nil, "cannot be parsed as start,end"},
		{"empty argument", "", "abcdef", nil, "unexpected null argument"},

		// Positions are characters (code points), not bytes: "é" is two bytes in
		// UTF-8, "€" three, "😀" four
		{"ascii prefix of a non-ascii value", "0,1", "héllo", "h", ""},
		{"a two-byte character", "1,2", "héllo", "é", ""},
		{"characters after a two-byte character", "0,4", "héllo", "héll", ""},
		{"a cut bytes would have split", "0,2", "héllo", "hé", ""},
		{"the whole non-ascii value", "0,5", "héllo", "héllo", ""},
		{"end past the characters, within the bytes", "0,6", "héllo", nil, ""},
		{"negative end on a non-ascii value", "0,-1", "été", "ét", ""},
		{"negative end drops a multi-byte character", "1,-1", "aéé", "é", ""},
		{"a three-byte character", "1,2", "5€!", "€", ""},
		{"a four-byte character", "1,2", "a😀b", "😀", ""},
		{"after a four-byte character", "2,3", "a😀b", "b", ""},
		{"empty value, non-ascii range", "0,1", "", nil, ""},
		// No normalisation: "e" and a combining acute (U+0301) are two characters
		{"a combining accent is its own character", "0,1", "e\u0301t", "e", ""},
		{"the combining accent alone", "1,2", "e\u0301t", "\u0301", ""},
		// A byte that does not start a valid sequence is one character, kept as is
		{"an invalid byte", "1,2", "a\xffb", "\xff", ""},
		{"after an invalid byte", "2,3", "a\xffb", "b", ""},
		{"stray continuation bytes", "0,2", "\x80\x80", "\x80\x80", ""},
		{"a truncated sequence is one character per byte", "1,3", "a\xe2\x82", "\xe2\x82", ""},
		{"a truncated sequence, its last byte", "2,3", "a\xe2\x82", "\x82", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errMsg := applySubstring(t, tt.argument, tt.input)
			if tt.wantErr != "" {
				if !strings.Contains(errMsg, tt.wantErr) {
					t.Errorf("substring(%q) on %q: error %q, want one containing %q", tt.argument, tt.input, errMsg, tt.wantErr)
				}
				if got != nil {
					t.Errorf("substring(%q) on %q: got %#v with an error, want nil", tt.argument, tt.input, got)
				}
				return
			}
			if errMsg != "" {
				t.Errorf("substring(%q) on %q: unexpected error %q", tt.argument, tt.input, errMsg)
			}
			if got != tt.want {
				t.Errorf("substring(%q) on %q: got %#v, want %#v", tt.argument, tt.input, got, tt.want)
			}
		})
	}
}

// A refused argument is not cached: every record gets the error, and a valid
// argument parsed afterwards in the same context is unaffected.
func TestSubstringRefusedArgumentIsNotCached(t *testing.T) {
	ctx := NewCleansingFunctionContext(&map[string]int{})
	for range 2 {
		if _, errMsg := ctx.ApplyCleasingFunction("substring", "-1,3", "abcdef", nil); errMsg == "" {
			t.Fatalf("expecting an error for a negative start")
		}
	}
	if got, errMsg := ctx.ApplyCleasingFunction("substring", "1,3", "abcdef", nil); errMsg != "" || got != "bc" {
		t.Errorf("got %#v, %q, want \"bc\" and no error", got, errMsg)
	}
}

func TestParseSubStringFunctionArgument(t *testing.T) {
	tests := []struct {
		rawArg    string
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{"0,3", 0, 3, false},
		{"2,-1", 2, -1, false},
		{"0,0", 0, 0, false},
		{"5,-10", 5, -10, false},
		{"-1,3", 0, 0, true},
		{"-1,0", 0, 0, true},
		{"3,3", 0, 0, true},
		{"4,2", 0, 0, true},
		{"", 0, 0, true},
		{"1", 0, 0, true},
		{"1,x", 0, 0, true},
	}
	for _, tt := range tests {
		arg, err := ParseSubStringFunctionArgument(tt.rawArg, "substring", map[string]any{})
		if tt.wantErr {
			if err == nil {
				t.Errorf("%q: expecting an error, got %+v", tt.rawArg, arg)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", tt.rawArg, err)
			continue
		}
		if arg.Start != tt.wantStart || arg.End != tt.wantEnd {
			t.Errorf("%q: got %d,%d, want %d,%d", tt.rawArg, arg.Start, arg.End, tt.wantStart, tt.wantEnd)
		}
	}
}

func TestOverpunchNumber(t *testing.T) {
	tests := []struct {
		value         string
		decimalPlaces int
		want          string
		wantErr       bool
	}{
		// In range: what the function has always given
		{"12345", 0, "12345", false},
		{"12345", 1, "1234.5", false},
		{"12345", 2, "123.45", false},
		{"1234E", 0, "+12345", false},
		{"1234E", 1, "+1234.5", false},
		{"1234E", 2, "+123.45", false},
		{"1234N", 2, "-123.45", false},
		{"00999E", 2, "+0099.95", false},
		{"12", 2, ".12", false},
		{"1E", 2, "+.15", false},
		{"", 2, "", false},
		{"1234f", 2, "1234f", true},
		{"12345", -2, "12345", true},

		// Fewer characters than decimal places: the missing leading zeros are
		// put back, as in a fixed-width field, instead of panicking
		{"5", 2, ".05", false},
		{"5", 3, ".005", false},
		{"E", 2, "+.05", false},
		{"N", 3, "-.005", false},
		{"1E", 3, "+.015", false},

		// Positions count characters, as substring does (2026-11-14): a cut
		// never splits a multi-byte character. Counting bytes, "1é5" gave
		// "1\xc3.\xa95" and "é5" with 3 places gave ".é5"
		{"1é5", 2, "1.é5", false},
		{"é5", 3, ".0é5", false},
		{"é1E", 2, "+é.15", false},
		{"é", 2, "0é", true},
		// A byte that does not start a valid UTF-8 sequence is one character,
		// kept as it is
		{"\xff12", 2, "\xff.12", false},
		{"12\xff", 0, "12\xff", true},
	}
	for _, tt := range tests {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("OverpunchNumber(%q, %d) panicked: %v", tt.value, tt.decimalPlaces, r)
				}
			}()
			got, err := OverpunchNumber(tt.value, tt.decimalPlaces)
			if (err != nil) != tt.wantErr {
				t.Errorf("OverpunchNumber(%q, %d): error %v, want error %v", tt.value, tt.decimalPlaces, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("OverpunchNumber(%q, %d) = %q, want %q", tt.value, tt.decimalPlaces, got, tt.want)
			}
		}()
	}
}

// overpunch_number through ApplyCleasingFunction: the argument is the number of
// decimal places, and a value shorter than it no longer panics.
func TestOverpunchNumberCleansingFunction(t *testing.T) {
	ctx := NewCleansingFunctionContext(&map[string]int{})
	tests := []struct {
		argument string
		input    string
		want     any
		wantErr  bool
	}{
		{"2", "12345", "123.45", false},
		{"2", "5", ".05", false},
		{"3", "1E", "+.015", false},
		{"-1", "12345", nil, true},
		{"x", "12345", nil, true},
		{"2", "1é5", "1.é5", false},
	}
	for _, tt := range tests {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("overpunch_number(%q) on %q panicked: %v", tt.argument, tt.input, r)
				}
			}()
			got, errMsg := ctx.ApplyCleasingFunction("overpunch_number", tt.argument, tt.input, nil)
			if (errMsg != "") != tt.wantErr {
				t.Errorf("overpunch_number(%q) on %q: error %q, want error %v", tt.argument, tt.input, errMsg, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("overpunch_number(%q) on %q: got %#v, want %#v", tt.argument, tt.input, got, tt.want)
			}
		}()
	}
}
