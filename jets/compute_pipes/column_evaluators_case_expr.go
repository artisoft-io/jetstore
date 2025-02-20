package compute_pipes

import (
	"fmt"
)

// TransformationColumnSpec Type case
type caseExprEvaluator struct {
	caseExpr []caseExprClause
	elseExpr []*columnExpression
}
type caseExprClause struct {
	whenCase  evalExpression
	thenCases []*columnExpression
}

// columnExpression indicate which column by position
// is to be updated by the evalExpression
type columnExpression struct {
	outputPos int
	evalExpr  evalExpression
}

func (ctx *caseExprEvaluator) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *caseExprEvaluator) Update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error caseExprEvaluator.update cannot have nil currentValue or input")
	}
	if ctx.caseExpr == nil {
		return fmt.Errorf("error caseExprEvaluator.update cannot have nil caseExpr")
	}
	for i := range ctx.caseExpr {
		when, err := ctx.caseExpr[i].whenCase.eval(*input)
		if err != nil {
			return fmt.Errorf("while evaluating case_expr when clause: %v", err)
		}
		if ToBool(when) {
			for _, node := range ctx.caseExpr[i].thenCases {
				value, err := node.evalExpr.eval(*input)
				if err != nil {
					return fmt.Errorf("while evaluating case_expr then clause: %v", err)
				}
				(*currentValue)[node.outputPos] = value
			}
			return nil
		}
	}
	// No match apply default if provided
	for _, node := range ctx.elseExpr {
		value, err := node.evalExpr.eval(*input)
		if err != nil {
			return fmt.Errorf("while evaluating case_expr then clause: %v", err)
		}
		(*currentValue)[node.outputPos] = value
	}
	return nil
}
func (ctx *caseExprEvaluator) Done(currentValue *[]interface{}) error {
	return nil
}
func (ctx *BuilderContext) BuildCaseExprTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.CaseExpr == nil {
		return nil, fmt.Errorf("error: Type case_expr must have CaseExpr != nil")
	}

	caseExpr := make([]caseExprClause, len(spec.CaseExpr))
	for i := range spec.CaseExpr {
		whenCase, err := ctx.BuildExprNodeEvaluator(source.name, *source.columns, &spec.CaseExpr[i].When)
		if err != nil {
			return nil, fmt.Errorf("while building when clause for item %d: %v", i, err)
		}
		thenCases := make([]*columnExpression, len(spec.CaseExpr[i].Then))
		for i, node := range spec.CaseExpr[i].Then {
			expr, err := ctx.BuildExprNodeEvaluator(source.name, *source.columns, node)
			if err != nil {
				return nil, fmt.Errorf("while building then clause for item %d: %v", i, err)
			}
			if node.Name == "" {
				return nil, fmt.Errorf("error: case operator is missing column name in then clause")
			}
			outputPos, ok := (*outCh.columns)[node.Name]
			if !ok {
				return nil, fmt.Errorf("error column %s not found in output source %s", node.Name, outCh.name)
			}
			thenCases[i] = &columnExpression{
				outputPos: outputPos,
				evalExpr:  expr,
			}
		}
		caseExpr[i] = caseExprClause{
			whenCase:  whenCase,
			thenCases: thenCases,
		}
	}

	elseExpr := make([]*columnExpression, len(spec.ElseExpr))
	for i, node := range spec.ElseExpr {
		expr, err := ctx.BuildExprNodeEvaluator(source.name, *source.columns, node)
		if err != nil {
			return nil, fmt.Errorf("while building else clause for case_expr: %v", err)
		}
		if node.Name == "" {
			return nil, fmt.Errorf("error: case operator is missing column name in else clause")
		}
		outputPos, ok := (*outCh.columns)[node.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", node.Name, outCh.name)
		}
		elseExpr[i] = &columnExpression{
			outputPos: outputPos,
			evalExpr:  expr,
		}
	}
	return &caseExprEvaluator{
		caseExpr: caseExpr,
		elseExpr: elseExpr,
	}, nil
}
