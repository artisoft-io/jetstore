package delegate

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridgego"
	"github.com/artisoft-io/jetstore/jets/cleansing_functions"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/serverv2/rdf"
	"github.com/artisoft-io/jetstore/jets/serverv2/workspace"
)

type ReteWorkspace struct {
	js             *bridgego.JetStore
	lookupDb       string
	outTables      []string
	pipelineConfig *PipelineConfig
}

type ExecuteRulesResult struct {
	ExecuteRulesCount int
}

var ps = flag.Bool("ps", false, "Print the rete session for each session (very verbose)")

// Load the rete workspace database via cgo
func LoadReteWorkspace(
	lookupDb string,
	pipelineConfig *PipelineConfig) (*ReteWorkspace, error) {

	// load the workspace db
	reteWorkspace := ReteWorkspace{
		lookupDb:       lookupDb,
		pipelineConfig: pipelineConfig,
	}
	var err error
	reteWorkspace.js, err = bridgego.LoadJetRules(pipelineConfig.processConfig.processName, 
		pipelineConfig.processConfig.mainRules, lookupDb)
	if err != nil {
		return &reteWorkspace, fmt.Errorf("while loading workspace db: %v", err)
	}

	// assert the rule config triples to meta graph
	err = reteWorkspace.assertRuleConfig()
	return &reteWorkspace, err
}

// Terminate the c++ allocated resources
func (rw *ReteWorkspace) Release() error {
	return rw.js.ReleaseJetRules()
}

