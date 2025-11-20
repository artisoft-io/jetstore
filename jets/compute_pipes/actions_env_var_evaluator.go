package compute_pipes

import "fmt"

// This file contains function to determine env var defined at ConditionalPipeSpec level

type ConditionalEnvVarEvaluator struct {
	caseExpr []envVarExprClause
	elseExpr []*envVarExpression
}
type envVarExprClause struct {
	whenCase  evalExpression
	thenCases []*envVarExpression
}

// envVarExpression indicate which column by position
// is to be updated by the evalExpression
type envVarExpression struct {
	name string
	evalExpr  evalExpression
}


func ApplyConditionalEnvVars(envVars []ConditionalEnvVariable, env map[string]any) error {
	for _, spec := range envVars {
		evaluator, err := buildConditionalEnvVarEvaluator(&spec, env)
		if err != nil {
			return fmt.Errorf("while building conditional env var evaluator: %v", err)
		}
		err = evaluator.Update(env)
		if err != nil {
			return fmt.Errorf("while evaluating conditional env var: %v", err)
		}
	}
	return nil
}

func buildConditionalEnvVarEvaluator(spec *ConditionalEnvVariable, env map[string]any) (*ConditionalEnvVarEvaluator, error) {
	caseExpr := make([]envVarExprClause, len(spec.CaseExpr))
	exprBuilderContex := ExprBuilderContext(env)
	for i := range spec.CaseExpr {
		whenCase, err := exprBuilderContex.BuildExprNodeEvaluator("addl_env", nil, &spec.CaseExpr[i].When)
		if err != nil {
			return nil, fmt.Errorf("while building when clause for item %d: %v", i, err)
		}
		thenCases := make([]*envVarExpression, len(spec.CaseExpr[i].Then))
		for i, node := range spec.CaseExpr[i].Then {
			expr, err := exprBuilderContex.BuildExprNodeEvaluator("addl_env", nil, node)
			if err != nil {
				return nil, fmt.Errorf("while building then clause for item %d: %v", i, err)
			}
			if node.Name == "" {
				return nil, fmt.Errorf("error: missing env var name in then clause of addl_env case operator")
			}
			thenCases[i] = &envVarExpression{
				name: node.Name,
				evalExpr:  expr,
			}
		}
		caseExpr[i] = envVarExprClause{
			whenCase:  whenCase,
			thenCases: thenCases,
		}
	}

	elseExpr := make([]*envVarExpression, len(spec.ElseExpr))
	for i, node := range spec.ElseExpr {
		expr, err := exprBuilderContex.BuildExprNodeEvaluator("addl_env", nil, node)
		if err != nil {
			return nil, fmt.Errorf("while building else clause of addl_env case_expr: %v", err)
		}
		if node.Name == "" {
			return nil, fmt.Errorf("error: case operator is missing env var name in else clause")
		}
		elseExpr[i] = &envVarExpression{
			name: node.Name,
			evalExpr:  expr,
		}
	}
	return &ConditionalEnvVarEvaluator{
		caseExpr: caseExpr,
		elseExpr: elseExpr,
	}, nil
}

func (evaluator *ConditionalEnvVarEvaluator) Update(env map[string]any) error {
	for _, clause := range evaluator.caseExpr {
		whenVal, err := clause.whenCase.Eval(env)
		if err != nil {
			return fmt.Errorf("while evaluating when clause for addl_env: %v", err)
		}
		if ToBool(whenVal) {
			for _, thenCase := range clause.thenCases {
				val, err := thenCase.evalExpr.Eval(env)
				if err != nil {
					return fmt.Errorf("while evaluating then clause for addl_env: %v", err)
				}
				env[thenCase.name] = val
			}
			return nil
		}
	}
	// else expr
	for _, elseCase := range evaluator.elseExpr {
		val, err := elseCase.evalExpr.Eval(env)
		if err != nil {
			return fmt.Errorf("while evaluating else clause for addl_env: %v", err)
		}
		env[elseCase.name] = val
	}
	return nil
}
