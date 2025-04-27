package compute_pipes

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
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
	expectedHash := uint64(5273485936479607779)
	for _, inputRow := range *inputValues {
		err = trsfEvaluator.Update(&currentOutputValue, &inputRow)
		if err != nil {
			t.Errorf("while calling Update: %v", err)
		}
		// fmt.Println("*** For", inputRow, ",Got:", currentOutputValue[0])
		if expectedHash != currentOutputValue[0] {
			t.Errorf("hash failed")
		}
	}
	// t.Error("done")
}

// Simulate compute domain key for domain entity
func TestComputeDomainKeyFull01(t *testing.T) {
	HashingSeed, _ = uuid.Parse("38dfae30-f8c9-1111-9dfe-62e96ab8a622")
	DomainKeyDelimit = "-"
	HashingAlgo = "sha1"
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
		"Claim:domain_key": 0,
	}
	// Build the Column Transformation Evaluator
	trsfEvaluator, err := ctx.BuildHashTCEvaluator(
		&InputChannel{
			name:    "input",
			columns: &inputColumns,
			domainKeySpec: &DomainKeysSpec{
				HashingOverride: "none",
				DomainKeys: map[string]*DomainKeyInfo{
					"Claim": {
						KeyExpr:    []string{"key"},
						ObjectType: "Claim",
					},
				},
			},
		},
		&OutputChannel{
			name:    "output",
			columns: &outputColumns,
		},
		&TransformationColumnSpec{
			Type: "hash",
			Name: "Claim:domain_key",
			HashExpr: &HashExpression{
				DomainKey:        "Claim",
				ComputeDomainKey: true,
			},
		},
	)
	if err != nil {
		t.Errorf("while calling BuildHashTCEvaluator: %v", err)
	}
	// Evaluate the compute domain key
	currentOutputValue := make([]any, 1)
	inputValues := &[]any{"38dfae30-f8c9-51ae-9dfe-62e96ab8a622", "NAME1", "M", "1969-01-01"}
	expectedKey := "38dfae30-f8c9-51ae-9dfe-62e96ab8a622"
		err = trsfEvaluator.Update(&currentOutputValue, inputValues)
		if err != nil {
			t.Errorf("while calling Update: %v", err)
		}
		// fmt.Println("*** Got:", currentOutputValue[0])
		if expectedKey != currentOutputValue[0] {
			t.Errorf("hash failed")
		}
	// t.Error("done")
}

// Simulate compute domain key for file entity with override
func TestComputeDomainKeyFull02(t *testing.T) {
	HashingSeed, _ = uuid.Parse("38dfae30-f8c9-1111-9dfe-62e96ab8a622")
	DomainKeyDelimit = "-"
	HashingAlgo = "sha1"
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
		"Claim:domain_key": 0,
	}
	// Build the Column Transformation Evaluator
	trsfEvaluator, err := ctx.BuildHashTCEvaluator(
		&InputChannel{
			name:    "input",
			columns: &inputColumns,
			domainKeySpec: &DomainKeysSpec{
				HashingOverride: "none",
				DomainKeys: map[string]*DomainKeyInfo{
					"Claim": {
						KeyExpr:    []string{"remove_mi(name)", "gender", "format_date(dob)"},
						ObjectType: "Claim",
					},
				},
			},
		},
		&OutputChannel{
			name:    "output",
			columns: &outputColumns,
		},
		&TransformationColumnSpec{
			Type: "hash",
			Name: "Claim:domain_key",
			HashExpr: &HashExpression{
				DomainKey:        "Claim",
				ComputeDomainKey: true,
			},
		},
	)
	if err != nil {
		t.Errorf("while calling BuildHashTCEvaluator: %v", err)
	}
	// Evaluate the compute domain key
	currentOutputValue := make([]any, 1)
	inputValues := &[]any{"key1", "name 1", "m", "1969-01-01"}
	expectedKey := "NAME-M-19690101"
		err = trsfEvaluator.Update(&currentOutputValue, inputValues)
		if err != nil {
			t.Errorf("while calling Update: %v", err)
		}
		// fmt.Println("*** Got:", currentOutputValue[0])
		if expectedKey != currentOutputValue[0] {
			t.Errorf("hash failed")
		}
	// t.Error("done")
}