// main processing function to execute rules
func (rw *ReteWorkspace) ExecuteRules(
	workerId int,
	dataInputc <-chan groupedJetRows,
	outputSpecs workspace.OutputTableSpecs,
	writeOutputc map[string][]chan []interface{}) (*ExecuteRulesResult, error) {

	var result ExecuteRulesResult
	// for each msg in dataInput:
	// 	- start a rete session,
	//	- assert input records
	//	- execute rules (ruleset chaining)
	//	- for each output types:
	//		- extract entities
	//		- write to ouput chanel
	// ---------------------------
	// ReteInputContext: context/cache across all rdf sessions
	// log.Println("Execute Rule Started")
	nbrReteSessionSaved := 0
	var ri ReteInputContext
	var err error
	// cache pre-defined resources
	ri.jets__client, err = rw.js.GetResource("jets:client")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__completed, err = rw.js.GetResource("jets:completed")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__istate, err = rw.js.GetResource("jets:iState")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__key, err = rw.js.GetResource("jets:key")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__loop, err = rw.js.GetResource("jets:loop")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__org, err = rw.js.GetResource("jets:org")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__source_period_sequence, err = rw.js.GetResource("jets:source_period_sequence")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__state, err = rw.js.GetResource("jets:State")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.rdf__type, err = rw.js.GetResource("rdf:type")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__input_record, err = rw.js.GetResource("jets:InputRecord")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__sourcePeriodType, err = rw.js.GetResource("jets:sourcePeriodType")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__currentSourcePeriod, err = rw.js.GetResource("jets:currentSourcePeriod")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__currentSourcePeriodDate, err = rw.js.GetResource("jets:currentSourcePeriodDate")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}
	ri.jets__exception, err = rw.js.GetResource("jets:exception")
	if err != nil {
		return &result, fmt.Errorf("while get resource: %v", err)
	}

	// Cleansing Function Context - argument caches
	ri.cleansingFunctionContext = cleansing_functions.NewCleansingFunctionContext(&rw.pipelineConfig.mainProcessInput.inputColumnName2Pos)
	var session_count int64

	for inBundle := range dataInputc {

		// setup the rdf session for the grouping
		session_count += 1
		reteSessionSaved := false
		rdfSession, err := rw.js.NewRDFSession()
		if err != nil {
			return &result, fmt.Errorf("while creating rdf session: %v", err)
		}

		for iset, ruleset := range rw.js.Factory.MainRuleFileNames {
			if glogv > 0 {
				log.Println("thread", workerId, ":: Start Rete Session", session_count, "for ruleset", ruleset, 
					"with grouping key", inBundle.groupingValue)
			}
			reteSession, err := rw.js.NewReteSession(rdfSession, ruleset)
			if err != nil {
				return &result, fmt.Errorf("while creating rete session: %v", err)
			}

			// Set the current source period and period type in the rdf session
			r, _ := reteSession.NewIntLiteral(rw.pipelineConfig.currentSourcePeriod)
			_, err = reteSession.Insert(ri.jets__istate, ri.jets__currentSourcePeriod, r)
			if err != nil {
				return &result, fmt.Errorf("while inserting jets:currentSourcePeriod to rdf session: %v", err)
			}
			r, _ = reteSession.NewDateLiteral(rw.pipelineConfig.currentSourcePeriodDate)
			_, err = reteSession.Insert(ri.jets__istate, ri.jets__currentSourcePeriodDate, r)
			if err != nil {
				return &result, fmt.Errorf("while inserting jets:currentSourcePeriodDate to rdf session: %v", err)
			}
			r, _ = reteSession.NewTextLiteral(rw.pipelineConfig.sourcePeriodType)
			_, err = reteSession.Insert(ri.jets__istate, ri.jets__sourcePeriodType, r)
			if err != nil {
				return &result, fmt.Errorf("while inserting jets:sourcePeriodType to rdf session: %v", err)
			}

			if iset == 0 {
				err = ri.assertInputBundle(reteSession, &inBundle, &writeOutputc)
				if err != nil {
					return &result, fmt.Errorf("while asserting input bundle for session: %v", err)
				}
			}
			// Step 0 of loop is pre loop or no loop
			// Step 1+ for looping
			reteSession.Erase(ri.jets__istate, ri.jets__loop, nil)
			reteSession.Erase(ri.jets__istate, ri.jets__completed, nil)
			jetStoreProp := *rw.js.MetaStore.JetStoreConfig
			var nloop, iloop int
			value, ok := jetStoreProp["$max_looping"]
			if ok {
				nloop, err = strconv.Atoi(value)
				if err != nil {
					return &result, fmt.Errorf("while parsing $max_looping value as int: %v", err)
				}
			}
			if nloop > 0 {
				log.Println("looping in use, max number of loops is ",nloop)
				rdfSession.Insert(ri.jets__istate, ri.rdf__type, ri.jets__state)
			}
			// do for iloop <= maxloop (since loop start at one!)
			for iloop = 0; iloop <= nloop; iloop++ {
				if glogv > 1 {
					log.Println("thread", workerId, ":: Calling Execute Rules, loop:", iloop, ", session count:", session_count, ", for ruleset:", ruleset, ", with grouping key:", inBundle.groupingValue)
				}
				if iloop > 0 {
					r, err := reteSession.NewIntLiteral(iloop)
					if err != nil {
						return &result, fmt.Errorf("while NewIntLiteral for loop %s: %v", ruleset, err)
					}
					rdfSession.Insert(ri.jets__istate, ri.jets__loop, r)
				}
				msg, err := reteSession.ExecuteRules()
				if err != nil {
					br := NewBadRow()
					br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
					br.ErrorMessage = sql.NullString{String: msg, Valid: true}
					log.Println("BAD ROW (ExecuteRules returned err):", br, "(", err.Error(), ")")
					br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
					break
				}
				// CHECK for jets__terminate
				if isDone, err := rdfSession.ContainsSP(ri.jets__istate, ri.jets__completed); isDone > 0 || err != nil {
					log.Println("Rete Session Looping Completed")
					break
				}
			}
			if nloop > 0 && iloop >= nloop {
				br := NewBadRow()
				br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
				br.ErrorMessage = sql.NullString{String: "error: max loop reached", Valid: true}
				log.Println("MAX LOOP REACHED:", br)
				br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
				break
			}
			if *ps {
				log.Println("Rule visits:")
				reteSession.DumpVertexVisit()

			}
			reteSession.ReleaseReteSession()
		}

		if *ps {
			log.Println("ExecuteRule() Completed, the rdf sesion contains:")
			rdfSession.DumpRdfGraph()
		}

		// Get the jets:exception(s)
		ctor, err := rdfSession.Find_sp(ri.jets__istate, ri.jets__exception)
		if err != nil {
			log.Printf("while finding all jets:exception in rdf graph: %v", err)
		} else {
			for t3 := range ctor.Itor {
				hasException := t3[2]
				if hasException != nil {
					txt := hasException.String()
					br := NewBadRow()
					br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
					br.ErrorMessage = sql.NullString{String: txt, Valid: true}
					if !reteSessionSaved && nbrReteSessionSaved < rw.pipelineConfig.maxReteSessionSaved {
						log.Println("Rete Session Has Rule Exception:", txt, "(rete session saved to process_errors table)")
						reteSessionSaved = true
						nbrReteSessionSaved += 1
						br.ReteSessionSaved = "Y"
						b, errx := rdf.RDFSessionAsTableJsonV2(rdfSession, rw.js)
						if errx != nil {
							log.Printf("while extracting RDFSessionAsTableJson: %v", errx)
							br.ReteSessionSaved = "N"
						} else {
							br.ReteSessionTriples = sql.NullString{
								String: string(b),
								Valid:  true,
							}
						}
					} else {
						log.Println("Rete Session Has Rule Exception:", txt)
					}
					br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
				}
			}
			ctor.Done()
		}

		// pulling the data out of the rete session
		for tableName, tableSpec := range outputSpecs {
			// check if this tableSpec is for the process_errors table
			if tableName == "jetsapi.process_errors" {
				continue
			}
			// extract entities by rdf type
			ctor, err := rdfSession.Find(nil, ri.rdf__type, tableSpec.ClassResource)
			if err != nil {
				return &result, fmt.Errorf("while finding all entities of type %s: %v", tableSpec.TableInfo.ClassName, err)
			}
			for t3 := range ctor.Itor {
				subject := t3[0]
				// //DEV**
				// log.Println("Extracting subject:", subject.String())

				// Check if subject is an entity for the current source period
				// i.e. is not an historical entity comming from the lookback period
				// We don't extract historical entities but only one from the current source period
				// identified with jets:source_period_sequence == 0 or
				// entities created during the rule session, identified with jets:source_period_sequence is null
				// Additional Measure: entities with jets:source_period_sequence == 0, must have jets:InputRecord
				// as rdf:type to ensure it's a mapped entity and not an injected entity.
				// Note: Do not save the jets:InputEntity marker type on the extracted obj.
				keepObj := true
				obj, err := rdfSession.GetObject(bridgego.NewResource(subject), ri.jets__source_period_sequence)
				if err != nil {
					return &result, fmt.Errorf("while getting obj for predicate jets:source_period_sequence of an entity of type %s: %v", tableSpec.TableInfo.ClassName, err)
				}
				if obj != nil {
					v, err := obj.GetInt()
					if err != nil {
						return &result, fmt.Errorf("range of predicate jets:source_period_sequence is not int for an entity of type %s: %v", tableSpec.TableInfo.ClassName, err)
					}
					if v == 0 {
						// Check if obj has marker type jets:InputRecord, extract obj if it does.
						isInputRecord, err := rdfSession.Contains(bridgego.NewResource(subject), ri.rdf__type, ri.jets__input_record)
						if err != nil {
							return &result, fmt.Errorf("while checking if entity has marker class jets:InputRecord for an entity of type %s: %v", tableSpec.TableInfo.ClassName, err)
						}
						if isInputRecord == 0 {
							// jets:InputEntity marker is missing, don't extract the obj
							keepObj = false
						}
					} else {
						keepObj = false
					}
				}
				// extract entity if we keep it (i.e. not an historical entity)
				if keepObj {
					// make a slice corresponding to the entity row, selecting predicates from the outputSpec
					ncol := len(tableSpec.Columns)
					// Compute the Domain Keys and ShardIds
					entityRow := make([]interface{}, ncol)
					for i := 0; i < ncol; i++ {
						domainColumn := &tableSpec.Columns[i]
						// log.Println("Found entity with subject:",subject.AsTextSilent(), "with column",domainColumn.ColumnName,"of type",domainColumn.DataType)
						switch {
						case domainColumn.ColumnInfo.ColumnName == "session_id":
							entityRow[i] = outSessionId

						case strings.HasSuffix(domainColumn.ColumnInfo.ColumnName, ":domain_key"):
							objectType := strings.Split(domainColumn.ColumnInfo.ColumnName, ":")[0]
							domainKey, _, err := tableSpec.DomainKeysInfo.ComputeGroupingKeyI(nbrShards, &objectType, &entityRow)
							if err != nil {
								return &result, fmt.Errorf("while ComputeGroupingKeyI: %v", err)
							}
							entityRow[i] = domainKey

						case strings.HasSuffix(domainColumn.ColumnInfo.ColumnName, ":shard_id"):
							objectType := strings.Split(domainColumn.ColumnInfo.ColumnName, ":")[0]
							_, shardId, err := tableSpec.DomainKeysInfo.ComputeGroupingKeyI(nbrShards, &objectType, &entityRow)
							if err != nil {
								return &result, fmt.Errorf("while ComputeGroupingKeyI: %v", err)
							}
							entityRow[i] = shardId

						default:
							var data []interface{}
							itor, err := rdfSession.Find_sp(bridgego.NewResource(subject), domainColumn.Predicate)
							if err != nil {
								return &result, fmt.Errorf("while finding triples of an entity of type %s: %v", tableSpec.TableInfo.ClassName, err)
							}
							for t3 := range itor.Itor {
								obj, err := bridgego.NewResource(t3[2]).AsInterface(schema.ToPgType(domainColumn.ColumnInfo.Type))
								if err != nil {
									br := NewBadRow()
									br.RowJetsKey = sql.NullString{String: subject.Name(), Valid: true}
									br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
									br.ErrorMessage = sql.NullString{
										String: fmt.Sprintf("error while getting value from graph for column %s: %v", domainColumn.ColumnInfo.ColumnName, err),
										Valid:  true}
									log.Println("BAD EXTRACT:", br)
									br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
								} else {
									if !(domainColumn.ColumnInfo.ColumnName == "rdf:type" && obj.(string) == "jets:InputRecord") {
										data = append(data, obj)
									}
								}
							}
							switch {
							// Use array as value
							case domainColumn.ColumnInfo.AsArray:
								entityRow[i] = data

							// Functional property, got single element
							case len(data) == 1:
								entityRow[i] = data[0]

							// There is no value, this is null
							case len(data) == 0:
								entityRow[i] = nil

							// Coalesce text array into functional text property
							case domainColumn.ColumnInfo.Type == "text" || domainColumn.ColumnInfo.Type == "resource" || domainColumn.ColumnInfo.Type == "volatile_resource":
								var buf strings.Builder
								buf.WriteString("{")
								isFirst := true
								for idata := range data {
									v := data[idata].(string)
									if v != "" {
										if !isFirst {
											buf.WriteString(",")
										}
										buf.WriteString(v)
										isFirst = false
									}
								}
								buf.WriteString("}")
								v := buf.String()
								if v != "{}" {
									entityRow[i] = v
								} else {
									entityRow[i] = nil
								}

							// Got multiple values for non text functional property
							default:
								// Invalid row, multiple values for a functional property
								br := NewBadRow()
								br.RowJetsKey = sql.NullString{String: subject.Name(), Valid: true}
								br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
								br.ErrorMessage = sql.NullString{
									String: fmt.Sprintf("error getting multiple values from graph for functional column %s", domainColumn.ColumnInfo.ColumnName),
									Valid:  true}
								log.Println("BAD EXTRACT:", br)
								br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
							}
							itor.Done()
						}
					}
					// entityRow is complete
					//* REMOVE MULTI DB CONNECTION BY NODES :: compute_node_id_from_shard_id
					// writeOutputc[tableName][compute_node_id_from_shard_id(shard)] <- entityRow
					writeOutputc[tableName][0] <- entityRow
				}
			}
			ctor.Done()
			// //**
			// log.Println("Done Extracting class:", tableSpec.TableInfo.ClassName)
		}
		result.ExecuteRulesCount += 1
		rdfSession.ReleaseRDFSession()
	}
	return &result, nil
}

