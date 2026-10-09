// F.2b: harvest the compute_pipes test fixtures into library parts.
//
// A§6.1 source 2 proposes the test fixtures as a source for the fragment library:
// author-written, correct by construction, and concentrated where the live corpus is
// thin. I-35 measured the catch: of 171 `*Spec` composite literals with a plain type
// name, 94 are self-contained and 77 reference test scope, and the scope-dependent ones
// are the tail. Reading them as values means resolving that scope.
//
// **It is a resolver, not an evaluator.** I-35 left the choice open between a static
// resolver and a test-time capture. The resolver wins because the references that
// matter are simple: `mergeColumn := []string{"key1"}` a few lines above the literal.
// Nothing here runs a test, links the package, or depends on `go/types` - it reads
// declarations and substitutes literals, and reports anything it cannot.
//
// **What it will not do is guess.** A reference it cannot resolve to a literal makes the
// whole fixture unharvested and counted, rather than harvested with a hole in it. A
// fixture with an invented field is worse than a missing one: the library is few-shot
// material, and a wrong example teaches the wrong thing.
//
//	go run ./tools/cpipes_contract/fixtures -pkg jets/compute_pipes
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type field struct {
	jsonName string
	omit     bool
	embedded string // the struct this field embeds, if it is an embedded field
	exported bool
}

type harvester struct {
	fset    *token.FileSet
	structs map[string]map[string]field  // struct -> Go field -> json name
	decls   map[string]map[string]string // struct -> Go field -> declared type name
	global  map[string]ast.Expr          // package-level literal bindings
	unres   map[string][]string          // type -> reasons it could not be read
	// Names currently being resolved. `rows := append(rows, x)` is one assignment, so
	// the rebound check does not see it, and resolving `rows` would recurse forever.
	resolving map[string]bool
	root      string
}

