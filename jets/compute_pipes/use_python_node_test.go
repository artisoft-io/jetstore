package compute_pipes

import (
	"strings"
	"testing"
)

// The tests for EvalUsePythonNode -- the step-spec pair use_python_node /
// use_python_node_when, which picks the Python cp_node over the Go worker for one
// reducing step.
//
// EvalUseEcsTask, the function this one mirrors, has no coverage at all on jets_ai, so
// these stand alone rather than extending a table. The claims under test are the five
// the phase plan's AA.8 names, plus two the 2x2 comment in build_cpipes_sm.go asserts
// and nothing had checked: that the two axes are independent, and that a step index past
// the end of conditional_pipes_config yields false rather than panicking.

// boolExpr is a constant expression node. ToBool reads the string "TRUE" as true
// (eval_operators.go, ToBool), so this is the shortest expression whose value is known
// without reading the environment -- which is what the override cases want to isolate.
// The quotes are load-bearing: parseValue treats an unquoted literal as a number and
// refuses TRUE with "expecting an int", so a constant expression is authored as 'TRUE'.
func boolExpr(v bool) *ExpressionNode {
	s := "'FALSE'"
	if v {
		s = "'TRUE'"
	}
	return &ExpressionNode{Type: "value", Expr: s}
}

// envExpr compares an env var against a literal, which is the shape the corpus actually
// authors for use_ecs_tasks_when ($TOTAL_FILE_SIZE_GB > 30.0). With columns == nil the
// select leaf reads its Expr as a key into EnvSettings at Eval time.
func envExpr(key, want string) *ExpressionNode {
	return &ExpressionNode{
		Lhs: &ExpressionNode{Type: "select", Expr: key},
		Op:  "==",
		Rhs: &ExpressionNode{Type: "value", Expr: "'" + want + "'"},
	}
}

// brokenExpr is a binary node with no rhs: BuildExprNodeEvaluator refuses it with
// "case node, must have lhs, rhs, and op != nil". It is the build failure the
// error-rather-than-false claim needs, and it fails at build time rather than at Eval.
func brokenExpr() *ExpressionNode {
	return &ExpressionNode{
		Lhs: &ExpressionNode{Type: "value", Expr: "'TRUE'"},
		Op:  "==",
	}
}

func startupWith(spec ConditionalPipeSpec, env map[string]any) *CpipesStartup {
	return &CpipesStartup{
		CpConfig: ComputePipesConfig{
			ConditionalPipesConfig: []ConditionalPipeSpec{spec},
		},
		EnvSettings: env,
	}
}

func TestEvalUsePythonNode(t *testing.T) {
	env := map[string]any{"$JETS_ENGINE": "python", "$OTHER": "go"}

	tests := []struct {
		name    string
		spec    ConditionalPipeSpec
		want    bool
		wantErr bool
	}{
		{
			name: "absent pair yields false",
			spec: ConditionalPipeSpec{StepName: "step0"},
			want: false,
		},
		{
			name: "the bool alone selects",
			spec: ConditionalPipeSpec{UsePythonNode: true},
			want: true,
		},
		{
			name: "the bool alone, false",
			spec: ConditionalPipeSpec{UsePythonNode: false},
			want: false,
		},
		{
			name: "the expression alone selects",
			spec: ConditionalPipeSpec{UsePythonNodeWhen: boolExpr(true)},
			want: true,
		},
		{
			name: "a true expression overrides a false bool",
			spec: ConditionalPipeSpec{UsePythonNode: false, UsePythonNodeWhen: boolExpr(true)},
			want: true,
		},
		{
			name: "a false expression overrides a true bool",
			spec: ConditionalPipeSpec{UsePythonNode: true, UsePythonNodeWhen: boolExpr(false)},
			want: false,
		},
		{
			name: "the expression reads EnvSettings, true",
			spec: ConditionalPipeSpec{UsePythonNodeWhen: envExpr("$JETS_ENGINE", "python")},
			want: true,
		},
		{
			name: "the expression reads EnvSettings, false, over a true bool",
			spec: ConditionalPipeSpec{UsePythonNode: true, UsePythonNodeWhen: envExpr("$OTHER", "python")},
			want: false,
		},
		{
			name:    "an expression that fails to build returns an error",
			spec:    ConditionalPipeSpec{UsePythonNode: true, UsePythonNodeWhen: brokenExpr()},
			want:    false,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := startupWith(tc.spec, env).EvalUsePythonNode(0)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expecting an error, got (%v, nil)", got)
				}
				// The claim is stronger than "an error happened": a misconfigured
				// expression must not look like "run this step on the Go worker",
				// and the bool it sits beside says true.
				if got {
					t.Errorf("expecting false alongside the error, got true")
				}
				return
			}
			if err != nil {
				t.Fatalf("expecting nil error, got %v", err)
			}
			if got != tc.want {
				t.Errorf("expecting %v, got %v", tc.want, got)
			}
		})
	}
}

