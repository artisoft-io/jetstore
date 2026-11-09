package runbook

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/sqlscript"
)

// BoundStatement is one statement of a step's SQL, ready for pgx: SQL with its
// `:name` placeholders rewritten to `$1..$n`, Args in the same order, and
// Names[i] the placeholder name `$i+1` stands for. A name used twice in one
// statement is one parameter.
type BoundStatement struct {
	SQL   string
	Args  []any
	Names []string
}

// Bind splits sql into statements with sqlscript.Split, and binds each one's
// `:name` placeholders: a name is resolved by the step's `bind` map (name to
// ref), else as the parameter `params.<name>`, and its Value is looked up in
// env. A name with no value is an error, as is any text substitution
// (`$NAME`, `${...}`, `{{...}}`, or a positional `$1`) outside strings and
// comments: no value reaches SQL except as a bind parameter.
//
// Each statement's Body is bound (sqlscript.Statement: the statement without
// its leading comment block); a chunk that is all comment yields nothing.
//
// Values become pgx arguments as they are: string, int64, float64, bool,
// time.Time or nil. A list becomes a typed slice — []string, []int64,
// []float64, []bool or []time.Time, or a slice of pointers to those when an
// element is NULL — which pgx sends as a PostgreSQL array, for `= ANY(:name)`.
func Bind(sql string, step *Step, env Env) ([]BoundStatement, error) {
	stmts, err := sqlscript.Split(sql)
	if err != nil {
		return nil, err
	}
	var out []BoundStatement
	for _, st := range stmts {
		if st.Body == "" {
			continue
		}
		scan, err := scanSQL(st.Body)
		if err != nil {
			return nil, fmt.Errorf("statement on line %d: %v", st.BodyLine, err)
		}
		if len(scan.substs) > 0 {
			s := scan.substs[0]
			return nil, fmt.Errorf("statement on line %d: %s", st.BodyLine+s.line-1, s.message)
		}
		bs := BoundStatement{}
		index := map[string]int{}
		var b strings.Builder
		last := 0
		for _, n := range scan.names {
			i, seen := index[n.name]
			if !seen {
				v, err := lookupName(n.name, step, env)
				if err != nil {
					return nil, fmt.Errorf("statement on line %d: %v", st.BodyLine+n.line-1, err)
				}
				arg, err := pgArg(v)
				if err != nil {
					return nil, fmt.Errorf("statement on line %d: :%s: %v", st.BodyLine+n.line-1, n.name, err)
				}
				bs.Args = append(bs.Args, arg)
				bs.Names = append(bs.Names, n.name)
				i = len(bs.Args)
				index[n.name] = i
			}
			b.WriteString(st.Body[last:n.start])
			b.WriteString("$" + strconv.Itoa(i))
			last = n.end
		}
		b.WriteString(st.Body[last:])
		bs.SQL = b.String()
		out = append(out, bs)
	}
	return out, nil
}

// BindRef is the ref a `:name` in a step's SQL resolves to: the step's `bind`
// entry for it, else `params.<name>`.
func BindRef(step *Step, name string) string {
	if step != nil {
		if ref, ok := step.Bind[name]; ok {
			return ref
		}
	}
	return ParamRef(name)
}

func lookupName(name string, step *Step, env Env) (Value, error) {
	ref := BindRef(step, name)
	v, ok := env.Lookup(ref)
	if !ok {
		return Value{}, fmt.Errorf(":%s resolves to %s, which has no value", name, ref)
	}
	return v, nil
}

