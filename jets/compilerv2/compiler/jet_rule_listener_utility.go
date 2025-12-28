package compiler

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// antlr v4 JetRuleListener interface implementation

// WorkspaceResourceManager functions and utility methods

// Parse the triple atom, identify it's type and return its key in the WorkspaceResourceManager
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
//
// Note: if the type is "var" then it's a variable and a new resource is created in the scope of the rule only. 
// Note: if the type is "identifier" then it's actually either "resource" or "volatile_resource",
// this is resolved by the WorkspaceResourceManager when adding to the model.
// returns the ResourceNode.
// Note: s.currentRuleVarByValue must be initialized (at the start of a rule) before this function is called
func (s *JetRuleListener) ParseObjectAtom(txt string, keywordsContextValue string) *rete.ResourceNode {
	r := s.parseObjectAtom(txt, keywordsContextValue)
	if r == nil {
		return nil
	}
	switch r.Type {

	case "var":
		return s.AddVariable(r.Value)

	case "identifier":
		// Type is either a defined resource / volatile_resource or an inlined literal resource
		if res, exists := s.resourceManager.ResourceById[r.Id]; exists {
			return res
		}
		// Resource not found - log error and create it as inline resource
		if !s.autoAddResources {
			fmt.Fprintf(s.errorLog, "error: identifier '%s' must be defined in a declaration section before use, creating as resource\n", r.Id)
			fmt.Fprintf(s.parseLog, "error: identifier '%s' must be defined in a declaration section before use, creating as resource\n", r.Id)
		}
		r.Type = "resource"
		r.Value = r.Id
	}

	// Check if resource already exists
	skey := r.SKey()
	if res, exists := s.resourceManager.Resources[skey]; exists {
		// Resource already exists
		return res
	}
	// It's a new Resource, inlined literal resource
	r.Inline = true
	s.newResource(r)
	return r
}

// Variable - Id is the normalized variable name
// Variables are unique in the context of a rule.
func (s *JetRuleListener) AddVariable(name string) *rete.ResourceNode {
	// Check for existing variables within the rule
	if varNode := s.currentRuleVarByValue[name]; varNode != nil {
		// Variable already exists
		return varNode
	}
	// New variable
	r := &rete.ResourceNode{
		Type:  "var",
		Value: name,
		Id:    fmt.Sprintf("?x%02d", len(s.currentRuleVarByValue)+1),
	}
	s.currentRuleVarByValue[name] = r
	s.newResource(r)
	return r
}

// AddResource adds a named resource or a literal to the model.
// This function is called for resources and literals declaration and in expressions
// (filters, consequent obj expressions).
// If The resource Type is "volatile_resource" then it adds the prefix "_0:" to the value
// to ensure it's unique and not conflicting with any other resources.
// If a resource with the same Type and Value already exists, it returns the existing resource.
// If a resource with the same Id already exists but with different Type or Value, it logs an error
// and keeps the existing resource.
// If the resource is new, it assigns a new key, sets the SourceFileName, and adds it to the model.
// It performs validation and ensures uniqueness based on type and value.
// It also validate that two resources with the same Id are identical.
// returns the resource node.
// Note: s.currentRuleVarByValue must be initialized (at the start of a rule) before this function is called
func (s *JetRuleListener) AddResource(r *rete.ResourceNode) *rete.ResourceNode {
	if r.Type == "var" {
		// Add variables, this is for a variable in an expression
		return s.AddVariable(r.Value)
	}
	if r.Type == "volatile_resource" {
		r.Value = fmt.Sprintf("_0:%s", r.Value) // add prefix
	}
	// Check if resource already exists
	skey := r.SKey()
	if res, exists := s.resourceManager.Resources[skey]; exists {
		// Resource already exists - see if we need to make any changes
		// Set Id if was not set
		if len(res.Id) == 0 && len(r.Id) > 0 {
			res.Id = r.Id
		} else {
			// Check if it's a duplicate resource
			if res.Id == r.Id {
				return res
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
					return resById
				}
			}
		}
	}
	// It's a new Resource (or Id is different)
	if len(r.Id) == 0 {
		r.Inline = true
	}
	s.newResource(r)
	return r
}

