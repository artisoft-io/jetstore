// The B.18 field-inventory reflector: walks the compute_pipes struct graph
// from ComputePipesConfig and emits every struct's field inventory - Go name,
// json key, type string - as JSON on stdout. The python side
// (`python -m cpipes_contract drift`) compares it against the matrix's
// Go-binding columns and fails on anything present in one and absent from the
// other. It does not check applicability - only the matrix knows that.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	cp "github.com/artisoft-io/jetstore/jets/compute_pipes"
)

type fieldOut struct {
	Name string `json:"name"`
	JSON string `json:"json"`
	Type string `json:"type"`
}

var inventory = map[string][]fieldOut{}

func typeName(t reflect.Type) string {
	return strings.ReplaceAll(t.String(), "compute_pipes.", "")
}

// inScope says whether a struct the walk reached is one the matrix is expected
// to carry rows for.
//
// **It is two packages rather than one, and the second arrived with the render
// operator.** `TextTemplateSpec.Elements` is `[]*template.Element` -- the
// notation's own type, declared in `jets/agentic/template` so that the engine
// and the configuration share one definition rather than two that drift. The
// matrix carries `Element/*` because `check` refuses a `ref_struct` with no
// types row, so a reflector scoped to `compute_pipes` would report that row as
// unreachable and the drift check would be red for a row that is correct. The
// alternative -- excluding out-of-package structs on the python side -- carves
// an exception where this carves a scope, and leaves the notation's field
// inventory guarded by nothing.
func inScope(t reflect.Type) bool {
	pkg := t.PkgPath()
	return strings.Contains(pkg, "compute_pipes") || strings.Contains(pkg, "jets/agentic/template")
}

func namedStructs(t reflect.Type, out map[reflect.Type]bool) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		namedStructs(t.Elem(), out)
	case reflect.Struct:
		if t.Name() != "" && inScope(t) {
			out[t] = true
		}
	}
}

func walk(t reflect.Type, seen map[reflect.Type]bool) {
	if seen[t] {
		return
	}
	seen[t] = true
	var fields []fieldOut
	for _, f := range reflect.VisibleFields(t) {
		if f.Anonymous || f.PkgPath != "" {
			continue // the embedded struct itself, or unexported
		}
		key := f.Name
		if tag, ok := f.Tag.Lookup("json"); ok {
			name, _, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			if name != "" {
				key = name
			}
		}
		fields = append(fields, fieldOut{Name: f.Name, JSON: key, Type: typeName(f.Type)})
		children := map[reflect.Type]bool{}
		namedStructs(f.Type, children)
		for child := range children {
			walk(child, seen)
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].JSON < fields[j].JSON })
	inventory[t.Name()] = fields
}

func main() {
	seen := map[reflect.Type]bool{}
	walk(reflect.TypeOf(cp.ComputePipesConfig{}), seen)
	out, err := json.Marshal(inventory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Stdout.Write(out)
}
