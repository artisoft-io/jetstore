package compute_pipes

import (
	"testing"
	"time"
)

func TestOpEqual(t *testing.T) {
	oper := &opEqual{}
	v, err := oper.Eval(1, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(1, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
	v, err = oper.Eval(1, "1.00")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("1.00", 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("1.00", float64(1))
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("1", float64(1))
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
}

func TestOpLT(t *testing.T) {
	oper := &opLT{}
	v, err := oper.Eval(1, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(2, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
	v, err = oper.Eval(1.0, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(2.0, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval("1.0", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(2, "1.0")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
}

func TestOpLT_Dates(t *testing.T) {
	oper := &opLT{}
	tm := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v, err := oper.Eval(tm, "2021-01-01")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	tm = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	v, err = oper.Eval(tm, "2021-01-01")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
}

func TestInvalidTypesOpLT(t *testing.T) {
	oper := &opLT{}
	_, err := oper.Eval(1, "abc")
	if err == nil {
		t.Errorf("error: expecting error when comparing incompatible types")
	}
	_, err = oper.Eval("abc", 1)
	if err == nil {
		t.Errorf("error: expecting error when comparing incompatible types")
	}
}

func TestOpLE(t *testing.T) {
	oper := &opLE{}
	v, err := oper.Eval(1, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(1, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(2, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
	v, err = oper.Eval(1.0, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(1.0, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(2.0, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval("1.0", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("2.0", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("2.0", 2.0)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(2.0, 2.0)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(2, "1.0")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
}

func TestOpGT(t *testing.T) {
	oper := &opGT{}
	v, err := oper.Eval(2, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(1, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
	v, err = oper.Eval(2.0, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(1.0, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval("2.0", 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("1.6", 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval(1, "2.0")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval(2.5, "2.0")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
}

func TestOpGE(t *testing.T) {
	oper := &opGE{}
	v, err := oper.Eval(2, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(1, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval(1, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
	v, err = oper.Eval(2.0, 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(1.0, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval("2.0", 1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval("2.6", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(2, "2.6")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
	v, err = oper.Eval(1, "2.0")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false, got %v", v)
	}
	v, err = oper.Eval(2.5, "2.0")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true, got %v", v)
	}
}

func TestOpGE_Dates(t *testing.T) {
	oper := &opGE{}
	tm1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v, err := oper.Eval(tm1, "2020-01-01")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	v, err = oper.Eval("2020-01-01", tm1)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	tm2 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	v, err = oper.Eval(tm2, "2020-01-01")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
	tm3 := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	v, err = oper.Eval(tm3, "2020-01-01")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if ToBool(v) {
		t.Errorf("error: expecting false")
	}
	v, err = oper.Eval("2020-01-01", tm3)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !ToBool(v) {
		t.Errorf("error: expecting true")
	}
}

func TestOpADD(t *testing.T) {
	oper := &opADD{}
	v, err := oper.Eval(1, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v.(int) != 3 {
		t.Errorf("error: expecting 3")
	}
	v, err = oper.Eval(1.5, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v.(float64) != 3.5 {
		t.Errorf("error: expecting 3.5")
	}
	v, err = oper.Eval("1.5", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3 {
		t.Errorf("error: expecting 3, got %v", v)
	}
	v, err = oper.Eval(2, "1.5")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3 {
		t.Errorf("error: expecting 3, got %v", v)
	}
	v, err = oper.Eval("1.5", 2.0)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3.5 {
		t.Errorf("error: expecting 3.5, got %v", v)
	}
	v, err = oper.Eval(2.1, "1.5")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3.6 {
		t.Errorf("error: expecting 3.6, got %v", v)
	}
	v, err = oper.Eval("5", "2")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != "52" {
		t.Errorf("error: expecting \"52\", got %v", v)
	}
	v, err = oper.Eval("hello", " world")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != "hello world" {
		t.Errorf("error: expecting \"hello world\", got %v", v)
	}
}

func TestOpADD_Dates(t *testing.T) {
	oper := &opADD{}
	tm := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v, err := oper.Eval(tm, 30)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	tmExpected := time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC)
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
	v, err = oper.Eval(30, tm)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
	v, err = oper.Eval(int64(30), tm)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	} else {
		if !v.(time.Time).Equal(tmExpected) {
			t.Errorf("error: expecting %v, got %v", tmExpected, v)
		}
	}
	v, err = oper.Eval("30", tm)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
	v, err = oper.Eval(tm, "30")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
}

func TestOpSUB(t *testing.T) {
	oper := &opSUB{}
	v, err := oper.Eval(5, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3 {
		t.Errorf("error: expecting 3")
	}
	v, err = oper.Eval(5.5, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3.5 {
		t.Errorf("error: expecting 3.5")
	}
	v, err = oper.Eval("5.5", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3 {
		t.Errorf("error: expecting 3, got %v", v)
	}
	v, err = oper.Eval(5, "2.5")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 3 {
		t.Errorf("error: expecting 3, got %v", v)
	}
	v, err = oper.Eval(int64(5), "2.5")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != int64(3) {
		t.Errorf("error: expecting 3, got %v", v)
	}
	v, err = oper.Eval("2.5", int64(5))
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != int64(-3) {
		t.Errorf("error: expecting -3, got %v", v)
	}
	v, err = oper.Eval(5.0, "2.5")
	if err != nil {
		t.Errorf("error: expecting nil got %v", err)
	}
	if v != 2.5 {
		t.Errorf("error: expecting 2.5, got %v", v)
	}
}

func TestOpSUB_Dates(t *testing.T) {
	oper := &opSUB{}
	tm := time.Date(2020, 1, 31, 0, 0, 0, 0, time.UTC)
	v, err := oper.Eval(tm, 30)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	tmExpected := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
	v, err = oper.Eval(tm, "30")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
	v, err = oper.Eval("30", tm)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if !v.(time.Time).Equal(tmExpected) {
		t.Errorf("error: expecting %v, got %v", tmExpected, v)
	}
	v, err = oper.Eval(30, tm)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	} else {
		if !v.(time.Time).Equal(tmExpected) {
			t.Errorf("error: expecting %v, got %v", tmExpected, v)
		}
	}
	v, err = oper.Eval(int64(30), tm)
	if err != nil {
		t.Errorf("error: expecting nil, got %v", err)
	} else {
		if !v.(time.Time).Equal(tmExpected) {
			t.Errorf("error: expecting %v, got %v", tmExpected, v)
		}
	}
}

func TestOpMUL(t *testing.T) {
	oper := &opMUL{}
	v, err := oper.Eval(5, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 10 {
		t.Errorf("error: expecting 10")
	}
	v, err = oper.Eval(5.5, 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 11.0 {
		t.Errorf("error: expecting 11.0")
	}
	v, err = oper.Eval("5.5", 2)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 10 {
		t.Errorf("error: expecting 10, got %v", v)
	}
	v, err = oper.Eval("5.5", 2.0)
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 11.0 {
		t.Errorf("error: expecting 11.0, got %v", v)
	}
	v, err = oper.Eval("5.5", uint(2))
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != uint(10) {
		t.Errorf("error: expecting 10, got %v", v)
	}
	v, err = oper.Eval(5.0, "2.5")
	if err != nil {
		t.Errorf("error: expecting nil got %v", err)
	}
	if v != 12.5 {
		t.Errorf("error: expecting 12.5, got %v", v)
	}
	v, err = oper.Eval(5, "2")
	if err != nil {
		t.Errorf("error: expecting nil")
	}
	if v != 10 {
		t.Errorf("error: expecting 10, got %v", v)
	}
}

func TestOpMUL_InvalidTypes(t *testing.T) {
	oper := &opMUL{}
	_, err := oper.Eval("abc", 2)
	if err == nil {
		t.Errorf("error: expecting error when multiplying incompatible types")
	}
	_, err = oper.Eval(5, "abc")
	if err == nil {
		t.Errorf("error: expecting error when multiplying incompatible types")
	}
	_, err = oper.Eval(5.0, "abc")
	if err == nil {
		t.Errorf("error: expecting error when multiplying incompatible types")
	}
	_, err = oper.Eval("5.0", "1.0")
	if err == nil {
		t.Errorf("error: expecting error when multiplying incompatible types")
	}
}

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
	v, err := evalExpr.Eval(nil)
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
	v, err := evalExpr.Eval(nil)
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
	v, err := evalExpr.Eval(nil)
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
	v, err := evalExpr.Eval(nil)
	if err != nil {
		t.Errorf("error: expecting nil when evaluating expr")
	}
	if v != nil {
		t.Errorf("invalid result '%s', expecting nil", v)
	}
}
