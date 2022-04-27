package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/jackc/pgx/v4/pgxpool"
)

type ReteWorkspace struct {
	js          *bridge.JetStore
	workspaceDb string
	lookupDb    string
	ruleset     string
	outTables   []string
	procConfig  *ProcessConfig
}

type ExecuteRulesResult struct {
	executeRulesCount int
}

// Load the rete workspace database via cgo
func LoadReteWorkspace(workspaceDb string, lookupDb string, ruleset string, procConfig *ProcessConfig, outTables []string) (*ReteWorkspace, error) {
	// load the workspace db
	reteWorkspace := ReteWorkspace{
		workspaceDb: workspaceDb,
		lookupDb:    lookupDb,
		ruleset:     ruleset,
		procConfig:  procConfig,
		outTables:   outTables,
	}
	js, err := bridge.LoadJetRules(workspaceDb, lookupDb)
	if err != nil {
		return &reteWorkspace, fmt.Errorf("while loading workspace db: %v", err)
	}
	reteWorkspace.js = js

	// assert the rule config triples to meta graph
	err = reteWorkspace.assertRuleConfig()
	return &reteWorkspace, err
}

// main processing function to execute rules
func (rw *ReteWorkspace) ExecuteRules(
	dbpool *pgxpool.Pool,
	processInput *ProcessInput,
	inputDataProperties []DomainColumn,
	dataInputc <-chan [][]string,
	outputMapping DomainColumnMapping,
	writeOutputc map[string]chan []string) (*ExecuteRulesResult, error) {
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
	ncol := len(inputDataProperties)
	rdfType, err := rw.js.GetResource("rdf:type")
	if err != nil {
		return &result, fmt.Errorf("while creating rdf:type resource: %v", err)
	}
	for inputRecords := range dataInputc {
		log.Println("Start Rete Session")
		reteSession, err := rw.js.NewReteSession(*ruleset)
		if err != nil {
			return &result, fmt.Errorf("while calling NewReteSession: %v", err)
		}
		// Each row in inputRecords is a jets:Entity, with it's own jets:key
		for _, row := range inputRecords {
			if len(row) == 0 {
				continue
			}
			log.Println("Asserting Row")
			jets__key := row[processInput.keyPosition]
			subject, err := reteSession.NewResource(jets__key)
			if err != nil {
				return &result, fmt.Errorf("while creating row's subject resource (NewResource): %v", err)
			}
			if subject == nil || rdfType == nil || processInput.entityRdfTypeResource == nil {
				return &result, fmt.Errorf("while asserting row rdf type")
			}
			ret, err := reteSession.Insert(subject, rdfType, processInput.entityRdfTypeResource)
			if err != nil {
				return &result, fmt.Errorf("while asserting row rdf type: %v", err)
			}
			if ret > 0 {
				log.Println("Row rdf:type asserted!")
			}
			for icol := 0; icol < ncol; icol++ {
				// asserting input row with mapping spec (make it conditional via func)
				inputColumn := &inputDataProperties[icol]
				if inputColumn.mappingSpec == nil {
					log.Println("ERROR MappingSpec is null")
					return &result, fmt.Errorf("ERROR mappingSpec is null")
				}
				var obj string
				if inputColumn.mappingSpec.functionName.Valid {
					switch inputColumn.mappingSpec.functionName.String {
					case "to_upper":
						obj = strings.ToUpper(row[icol])
					case "to_zip5":
					case "reformat0":
					case "apply_regex":
					case "scale_units":
					case "parse_amount":
					default:
						return &result, fmt.Errorf("ERROR unknown mapping function: %s", inputColumn.mappingSpec.functionName.String)
					}

				} else {
					obj = row[icol]
				}
				// cast obj to type
				// switch inputColumn.DataType {
				var object *bridge.Resource
				var err error
				switch inputColumn.DataType {
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
					err = fmt.Errorf("ERROR assertRuleConfig: unknown rdf type for object: %s", inputColumn.DataType)
				}
				if err != nil || len(obj)==0 {
					//* try the default value
					return &result, fmt.Errorf("while mapping input value: %v", err)
				}
				if subject == nil {
					return &result, fmt.Errorf("ERROR subject is null")
				}
				if inputColumn.Predicate == nil {
					return &result, fmt.Errorf("ERROR predicate is null")
				}
				if object == nil {
					return &result, fmt.Errorf("ERROR object is null")
				}
				reteSession.Insert(subject, inputColumn.Predicate, object)
			}
			// done asserting
			err = reteSession.ExecuteRules()
			if err != nil {
				return &result, fmt.Errorf("while reteSession.ExecuteRules: %v", err)
			}
			log.Println("ExecuteRule() Completed sucessfully")
			result.executeRulesCount += 1
		}

	}
	return &result, nil
}

// helper function to add meta graph resource corresponding to column names
func (rw *ReteWorkspace) addPredicate(domainColumns []DomainColumn) error {
	for _, dc := range domainColumns {
		var err error
		dc.Predicate, err = rw.js.NewResource(dc.PropertyName)
		if err != nil {
			return fmt.Errorf("while creating predicate for DomainColumn: %v", err)
		}
	}
	return nil
}
func (rw *ReteWorkspace) addRdfType(processInput *ProcessInput) error {
	var err error
	processInput.entityRdfTypeResource, err = rw.js.NewResource(processInput.entityRdfType)
	return err
}

func (rw *ReteWorkspace) assertRuleConfig() error {
	if rw == nil {
		return fmt.Errorf("ERROR: ReteWorkspace cannot be nil")
	}
	for _, t3 := range rw.procConfig.ruleConfigs {
		subject, err := rw.js.NewResource(t3.subject)
		if err != nil {
			return fmt.Errorf("while asserting rule config (NewResource): %v", err)
		}
		predicate, err := rw.js.NewResource(t3.subject)
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