// addOutputClassResource: Add the rdf resource to DomainTable for output table
func (rw *ReteWorkspace) addOutputClassResource(domainTable *workspace.DomainTable) error {
	var err error
	domainTable.ClassResource, err = rw.js.NewResource(domainTable.TableInfo.ClassName)
	if err != nil {
		return fmt.Errorf("while adding class resource to DomainTable: %v", err)
	}
	return nil
}

// addOutputPredicate: add meta graph resource corresponding to output column names
func (rw *ReteWorkspace) addOutputPredicate(domainColumns []workspace.DomainColumn) error {
	for ipos := range domainColumns {
		var err error
		domainColumns[ipos].Predicate, err = rw.js.NewResource(domainColumns[ipos].ColumnInfo.PropertyName)
		if err != nil {
			return fmt.Errorf("while adding predicate to DomainColumn: %v", err)
		}
	}
	return nil
}

// addInputPredicate: add meta graph resource corresponding to input column names
func (rw *ReteWorkspace) addInputPredicate(inputColumns []ProcessMap) error {
	for ipos := range inputColumns {
		var err error
		inputColumns[ipos].predicate, err = rw.js.NewResource(inputColumns[ipos].dataProperty)
		if err != nil {
			return fmt.Errorf("while adding predicate to ProcessMap: %v", err)
		}
	}
	return nil
}

