package compute_pipes

import (
	"fmt"
)

// TransformationColumnSpec Type case
type caseExprEvaluator struct {
	caseExpr []caseExprClause
	elseExpr []TransformationColumnEvaluator
}
type caseExprClause struct {
	whenCase  evalExpression
	thenCases []TransformationColumnEvaluator
}

// columnExpression indicate which column by position
// is to be updated by the evalExpression
type columnExpression struct {
	outputPos int
	evalExpr  evalExpression
}

func (ctx *caseExprEvaluator) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error caseExprEvaluator.update cannot have nil currentValue or input")
	}
	if ctx.caseExpr == nil {
		return fmt.Errorf("error caseExprEvaluator.update cannot have nil caseExpr")
	}
	for i := range ctx.caseExpr {
		when, err := ctx.caseExpr[i].whenCase.Eval(*input)
		if err != nil {
			return fmt.Errorf("while evaluating case_expr when clause #%d: %v", i, err)
		}
		if ToBool(when) {
			for _, node := range ctx.caseExpr[i].thenCases {
				err := node.Update(currentValue, input)
				if err != nil {
					return fmt.Errorf("while evaluating case_expr then clause #%d: %v", i, err)
				}
			}
			return nil
		}
	}
	// No match apply default if provided
	for _, node := range ctx.elseExpr {
		err := node.Update(currentValue, input)
		if err != nil {
			return fmt.Errorf("while evaluating case_expr else clause: %v", err)
		}
	}
	return nil
}
func (ctx *caseExprEvaluator) Done(currentValue *[]any) error {
	return nil
}
func (ctx *BuilderContext) BuildCaseExprTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.CaseExpr == nil {
		return nil, fmt.Errorf("error: Type case_expr must have CaseExpr != nil")
	}

	caseExpr := make([]caseExprClause, len(spec.CaseExpr))
	for i := range spec.CaseExpr {
		whenCase, err := ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, &spec.CaseExpr[i].When)
		if err != nil {
			return nil, fmt.Errorf("while building when clause for item %d: %v", i, err)
		}
		thenCases := make([]TransformationColumnEvaluator, len(spec.CaseExpr[i].Then))
		for i, tcSpec := range spec.CaseExpr[i].Then {
			expr, err := ctx.BuildTransformationColumnEvaluator(source, outCh, tcSpec)
			if err != nil {
				return nil, fmt.Errorf("while building then clause for item %d: %v", i, err)
			}
			thenCases[i] = expr
		}
		caseExpr[i] = caseExprClause{
			whenCase:  whenCase,
			thenCases: thenCases,
		}
	}

	elseExpr := make([]TransformationColumnEvaluator, len(spec.ElseExpr))
	for i, tcSpec := range spec.ElseExpr {
		expr, err := ctx.BuildTransformationColumnEvaluator(source, outCh, tcSpec)
		if err != nil {
			return nil, fmt.Errorf("while building else clause for case_expr: %v", err)
		}
		elseExpr[i] = expr
	}
	return &caseExprEvaluator{
		caseExpr: caseExpr,
		elseExpr: elseExpr,
	}, nil
}
