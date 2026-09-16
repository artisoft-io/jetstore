package compute_pipes

import (
	"log"
	"testing"
	"time"
)
func TestOpDMonths(t *testing.T) {
	op := &opDMonths{}

	tests := []struct {
		name    string
		lhs     any
		rhs     any
		want    any
		wantErr bool
	}{
		{"both null", nil, nil, nil, false},
		{"lhs null", nil, time.Now(), nil, false},
		{"rhs null", time.Now(), nil, nil, false},
		{"valid dates", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC), 14, false},
		{"valid date strings", "2020-01-01", "2021-03-01", 14, false},
		{"invalid date string", "not a date", "2021-03-01", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := op.Eval(tt.lhs, tt.rhs)
			if (err != nil) != tt.wantErr {
				t.Errorf("opDMonths.Eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("opDMonths.Eval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpApplyFormat(t *testing.T) {
	op := &opApplyFormat{}

	tests := []struct {
		name    string
		lhs     any
		rhs     any
		want    any
		wantErr bool
	}{
		{"valid date with java format", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "yyyy-MM-dd", "2020-01-01", false},
		{"valid date with go format", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "2006-01-02", "2020-01-01", false},
		{"valid date with short go format", time.Date(2020, 1, 11, 0, 0, 0, 0, time.UTC), "2006-01", "2020-01", false},
		{"valid date with short java format", time.Date(2020, 1, 11, 0, 0, 0, 0, time.UTC), "yyyy-MM", "2020-01", false},
		{"valid date with fmt format", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "%04d-%02d-%02d", "2020-01-01", false},
		{"valid date with fmt format with time component", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "%04d-%02d-%02d %02d:%02d:%02d.%09d", "2020-01-01 00:00:00.000000000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := op.Eval(tt.lhs, tt.rhs)
			if (err != nil) != tt.wantErr {
				t.Errorf("opApplyFormat.Eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("opApplyFormat.Eval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpApplyRegex(t *testing.T) {

	tests := []struct {
		name    string
		lhs     any
		rhs     any
		want    any
		wantErr bool
	}{
		{"valid regex match", "abc123", "^[a-z]+\\d+$", "abc123", false},
		{"valid regex no match", "123abc", "^[a-z]+\\d+$", nil, false},
		{"invalid regex pattern", "abc123", "[a-z+\\d+", nil, true},
		{"get partial value", "abc123", "[a-z]+", "abc", false},
	}

	for _, tt := range tests {
		op := &opApplyRegex{}
		t.Run(tt.name, func(t *testing.T) {
			log.Printf("Testing opApplyRegex with lhs: %v, rhs: %v\n", tt.lhs, tt.rhs)
			got, err := op.Eval(tt.lhs, tt.rhs)
			if (err != nil) != tt.wantErr {
				t.Errorf("opApplyRegex.Eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("opApplyRegex.Eval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpToDate(t *testing.T) {
	op := &opToDate{}

	tests := []struct {
		name    string
		lhs     any
		want    any
		wantErr bool
	}{
		{"valid date string", "2020-01-01", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), false},
		{"invalid date string", "not a date", nil, true},
		{"int as seconds since epoch", int(1577836800), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := op.Eval(tt.lhs, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("opToDate.Eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("opToDate.Eval() = %v, want %v", got, tt.want)
			}
		})
	}
}
// TestOpContains pins the semantics opContains copies from the rules engine's ContainsOp
// (jets/jetrules/rete/expr_operator_str_contains.go): 1 when lhs contains rhs, 0 when it does not,
// and nil when either operand is not a string.
//
// The four non-string rows are deliberate. Returning nil rather than an error for a non-string
// operand reads oddly in isolation -- every other operator in this file either coerces or returns an
// error -- and it is the behaviour the rules engine has, which is the whole point of copying it: the
// same expression written in a .jr file and in a .pc.json must mean the same thing. These rows exist
// so that a later reader who finds the behaviour surprising finds it asserted rather than accidental.
//
// Each row is driven through both opContains{} and opContains{noCase: true}, so that
// contains_no_case differing from contains on case and agreeing on everything else is a property of
// the table rather than of a reader comparing two lists. differsOnCase says which rows are the ones
// allowed to differ.
func TestOpContains(t *testing.T) {

	tests := []struct {
		name          string
		lhs           any
		rhs           any
		want          any
		wantNoCase    any
		differsOnCase bool
		wantErr       bool
	}{
		{"substring present", "S1-Hedis-E01", "Hedis", 1, 1, false, false},
		{"substring absent", "S1-E01", "Hedis", 0, 0, false, false},
		{"substring absent, other discriminator", "S1-HCC-M02", "Hedis", 0, 0, false, false},
		{"empty rhs", "S1-E01", "", 1, 1, false, false},
		{"empty lhs, empty rhs", "", "", 1, 1, false, false},
		{"empty lhs, non-empty rhs", "", "HCC", 0, 0, false, false},
		{"identical strings", "HCC", "HCC", 1, 1, false, false},
		{"rhs longer than lhs", "HCC", "S1-HCC-M02", 0, 0, false, false},
		{"case difference in lhs", "S1-hedis-E01", "Hedis", 0, 1, true, false},
		{"case difference in rhs", "S1-HCC-M02", "hcc", 0, 1, true, false},
		{"case difference both sides", "s1-hedis-e01", "HEDIS", 0, 1, true, false},
		{"nil lhs", nil, "Hedis", nil, nil, false, false},
		{"nil rhs", "S1-Hedis-E01", nil, nil, nil, false, false},
		{"both nil", nil, nil, nil, nil, false, false},
		// Non-string operands: nil on each side, deliberately, per ContainsOp.
		{"non-string lhs", 42, "4", nil, nil, false, false},
		{"non-string rhs", "42", 4, nil, nil, false, false},
		{"non-string both sides", 42, 42, nil, nil, false, false},
		{"bool lhs", true, "true", nil, nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.want != tt.wantNoCase) != tt.differsOnCase {
				t.Errorf("table row %q: want %v and wantNoCase %v, differsOnCase %v -- the row contradicts itself",
					tt.name, tt.want, tt.wantNoCase, tt.differsOnCase)
			}
			for _, c := range []struct {
				noCase bool
				want   any
			}{{false, tt.want}, {true, tt.wantNoCase}} {
				op := &opContains{noCase: c.noCase}
				got, err := op.Eval(tt.lhs, tt.rhs)
				if (err != nil) != tt.wantErr {
					t.Errorf("opContains{noCase:%v}.Eval() error = %v, wantErr %v", c.noCase, err, tt.wantErr)
					return
				}
				if got != c.want {
					t.Errorf("opContains{noCase:%v}.Eval() = %v, want %v", c.noCase, got, c.want)
				}
			}
		})
	}
}

// TestBuildEvalOperatorContains demonstrates that contains is registered and that an unknown
// operator still is not. BuildEvalOperator upper-cases before dispatching, so the .pc.json may
// spell the operator in any case; the lower-case spellings are the ones the workspace configs use.
func TestBuildEvalOperatorContains(t *testing.T) {

	tests := []struct {
		name       string
		op         string
		wantNoCase bool
		wantErr    bool
	}{
		{"contains, as a .pc.json spells it", "contains", false, false},
		{"contains, upper case", "CONTAINS", false, false},
		{"contains_no_case, as a .pc.json spells it", "contains_no_case", true, false},
		{"contains_no_case, upper case", "CONTAINS_NO_CASE", true, false},
		{"unknown operator still fails", "icontains", false, true},
		{"unknown operator still fails, empty", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildEvalOperator(tt.op)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildEvalOperator(%q) error = %v, wantErr %v", tt.op, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			op, ok := got.(*opContains)
			if !ok {
				t.Errorf("BuildEvalOperator(%q) = %T, want *opContains", tt.op, got)
				return
			}
			if op.noCase != tt.wantNoCase {
				t.Errorf("BuildEvalOperator(%q) noCase = %v, want %v", tt.op, op.noCase, tt.wantNoCase)
			}
		})
	}
}
