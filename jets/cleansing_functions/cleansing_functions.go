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
	inputColumns            map[string]int
}

func NewCleansingFunctionContext(inputColumns map[string]int) *CleansingFunctionContext {
	return &CleansingFunctionContext{
		reMap:                   make(map[string]*regexp.Regexp),
		argdMap:                 make(map[string]float64),
		parsedFunctionArguments: make(map[string]interface{}),
		inputColumns:            inputColumns,
	}
}
func (ctx *CleansingFunctionContext) With(inputColumns map[string]int) *CleansingFunctionContext {
	return &CleansingFunctionContext{
		reMap:                   ctx.reMap,
		argdMap:                 ctx.argdMap,
		parsedFunctionArguments: ctx.parsedFunctionArguments,
		inputColumns:            inputColumns,
	}
}

// inputColumnName can be null
func (ctx *CleansingFunctionContext) ApplyCleasingFunction(functionName *string, argument *string, inputValue *string,
	inputPos int, inputRow *[]interface{}) (string, string) {
	var obj, errMsg string
	var err error
	var sz int
	switch *functionName {

	case "trim":
		obj = strings.TrimSpace(*inputValue)

	case "validate_date":
		_, err2 := rdf.ParseDate(*inputValue)
		if err2 == nil {
			obj = *inputValue
		} else {
			errMsg = err2.Error()
		}

	case "to_upper":
		obj = strings.ToUpper(*inputValue)

	case "to_zip5":
		// Remove non digits characters
		inVal := filterDigits(*inputValue)
		sz = len(inVal)
		switch {
		case sz == 0:
			obj = ""
		case sz < 5:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%05d", v)
				if obj == "00000" {
					obj = ""
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 5:
			obj = inVal
			if obj == "00000" {
				obj = ""
			}
		case sz > 5 && sz < 9:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%09d", v)[:5]
				if obj == "00000" {
					obj = ""
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 9:
			obj = inVal[:5]
			if obj == "00000" {
				obj = ""
			}
		default:
		}

	case "to_zipext4_from_zip9": // from a zip9 input
		// Remove non digits characters
		inVal := filterDigits(*inputValue)
		sz = len(inVal)
		switch {
		case sz == 0:
			obj = ""
		case sz > 5 && sz < 9:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%09d", v)[5:]
				if obj == "0000" {
					obj = ""
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 9:
			obj = inVal[5:]
			if obj == "0000" {
				obj = ""
			}
		default:
		}

	case "to_zipext4": // from a zip ext4 input
		// Remove non digits characters
		inVal := filterDigits(*inputValue)
		sz = len(inVal)
		switch {
		case sz == 0:
			obj = ""
		case sz < 4:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%04d", v)
				if obj == "0000" {
					obj = ""
				}
			} else {
				errMsg = err.Error()
			}
		case sz == 4:
			obj = inVal
			if obj == "0000" {
				obj = ""
			}
		default:
		}

	case "format_phone": // Validate & format phone according to E.164
		// Output: +1 area_code exchange_code subscriber_nbr
		// area_code: 3 digits, 1st digit is not 0 or 1
		// exchange_code: 3 digits, 1st digit is not 0 or 1
		// subscriber_nbr: 4 digits
		// Optional function argument is fmt.Sprintf formatter, expecting 3 string arguments (area_code, exchange_code, subscriber_nbr)
		inVal := filterDigits(*inputValue)
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
		if len(*argument) == 0 {
			*argument = "+1%s%s%s"
		}
		obj = fmt.Sprintf(*argument, areaCode, exchangeCode, subscriberNbr)

	case "reformat0":
		if argument != nil {
			// Remove non digits characters
			inVal := filterDigits(*inputValue)
			var v int
			if len(inVal) == 0 {
				obj = ""
			} else {
				v, err = strconv.Atoi(inVal)
				if err == nil {
					obj = fmt.Sprintf(*argument, v)
				} else {
					errMsg = err.Error()
				}
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function reformat0 for input column pos %d", inputPos)
		}

	case "overpunch_number":
		if argument != nil {
			// Get the number of decimal position
			var npos int
			npos, err = strconv.Atoi(*argument)
			if err == nil {
				obj, err = OverpunchNumber(*inputValue, npos)
				if err != nil {
					errMsg = err.Error()
				}
			} else {
				errMsg = err.Error()
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function overpunch_number for input column pos %d", inputPos)
		}
	case "apply_regex":
		if argument != nil {
			re, ok := ctx.reMap[*argument]
			if !ok {
				re, err = regexp.Compile(*argument)
				if err != nil {
					// configuration error, bailing out
					log.Panicf("ERROR regex argument does not compile: %s", *argument)
				}
				ctx.reMap[*argument] = re
			}
			obj = re.FindString(*inputValue)
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function apply_regex for input column pos %d", inputPos)
		}
	case "scale_units":
		if argument != nil {
			if *argument == "1" {
				obj = filterDouble(*inputValue)
			} else {
				divisor, ok := ctx.argdMap[*argument]
				if !ok {
					divisor, err = strconv.ParseFloat(*argument, 64)
					if err != nil {
						// configuration error, bailing out
						log.Panicf("ERROR divisor argument to function scale_units is not a double: %s", *argument)
					}
					ctx.argdMap[*argument] = divisor
				}
				// Remove non digits characters
				inVal := filterDouble(*inputValue)
				var unit float64
				unit, err = strconv.ParseFloat(inVal, 64)
				if err == nil {
					obj = fmt.Sprintf("%f", math.Ceil(unit/divisor))
				} else {
					errMsg = err.Error()
				}
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function scale_units for input column pos %d", inputPos)
		}
	case "parse_amount":
		// clean up the amount
		inVal := filterDouble(*inputValue)
		if len(inVal) > 0 {
			obj = inVal
			// argument is optional, assume divisor is 1 if absent
			if argument != nil {
				if *argument != "1" {
					divisor, ok := ctx.argdMap[*argument]
					if !ok {
						divisor, err = strconv.ParseFloat(*argument, 64)
						if err != nil {
							// configuration error, bailing out
							log.Panicf("ERROR divisor argument to function scale_units is not a double: %s", *argument)
						}
						ctx.argdMap[*argument] = divisor
					}
					var amt float64
					amt, err = strconv.ParseFloat(obj, 64)
					if err == nil {
						obj = fmt.Sprintf("%f", amt/divisor)
					} else {
						errMsg = err.Error()
					}
				}
			}
		}

	case "concat", "concat_with":
		// Cleansing function that concatenate inputRow columns w delimiter
		// Get the parsed argument
		arg, err := ParseConcatFunctionArgument(argument, *functionName, ctx.inputColumns, ctx.parsedFunctionArguments, inputRow)
		if err != nil {
			errMsg = err.Error()
		} else {
			var buf strings.Builder
			buf.WriteString(*inputValue)
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
			obj = buf.String()
		}

	case "find_and_replace":
		// Cleansing function that replace portion of the input column
		// Get the parsed argument
		arg, err := ParseFindReplaceFunctionArgument(argument, *functionName, ctx.parsedFunctionArguments)
		if err != nil {
			errMsg = err.Error()
		} else {
			obj = strings.ReplaceAll(*inputValue, arg.Find, arg.ReplaceWith)
		}

	case "substring":
		// Cleansing function that takes a substring of input columns
		// Get the parsed argument
		arg, err := ParseSubStringFunctionArgument(argument, *functionName, ctx.parsedFunctionArguments)
		if err != nil {
			errMsg = err.Error()
		} else {
			end := arg.End
			if end < 0 {
				end = len(*inputValue) + end
			}
			if end > len(*inputValue) || end <= arg.Start {
				obj = ""
			} else {
				obj = (*inputValue)[arg.Start:end]
			}
		}

	default:
		log.Panicf("ERROR unknown mapping function: %s", *functionName)
	}

	return obj, errMsg
}