type part struct {
	DefsName string `json:"defs_name"`
	Value    any    `json:"value"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Func     string `json:"func"`
}

func main() {
	pkg := flag.String("pkg", "jets/compute_pipes", "package directory to harvest")
	out := flag.String("out", "", "write parts as JSONL here (default: stdout report only)")
	flag.Parse()

	h := &harvester{fset: token.NewFileSet(), structs: map[string]map[string]field{},
		decls: map[string]map[string]string{}, global: map[string]ast.Expr{},
		unres: map[string][]string{}, resolving: map[string]bool{}, root: *pkg}
	files, _ := filepath.Glob(filepath.Join(*pkg, "*.go"))
	sort.Strings(files)

	var tests []*ast.File
	for _, f := range files {
		af, err := parser.ParseFile(h.fset, f, nil, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", f, err)
			continue
		}
		h.collectStructs(af)
		h.collectGlobals(af)
		if strings.HasSuffix(f, "_test.go") {
			tests = append(tests, af)
		}
	}

	var parts []part
	for _, af := range tests {
		parts = append(parts, h.harvestFile(af)...)
	}
	h.report(parts)

	if *out != "" {
		fh, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer fh.Close()
		enc := json.NewEncoder(fh)
		for _, p := range parts {
			if err := enc.Encode(p); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		fmt.Printf("\nwrote %d part(s) to %s\n", len(parts), *out)
	}
}

// collectStructs records each struct's Go field -> JSON name mapping, which is the only
// way a fixture's field names become the contract's field names. `pipes_model.go` is the
// single source, so a field without a tag is a field the contract does not have.
func (h *harvester) collectStructs(af *ast.File) {
	ast.Inspect(af, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		m := map[string]field{}
		d := map[string]string{}
		for _, f := range st.Fields.List {
			for _, id := range f.Names {
				d[id.Name] = typeName(stripSeq(f.Type))
			}
			name, omit := "", false
			if f.Tag != nil {
				tag, _ := strconv.Unquote(f.Tag.Value)
				if j := structTag(tag, "json"); j != "" {
					bits := strings.Split(j, ",")
					name = bits[0]
					for _, b := range bits[1:] {
						if b == "omitempty" || b == "omitzero" {
							omit = true
						}
					}
				}
			}
			if len(f.Names) == 0 {
				// An embedded struct. Go's encoding/json promotes its fields into the
				// parent unless the tag names it, so the fixture writes
				// `OllamaSpec{InferCommonSpec: InferCommonSpec{...}}` and the config
				// carries those fields flat.
				if e := typeName(f.Type); e != "" {
					m[e] = field{jsonName: name, embedded: e, exported: true}
				}
				continue
			}
			for _, id := range f.Names {
				m[id.Name] = field{jsonName: name, omit: omit,
					exported: id.Name != "" && id.Name[0] >= 'A' && id.Name[0] <= 'Z'}
			}
		}
		h.structs[ts.Name.Name] = m
		h.decls[ts.Name.Name] = d
		return true
	})
}

func structTag(tag, key string) string {
	for _, kv := range strings.Fields(tag) {
		if strings.HasPrefix(kv, key+":\"") {
			v, err := strconv.Unquote(kv[len(key)+1:])
			if err == nil {
				return v
			}
		}
	}
	return ""
}

func (h *harvester) collectGlobals(af *ast.File) {
	for _, d := range af.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range gd.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok || len(vs.Values) != len(vs.Names) {
				continue
			}
			for i, n := range vs.Names {
				h.global[n.Name] = vs.Values[i]
			}
		}
	}
}

// harvestFile walks each test function with its own local bindings, so a name assigned
// in two functions resolves to the right one in each.
func (h *harvester) harvestFile(af *ast.File) []part {
	var parts []part
	for _, d := range af.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		locals := map[string]ast.Expr{}
		// Collected over the whole body rather than up to the literal: a test that
		// rebinds a name mid-function would be resolved wrongly, so that case is
		// detected and refused rather than silently taking the last one.
		rebound := map[string]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.AssignStmt:
				for i, lhs := range v.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || i >= len(v.Rhs) {
						continue
					}
					if _, seen := locals[id.Name]; seen {
						rebound[id.Name] = true
					}
					locals[id.Name] = v.Rhs[i]
				}
			case *ast.RangeStmt:
				// A loop variable has no single value; mark it unresolvable.
				for _, e := range []ast.Expr{v.Key, v.Value} {
					if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
						rebound[id.Name] = true
					}
				}
			}
			return true
		})

		ast.Inspect(fd.Body, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			id, ok := cl.Type.(*ast.Ident)
			if !ok || !strings.HasSuffix(id.Name, "Spec") {
				return true
			}
			if _, known := h.structs[id.Name]; !known {
				h.unres[id.Name] = append(h.unres[id.Name], "no struct definition")
				return true
			}
			v, err := h.value(cl, locals, rebound)
			if err != nil {
				h.unres[id.Name] = append(h.unres[id.Name], err.Error())
				return true
			}
			pos := h.fset.Position(cl.Pos())
			parts = append(parts, part{DefsName: id.Name, Value: v,
				File: filepath.ToSlash(pos.Filename), Line: pos.Line, Func: fd.Name.Name})
			return true
		})
	}
	return parts
}

type unresolved struct{ what string }

func (u unresolved) Error() string { return u.what }

// value turns one expression into JSON, or says why it cannot.
//
// **Every branch either produces a literal or fails.** There is no default that returns
// something plausible - an expression this does not understand is an unharvested
// fixture, which is a number in the report, rather than a part with a guess in it.
func (h *harvester) value(e ast.Expr, locals map[string]ast.Expr, rebound map[string]bool) (any, error) {
	switch v := e.(type) {
	case *ast.BasicLit:
		return basic(v)

	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return h.value(v.X, locals, rebound) // &T{...} is T{...} once serialised
		}
		if v.Op == token.SUB {
			inner, err := h.value(v.X, locals, rebound)
			if err != nil {
				return nil, err
			}
			switch n := inner.(type) {
			case int64:
				return -n, nil
			case float64:
				return -n, nil
			}
		}
		return nil, unresolved{"unary " + v.Op.String()}

	case *ast.ParenExpr:
		return h.value(v.X, locals, rebound)

	case *ast.Ident:
		switch v.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil":
			return nil, nil
		}
		if rebound[v.Name] {
			return nil, unresolved{"reads " + v.Name + ", which is assigned more than once"}
		}
		if h.resolving[v.Name] {
			return nil, unresolved{"reads " + v.Name + ", which is defined in terms of itself"}
		}
		bound, ok := locals[v.Name]
		if !ok {
			bound, ok = h.global[v.Name]
		}
		if ok {
			h.resolving[v.Name] = true
			out, err := h.value(bound, locals, rebound)
			delete(h.resolving, v.Name)
			return out, err
		}
		return nil, unresolved{"reads " + v.Name + ", which is not bound to a literal"}

	case *ast.CallExpr:
		// A conversion carries its argument through; a helper call does not.
		if id, ok := v.Fun.(*ast.Ident); ok && isConversion(id.Name) && len(v.Args) == 1 {
			return h.value(v.Args[0], locals, rebound)
		}
		// `append(x, y...)` of literals is still a literal; the tests use it to build
		// a variant of a shared fixture, which is exactly the material worth having.
		if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "append" {
			out := []any{}
			for i, a := range v.Args {
				av, err := h.value(a, locals, rebound)
				if err != nil {
					return nil, err
				}
				if i == 0 || v.Ellipsis.IsValid() && i == len(v.Args)-1 {
					if seq, ok := av.([]any); ok {
						out = append(out, seq...)
						continue
					}
				}
				out = append(out, av)
			}
			return out, nil
		}
		return nil, unresolved{"calls " + exprText(v.Fun) + "()"}

	case *ast.SelectorExpr:
		return nil, unresolved{"reads " + exprText(v)}

	case *ast.CompositeLit:
		return h.composite(v, locals, rebound)
	}
	return nil, unresolved{fmt.Sprintf("%T", e)}
}

func (h *harvester) composite(cl *ast.CompositeLit, locals map[string]ast.Expr, rebound map[string]bool) (any, error) {
	// A struct literal, if we can name the struct. An element of `[]*GroupBySpec{{...}}`
	// has no type of its own, so the element type is carried down from the slice.
	if name := typeName(cl.Type); name != "" {
		if fields, ok := h.structs[name]; ok {
			out := map[string]any{}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					return nil, unresolved{name + " uses positional fields"}
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					return nil, unresolved{name + " has a non-name field key"}
				}
				f, ok := fields[key.Name]
				if !ok {
					return nil, unresolved{name + " has no field " + key.Name}
				}
				val, err := h.valueAs(kv.Value, h.fieldType(name, key.Name), locals, rebound)
				if err != nil {
					return nil, err
				}
				if f.embedded != "" && f.jsonName == "" {
					// Promoted, as encoding/json would.
					inner, ok := val.(map[string]any)
					if !ok {
						return nil, unresolved{name + " embeds a non-object"}
					}
					for k, v := range inner {
						out[k] = v
					}
					continue
				}
				if f.jsonName == "" || f.jsonName == "-" {
					// An unexported or untagged field is runtime state the config does
					// not carry, and encoding/json drops it too - so dropping it here
					// is agreement with the contract rather than a guess. An *exported*
					// untagged field would be a real gap, and is refused.
					if f.exported {
						return nil, unresolved{name + "." + key.Name + " is exported but untagged"}
					}
					continue
				}
				out[f.jsonName] = val
			}
			return out, nil
		}
	}

	// A slice, array or map literal.
	switch t := cl.Type.(type) {
	case *ast.MapType:
		out := map[string]any{}
		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				return nil, unresolved{"map element is not a pair"}
			}
			k, err := h.value(kv.Key, locals, rebound)
			if err != nil {
				return nil, err
			}
			ks, ok := k.(string)
			if !ok {
				return nil, unresolved{"map key is not a string"}
			}
			// `map[string]*DomainKeyInfo{"Claim": {...}}` writes its values untyped,
			// so the map's own value type is what names them.
			val, err := h.valueAs(kv.Value, typeName(stripSeq(t.Value)), locals, rebound)
			if err != nil {
				return nil, err
			}
			out[ks] = val
		}
		return out, nil
	case *ast.ArrayType:
		return h.elements(cl, elementType(t.Elt), locals, rebound)
	case nil:
		return h.elements(cl, "", locals, rebound)
	}
	return nil, unresolved{"composite of " + exprText(cl.Type)}
}

// elements walks a slice literal, carrying the element type down to untyped elements -
// `[]*GroupBySpec{{GroupByName: x}}` writes its elements without repeating the type.
func (h *harvester) elements(cl *ast.CompositeLit, elem string, locals map[string]ast.Expr, rebound map[string]bool) (any, error) {
	out := []any{}
	for _, e := range cl.Elts {
		if inner, ok := e.(*ast.CompositeLit); ok && inner.Type == nil && elem != "" {
			copied := *inner
			copied.Type = ast.NewIdent(elem)
			v, err := h.composite(&copied, locals, rebound)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			continue
		}
		v, err := h.value(e, locals, rebound)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func basic(v *ast.BasicLit) (any, error) {
	switch v.Kind {
	case token.STRING:
		s, err := strconv.Unquote(v.Value)
		if err != nil {
			return nil, unresolved{"unquotable string"}
		}
		return s, nil
	case token.INT:
		n, err := strconv.ParseInt(v.Value, 0, 64)
		if err != nil {
			return nil, unresolved{"unparsable int"}
		}
		return n, nil
	case token.FLOAT:
		f, err := strconv.ParseFloat(v.Value, 64)
		if err != nil {
			return nil, unresolved{"unparsable float"}
		}
		return f, nil
	}
	return nil, unresolved{"literal of kind " + v.Kind.String()}
}

// fieldType names the declared type of one struct field, so an untyped composite in
// that position knows what it is.
func (h *harvester) fieldType(structName, fieldName string) string {
	decl, ok := h.decls[structName]
	if !ok {
		return ""
	}
	return decl[fieldName]
}

func (h *harvester) valueAs(e ast.Expr, want string, locals map[string]ast.Expr, rebound map[string]bool) (any, error) {
	if cl, ok := e.(*ast.CompositeLit); ok && cl.Type == nil && want != "" {
		copied := *cl
		copied.Type = ast.NewIdent(want)
		return h.composite(&copied, locals, rebound)
	}
	return h.value(e, locals, rebound)
}

func typeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return typeName(v.X)
	}
	return ""
}

func elementType(e ast.Expr) string { return typeName(e) }

// stripSeq reduces `[]*T` and `map[K]*T` to `T`, which is what an untyped element of
// such a field is.
func stripSeq(e ast.Expr) ast.Expr {
	switch v := e.(type) {
	case *ast.ArrayType:
		return stripSeq(v.Elt)
	case *ast.MapType:
		return stripSeq(v.Value)
	case *ast.StarExpr:
		return stripSeq(v.X)
	}
	return e
}

func isConversion(n string) bool {
	switch n {
	case "string", "int", "int32", "int64", "float64", "float32", "byte", "rune", "bool", "uint", "uint64":
		return true
	}
	return false
}

func exprText(n ast.Node) string {
	switch v := n.(type) {
	case *ast.CallExpr:
		return exprText(v.Fun)
	case *ast.SelectorExpr:
		return exprText(v.X) + "." + v.Sel.Name
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprText(v.X)
	case *ast.ArrayType:
		return "[]" + exprText(v.Elt)
	}
	return "?"
}

func (h *harvester) report(parts []part) {
	got := map[string]int{}
	for _, p := range parts {
		got[p.DefsName]++
	}
	types := map[string]bool{}
	for t := range got {
		types[t] = true
	}
	for t := range h.unres {
		types[t] = true
	}
	names := []string{}
	for t := range types {
		names = append(names, t)
	}
	sort.Slice(names, func(i, j int) bool {
		a := got[names[i]] + len(h.unres[names[i]])
		b := got[names[j]] + len(h.unres[names[j]])
		if a != b {
			return a > b
		}
		return names[i] < names[j]
	})
	fmt.Printf("%-28s %9s %6s   %s\n", "type", "harvested", "missed", "commonest reason")
	var harvested, missed int
	for _, t := range names {
		harvested += got[t]
		missed += len(h.unres[t])
		reason := ""
		if len(h.unres[t]) > 0 {
			counts := map[string]int{}
			for _, r := range h.unres[t] {
				counts[r]++
			}
			best := 0
			for r, n := range counts {
				if n > best {
					best, reason = n, r
				}
			}
			if len(reason) > 46 {
				reason = reason[:46]
			}
		}
		fmt.Printf("%-28s %9d %6d   %s\n", t, got[t], len(h.unres[t]), reason)
	}
	fmt.Printf("%-28s %9d %6d\n", "TOTAL", harvested, missed)
}