// addEntityRdfType: Add rdf type resource to input entity metadata
func (rw *ReteWorkspace) addEntityRdfType(processInput *ProcessInput) error {
	var err error
	processInput.entityRdfTypeResource, err = rw.js.NewResource(processInput.entityRdfType)
	return err
}

// GetRangeDataType: Get the data type for the range of the dataProperty arg
func (rw *ReteWorkspace) GetRangeDataType(dataProperty string) (string, bool, error) {
	switch {
	case strings.HasPrefix(dataProperty, "_0:"):
		return "text", true, nil

	case dataProperty == "jets:source_period_sequence":
		return "int", false, nil

	default:
		return rw.js.MetaStore.GetRangeDataType(dataProperty)
	}
}

// assertRuleConfig: assert rule config triples to metadata graph
// This asserts client specific triples and loads process specific meta triples from workspace db
func (rw *ReteWorkspace) assertRuleConfig() error {
	if rw == nil {
		return fmt.Errorf("ERROR: ReteWorkspace cannot be nil")
	}
	// Load process meta triples
	rw.js.LoadProcessMetaTriples(rw.pipelineConfig.processConfig.mainRules, rw.pipelineConfig.processConfig.isRuleSet)

	// Assert client specific triples
	for _, t3 := range rw.pipelineConfig.ruleConfigs {
		subject, err := rw.js.NewResource(t3.subject)
		if err != nil {
			return fmt.Errorf("while asserting rule config (NewResource): %v", err)
		}
		predicate, err := rw.js.NewResource(t3.predicate)
		if err != nil {
			return fmt.Errorf("while asserting rule config (NewResource): %v", err)
		}
		// Constructing a Resource from meta graph (not from a rete session!)
		// Same construct is used with rete session handle
		var object *bridgego.Resource
		switch strings.TrimSpace(t3.rdfType) {
		case "null":
			object, err = rw.js.NewNull()
		case "bn":
			object, err = rw.js.NewBlankNode(0)
		case "resource":
			object, err = rw.js.NewResource(t3.object)
		case "int":
			var v int
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.NewIntLiteral(v)
		case "bool":
			v := 0
			if len(t3.object) > 0 {
				c := strings.ToLower(t3.object[0:1])
				switch c {
				case "t", "1", "y":
					v = 1
				case "f", "0", "n":
					v = 0
				default:
					return fmt.Errorf("while rule config triple; object is not bool: %s", t3.object)
				}
			}
			object, err = rw.js.NewIntLiteral(v)
		case "uint":
			var v uint
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.NewUIntLiteral(v)
		case "long":
			var v int
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.NewLongLiteral(v)
		case "ulong":
			var v uint
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.NewULongLiteral(v)
		case "double":
			var v float64
			_, err = fmt.Sscan(t3.object, &v)
			if err != nil {
				return fmt.Errorf("while asserting rule config: %v", err)
			}
			object, err = rw.js.NewDoubleLiteral(v)
		case "text":
			object, err = rw.js.NewTextLiteral(t3.object)
		case "date":
			object, err = rw.js.NewDateLiteral(t3.object)
		case "datetime":
			object, err = rw.js.NewDatetimeLiteral(t3.object)
		default:
			err = fmt.Errorf("ERROR assertRuleConfig: unknown rdf type for object: %s", t3.rdfType)
		}
		if err != nil {
			return fmt.Errorf("while asserting rule config: %v", err)
		}
		_, err = rw.js.InsertRuleConfig(subject, predicate, object)
		if err != nil {
			return fmt.Errorf("while calling InsertRuleConfig: %v", err)
		}
	}
	return nil
}
