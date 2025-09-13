package main

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)
// antlr v4 JetRuleListener interface implementation

// Utility methods

// Parse the triple atom, identify it's type and return it as a ResourceNode
// possible inputs:
//
//	?clm        -> {type: "var", value: "?clm"}
//	rdf:type    -> {type: "identifier", value: "rdf:type"}
//	localVal    -> {type: "identifier", value: "localVal"}
//	"XYZ"       -> {type: "text", value: "XYZ"}
//	text("XYZ") -> {type: "text", value: "XYZ"}
//	int(1)      -> {type: "int", value: "1"}
//	bool("1")   -> {type: "bool", value: "1"}
//	true        -> {type: "keyword", value: "true"}
//	-123        -> {type: "int", value: "-123"}
//	+12.3       -> {type: "double", value: "+12.3"}
func (s *JetRuleListener) ParseObjectAtom(txt string, keywordsContextValue string) *rete.ResourceNode {
	if len(txt) == 0 && len(keywordsContextValue) == 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(txt, "?"):
		// Variable
		s.nextKey++
		return &rete.ResourceNode{
			Type:  "var",
			Value: txt,
			Key:   s.nextKey,
		}
	case strings.HasPrefix(txt, "\"") && strings.HasSuffix(txt, "\""):
		// String
		s.nextKey++
		return &rete.ResourceNode{
			Type:  "text",
			Value: StripQuotes(txt),
			Key:   s.nextKey,
		}
	case strings.HasSuffix(txt, ")"):
		// Literal cast
		v := strings.Split(txt, "(")
		if len(v) == 2 {
			typ := v[0]
			val := strings.TrimSuffix(v[1], ")")
			s.nextKey++
			return &rete.ResourceNode{
				Type:  typ,
				Value: StripQuotes(val),
				Key:   s.nextKey,
			}
		}
	case len(keywordsContextValue) > 0:
		// Keyword
		s.nextKey++
		return &rete.ResourceNode{
			Type:  "keyword",
			Value: keywordsContextValue,
			Key:   s.nextKey,
		}
	case isNumeric(txt):
		// Numeric (int or double)
		s.nextKey++
		if strings.Contains(txt, ".") {
			return &rete.ResourceNode{
				Type:  "double",
				Value: txt,
				Key:   s.nextKey,
			}
		}
		return &rete.ResourceNode{
			Type:  "int",
			Value: txt,
			Key:   s.nextKey,
		}
	default:
		// Identifier (resource / volatile_resource)
		s.nextKey++
		return &rete.ResourceNode{
			Type:  "identifier",
			Value: EscR(txt),
			Key:   s.nextKey,
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
