package compute_pipes

import (
	"fmt"
	"log"
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
				if len(outV) > 0 {
					outputVal = outV
				}
			}
			if outputVal == nil && ctx.mapConfig.defaultValue != nil {
				outputVal = ctx.mapConfig.defaultValue
			}
		} else {
			var temp interface{}
			switch ctx.mapConfig.mapConfig.RdfType {
			case "int":
				outputVal, err = strconv.Atoi(inputVal.(string))
				if err != nil {
					// fmt.Println("input is not int:", inputVal.(string))
					outputVal = nil
				}
			case "int64", "long":
				outputVal, err = strconv.ParseInt(inputVal.(string), 10, 64)
				if err != nil {
					// fmt.Println("input is not long:", inputVal.(string))
					outputVal = nil
				}
			case "float64", "double":
				outputVal, err = strconv.ParseFloat(inputVal.(string), 64)
				if err != nil {
					// fmt.Println("input is not double:", inputVal.(string))
					outputVal = nil
				}
			case "date":
				temp, err = ParseDate(inputVal.(string))
				if err != nil {
					// fmt.Println("input is not date:", inputVal.(string))
					outputVal = nil
				} else {
					outputVal = *(temp.(*time.Time))
				}
			case "datetime":
				temp, err = ParseDatetime(inputVal.(string))
				if err != nil {
					// fmt.Println("input is not date:", inputVal.(string))
					outputVal = nil
				} else {
					outputVal = *(temp.(*time.Time))	
				}
			case "string", "text":
				switch v := inputVal.(type) {
				case string:
					if len(v) > 0 {
						outputVal = inputVal	
					}
				default:
					outputVal = fmt.Sprintf("%v", inputVal)
				}
			default:
				outputVal = inputVal
				log.Printf("warning: unknown rdf_type %s while mapping column value", ctx.mapConfig.mapConfig.RdfType)
			}
			
		}
	}
	(*currentValue)[ctx.mapConfig.outputPos] = outputVal
	return nil
}
func (ctx *mapColumnEval) done(currentValue *[]interface{}) error {
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
	inputPos, ok := source.columns[*spec.Expr]
	if !ok {
		err = fmt.Errorf("error column %s not found in input source %s", *spec.Expr, source.config.Name)
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		err = fmt.Errorf("error column %s not found in output source %s", spec.Name, outCh.config.Name)
	}
	return &mapColumnEval{
		mapConfig: &mapColumnConfig{
			inputPos: inputPos,
			outputPos: outputPos,
			defaultValue: defaultValue,
			mapConfig: spec.MapExpr},
		cleansingCtx: &cleansingFunctionContext{
			reMap: make(map[string]*regexp.Regexp),
			argdMap: make(map[string]float64),
			parsedFunctionArguments: make(map[string]interface{}),
		},
	}, err
}

