package compute_pipes

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// This file contains test cases for hashColumnEval

// Full end-to-end tests
// Simulate compute hash key for partitioning
// Note: when computing hash (and not the domain key), the hasing algo is always FNV-1a 64bit and the output is always uint64, even if the input is composite key. The hashing override and computeDomainKey are only for computing domain key, not for regular hash-based partitioning.
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
			Name:    "input",
			Columns: &inputColumns,
		},
		&OutputChannel{
			Name:    "output",
			Columns: &outputColumns,
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
	var h any = trsfEvaluator
	evaluator := h.(*hashColumnEval)
	log.Printf("Hash Evaluator is using '%s'", evaluator.hashEvaluator.hashingAlgo.String())

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
		fmt.Println("*** For", inputRow, ",Got:", currentOutputValue[0])
		if expectedHash != currentOutputValue[0] {
			t.Errorf("hash failed")
		}
	}
	t.Error("done")
}

// Simulate compute domain key for domain entity using single column with override to no hashing (i.e. use the column value as is for domain key)
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
			Name:    "input",
			Columns: &inputColumns,
			DomainKeySpec: &DomainKeysSpec{
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
			Name:    "output",
			Columns: &outputColumns,
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
	// Evaluate the domain key
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

// Simulate compute domain key for file entity with composite key and override
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
			Name:    "input",
			Columns: &inputColumns,
			DomainKeySpec: &DomainKeysSpec{
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
			Name:    "output",
			Columns: &outputColumns,
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
	// Evaluate the compute domain key: case override to upper case only with composite key
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

// Simulate compute domain key for file entity with and without override
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
	// Evaluate the domain key
	currentOutputValue := make([]any, 1)
	var expectedKey string
	inputValues := [][]any{
		{"key1", "name 1", "m", "1969-01-01"},
		{"", "name 2", "M", "1969-01-01"},
	}
	for _, hashingOverride := range []string{"", "none"} {
		if hashingOverride == "none" {
			// do the override
			expectedKey = "NAME-M-19690101"
		} else {
			// take the default of sha1 hashing
			expectedKey = "a1366cc9-38bb-50aa-b6dc-9d91f0249039"
		}

		// Build the Column Transformation Evaluator
		inputChannel := &InputChannel{
			Name:    "input",
			Columns: &inputColumns,
			DomainKeySpec: &DomainKeysSpec{
				HashingOverride: hashingOverride,
				DomainKeys: map[string]*DomainKeyInfo{
					"Claim": {
						KeyExpr:    []string{"remove_mi(name)", "gender", "format_date(dob)"},
						ObjectType: "Claim",
					},
				},
			},
		}

		trsfEvaluator, err := ctx.BuildHashTCEvaluator(
			inputChannel,
			&OutputChannel{
				Name:    "output",
				Columns: &outputColumns,
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
		for i, row := range inputValues {
			err = trsfEvaluator.Update(&currentOutputValue, &row)
			if err != nil {
				t.Errorf("while calling Update: %v", err)
			}
			// log.Printf("*** Got[%d]: %+v\n", i, currentOutputValue[0])
			if expectedKey != currentOutputValue[0] {
				t.Errorf("hash failed for row %d", i)
			}
		}
	}
	// t.Error("done")
	log.Println() //so the import of log is not optimized out
}

// Simulate compute domain key without domain key spec (using supplied composite key) with and without override
// Same as TestComputeDomainKeyFull03 but without domain key spec and using directly the column name in the hash expression.
// This is to test compute domain key without domain key spec.
func TestComputeDomainKeyFull04(t *testing.T) {
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
	// Evaluate the domain key
	currentOutputValue := make([]any, 1)
	var expectedKey string
	inputValues := [][]any{
		{"key1", "name 1", "m", "1969-01-01"},
		{"", "name 2", "M", "1969-01-01"},
	}
	for _, hashingOverride := range []string{"", "none"} {
		if hashingOverride == "none" {
			// do the override
			expectedKey = "NAME-M-19690101"
		} else {
			// take the default of sha1 hashing
			expectedKey = "a1366cc9-38bb-50aa-b6dc-9d91f0249039"
		}

		// Build the Column Transformation Evaluator
		inputChannel := &InputChannel{
			Name:    "input",
			Columns: &inputColumns,
			DomainKeySpec: &DomainKeysSpec{
				HashingOverride: hashingOverride,
				// DomainKeys: map[string]*DomainKeyInfo{
				// 	"Claim": {
				// 		KeyExpr:    []string{"remove_mi(name)", "gender", "format_date(dob)"},
				// 		ObjectType: "Claim",
				// 	},
				// },
			},
		}

		trsfEvaluator, err := ctx.BuildHashTCEvaluator(
			inputChannel,
			&OutputChannel{
				Name:    "output",
				Columns: &outputColumns,
			},
			&TransformationColumnSpec{
				Type: "hash",
				Name: "Claim:domain_key",
				HashExpr: &HashExpression{
					// DomainKey:        "Claim",
					CompositeExpr:    []string{"remove_mi(name)", "gender", "format_date(dob)"},
					ComputeDomainKey: true,
				},
			},
		)
		if err != nil {
			t.Errorf("while calling BuildHashTCEvaluator: %v", err)
		}
		for i, row := range inputValues {
			err = trsfEvaluator.Update(&currentOutputValue, &row)
			if err != nil {
				t.Errorf("while calling Update: %v", err)
			}
			// log.Printf("*** Got[%d]: %+v\n", i, currentOutputValue[0])
			if expectedKey != currentOutputValue[0] {
				t.Errorf("hash failed for row %d", i)
			}
		}
	}
	// t.Error("done")
	log.Println() //so the import of log is not optimized out
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
	fmt.Println("*** DomainKey:", domainKey)
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
	buf := &bytes.Buffer{}
	err = makeAlternateKey(buf, &pfnc, "", &[]any{nil, "name", "6-14-2024"})
	if err != nil {
		t.Errorf("while calling makeAlternateKey: %v", err)
	}
	v := buf.String()
	if v != "NAME20240614" {
		t.Errorf("error: expecting NAME20240614 got %v", v)
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

func TestEvalHash02(t *testing.T) {
	var v *uint64
	freq := make(map[uint64]int)
	for i := 0; i < 100; i++ {
		v = EvalHash(nil, 10)
		if v == nil {
			t.Fatal("error: got nil from EvalHash")
		}
		if *v >= 10 {
			t.Errorf("error: expecting [0,10) from EvalHash, got %d", *v)
		}
		c := freq[*v]
		freq[*v] = c + 1
	}
	for k, v := range freq {
		fmt.Printf("hash %d: %d\n", k, v)
	}
	// t.Error("Done")
}
