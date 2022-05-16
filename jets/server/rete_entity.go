package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

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
		// jetsKey, err := reteSession.NewTextLiteral(jets__key)
		// if err != nil {
		// 	return fmt.Errorf("while creating row's jets:key literal (NewTextLiteral): %v", err)
		// }
		// if subject == nil || ri.rdf__type == nil || processInput.entityRdfTypeResource == nil {
		// 	return fmt.Errorf("ERROR while asserting row rdf type")
		// }
		// _, err = reteSession.Insert(subject, ri.rdf__type, processInput.entityRdfTypeResource)
		// if err != nil {
		// 	return fmt.Errorf("while asserting row rdf type: %v", err)
		// }
		// _, err = reteSession.Insert(subject, ri.jets__key, jetsKey)
		// if err != nil {
		// 	return fmt.Errorf("while asserting row jets key: %v", err)
		// }

		for icol := 0; icol < ri.ncol; icol++ {
			inputColumnSpec := &processInput.processInputMapping[icol]
			var object *bridge.Resource
			var objectArr []*bridge.Resource
			var err error
			if inputColumnSpec.isArray {
				objectArr = make([]*bridge.Resource, 0)
			}
			switch inputColumnSpec.rdfType {

			// case "null":
			// 	object, err = ri.rw.js.NewNull()
			case "resource":
				if inputColumnSpec.isArray {
					va := row[icol].([]sql.NullString)

				}
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
				br.RowJetsKey = sql.NullString{String: jetsKeyStr, Valid: true}
				if row[processInput.groupingPosition].Valid {
					br.GroupingKey = sql.NullString{String: row[processInput.groupingPosition].String, Valid: true}
				}
				br.InputColumn = sql.NullString{String: inputColumnSpec.inputColumn, Valid: true}
				br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting input value to column type: %v", err), Valid: true}
				//*
				fmt.Println("BAD Input ROW:", br)
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
