package compute_pipes

import (
	"fmt"
)

// TransformationColumnSpec Type case_expr
type caseExprColumnEval struct {
	outputPos int
	caseExpr []caseExprClause
	elseExpr *evalExpression
}
type caseExprClause struct {
	whenCase evalExpression
	thenCase evalExpression
}
func (ctx *caseExprColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *caseExprColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error caseExprColumnEval.update cannot have nil currentValue or input")
	}
	if ctx.caseExpr == nil {
		return fmt.Errorf("error caseExprColumnEval.update cannot have nil caseExpr")
	}	
	for i := range ctx.caseExpr {
		when, err := ctx.caseExpr[i].whenCase.eval(input)
		if err != nil {
			return fmt.Errorf("while evaluating case_expr when clause: %v", err)
		}
		if ToBool(when) {
			thenCase, err := ctx.caseExpr[i].thenCase.eval(input)
			if err != nil {
				return fmt.Errorf("while evaluating case_expr then clause: %v", err)
			}
			(*currentValue)[ctx.outputPos] = thenCase
			return nil
		}
	}
	// No match apply default if not null
	if ctx.elseExpr != nil {
		elseCase, err := (*ctx.elseExpr).eval(input)
		if err != nil {
			return fmt.Errorf("while evaluating case_expr else clause: %v", err)
		}
		(*currentValue)[ctx.outputPos] = elseCase
		return nil
	}
	// Got no match and no default clause
	(*currentValue)[ctx.outputPos] = nil
	return nil
}
func (ctx *caseExprColumnEval) done(currentValue *[]interface{}) error {
	return nil
}
func (ctx *BuilderContext) buildCaseExprEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*caseExprColumnEval, error) {
	if spec == nil || spec.CaseExpr == nil {
		return nil, fmt.Errorf("error: Type case_expr must have CaseExpr != nil")
	}

	caseExpr := make([]caseExprClause, len(spec.CaseExpr))
	for i := range spec.CaseExpr {
		whenCase, err := ctx.buildExprNodeEvaluator(source, outCh, &spec.CaseExpr[i].When)
		if err != nil {
			return nil, fmt.Errorf("while building when clause for item %d: %v",i, err)
		}
		thenCase, err := ctx.buildExprNodeEvaluator(source, outCh, &spec.CaseExpr[i].Then)
		if err != nil {
			return nil, fmt.Errorf("while building then clause for item %d: %v",i, err)
		}
		caseExpr[i] = caseExprClause{
			whenCase: whenCase,
			thenCase: thenCase,
		}
	}

	var  elseExpr evalExpression
	var err error
	if spec.ElseExpr != nil {
		elseExpr, err = ctx.buildExprNodeEvaluator(source, outCh, spec.ElseExpr)
		if err != nil {
			return nil, fmt.Errorf("while building else clause for case_expr: %v", err)
		}
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	return &caseExprColumnEval{
		outputPos: outputPos,
		caseExpr: caseExpr,
		elseExpr: &elseExpr,
	}, err
}

