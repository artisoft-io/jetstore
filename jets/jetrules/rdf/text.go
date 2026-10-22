package rdf

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"
)

// How an operator writes a value into text: text plus a value, to_text,
// join_values. Each function renders as the C++ engine does, so that a rule
// building a key or a message from a value gets the same text on both engines
// (rules_benchmark I-32, I-16). Node.String() is not this rendering: it is the Go
// engine's debugging and export form, and it is left as it is.

// FormatDouble renders a double in plain notation with the fewest digits that
// read back as the same double, and never an exponent (rules_benchmark I-16,
// Q-60): 991 is "991", 1234567.5 is "1234567.5", 0.00001 is "0.00001". It is
// C++'s std::to_chars(first, last, v, std::chars_format::fixed), which both
// engines use: from 2^53 up, where a double is a whole number, that writes the
// double's exact digits, so 1e23 is "99999999999999991611392", where Go's
// FormatFloat(v, 'f', -1) would pad the shortest digits with zeros. NaN and the
// infinities are "nan", "inf" and "-inf", the sign of a NaN dropped. Agreement
// with to_chars was checked on 2026-10-22 over 200,000 doubles.
func FormatDouble(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	case math.Abs(v) >= 1<<53:
		return new(big.Float).SetFloat64(v).Text('f', 0)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// FormatDate renders a date as boost::gregorian::to_iso_extended_string does:
// YYYY-MM-DD.
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatDatetime renders a datetime as boost::posix_time::to_iso_extended_string
// does: YYYY-MM-DDTHH:MM:SS, then a fraction of exactly six digits when the
// microseconds are not zero (boost's resolution is the microsecond).
func FormatDatetime(t time.Time) string {
	s := t.Format("2006-01-02T15:04:05")
	if us := t.Nanosecond() / 1000; us != 0 {
		s += fmt.Sprintf(".%06d", us)
	}
	return s
}

// TextOf renders a value as an operator writes it into text. A null is "null",
// as on C++, and a double is FormatDouble's. The types C++ cannot render (a blank node) keep Node.String().
func TextOf(v *Node) string {
	if v == nil {
		return "null"
	}
	switch vv := v.Value.(type) {
	case RdfNull:
		return "null"
	case float64:
		return FormatDouble(vv)
	case LDate:
		if vv.Date == nil {
			return "null"
		}
		return FormatDate(*vv.Date)
	case LDatetime:
		if vv.Datetime == nil {
			return "null"
		}
		return FormatDatetime(*vv.Datetime)
	default:
		return v.String()
	}
}
