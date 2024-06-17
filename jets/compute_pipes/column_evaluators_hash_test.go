package compute_pipes

import (
	"reflect"
	"testing"
)

// This file contains test cases for hashColumnEval
func TestHashColumnEval(t *testing.T) {
	// altExpr []string, columns map[string]int
	altExpr := []string{
		"key",
		"name",
		"format_date(dob)",
	}
	columns := map[string]int{
		"key":  0,
		"name": 1,
		"dob":  2,
	}
	pfnc, err := ParseAltKeyDefinition(altExpr, columns)
	if err != nil {
		t.Errorf("while calling ParseAltKeyDefinition: %v", err)
	}
	defaultPF := reflect.TypeOf(&DefaultPF{})
	formatDatePF := reflect.TypeOf(&FormatDatePF{})
	for i := range pfnc {
		switch reflect.TypeOf(pfnc[i]) {
		case formatDatePF:
		case defaultPF:
		default:
			t.Errorf("error unknown PreprocessingFunction implementation: %v", err)
		}
	}
	ctx := &hashColumnEval{
		inputPos:    0,
		outputPos:   0,
		partitions:  1,
		altInputKey: pfnc,
	}
	out, err := makeAlternateKey(&ctx.altInputKey, &[]interface{}{nil, "name", "6-14-2024"})
	if err != nil {
		t.Errorf("while calling makeAlternateKey: %v", err)
	}
	v, ok := out.(string)
	if !ok || v != "NAME20240614" {
		t.Errorf("error: expecting NAME20240614 got %v", out)
	}
}
