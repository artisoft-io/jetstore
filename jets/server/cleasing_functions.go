package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

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

func (ri *ReteInputContext) applyCleasingFunction(reteSession *bridge.ReteSession, inputColumnSpec *ProcessMap, inputValue *string) (obj, errMsg string, err error) {
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
		case sz < 5:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%05d", v)
			} else {
				errMsg = err.Error()
			}
		case sz == 5:
			obj = inVal
		case sz > 5 && sz < 9:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%09d", v)[:5]
			} else {
				errMsg = err.Error()
			}
		case sz == 9:
			obj = inVal[:5]
		default:
		}
	case "to_zipext4_from_zip9":	// from a zip9 input
		// Remove non digits characters
		inVal := filterDigits(*inputValue)
		sz = len(inVal)
		switch {
		case sz > 5 && sz < 9:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%09d", v)[5:]
			} else {
				errMsg = err.Error()
			}
		case sz == 9:
			obj = inVal[5:]
		default:
		}
	case "to_zipext4":	// from a zip ext4 input
		// Remove non digits characters
		inVal := filterDigits(*inputValue)
		sz = len(inVal)
		switch {
		case sz < 4:
			var v int
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf("%04d", v)
			} else {
				errMsg = err.Error()
			}
		case sz == 4:
			obj = inVal
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
			return
		}
		if inVal[0] == '0' {
			inVal = inVal[1:]
		}
		if inVal[0] == '1' {
			inVal = inVal[1:]
		}
		if len(inVal) < 10 {
			errMsg = "invalid sequence of digits"
			return
		}
		areaCode := inVal[0:3]
		exchangeCode := inVal[3:6]
		subscriberNbr := inVal[6:10]
		if areaCode[0] == '0' || areaCode[0] == '1' {
			errMsg = "invalid area code"
			return
		}
		if exchangeCode[0] == '0' || exchangeCode[0] == '1' {
			errMsg = "invalid exchange code"
			return
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
			v, err = strconv.Atoi(inVal)
			if err == nil {
				obj = fmt.Sprintf(arg, v)
			} else {
				errMsg = err.Error()
			}
		} else {
			// configuration error, bailing out
			return "", "", fmt.Errorf("ERROR missing argument for function reformat0 for input column: %s", inputColumnSpec.inputColumn.String)
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
			return "", "", fmt.Errorf("ERROR missing argument for function overpunch_number for input column: %s", inputColumnSpec.inputColumn.String)
		}
	case "apply_regex":
		if inputColumnSpec.argument.Valid {
			arg := inputColumnSpec.argument.String
			re, ok := ri.reMap[arg]
			if !ok {
				re, err = regexp.Compile(arg)
				if err != nil {
					// configuration error, bailing out
					return "", "", fmt.Errorf("ERROR regex argument does not compile: %s", arg)
				}
				ri.reMap[arg] = re
			}
			obj = re.FindString(*inputValue)
		} else {
			// configuration error, bailing out
			return "", "", fmt.Errorf("ERROR missing argument for function apply_regex for input column: %s", inputColumnSpec.inputColumn.String)
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
						return "", "", fmt.Errorf("ERROR divisor argument to function scale_units is not a double: %s", arg)
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
			return "", "", fmt.Errorf("ERROR missing argument for function scale_units for input column: %s", inputColumnSpec.inputColumn.String)
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
							return "", "", fmt.Errorf("ERROR divisor argument to function scale_units is not a double: %s", arg)
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
	default:
		return "", "", fmt.Errorf("ERROR unknown mapping function: %s", inputColumnSpec.functionName.String)
	}

	return
}