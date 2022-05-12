package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
)

type ReteWorkspace struct {
	js          *bridge.JetStore
	workspaceDb string
	lookupDb    string
	ruleset     []string
	ruleseq     string
	outTables   []string
	extTables   map[string][]string
	procConfig  *ProcessConfig
}

type ExecuteRulesResult struct {
	executeRulesCount int
}

var ps = flag.Bool("ps", false, "Print the rete session for each session (very verbose)")

// Load the rete workspace database via cgo
func LoadReteWorkspace(
	workspaceDb string, 
	lookupDb string, 
	ruleset string, 
	ruleseq string, 
	procConfig *ProcessConfig, 
	outTables []string,
	extTables map[string][]string) (*ReteWorkspace, error) {

	// load the workspace db
	reteWorkspace := ReteWorkspace{
		workspaceDb: workspaceDb,
		lookupDb:    lookupDb,
		ruleseq:     ruleseq,
		procConfig:  procConfig,
		outTables:   outTables,
		extTables:   extTables,
	}
	var err error
	// case invoking single ruleset, in pipeline for case ruleseq
	if len(ruleset) > 0 {
		reteWorkspace.ruleset = []string {ruleset}
	}
	reteWorkspace.js, err = bridge.LoadJetRules(workspaceDb, lookupDb)
	if err != nil {
		return &reteWorkspace, fmt.Errorf("while loading workspace db: %v", err)
	}

	// assert the rule config triples to meta graph
	err = reteWorkspace.assertRuleConfig()
	return &reteWorkspace, err
}

// main processing function to execute rules
func (rw *ReteWorkspace) ExecuteRules(
	dbpool *pgxpool.Pool,
	processInput *ProcessInput,
	dataInputc <-chan [][]sql.NullString,
	outputSpecs workspace.OutputTableSpecs,
	writeOutputc map[string]chan []interface{}) (*ExecuteRulesResult, error) {
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
	ri.ncol = len(processInput.processInputMapping)
	ri.rdfType, err = rw.js.GetResource("rdf:type")
	if err != nil {
		return &result, fmt.Errorf("while creating rdf:type resource: %v", err)
	}
	ri.jets__key, err = rw.js.GetResource("jets:key")
	if err != nil {
		return &result, fmt.Errorf("while creating jets:key resource: %v", err)
	}
	// keep a map of compiled regex, keyed by the regex pattern
	ri.reMap = make(map[string]*regexp.Regexp)
	// keep a map of map function argument that needs to be cast to double
	ri.argdMap = make(map[string]float64)
	var session_count int64

	for inputRecords := range dataInputc {
		var groupingKey sql.NullString
		if len(inputRecords)>0 && inputRecords[0][processInput.groupingPosition].Valid {
			gp := inputRecords[0][processInput.groupingPosition].String
			groupingKey = sql.NullString{String: gp, Valid: true}
		}
		
		// setup the rdf session for the grouping
		if glogv > 0 {
			session_count += 1
			log.Println("Start RDF Session", session_count)
		}
		rdfSession,err := rw.js.NewRDFSession()
		if err != nil {
			return &result, fmt.Errorf("while creating rdf session: %v", err)
		}

		for i, ruleset := range rw.ruleset {
			// log.Println("Start Rete Session for ruleset", ruleset)
			reteSession, err := rw.js.NewReteSession(rdfSession, ruleset)
			if err != nil {
				return &result, fmt.Errorf("while creating rete session: %v", err)
			}
			if i == 0 {
				// log.Println("Asserting input records with ruleset", ruleset)
				err = ri.assertInputRecords(reteSession, processInput, &inputRecords, &writeOutputc)
				if err != nil {
					return &result, fmt.Errorf("while assertInputRecords: %v", err)
				}	
			}
			msg, err := reteSession.ExecuteRules()
			if err != nil {
				var br BadRow
				br.GroupingKey = groupingKey
				br.ErrorMessage = sql.NullString{String: msg, Valid: true}
				//*
				fmt.Println("BAD ROW:",br)
				br.write2Chan(writeOutputc["process_errors"])
				break
			}
			reteSession.ReleaseReteSession()
		}

		// log.Println("ExecuteRule() Completed sucessfully")
		if *ps {
			rdfSession.DumpRdfGraph()
		}
		var sid string
		if sessionId!=nil && len(*sessionId)>0 {
			sid = *sessionId
		}
		shard := 0
		if shardId != nil {
			shard = *shardId
		}

		// pulling the data out of the rete session
		for tableName, tableSpec := range outputSpecs {
			// check if this tableSpec is for the process_errors table
			if tableName == "process_errors" {
				continue
			}
			// extract entities by rdf type
			ctor, err := rdfSession.Find(nil, ri.rdfType, tableSpec.ClassResource)
			if err != nil {
				return &result, fmt.Errorf("while finding all entities of type %s: %v", tableSpec.ClassName, err)
			}
			for !ctor.IsEnd() {
				subject := ctor.GetSubject()
				// log.Println("Found entity with subject:",subject.AsText())
				// make a slice corresponding to the entity row, selecting predicates from the outputSpec
				ncol := len(tableSpec.Columns)
				entityRow := make([]interface{}, ncol)
				for i:=0; i<ncol; i++ {
					domainColumn := &tableSpec.Columns[i]
					switch domainColumn.ColumnName {
					case "session_id":
						entityRow[i] = sid
					case "shard_id":
						entityRow[i] = shard
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
								br.GroupingKey = groupingKey
								br.ErrorMessage = sql.NullString {
									String: fmt.Sprintf("error while getting value from graph for column %s: %v", domainColumn.ColumnName, err),
									Valid: true}
								//*
								fmt.Println("BAD EXTRACT:",br)
								br.write2Chan(writeOutputc["process_errors"])
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
								br.GroupingKey = groupingKey
								br.ErrorMessage = sql.NullString {
									String: fmt.Sprintf("error getting multiple values from graph for functional column %s", domainColumn.ColumnName), 
									Valid: true}
								//*
								fmt.Println("BAD EXTRACT:",br)
								br.write2Chan(writeOutputc["process_errors"])
							default:
							}
						}
						itor.ReleaseIterator()
					}
				}
				// entityRow is complete
				writeOutputc[tableName] <- entityRow
				ctor.Next()
			}
			ctor.ReleaseIterator()
		}
		result.executeRulesCount += 1
		rdfSession.ReleaseRDFSession()
	}
	return &result, nil
}

// addExtTablesInfo: Add columns corresponding to volatile resources added to output tables
func (rw *ReteWorkspace) addExtTablesInfo(tableSpecs *workspace.OutputTableSpecs) error {
	for tableName, vrs := range rw.extTables {
		outTable,ok := (*tableSpecs)[tableName]
		if !ok {
			return fmt.Errorf("error: -extTable table %s does not found in output table specs", tableName)
		}
		for _, vr := range vrs {
			outTable.Columns = append(outTable.Columns, 
				workspace.DomainColumn{PropertyName: "_0:"+vr, ColumnName: strings.ToLower(vr), DataType: "text", IsArray: true})
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
func (rw *ReteWorkspace) assertRuleConfig() error {
	if rw == nil {
		return fmt.Errorf("ERROR: ReteWorkspace cannot be nil")
	}
	for _, t3 := range rw.procConfig.ruleConfigs {
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
