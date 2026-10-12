package symbolic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Rendering a symbolic rule file to JetRules source.
//
// **Every string that reaches the output is checked against the lexer first**,
// because the renderer is the one place a string from the author is spliced
// into source. An id is not copied through: "?x) -> (?y" is not a variable,
// and rendering it verbatim would let a JSON document say something its
// structure does not. So an id must be what the grammar calls an atom, a
// literal's value must be what its type's grammar admits, and an operator must
// be one the grammar has. Anything else is refused here, with a JSON Pointer to
// the element, before the compiler is asked anything.

// Rendered is a rendering and its provenance.
type Rendered struct {
	// Source is the JetRules text.
	Source string
	// Pointers maps a 1-based line of Source to the JSON Pointer of the element
	// the line was rendered from — "/rules/2/antecedents/1" — so a diagnostic
	// the compiler reports against Source can name the element the author
	// wrote. Lines rendered from nothing the author wrote (the header comment,
	// the arrow) map to "".
	Pointers []string
}

// PointerAt returns the JSON Pointer for a 1-based line of Source, or "".
func (r *Rendered) PointerAt(line int) string {
	if line < 1 || line > len(r.Pointers) {
		return ""
	}
	return r.Pointers[line-1]
}

// RenderError is a refusal to render, located in the document.
type RenderError struct {
	Pointer string
	Message string
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Pointer, e.Message)
}

// Header is the first line of every rendering, so a rendered file says where
// it came from if it is ever read on its own.
const Header = "# Rendered from a symbolic rule file (" + Suffix + ") by jets/compilerv2/symbolic; edit the JSON, not this."

var (
	identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	intRe        = regexp.MustCompile(`^[+-]*[0-9]+$`)
	uintRe       = regexp.MustCompile(`^\+*[0-9]+$`)
	doubleRe     = regexp.MustCompile(`^[+-]*[0-9]+(\.[0-9]+)?$`)
)

// binaryOps are the grammar's symbolic binary operators (binaryOp in
// JetRule.g4). An operator that is an identifier — "range", "exist_not",
// "lookup" and the rest — is admitted by identifierRe instead, as the grammar
// admits it by Identifier: which named operators exist is the engine's
// registry, and the compiler is what reports one it does not know.
var binaryOps = map[string]bool{
	"+": true, "==": true, "<": true, "<=": true, ">": true, ">=": true, "!=": true,
	"r?": true, "-": true, "*": true, "/": true, "or": true, "and": true,
}

// unaryKeywordOps are the reserved words the grammar admits as unary
// operators (unaryOp in JetRule.g4).
var unaryKeywordOps = map[string]bool{"not": true, "toText": true}

// literalTypes are the literal types the grammar has a cast for, and keyword.
var literalTypes = map[string]bool{
	"int": true, "uint": true, "long": true, "ulong": true, "double": true,
	"text": true, "date": true, "datetime": true, "bool": true, "keyword": true,
}

// Render renders a symbolic rule file as JetRules source.
func Render(f *RuleFile) (*Rendered, error) {
	r := &renderer{}
	r.line(Header, "")
	for i, imp := range f.Imports {
		ptr := fmt.Sprintf("/imports/%d", i)
		if err := checkImport(imp); err != nil {
			return nil, &RenderError{ptr, err.Error()}
		}
		r.line(fmt.Sprintf("import %q", imp), ptr)
	}
	if len(f.Rules) == 0 {
		return nil, &RenderError{"/rules", "a rule file has at least one rule"}
	}
	seen := map[string]int{}
	for i := range f.Rules {
		ptr := fmt.Sprintf("/rules/%d", i)
		if prev, dup := seen[f.Rules[i].Name]; dup {
			return nil, &RenderError{ptr + "/name",
				fmt.Sprintf("rule name %q is already used by /rules/%d", f.Rules[i].Name, prev)}
		}
		seen[f.Rules[i].Name] = i
		r.line("", "")
		if err := r.rule(&f.Rules[i], ptr); err != nil {
			return nil, err
		}
	}
	return &Rendered{Source: r.b.String(), Pointers: r.ptrs}, nil
}