// Alias for AddResource
func (s *JetRuleListener) AddR(id string) *rete.ResourceNode {
	return s.AddResource(&rete.ResourceNode{
		Type:  "resource",
		Id:    id,
		Value: id,
	})
}
func (s *JetRuleListener) AddV(name string) *rete.ResourceNode {
	return s.AddResource(&rete.ResourceNode{
		Type:  "var",
		Value: name,
	})
}

// Add resource to Resource Manager, assign a new key, set source file name and add to model
// Case of var, add a temp resource (ie, do not add to ResourceById and Resources maps)
func (s *JetRuleListener) newResource(r *rete.ResourceNode) {
	s.resourceManager.NextKey++
	r.Key = s.resourceManager.NextKey
	r.SourceFileName = s.currentRuleFileName
	s.resourceManager.ResourceByKey[r.Key] = r
	if r.Type != "var" {
		s.resourceManager.ResourceById[r.Id] = r
		s.resourceManager.Resources[fmt.Sprintf("%s|%s", r.Type, r.Value)] = r
	} else {
		// Collect temp var nodes
		s.collectedTempVarNodes = append(s.collectedTempVarNodes, r)
	}
	s.jetRuleModel.Resources = append(s.jetRuleModel.Resources, r)
	// if s.trace {
	// 	fmt.Fprintf(s.parseLog, "** New resource: %+v\n", r)
	// }
}

// Add var resource to Resource Manager by domain key
// Domain key is: var|Vertex|Id|VarPos|isBinded
func (s *JetRuleListener) addVarResourceByDomainKey(r *rete.ResourceNode) *rete.ResourceNode {
	dkey := fmt.Sprintf("var|%d|%s|%d|%t", r.Vertex, r.Id, r.VarPos, r.IsBinded)
	if res, exists := s.resourceManager.Resources[dkey]; exists {
		// Resource already exists
		return res
	}
	// New variable resource
	s.resourceManager.NextKey++
	r.Key = s.resourceManager.NextKey
	r.Type = "var"
	// r.SourceFileName = s.currentRuleFileName
	s.resourceManager.ResourceByKey[r.Key] = r
	s.resourceManager.Resources[dkey] = r
	s.jetRuleModel.Resources = append(s.jetRuleModel.Resources, r)
	// if s.trace {
	// 	fmt.Fprintf(s.parseLog, "** New var resource: %+v\n", r)
	// }
	return r
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
func (s *JetRuleListener) parseObjectAtom(txt string, keywordsContextValue string) *rete.ResourceNode {
	if len(txt) == 0 && len(keywordsContextValue) == 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(txt, "?"):
		// Variable
		return &rete.ResourceNode{
			Type:  "var",
			Value: txt,
		}
	case strings.HasPrefix(txt, "\"") && strings.HasSuffix(txt, "\""):
		// String
		return &rete.ResourceNode{
			Type:  "text",
			Value: StripQuotes(txt),
		}
	case strings.HasSuffix(txt, ")"):
		// Literal cast
		v := strings.Split(txt, "(")
		if len(v) == 2 {
			typ := v[0]
			val := strings.TrimSuffix(v[1], ")")
			// validate typ is one of text, int, double, bool
			switch typ {
			case "text", "int", "uint", "long", "ulong", "double", "bool", "date", "datetime":
			default:
				// invalid type - report error, return nil
				fmt.Fprintf(s.errorLog, "error: invalid literal type '%s' for value '%s', source text: %s\n", typ, val, txt)
				fmt.Fprintf(s.parseLog, "error: invalid literal type '%s' for value '%s', source text: %s\n", typ, val, txt)
				return nil
			}
			return &rete.ResourceNode{
				Type:  typ,
				Value: StripQuotes(val),
			}
		}
	case len(keywordsContextValue) > 0:
		// Keyword
		return &rete.ResourceNode{
			Type:  "keyword",
			Value: keywordsContextValue,
		}
	case isNumeric(txt):
		// Numeric (int or double)
		if strings.Contains(txt, ".") {
			return &rete.ResourceNode{
				Type:  "double",
				Value: txt,
			}
		}
		return &rete.ResourceNode{
			Type:  "int",
			Value: txt,
		}
	default:
		// Identifier (resource / volatile_resource)
		return &rete.ResourceNode{
			Type: "identifier",
			Id:   EscR(txt),
		}
	}
	return nil
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
	if len(s) > 4 && s[0] != '"' && strings.Contains(s, ":\"") {
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