// pgArg converts a Value to what pgx binds.
func pgArg(v Value) (any, error) {
	if !v.Type.List {
		return coerceScalar(v.Type.Base, v.V)
	}
	elems := v.Elems()
	hasNull := false
	canon := make([]any, len(elems))
	for i, e := range elems {
		c, err := coerceScalar(v.Type.Base, e)
		if err != nil {
			return nil, fmt.Errorf("element %d: %v", i, err)
		}
		canon[i] = c
		hasNull = hasNull || c == nil
	}
	var elemType reflect.Type
	switch v.Type.Base {
	case TypeText, TypeSessionID:
		elemType = reflect.TypeFor[string]()
	case TypeInteger:
		elemType = reflect.TypeFor[int64]()
	case TypeNumeric:
		elemType = reflect.TypeFor[float64]()
	case TypeBoolean:
		elemType = reflect.TypeFor[bool]()
	case TypeTimestamp:
		elemType = reflect.TypeFor[time.Time]()
	default:
		return nil, fmt.Errorf("unknown type %q", v.Type.Base)
	}
	if hasNull {
		elemType = reflect.PointerTo(elemType)
	}
	out := reflect.MakeSlice(reflect.SliceOf(elemType), len(canon), len(canon))
	for i, c := range canon {
		if c == nil {
			continue
		}
		x := reflect.ValueOf(c)
		if hasNull {
			p := reflect.New(x.Type())
			p.Elem().Set(x)
			x = p
		}
		out.Index(i).Set(x)
	}
	return out.Interface(), nil
}

// The SQL scanner.
//
// **It mirrors sqlscript.Split's lexing rather than calling it**, because
// Split's scanners are unexported and Split answers a different question (where
// a statement ends); this one answers where a `:name` is. It recognises the
// same constructs — `--` and nested `/* */` comments, `'...'`, `E'...'`,
// `"..."`, `$tag$...$tag$`, and identifiers with `$` in them — and on top of
// them: `:name` outside all of those, `::` as a cast and not a name, and text
// substitution, which a runbook refuses.

type sqlName struct {
	name       string
	start, end int // byte offsets of `:name` in the scanned text
	line       int // 1-based
}

type sqlSubst struct {
	text    string
	line    int
	inQuote bool // inside a string literal: reported as a warning only
	message string
}

type sqlScan struct {
	names  []sqlName
	substs []sqlSubst // outside strings and comments
	quoted []sqlSubst // inside string literals
}

func scanSQL(s string) (sqlScan, error) {
	var out sqlScan
	n := len(s)
	line := 1
	i := 0
	for i < n {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == '-' && i+1 < n && s[i+1] == '-':
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				i = n
			} else {
				i += j
			}
		case c == '/' && i+1 < n && s[i+1] == '*':
			depth, open := 0, line
			for {
				if i >= n {
					return out, &sqlscript.UnterminatedError{Construct: "block comment", Line: open}
				}
				if i+1 < n && s[i] == '/' && s[i+1] == '*' {
					depth++
					i += 2
				} else if i+1 < n && s[i] == '*' && s[i+1] == '/' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
				} else {
					if s[i] == '\n' {
						line++
					}
					i++
				}
			}
		case c == '\'' || c == '"' || ((c == 'e' || c == 'E') && i+1 < n && s[i+1] == '\'' && !identCont(prev(s, i))):
			quote, start, backslash := c, i+1, false
			if c == 'e' || c == 'E' {
				quote, start, backslash = '\'', i+2, true
			}
			open := line
			j := start
			closed := false
			for j < n {
				d := s[j]
				if backslash && d == '\\' && j+1 < n {
					if s[j+1] == '\n' {
						line++
					}
					j += 2
					continue
				}
				if d == quote {
					if j+1 < n && s[j+1] == quote {
						j += 2
						continue
					}
					closed = true
					break
				}
				if d == '\n' {
					line++
				}
				j++
			}
			if !closed {
				kind := "string literal"
				if quote == '"' {
					kind = "quoted identifier"
				}
				return out, &sqlscript.UnterminatedError{Construct: kind, Line: open}
			}
			if quote == '\'' {
				for _, sub := range findSubsts(s[start:j], open) {
					sub.inQuote = true
					out.quoted = append(out.quoted, sub)
				}
			}
			i = j + 1
		case c == '$':
			if tag, ok := dollarOpen(s, i); ok {
				rest := s[i+len(tag):]
				j := strings.Index(rest, tag)
				if j < 0 {
					return out, &sqlscript.UnterminatedError{Construct: "dollar-quoted string " + tag, Line: line}
				}
				line += strings.Count(rest[:j+len(tag)], "\n")
				i += len(tag) + j + len(tag)
				continue
			}
			if sub, ok := substAt(s, i, line); ok {
				out.substs = append(out.substs, sub)
				i += len(sub.text)
				continue
			}
			i++
		case c == '{' && i+1 < n && s[i+1] == '{':
			sub, _ := substAt(s, i, line)
			out.substs = append(out.substs, sub)
			i += len(sub.text)
		case c == ':':
			if i+1 < n && s[i+1] == ':' {
				i += 2 // a cast
				continue
			}
			if i+1 < n && identStart(s[i+1]) {
				j := i + 1
				for j < n && (identStart(s[j]) || (s[j] >= '0' && s[j] <= '9')) {
					j++
				}
				out.names = append(out.names, sqlName{name: s[i+1 : j], start: i, end: j, line: line})
				i = j
				continue
			}
			i++
		case identStart(c):
			i++
			for i < n && identCont(s[i]) {
				i++
			}
		default:
			i++
		}
	}
	return out, nil
}

