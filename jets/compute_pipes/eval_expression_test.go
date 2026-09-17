package compute_pipes

import (
	"encoding/json"
	"testing"
)

func TestBuildExprNodeEvaluator01(t *testing.T) {

	// The expression to evaluate
	exprAst := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "select",
			Expr: "col1",
		},
		Op: "IN",
		Rhs: &ExpressionNode{
			Type: "static_list",
			ExprList: []string{
				"'string'",
				"'binary'",
				"'date'",
			},
		},
	}
	// save it as json
	var exprJson string
	b, err := json.Marshal(exprAst)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	exprJson = string(b)

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	m["${expr}"] = exprJson
	ctx := ExprBuilderContext(m)

	// Create an expression with a proxy to the expression json
	spec := &ExpressionNode{
		Type:            "expr_proxy",
		ExprEnvVarProxy: "${expr}",
	}
	// Build the expression evaluator
	columns := map[string]int{
		"col1": 0,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{"string"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(value) {
		t.Errorf("error: expecting true, got %v", value)
	}
	row = []any{"other"}
	value, err = eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(value) {
		t.Errorf("error: expecting false, got %v", value)
	}
}

func TestBuildExprNodeEvaluator02(t *testing.T) {

	// The expression to evaluate
	exprAst := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "select",
			Expr: "col1",
		},
		Op: "in_no_case",
		Rhs: &ExpressionNode{
			Type: "static_list",
			ExprList: []string{
				"'string'",
				"'binary'",
				"'date'",
			},
		},
	}
	// save it as json
	var exprJson string
	b, err := json.Marshal(exprAst)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	exprJson = string(b)

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	m["${expr}"] = exprJson
	ctx := ExprBuilderContext(m)

	// Create an expression with a proxy to the expression json
	spec := &ExpressionNode{
		Type:            "expr_proxy",
		ExprEnvVarProxy: "${expr}",
	}
	// Build the expression evaluator
	columns := map[string]int{
		"col1": 0,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{"String"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(value) {
		t.Errorf("error: expecting true, got %v", value)
	}
	row = []any{"other"}
	value, err = eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(value) {
		t.Errorf("error: expecting false, got %v", value)
	}
}

func TestBuildExprNodeEvaluator03(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "select",
			Expr: "Y0",
		},
		Op: ">",
		Rhs: &ExpressionNode{
			Type: "function",
			Expr: "current_year",
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{2099}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(value) {
		t.Errorf("error: expecting true, got %v", value)
	}
	row = []any{2000}
	value, err = eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(value) {
		t.Errorf("error: expecting false, got %v", value)
	}
}

func TestBuildExprNodeEvaluator04(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "select",
			Expr: "Y0",
		},
		Op: ">",
		Rhs: &ExpressionNode{
			Type: "function",
			Expr: "current_year",
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{"2099"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(value) {
		t.Errorf("error: expecting true, got %v", value)
	}
	row = []any{"2000"}
	value, err = eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(value) {
		t.Errorf("error: expecting false, got %v", value)
	}
}

func TestBuildExprNodeEvaluator05(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "select",
			Expr: "Y0",
		},
		Op: ">",
		Rhs: &ExpressionNode{
			Type: "function",
			Expr: "current_year",
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{"something else"}
	_, err = eval.Eval(row)
	if err == nil {
		t.Errorf("error: expecting error, got nil")
	}
}

func TestBuildExprNodeEvaluator05A(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "select",
			Expr: "Y0",
		},
		Op: ">",
		Rhs: &ExpressionNode{
			Type: "function",
			Expr: "current_year",
		},
		Default: &ExpressionNode{
			Type: "value",
			Expr: "0",
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{"something else"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	if ToBool(value) {
		t.Errorf("error: expecting false, got %v", value)
	}
}

func TestBuildExprNodeEvaluator06(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Lhs: &ExpressionNode{
				Type: "select",
				Expr: "Y0",
			},
			Op: ">",
			Rhs: &ExpressionNode{
				Type: "value",
				Expr: "10",
			},
		},
		Op: "and",
		Rhs: &ExpressionNode{
			Lhs: &ExpressionNode{
				Type: "select",
				Expr: "Y1",
			},
			Op: ">",
			Rhs: &ExpressionNode{
				Type: "function",
				Expr: "current_year",
			},
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
		"Y1": 1,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	// Evaluate the expression with a value for col1
	row := []any{"0", "something"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	if ToBool(value) {
		t.Errorf("error: expecting false, got %v", value)
	}

	row = []any{"11", "something else"}
	_, err = eval.Eval(row)
	if err == nil {
		t.Errorf("error: expecting error, got nil")
	}

	row = []any{"11", "2099"}
	value, err = eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	if !ToBool(value) {
		t.Errorf("error: expecting true, got %v", value)
	}
}

func TestBuildExprNodeEvaluator07(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Type: "select",
		Expr: "XX",
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
		"Y1": 1,
	}
	_, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err == nil {
		t.Errorf("error: expecting error, got nil")
	}
}

func TestBuildExprNodeEvaluator07A(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Lhs: &ExpressionNode{
				Lhs: &ExpressionNode{
					Type: "select",
					Expr: "min_paid_date",
				},
				Op: "+",
				Rhs: &ExpressionNode{
					Type: "value",
					Expr: "'-01'",
				},
			},
			Op: "distance_months",
			Rhs: &ExpressionNode{
				Lhs: &ExpressionNode{
					Type: "select",
					Expr: "max_paid_date",
				},
				Op: "+",
				Rhs: &ExpressionNode{
					Type: "value",
					Expr: "'-01'",
				},
			},
		},
		Op: "==",
		Rhs: &ExpressionNode{
			Lhs: &ExpressionNode{
				Type: "select",
				Expr: "distinct_paid_date_count",
				AsRdfType: "int",
			},
			Op: "-",
			Rhs: &ExpressionNode{
				Type: "value",
				Expr: "1",
			},
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"min_paid_date":            0,
		"max_paid_date":            1,
		"distinct_paid_date_count": 2,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	// Evaluate the expression with a value for col1
	row := []any{"2020-01", "2020-02", "2"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	if !ToBool(value) {
		t.Errorf("error: expecting true, got %v", value)
	}
}

func TestBuildExprNodeEvaluator10(t *testing.T) {

	// Create an expression builder context with the expression json as argument
	m := make(map[string]any)
	ctx := ExprBuilderContext(m)

	// The expression to evaluate
	spec := &ExpressionNode{
		Type: "select",
		Expr: "XX",
		Default: &ExpressionNode{
			Type: "select",
			Expr: "Y0",
		},
	}
	// Build the expression evaluator
	columns := map[string]int{
		"Y0": 0,
		"Y1": 1,
	}
	eval, err := ctx.BuildExprNodeEvaluator("source1", columns, spec)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	// Evaluate the expression with a value for col1
	row := []any{"77", "something"}
	value, err := eval.Eval(row)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	}
	if value != "77" {
		t.Errorf("error: expecting 77, got %v", value)
	}
}

// TestBuildExprNodeEvaluatorContains drives the contains operator through the whole expression
// layer, in the exact shape a .pc.json writes it:
//
//	{"lhs": {"type":"select","expr":"field_id"}, "op":"contains", "rhs": {"type":"value","expr":"'Hedis'"}}
//
// TestBuildEvalOperatorContains covers the registration; this one covers what a configuration author
// actually writes -- the select by column name, the quoted string literal on the rhs, and the
// operator name spelled in lower case, which BuildEvalOperator upper-cases before dispatching.
func TestBuildExprNodeEvaluatorContains(t *testing.T) {

	tests := []struct {
		name    string
		op      string
		rhs     string
		fieldId string
		want    bool
	}{
		{"contains, discriminator present", "contains", "'Hedis'", "S1-Hedis-E01", true},
		{"contains, discriminator absent", "contains", "'Hedis'", "S1-E01", false},
		{"contains, HCC discriminator", "contains", "'HCC'", "S1-HCC-M02", true},
		{"contains is case sensitive", "contains", "'Hedis'", "S1-HEDIS-E01", false},
		{"contains_no_case is not", "contains_no_case", "'Hedis'", "S1-HEDIS-E01", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &ExpressionNode{
				Lhs: &ExpressionNode{
					Type: "select",
					Expr: "field_id",
				},
				Op: tt.op,
				Rhs: &ExpressionNode{
					Type: "value",
					Expr: tt.rhs,
				},
			}
			// Round-trip through json, the way a .pc.json reaches the builder.
			b, err := json.Marshal(spec)
			if err != nil {
				t.Fatalf("error marshalling the expression: %v", err)
			}
			var parsed ExpressionNode
			if err = json.Unmarshal(b, &parsed); err != nil {
				t.Fatalf("error unmarshalling the expression: %v", err)
			}
			ctx := ExprBuilderContext(make(map[string]any))
			eval, err := ctx.BuildExprNodeEvaluator("qc_metrics.mapped", map[string]int{"field_id": 0}, &parsed)
			if err != nil {
				t.Fatalf("error building the expression evaluator: %v", err)
			}
			value, err := eval.Eval([]any{tt.fieldId})
			if err != nil {
				t.Fatalf("error evaluating the expression: %v", err)
			}
			if ToBool(value) != tt.want {
				t.Errorf("%s contains %s with field_id %q = %v, want %v", tt.op, tt.rhs, tt.fieldId, value, tt.want)
			}
		})
	}
}
