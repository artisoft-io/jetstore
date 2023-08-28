package rdf

import (
	"encoding/json"
	"log"
	"sort"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

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