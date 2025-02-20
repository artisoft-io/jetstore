package compute_pipes

import (
	"fmt"
)

// build the runtime evaluator for the column transformation
func (ctx *BuilderContext) BuildTransformationColumnEvaluator(source *InputChannel, outCh *OutputChannel, spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	switch spec.Type {
	// select, multi_select, value, eval, map, count, distinct_count, sum, min, case, hash, map_reduce, lookup
	case "select":
		if spec.Expr == nil {
			return nil, fmt.Errorf("error: Type select must have Expr != nil")
		}
		inputPos, ok := (*source.columns)[*spec.Expr]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.name)
		}
		outputPos, ok := (*outCh.columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.name)
		}
		return &selectColumnEval{
			inputPos:  inputPos,
			outputPos: outputPos,
		}, nil

	case "multi_select":
		if len(spec.ExprArray) == 0 {
			return nil, fmt.Errorf("error: Type multi_select must have expr_array specified")
		}
		inputPos := make([]int, 0, len(spec.ExprArray))
		for _, columnName := range spec.ExprArray {
			inputPos = append(inputPos, (*source.columns)[columnName])
		}
		outputPos, ok := (*outCh.columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.name)
		}
		return &multiSelectColumnEval{
			inputPos:  inputPos,
			outputPos: outputPos,
		}, nil

	case "value":
		if spec.Expr == nil {
			return nil, fmt.Errorf("error: Type value must have Expr != nil")
		}
		value, err := ctx.parseValue(spec.Expr)
		if err != nil {
			return nil, err
		}
		outputPos, ok := (*outCh.columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.name)
		}
		return &valueColumnEval{
			value:     value,
			outputPos: outputPos,
		}, nil

	case "eval":
		evalEpr, err := ctx.BuildExprNodeEvaluator(source.name, *source.columns, spec.EvalExpr)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildExprNodeEvaluator: %v", err)
		}
		outputPos, ok := (*outCh.columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.name)
		}
		return &evalExprColumnEval{
			expr:      evalEpr,
			outputPos: outputPos,
		}, nil

	case "map":
		mapEvaluator, err := ctx.BuildMapTCEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildMapTCEvaluator: %v", err)
		}
		return mapEvaluator, nil

	case "count":
		countEvaluator, err := ctx.BuildCountTCEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildCountTCEvaluator: %v", err)
		}
		return countEvaluator, nil

	case "distinct_count":
		distinctCountEvaluator, err := ctx.BuildDistinctCountTCEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildDistinctCountTCEvaluator: %v", err)
		}
		return distinctCountEvaluator, nil

	case "sum":
		sumEvaluator, err := ctx.BuildSumTCEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildSumTCEvaluator: %v", err)
		}
		return sumEvaluator, nil

	case "min":
		minEvaluator, err := ctx.BuildMinTCEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildMinTCEvaluator: %v", err)
		}
		return minEvaluator, nil

	case "case":
		return ctx.BuildCaseExprTCEvaluator(source, outCh, spec)

	case "hash":
		return ctx.BuildHashTCEvaluator(source, outCh, spec)

	case "map_reduce":
		return ctx.BuildMapReduceTCEvaluator(source, outCh, spec)

	case "lookup":
		return ctx.BuildLookupTCEvaluator(source, outCh, spec)
	}
	return nil, fmt.Errorf("error: unknown TransformationColumnSpec Type: %v", spec.Type)
}

// TransformationColumnSpec Type eval
type evalExprColumnEval struct {
	expr      evalExpression
	outputPos int
}

func (ctx *evalExprColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *evalExprColumnEval) Update(currentValue *[]interface{}, input *[]interface{}) error {
	value, err := ctx.expr.eval(*input)
	if err != nil {
		return err
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}
func (ctx *evalExprColumnEval) Done(currentValue *[]interface{}) error {
	return nil
}

// TransformationColumnSpec Type value
type valueColumnEval struct {
	value     interface{}
	outputPos int
}

func (ctx *valueColumnEval) Done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *valueColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *valueColumnEval) Update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error valueColumnEval.update cannot have nil currentValue or input")
	}
	(*currentValue)[ctx.outputPos] = ctx.value
	return nil
}

// TransformationColumnSpec Type select
type selectColumnEval struct {
	inputPos  int
	outputPos int
}

func (ctx *selectColumnEval) Done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *selectColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *selectColumnEval) Update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error selectColumnEval.update cannot have nil currentValue or input")
	}
	(*currentValue)[ctx.outputPos] = (*input)[ctx.inputPos]
	return nil
}

// TransformationColumnSpec Type multi_select
type multiSelectColumnEval struct {
	inputPos  []int
	outputPos int
}

func (ctx *multiSelectColumnEval) Done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *multiSelectColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *multiSelectColumnEval) Update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error selectColumnEval.update cannot have nil currentValue or input")
	}
	value := make([]any, 0, len(ctx.inputPos))
	for _, ipos := range ctx.inputPos {
		value = append(value, (*input)[ipos])
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}
