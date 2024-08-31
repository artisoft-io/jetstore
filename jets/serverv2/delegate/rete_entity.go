package delegate

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/bridgego"
	"github.com/google/uuid"
)

func createStringLiteral(reteSession *bridgego.ReteSession, rdfType string, obj string) (*bridgego.Resource, error) {
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
		return nil, fmt.Errorf("ERROR Incorrect type %s for CreateStringLiteral", rdfType)
	}
}

// main function for asserting input entity row (from persisted entities)
func (ri *ReteInputContext) assertInputEntityRecord(reteSession *bridgego.ReteSession, inBundleRow *jetRow, writeOutputc *map[string][]chan []interface{}) error {
	// // For development
	// log.Println("ASSERT ENTITY:")
	// for ipos := range inBundleRow.rowData {
	// 	log.Println("    ",inBundleRow.processInput.processInputMapping[ipos].dataProperty,"  =  ",inBundleRow.rowData[ipos], ", range ",inBundleRow.processInput.processInputMapping[ipos].rdfType,", array?",inBundleRow.processInput.processInputMapping[ipos].isArray)
	// }
	// get the jets:key and create the subject for the row
	// if it's an alias_domain_table, assign a new jets:key unless Class Name is unchanged
	isAliasTable := false
	var jetsKey, tagName string

	if inBundleRow.processInput.sourceType == "alias_domain_table" {
		tagName = "Alias Domain" // for printing only
		if inBundleRow.processInput.entityRdfType != inBundleRow.processInput.tableName {
			isAliasTable = true // apply special processinf: assign new jets:key and rdf:type
		}
	} else {
		tagName = "Domain"
	}

	if isAliasTable {
		jetsKey = uuid.New().String()
	} else {
		jets__key := inBundleRow.rowData[inBundleRow.processInput.keyPosition].(*sql.NullString)
		if !jets__key.Valid {
			return fmt.Errorf("error jets:key in input row is not valid")
		}
		jetsKey = jets__key.String
	}

	subject, err := reteSession.NewResource(jetsKey)
	if err != nil {
		return fmt.Errorf("while creating row's subject resource (NewResource): %v", err)
	}
	if glogv > 2 {
		log.Printf("Asserting %s Entity with jets:key %s", tagName, jetsKey)
	}
	// For Each Column
	// Note that default value from mapping is not applied when input value (inBundleRow) is null
	ncol := len(inBundleRow.rowData)
	for icol := 0; icol < ncol; icol++ {
		inputColumnSpec := &inBundleRow.processInput.processInputMapping[icol]
		var object *bridgego.Resource
		var objectArr []*bridgego.Resource
		var err error
		if inputColumnSpec.isArray {
			objectArr = make([]*bridgego.Resource, 0)
		}

		// check for special case, alias input record
		if isAliasTable {
			switch inputColumnSpec.inputColumn.String {
			case "rdf:type":
				// intercept the rdf:type and put the alias one instread of the one comming from the read
				object, err = reteSession.NewResource(inBundleRow.processInput.entityRdfType)
				objectArr = append(objectArr, object)
				goto ERRCHECK
			case "jets:key":
				// intercept the jets:key and put the alias one instread of the one comming from the read
				object, err = reteSession.NewTextLiteral(jetsKey)
				goto ERRCHECK
			}
		}

		switch inputColumnSpec.rdfType {
		case "null":
			object, err = reteSession.NewNull()
		case "resource", "text", "date", "datetime":
			if inputColumnSpec.isArray {
				va := inBundleRow.rowData[icol].(*[]string)
				for _, item := range *va {
					object, err = createStringLiteral(reteSession, inputColumnSpec.rdfType, item)
					if err != nil {
						goto ERRCHECK
					}
					objectArr = append(objectArr, object)
				}
			} else {
				v := inBundleRow.rowData[icol].(*sql.NullString)
				if v.Valid {
					object, err = createStringLiteral(reteSession, inputColumnSpec.rdfType, v.String)
					if err != nil {
						fmt.Printf("ERROR::%v\n", err)
						goto ERRCHECK
					}
				}
			}
		case "int", "bool":
			if inputColumnSpec.isArray {
				va := inBundleRow.rowData[icol].(*[]int)
				for _, item := range *va {
					object, err = reteSession.NewIntLiteral(int(item))
					if err != nil {
						goto ERRCHECK
					}
					objectArr = append(objectArr, object)
				}
			} else {
				v := inBundleRow.rowData[icol].(*sql.NullInt32)
				if v.Valid {
					object, err = reteSession.NewIntLiteral(int(v.Int32))
					if err != nil {
						goto ERRCHECK
					}
				}
			}
		case "long", "ulong", "uint":
			if inputColumnSpec.isArray {
				va := inBundleRow.rowData[icol].(*[]int64)
				for _, item := range *va {
					object, err = reteSession.NewLongLiteral(int64(item))
					if err != nil {
						goto ERRCHECK
					}
					objectArr = append(objectArr, object)
				}
			} else {
				v := inBundleRow.rowData[icol].(*sql.NullInt64)
				if v.Valid {
					object, err = reteSession.NewLongLiteral(int64(v.Int64))
					if err != nil {
						goto ERRCHECK
					}
				}
			}
		case "double":
			if inputColumnSpec.isArray {
				va := inBundleRow.rowData[icol].(*[]float64)
				for _, item := range *va {
					object, err = reteSession.NewDoubleLiteral(float64(item))
					if err != nil {
						goto ERRCHECK
					}
					objectArr = append(objectArr, object)
				}
			} else {
				v := inBundleRow.rowData[icol].(*sql.NullFloat64)
				if v.Valid {
					object, err = reteSession.NewDoubleLiteral(float64(v.Float64))
					if err != nil {
						goto ERRCHECK
					}
				}
			}
		default:
			err = fmt.Errorf("ERROR unknown or invalid type for column %s: %s", inputColumnSpec.inputColumn.String, inputColumnSpec.rdfType)
		}
	ERRCHECK:
		if err != nil {
			br := NewBadRow()
			br.RowJetsKey = sql.NullString{String: jetsKey, Valid: true}
			gp := inBundleRow.rowData[inBundleRow.processInput.groupingPosition].(*sql.NullString)
			if gp.Valid {
				br.GroupingKey = sql.NullString{String: gp.String, Valid: true}
			}
			if inputColumnSpec.inputColumn.Valid {
				br.InputColumn = sql.NullString{String: inputColumnSpec.inputColumn.String, Valid: true}
			} else {
				br.InputColumn = sql.NullString{String: "UNNAMED", Valid: true}
			}
			br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting input value to column type: %v", err), Valid: true}
			br.write2Chan((*writeOutputc)["jetsapi.process_errors"][0])
			continue
		}
		if inputColumnSpec.predicate == nil {
			return fmt.Errorf("ERROR predicate is null")
		}
		if object == nil {
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
	return nil
}
