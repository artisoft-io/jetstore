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
	ncol int // len(processInput.processInputMapping)
	jets__completed *bridge.Resource
	jets__istate *bridge.Resource
	jets__key *bridge.Resource
	jets__loop *bridge.Resource
	jets__state *bridge.Resource
	rdf__type *bridge.Resource
	reMap map[string]*regexp.Regexp
	argdMap map[string]float64
}

// main processing function to execute rules
func (ri *ReteInputContext) assertInputRecords(
	reteSession *bridge.ReteSession,
	inBundle *inputBundle,
	writeOutputc *map[string]chan []interface{}) error {

	// Each row in inputRecords is a jets:Entity, with it's own jets:key
	for _, bunRow := range inBundle.inputRows {
		rowl := len(bunRow.inputRows)
		if rowl == 0 {
			continue
		}
		row := make([]sql.NullString, rowl)
		for i := range row {
			row[i] = *bunRow.inputRows[i].(*sql.NullString)
		}
		var jetsKeyStr string
		if row[bunRow.processInput.keyPosition].Valid {
			jetsKeyStr = row[bunRow.processInput.keyPosition].String
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
		if subject == nil || ri.rdf__type == nil || bunRow.processInput.entityRdfTypeResource == nil {
			return fmt.Errorf("ERROR while asserting row rdf type")
		}
		_, err = reteSession.Insert(subject, ri.rdf__type, bunRow.processInput.entityRdfTypeResource)
		if err != nil {
			return fmt.Errorf("while asserting row rdf type: %v", err)
		}
		_, err = reteSession.Insert(subject, ri.jets__key, jetsKey)
		if err != nil {
			return fmt.Errorf("while asserting row jets key: %v", err)
		}
		for icol := 0; icol < ri.ncol; icol++ {
			// asserting input row with mapping spec
			inputColumnSpec := &bunRow.processInput.processInputMapping[icol]
			var obj string
			var err error
			sz := len(row[icol].String)
			if row[icol].Valid && sz>0 {
				if inputColumnSpec.functionName.Valid {
					switch inputColumnSpec.functionName.String {
					case "to_upper":
						obj = strings.ToUpper(row[icol].String)
					case "to_zip5":
						switch {
						case sz < 5:
							var v int
							v, err = strconv.Atoi(row[icol].String)
							if err == nil {
								obj = fmt.Sprintf("%05d", v)
							}
						case sz == 5:
							obj = row[icol].String
						case sz>5 && sz<9:
							var v int
							v, err = strconv.Atoi(row[icol].String)
							if err == nil {
								obj = fmt.Sprintf("%09d", v)[:5]
							}
						case sz == 9:
							obj = row[icol].String[:5]
						default:
						}
					case "reformat0":
						if inputColumnSpec.argument.Valid {
							arg := inputColumnSpec.argument.String
							var v int
							v, err = strconv.Atoi(row[icol].String)
							if err == nil {
								obj = fmt.Sprintf(arg, v)
							}
						} else {
							// configuration error, bailing out
							return fmt.Errorf("ERROR missing argument for function reformat0 for input column: %s", inputColumnSpec.inputColumn)
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
							return fmt.Errorf("ERROR missing argument for function apply_regex for input column: %s", inputColumnSpec.inputColumn)
						}
					case "scale_units":
						if inputColumnSpec.argument.Valid {
							arg := inputColumnSpec.argument.String
							if arg == "1" {
								obj = row[icol].String
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
								var unit float64
								unit, err = strconv.ParseFloat(row[icol].String, 64)
								if err == nil {
									obj = fmt.Sprintf("%f", math.Ceil(unit/divisor))	
								}
							}
						} else {
							// configuration error, bailing out
							return fmt.Errorf("ERROR missing argument for function scale_units for input column: %s", inputColumnSpec.inputColumn)
						}
					case "parse_amount":
						// clean up the amount
						var buf strings.Builder
						var c rune
						for _,c = range row[icol].String {
							if c=='(' || c=='-' {
								buf.WriteRune('-')
							} else if unicode.IsDigit(c) || c=='.' {
								buf.WriteRune(c)
							}
						}
						if buf.Len() > 0 {
							obj = buf.String()
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
			if err!=nil || len(obj) == 0 {
				// get the default or report error or ignore the filed if no default or error message is avail
				if inputColumnSpec.defaultValue.Valid {
					obj = inputColumnSpec.defaultValue.String
				} else {
					if inputColumnSpec.errorMessage.Valid {
						// report error
						var br BadRow
						br.RowJetsKey = sql.NullString{String:jetsKeyStr, Valid: true}
						if row[bunRow.processInput.groupingPosition].Valid {
							br.GroupingKey = sql.NullString{String: row[bunRow.processInput.groupingPosition].String, Valid: true}
						}
						br.InputColumn = sql.NullString{String:inputColumnSpec.inputColumn, Valid: true}
						if err != nil {
							br.ErrorMessage = sql.NullString{String: fmt.Sprintf("%v", err), Valid: true}
						} else {
							br.ErrorMessage = inputColumnSpec.errorMessage
						}
						log.Println("BAD Input ROW:",br)
						br.write2Chan((*writeOutputc)["process_errors"])
					}
					continue
				}
			}
			// cast obj to type
			// switch inputColumn.DataType {
			var object *bridge.Resource
			switch inputColumnSpec.rdfType {
			// case "null":
			// 	object, err = ri.rw.js.NewNull()
			case "resource":
				object, err = reteSession.NewResource(obj)
			case "int":
				var v int
				v, err = strconv.Atoi(obj)
				if err == nil {
					object, err = reteSession.NewIntLiteral(v)
				}
			case "bool":
				v := 0
				if len(obj) > 0 {
					c := strings.ToLower(obj[0:1])
					switch c {
					case "t", "1", "y":
						v = 1
					case "f", "0", "n":
						v = 0
					default:
						err = fmt.Errorf("object is not boolean: %s", obj)
					}
				}
				if err == nil {
					object, err = reteSession.NewIntLiteral(v)
				}
			case "uint":
				var v uint64
				v, err = strconv.ParseUint(obj, 10, 32)
				if err == nil {
					object, err = reteSession.NewUIntLiteral(uint(v))
				}
			case "long":
				var v int64
				v, err = strconv.ParseInt(obj, 10, 64)
				if err == nil {
					object, err = reteSession.NewLongLiteral(v)
				}
			case "ulong":
				var v uint64
				v, err = strconv.ParseUint(obj, 10, 64)
				if err != nil {
					return fmt.Errorf("while mapping input value: %v", err)
				}
				object, err = reteSession.NewULongLiteral(v)
			case "double":
				var v float64
				v, err = strconv.ParseFloat(obj, 64)
				if err == nil {
					object, err = reteSession.NewDoubleLiteral(v)
				}
			case "text":
				object, err = reteSession.NewTextLiteral(obj)
			case "date":
				object, err = reteSession.NewDateLiteral(obj)
			case "datetime":
				object, err = reteSession.NewDatetimeLiteral(obj)
			default:
				err = fmt.Errorf("ERROR unknown or invalid type for column %s: %s", inputColumnSpec.inputColumn, inputColumnSpec.rdfType)
			}
			if err != nil {
				var br BadRow
				br.RowJetsKey = sql.NullString{String:jetsKeyStr, Valid: true}
				if row[bunRow.processInput.groupingPosition].Valid {
					br.GroupingKey = sql.NullString{String: row[bunRow.processInput.groupingPosition].String, Valid: true}
				}
				br.InputColumn = sql.NullString{String:inputColumnSpec.inputColumn, Valid: true}
				br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting input value to column type: %v", err), Valid: true}
				log.Println("BAD Input ROW:",br)
				br.write2Chan((*writeOutputc)["process_errors"])
				continue
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
	}
	return nil
}