type renderer struct {
	b    strings.Builder
	ptrs []string
}

func (r *renderer) line(text, ptr string) {
	r.b.WriteString(text)
	r.b.WriteString("\n")
	r.ptrs = append(r.ptrs, ptr)
}

func (r *renderer) rule(rule *Rule, ptr string) error {
	if !identifierRe.MatchString(rule.Name) || reserved[rule.Name] {
		return &RenderError{ptr + "/name",
			fmt.Sprintf("rule name %q must be an identifier (letters, digits, '_', not starting with a digit) and not a reserved word", rule.Name)}
	}
	header := &strings.Builder{}
	fmt.Fprintf(header, "[%s", rule.Name)
	if rule.Optimization != nil {
		fmt.Fprintf(header, ", o=%t", *rule.Optimization)
	}
	if rule.Salience != nil {
		fmt.Fprintf(header, ", s=%d", *rule.Salience)
	}
	for _, k := range sortedKeys(rule.Properties) {
		kptr := ptr + "/properties/" + escapePointer(k)
		switch {
		case k == "i":
			return &RenderError{kptr, "property \"i\" marks a rule the compiler generated (class inheritance) and cannot be written by an author"}
		case k == "s" || k == "o":
			return &RenderError{kptr, fmt.Sprintf(
				"property %q is written as the rule's %q field, not as a property", k,
				map[string]string{"s": "salience", "o": "optimization"}[k])}
		case !identifierRe.MatchString(k) || reserved[k]:
			return &RenderError{kptr, fmt.Sprintf("property name %q must be an identifier and not a reserved word", k)}
		}
		v, err := renderPropertyValue(rule.Properties[k].V)
		if err != nil {
			return &RenderError{kptr, err.Error()}
		}
		fmt.Fprintf(header, ", %s=%s", k, v)
	}
	header.WriteString("]:")
	r.line(header.String(), ptr)

	if len(rule.Antecedents) == 0 {
		return &RenderError{ptr + "/antecedents", "a rule has at least one antecedent"}
	}
	if len(rule.Consequents) == 0 {
		return &RenderError{ptr + "/consequents", "a rule has at least one consequent"}
	}
	for j := range rule.Antecedents {
		aptr := fmt.Sprintf("%s/antecedents/%d", ptr, j)
		text, err := renderAntecedent(&rule.Antecedents[j], aptr)
		if err != nil {
			return err
		}
		if j < len(rule.Antecedents)-1 {
			text += "."
		}
		r.line("  "+text, aptr)
	}
	r.line("  ->", "")
	for j := range rule.Consequents {
		cptr := fmt.Sprintf("%s/consequents/%d", ptr, j)
		text, err := renderConsequent(&rule.Consequents[j], cptr)
		if err != nil {
			return err
		}
		if j < len(rule.Consequents)-1 {
			text += "."
		}
		r.line("  "+text, cptr)
	}
	r.line(";", ptr)
	return nil
}

func renderAntecedent(a *Antecedent, ptr string) (string, error) {
	s, err := renderIDTerm(a.Subject, ptr+"/subject")
	if err != nil {
		return "", err
	}
	p, err := renderIDTerm(a.Predicate, ptr+"/predicate")
	if err != nil {
		return "", err
	}
	o, err := renderTerm(a.Object, ptr+"/object")
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf("(%s %s %s)", s, p, o)
	if a.Not {
		text = "not" + text
	}
	if a.Filter != nil {
		f, err := renderExpr(a.Filter, ptr+"/filter")
		if err != nil {
			return "", err
		}
		text += ".[" + f + "]"
	}
	return text, nil
}

func renderConsequent(c *Consequent, ptr string) (string, error) {
	s, err := renderIDTerm(c.Subject, ptr+"/subject")
	if err != nil {
		return "", err
	}
	p, err := renderIDTerm(c.Predicate, ptr+"/predicate")
	if err != nil {
		return "", err
	}
	o, err := renderExpr(&c.Object, ptr+"/object")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s %s %s)", s, p, o), nil
}

