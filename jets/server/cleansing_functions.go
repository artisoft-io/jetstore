package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
)

type ConcatFunctionArg struct {
	Delimit string
	ColumnPositions []int
}

func ParseConcatFunctionArgument(rawArg *string, functionName string, inputColumnName2Pos map[string]int, cache map[string]interface{}) (*ConcatFunctionArg, error) {
	// rawArg is csv-encoded
	if rawArg == nil {
		return nil, fmt.Errorf("unexpected null argument to concat or concat_with function")
	}
	// Check if we have it cached
	v := cache[*rawArg]
	if v != nil {
		// fmt.Println("*** OK Got Cached value for", *rawArg)
		return v.(*ConcatFunctionArg), nil
	}
	// Parsed the raw argument into ConcatFunctionArg and put it in the cache
	rows, err := jcsv.Parse(*rawArg)
	if len(rows)==0 || len(rows[0])==0 || err != nil {
		// It's not csv or there's no data
		return nil, fmt.Errorf("error:no-data: argument %s cannot be parsed as csv: %v (%s function)", *rawArg, err, functionName)
	}
	results := &ConcatFunctionArg{
		ColumnPositions: make([]int, 0),
	}
	for i := range rows[0] {
		if i==0 && functionName=="concat_with" {
			results.Delimit = rows[0][i]
		} else {
			colPos, ok := inputColumnName2Pos[rows[0][i]]
			// fmt.Println("*** concat:",row[i],"value @:", colPos,"ok?",ok)
			if !ok {
				// Column not found
				return nil, fmt.Errorf("error:column-not-fount: argument %s is not an input column name (%s function)", *rawArg, functionName)
			}
			results.ColumnPositions = append(results.ColumnPositions, colPos)
		}
	}
	cache[*rawArg] = results
	return results, nil
}

func filterDigits(str string) string {
	// Remove non digits characters
	var buf strings.Builder
	for _, c := range str {
		if unicode.IsDigit(c) {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

func filterDouble(str string) string {
	// clean up the amount
	var buf strings.Builder
	var c rune
	for _, c = range str {
		if c == '(' || c == '-' {
			buf.WriteRune('-')
		} else if unicode.IsDigit(c) || c == '.' {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

func (ri *ReteInputContext) applyCleasingFunction(reteSession *bridge.ReteSession, inputColumnSpec *ProcessMap, 
	inputValue *string, row []sql.NullString, inputColumnName2Pos map[string]int) (string, string) {
	var obj, errMsg string
	var err error
	var sz int
	switch inputColumnSpec.functionName.String {
	case "trim":
		obj = strings.TrimSpace(*inputValue)
	case "validate_date":
		_, err = reteSession.NewDateLiteral(*inputValue)
		if err == nil {
			obj = *inputValue
		} else {
			errMsg = err.Error()
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
	case "to_zipext4_from_zip9":	// from a zip9 input
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
	case "to_zipext4":	// from a zip ext4 input
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
		arg := inputColumnSpec.argument.String
		if len(arg) == 0 {
			arg = "+1%s%s%s"
		}
		obj = fmt.Sprintf(arg, areaCode, exchangeCode, subscriberNbr)

	case "reformat0":
		if inputColumnSpec.argument.Valid {
			// Remove non digits characters
			inVal := filterDigits(*inputValue)
			arg := inputColumnSpec.argument.String
			var v int
			if len(inVal) == 0 {
				obj = ""
			} else {
				v, err = strconv.Atoi(inVal)
				if err == nil {
					obj = fmt.Sprintf(arg, v)
				} else {
					errMsg = err.Error()
				}	
			}
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function reformat0 for input column: %s", inputColumnSpec.inputColumn.String)
		}
	case "overpunch_number":
		if inputColumnSpec.argument.Valid {
			// Get the number of decimal position
			arg := inputColumnSpec.argument.String
			var npos int
			npos, err = strconv.Atoi(arg)
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
			log.Panicf("ERROR missing argument for function overpunch_number for input column: %s", inputColumnSpec.inputColumn.String)
		}
	case "apply_regex":
		if inputColumnSpec.argument.Valid {
			arg := inputColumnSpec.argument.String
			re, ok := ri.reMap[arg]
			if !ok {
				re, err = regexp.Compile(arg)
				if err != nil {
					// configuration error, bailing out
					log.Panicf("ERROR regex argument does not compile: %s", arg)
				}
				ri.reMap[arg] = re
			}
			obj = re.FindString(*inputValue)
		} else {
			// configuration error, bailing out
			log.Panicf("ERROR missing argument for function apply_regex for input column: %s", inputColumnSpec.inputColumn.String)
		}
	case "scale_units":
		if inputColumnSpec.argument.Valid {
			arg := inputColumnSpec.argument.String
			if arg == "1" {
				obj = filterDouble(*inputValue)
			} else {
				divisor, ok := ri.argdMap[arg]
				if !ok {
					divisor, err = strconv.ParseFloat(arg, 64)
					if err != nil {
						// configuration error, bailing out
						log.Panicf("ERROR divisor argument to function scale_units is not a double: %s", arg)
					}
					ri.argdMap[arg] = divisor
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
			log.Panicf("ERROR missing argument for function scale_units for input column: %s", inputColumnSpec.inputColumn.String)
		}
	case "parse_amount":
		// clean up the amount
		inVal := filterDouble(*inputValue)
		if len(inVal) > 0 {
			obj = inVal
			// argument is optional, assume divisor is 1 if absent
			if inputColumnSpec.argument.Valid {
				arg := inputColumnSpec.argument.String
				if arg != "1" {
					divisor, ok := ri.argdMap[arg]
					if !ok {
						divisor, err = strconv.ParseFloat(arg, 64)
						if err != nil {
							// configuration error, bailing out
							log.Panicf("ERROR divisor argument to function scale_units is not a double: %s", arg)
						}
						ri.argdMap[arg] = divisor
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
		// Cleansing function that concatenate input columns w delimiter
		// Get the parsed argument
		arg, err := ParseConcatFunctionArgument(&inputColumnSpec.argument.String, inputColumnSpec.functionName.String, inputColumnName2Pos, ri.parsedFunctionArguments)
		if err != nil {
			errMsg = err.Error()
		} else {
			var buf strings.Builder
			buf.WriteString(*inputValue)
			for i := range arg.ColumnPositions {
				// fmt.Println("=== concat value @pos:",arg.ColumnPositions[i])
				if row[arg.ColumnPositions[i]].Valid {
					if arg.Delimit != "" {
						buf.WriteString(arg.Delimit)
					}
					buf.WriteString(row[arg.ColumnPositions[i]].String)
				}
			}
			obj = buf.String()		
		}

	default:
		log.Panicf("ERROR unknown mapping function: %s", inputColumnSpec.functionName.String)
	}

	return obj, errMsg
}