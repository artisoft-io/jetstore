package compute_pipes

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func (ctx *BuilderContext) parseValue(expr *string) (interface{}, error) {
	var value interface{}
	var err error
	switch {
	case *expr == "NULL":
		value = nil
	case *expr == "NaN" || *expr == "NAN":
		value = math.NaN()

	case strings.HasPrefix(*expr, "$"):
		// value is an env var
		value = ctx.env[*expr]
		
	case strings.HasPrefix(*expr, "'"):
		// value is a string
		value = strings.TrimSuffix(strings.TrimPrefix(*expr, "'"), "'")
		
	case strings.Contains(*expr, "."):
		// value is double
		value, err = strconv.ParseFloat(*expr, 64)
		if err != nil {
			return nil, fmt.Errorf("error: expecting a double: %s", *expr)
		}
	default:
		// default to int
		value, err = strconv.ParseInt(*expr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error: expecting an int: %s", *expr)
		}
	}
	// fmt.Printf("**! PARSEVALUE: %s => = %v of type %T\n", *expr, value, value)
	return value, err
}

// build the runtime evaluator for the column transformation
func (ctx *BuilderContext) buildTransformationColumnEvaluator(source *InputChannel, outCh *OutputChannel, spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	switch spec.Type {
	// select, value, eval, map, count, distinct_count, sum, min, case
	case "select":
		if spec.Expr == nil {
			return nil, fmt.Errorf("error: Type select must have Expr != nil")
		}
		inputPos, ok := source.columns[*spec.Expr]
		var err error
		if !ok {
			err = fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.config.Name)
		}
		outputPos, ok := outCh.columns[spec.Name]
		if !ok {
			err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
		}
		return &selectColumnEval{
			inputPos:  inputPos,
			outputPos: outputPos,
		}, err

	case "value":
		if spec.Expr == nil {
			return nil, fmt.Errorf("error: Type value must have Expr != nil")
		}
		value, err := ctx.parseValue(spec.Expr)
		if err != nil {
			return nil, err
		}
		outputPos, ok := outCh.columns[spec.Name]
		if !ok {
			err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
		}
			return &valueColumnEval{
			value:     value,
			outputPos: outputPos,
		}, err

	case "eval":
		evalEpr, err := ctx.buildExprNodeEvaluator(source, outCh, spec.EvalExpr)
		if err != nil {
			return nil, fmt.Errorf("while calling buildExprNodeEvaluator: %v", err)
		}
		outputPos, ok := outCh.columns[spec.Name]
		if !ok {
			err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
		}
			return &evalExprColumnEval{
			expr: evalEpr,
			outputPos: outputPos,
		}, err

	case "map":
		mapEvaluator, err := ctx.buildMapEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling buildMapEvaluator: %v", err)
		}
		return mapEvaluator, nil

	case "count":
		countEvaluator, err := ctx.buildCountEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling buildCountEvaluator: %v", err)
		}
		return countEvaluator, nil

	case "distinct_count":
		distinctCountEvaluator, err := ctx.buildDistinctCountEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling buildDistinctCountEvaluator: %v", err)
		}
		return distinctCountEvaluator, nil

	case "sum":
		sumEvaluator, err := ctx.buildSumEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling buildSumEvaluator: %v", err)
		}
		return sumEvaluator, nil

	case "min":
		minEvaluator, err := ctx.buildMinEvaluator(source, outCh, spec)
		if err != nil {
			return nil, fmt.Errorf("while calling buildMinEvaluator: %v", err)
		}
		return minEvaluator, nil

	case "case":
		return ctx.buildCaseExprEvaluator(source, outCh, spec)

	case "map_reduce":
		return ctx.buildMapReduceEvaluator(source, outCh, spec)
	}
	return nil, fmt.Errorf("error: unknown TransformationColumnSpec Type: %v", spec.Type)
}


// TransformationColumnSpec Type eval
type evalExprColumnEval struct {
	expr evalExpression
	outputPos int
}

func (ctx *evalExprColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *evalExprColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	value, err := ctx.expr.eval(input)
	if err != nil {
		return err
	}
	(*currentValue)[ctx.outputPos] = value
	return nil
}
func (ctx *evalExprColumnEval) done(currentValue *[]interface{}) error {
	return nil
}


// TransformationColumnSpec Type value
type valueColumnEval struct {
	value     interface{}
	outputPos int
}
func (ctx *valueColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *valueColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *valueColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
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
func (ctx *selectColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *selectColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *selectColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error selectColumnEval.update cannot have nil currentValue or input")
	}
	(*currentValue)[ctx.outputPos] = (*input)[ctx.inputPos]
	return nil
}
