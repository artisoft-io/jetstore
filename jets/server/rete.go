package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

type ReteWorkspace struct {
	js             *bridge.JetStore
	workspaceDb    string
	lookupDb       string
	ruleset        []string
	ruleseq        string
	outTables      []string
	extTables      map[string][]string
	pipelineConfig *PipelineConfig
}

type ExecuteRulesResult struct {
	ExecuteRulesCount int
}

var ps = flag.Bool("ps", false, "Print the rete session for each session (very verbose)")

// Load the rete workspace database via cgo
func LoadReteWorkspace(
	workspaceDb string,
	lookupDb string,
	ruleset string,
	ruleseq string,
	pipelineConfig *PipelineConfig,
	outTables []string,
	extTables map[string][]string) (*ReteWorkspace, error) {

	// load the workspace db
	reteWorkspace := ReteWorkspace{
		workspaceDb:    workspaceDb,
		lookupDb:       lookupDb,
		ruleseq:        ruleseq,
		pipelineConfig: pipelineConfig,
		outTables:      outTables,
		extTables:      extTables,
	}
	var err error
	// case invoking single ruleset, in pipeline for case ruleseq
	if len(ruleset) > 0 {
		reteWorkspace.ruleset = []string{ruleset}
	}
	reteWorkspace.js, err = bridge.LoadJetRules(pipelineConfig.processConfig.processName, workspaceDb, lookupDb)
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
	workspaceMgr *workspace.WorkspaceDb,
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
	log.Println("Execute Rule Started")
	var ri ReteInputContext
	var err error
	// cache pre-defined resources
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

	// keep a map of compiled regex, keyed by the regex pattern
	ri.reMap = make(map[string]*regexp.Regexp)
	// keep a map of map function argument that needs to be cast to double
	ri.argdMap = make(map[string]float64)
	var session_count int64

	for inBundle := range dataInputc {

		// setup the rdf session for the grouping
		session_count += 1
		rdfSession, err := rw.js.NewRDFSession()
		if err != nil {
			return &result, fmt.Errorf("while creating rdf session: %v", err)
		}

		for iset, ruleset := range rw.ruleset {
			if glogv > 0 {
				log.Println("thread", workerId, ":: Start Rete Session", session_count, "for ruleset", ruleset, "with grouping key", inBundle.groupingValue)
			}
			reteSession, err := rw.js.NewReteSession(rdfSession, ruleset)
			if err != nil {
				return &result, fmt.Errorf("while creating rete session: %v", err)
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
			jetStoreProp, err := workspaceMgr.LoadJetStoreProperties(ruleset)
			if err != nil {
				return &result, fmt.Errorf("while LoadJetStoreProperties for ruleset %s: %v", ruleset, err)
			}
			var nloop, iloop int64
			value, ok := jetStoreProp["$max_looping"]
			if ok {
				nloop, err = strconv.ParseInt(value, 10, 64)
				if err != nil {
					return &result, fmt.Errorf("while parsing $max_looping value as int: %v", err)
				}
			}
			if nloop > 0 {
				// log.Println("looping in use, max number of loops is ",nloop)
				rdfSession.Insert(ri.jets__istate, ri.rdf__type, ri.jets__state)
			}
			// do for iloop <= maxloop (since loop start at one!)
			for iloop = 0; iloop <= nloop; iloop++ {
				if glogv > 1 {
					log.Println("thread", workerId, ":: Calling Execute Rules, loop:", iloop, ", session count:", session_count, ", for ruleset:", ruleset, ", with grouping key:", inBundle.groupingValue)
				}
				if iloop > 0 {
					r, err := reteSession.NewIntLiteral(int(iloop))
					if err != nil {
						return &result, fmt.Errorf("while NewIntLiteral for loop %s: %v", ruleset, err)
					}
					rdfSession.Insert(ri.jets__istate, ri.jets__loop, r)
				}
				msg, err := reteSession.ExecuteRules()
				if err != nil {
					var br BadRow
					br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
					br.ErrorMessage = sql.NullString{String: msg, Valid: true}
					log.Println("BAD ROW:", br)
					br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
					break
				}
				// CHECK for jets__terminate and jets__exception
				if isDone, err := rdfSession.ContainsSP(ri.jets__istate, ri.jets__completed); isDone > 0 || err != nil {
					// log.Println("Rete Session Looping Completed")
					break
				}
			}
			if nloop > 0 && iloop >= nloop {
				var br BadRow
				br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
				br.ErrorMessage = sql.NullString{String: "error: max loop reached", Valid: true}
				log.Println("MAX LOOP REACHED:", br)
				br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
				break
			}
			reteSession.ReleaseReteSession()
		}

		if *ps {
			log.Println("ExecuteRule() Completed sucessfully, the rdf sesion contains:")
			rdfSession.DumpRdfGraph()
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
				return &result, fmt.Errorf("while finding all entities of type %s: %v", tableSpec.ClassName, err)
			}
			for !ctor.IsEnd() {
				subject := ctor.GetSubject()

				// Check if subject is an entity for the current source period
				// i.e. is not an historical entity comming from the lookback period
				// We don't extract historical entities but only one from the current source period
				// identified with jets:source_period_sequence == 0 or 
				// entities created during the rule session, identified with jets:source_period_sequence is null
				keepObj := true
				obj, err := rdfSession.GetObject(subject, ri.jets__source_period_sequence)
				if err != nil {
					return &result, fmt.Errorf("while getting obj for predicate jets:source_period_sequence of an entity of type %s: %v", tableSpec.ClassName, err)
				}
				if obj != nil {
					v, err := obj.GetInt()
					if err != nil {
						return &result, fmt.Errorf("range of predicate jets:source_period_sequence is not int for an entity of type %s: %v", tableSpec.ClassName, err)
					}
					if v > 0 {
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
						// log.Println("Found entity with subject:",stxt, "with column",domainColumn.ColumnName,"of type",domainColumn.DataType)
						switch {
						case domainColumn.ColumnName == "session_id":
							entityRow[i] = *outSessionId

						case strings.HasSuffix(domainColumn.ColumnName, ":domain_key"):
							objectType := strings.Split(domainColumn.ColumnName, ":")[0]
							domainKey, _, err := tableSpec.DomainKeysInfo.ComputeGroupingKeyI(*nbrShards, &objectType, &entityRow)
							if err != nil {
								return &result, fmt.Errorf("while ComputeGroupingKeyI: %v", err)
							}
							entityRow[i] = domainKey

						case strings.HasSuffix(domainColumn.ColumnName, ":shard_id"):
							objectType := strings.Split(domainColumn.ColumnName, ":")[0]
							_, shardId, err := tableSpec.DomainKeysInfo.ComputeGroupingKeyI(*nbrShards, &objectType, &entityRow)
							if err != nil {
								return &result, fmt.Errorf("while ComputeGroupingKeyI: %v", err)
							}
							entityRow[i] = shardId

						default:
							var data []interface{}
							itor, err := rdfSession.Find_sp(subject, domainColumn.Predicate)
							if err != nil {
								return &result, fmt.Errorf("while finding triples of an entity of type %s: %v", tableSpec.ClassName, err)
							}
							for !itor.IsEnd() {
								obj, err := itor.GetObject().AsInterface(schema.ToPgType(domainColumn.DataType))
								if err != nil {
									var br BadRow
									rowkey, err := subject.GetName()
									if err == nil {
										br.RowJetsKey = sql.NullString{String: rowkey, Valid: true}
									}
									br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
									br.ErrorMessage = sql.NullString{
										String: fmt.Sprintf("error while getting value from graph for column %s: %v", domainColumn.ColumnName, err),
										Valid:  true}
									log.Println("BAD EXTRACT:", br)
									br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
								}
								data = append(data, obj)
								itor.Next()
							}
							if domainColumn.IsArray {
								entityRow[i] = data
							} else {
								ld := len(data)
								switch {
								case ld == 1:
									entityRow[i] = data[0]
								case ld > 1:
									// Invalid row, multiple values for a functional property
									var br BadRow
									rowkey, err := subject.GetName()
									if err == nil {
										br.RowJetsKey = sql.NullString{String: rowkey, Valid: true}
									}
									br.GroupingKey = sql.NullString{String: inBundle.groupingValue, Valid: true}
									br.ErrorMessage = sql.NullString{
										String: fmt.Sprintf("error getting multiple values from graph for functional column %s", domainColumn.ColumnName),
										Valid:  true}
									log.Println("BAD EXTRACT:", br)
									br.write2Chan(writeOutputc["jetsapi.process_errors"][0])
								default:
								}
							}
							itor.ReleaseIterator()
						}
					}
					// entityRow is complete
					//* REMOVE MULTI DB CONNECTION BY NODES :: compute_node_id_from_shard_id
					// writeOutputc[tableName][compute_node_id_from_shard_id(shard)] <- entityRow
					writeOutputc[tableName][0] <- entityRow
				}
				ctor.Next()
			}
			ctor.ReleaseIterator()
		}
		result.ExecuteRulesCount += 1
		rdfSession.ReleaseRDFSession()
	}
	return &result, nil
}

