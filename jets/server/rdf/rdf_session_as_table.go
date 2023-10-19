package rdf

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

// Returns map[string]interface{} which is
//     {
//       "totalRowCount": int,
//       "rows": [][]interface{}
//     }
// rows is a list of triples, each triple is (s, p, o, o.type)
func RDFSessionAsTable(rdfSession *bridge.RDFSession, limit int) *map[string]interface{} {
	if rdfSession == nil {
		return nil
	}
	ctor, err := rdfSession.FindAll()
	if err != nil {
		log.Printf("while call findAll on rdfSession: %v", err)
		return nil
	} 
	defer ctor.ReleaseIterator()
	resultRows := make([][]interface{}, 0, limit)

	sz := 0
	for !ctor.IsEnd() && sz < limit {
		o := ctor.GetObject()
		flatRow := []interface{} {
			ctor.GetSubject().AsTextSilent(), 
			ctor.GetPredicate().AsTextSilent(), 
			o.AsTextSilent(), 
			o.GetTypeName(),
		}
		resultRows = append(resultRows, flatRow)
		ctor.Next()
		sz += 1
	}
	sort.Slice(resultRows, func(i, j int) bool { 
		if resultRows[i][0] == resultRows[j][0] {
			if resultRows[i][1] == resultRows[j][1] {
				return resultRows[i][2].(string) < resultRows[j][2].(string)
			} else {
				return resultRows[i][1].(string) < resultRows[j][1].(string)
			}
		} else {
			return resultRows[i][0].(string) < resultRows[j][0].(string)
		}
	})
	results := make(map[string]interface{})
	results["totalRowCount"] = sz
	results["rows"] = resultRows
	return &results
}

func RDFSessionAsTableJson(rdfSession *bridge.RDFSession, limit int) []byte {
	r,_ := json.Marshal(RDFSessionAsTable(rdfSession, limit))
	return r
}

// Returns map[string]interface{} which is
//    {
//      "rdf_types": string (json of [][]string),
//      "entity_key_by_type": map[string]string (json of [][]string),
//      "entity_details_by_key": map[string]string (json of [][]string),
//    }
// rdf_type: JetModel ([][]string): List of rdf:type, single column model
// entity_key_by_type: Map[rdf:type]JetModel: JetModel is list of jet:key, single column model
// entity_details_by_key: Map[jets:key]EncodedJetModel, 
//	where EncodedJetModel is encoded json of JetModel ([][]string): List of ["property", "value", "value.type"] of obj w/ jets:key, 2 columns JetModel
func RDFSessionAsTableV2(rdfSession *bridge.RDFSession, js *bridge.JetStore) (*map[string]interface{}, error) {
	if rdfSession == nil {
		return nil, fmt.Errorf("RDFSessionAsTableV2: error rdfSession cannot be nil")
	}

	// Set of rdf:type
	rdfTypeSet := make(map[string]bool)

	// Set of entity
	entitySet := make(map[string]*bridge.Resource)

	// Map of rdf:key by rdf:type: map[rdf:type][]rdf:key
	entityKeyByType := make(map[string]*[][]string)

	ri := NewRdfResources(js)
	// Create the rdf_type (rdfTypeSet) and entity_key_by_type (entityKeyByType) data structures
	ctor, err := rdfSession.Find(nil, ri.rdf__type, nil)
	if err != nil {
		return nil, fmt.Errorf("while calling Find(nil, ri.rdf__type, nil) on rdfSession: %v", err)
	} 
	for !ctor.IsEnd() {
		entity := ctor.GetSubject()
		entityName,_ := entity.AsText()

		rdfType,_ := ctor.GetObject().AsText()
		rdfTypeSet[rdfType] = true
		entitySet[entityName] = entity
		entities := entityKeyByType[rdfType]
		if entities == nil {
			entities = &[][]string{}
			entityKeyByType[rdfType] = entities
		}
		*entities = append(*entities, []string{entityName})
		ctor.Next()
	}
	ctor.ReleaseIterator()

	// Now create the entity_details_by_key: Map[jets:key]*[][]string
	entityDetailsByKey := make(map[string]*[][]string)
	for entityKey, entity := range entitySet {
		ctor, err := rdfSession.Find_s(entity)
		if err != nil {
			return nil, fmt.Errorf("while calling Find_s(entity) on rdfSession: %v", err)
		} 
		for !ctor.IsEnd() {
			propertyName,_ := ctor.GetPredicate().AsText()
			value := ctor.GetObject()
			valueType := value.GetTypeName()
			model := entityDetailsByKey[entityKey]
			if model == nil {
				model = &[][]string{}
				entityDetailsByKey[entityKey] = model
			}
			*model = append(*model, []string{propertyName, value.AsTextSilent(), valueType})
			ctor.Next()
		}
		ctor.ReleaseIterator()	
	}

	// Put all the results in the output map
	results := make(map[string]interface{})
	
	// Package rdfTypeSet
	rdfTypesResult := make([][]string, 0)
	for rdfType := range rdfTypeSet {
		rdfTypesResult = append(rdfTypesResult, []string{rdfType})
	}
	sort.Slice(rdfTypesResult, func(i, j int) bool { 
		return rdfTypesResult[i][0] < rdfTypesResult[j][0]
	})
	r, err := json.Marshal(rdfTypesResult)
	if err != nil {
		return nil, err
	}
	results["rdf_types"] = string(r)

	// Package entityKeyByType
	entityKeyByTypeResult := make(map[string]string)
	for rdfType, keys := range entityKeyByType {
		sort.Slice(*keys, func(i, j int) bool { 
			return (*keys)[i][0] < (*keys)[j][0]
		})
		r, err := json.Marshal(*keys)
		if err != nil {
			return nil, err
		}
		entityKeyByTypeResult[rdfType] = string(r)
	} 
	results["entity_key_by_type"] = entityKeyByTypeResult

	// Package entityDetailsByKey
	entityDetailsByKeyResult := make(map[string]string)
	for key, details := range entityDetailsByKey {
		sort.Slice(*details, func(i, j int) bool { 
			if (*details)[i][0] == (*details)[j][0] {
				if (*details)[i][1] == (*details)[j][1] {
					return (*details)[i][2] < (*details)[j][2]
				}
				return (*details)[i][1] < (*details)[j][1]	
			}
			return (*details)[i][0] < (*details)[j][0]
		})
		r, err := json.Marshal(*details)
		if err != nil {
			return nil, err
		}
		entityDetailsByKeyResult[key] = string(r)
	} 
	results["entity_details_by_key"] = entityDetailsByKeyResult

	return &results, nil
}

func RDFSessionAsTableJsonV2(rdfSession *bridge.RDFSession, js *bridge.JetStore) ([]byte, error) {
	v, err := RDFSessionAsTableV2(rdfSession, js)
	if err != nil {
		return nil, err
	}
	r,_ := json.Marshal(v)
	return r, nil
}
