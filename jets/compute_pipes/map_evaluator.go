package compute_pipes

import (
	"fmt"
	"regexp"
	"strconv"
)

// TransformationColumnSpec Type map
type mapColumnEval struct {
	mapConfig *mapColumnConfig
	cleansingCtx *cleansingFunctionContext
}

type mapColumnConfig struct {
	inputPos  int
	outputPos int
	defaultValue interface{}
	mapConfig *MapExpression
}

func (ctx *mapColumnEval) initializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *mapColumnEval) update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error mapColumnEval.update cannot have nil currentValue or input")
	}
	// update currentValue using input applying cleansing function and default value
	inputVal := (*input)[ctx.mapConfig.inputPos]
	var outputVal interface{}
	if inputVal == nil {
		// Apply default
		outputVal = ctx.mapConfig.defaultValue
		if ctx.mapConfig.defaultValue == nil && ctx.mapConfig.mapConfig.ErrMsg != nil {
			fmt.Println("Error raise, null on input")
		}
	} else {
		if ctx.mapConfig.mapConfig.CleansingFunction != nil {
			inputV := inputVal.(string)
			outV, errMsg := ctx.cleansingCtx.applyCleasingFunction(ctx.mapConfig.mapConfig.CleansingFunction, ctx.mapConfig.mapConfig.Argument, &inputV)
			if len(errMsg) > 0 {
				fmt.Println("*** Error while applying cleansing function:", errMsg)
				outputVal = nil
			} else {
				outputVal = outV
			}
			if outputVal == nil && ctx.mapConfig.defaultValue != nil {
				outputVal = ctx.mapConfig.defaultValue
			}
		} else {
			outputVal = inputVal
		}
	}
	(*currentValue)[ctx.mapConfig.outputPos] = outputVal
	return nil
}

func (ctx *BuilderContext) buildMapEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (*mapColumnEval, error) {
	if spec == nil || spec.MapExpr == nil {
		return nil, fmt.Errorf("error: Type map must have MapExpr != nil")
	}
	var defaultValue interface{}
	var err error
	switch spec.MapExpr.RdfType {
	case "int", "bool":
		switch {
		case *spec.MapExpr.Default == "true" || *spec.MapExpr.Default == "TRUE":
			defaultValue = 1
		case *spec.MapExpr.Default == "false" || *spec.MapExpr.Default == "FALSE":
			defaultValue = 0
		default:
			defaultValue, err = strconv.Atoi(*spec.MapExpr.Default)
			if err != nil {
				return nil, err
			}	
		}
	case "int64", "long":
		defaultValue, err = strconv.ParseInt(*spec.MapExpr.Default, 10, 64)
		if err != nil {
			return nil, err
		}
	case "string", "text":
		defaultValue = *spec.MapExpr.Default
	}
	return &mapColumnEval{
		mapConfig: &mapColumnConfig{
			inputPos: source.columns[*spec.Expr],
			outputPos: outCh.columns[spec.Name],
			defaultValue: defaultValue,
			mapConfig: spec.MapExpr},
		cleansingCtx: &cleansingFunctionContext{
			reMap: make(map[string]*regexp.Regexp),
			argdMap: make(map[string]float64),
			parsedFunctionArguments: make(map[string]interface{}),
		},
	}, nil
}

