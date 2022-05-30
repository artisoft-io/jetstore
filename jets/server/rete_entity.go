package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

func createStringLiteral(reteSession *bridge.ReteSession, rdfType string, obj string) (*bridge.Resource, error) {
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
	inBundle *inputBundle,
	// processInput *ProcessInput,
	// inputEntities *[][]interface{},
	writeOutputc *map[string]chan []interface{}) error {
	// Each row in inputEntities is a jets:Entity, with it's own jets:key
	for _, bunRow := range inBundle.inputRows {
		rowl := len(bunRow.inputRows)
		if rowl == 0 {
		continue
		}
		// get the jets:key and create the subject for the row
		jets__key := bunRow.inputRows[bunRow.processInput.keyPosition].(*sql.NullString)
		subject, err := reteSession.NewResource(jets__key.String)
		if !jets__key.Valid || err != nil {
			return fmt.Errorf("while creating row's subject resource (NewResource): %v", err)
		}
		if glogv > 0 {
			log.Printf("Asserting Entity with jets:key %s",jets__key.String)
		}
		// For Each Column
		for icol := 0; icol < ri.ncol; icol++ {
			inputColumnSpec := &bunRow.processInput.processInputMapping[icol]
			var object *bridge.Resource
			var objectArr []*bridge.Resource
			var err error
			if inputColumnSpec.isArray {
				objectArr = make([]*bridge.Resource, 0)
			}

			switch inputColumnSpec.rdfType {
			case "null":
				object, err = reteSession.NewNull()
			case "resource", "text", "date", "datetime":
				if inputColumnSpec.isArray {
					va := bunRow.inputRows[icol].(*[]string)
					for _, item := range *va {
						object, err = createStringLiteral(reteSession, inputColumnSpec.rdfType, item)
						if err != nil {
							goto ERRCHECK
						}
						objectArr = append(objectArr, object)
					}
				} else {
					v := bunRow.inputRows[icol].(*sql.NullString)
					if v.Valid {
						object, err = createStringLiteral(reteSession, inputColumnSpec.rdfType, v.String)
						if err != nil {
							fmt.Printf("ERROR::%v\n",err)
							goto ERRCHECK
						}
					}
				}
			case "int", "bool":
				if inputColumnSpec.isArray {
					va := bunRow.inputRows[icol].(*[]int)
					for _, item := range *va {
						object, err = reteSession.NewIntLiteral(int(item))
						if err != nil {
							goto ERRCHECK
						}
						objectArr = append(objectArr, object)
					}
				} else {
					v := bunRow.inputRows[icol].(*sql.NullInt32)
					if v.Valid {
						object, err = reteSession.NewIntLiteral(int(v.Int32))
						if err != nil {
							goto ERRCHECK
						}
					}
				}
			case "long", "ulong", "uint":
				if inputColumnSpec.isArray {
					va := bunRow.inputRows[icol].(*[]int64)
					for _, item := range *va {
						object, err = reteSession.NewLongLiteral(int64(item))
						if err != nil {
							goto ERRCHECK
						}
						objectArr = append(objectArr, object)
					}
				} else {
					v := bunRow.inputRows[icol].(*sql.NullInt64)
					if v.Valid {
						object, err = reteSession.NewLongLiteral(int64(v.Int64))
						if err != nil {
							goto ERRCHECK
						}
					}
				}
			case "double":
				if inputColumnSpec.isArray {
					va := bunRow.inputRows[icol].(*[]float64)
					for _, item := range *va {
						object, err = reteSession.NewDoubleLiteral(float64(item))
						if err != nil {
							goto ERRCHECK
						}
						objectArr = append(objectArr, object)
					}
				} else {
					v := bunRow.inputRows[icol].(*sql.NullFloat64)
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
				br.RowJetsKey = *jets__key
				gp := bunRow.inputRows[bunRow.processInput.groupingPosition].(*sql.NullString)
				if gp.Valid {
					br.GroupingKey = sql.NullString{String: gp.String, Valid: true}
				}
				br.InputColumn = sql.NullString{String: inputColumnSpec.inputColumn, Valid: true}
				br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting input value to column type: %v", err), Valid: true}
				br.write2Chan((*writeOutputc)["process_errors"])
				continue
			}
			if inputColumnSpec.predicate == nil {
				return fmt.Errorf("ERROR predicate is null")
			}
			if object == nil {
				log.Println("** Object is nil nothing to assert")
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
