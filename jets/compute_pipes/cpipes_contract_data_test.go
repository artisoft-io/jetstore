package compute_pipes

import (
	"slices"
	"testing"
)

// The generated contract table is the applicability matrix as data; these are
// spot checks that the emission carries the facts the matrix settled, so a
// broken regeneration fails here rather than in a consumer.
func TestCpipesContractData(t *testing.T) {
	if len(CpipesContract) < 100 {
		t.Fatalf("contract table suspiciously small: %d tokens", len(CpipesContract))
	}
	jr, ok := CpipesContract["TransformationSpec/jetrules"]
	if !ok {
		t.Fatal("TransformationSpec/jetrules missing")
	}
	if !jr["jetrules_config"].Required {
		t.Error("jetrules_config must be required on the jetrules token")
	}
	if _, ok := jr["anonymize_config"]; ok {
		t.Error("anonymize_config is inapplicable on the jetrules token")
	}
	sql := CpipesContract["OutputChannelConfig/sql"]
	if !sql["output_table_key"].Required {
		t.Error("output_table_key must be required on the sql output channel")
	}
	if _, ok := sql["format"]; ok {
		t.Error("format is inapplicable on the sql output channel")
	}
	sel := CpipesContract["ExpressionNode/select"]
	if sel["expr"].RequiredWhen != "absent(expr_pos)" {
		t.Errorf("expr's conditional requirement lost: %q", sel["expr"].RequiredWhen)
	}
	stage := CpipesContract["OutputChannelConfig/stage"]
	if !slices.Contains(stage["format"].Values, "parquet") {
		t.Errorf("stage format range lost parquet: %v", stage["format"].Values)
	}
}
