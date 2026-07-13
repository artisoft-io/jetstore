package compute_pipes

import (
	"log"
	"strings"
)

// Navigate recursively the object properties and extract their values into a map[string]any
// excluding the properties starting with _0:
func ExtractAsEntity(rdfSession JetRdfSession, subject RdfNode,
	entityObj map[string]any, currentSourcePeriod int, outChannel *JetrulesOutputChan) {
	var objProperties map[string]RdfNode
	itor := rdfSession.FindS(subject)
	defer itor.Release()
	for !itor.IsEnd() {
		log.Printf("*** Triple (%s, %s, %s)", itor.GetSubject(), itor.GetPredicate(), itor.GetObject())
		prop := itor.GetPredicate()
		if strings.HasPrefix(prop.String(), "_0:") {
			itor.Next()
			continue
		}
		// Check if it's an obj property
		if isEntity(rdfSession, itor.GetObject()) {
			if objProperties == nil {
				objProperties = make(map[string]RdfNode)
			}
			objProperties[prop.String()] = prop
		} else {
			// It's a literal property, extract its value
			addToEntityObj(entityObj, prop.String(), itor.GetObject().Value())
		}
		itor.Next()
	}
	// extract the object properties recursively
	for prop, node := range objProperties {
		jtor := rdfSession.FindSP(subject, node)
		for !jtor.IsEnd() {
			subEntityObj := make(map[string]any)
			addToEntityObj(entityObj, prop, subEntityObj)
			ExtractAsEntity(rdfSession, jtor.GetObject(), subEntityObj, currentSourcePeriod, outChannel)
			jtor.Next()
		}
	}
}

func isEntity(rdfSession JetRdfSession, node RdfNode) bool {
	itor := rdfSession.FindS(node)
	defer itor.Release()
	return !itor.IsEnd()
}

func addToEntityObj(entityObj map[string]any, prop string, value any) {
	if value == nil {
		return
	}
	if existing, ok := entityObj[prop]; ok {
		// If existing is any, then create a slice to hold current and existing values
		// If existing is []any then add to it
		switch existingVal := existing.(type) {
		case []any:
			existingVal = append(existingVal, value)
			entityObj[prop] = existingVal
		case nil:
			entityObj[prop] = value
		default:
			entityObj[prop] = []any{existingVal, value}
		}
	} else {
		entityObj[prop] = value
	}
}
