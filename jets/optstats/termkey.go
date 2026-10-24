package optstats

import (
	"strconv"
	"strings"
)

// The kinds TermKey takes. They are workspace.db's resource types and the rule
// compiler's (rete.ResourceNode.Type), and the kinds a graph's node decodes to
// (rulestest/harness.Term): the two vocabularies agree on these four.
const (
	KindResource         = "resource"
	KindVolatileResource = "volatile_resource"
	KindText             = "text"
	KindInt              = "int"
)

// VolatilePrefix is the prefix the engine gives a volatile resource's name in
// the graph (jets/rete/rete_meta_store_factory.cc). workspace.db and the
// compiler hold the name without it.
const VolatilePrefix = "_0:"

// TermKey is a constant's key in the statistics file: its form in the graph.
//
//   - A resource is its name, as the engine names it in the graph.
//   - A volatile resource, given by its name without the prefix as workspace.db
//     and the compiler hold it, is that name prefixed with _0:, as the engine
//     names it. A graph node is always a resource, its name already prefixed.
//   - Text is Go-quoted (strconv.Quote).
//   - An int is <decimal>^^int, the decimal normalised.
//
// A constant of any other kind (uint, double, date, datetime, keyword, null) is
// not counted, and neither is an empty resource name or an int that does not
// parse: TermKey reports false. A keyword's true and false are ints in the
// graph, and are still not counted: the statistics follow the rule's kind.
func TermKey(kind, text string) (string, bool) {
	switch kind {
	case KindResource:
		if text == "" {
			return "", false
		}
		return text, true
	case KindVolatileResource:
		if text == "" {
			return "", false
		}
		return VolatilePrefix + text, true
	case KindText:
		return strconv.Quote(text), true
	case KindInt:
		n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return "", false
		}
		return strconv.FormatInt(n, 10) + "^^int", true
	}
	return "", false
}
