package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/cleansing_functions"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// TransformationColumnSpec Type map
type mapColumnEval struct {
	mapConfig    *mapColumnConfig
	cleansingCtx *cleansing_functions.CleansingFunctionContext
	// perRecord is true when the evaluator writes one record per input record
	// (map_record), and false when it accumulates into a value kept across
	// records (aggregate, map_reduce). See Update for what it changes.
	perRecord bool
}

// markPerRecord tells a map column evaluator that it writes one record per input
// record, as in map_record. Other evaluators are left as they are.
func markPerRecord(ce TransformationColumnEvaluator) {
	if me, ok := ce.(*mapColumnEval); ok {
		me.perRecord = true
	}
}

type mapColumnConfig struct {
	inputPos     int
	outputPos    int
	defaultValue any
	mapConfig    *MapExpression
}

// Update maps one column. The rules, in the order they apply:
//
//   - The input value is present when it is not nil. A present value is passed
//     through the cleansing function if any, cast to rdf_type, and mapped by the
//     code value mapping if any.
//   - A present value is rejected when the cleansing function reports an error or
//     the cast fails. It is empty when it is blank (empty or only white space)
//     before or after cleansing, when the cast yields nil, or when the code value
//     mapping yields a blank string. A map never outputs a blank string.
//   - A rejected or empty value is replaced by the default when one is defined,
//     which is not an error. Without a default it is replaced by null, written to
//     the output; a rejection returns its error, and an empty value returns the
//     err_msg error when err_msg is set. map_record's on_error policy then applies.
//   - An absent input (nil) takes the default when one is defined. Without a
//     default the output is left as it is, and the err_msg error is returned when
//     err_msg is set.
//   - When the evaluator accumulates across records (aggregate, map_reduce, ie
//     not perRecord), a rejected or empty value is skipped instead: the output
//     keeps the value an earlier record gave it, nothing is written, not even the
//     default, and no error is returned. One record whose value fails the cast
//     must not reset the accumulated value to null or to the default part way
//     through, and an error would stop the step, since these pipes have no
//     on_error policy. This is the case the "don't overwrite with nil" rule was
//     written for.
//
// Why an absent input and an empty value differ. Leaving the output alone for an
// absent input is what lets several map columns fall back onto one output: the
// first one with an input value wins, and a later column whose input is missing
// does not erase it. An empty or rejected value is different, because the input
// had a value: when map_record updates its input record in place (no new_record)
// and the column maps onto itself, the output position still holds that raw
// value, so leaving it alone would let the value the map just emptied or rejected
// flow downstream unchanged.
func (ctx *mapColumnEval) Update(currentValue *[]any, input *[]any) error {
	if currentValue == nil || input == nil {
		return fmt.Errorf("error mapColumnEval.update cannot have nil currentValue or input")
	}
	var inputVal, outputVal any
	var err error
	config := ctx.mapConfig
	mapConfig := config.mapConfig
	if config.inputPos >= 0 {
		inputVal = (*input)[config.inputPos]
	}
	if inputVal == nil {
		// Absent input
		if config.defaultValue != nil {
			outputVal = blankToNil(ctx.applyCodeValueMapping(config.defaultValue))
			if outputVal != nil {
				(*currentValue)[config.outputPos] = outputVal
				return nil
			}
		}
		// No default: don't overrite currentValue with nil (fallback chains, see above)
		if mapConfig.ErrMsg != "" {
			return fmt.Errorf("error mapping column at output position %d: %s", config.outputPos, mapConfig.ErrMsg)
		}
		return nil
	}

	// Present input
	inputV, ok := inputVal.(string)
	if !ok {
		// humm, was expecting a string
		inputV = fmt.Sprintf("%v", inputVal)
	}
	if strings.TrimSpace(inputV) != "" {
		outputVal = inputV
		if mapConfig.CleansingFunction != "" {
			var errMsg string
			outputVal, errMsg =
				ctx.cleansingCtx.ApplyCleasingFunction(mapConfig.CleansingFunction,
					mapConfig.Argument, inputV, config.inputPos, input)
			if len(errMsg) > 0 {
				log.Println("*** Error while applying cleansing function:", errMsg)
				outputVal = nil
				err = fmt.Errorf("error mapping column at output position %d: %s", config.outputPos, errMsg)
			}
		}
		outputVal = blankToNil(outputVal)
		if outputVal != nil {
			// Cast to rdf type
			outputVal, err = CastToRdfType(outputVal, mapConfig.RdfType, nil)
			if err != nil {
				err = fmt.Errorf("while casting value to rdf type '%s': %v", mapConfig.RdfType, err)
				outputVal = nil
			}
		}
		if outputVal != nil {
			outputVal = blankToNil(ctx.applyCodeValueMapping(outputVal))
		}
	}
	if outputVal == nil && !ctx.perRecord {
		// Accumulating across records: skip the rejected or empty value, keep the
		// value accumulated so far (see above)
		return nil
	}
	if outputVal == nil {
		// Rejected or empty: use the default, else null
		if config.defaultValue != nil {
			outputVal = blankToNil(ctx.applyCodeValueMapping(config.defaultValue))
			if outputVal != nil {
				// The default stands in for the rejected or empty value: not an error
				err = nil
			}
		}
		if outputVal == nil && err == nil && mapConfig.ErrMsg != "" {
			err = fmt.Errorf("error mapping column at output position %d: %s", config.outputPos, mapConfig.ErrMsg)
		}
	}
	// Written even when nil, so the raw input does not pass through
	(*currentValue)[config.outputPos] = outputVal
	return err
}

// applyCodeValueMapping maps value with the code value mapping when one is
// defined. A value not in the mapping takes the __DEFAULT__ entry if any, else
// is kept as is.
func (ctx *mapColumnEval) applyCodeValueMapping(value any) any {
	cvm := ctx.mapConfig.mapConfig.CodeValueMapping
	if value == nil || cvm == nil {
		return value
	}
	valueStr, ok := value.(string)
	if !ok {
		// humm, expecting a string
		valueStr = fmt.Sprintf("%v", value)
	}
	mappedVal, ok := cvm[valueStr]
	if !ok {
		// Look for default value in code value mapping
		mappedVal, ok = cvm["__DEFAULT__"]
		if !ok {
			// No mapping found, use the original value as output
			mappedVal = valueStr
		}
	}
	return mappedVal
}

// blankToNil returns nil for a string that is empty or only white space, so a
// map never outputs a blank string.
func blankToNil(value any) any {
	if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
		return nil
	}
	return value
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
		switch meRdfType {
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
				log.Println("default value is not date:", meDefault)
				defaultValue = nil
				err = nil
			} else {
				defaultValue = *temp
			}
		case "datetime":
			temp, err := ParseDatetime(meDefault)
			if err != nil || temp == nil {
				log.Println("default value is not datetime:", meDefault)
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
