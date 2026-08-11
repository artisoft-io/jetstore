package compute_pipes

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	togo "github.com/toon-format/toon-go"
)

func (ce *JrSpecialColumnEncoding) EncodeColumnData(rdfSession JetRdfSession, subject RdfNode) any {
	if ce.Config == nil {
		log.Panicf("bug: JrSpecialColumnEncoding.Config is nil")
	}
	// For toon and json encoding, we extract the entire object as a map[string]any
	// log.Printf("*** Extracting json/toon obj - start")
	entityObj := make(map[string]any)
	extractAsEntity(rdfSession, ce.Config.RemoveModelPrefixes, subject, entityObj, ce.ExcludeProperties)
	// log.Printf("*** Extracting json/toon obj - end")
	if ce.Config.EntityEncoding == "toon" {
		// For toon encoding, we need to convert the map to a toon string
		toonBytes, err := togo.Marshal(entityObj, togo.WithTimeFormatter(func(t time.Time) string {
			switch {
			case t.IsZero():
				return ""
			case t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0:
				return t.Format("2006-01-02")
			default:
				return t.Format("2006-01-02T15:04:05")
			}
		}))
		if err != nil {
			err = fmt.Errorf("error: failed to marshal entity object to toon for subject %s: %v", subject, err)
			log.Println(err)
			return err
		}
		// log.Printf("*** toon encoded obj:\n%s", string(toonBytes))
		return string(toonBytes)
	} else {
		// For json encoding, we need to convert the map to a json string
		jsonBytes, err := json.Marshal(entityObj)
		if err != nil {
			err = fmt.Errorf("error: failed to marshal entity object to json for subject %s: %v", subject, err)
			log.Println(err)
			return err
		}
		// log.Printf("*** json encoded obj:\n%s", string(jsonBytes))
		return string(jsonBytes)
	}
}

// Navigate recursively the object properties and extract their values into a map[string]any
// excluding the properties starting with _0:
func extractAsEntity(rdfSession JetRdfSession, removeModelPrefixes bool, subject RdfNode,
	entityObj map[string]any, excludeProp map[string]bool) {

	var objProperties map[string]RdfNode
	itor := rdfSession.FindS(subject)
	defer itor.Release()
	for !itor.IsEnd() {
		// log.Printf("*** Triple (%s, %s (%s), %s)", itor.GetSubject(), itor.GetPredicate(), itor.GetPredicate().Type(), itor.GetObject())
		prop := itor.GetPredicate()
		if strings.HasPrefix(prop.String(), "_0:") || prop.Type() != "named_resource" || excludeProp[prop.String()] {
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
			addToEntityObj(entityObj, removeModelPrefixes, prop.String(), itor.GetObject().Value())
		}
		itor.Next()
	}
	// extract the object properties recursively
	for prop, node := range objProperties {
		jtor := rdfSession.FindSP(subject, node)
		for !jtor.IsEnd() {
			subEntityObj := make(map[string]any)
			addToEntityObj(entityObj, removeModelPrefixes, prop, subEntityObj)
			extractAsEntity(rdfSession, removeModelPrefixes, jtor.GetObject(), subEntityObj, excludeProp)
			jtor.Next()
		}
	}
}

func isEntity(rdfSession JetRdfSession, node RdfNode) bool {
	itor := rdfSession.FindS(node)
	defer itor.Release()
	return !itor.IsEnd()
}

func addToEntityObj(entityObj map[string]any, removeModelPrefixes bool, prop string, value any) {
	if value == nil {
		return
	}
	if removeModelPrefixes {
		// Remove model prefixes from the property name, e.g. remove the start of prop upto the char : if present, e.g. jets: or rdf:
		if idx := strings.Index(prop, ":"); idx != -1 {
			prop = prop[idx+1:]
		}
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
