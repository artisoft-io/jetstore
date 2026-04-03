package compute_pipes

import (
	"fmt"
	"strings"
)

// build the runtime evaluator for the column transformation
func (ctx *BuilderContext) BuildTransformationColumnEvaluator(source *InputChannel, outCh *OutputChannel, spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	switch strings.ToLower(spec.Type) {
	// select, multi_select, value, eval, map, count, distinct_count, sum, min, case, hash, map_reduce, lookup
	case "select":
		if spec.Expr == nil {
			return nil, fmt.Errorf("error: Type select must have Expr != nil")
		}
		inputPos, ok := (*source.Columns)[*spec.Expr]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.Name)
		}
		outputPos, ok := (*outCh.Columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
		}
		var cast2RdfType *CastToRdfFnc
		if spec.AsRdfType != "" {
			cast2RdfType = NewCastToRdfFnc(spec.Name, spec.AsRdfType, new(false))
		}
		return &selectColumnEval{
			inputPos:     inputPos,
			outputPos:    outputPos,
			cast2RdfType: cast2RdfType,
		}, nil

	case "multi_select":
		if len(spec.ExprArray) == 0 {
			return nil, fmt.Errorf("error: Type multi_select must have expr_array specified")
		}
		inputPos := make([]int, 0, len(spec.ExprArray))
		for _, columnName := range spec.ExprArray {
			inputPos = append(inputPos, (*source.Columns)[columnName])
		}
		outputPos, ok := (*outCh.Columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
		}
		var cast2RdfType *CastToRdfFnc
		if spec.AsRdfType != "" {
			cast2RdfType = NewCastToRdfFnc(spec.Name, spec.AsRdfType, new(false))
		}
		return &multiSelectColumnEval{
			inputPos:     inputPos,
			outputPos:    outputPos,
			cast2RdfType: cast2RdfType,
		}, nil

	case "value":
		if spec.Expr == nil {
			return nil, fmt.Errorf("error: Type value must have Expr != nil")
		}
		value, err := ctx.parseValue(spec.Expr)
		if err != nil {
			return nil, err
		}
		outputPos, ok := (*outCh.Columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
		}
		var cast2RdfType *CastToRdfFnc
		if spec.AsRdfType != "" {
			cast2RdfType = NewCastToRdfFnc(spec.Name, spec.AsRdfType, new(false))
		}
		return &valueColumnEval{
			value:        value,
			outputPos:    outputPos,
			cast2RdfType: cast2RdfType,
		}, nil

	case "eval":
		evalEpr, err := ctx.BuildExprNodeEvaluator(source.Name, *source.Columns, spec.EvalExpr)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildExprNodeEvaluator: %v", err)
		}
		outputPos, ok := (*outCh.Columns)[spec.Name]
		if !ok {
			return nil, fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.Name)
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
		minEvaluator, err := ctx.BuildMinMaxTCEvaluator(source, outCh, spec, true)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildMinMaxTCEvaluator: %v", err)
		}
		return minEvaluator, nil

	case "max":
		maxEvaluator, err := ctx.BuildMinMaxTCEvaluator(source, outCh, spec, false)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildMinMaxTCEvaluator: %v", err)
		}
		return maxEvaluator, nil

	case "avrg":
		avrgEvaluator, err := ctx.BuildAvrgTCEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling BuildAvrgTCEvaluator: %v", err)
		}
		return avrgEvaluator, nil

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

func (ctx *evalExprColumnEval) Update(currentValue *[]any, input *[]any) error {
	value, err := ctx.expr.Eval(*input)
	if err != nil {
		return err
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}
func (ctx *evalExprColumnEval) Done(currentValue *[]any) error {
	return nil
}

// TransformationColumnSpec Type value
type valueColumnEval struct {
	value        any
	outputPos    int
	cast2RdfType *CastToRdfFnc
}

func (ctx *valueColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *valueColumnEval) Update(currentValue *[]any, _ *[]any) error {
	if currentValue == nil {
		return fmt.Errorf("error valueColumnEval.update cannot have nil currentValue")
	}
	value := ctx.value
	if ctx.cast2RdfType != nil {
		var err error
		value, err = ctx.cast2RdfType.Cast(value)
		if err != nil {
			return err
		}
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}

// TransformationColumnSpec Type select
type selectColumnEval struct {
	inputPos     int
	outputPos    int
	cast2RdfType *CastToRdfFnc
}

func (ctx *selectColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *selectColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error selectColumnEval.update cannot have nil currentValue or input")
	}
	value := (*input)[ctx.inputPos]
	if ctx.cast2RdfType != nil {
		var err error
		value, err = ctx.cast2RdfType.Cast(value)
		if err != nil {
			return err
		}
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}

// TransformationColumnSpec Type multi_select
type multiSelectColumnEval struct {
	inputPos     []int
	outputPos    int
	cast2RdfType *CastToRdfFnc
}

func (ctx *multiSelectColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *multiSelectColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error multiSelectColumnEval.update cannot have nil currentValue or input")
	}
	value := make([]any, 0, len(ctx.inputPos))
	for _, ipos := range ctx.inputPos {
		v := (*input)[ipos]
		if ctx.cast2RdfType != nil {
			var err error
			v, err = ctx.cast2RdfType.Cast(v)
			if err != nil {
				return err
			}
		}
		value = append(value, v)
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}