// renderIDTerm renders a subject or a predicate, which the grammar admits only
// as an id: a variable or a resource, never a literal.
func renderIDTerm(t Term, ptr string) (string, error) {
	if t.Literal != nil {
		return "", &RenderError{ptr, "a subject or predicate is an id (a variable or a resource), not a literal"}
	}
	return renderTerm(t, ptr)
}

func renderTerm(t Term, ptr string) (string, error) {
	switch {
	case t.Literal != nil && t.ID != "":
		return "", &RenderError{ptr, "a term is an id or a literal, not both"}
	case t.Literal != nil:
		text, err := renderLiteral(t.Literal)
		if err != nil {
			return "", &RenderError{ptr, err.Error()}
		}
		return text, nil
	default:
		text, err := renderID(t.ID)
		if err != nil {
			return "", &RenderError{ptr, err.Error()}
		}
		return text, nil
	}
}

// renderID renders an id as the grammar's atom.
//
//	?name              a variable
//	name               a declared resource or literal
//	prefix:name        a resource
//
// A local name the lexer would not read as an identifier — a reserved word,
// as in rdf:type's neighbour ex:date, or anything else — is written
// prefix:"name", which is the grammar's own escape and which the compiler
// reads back as prefix:name (EscR).
func renderID(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("an id cannot be empty")
	}
	if v, isVar := strings.CutPrefix(id, "?"); isVar {
		if !identifierRe.MatchString(v) || reserved[v] {
			return "", fmt.Errorf("variable %q must be '?' followed by an identifier that is not a reserved word", id)
		}
		return id, nil
	}
	prefix, local, hasPrefix := strings.Cut(id, ":")
	if !hasPrefix {
		if !identifierRe.MatchString(id) || reserved[id] {
			return "", fmt.Errorf("id %q is not an identifier, a prefixed name (prefix:name) or a variable (?name)", id)
		}
		return id, nil
	}
	if !identifierRe.MatchString(prefix) || reserved[prefix] {
		return "", fmt.Errorf("the prefix of %q must be an identifier that is not a reserved word", id)
	}
	if identifierRe.MatchString(local) && !reserved[local] {
		return id, nil
	}
	// The compiler only strips the escape from a name longer than four
	// characters (EscR), so a shorter escaped name would keep its quotes.
	if local == "" || strings.ContainsAny(local, "\"\\\r\n") || len(id) <= 4 {
		return "", fmt.Errorf("id %q has a local name that cannot be written in a rule file", id)
	}
	return fmt.Sprintf("%s:%q", prefix, local), nil
}

func renderLiteral(l *Literal) (string, error) {
	if !literalTypes[l.Type] {
		return "", fmt.Errorf("literal type %q is not one of int, uint, long, ulong, double, text, date, datetime, bool, keyword", l.Type)
	}
	switch l.Type {
	case "keyword":
		switch l.Value {
		case "true", "false", "null":
			return l.Value, nil
		}
		return "", fmt.Errorf("a keyword is true, false or null, got %q", l.Value)
	case "int", "long":
		if !intRe.MatchString(l.Value) {
			return "", fmt.Errorf("%s literal %q is not an integer", l.Type, l.Value)
		}
	case "uint", "ulong":
		if !uintRe.MatchString(l.Value) {
			return "", fmt.Errorf("%s literal %q is not an unsigned integer", l.Type, l.Value)
		}
	case "double":
		if !doubleRe.MatchString(l.Value) {
			return "", fmt.Errorf("double literal %q is not digits with an optional fraction", l.Value)
		}
	case "text":
		if err := checkStringBody(l.Value); err != nil {
			return "", fmt.Errorf("text literal: %w", err)
		}
		// The bare quoted form rather than text("..."): the compiler splits a
		// cast on "(", so text("a(b") would not read back (parseObjectAtom).
		return `"` + l.Value + `"`, nil
	default: // date, datetime, bool
		if err := checkStringBody(l.Value); err != nil {
			return "", fmt.Errorf("%s literal: %w", l.Type, err)
		}
		if strings.Contains(l.Value, "(") {
			return "", fmt.Errorf("%s literal %q cannot contain '(' (the compiler splits a cast on it)", l.Type, l.Value)
		}
		return fmt.Sprintf(`%s("%s")`, l.Type, l.Value), nil
	}
	return fmt.Sprintf("%s(%s)", l.Type, l.Value), nil
}

