package compute_pipes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Utility functions for jetrules transformation pipes operator

// metaStoreFactoryMap is a map mainRuleName -> *ReteMetaStoreFactory
var metaStoreFactoryMap *sync.Map = new(sync.Map)
var inputMappingCache *sync.Map = new(sync.Map)
var dataPropertyInfoMap map[string]*rete.DataPropertyNode
var dataPropertyInfoMx sync.Mutex

// Assert source period info (date, period, type) to rdf graph
func AssertSourcePeriodInfo(config *JetrulesSpec, graph *rdf.RdfGraph, rm *rdf.ResourceManager) (err error) {
	jr := rm.JetsResources
	_, err = graph.Insert(jr.Jets__istate, jr.Jets__currentSourcePeriod, rm.NewIntLiteral(config.CurrentSourcePeriod))
	if err != nil {
		return
	}
	if config.CurrentSourcePeriodDate != "" {
		d, err2 := rdf.NewLDate(config.CurrentSourcePeriodDate)
		if err2 == nil {
			_, err = graph.Insert(jr.Jets__istate, jr.Jets__currentSourcePeriodDate, rm.NewDateLiteral(d))
			if err != nil {
				return
			}
		}
	}
	if config.CurrentSourcePeriodType != "" {
		_, err = graph.Insert(jr.Jets__istate, jr.Jets__currentSourcePeriodDate,
			rm.NewTextLiteral(config.CurrentSourcePeriodType))
		if err != nil {
			return
		}
	}
	return
}

// Assert rule config to meta graph from the pipeline configuration
func AssertRuleConfiguration(reteMetaStore *rete.ReteMetaStoreFactory, config *JetrulesSpec) (err error) {
	var object *rdf.Node
	for _, rc := range config.RuleConfig {

		// determine the subject of rc (look for jets:key or use a uuid)
		var subjectTxt string
		s, ok := rc["jets:key"]
		if ok {
			subjectTxt, _, err = extractValue(s)
			if err != nil {
				return
			}
		} else {
			subjectTxt = uuid.New().String()
		}
		subject := reteMetaStore.ResourceMgr.NewResource(subjectTxt)

		for predicateTxt := range rc {
			value, rdfType, err2 := extractValue(rc[predicateTxt])
			if err2 != nil {
				return err2
			}
			predicate := reteMetaStore.ResourceMgr.NewResource(predicateTxt)
			object, err = ParseObject(reteMetaStore.ResourceMgr, value, rdfType)
			if err != nil {
				return
			}
			// Assert the triple
			_, err = reteMetaStore.MetaGraph.Insert(subject, predicate, object)
			if err != nil {
				return
			}
		}
	}
	return
}

// Function to extract value and type from json struct
func extractValue(e interface{}) (value, rdfType string, err error) {
	switch obj := e.(type) {
	case string:
		value = obj
		rdfType = "text"
		return
	case map[string]interface{}:
		// fmt.Println("*** Domain Key is a struct of composite keys", value)
		for k, v := range obj {
			switch vv := v.(type) {
			case string:
				switch k {
				case "value":
					value = vv
				case "type":
					rdfType = vv
				default:
					err = fmt.Errorf("rule_config_json contains invalid key %s", k)
					return
				}
			default:
				err = fmt.Errorf("rule_config_json contains %v which is of a type that is not supported", vv)
				return
			}
		}
		return
	default:
		err = fmt.Errorf("rule_config_json contains %v which is of a type that is not supported", obj)
		return
	}
}

func ParseObject(rm *rdf.ResourceManager, object, rdfType string) (node *rdf.Node, err error) {
	var key int
	var date rdf.LDate
	var datetime rdf.LDatetime
	switch strings.TrimSpace(rdfType) {
	case "null":
		node = rdf.Null()
	case "bn":
		key, err = strconv.Atoi(object)
		if err != nil {
			return
		}
		node = rm.CreateBNode(key)
	case "resource":
		node = rm.NewResource(object)
	case "int":
		var v int
		_, err = fmt.Sscan(object, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: %v", err)
		}
		node = rm.NewIntLiteral(v)
	case "bool":
		v := 0
		if len(object) > 0 {
			c := strings.ToLower(object[0:1])
			switch c {
			case "t", "1", "y":
				v = 1
			case "f", "0", "n":
				v = 0
			default:
				return nil, fmt.Errorf("while rule config triple; object is not bool: %s", object)
			}
		}
		node = rm.NewIntLiteral(v)
	case "long":
		var v int
		_, err = fmt.Sscan(object, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: %v", err)
		}
		node = rm.NewIntLiteral(v)
	case "double":
		var v float64
		_, err = fmt.Sscan(object, &v)
		if err != nil {
			return nil, fmt.Errorf("while asserting rule config: %v", err)
		}
		node = rm.NewDoubleLiteral(v)
	case "text":
		node = rm.NewTextLiteral(object)
	case "date":
		date, err = rdf.NewLDate(object)
		node = rm.NewDateLiteral(date)
	case "datetime":
		datetime, err = rdf.NewLDatetime(object)
		node = rm.NewDatetimeLiteral(datetime)
	default:
		err = fmt.Errorf("ERROR ParseObject: unknown rdf type for object: %s", rdfType)
	}
	return
}

