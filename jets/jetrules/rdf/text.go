package rdf

import (
	"fmt"
	"time"
)

// How an operator writes a value into text: text plus a value, to_text,
// join_values. Each function renders as the C++ engine does, so that a rule
// building a key or a message from a value gets the same text on both engines
// (rules_benchmark I-32). Node.String() is not this rendering: it is the Go
// engine's debugging and export form, and it is left as it is.

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
// as on C++. The types C++ cannot render (a blank node) keep Node.String().
func TextOf(v *Node) string {
	if v == nil {
		return "null"
	}
	switch vv := v.Value.(type) {
	case RdfNull:
		return "null"
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
