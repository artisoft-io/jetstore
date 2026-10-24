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
// the graph (jets/rete/rete_meta_store_factory.cc). workspace.db holds the name
// without it; the compiler holds a declared volatile resource with it and an
// inline one without it, until it writes workspace.db.
const VolatilePrefix = "_0:"

// VolatileName is a volatile resource's name in the graph: text with the
// prefix, added only when text does not already carry it.
func VolatileName(text string) string {
	if strings.HasPrefix(text, VolatilePrefix) {
		return text
	}
	return VolatilePrefix + text
}

// TermKey is a constant's key in the statistics file: its form in the graph.
//
//   - A resource is its name, as the engine names it in the graph.
//   - A volatile resource is its name prefixed with _0:, as the engine names it
//     (VolatileName). The text may carry the prefix or not: the compiler holds a
//     declared volatile resource with it (AddResource) and an inline one without
//     it, and workspace.db holds neither prefixed. A graph node is always a
//     resource, its name already prefixed.
//   - Text is Go-quoted (strconv.Quote).
//   - An int is <decimal>^^int, the decimal normalised.
//
// Pass the constant's value (rete.ResourceNode.Value), never its Id. The kind
// identifier is not counted: by the time a rule's term is resolved, the
// compiler's ParseObjectAtom has made every identifier a resource or a volatile
// resource, and an identifier carries an Id rather than the graph's name.
//
// A constant of any other kind (uint, double, date, datetime, keyword, null,
// identifier) is not counted, and neither is an empty resource name or an int
// that does not parse: TermKey reports false. A keyword's true and false are
// ints in the graph, and are still not counted: the statistics follow the rule's
// kind.
func TermKey(kind, text string) (string, bool) {
	switch kind {
	case KindResource:
		if text == "" {
			return "", false
		}
		return text, true
	case KindVolatileResource:
		if text == "" || text == VolatilePrefix {
			return "", false
		}
		return VolatileName(text), true
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
