package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/artisoft-io/jetstore/jets/cleansing_functions"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TransformationColumnSpec Type map
type mapColumnEval struct {
	mapConfig    *mapColumnConfig
	cleansingCtx *cleansing_functions.CleansingFunctionContext
}

type mapColumnConfig struct {
	inputPos     int
	outputPos    int
	defaultValue interface{}
	mapConfig    *MapExpression
}

func (ctx *mapColumnEval) InitializeCurrentValue(currentValue *[]interface{}) {}
func (ctx *mapColumnEval) Update(currentValue *[]interface{}, input *[]interface{}) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error mapColumnEval.update cannot have nil currentValue or input")
	}
	// Steps:
	// - if inputVal != nil && inputVal is not empty string:
	//		if valid cleansing function:
	//			- apply cleansing function (which returns a string), set to inputV
	//		else
	//			- set inputV to inputVal as string
	// - if inputV is empty or cleansing function returned an errMsg:
	//		set inputV to defaultVal if not empty, else report the errMsg or the default errMsg if any.
	// - update currentValue using input applying cleansing function and default value
	// - map inputV to correct rdf type if specified
	//
	var inputVal, outputVal interface{}
	var inputV, errMsg string
	var ok bool
	var err error
	if ctx.mapConfig.inputPos >= 0 {
		inputVal = (*input)[ctx.mapConfig.inputPos]
	}
	if inputVal != nil {
		inputV, ok = inputVal.(string)
		if !ok {
			// humm, was expecting a string
			inputV = fmt.Sprintf("%v", inputVal)
		}
		if len(inputV) > 0 {
			outputVal = inputV
			if ctx.mapConfig.mapConfig.CleansingFunction != "" {
				outputVal, errMsg =
					ctx.cleansingCtx.ApplyCleasingFunction(ctx.mapConfig.mapConfig.CleansingFunction,
						ctx.mapConfig.mapConfig.Argument, inputV, ctx.mapConfig.inputPos, input)
				if len(errMsg) > 0 {
					//*TODO Report error on cleansing function
					// fmt.Println("*** Error while applying cleansing function:", errMsg)
					outputVal = nil
				}
			}
		}
	}
	if outputVal == nil {
		// Apply default if defined
		outputVal = ctx.mapConfig.defaultValue
		if outputVal == nil && (ctx.mapConfig.mapConfig.ErrMsg != "" || errMsg != "") {
			if errMsg == "" {
				errMsg = ctx.mapConfig.mapConfig.ErrMsg
			}
			fmt.Println("TODO Report Error, null on input and have errMsg:", errMsg)
		}
	} else {
		// Cast to rdf type
		outputVal, err = CastToRdfType(outputVal, ctx.mapConfig.mapConfig.RdfType)
		if err != nil {
			log.Printf("error while casting value to rdf type (will set to null): %v", err)
		}
	}
	(*currentValue)[ctx.mapConfig.outputPos] = outputVal
	return nil
}

