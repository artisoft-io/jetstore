package graphdump

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
)

// uuidV4 is the form of a random UUID, which both engines mint: Go's
// uuid.NewString and C++'s boost random_generator (jets/rdf/uuid.cc).
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// IsMintedName reports whether name is one an engine invents at random.
func IsMintedName(name string) bool { return uuidV4.MatchString(name) }

// Quantise is the Go engine's rounding of a double literal: 15 bits of mantissa
// (rdf.ResourceManager.NewDoubleLiteral, jets/jetrules/rdf/resource_manager.go).
func Quantise(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	v, _ := big.NewFloat(x).SetPrec(15).Float64()
	return v
}

// Quantum is the step of the Go engine's 15-bit grid at x: the difference
// between two neighbouring quantised doubles of x's magnitude. 0 for 0, NaN and
// the infinities.
func Quantum(x float64) float64 {
	if x == 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	_, exp := math.Frexp(x)
	return math.Ldexp(1, exp-15)
}

// FormatDouble is how a double is written: the shortest decimal that reads back
// to x.
func FormatDouble(x float64) string { return strconv.FormatFloat(x, 'g', -1, 64) }

// renderGround writes a term that is not minted. quantised writes a double as
// the Go engine would hold it, which is how terms are written into colours.
func renderGround(t harness.Term, quantised bool) string {
	switch t.Kind {
	case harness.TermNull:
		return "null"
	case harness.TermResource:
		return renderResource(t.Text)
	case harness.TermText:
		return strconv.Quote(t.Text)
	case harness.TermInt, harness.TermUint, harness.TermDate:
		return t.Text + "^^" + t.Kind
	case harness.TermDatetime:
		return t.Text + "Z^^" + t.Kind
	case harness.TermDouble:
		x := t.Num
		if quantised {
			x = Quantise(x)
		}
		return FormatDouble(x) + "^^double"
	case harness.TermBlank:
		// Always minted; written here only if renaming was skipped.
		return "_:" + t.Text
	}
	return strconv.Quote(t.Text) + "^^" + t.Kind
}

func renderResource(name string) string {
	if name == "" || name == "null" || strings.ContainsAny(name, "\t\n\r") ||
		strings.HasPrefix(name, `"`) || strings.HasPrefix(name, "_:") || strings.Contains(name, "^^") {
		return strconv.Quote(name) + "^^resource"
	}
	return name
}

// Term is one term of a dump line, read back by ParseTerm.
type Term struct {
	// Kind is resource, text, int, uint, double, date, datetime, null, minted or
	// minted_text, the last two for a minted node and its text form.
	Kind string
	// Value is the resource name, the text, the label of a minted node, or the
	// literal as written before its ^^.
	Value string
}

// Num is a double term's value; false for any other kind.
func (t Term) Num() (float64, bool) {
	if t.Kind != harness.TermDouble {
		return 0, false
	}
	v, err := strconv.ParseFloat(t.Value, 64)
	return v, err == nil
}

// ParseLine splits a dump line into its three terms.
func ParseLine(line string) (s, p, o Term, ok bool) {
	parts := strings.Split(line, "\t")
	if len(parts) != 3 {
		return
	}
	var ok1, ok2, ok3 bool
	s, ok1 = ParseTerm(parts[0])
	p, ok2 = ParseTerm(parts[1])
	o, ok3 = ParseTerm(parts[2])
	return s, p, o, ok1 && ok2 && ok3
}

// ParseTerm reads one term as written in a dump.
func ParseTerm(w string) (Term, bool) {
	switch {
	case w == "null":
		return Term{Kind: harness.TermNull}, true
	case strings.HasPrefix(w, "_:"):
		if label, ok := strings.CutSuffix(w[2:], "^^text"); ok {
			return Term{Kind: "minted_text", Value: label}, true
		}
		return Term{Kind: "minted", Value: w[2:]}, true
	case strings.HasPrefix(w, `"`):
		q, err := strconv.QuotedPrefix(w)
		if err != nil {
			return Term{}, false
		}
		v, err := strconv.Unquote(q)
		if err != nil {
			return Term{}, false
		}
		switch rest := w[len(q):]; rest {
		case "":
			return Term{Kind: harness.TermText, Value: v}, true
		case "^^resource":
			return Term{Kind: harness.TermResource, Value: v}, true
		}
		return Term{}, false
	}
	if i := strings.LastIndex(w, "^^"); i >= 0 {
		return Term{Kind: w[i+2:], Value: w[:i]}, true
	}
	return Term{Kind: harness.TermResource, Value: w}, true
}