// TestEvalUsePythonNodeStepOutOfRange: the guard is len(pipeSpec) > stepId, and a reducing
// iteration past the last conditional step is a real state (NoMoreTask is computed from it).
func TestEvalUsePythonNodeStepOutOfRange(t *testing.T) {
	cpipesStartup := startupWith(ConditionalPipeSpec{UsePythonNode: true}, nil)
	got, err := cpipesStartup.EvalUsePythonNode(7)
	if err != nil {
		t.Fatalf("expecting nil error, got %v", err)
	}
	if got {
		t.Errorf("expecting false for a step past the end, got true")
	}
}

// TestEvalUsePythonNodeIsIndependentOfEcs is the 2x2 the comment in
// cdk/jetstore_one/stack/build_cpipes_sm.go asserts, checked rather than asserted: the two
// flags are two axes, so one step can author both and each evaluator reads only its own
// pair. Without this, "an arm rather than a redesign" rests on reading the code.
func TestEvalUsePythonNodeIsIndependentOfEcs(t *testing.T) {
	for _, tc := range []struct {
		name                string
		ecs, python         bool
		wantEcs, wantPython bool
	}{
		{"neither", false, false, false, false},
		{"ecs only", true, false, true, false},
		{"python only", false, true, false, true},
		{"both -- the fourth quadrant, legal to author and not yet built", true, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cpipesStartup := startupWith(ConditionalPipeSpec{
				UseEcsTasks:   tc.ecs,
				UsePythonNode: tc.python,
			}, nil)
			gotEcs, err := cpipesStartup.EvalUseEcsTask(0)
			if err != nil {
				t.Fatalf("EvalUseEcsTask: expecting nil error, got %v", err)
			}
			gotPython, err := cpipesStartup.EvalUsePythonNode(0)
			if err != nil {
				t.Fatalf("EvalUsePythonNode: expecting nil error, got %v", err)
			}
			if gotEcs != tc.wantEcs || gotPython != tc.wantPython {
				t.Errorf("expecting (ecs=%v, python=%v), got (ecs=%v, python=%v)",
					tc.wantEcs, tc.wantPython, gotEcs, gotPython)
			}
		})
	}
}

// TestEvalUsePythonNodeErrorNamesItself: the two evaluators are copies of each other and the
// call sites wrap their errors by hand, so an error from one arriving under the other's name
// is a live confusion rather than a hypothetical. The expression is built under the name
// "use_python_node", and that is what the message must carry.
func TestEvalUsePythonNodeErrorNamesItself(t *testing.T) {
	_, err := startupWith(ConditionalPipeSpec{UsePythonNodeWhen: brokenExpr()}, nil).EvalUsePythonNode(0)
	if err == nil {
		t.Fatal("expecting an error")
	}
	if strings.Contains(err.Error(), "use_ecs_tasks") {
		t.Errorf("the error names the ECS axis: %v", err)
	}
}