// checkStringBody checks that a value can sit between the double quotes of
// the grammar's STRING without ending it: a double quote must be escaped, and
// the escape is \" or \\. The value is the text the compiler stores, so it is
// written back verbatim rather than escaped a second time.
func checkStringBody(v string) error {
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '"':
			return fmt.Errorf("%q has an unescaped double quote; write it as \\\"", v)
		case '\\':
			if i+1 < len(v) && (v[i+1] == '"' || v[i+1] == '\\') {
				i++
			}
		}
	}
	if strings.HasSuffix(v, `\`) && !strings.HasSuffix(v, `\\`) {
		return fmt.Errorf("%q ends in a backslash, which would escape the closing quote", v)
	}
	return nil
}

func renderExpr(e *Expr, ptr string) (string, error) {
	switch {
	case e == nil:
		return "", &RenderError{ptr, "missing expression"}
	case e.Term != nil:
		if e.Op != "" || e.Lhs != nil || e.Rhs != nil || e.Arg != nil {
			return "", &RenderError{ptr, "an expression is a term or an operation, not both"}
		}
		return renderTerm(*e.Term, ptr)
	case e.Arg != nil:
		if e.Lhs != nil || e.Rhs != nil {
			return "", &RenderError{ptr, "an operation has 'arg' (unary) or 'lhs' and 'rhs' (binary), not both"}
		}
		if !unaryKeywordOps[e.Op] && (!identifierRe.MatchString(e.Op) || reserved[e.Op]) {
			return "", &RenderError{ptr + "/op", fmt.Sprintf("%q is not a unary operator: not, toText, or a named operator", e.Op)}
		}
		arg, err := renderExpr(e.Arg, ptr+"/arg")
		if err != nil {
			return "", err
		}
		// (op arg) and not op(arg): the grammar also admits a bare "op arg",
		// so "T lookup create_literal(x)" reads as the unary "T lookup"
		// applied by "create_literal" to x. The parenthesised form is the one
		// that delimits itself (UnaryExprTerm2), and the corpus round trip
		// is what found the other one ambiguous.
		return fmt.Sprintf("(%s %s)", e.Op, arg), nil
	default:
		if e.Lhs == nil || e.Rhs == nil {
			return "", &RenderError{ptr, "an operation has 'arg' (unary) or both 'lhs' and 'rhs' (binary)"}
		}
		if !binaryOps[e.Op] && (!identifierRe.MatchString(e.Op) || reserved[e.Op]) {
			return "", &RenderError{ptr + "/op", fmt.Sprintf("%q is not a binary operator", e.Op)}
		}
		lhs, err := renderExpr(e.Lhs, ptr+"/lhs")
		if err != nil {
			return "", err
		}
		rhs, err := renderExpr(e.Rhs, ptr+"/rhs")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", lhs, e.Op, rhs), nil
	}
}

func renderPropertyValue(v any) (string, error) {
	switch x := v.(type) {
	case string:
		if err := checkStringBody(x); err != nil {
			return "", err
		}
		return `"` + x + `"`, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case int:
		return strconv.Itoa(x), nil
	default:
		return "", fmt.Errorf("a property value is a string, a boolean or an integer, got %T", v)
	}
}

func checkImport(path string) error {
	switch {
	case path == "":
		return fmt.Errorf("an import names a file")
	case strings.ContainsAny(path, "\"\\\r\n"):
		return fmt.Errorf("import %q cannot contain a quote, a backslash or a line break", path)
	case !strings.HasSuffix(path, ".jr"):
		return fmt.Errorf("import %q must name a .jr file", path)
	}
	return nil
}

// escapePointer escapes a JSON Pointer reference token (RFC 6901).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
