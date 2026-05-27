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