func (ctx *mapColumnEval) Done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) BuildMapTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.MapExpr == nil {
		return nil, fmt.Errorf("error: Type map must have MapExpr != nil")
	}
	var defaultValue interface{}
	var err error
	meRdfType := spec.MapExpr.RdfType
	meDefault := spec.MapExpr.Default
	switch {
	case meDefault == "":
		defaultValue = nil
	case meRdfType == "int", meRdfType == "bool":
		switch {
		case meDefault == "true" || meDefault == "TRUE":
			defaultValue = 1
		case meDefault == "false" || meDefault == "FALSE":
			defaultValue = 0
		default:
			defaultValue, err = strconv.Atoi(meDefault)
			if err != nil {
				return nil, err
			}
		}
	case meRdfType == "double", meRdfType == "float64":
		defaultValue, err = strconv.ParseFloat(meDefault, 64)
		if err != nil {
			return nil, err
		}
	case meRdfType == "string", meRdfType == "text":
		defaultValue = meDefault

	case meRdfType == "date":
		temp, err := ParseDate(meDefault)
		if err != nil || temp == nil {
			fmt.Println("default value is not date:", meDefault)
			defaultValue = nil
			err = nil
		} else {
			defaultValue = *temp
		}
	case meRdfType == "datetime":
		temp, err := ParseDatetime(meDefault)
		if err != nil || temp == nil {
			fmt.Println("default value is not datetime:", meDefault)
			defaultValue = nil
			err = nil
		} else {
			defaultValue = *temp
		}

	case meRdfType == "int64", meRdfType == "long":
		defaultValue, err = strconv.ParseInt(meDefault, 10, 64)
		if err != nil {
			return nil, err
		}
	}

	inputPos, ok := (*source.columns)[*spec.Expr]
	if !ok {
		// Check for special jetstore properties
		if *spec.Expr == "jets:key" {
			// Assign to nil when column not on input
			inputPos = -1
		} else {
			return nil, fmt.Errorf("mapping column: error column %s not found in input source %s", *spec.Expr, source.name)
		}
	}
	outputPos, ok := (*outCh.columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("mapping column: error column %s not found in output source %s", spec.Name, outCh.name)
	}
	return &mapColumnEval{
		mapConfig: &mapColumnConfig{
			inputPos:     inputPos,
			outputPos:    outputPos,
			defaultValue: defaultValue,
			mapConfig:    spec.MapExpr},
		cleansingCtx: cleansing_functions.NewCleansingFunctionContext(source.columns),
	}, nil
}

// Utility function for casting to specified rdf type
func CastToRdfType(input interface{}, rdfType string) (interface{}, error) {
	if input == nil {
		switch rdfType {
		case "string", "text":
			return "", nil
		case "int", "integer", "int64", "long", "bool":
			return 0, nil
		case "float64", "double":
			return 0.0, nil
		case "date", "datetime":
			return nil, nil
		default:
			return nil, fmt.Errorf("error: unknown rdf_type %s while mapping column value", rdfType)
		}
	}

	var inputV string
	var inputArr []string
	switch vv := input.(type) {
	case string:
		if len(vv) == 0 {
			return nil, nil
		}
		inputV = vv
	case []string:
		if len(vv) == 0 {
			return nil, nil
		}
		inputArr = vv

	default:
		// humm, expecting string or []string
		inputV = fmt.Sprintf("%v", vv)
	}
	switch rdfType {
	case "string", "text":
		return input, nil
	case "int", "integer", "int64", "long":
		if inputArr == nil {
			return strconv.Atoi(inputV)
		}
		outV := make([]int, 0, len(inputArr))
		for _, v := range inputArr {
			vi, err := strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
			outV = append(outV, vi)
		}
		return outV, nil

	case "float64", "double":
		if inputArr == nil {
			return strconv.ParseFloat(inputV, 64)
		}
		outV := make([]float64, 0, len(inputArr))
		for _, v := range inputArr {
			vi, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, err
			}
			outV = append(outV, vi)
		}
		return outV, nil

	case "bool":
		if inputArr == nil {
			return rdf.ParseBool(inputV), nil
		}
		outV := make([]int, 0, len(inputArr))
		for _, v := range inputArr {
			outV = append(outV, rdf.ParseBool(v))
		}
		return outV, nil

	case "date":
		if inputArr == nil {
			temp, err := ParseDate(inputV)
			if err == nil {
				return *temp, nil
			} else {
				return nil, err
			}
		}
		outV := make([]time.Time, 0, len(inputArr))
		for _, v := range inputArr {
			vi, err := ParseDate(v)
			if err != nil {
				return nil, err
			}
			outV = append(outV, *vi)
		}
		return outV, nil

	case "datetime":
		if inputArr == nil {
			temp, err := ParseDatetime(inputV)
			if err == nil {
				return *temp, nil
			} else {
				return nil, err
			}
		}
		outV := make([]time.Time, 0, len(inputArr))
		for _, v := range inputArr {
			vi, err := ParseDatetime(v)
			if err != nil {
				return nil, err
			}
			outV = append(outV, *vi)
		}
		return outV, nil

	default:
		return nil, fmt.Errorf("error: unknown rdf_type %s while mapping column value", rdfType)
	}
}