// addExtTablesInfo: Add columns corresponding to volatile resources added to output tables
func (rw *ReteWorkspace) addExtTablesInfo(tableSpecs *workspace.OutputTableSpecs) error {
	for tableName, vrs := range rw.extTables {
		outTable, ok := (*tableSpecs)[tableName]
		if !ok {
			return fmt.Errorf("error: -extTable table %s does not found in output table specs", tableName)
		}
		for _, vr := range vrs {
			outTable.Columns = append(outTable.Columns,
				workspace.DomainColumn{PropertyName: "_0:" + vr, ColumnName: strings.ToLower(vr), DataType: "text", IsArray: true})
		}
	}
	return nil
}

// addOutputClassResource: Add the rdf resource to DomainTable for output table
func (rw *ReteWorkspace) addOutputClassResource(domainTable *workspace.DomainTable) error {
	var err error
	domainTable.ClassResource, err = rw.js.NewResource(domainTable.ClassName)
	if err != nil {
		return fmt.Errorf("while adding class resource to DomainTable: %v", err)
	}
	return nil
}

// addOutputPredicate: add meta graph resource corresponding to output column names
func (rw *ReteWorkspace) addOutputPredicate(domainColumns []workspace.DomainColumn) error {
	for ipos := range domainColumns {
		var err error
		domainColumns[ipos].Predicate, err = rw.js.NewResource(domainColumns[ipos].PropertyName)
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

// assertRuleConfig: assert rule config triples to metadata graph
// This asserts client specific triples and loads process specific meta triples from workspace db
func (rw *ReteWorkspace) assertRuleConfig() error {
	if rw == nil {
		return fmt.Errorf("ERROR: ReteWorkspace cannot be nil")
	}
	//*
	fmt.Println("****** assertRuleConfig CALLED, using mainRules:",rw.pipelineConfig.processConfig.mainRules)
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
		var object *bridge.Resource
		switch t3.rdfType {
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
		rw.js.InsertRuleConfig(subject, predicate, object)
	}
	return nil
}