// Function to get the jetrules factory for a rule process
func GetJetrulesFactory(dbpool *pgxpool.Pool, processName string) (reteMetaStore *rete.ReteMetaStoreFactory, err error) {
	// Get the Rete MetaStore for the mainRules
	msf, _ := metaStoreFactoryMap.Load(processName)
	if msf == nil {
		var mainRules string
		stmt := `SELECT	pc.main_rules FROM jetsapi.process_config pc WHERE pc.process_name = $1`
		err := dbpool.QueryRow(context.Background(), stmt, processName).Scan(&mainRules)
		if err != nil {
			return nil,
				fmt.Errorf("quering main rule file name for process %s from jetsapi.process_config failed: %v",
					processName, err)
		}
		if len(mainRules) == 0 {
			return nil, fmt.Errorf("error: main rule file name is empty for process %s", processName)
		}
		log.Printf("Rete Meta Store for ruleset '%s' for process '%s' not loaded, loading from local workspace",
			mainRules, processName)
		reteMetaStore, err = rete.NewReteMetaStoreFactory(mainRules)
		if err != nil {
			return nil,
				fmt.Errorf("while loading ruleset '%s' for process '%s' from local workspace via NewReteMetaStoreFactory: %v",
					mainRules, processName, err)
		}
		metaStoreFactoryMap.Store(processName, reteMetaStore)
	} else {
		reteMetaStore = msf.(*rete.ReteMetaStoreFactory)
	}
	return
}

func GetWorkspaceDataProperties() (map[string]*rete.DataPropertyNode, error) {
	if dataPropertyInfoMap == nil {
		dataPropertyInfoMx.Lock()
		defer dataPropertyInfoMx.Unlock()
		fmt.Println("Load Data Properties from local Workspace")
		dataPropertyInfoMap = make(map[string]*rete.DataPropertyNode)
		fpath := fmt.Sprintf("%s/%s/build/properties.json", workspaceHome, wsPrefix)
		log.Println("Reading JetStore tables definitions from:", fpath)
		file, err := os.ReadFile(fpath)
		if err != nil {
			err = fmt.Errorf("while reading properties.json file (GetWorkspaceDataProperties):%v", err)
			log.Println(err)
			return nil, err
		}
		err = json.Unmarshal(file, &dataPropertyInfoMap)
		if err != nil {
			err = fmt.Errorf("while unmarshaling properties.json (GetWorkspaceDataProperties):%v", err)
			log.Println(err)
			return nil, err
		}
	}
	return dataPropertyInfoMap, nil
}

type InputMappingExpr struct {
	InputColumn           sql.NullString
	DataProperty          string
	CleansingFunctionName sql.NullString
	Argument              sql.NullString
	DefaultValue          sql.NullString
	ErrorMessage          sql.NullString
}

// read mapping definitions
func GetInputMapping(dbpool *pgxpool.Pool, tableName string) ([]InputMappingExpr, error) {
	item, _ := inputMappingCache.Load(tableName)
	if item == nil {
		rows, err := dbpool.Query(context.Background(),
			`SELECT input_column, data_property, function_name, argument, default_value, error_message
		FROM jetsapi.process_mapping WHERE table_name=$1`, tableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		// Loop through rows, using Scan to assign column data to struct fields.
		result := make([]InputMappingExpr, 0)
		for rows.Next() {
			var pm InputMappingExpr
			if err := rows.Scan(&pm.InputColumn, &pm.DataProperty, &pm.CleansingFunctionName,
				&pm.Argument, &pm.DefaultValue, &pm.ErrorMessage); err != nil {
				return nil, err
			}
			// validate that we don't have both a default and an error message
			if pm.ErrorMessage.Valid && pm.DefaultValue.Valid {
				if len(pm.DefaultValue.String) > 0 && len(pm.ErrorMessage.String) > 0 {
					log.Printf(
						"WARNING: Cannot have both a default value and an error message in table %s jetsapi.process_mapping\n",
						tableName)
				}
			}
			result = append(result, pm)
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
		item = result
		inputMappingCache.Store(tableName, item)
	}
	return item.([]InputMappingExpr), nil
}