// Simulate compute domain key for file entity w/o override
func TestComputeDomainKeyFull03(t *testing.T) {
	HashingSeed, _ = uuid.Parse("38dfae30-f8c9-1111-9dfe-62e96ab8a622")
	DomainKeyDelimit = "-"
	HashingAlgo = "sha1"
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
		"Claim:domain_key": 0,
	}
	// Build the Column Transformation Evaluator
	trsfEvaluator, err := ctx.BuildHashTCEvaluator(
		&InputChannel{
			name:    "input",
			columns: &inputColumns,
			domainKeySpec: &DomainKeysSpec{
				// HashingOverride: "none",
				DomainKeys: map[string]*DomainKeyInfo{
					"Claim": {
						KeyExpr:    []string{"remove_mi(name)", "gender", "format_date(dob)"},
						ObjectType: "Claim",
					},
				},
			},
		},
		&OutputChannel{
			name:    "output",
			columns: &outputColumns,
		},
		&TransformationColumnSpec{
			Type: "hash",
			Name: "Claim:domain_key",
			HashExpr: &HashExpression{
				DomainKey:        "Claim",
				ComputeDomainKey: true,
			},
		},
	)
	if err != nil {
		t.Errorf("while calling BuildHashTCEvaluator: %v", err)
	}
	// Evaluate the compute domain key
	currentOutputValue := make([]any, 1)
	inputValues := &[]any{"key1", "name 1", "m", "1969-01-01"}
	expectedKey := "a1366cc9-38bb-50aa-b6dc-9d91f0249039"
		err = trsfEvaluator.Update(&currentOutputValue, inputValues)
		if err != nil {
			t.Errorf("while calling Update: %v", err)
		}
		// fmt.Println("*** Got:", currentOutputValue[0])
		if expectedKey != currentOutputValue[0] {
			t.Errorf("hash failed")
		}
	// t.Error("done")
}

// Simplified tests

func TestComputeDomainKey01(t *testing.T) {
	HashingSeed, _ = uuid.Parse("38dfae30-f8c9-1111-9dfe-62e96ab8a622")
	DomainKeyDelimit = "-"
	HashingAlgo = "sha1"

	inputColumns := map[string]int{
		"key":    0,
		"name":   1,
		"gender": 2,
		"dob":    3,
	}
	keys := []string{"remove_mi(name)", "gender", "format_date(dob)"}
	compositeInputKey, err := ParsePreprocessingExpressions(keys, len(keys) > 1, &inputColumns)
	if err != nil {
		t.Errorf("while calling ParsePreprocessingExpressions: %v", err)
	}

	input := []any{"NAME1M19690101", "name 1", "M", "1969-01-01"}
	hashEval := &HashEvaluator{
		compositeInputKey: compositeInputKey,
		computeDomainKey:  true,
		hashingAlgo:       HashingAlgo_None,
		delimit:           "-",
	}
	domainKey, err := hashEval.ComputeDomainKey(input)
	if err != nil {
		t.Error(err)
	}
	// fmt.Println("*** DomainKey:", domainKey)
	if domainKey != "NAME-M-19690101" {
		t.Error("expecting NAME-M-19690101")
	}
	// t.Error("done")
}

func TestComputeDomainKey02(t *testing.T) {
	HashingSeed, _ = uuid.Parse("38dfae30-f8c9-1111-9dfe-62e96ab8a622")
	DomainKeyDelimit = "-"
	HashingAlgo = "sha1"

	inputColumns := map[string]int{
		"key":    0,
		"name":   1,
		"gender": 2,
		"dob":    3,
	}
	keys := []string{"remove_mi(name)", "gender", "format_date(dob)"}
	compositeInputKey, err := ParsePreprocessingExpressions(keys, len(keys) > 1, &inputColumns)
	if err != nil {
		t.Errorf("while calling ParsePreprocessingExpressions: %v", err)
	}

	input := []any{"NAME1M19690101", "name 1", "m", "1969-01-01"}
	hashEval := &HashEvaluator{
		compositeInputKey: compositeInputKey,
		computeDomainKey:  true,
		hashingAlgo:       HashingAlgo_SHA1,
		delimit:           "-",
	}
	domainKey, err := hashEval.ComputeDomainKey(input)
	if err != nil {
		t.Error(err)
	}
	// fmt.Println("*** DomainKey:", domainKey)
	if domainKey != "a1366cc9-38bb-50aa-b6dc-9d91f0249039" {
		t.Error("not good")
	}
	// t.Error("done")
}

func TestComputeDomainKey03(t *testing.T) {
	HashingSeed, _ = uuid.Parse("38dfae30-f8c9-1111-9dfe-62e96ab8a622")
	DomainKeyDelimit = "-"
	HashingAlgo = "sha1"

	inputColumns := map[string]int{
		"key":    0,
		"name":   1,
		"gender": 2,
		"dob":    3,
	}
	keys := []string{"key"}
	compositeInputKey, err := ParsePreprocessingExpressions(keys, len(keys) > 1, &inputColumns)
	if err != nil {
		t.Errorf("while calling ParsePreprocessingExpressions: %v", err)
	}

	input := []any{"38dfae30-f8c9-51ae-9dfe-62e96ab8a622", "name 1", "m", "1969-01-01"}
	hashEval := &HashEvaluator{
		compositeInputKey: compositeInputKey,
		computeDomainKey:  true,
		hashingAlgo:       HashingAlgo_None,
		delimit:           "-",
	}
	domainKey, err := hashEval.ComputeDomainKey(input)
	if err != nil {
		t.Error(err)
	}
	// fmt.Println("*** DomainKey:", domainKey)
	if domainKey != "38dfae30-f8c9-51ae-9dfe-62e96ab8a622" {
		t.Error("not good")
	}
	// t.Error("done")
}

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
	pfnc, err := ParsePreprocessingExpressions(altExpr, true, columns)
	if err != nil {
		t.Errorf("while calling ParsePreprocessingExpressions: %v", err)
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
