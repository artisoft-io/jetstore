package compute_pipes

import (
	"fmt"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/cleansing_functions"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// TransformationColumnSpec Type map
type mapColumnEval struct {
	mapConfig    *mapColumnConfig
	cleansingCtx *cleansing_functions.CleansingFunctionContext
}

type mapColumnConfig struct {
	inputPos     int
	outputPos    int
	defaultValue any
	mapConfig    *MapExpression
}

func (ctx *mapColumnEval) Update(currentValue *[]any, input *[]any) error {
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
	// - update currentValue using updated inputV
	// - map inputV to correct rdf type if specified
	// - if code value mapping defined, then map the inputV to the corresponding code value based on the mapping definition,
	//   and use the code value as outputVal. If no code value found in map, use key __DEFAULT__ if defined, else use the inputV as outputVal.
	//
	var inputVal, outputVal any
	var inputV, errMsg string
	var ok bool
	var err error
	config := ctx.mapConfig
	mapConfig := config.mapConfig
	if config.inputPos >= 0 {
		inputVal = (*input)[config.inputPos]
	}
	if inputVal != nil {
		inputV, ok = inputVal.(string)
		if !ok {
			// humm, was expecting a string
			inputV = fmt.Sprintf("%v", inputVal)
		}
		if len(inputV) > 0 {
			outputVal = inputV
			if mapConfig.CleansingFunction != "" {
				outputVal, errMsg =
					ctx.cleansingCtx.ApplyCleasingFunction(mapConfig.CleansingFunction,
						mapConfig.Argument, inputV, config.inputPos, input)
				if len(errMsg) > 0 {
					// fmt.Println("*** Error while applying cleansing function:", errMsg)
					outputVal = nil
				}
			}
		}
	}
	if outputVal == nil {
		// Apply default if defined
		outputVal = config.defaultValue
		if outputVal == nil && (mapConfig.ErrMsg != "" || errMsg != "") {
			if errMsg == "" {
				errMsg = mapConfig.ErrMsg
			}
			return fmt.Errorf("error mapping column at output position %d: %s", config.outputPos, errMsg)
		}
	} else {
		// Cast to rdf type
		outputVal, err = CastToRdfType(outputVal, mapConfig.RdfType, nil)
		if err != nil {
			err = fmt.Errorf("while casting value to rdf type '%s': %v", mapConfig.RdfType, err)
		}
	}
	// Apply code value mapping if defined
	if outputVal != nil && mapConfig.CodeValueMapping != nil {
		outputValStr, ok := outputVal.(string)
		if !ok {
			// humm, expecting a string
			outputValStr = fmt.Sprintf("%v", outputVal)
		}
		mappedVal, ok := mapConfig.CodeValueMapping[outputValStr]
		if !ok {
			// Look for default value in code value mapping
			mappedVal, ok = mapConfig.CodeValueMapping["__DEFAULT__"]
			if !ok {
				// No mapping found, use the original value as output
				mappedVal = outputValStr
			}
		}
		outputVal = mappedVal
	}

	(*currentValue)[config.outputPos] = outputVal
	return err
}

func (ctx *mapColumnEval) Done(currentValue *[]any) error {
	return nil
}

func (ctx *BuilderContext) BuildMapTCEvaluator(source *InputChannel, outCh *OutputChannel,
	spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {

	if spec == nil || spec.MapExpr == nil {
		return nil, fmt.Errorf("error: Type map must have MapExpr != nil")
	}
	var defaultValue any
	var err error
	meRdfType := spec.MapExpr.RdfType
	meDefault := spec.MapExpr.Default
	if meDefault == "" {
		defaultValue = nil
	} else {
		switch  meRdfType{
		case "int", "bool":
			switch meDefault {
			case "true", "TRUE":
				defaultValue = 1
			case "false", "FALSE":
				defaultValue = 0
			default:
				defaultValue, err = utils.String2Int(meDefault)
				if err != nil {
					return nil, err
				}
			}
		case "double", "float64":
			defaultValue, err = strconv.ParseFloat(meDefault, 64)
			if err != nil {
				return nil, err
			}
		case "string", "text":
			defaultValue = meDefault

		case "date":
			temp, err := ParseDate(meDefault)
			if err != nil || temp == nil {
				fmt.Println("default value is not date:", meDefault)
				defaultValue = nil
				err = nil
			} else {
				defaultValue = *temp
			}
		case "datetime":
			temp, err := ParseDatetime(meDefault)
			if err != nil || temp == nil {
				fmt.Println("default value is not datetime:", meDefault)
				defaultValue = nil
				err = nil
			} else {
				defaultValue = *temp
			}

		case "int64", "long", "uint", "uint64":
			defaultValue, err = utils.String2Int(meDefault)
			if err != nil {
				return nil, err
			}
		}
	}
	// validation
	if spec.MapExpr.CodeValueMapping != nil {
		if len(meRdfType) > 0 && meRdfType != "text" && meRdfType != "string" {
			return nil, fmt.Errorf("error: code value mapping is only supported for string/text rdf type, but got rdf type: %s", meRdfType)
		}
	}
	expr := *spec.Expr
	inputPos, ok := (*source.Columns)[expr]
	if !ok {
		// Check for special jetstore properties
		if len(expr) == 0 || expr == "jets:key" {
			// Assign to nil when column not on input
			inputPos = -1
		} else {
			return nil, fmt.Errorf("mapping column: error column %s not found in input source %s", expr, source.Name)
		}
	}
	outputPos, ok := (*outCh.Columns)[spec.Name]
	if !ok {
		return nil, fmt.Errorf("mapping column: error column %s not found in output source %s", spec.Name, outCh.Name)
	}
	return &mapColumnEval{
		mapConfig: &mapColumnConfig{
			inputPos:     inputPos,
			outputPos:    outputPos,
			defaultValue: defaultValue,
			mapConfig:    spec.MapExpr},
		cleansingCtx: cleansing_functions.NewCleansingFunctionContext(source.Columns),
	}, nil
}
