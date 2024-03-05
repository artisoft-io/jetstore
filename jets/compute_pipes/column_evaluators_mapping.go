package compute_pipes

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
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
	var err error
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
			switch ctx.mapConfig.mapConfig.RdfType {
			case "int":
				outputVal, err = strconv.Atoi(inputVal.(string))
				if err != nil {
					fmt.Println("input is not int:", inputVal.(string))
					outputVal = nil
				}
			case "int64", "long":
				outputVal, err = strconv.ParseInt(inputVal.(string), 10, 64)
				if err != nil {
					fmt.Println("input is not long:", inputVal.(string))
					outputVal = nil
				}
			case "float64", "double":
				outputVal, err = strconv.ParseFloat(inputVal.(string), 64)
				if err != nil {
					fmt.Println("input is not double:", inputVal.(string))
					outputVal = nil
				}
			case "date", "datetime":
				outputVal, err = time.Parse(time.RFC3339, inputVal.(string))
				// outputVal, err = time.Parse("1/29/2024", inputVal.(string))
				if err != nil {
					fmt.Println("input is not date:", inputVal.(string))
					outputVal = nil
				}
			default:
				outputVal = inputVal
			}
			
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
	switch  {
	case spec.MapExpr.Default == nil:
		defaultValue = nil
	case spec.MapExpr.RdfType=="int", spec.MapExpr.RdfType=="bool":
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
	case spec.MapExpr.RdfType=="int64", spec.MapExpr.RdfType=="long":
		defaultValue, err = strconv.ParseInt(*spec.MapExpr.Default, 10, 64)
		if err != nil {
			return nil, err
		}
	case spec.MapExpr.RdfType=="string", spec.MapExpr.RdfType=="text":
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

