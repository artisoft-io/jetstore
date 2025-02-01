package delegate

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridgego"
	"github.com/artisoft-io/jetstore/jets/cleansing_functions"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
)

type ReteInputContext struct {
	jets__client                  *bridgego.Resource
	jets__completed               *bridgego.Resource
	jets__currentSourcePeriod     *bridgego.Resource
	jets__currentSourcePeriodDate *bridgego.Resource
	jets__exception               *bridgego.Resource
	jets__input_record            *bridgego.Resource
	jets__istate                  *bridgego.Resource
	jets__key                     *bridgego.Resource
	jets__loop                    *bridgego.Resource
	jets__org                     *bridgego.Resource
	jets__source_period_sequence  *bridgego.Resource
	jets__sourcePeriodType        *bridgego.Resource
	jets__state                   *bridgego.Resource
	rdf__type                     *bridgego.Resource
	cleansingFunctionContext      *cleansing_functions.CleansingFunctionContext
}

// main processing function to execute rules
func (ri *ReteInputContext) assertInputBundle(reteSession *bridgego.ReteSession, inBundle *groupedJetRows, writeOutputc *map[string][]chan []interface{}) error {
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

func castToRdfType(objValue interface{}, inputColumnSpec *ProcessMap,
	reteSession *bridgego.ReteSession) (object interface{}, err error) {
	if objValue == nil {
		return nil, nil
	}
	var inputV string
	var inputArr []string
	var outArr []*bridgego.Resource
	switch vv := objValue.(type) {
	case string:
		if len(vv) == 0 {
			return nil, nil
		}
		inputV = vv
	case []string:
		if len(vv) == 0 {
			return nil, nil
		}
		inputArr = vv
		outArr = make([]*bridgego.Resource, 0, len(vv))
	default:
		// humm, expecting string or []string
		inputV = fmt.Sprintf("%v", vv)
	}
	switch inputColumnSpec.rdfType {

	case "text":
		if inputArr == nil {
			return reteSession.NewTextLiteral(inputV)
		}
		for _, v := range inputArr {
			r, err := reteSession.NewTextLiteral(v)
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	case "date":
		if inputArr == nil {
			return reteSession.NewDateLiteral(inputV)
		}
		for _, v := range inputArr {
			r, err := reteSession.NewDateLiteral(v)
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	case "double":
		if inputArr == nil {
			vi, err := strconv.ParseFloat(strings.TrimSpace(inputV), 64)
			if err != nil {
				return nil, err
			}
			return reteSession.NewDoubleLiteral(vi)
		}
		for _, v := range inputArr {
			vi, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, err
			}
			r, err := reteSession.NewDoubleLiteral(vi)
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	case "int", "uint", "long", "ulong", "integer":
		if inputArr == nil {
			vi, err := strconv.Atoi(strings.TrimSpace(inputV))
			if err != nil {
				return nil, err
			}
			return reteSession.NewIntLiteral(vi)
		}
		for _, v := range inputArr {
			vi, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, err
			}
			r, err := reteSession.NewIntLiteral(vi)
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	case "bool":
		if inputArr == nil {
			return reteSession.NewIntLiteral(rdf.ParseBool(inputV))
		}
		for _, v := range inputArr {
			r, err := reteSession.NewIntLiteral(rdf.ParseBool(v))
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	case "resource":
		if inputArr == nil {
			return reteSession.NewResource(inputV)
		}
		for _, v := range inputArr {
			r, err := reteSession.NewResource(v)
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	case "datetime":
		if inputArr == nil {
			return reteSession.NewDatetimeLiteral(inputV)
		}
		for _, v := range inputArr {
			r, err := reteSession.NewDatetimeLiteral(v)
			if err != nil {
				return nil, err
			}
			outArr = append(outArr, r)
		}
		return outArr, nil

	default:
		var cn string
		if inputColumnSpec.inputColumn.Valid {
			cn = inputColumnSpec.inputColumn.String
		} else {
			cn = "UNNAMED"
		}
		log.Panicf("ERROR unknown or invalid type for column %s: %s", cn, inputColumnSpec.rdfType)
		return
	}
}

// main function for asserting input text row (from csv files)
func (ri *ReteInputContext) assertInputTextRecord(reteSession *bridgego.ReteSession, aJetRow *jetRow, writeOutputc *map[string][]chan []interface{}) error {
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
		log.Panicf("while creating row's subject resource (NewResource): %v", err)
	}
	jetsKey, err := reteSession.NewTextLiteral(jetsKeyStr)
	if err != nil {
		log.Panicf("while creating row's jets:key literal (NewTextLiteral): %v", err)
	}
	if subject == nil || ri.rdf__type == nil || ri.jets__input_record == nil || aJetRow.processInput.entityRdfTypeResource == nil {
		log.Panicf("ERROR while asserting row rdf type")
	}
	// Assert the rdf:type of the row
	_, err = reteSession.Insert(subject, ri.rdf__type, aJetRow.processInput.entityRdfTypeResource)
	if err != nil {
		log.Panicf("while asserting row rdf type: %v", err)
	}
	_, err = reteSession.Insert(subject, ri.rdf__type, ri.jets__input_record)
	if err != nil {
		log.Panicf("while asserting row rdf type: %v", err)
	}
	// Assert jets:key of the row
	_, err = reteSession.Insert(subject, ri.jets__key, jetsKey)
	if err != nil {
		log.Panicf("while asserting row jets key: %v", err)
	}
	// Asserting client and org (assert empty string if empty)
	v, _ := reteSession.NewTextLiteral(aJetRow.processInput.client)
	reteSession.Insert(subject, ri.jets__client, v)
	v, _ = reteSession.NewTextLiteral(aJetRow.processInput.organization)
	reteSession.Insert(subject, ri.jets__org, v)
	// Set the column name to pos according to aJetRow.processInput
	ri.cleansingFunctionContext = ri.cleansingFunctionContext.With(&aJetRow.processInput.inputColumnName2Pos)
	// Assert domain columns of the row
	for icol := 0; icol < ncol; icol++ {
		// asserting input row with mapping spec
		inputColumnSpec := &aJetRow.processInput.processInputMapping[icol]
		// fmt.Println("** assert from table:",inputColumnSpec.tableName,", property:",inputColumnSpec.dataProperty,", value:",row[icol].String,", with rdfTpe",inputColumnSpec.rdfType)
		var obj interface{}
		var errMsg string
		var err error
		sz := len(row[icol].String)
		if row[icol].Valid && sz > 0 {
			if inputColumnSpec.functionName.Valid {
				// Apply cleansing function
				obj, errMsg =
					ri.cleansingFunctionContext.ApplyCleasingFunction(inputColumnSpec.functionName.String,
						inputColumnSpec.argument.String, row[icol].String, icol, &aJetRow.rowData)
			} else {
				if len(row[icol].String) > 0 {
					obj = row[icol].String
				}
			}
		}
		if obj == nil || len(errMsg) > 0 {
			// Value from input is null or empty or mapping function returned err or empty for this property,
			// get the default or report error or ignore the field if no default or error message is avail
			if inputColumnSpec.defaultValue.Valid && len(inputColumnSpec.defaultValue.String) > 0 {
				obj = inputColumnSpec.defaultValue.String
			} else {
				if inputColumnSpec.errorMessage.Valid || len(errMsg) > 0 {
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
					if len(errMsg) > 0 {
						if inputColumnSpec.errorMessage.Valid {
							br.ErrorMessage = sql.NullString{String: fmt.Sprintf("%s (%s)", inputColumnSpec.errorMessage.String, errMsg), Valid: true}
						} else {
							br.ErrorMessage = sql.NullString{String: errMsg, Valid: true}
						}
					} else {
						br.ErrorMessage = inputColumnSpec.errorMessage
					}
					log.Println("Error when mapping input value:", br)
					br.write2Chan((*writeOutputc)["jetsapi.process_errors"][0])
				}
				continue
			}
		}
		// Map client-specific code value to canonical code value -- applicable to string not []string
		vv, ok := obj.(string)
		if ok {
			obj = aJetRow.processInput.mapCodeValue(&vv, inputColumnSpec)
		}
		// cast obj to type
		object, err := castToRdfType(obj, inputColumnSpec, reteSession)
		if err != nil {
			// Error casting obj value to colum type
			if inputColumnSpec.defaultValue.Valid {
				object, err = castToRdfType(inputColumnSpec.defaultValue.String, inputColumnSpec, reteSession)
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
				br.ErrorMessage = sql.NullString{String: fmt.Sprintf("while converting value from column %s to property %s: %v",
					inputColumnSpec.inputColumn.String, inputColumnSpec.dataProperty, err), Valid: true}
				log.Println("Error while casting object value to column type:", br)
				br.write2Chan((*writeOutputc)["jetsapi.process_errors"][0])
				continue
			}
		}
		if inputColumnSpec.predicate == nil {
			log.Panicf("ERROR predicate is null")
		}
		if object == nil {
			continue
		}
		switch vv := object.(type) {
		case *bridgego.Resource:
			_, err = reteSession.Insert(subject, inputColumnSpec.predicate, vv)
		case []*bridgego.Resource:
			for _, r := range vv {
				_, err = reteSession.Insert(subject, inputColumnSpec.predicate, r)
				if err != nil {
					log.Panicf("while asserting triple to rete session: %v", err)
				}
			}
		}
		if err != nil {
			log.Panicf("while asserting triple to rete session: %v", err)
		}
	}
	return nil
}