// findSubsts finds substitution markers in a string literal's text.
func findSubsts(s string, line int) []sqlSubst {
	var out []sqlSubst
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			line++
		case '$', '{':
			if sub, ok := substAt(s, i, line); ok {
				out = append(out, sub)
				i += len(sub.text) - 1
			}
		}
	}
	return out
}

// substAt reports a text-substitution marker at s[i]: `$NAME`, `${...}`,
// `{{...}}`, or a positional `$1`.
func substAt(s string, i, line int) (sqlSubst, bool) {
	n := len(s)
	switch {
	case s[i] == '{' && i+1 < n && s[i+1] == '{':
		end := strings.Index(s[i:], "}}")
		text := s[i:min(n, i+2)]
		if end >= 0 {
			text = s[i : i+end+2]
		}
		return sqlSubst{text: text, line: line, message: fmt.Sprintf(
			"%s is text substitution; a runbook binds values: write :name and declare it as a parameter or in \"bind\"", text)}, true
	case s[i] != '$' || i+1 >= n:
		return sqlSubst{}, false
	case s[i+1] == '{':
		end := strings.IndexByte(s[i:], '}')
		text := s[i : i+2]
		if end >= 0 {
			text = s[i : i+end+1]
		}
		return sqlSubst{text: text, line: line, message: fmt.Sprintf(
			"%s is text substitution; a runbook binds values: write :name and declare it as a parameter or in \"bind\"", text)}, true
	case identStart(s[i+1]):
		j := i + 1
		for j < n && (identStart(s[j]) || (s[j] >= '0' && s[j] <= '9')) {
			j++
		}
		text := s[i:j]
		return sqlSubst{text: text, line: line, message: fmt.Sprintf(
			"%s is text substitution, which a runbook does not do; write :%s and declare it as a parameter or in \"bind\"",
			text, strings.ToLower(text[1:]))}, true
	case s[i+1] >= '0' && s[i+1] <= '9':
		j := i + 1
		for j < n && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		text := s[i:j]
		return sqlSubst{text: text, line: line, message: fmt.Sprintf(
			"%s is a positional parameter; a runbook names its parameters (:name) and numbers them itself", text)}, true
	}
	return sqlSubst{}, false
}

// dollarOpen reports whether the `$` at i opens a dollar-quoted body, as
// sqlscript's dollarTag does: `$$` or `$tag$`.
func dollarOpen(s string, i int) (string, bool) {
	j, n := i+1, len(s)
	if j < n && identStart(s[j]) {
		j++
		for j < n && identCont(s[j]) && s[j] != '$' {
			j++
		}
	}
	if j < n && s[j] == '$' {
		return s[i : j+1], true
	}
	return "", false
}

func prev(s string, i int) byte {
	if i == 0 {
		return 0
	}
	return s[i-1]
}

func identStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func identCont(c byte) bool {
	return identStart(c) || (c >= '0' && c <= '9') || c == '$'
}
