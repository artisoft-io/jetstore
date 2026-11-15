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
// comments: no value reaches SQL except as a bind parameter. So is a statement
// no step may run (ForbiddenStatement), which Validate also refuses when a run
// starts; Bind refuses it again because a paused run reads its files anew.
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
		if why := ForbiddenStatement(st.Body); why != "" {
			return nil, fmt.Errorf("statement on line %d is refused: %s", st.BodyLine, why)
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
// It reads the text with sqlscript.Scanner, the lexing sqlscript.Split finds
// statement boundaries with, so that a `:name` is recognised outside exactly
// the constructs Split skips — `--` and nested `/* */` comments, `'...'`,
// `E'...'`, `"..."`, `$tag$...$tag$`, and identifiers with `$` in them — and
// adds what a runbook needs on top of them: `:name` among the other tokens,
// `::` as a cast and not a name, and text substitution, which a runbook
// refuses.

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
	sc := sqlscript.NewScanner(s)
	for {
		tok, ok, err := sc.Next()
		if err != nil {
			return out, err
		}
		if !ok {
			return out, nil
		}
		i, line := tok.Offset, tok.Line
		switch tok.Kind {
		case sqlscript.String, sqlscript.EscapeString:
			for _, sub := range findSubsts(tok.Inner(), line) {
				sub.inQuote = true
				out.quoted = append(out.quoted, sub)
			}
		case sqlscript.Other:
			switch c := tok.Text[0]; {
			case c == '$' || (c == '{' && i+1 < n && s[i+1] == '{'):
				// A `$` here does not open a dollar-quoted body: the scanner
				// would have read one.
				if sub, ok := substAt(s, i, line); ok {
					out.substs = append(out.substs, sub)
					sc.Skip(len(sub.text) - 1)
				}
			case c == ':' && i+1 < n && s[i+1] == ':':
				sc.Skip(1) // a cast
			case c == ':' && i+1 < n && sqlscript.IsIdentStart(s[i+1]):
				j := i + 1
				for j < n && (sqlscript.IsIdentStart(s[j]) || (s[j] >= '0' && s[j] <= '9')) {
					j++
				}
				out.names = append(out.names, sqlName{name: s[i+1 : j], start: i, end: j, line: line})
				sc.Skip(j - i - 1)
			}
		}
	}
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
	case sqlscript.IsIdentStart(s[i+1]):
		j := i + 1
		for j < n && (sqlscript.IsIdentStart(s[j]) || (s[j] >= '0' && s[j] <= '9')) {
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
