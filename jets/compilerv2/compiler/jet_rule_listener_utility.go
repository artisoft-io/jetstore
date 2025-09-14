package compiler

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// antlr v4 JetRuleListener interface implementation

// ResourceManager functions and utility methods

func (s *JetRuleListener) ParseObjectAtom(txt string, keywordsContextValue string) int {
	r := parseObjectAtom(txt, keywordsContextValue)
	if len(r.Id) > 0 { // Type == "identifier"
		// Type is actually resource or volatile_resource
		if res, exists := s.resourceManager.ResourceById[r.Id]; exists {
			return res.Key
		}
		// Resource not found - log error and create it as inline resource
		fmt.Fprintf(s.errorLog, "error: identifier '%s' must be defined in a declaration section before use, creating as resource\n", r.Id)
		fmt.Fprintf(s.parseLog, "error: identifier '%s' must be defined in a declaration section before use, creating as resource\n", r.Id)
		r.Type = "resource"
		r.Value = r.Id
	}
	skey := r.SKey()
	if res, exists := s.resourceManager.Resources[skey]; exists {
		// Resource already exists
		return res.Key
	}
	// It's a new Resource
	if r.Type == "var" {
		// Variable - Id is the normalized variable name
		r.Id = fmt.Sprintf("?x%d", len(s.currentRuleVarByValue)+1)
		s.currentRuleVarByValue[r.Id] = r.Value
	} else {
		r.Inline = true
	}
	s.newResource(&r)
	return r.Key
}

// AddResource adds a resource to the model if it does not already exist.
// It performs validation and ensures uniqueness based on type and value.
// It also validate that two resources with the same Id are identical.
// returns its key
func (s *JetRuleListener) AddResource(r rete.ResourceNode) int {
	if r.Type == "volatile_resource" {
		r.Value = fmt.Sprintf("_0:%s", r.Value) // add prefix
	}
	skey := r.SKey()
	if res, exists := s.resourceManager.Resources[skey]; exists {
		// Resource already exists - see if we need to make any
		if r.Type == "var" {
			// Variable - nothing to do
			return res.Key
		}
		// Set Id if was not set
		if len(res.Id) == 0 && len(r.Id) > 0 {
			res.Id = r.Id
		} else {
			// Check if it's a duplicate resource
			if res.Id == r.Id {
				return res.Key
			}
		}
	} else {
		// Check if Id already exists with different type/value
		if len(r.Id) > 0 {
			if resById, existsById := s.resourceManager.ResourceById[r.Id]; existsById {
				if resById.Type != r.Type || resById.Value != r.Value {
					// Conflict - same Id with different type/value
					fmt.Fprintf(s.errorLog, "error: resource Id conflict for Id '%s': existing (%s|%s), new (%s|%s), keeping the existing resource\n",
						r.Id, resById.Type, resById.Value, r.Type, r.Value)
					fmt.Fprintf(s.parseLog, "error: resource Id conflict for Id '%s': existing (%s|%s), new (%s|%s), keeping the existing resource\n",
						r.Id, resById.Type, resById.Value, r.Type, r.Value)
					// Keep the existing resource
					return resById.Key
				}
			}
		}
	}
	// It's a new Resource (or Id is different)
	if r.Type == "var" {
		// Variable - Id is the normalized variable name
		r.Id = fmt.Sprintf("?x%d", len(s.currentRuleVarByValue)+1)
		s.currentRuleVarByValue[r.Id] = r.Value
	}
	if len(r.Id) == 0 {
		r.Inline = true
	}
	s.newResource(&r)
	return r.Key
}

// Alias for AddResource
func (s *JetRuleListener) AddR(id string) int {
	return s.AddResource(rete.ResourceNode{
		Type:  "resource",
		Id:    id,
		Value: id,
	})
}
func (s *JetRuleListener) AddV(name string) int {
	return s.AddResource(rete.ResourceNode{
		Type:  "var",
		Value: name,
	})
}

func (s *JetRuleListener) newResource(r *rete.ResourceNode) {
	s.resourceManager.NextKey++
	r.Key = s.resourceManager.NextKey
	r.SourceFileName = s.currentRuleFileName
	s.resourceManager.ResourceById[r.Id] = r
	s.resourceManager.ResourceByKey[r.Key] = r
	s.resourceManager.Resources[fmt.Sprintf("%s|%s", r.Type, r.Value)] = r
	s.jetRuleModel.Resources = append(s.jetRuleModel.Resources, *r)
	// if s.trace {
	// 	fmt.Fprintf(s.parseLog, "** New resource: %+v\n", r)
	// }
}

func (s *JetRuleListener) Resource(key int) *rete.ResourceNode {
	if res, exists := s.resourceManager.ResourceByKey[key]; exists {
		return res
	}
	return nil
}

// Parse the triple atom, identify it's type and return it as a ResourceNode
// possible inputs:
//
//	?clm        -> {type: "var", value: "?clm"}
//	rdf:type    -> {type: "identifier", id: "rdf:type"}
//	localVal    -> {type: "identifier", id: "localVal"}
//	"XYZ"       -> {type: "text", value: "XYZ"}
//	text("XYZ") -> {type: "text", value: "XYZ"}
//	int(1)      -> {type: "int", value: "1"}
//	bool("1")   -> {type: "bool", value: "1"}
//	true        -> {type: "keyword", value: "true"}
//	-123        -> {type: "int", value: "-123"}
//	+12.3       -> {type: "double", value: "+12.3"}
func parseObjectAtom(txt string, keywordsContextValue string) rete.ResourceNode {
	if len(txt) == 0 && len(keywordsContextValue) == 0 {
		return rete.ResourceNode{}
	}
	switch {
	case strings.HasPrefix(txt, "?"):
		// Variable
		return rete.ResourceNode{
			Type:  "var",
			Value: txt,
		}
	case strings.HasPrefix(txt, "\"") && strings.HasSuffix(txt, "\""):
		// String
		return rete.ResourceNode{
			Type:  "text",
			Value: StripQuotes(txt),
		}
	case strings.HasSuffix(txt, ")"):
		// Literal cast
		v := strings.Split(txt, "(")
		if len(v) == 2 {
			typ := v[0]
			val := strings.TrimSuffix(v[1], ")")
			return rete.ResourceNode{
				Type:  typ,
				Value: StripQuotes(val),
			}
		}
	case len(keywordsContextValue) > 0:
		// Keyword
		return rete.ResourceNode{
			Type:  "keyword",
			Value: keywordsContextValue,
		}
	case isNumeric(txt):
		// Numeric (int or double)
		if strings.Contains(txt, ".") {
			return rete.ResourceNode{
				Type:  "double",
				Value: txt,
			}
		}
		return rete.ResourceNode{
			Type:  "int",
			Value: txt,
		}
	default:
		// Identifier (resource / volatile_resource)
		return rete.ResourceNode{
			Type: "identifier",
			Id:   EscR(txt),
		}
	}
	return rete.ResourceNode{}
}

// Determine if the string is numeric (int or double)
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	dotCount := 0
	for i, c := range s {
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
		} else if c < '0' || c > '9' {
			if i == 0 && (c == '-' || c == '+') {
				continue
			}
			return false
		}
	}
	return true
}

// Escape resource name that conflicts with keywords such as rdf:type becomes rdf:"type"
// this function removes the quotes
func EscR(s string) string {
	if len(s) > 4 && strings.Contains(s, ":\"") {
		return strings.ReplaceAll(s, "\"", "")
	}
	return s
}

func StripQuotes(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
