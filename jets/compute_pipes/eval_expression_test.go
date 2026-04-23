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
		Type: "expr_proxy",
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
