package main

import (
	"fmt"
	"unicode"
)

// Data cleansing functions

// OverpunchNumber
// Overpunch allows representation of positive and negatives in a numeric field without having to expand the size of the field
// for a plus or minus sign.  The overpunch character replaces the right-most character in a numeric field.
// In a six-digit field, 99.95 would be represented as 00999E.  In the same field, a negative 99.95 would be represented as 00999N.
// Lack of an overpunch character implies a positive amount - in other words, a positive 99.95 could be sent on the file as 009995.
//   Number,Positive Overpunch,Negative Overpunch
//   0,     {,                 }
//   1,     A,                 J
//   2,     B,                 K
//   3,     C,                 L
//   4,     D,                 M
//   5,     E,                 N
//   6,     F,                 O
//   7,     G,                 P
//   8,     H,                 Q
//   9,     I,                 R
type Overpunch struct {
	Sign   string
	Digit  string
}
var overpunchCharacters = map[rune]Overpunch{
	'{': {Sign: "+", Digit: "0"},
	'A': {Sign: "+", Digit: "1"},
	'B': {Sign: "+", Digit: "2"},
	'C': {Sign: "+", Digit: "3"},
	'D': {Sign: "+", Digit: "4"},
	'E': {Sign: "+", Digit: "5"},
	'F': {Sign: "+", Digit: "6"},
	'G': {Sign: "+", Digit: "7"},
	'H': {Sign: "+", Digit: "8"},
	'I': {Sign: "+", Digit: "9"},
	'}': {Sign: "-", Digit: "0"},
	'J': {Sign: "-", Digit: "1"},
	'K': {Sign: "-", Digit: "2"},
	'L': {Sign: "-", Digit: "3"},
	'M': {Sign: "-", Digit: "4"},
	'N': {Sign: "-", Digit: "5"},
	'O': {Sign: "-", Digit: "6"},
	'P': {Sign: "-", Digit: "7"},
	'Q': {Sign: "-", Digit: "8"},
	'R': {Sign: "-", Digit: "9"},
}

func OverpunchNumber(value string, decimalPlaces int) (string, error) {
	if decimalPlaces < 0 {
		return value, fmt.Errorf("decimalPlaces must be >= 0, we got %d", decimalPlaces)
	}
	l := len(value)
	if l == 0 {
		return "", nil
	}
	r := rune(value[l-1])

	if unicode.IsDigit(r) {
		if decimalPlaces == 0 {
			return value, nil
		}
		return fmt.Sprintf("%s.%s", value[:l-decimalPlaces], value[l-decimalPlaces:]), nil
	}

	oc, ok := overpunchCharacters[r]
	if !ok {
		return value, fmt.Errorf("unkown overpunch character: %s", string(r))
	}
	switch decimalPlaces {
	case 0:
		return fmt.Sprintf("%s%s%s", oc.Sign, value[:l-1], oc.Digit), nil	
	case 1:
		return fmt.Sprintf("%s%s.%s", oc.Sign, value[:l-1], oc.Digit), nil
	default:
		return fmt.Sprintf("%s%s.%s%s", oc.Sign, value[:l-decimalPlaces], value[l-decimalPlaces:l-1], oc.Digit), nil
	}
}
// func main() {
// 	fmt.Println(OverpunchNumber("12345", 0))
// 	fmt.Println(OverpunchNumber("12345", 1))
// 	fmt.Println(OverpunchNumber("12345", 2))
// 	fmt.Println(OverpunchNumber("1234E", 0))
// 	fmt.Println(OverpunchNumber("1234E", 1))
// 	fmt.Println(OverpunchNumber("1234E", 2))
// 	fmt.Println(OverpunchNumber("1234N", 0))
// 	fmt.Println(OverpunchNumber("1234N", 1))
// 	fmt.Println(OverpunchNumber("1234N", 2))
// 	fmt.Println(OverpunchNumber("1234f", 2))
// 	fmt.Println(OverpunchNumber("12345", -2))
// }
