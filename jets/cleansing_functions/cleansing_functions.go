package cleansing_functions

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

type CleansingFunctionContext struct {
	reMap                   map[string]*regexp.Regexp
	argdMap                 map[string]float64
	parsedFunctionArguments map[string]interface{}
	inputColumns            *map[string]int
}

func NewCleansingFunctionContext(inputColumns *map[string]int) *CleansingFunctionContext {
	return &CleansingFunctionContext{
		reMap:                   make(map[string]*regexp.Regexp),
		argdMap:                 make(map[string]float64),
		parsedFunctionArguments: make(map[string]interface{}),
		inputColumns:            inputColumns,
	}
}
func (ctx *CleansingFunctionContext) With(inputColumns *map[string]int) *CleansingFunctionContext {
	return &CleansingFunctionContext{
		reMap:                   ctx.reMap,
		argdMap:                 ctx.argdMap,
		parsedFunctionArguments: ctx.parsedFunctionArguments,
		inputColumns:            inputColumns,
	}
}

// inputColumnName can be null
func (ctx *CleansingFunctionContext) ApplyCleasingFunction(functionName string, argument string, inputValue string,
	inputPos int, inputRow *[]interface{}) (obj interface{}, errMsg string) {
	var err error
	var sz int
	switch functionName {

	case "trim":
		vv := strings.TrimSpace(inputValue)
		if len(vv) == 0 {
			obj = nil
		} else {
			obj = vv
		}

	case "validate_date":
		_, err2 := rdf.ParseDate(inputValue)
		if err2 == nil {
			obj = inputValue
		} else {
			errMsg = err2.Error()
		}

	case "to_upper":
		obj = strings.ToUpper(inputValue)

	case "to_zip5":
		// Remove non digits characters
		inVal := filterDigits(inputValue)
		sz = len(inVal)
		switch {
		case sz == 0:
			obj = nil
		case sz < 5:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%05d", v)
				if obj == "00000" {
					obj = nil
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 5:
			obj = inVal
			if obj == "00000" {
				obj = nil
			}
		case sz > 5 && sz < 9:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%09d", v)[:5]
				if obj == "00000" {
					obj = nil
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 9:
			obj = inVal[:5]
			if obj == "00000" {
				obj = nil
			}
		default:
		}

	case "to_zipext4_from_zip9": // from a zip9 input
		// Remove non digits characters
		inVal := filterDigits(inputValue)
		sz = len(inVal)
		switch {
		case sz == 0:
			obj = nil
		case sz > 5 && sz < 9:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%09d", v)[5:]
				if obj == "0000" {
					obj = nil
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 9:
			obj = inVal[5:]
			if obj == "0000" {
				obj = nil
			}
		default:
		}

	case "to_zipext4": // from a zip ext4 input
		// Remove non digits characters
		inVal := filterDigits(inputValue)
		sz = len(inVal)
		switch {
		case sz == 0:
			obj = nil
		case sz < 4:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%04d", v)
				if obj == "0000" {
					obj = nil
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 4:
			obj = inVal
			if obj == "0000" {
				obj = nil
			}
		default:
		}

	case "format_phone": // Validate & format phone according to E.164
		// Output: +1 area_code exchange_code subscriber_nbr
		// area_code: 3 digits, 1st digit is not 0 or 1
		// exchange_code: 3 digits, 1st digit is not 0 or 1
		// subscriber_nbr: 4 digits
		// Optional function argument is fmt.Sprintf formatter, expecting 3 string arguments (area_code, exchange_code, subscriber_nbr)
		inVal := filterDigits(inputValue)
		if len(inVal) < 10 {
			errMsg = "too few digits"
			return obj, errMsg
		}
		if inVal[0] == '0' {
			inVal = inVal[1:]
		}
		if inVal[0] == '1' {
			inVal = inVal[1:]
		}
		if len(inVal) < 10 {
			errMsg = "invalid sequence of digits"
			return obj, errMsg
		}
		areaCode := inVal[0:3]
		exchangeCode := inVal[3:6]
		subscriberNbr := inVal[6:10]
		if areaCode[0] == '0' || areaCode[0] == '1' {
			errMsg = "invalid area code"
			return obj, errMsg
		}
		if exchangeCode[0] == '0' || exchangeCode[0] == '1' {
			errMsg = "invalid exchange code"
			return obj, errMsg
		}
		if len(argument) == 0 {
			argument = "+1%s%s%s"
		}
		obj = fmt.Sprintf(argument, areaCode, exchangeCode, subscriberNbr)

	case "reformat0":
		if argument != "" {
			// Remove non digits characters
			inVal := filterDigits(inputValue)
			var v int
			if len(inVal) == 0 {
				obj = nil
			} else {
				v, err = strconv.Atoi(inVal)
				if err == nil {
					obj = fmt.Sprintf(argument, v)
				} else {
					errMsg = err.Error()
				}
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function reformat0 for input column pos %d", inputPos)
		}

	case "overpunch_number":
		if argument != "" {
			// Get the number of decimal position
			var npos int
			npos, err = strconv.Atoi(argument)
			if err == nil {
				vv, err := OverpunchNumber(inputValue, npos)
				if err != nil {
					obj = nil
					errMsg = err.Error()
				} else {
					if len(vv) == 0 {
						obj = nil
					} else {
						obj = vv
					}
				}
			} else {
				obj = nil
				errMsg = err.Error()
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function overpunch_number for input column pos %d", inputPos)
		}

	case "apply_regex":
		if argument != "" {
			re, ok := ctx.reMap[argument]
			if !ok {
				re, err = regexp.Compile(argument)
				if err != nil {
					// configuration error, bailing out
					log.Panicf("ERROR regex argument does not compile: %s", argument)
				}
				ctx.reMap[argument] = re
			}
			vv := re.FindString(inputValue)
			if len(vv) == 0 {
				obj = nil
			} else {
				obj = vv
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function apply_regex for input column pos %d", inputPos)
		}

	case "scale_units":
		if argument != "" {
			if argument == "1" {
				vv := filterDouble(inputValue)
				if len(vv) == 0 {
					obj = nil
				} else {
					obj = vv
				}
			} else {
				divisor, ok := ctx.argdMap[argument]
				if !ok {
					divisor, err = strconv.ParseFloat(argument, 64)
					if err != nil {
						// configuration error, bailing out
						log.Panicf("ERROR divisor argument to function scale_units is not a double: %s", argument)
					}
					ctx.argdMap[argument] = divisor
				}
				// Remove non digits characters
				inVal := filterDouble(inputValue)
				var unit float64
				unit, err = strconv.ParseFloat(inVal, 64)
				if err == nil {
					obj = fmt.Sprintf("%f", math.Ceil(unit/divisor))
				} else {
					obj = nil
					errMsg = err.Error()
				}
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function scale_units for input column pos %d", inputPos)
		}

	case "parse_amount":
		// clean up the amount
		inVal := filterDouble(inputValue)
		if len(inVal) > 0 {
			obj = inVal
			// argument is optional, assume divisor is 1 if absent
			if argument != "" && argument != "1" {
				divisor, ok := ctx.argdMap[argument]
				if !ok {
					divisor, err = strconv.ParseFloat(argument, 64)
					if err != nil {
						// configuration error, bailing out
						log.Panicf("ERROR divisor argument to function scale_units is not a double: %s", argument)
					}
					ctx.argdMap[argument] = divisor
				}
				var amt float64
				amt, err = strconv.ParseFloat(inVal, 64)
				if err == nil {
					obj = fmt.Sprintf("%f", amt/divisor)
				} else {
					obj = nil
					errMsg = err.Error()
				}
			}
		}

	case "concat", "concat_with":
		// Cleansing function that concatenate inputRow columns w delimiter
		// Get the parsed argument
		arg, err := ParseConcatFunctionArgument(argument, functionName, ctx.inputColumns, ctx.parsedFunctionArguments, inputRow)
		if err != nil {
			errMsg = err.Error()
		} else {
			var buf strings.Builder
			buf.WriteString(inputValue)
			for i := range arg.ColumnPositions {
				// fmt.Println("=== concat value @pos:",arg.ColumnPositions[i])
				if (*inputRow)[arg.ColumnPositions[i]] != nil {
					if arg.Delimit != "" {
						buf.WriteString(arg.Delimit)
					}
					switch vv := (*inputRow)[arg.ColumnPositions[i]].(type) {
					case string:
						buf.WriteString(vv)
					case *sql.NullString:
						if vv.Valid {
							buf.WriteString(vv.String)
						}
					default:
						buf.WriteString(fmt.Sprint(vv))
					}

				}
			}
			vv := buf.String()
			if len(vv) == 0 {
				obj = nil
			} else {
				obj = vv
			}
		}

	case "find_and_replace":
		// Cleansing function that replace portion of the input column
		// Get the parsed argument
		arg, err := ParseFindReplaceFunctionArgument(argument, functionName, ctx.parsedFunctionArguments)
		if err != nil {
			errMsg = err.Error()
		} else {
			vv := strings.ReplaceAll(inputValue, arg.Find, arg.ReplaceWith)
			if len(vv) == 0 {
				obj = nil
			} else {
				obj = vv
			}
		}

	case "substring":
		// Cleansing function that takes a substring of input columns
		// Get the parsed argument
		arg, err := ParseSubStringFunctionArgument(argument, functionName, ctx.parsedFunctionArguments)
		if err != nil {
			errMsg = err.Error()
		} else {
			end := arg.End
			if end < 0 {
				end = len(inputValue) + end
			}
			if end > len(inputValue) || end <= arg.Start {
				obj = nil
			} else {
				obj = (inputValue)[arg.Start:end]
			}
		}

	case "split_on":
		if argument != "" {
			obj = SplitOn(inputValue, argument)
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function split_on for input column pos %d", inputPos)
		}

	case "unique_split_on":
		if argument != "" {
			obj = UniqueSplitOn(inputValue, argument)
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function unique_split_on for input column pos %d", inputPos)
		}

	case "slice_input":
		if argument != "" {
			obj = SliceInput(inputValue, argument, ctx.parsedFunctionArguments)
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function slice_input for input column pos %d", inputPos)
		}

	case "ndc10_to_11":
		obj = Ndc10To11(inputValue)

	default:
		log.Panicf("ERROR unknown mapping function: %s", functionName)
	}

	return obj, errMsg
}

func SliceInput(inputValue, argument string, parsedFunctionArguments map[string]any) any {
	if inputValue == "" {
		return nil
	}
	sliceArg, err := ParseSliceInputFunctionArgument(argument, "slice_input", parsedFunctionArguments)
	if err != nil {
		log.Panicf("while parsing arguments for cleansing function parse_input: %v", err)
	}
	sliceValues := strings.Split(inputValue, sliceArg.Delimit)
	l := len(sliceValues)
	var values []string
	switch {
	case l == 0:
		return nil
	case sliceArg.From == nil && sliceArg.To == nil && sliceArg.Values == nil:
		return sliceValues
	case sliceArg.Values != nil:
		values = make([]string, 0, len(*sliceArg.Values))
		for _, i := range *sliceArg.Values {
			if i < l {
				values = append(values, sliceValues[i])
			}
		}
	case sliceArg.From != nil && sliceArg.To == nil:
		lenValues := l - *sliceArg.From
		if lenValues > 0 {
			values = make([]string, 0, lenValues)
			for i := range lenValues {
				index := *sliceArg.From + i
				if index < l {
					values = append(values, sliceValues[index])
				}
			}
		}
	default:
		lenValues := *sliceArg.To - *sliceArg.From
		values = make([]string, 0, lenValues)
		for i := range lenValues {
			index := *sliceArg.From + i
			if index < l {
				values = append(values, sliceValues[index])
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

// Cleansing function to convert NDC with 10 digits with slash separators
// to an 11 digits without slashes. The input is expected to contains 2 sliashes.
// If the input contains no slash, the the input is returned;
// If the input does not contain 2 slashes, the input is returned unchanged.
// If the input contains 2 slashes but not the expected number of digits,
// then the input is returned with the slashes removed.
// It does not validate the ndc, nor that the resulting string contains
// only digits. The returned number of digits is not validated either.
// See the transformation rule at:
// https://health.maryland.gov/phpa/OIDEOR/IMMUN/Shared%20Documents/Handout%203%20-%20NDC%20conversion%20to%2011%20digits.pdf
func Ndc10To11(inputValue string) any {
	if inputValue == "" {
		return nil
	}
	ndcSlice := strings.Split(inputValue, "-")
	if len(ndcSlice) != 3 {
		return inputValue
	}
	l1 := len(ndcSlice[0])
	l2 := len(ndcSlice[1])
	l3 := len(ndcSlice[2])
	switch {
	case l1 == 4 && l2 == 4 && l3 == 2:
		return fmt.Sprintf("0%s%s%s", ndcSlice[0], ndcSlice[1], ndcSlice[2])
	case l1 == 5 && l2 == 3 && l3 == 2:
		return fmt.Sprintf("%s0%s%s", ndcSlice[0], ndcSlice[1], ndcSlice[2])
	case l1 == 5 && l2 == 4 && l3 == 1:
		return fmt.Sprintf("%s%s0%s", ndcSlice[0], ndcSlice[1], ndcSlice[2])
	default:
		return fmt.Sprintf("%s%s%s", ndcSlice[0], ndcSlice[1], ndcSlice[2])
	}
}

func SplitOn(inputValue, argument string) any {
	if inputValue == "" || argument == "" {
		return nil
	}
	return strings.Split(inputValue, argument)
}

func UniqueSplitOn(inputValue, argument string) any {
	if inputValue == "" || argument == "" {
		return nil
	}
	vv := strings.Split(inputValue, argument)
	// vv may contains duplicated value, to make each value unique we append -%d to the
	// value, where %d is the value of a counter such that:
	//   if inputValue is "value1,value2,value1,value3"
	//   then the parsed values will be:
	//     value1-0
	//     value1-1
	//     value2-0
	//     value3-0
	// Group the common values
	cm := make(map[string]*[]string)
	for _, v := range vv {
		cv := cm[v]
		if cv == nil {
			cv = &[]string{}
			cm[v] = cv
		}
		*cv = append(*cv, v)
	}
	// reuse vv
	vv = vv[:0]
	for _, cv := range cm {
		for i := range *cv {
			vv = append(vv, fmt.Sprintf("%s-%d", (*cv)[i], i))
		}
	}
	return vv
}
