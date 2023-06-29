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
	"github.com/google/uuid"
)

type ReteInputContext struct {
	jets__client                 *bridge.Resource
	jets__completed              *bridge.Resource
	jets__sourcePeriodType       *bridge.Resource
	jets__currentSourcePeriod    *bridge.Resource
	jets__exception              *bridge.Resource
	jets__input_record           *bridge.Resource
	jets__istate                 *bridge.Resource
	jets__key                    *bridge.Resource
	jets__loop                   *bridge.Resource
	jets__org                    *bridge.Resource
	jets__source_period_sequence *bridge.Resource
	jets__state                  *bridge.Resource
	rdf__type                    *bridge.Resource
	reMap                        map[string]*regexp.Regexp
	argdMap                      map[string]float64
}

// main processing function to execute rules
func (ri *ReteInputContext) assertInputBundle(reteSession *bridge.ReteSession, inBundle *groupedJetRows, writeOutputc *map[string][]chan []interface{}) error {
	// Each row in inputRecords is a jets:Entity, with it's own jets:key
	for _, aJetRow := range inBundle.jetRowSlice {
		rowl := len(aJetRow.rowData)
		if rowl == 0 {
			continue
		}
		var err error
		switch aJetRow.processInput.sourceType {
		case "file":
			err = ri.assertInputTextRecord(reteSession, &aJetRow, writeOutputc)
		case "domain_table", "alias_domain_table":
			err = ri.assertInputEntityRecord(reteSession, &aJetRow, writeOutputc)
		default:
			err = fmt.Errorf("error: unknown source_type in assertInputBundle: %s", aJetRow.processInput.sourceType)
		}
		if err != nil {
			return err
		}
	}
	return nil
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

func castToRdfType(objValue *string, inputColumnSpec *ProcessMap,
	reteSession *bridge.ReteSession) (object *bridge.Resource, err error) {

	switch inputColumnSpec.rdfType {
	// case "null":
	// 	object, err = ri.rw.js.NewNull()
	case "resource":
		object, err = reteSession.NewResource(*objValue)
	case "int":
		var v int
		v, err = strconv.Atoi(strings.TrimSpace(*objValue))
		if err == nil {
			object, err = reteSession.NewIntLiteral(v)
		}
	case "bool":
		v := 0
		if len(*objValue) > 0 {
			c := strings.ToLower((*objValue)[0:1])
			switch c {
			case "t", "1", "y":
				v = 1
			case "f", "0", "n":
				v = 0
			default:
				err = fmt.Errorf("object is not boolean: %s", *objValue)
			}
		}
		if err == nil {
			object, err = reteSession.NewIntLiteral(v)
		}
	case "uint":
		var v uint64
		v, err = strconv.ParseUint(strings.TrimSpace(*objValue), 10, 32)
		if err == nil {
			object, err = reteSession.NewUIntLiteral(uint(v))
		}
	case "long":
		var v int64
		v, err = strconv.ParseInt(strings.TrimSpace(*objValue), 10, 64)
		if err == nil {
			object, err = reteSession.NewLongLiteral(v)
		}
	case "ulong":
		var v uint64
		v, err = strconv.ParseUint(strings.TrimSpace(*objValue), 10, 64)
		if err == nil {
			object, err = reteSession.NewULongLiteral(v)
		}
	case "double":
		var v float64
		v, err = strconv.ParseFloat(strings.TrimSpace(*objValue), 64)
		if err == nil {
			object, err = reteSession.NewDoubleLiteral(v)
		}
	case "text":
		object, err = reteSession.NewTextLiteral(*objValue)
	case "date":
		object, err = reteSession.NewDateLiteral(*objValue)
	case "datetime":
		object, err = reteSession.NewDatetimeLiteral(*objValue)
	default:
		var cn string
		if inputColumnSpec.inputColumn.Valid {
			cn = inputColumnSpec.inputColumn.String
		} else {
			cn = "UNNAMED"
		}
		err = fmt.Errorf("ERROR unknown or invalid type for column %s: %s", cn, inputColumnSpec.rdfType)
	}
	return
}

// main function for asserting input text row (from csv files)
func (ri *ReteInputContext) assertInputTextRecord(reteSession *bridge.ReteSession, aJetRow *jetRow, writeOutputc *map[string][]chan []interface{}) error {
	// Each row in inputRecords is a jets:Entity, with it's own jets:key
	ncol := len(aJetRow.rowData)
	row := make([]sql.NullString, ncol)
	for i := range row {
		row[i] = *aJetRow.rowData[i].(*sql.NullString)
	}
	var jetsKeyStr string
	if row[aJetRow.processInput.keyPosition].Valid {
		jetsKeyStr = row[aJetRow.processInput.keyPosition].String
	} else {
		jetsKeyStr = uuid.New().String()
	}
	subject, err := reteSession.NewResource(jetsKeyStr)
	if err != nil {
		return fmt.Errorf("while creating row's subject resource (NewResource): %v", err)
	}
	jetsKey, err := reteSession.NewTextLiteral(jetsKeyStr)
	if err != nil {
		return fmt.Errorf("while creating row's jets:key literal (NewTextLiteral): %v", err)
	}
	if subject == nil || ri.rdf__type == nil || ri.jets__input_record == nil || aJetRow.processInput.entityRdfTypeResource == nil {
		return fmt.Errorf("ERROR while asserting row rdf type")
	}
	// Assert the rdf:type of the row
	_, err = reteSession.Insert(subject, ri.rdf__type, aJetRow.processInput.entityRdfTypeResource)
	if err != nil {
		return fmt.Errorf("while asserting row rdf type: %v", err)
	}
	_, err = reteSession.Insert(subject, ri.rdf__type, ri.jets__input_record)
	if err != nil {
		return fmt.Errorf("while asserting row rdf type: %v", err)
	}
	// Assert jets:key of the row
	_, err = reteSession.Insert(subject, ri.jets__key, jetsKey)
	if err != nil {
		return fmt.Errorf("while asserting row jets key: %v", err)
	}
	// Asserting client and org (assert empty string if empty)
	v,_ := reteSession.NewTextLiteral(aJetRow.processInput.client)
	reteSession.Insert(subject, ri.jets__client, v)
	v,_ = reteSession.NewTextLiteral(aJetRow.processInput.organization)
	reteSession.Insert(subject, ri.jets__org, v)
	// Assert domain columns of the row
	for icol := 0; icol < ncol; icol++ {
		// asserting input row with mapping spec
		inputColumnSpec := &aJetRow.processInput.processInputMapping[icol]
		// fmt.Println("** assert from table:",inputColumnSpec.tableName,", property:",inputColumnSpec.dataProperty,", value:",row[icol].String,", with rdfTpe",inputColumnSpec.rdfType)
		var obj string
		var err error
		sz := len(row[icol].String)
		if row[icol].Valid && sz > 0 {
			if inputColumnSpec.functionName.Valid {
				switch inputColumnSpec.functionName.String {
				case "trim":
					obj = strings.TrimSpace(row[icol].String)
				case "validate_date":
					_, err = reteSession.NewDateLiteral(row[icol].String)
					if err == nil {
						obj = row[icol].String
					}
				case "to_upper":
					obj = strings.ToUpper(row[icol].String)
				case "to_zip5":
					// Remove non digits characters
					inVal := filterDigits(row[icol].String)
					sz = len(inVal)
					switch {
					case sz < 5:
						var v int
						v, err = strconv.Atoi(inVal)
						if err == nil {
							obj = fmt.Sprintf("%05d", v)
						}
					case sz == 5:
						obj = inVal
					case sz > 5 && sz < 9:
						var v int
						v, err = strconv.Atoi(inVal)
						if err == nil {
							obj = fmt.Sprintf("%09d", v)[:5]
						}
					case sz == 9:
						obj = inVal[:5]
					default:
					}
				case "reformat0":
					if inputColumnSpec.argument.Valid {
						// Remove non digits characters
						inVal := filterDigits(row[icol].String)
						arg := inputColumnSpec.argument.String
						var v int
						v, err = strconv.Atoi(inVal)
						if err == nil {
							obj = fmt.Sprintf(arg, v)
						}
					} else {
						// configuration error, bailing out
						return fmt.Errorf("ERROR missing argument for function reformat0 for input column: %s", inputColumnSpec.inputColumn.String)
					}
				case "overpunch_number":
					if inputColumnSpec.argument.Valid {
						// Get the number of decimal position
						arg := inputColumnSpec.argument.String
						var npos int
						npos, err = strconv.Atoi(arg)
						if err == nil {
							obj, err = OverpunchNumber(row[icol].String, npos)
						}
					} else {
						// configuration error, bailing out
						return fmt.Errorf("ERROR missing argument for function overpunch_number for input column: %s", inputColumnSpec.inputColumn.String)
					}
				case "apply_regex":
					if inputColumnSpec.argument.Valid {
						arg := inputColumnSpec.argument.String
						re, ok := ri.reMap[arg]
						if !ok {
							re, err = regexp.Compile(arg)
							if err != nil {
								// configuration error, bailing out
								return fmt.Errorf("ERROR regex argument does not compile: %s", arg)
							}
							ri.reMap[arg] = re
						}
						obj = re.FindString(row[icol].String)
					} else {
						// configuration error, bailing out
						return fmt.Errorf("ERROR missing argument for function apply_regex for input column: %s", inputColumnSpec.inputColumn.String)
					}
				case "scale_units":
					if inputColumnSpec.argument.Valid {
						arg := inputColumnSpec.argument.String
						if arg == "1" {
							obj = filterDouble(row[icol].String)
						} else {
							divisor, ok := ri.argdMap[arg]
							if !ok {
								divisor, err = strconv.ParseFloat(arg, 64)
								if err != nil {
									// configuration error, bailing out
									return fmt.Errorf("ERROR divisor argument to function scale_units is not a double: %s", arg)
								}
								ri.argdMap[arg] = divisor
							}
							// Remove non digits characters
							inVal := filterDouble(row[icol].String)
							var unit float64
							unit, err = strconv.ParseFloat(inVal, 64)
							if err == nil {
								obj = fmt.Sprintf("%f", math.Ceil(unit/divisor))
							}
						}
					} else {
						// configuration error, bailing out
						return fmt.Errorf("ERROR missing argument for function scale_units for input column: %s", inputColumnSpec.inputColumn.String)
					}
				case "parse_amount":
					// clean up the amount
					inVal := filterDouble(row[icol].String)
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
										return fmt.Errorf("ERROR divisor argument to function scale_units is not a double: %s", arg)
									}
									ri.argdMap[arg] = divisor
								}
								var amt float64
								amt, err = strconv.ParseFloat(obj, 64)
								if err == nil {
									obj = fmt.Sprintf("%f", amt/divisor)
								}
							}
						}
					}
				default:
					return fmt.Errorf("ERROR unknown mapping function: %s", inputColumnSpec.functionName.String)
				}
			} else {
				obj = row[icol].String
			}
		}
		if err != nil || len(obj) == 0 {
			// Value from input is null or empty or mapping function returned err or empty for this property,
			// get the default or report error or ignore the field if no default or error message is avail
			if inputColumnSpec.defaultValue.Valid {
				obj = inputColumnSpec.defaultValue.String
			} else {
				if inputColumnSpec.errorMessage.Valid {
					// report error
					br := NewBadRow()
					br.RowJetsKey = sql.NullString{String: jetsKeyStr, Valid: true}
					if row[aJetRow.processInput.groupingPosition].Valid {
						br.GroupingKey = sql.NullString{String: row[aJetRow.processInput.groupingPosition].String, Valid: true}
					}
					if inputColumnSpec.inputColumn.Valid {
						br.InputColumn = inputColumnSpec.inputColumn
					} else {
						br.InputColumn = sql.NullString{String: "UNNAMED", Valid: true}
					}
					if err != nil {
						br.ErrorMessage = sql.NullString{String: fmt.Sprintf("%v", err), Valid: true}
					} else {
						br.ErrorMessage = inputColumnSpec.errorMessage
					}
					log.Println("Error when mapping input value:", br)
					br.write2Chan((*writeOutputc)["jetsapi.process_errors"][0])
				}
				continue
			}
		}
		// Map client-specific code value to canonical code value
		canonicalObj := aJetRow.processInput.mapCodeValue(&obj, inputColumnSpec)
		// cast obj to type
		object, err := castToRdfType(canonicalObj, inputColumnSpec, reteSession)
		if err != nil {
			// Error casting obj value to colum type
			if inputColumnSpec.defaultValue.Valid {
				obj = inputColumnSpec.defaultValue.String
				object, err = castToRdfType(&obj, inputColumnSpec, reteSession)
			}
			// Check if casting the default value failed or default value is not valid
			if err != nil {
				br := NewBadRow()
				br.RowJetsKey = sql.NullString{String: jetsKeyStr, Valid: true}
				if row[aJetRow.processInput.groupingPosition].Valid {
					br.GroupingKey = sql.NullString{String: row[aJetRow.processInput.groupingPosition].String, Valid: true}
				}
				if inputColumnSpec.inputColumn.Valid {
					br.InputColumn = inputColumnSpec.inputColumn
				} else {
					br.InputColumn = sql.NullString{String: "UNNAMED", Valid: true}
				}
				br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting input value to column type: %v", err), Valid: true}
				log.Println("Error while casting object value to column type:", br)
				br.write2Chan((*writeOutputc)["jetsapi.process_errors"][0])
				continue
			}
		}
		if inputColumnSpec.predicate == nil {
			return fmt.Errorf("ERROR predicate is null")
		}
		if object == nil {
			continue
		}
		_, err = reteSession.Insert(subject, inputColumnSpec.predicate, object)
		if err != nil {
			return fmt.Errorf("while asserting triple to rete session: %v", err)
		}
	}
	return nil
}
