package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/google/uuid"
)

type ReteWorkspace struct {
	js          *bridge.JetStore
	workspaceDb string
	lookupDb    string
	ruleset     string
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
	procConfig *ProcessConfig, 
	outTables []string,
	extTables map[string][]string) (*ReteWorkspace, error) {

	// load the workspace db
	reteWorkspace := ReteWorkspace{
		workspaceDb: workspaceDb,
		lookupDb:    lookupDb,
		ruleset:     ruleset,
		procConfig:  procConfig,
		outTables:   outTables,
		extTables:   extTables,
	}
	var err error
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
	// get the grouping column position
	log.Println("Execute Rule Started")
	ncol := len(processInput.processInputMapping)
	rdfType, err := rw.js.GetResource("rdf:type")
	if err != nil {
		return &result, fmt.Errorf("while creating rdf:type resource: %v", err)
	}
	jets__key, err := rw.js.GetResource("jets:key")
	if err != nil {
		return &result, fmt.Errorf("while creating jets:key resource: %v", err)
	}
	for inputRecords := range dataInputc {
		// log.Println("Start Rete Session")
		reteSession, err := rw.js.NewReteSession(*ruleset)
		if err != nil {
			return &result, fmt.Errorf("while calling NewReteSession: %v", err)
		}
		// Each row in inputRecords is a jets:Entity, with it's own jets:key
		for _, row := range inputRecords {
			if len(row) == 0 {
				continue
			}
			var jetsKeyStr string
			if row[processInput.keyPosition].Valid {
				jetsKeyStr = row[processInput.keyPosition].String
			} else {
				jetsKeyStr = uuid.New().String()
			}
			subject, err := reteSession.NewResource(jetsKeyStr)
			if err != nil {
				return &result, fmt.Errorf("while creating row's subject resource (NewResource): %v", err)
			}
			jetsKey, err := reteSession.NewTextLiteral(jetsKeyStr)
			if err != nil {
				return &result, fmt.Errorf("while creating row's jets__key literal (NewTextLiteral): %v", err)
			}
			if subject == nil || rdfType == nil || processInput.entityRdfTypeResource == nil {
				return &result, fmt.Errorf("while asserting row rdf type")
			}
			_, err = reteSession.Insert(subject, rdfType, processInput.entityRdfTypeResource)
			if err != nil {
				return &result, fmt.Errorf("while asserting row rdf type: %v", err)
			}
			_, err = reteSession.Insert(subject, jets__key, jetsKey)
			if err != nil {
				return &result, fmt.Errorf("while asserting row rdf type: %v", err)
			}
			for icol := 0; icol < ncol; icol++ {
				// asserting input row with mapping spec
				inputColumnSpec := &processInput.processInputMapping[icol]
				var obj string
				if row[icol].Valid {
					if inputColumnSpec.functionName.Valid {
						switch inputColumnSpec.functionName.String {
						case "to_upper":
							obj = strings.ToUpper(row[icol].String)
						case "to_zip5":
						case "reformat0":
						case "apply_regex":
						case "scale_units":
						case "parse_amount":
						default:
							return &result, fmt.Errorf("ERROR unknown mapping function: %s", inputColumnSpec.functionName.String)
						}
	
					} else {
						obj = row[icol].String
					}
				} else {
					// get the default or ignore the filed if no default is avail
					if inputColumnSpec.defaultValue.Valid {
						obj = inputColumnSpec.defaultValue.String
					} else {
						continue
					}
				}
				
				// cast obj to type
				// switch inputColumn.DataType {
				var object *bridge.Resource
				var err error
				switch inputColumnSpec.rdfType {
				// case "null":
				// 	object, err = rw.js.NewNull()
				case "resource":
					object, err = reteSession.NewResource(obj)
				case "int":
					var v int
					_, err = fmt.Sscan(obj, &v)
					if err != nil {
						return &result, fmt.Errorf("while scaning an int from input valut: %v", err)
					}
					object, err = reteSession.NewIntLiteral(v)
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
							return &result, fmt.Errorf("while mapping input value; object is not bool: %s", obj)
						}
					}
					object, err = reteSession.NewIntLiteral(v)
				case "uint":
					var v uint
					_, err = fmt.Sscan(obj, &v)
					if err != nil {
						return &result, fmt.Errorf("while mapping input value: %v", err)
					}
					object, err = reteSession.NewUIntLiteral(v)
				case "long":
					var v int
					_, err = fmt.Sscan(obj, &v)
					if err != nil {
						return &result, fmt.Errorf("while mapping input value: %v", err)
					}
					object, err = reteSession.NewLongLiteral(v)
				case "ulong":
					var v uint
					_, err = fmt.Sscan(obj, &v)
					if err != nil {
						return &result, fmt.Errorf("while mapping input value: %v", err)
					}
					object, err = reteSession.NewULongLiteral(v)
				case "double":
					var v float64
					_, err = fmt.Sscan(obj, &v)
					if err != nil {
						return &result, fmt.Errorf("while mapping input value: %v", err)
					}
					object, err = reteSession.NewDoubleLiteral(v)
				case "text":
					object, err = reteSession.NewTextLiteral(obj)
				case "date":
					object, err = reteSession.NewDateLiteral(obj)
				case "datetime":
					object, err = reteSession.NewDatetimeLiteral(obj)
				default:
					err = fmt.Errorf("ERROR assertRuleConfig: unknown or invalid rdf type for object: %s", inputColumnSpec.rdfType)
				}
				if err != nil {
					//* TODO try the default value
					return &result, fmt.Errorf("while mapping input value: %v", err)
				}
				if subject == nil {
					return &result, fmt.Errorf("ERROR subject is null")
				}
				if inputColumnSpec.predicate == nil {
					return &result, fmt.Errorf("ERROR predicate is null")
				}
				if object == nil {
					return &result, fmt.Errorf("ERROR object is null")
				}
				_, err = reteSession.Insert(subject, inputColumnSpec.predicate, object)
				if err != nil {
					return &result, fmt.Errorf("while asserting triple to rete sesson: %v", err)
				}
			}
		}
		// done asserting
		err = reteSession.ExecuteRules()
		if err != nil {
			return &result, fmt.Errorf("while reteSession.ExecuteRules: %v", err)
		}
		// log.Println("ExecuteRule() Completed sucessfully")
		if *ps {
			reteSession.DumpRdfGraph()
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
			// extract entities by rdf type
			ctor, err := reteSession.Find(nil, rdfType, tableSpec.ClassResource)
			if err != nil {
				return &result, fmt.Errorf("while finding all entities of type %s: %v", tableSpec.ClassName, err)
			}
			defer ctor.ReleaseIterator()
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
						if domainColumn.IsArray {
							itor, err := reteSession.Find_sp(subject, domainColumn.Predicate)
							if err != nil {
								return &result, fmt.Errorf("while finding triples of an entity of type %s: %v", tableSpec.ClassName, err)
							}
							var data []interface{}
							for !itor.IsEnd() {
								data = append(data, itor.GetObject().AsInterface())
								itor.Next()
							}
							entityRow[i] = data
							itor.ReleaseIterator()
						} else {
							obj, err := reteSession.GetObject(subject, domainColumn.Predicate)
							if err != nil {
								return &result, fmt.Errorf("while finding triples of an entity of type %s: %v", tableSpec.ClassName, err)
							}
							if obj != nil {
								entityRow[i] = obj.AsInterface()
							}
						}
					}
				}
				// entityRow is complete
				writeOutputc[tableName] <- entityRow
				ctor.Next()
			}
		}

		result.executeRulesCount += 1
	}
	return &result, nil
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
