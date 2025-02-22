package compute_pipes

import (
	"testing"
)

func TestInStaticList1(t *testing.T) {
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "value",
			Expr: "'ding'",
		},
		Op: "in",
		Rhs: &ExpressionNode{
			Type: "static_list",
			ExprList: []string{
				"'ding'",
				"'dong'",
			},
		},
	}
	var ctx *BuilderContext
	evalExpr, err := ctx.BuildExprNodeEvaluator("", nil, spec)
	if err != nil {
		t.Fatalf("error: expecting nil")
	}
	v, err := evalExpr.eval(nil)
	if err != nil {
		t.Errorf("error: expecting nil when evaluating expr")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting in static_list")
	}
}

func TestInStaticList2(t *testing.T) {
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "value",
			Expr: "'pong'",
		},
		Op: "in",
		Rhs: &ExpressionNode{
			Type: "static_list",
			ExprList: []string{
				"'ding'",
				"'dong'",
			},
		},
	}
	var ctx *BuilderContext
	evalExpr, err := ctx.BuildExprNodeEvaluator("", nil, spec)
	if err != nil {
		t.Fatalf("error: expecting nil")

	}
	v, err := evalExpr.eval(nil)
	if err != nil {
		t.Errorf("error: expecting nil when evaluating expr")
	}
	if ToBool(v) {
		t.Errorf("error: NOT expecting in static_list")
	}
}

func TestInStaticList3(t *testing.T) {
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "value",
			Expr: "'pong'",
		},
		Op: "in",
		Rhs: &ExpressionNode{
			Type: "value",
			Expr: "'pong'",
		},
	}
	var ctx *BuilderContext
	_, err := ctx.BuildExprNodeEvaluator("", nil, spec)
	if err == nil {
		t.Fatalf("error: NOT expecting nil")
	}
}

func TestApplyRegex1(t *testing.T) {
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "value",
			Expr: "'123456789'",
		},
		Op: "apply_regex",
		Rhs: &ExpressionNode{
			Type: "value",
			Expr: "'^\\d{9}$'",
		},
	}
	var ctx *BuilderContext
	evalExpr, err := ctx.BuildExprNodeEvaluator("", nil, spec)
	if err != nil {
		t.Fatalf("error: expecting nil")
	}
	v, err := evalExpr.eval(nil)
	if err != nil {
		t.Errorf("error: expecting nil when evaluating expr")
	}
	vv, ok := v.(string)
	if !ok {
		t.Fatalf("error: expecting a string, got %v", v)
	}
	if vv != "123456789" {
		t.Errorf("invalid result '%s' from operator", vv)
	}
}

func TestApplyRegex2(t *testing.T) {
	spec := &ExpressionNode{
		Lhs: &ExpressionNode{
			Type: "value",
			Expr: "'12345a789'",
		},
		Op: "apply_regex",
		Rhs: &ExpressionNode{
			Type: "value",
			Expr: "'^\\d{9}$'",
		},
	}
	var ctx *BuilderContext
	evalExpr, err := ctx.BuildExprNodeEvaluator("", nil, spec)
	if err != nil {
		t.Fatalf("error: expecting nil")
	}
	v, err := evalExpr.eval(nil)
	if err != nil {
		t.Errorf("error: expecting nil when evaluating expr")
	}
	if v != nil {
		t.Errorf("invalid result '%s', expecting nil", v)
	}
}
