package symbolic

// reserved are the grammar's keywords that have the shape of an identifier
// (every quoted literal token in JetRule.g4 matching [A-Za-z_][A-Za-z0-9_]*).
// The lexer reads each as its own token and never as an Identifier, so none
// can stand where the grammar wants one: a rule name, a variable, a bare id or
// a prefix. A local name that is one is escaped instead (renderID).
//
// TestReservedMatchesGrammar re-derives this set from JetRule.g4, so a keyword
// added to the grammar fails a test here rather than producing a rule file
// the parser refuses.
var reserved = map[string]bool{
	"and": true, "as": true, "bool": true, "class": true, "date": true, "datetime": true,
	"deleted": true, "double": true, "false": true, "int": true, "jetstore_config": true,
	"long": true, "lookup_table": true, "main": true, "not": true, "null": true, "or": true,
	"resource": true, "rule_sequence": true, "text": true, "toText": true, "triple": true,
	"true": true, "uint": true, "ulong": true, "volatile_resource": true,
}
