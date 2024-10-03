package compute_pipes

import (
	"fmt"
	"log"
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
	inputVal := (*input)[ctx.mapConfig.inputPos]
	var outputVal interface{}
	var inputV, errMsg string
	var ok bool
	var err error
	if inputVal != nil {
		inputV, ok = inputVal.(string)
		if !ok {
			// humm, was expecting a string
			inputV = fmt.Sprintf("%v", inputVal)
		}
		if len(inputV) > 0 && ctx.mapConfig.mapConfig.CleansingFunction != nil {
			inputV, errMsg = ctx.cleansingCtx.applyCleasingFunction(ctx.mapConfig.mapConfig.CleansingFunction, ctx.mapConfig.mapConfig.Argument, &inputV)
			if len(errMsg) > 0 {
				// fmt.Println("*** Error while applying cleansing function:", errMsg)
				inputV = ""
			}
		}
	}
	if len(inputV) == 0 {
		// Apply default if defined
		outputVal = ctx.mapConfig.defaultValue
		if outputVal == nil && (ctx.mapConfig.mapConfig.ErrMsg != nil || errMsg != "") {
			if errMsg == "" {
				errMsg = *ctx.mapConfig.mapConfig.ErrMsg
			}
			fmt.Println("TODO Report Error, null on input and have errMsg:", errMsg)
		}
	} else {
		// Cast to rdf type
		outputVal, err = CastToRdfType(inputV, ctx.mapConfig.mapConfig.RdfType)
		if err != nil {
			log.Printf("error while casting value to rdf type (will set to null): %v", err)
		}
	}
	(*currentValue)[ctx.mapConfig.outputPos] = outputVal
	return nil
}
func (ctx *mapColumnEval) done(currentValue *[]interface{}) error {
	return nil
}

func (ctx *BuilderContext) buildMapEvaluator(source *InputChannel, outCh *OutputChannel,  spec *TransformationColumnSpec) (TransformationColumnEvaluator, error) {
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
	case spec.MapExpr.RdfType=="double", spec.MapExpr.RdfType=="float64":
		defaultValue, err = strconv.ParseFloat(*spec.MapExpr.Default, 64)
		if err != nil {
			return nil, err
		}
	case spec.MapExpr.RdfType=="string", spec.MapExpr.RdfType=="text":
		defaultValue = *spec.MapExpr.Default

	case spec.MapExpr.RdfType=="date":
		temp, err := ParseDate(*spec.MapExpr.Default)
		if err != nil || temp == nil {
			fmt.Println("default value is not date:", *spec.MapExpr.Default)
			defaultValue = nil
			err = nil
		} else {
			defaultValue = *temp
		}
	case spec.MapExpr.RdfType=="datetime":
		temp, err := ParseDatetime(*spec.MapExpr.Default)
		if err != nil || temp == nil {
			fmt.Println("default value is not datetime:", *spec.MapExpr.Default)
			defaultValue = nil
			err = nil
		} else {
			defaultValue = *temp
		}

	case spec.MapExpr.RdfType=="int64", spec.MapExpr.RdfType=="long":
		defaultValue, err = strconv.ParseInt(*spec.MapExpr.Default, 10, 64)
		if err != nil {
			return nil, err
		}
	}

	inputPos, ok := source.columns[*spec.Expr]
	if !ok {
		return nil, fmt.Errorf("mapping column: error column %s not found in input source %s", *spec.Expr, source.config.Name)
	}
	outputPos, ok := outCh.columns[spec.Name]
	if !ok {
		return nil, fmt.Errorf("mapping column: error column %s not found in output source %s", spec.Name, outCh.config.Name)
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
	}, nil
}

// Utility function for casting to specified rdf type
func CastToRdfType(inputV string, rdfType string) (interface{}, error) {
	switch rdfType {

	case "string", "text":
		return inputV, nil

	case "int", "integer", "int64", "long":
		if inputV == "" {
			return nil, nil
		}
		return strconv.Atoi(inputV)

	case "float64", "double":
		if inputV == "" {
			return nil, nil
		}
		return strconv.ParseFloat(inputV, 64)

	case "bool":
		if inputV == "" {
			return nil, nil
		}
		if inputV[0] == 'T' || inputV[0] == 't' || inputV[0] == '1' {
			return 1, nil
		}
		return 0, nil
	
	case "date":
		if inputV == "" {
			return nil, nil
		}
		temp, err := ParseDate(inputV)
		if err == nil {
			return *temp, nil
		} else {
			return nil, err
		}
	case "datetime":
		if inputV == "" {
			return nil, nil
		}
		temp, err := ParseDatetime(inputV)
		if err == nil {
			return *temp, nil
		} else {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("error: unknown rdf_type %s while mapping column value", rdfType)
	}
}
