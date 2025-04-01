package compute_pipes

import (
	"fmt"
	"reflect"
	"testing"
)

// This file contains test cases for hashColumnEval

// Full end-to-end tests

func TestHashColumnEvalFull01(t *testing.T) {
	ctx := &BuilderContext{
		cpConfig: &ComputePipesConfig{
			ClusterConfig: &ClusterSpec{
				ShardingInfo: &ClusterShardingInfo{
					MaxNbrPartitions: 400,
					NbrPartitions:    131,
				},
			},
		},
	}
	inputColumns := map[string]int{
		"key":    0,
		"name":   1,
		"gender": 2,
		"dob":    3,
	}
	outputColumns := map[string]int{
		"jets_partition": 0,
	}
	// Build the Column Transformation Evaluator
	trsfEvaluator, err := ctx.BuildHashTCEvaluator(
		&InputChannel{
			name:    "input",
			columns: &inputColumns,
		},
		&OutputChannel{
			name:    "output",
			columns: &outputColumns,
		},
		&TransformationColumnSpec{
			Type: "hash",
			Name: "jets_partition",
			HashExpr: &HashExpression{
				Expr:                   "key",
				AlternateCompositeExpr: []string{"name", "gender", "format_date(dob)"},
				NoPartitions:           true,
			},
		},
	)
	if err != nil {
		t.Errorf("while calling BuildHashTCEvaluator: %v", err)
	}
	// Evaluate the column transformation operator
	currentOutputValue := make([]any, 1)
	inputValues := &[][]any{
		{"NAME1M19690101", "NAME1", "M", "1969-01-01"},
		{nil, "NAME1", "M", "1969-01-01"},
	}
	for _, inputRow := range *inputValues {
		err = trsfEvaluator.Update(&currentOutputValue, &inputRow)
		if err != nil {
			t.Errorf("while calling Update: %v", err)
		}
		fmt.Println("*** For", inputRow, ",Got:", currentOutputValue[0])
	}
	t.Error("done")
}

// Simplified tests

func TestHashColumnEvalSimple01(t *testing.T) {
	// altExpr []string, columns map[string]int
	altExpr := []string{
		"key",
		"name",
		"format_date(dob)",
	}
	columns := &map[string]int{
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

	out, err := makeAlternateKey(&pfnc, &[]interface{}{nil, "name", "6-14-2024"})
	if err != nil {
		t.Errorf("while calling makeAlternateKey: %v", err)
	}
	v, ok := out.(string)
	if !ok || v != "NAME20240614" {
		t.Errorf("error: expecting NAME20240614 got %v", out)
	}
}

func TestEvalHash(t *testing.T) {
	v := EvalHash(nil, 0)
	if v == nil {
		t.Fatal("error: got nil from EvalHash (1)")
	}
	if *v != 0 {
		t.Errorf("error: expecting 0 from EvalHash (1)")
	}
	v = EvalHash(nil, 1)
	if v == nil {
		t.Fatal("error: got nil from EvalHash (2)")
	}
	if *v != 0 {
		t.Errorf("error: expecting 0 from EvalHash (2)")
	}
	v = EvalHash(nil, 5)
	if v == nil {
		t.Fatal("error: got nil from EvalHash (3)")
	}
	if *v > 4 {
		t.Errorf("error: expecting [0,5) from EvalHash (3)")
	}
}
