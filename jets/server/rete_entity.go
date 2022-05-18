package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

func createStringLiteral(reteSession *bridge.ReteSession, rdfType string, obj string) (*bridge.Resource, error) {
	//*
	fmt.Println("CreateStringLiteral called with",obj,"of type",rdfType)
	switch rdfType {
	case "resource":
		return reteSession.NewResource(obj)
	case "text":
		return reteSession.NewTextLiteral(obj)
	case "date":
		return reteSession.NewDateLiteral(obj)
	case "datetime":
		return reteSession.NewDatetimeLiteral(obj)
	case "int":
		v, err := strconv.Atoi(obj)
		if err != nil {
			return nil, fmt.Errorf("error casting int: %v", err)
		}
		return reteSession.NewIntLiteral(v)
	default:
		return nil, fmt.Errorf("ERROR Incorrect type %s for CreateStringLiteral",rdfType)
	}
}

// main processing function to execute rules
func (ri *ReteInputContext) assertEntities(
	reteSession *bridge.ReteSession,
	processInput *ProcessInput,
	inputEntities *[][]interface{},
	writeOutputc *map[string]chan []interface{}) error {

	// Each row in inputEntities is a jets:Entity, with it's own jets:key
	for _, row := range *inputEntities {
		if len(row) == 0 {
			continue
		}
		// get the jets:key and create the subject for the row
		var jets__key string
		jk := row[processInput.keyPosition].(sql.NullString)
		if jk.Valid {
			jets__key = jk.String
		} else {
			fmt.Println("ERROR entity with null jets:key")
			return fmt.Errorf("ERROR entity with null jets:key")
		}
		subject, err := reteSession.NewResource(jets__key)
		if err != nil {
			return fmt.Errorf("while creating row's subject resource (NewResource): %v", err)
		}
		// For Each Column
		for icol := 0; icol < ri.ncol; icol++ {
			inputColumnSpec := &processInput.processInputMapping[icol]
			var object *bridge.Resource
			var objectArr []*bridge.Resource
			var err error
			if inputColumnSpec.isArray {
				objectArr = make([]*bridge.Resource, 0)
			}
			fmt.Println("Conversion",inputColumnSpec.rdfType,"on",inputColumnSpec.inputColumn)

			switch inputColumnSpec.rdfType {
			// case "null":
			// 	object, err = ri.rw.js.NewNull()
			case "resource", "text", "date", "datetime", "int":
				// if inputColumnSpec.isArray {
				// 	//*
				// 	fmt.Println("~~Got array for",inputColumnSpec.inputColumn)
				// 	va := row[icol].([]sql.NullString)
				// 	for _, item := range va {
				// 		if item.Valid {
				// 			object, err = createStringLiteral(reteSession, inputColumnSpec.rdfType, item.String)
				// 			if err != nil {
				// 				goto ERRCHECK
				// 			}
				// 			objectArr = append(objectArr, object)
				// 		}
				// 	}
				// } else {
					v := row[icol].(sql.NullString)
					if v.Valid {
						object, err = createStringLiteral(reteSession, inputColumnSpec.rdfType, v.String)
						if err != nil {
							fmt.Printf("ERROR::%v\n",err)
							goto ERRCHECK
						}
						str, _ := object.AsText()
						fmt.Println("###### We are here object",str,"of type",object.GetType())
					} else {
						//*
						fmt.Println("**Got null for",inputColumnSpec.inputColumn)
					}
				// }
			case "bool":
			// case "int":
				if inputColumnSpec.isArray {
					va := row[icol].([]sql.NullInt32)
					for _, item := range va {
						if item.Valid {
							object, err = reteSession.NewIntLiteral(int(item.Int32))
							if err != nil {
								goto ERRCHECK
							}
							objectArr = append(objectArr, object)
						}
					}
				} else {
					v := row[icol].(sql.NullInt32)
					if v.Valid {
						object, err = reteSession.NewIntLiteral(int(v.Int32))
						if err != nil {
							goto ERRCHECK
						}
					}
				}
			case "long", "ulong", "uint":
				if inputColumnSpec.isArray {
					va := row[icol].([]sql.NullInt64)
					for _, item := range va {
						if item.Valid {
							object, err = reteSession.NewLongLiteral(int64(item.Int64))
							if err != nil {
								goto ERRCHECK
							}
							objectArr = append(objectArr, object)
						}
					}
				} else {
					v := row[icol].(sql.NullInt64)
					if v.Valid {
						object, err = reteSession.NewLongLiteral(int64(v.Int64))
						if err != nil {
							goto ERRCHECK
						}
					}
				}
			case "double":
				if inputColumnSpec.isArray {
					va := row[icol].([]sql.NullFloat64)
					for _, item := range va {
						if item.Valid {
							object, err = reteSession.NewDoubleLiteral(float64(item.Float64))
							if err != nil {
								goto ERRCHECK
							}
							objectArr = append(objectArr, object)
						}
					}
				} else {
					v := row[icol].(sql.NullFloat64)
					if v.Valid {
						object, err = reteSession.NewDoubleLiteral(float64(v.Float64))
						if err != nil {
							goto ERRCHECK
						}
					}
				}
			default:
				err = fmt.Errorf("ERROR unknown or invalid type for column %s: %s", inputColumnSpec.inputColumn, inputColumnSpec.rdfType)
			}
			ERRCHECK:
			if err != nil {
				var br BadRow
				br.RowJetsKey = sql.NullString{String: jets__key, Valid: true}
				gp := row[processInput.groupingPosition].(sql.NullString)
				if gp.Valid {
					br.GroupingKey = sql.NullString{String: gp.String, Valid: true}
				}
				br.InputColumn = sql.NullString{String: inputColumnSpec.inputColumn, Valid: true}
				br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting input value to column type: %v", err), Valid: true}
				//*
				fmt.Println("BAD Input Entity ROW:", br)
				br.write2Chan((*writeOutputc)["process_errors"])
				continue
			}
			if inputColumnSpec.predicate == nil {
				return fmt.Errorf("ERROR predicate is null")
			}
			if object == nil {
				//*
				fmt.Println("**Object is nil nothing to assert")
				continue
			}
			// This is when we insert!....
			if inputColumnSpec.isArray {
				for _, obj_ := range objectArr {
					_, err = reteSession.Insert(subject, inputColumnSpec.predicate, obj_)
				}
			} else {
				_, err = reteSession.Insert(subject, inputColumnSpec.predicate, object)
			}
			if err != nil {
				return fmt.Errorf("while asserting triple to rete session: %v", err)
			}

		}
	}
	return nil
}
