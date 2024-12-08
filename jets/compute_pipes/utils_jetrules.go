package compute_pipes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/google/uuid"
)

// Utility functions for jetrules transformation pipes operator

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
func AssertRuleConfiguration(reteMetaStore *rete.ReteMetaStoreFactory, 	config *JetrulesSpec) (err error) {
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