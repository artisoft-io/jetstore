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

func namedStructs(t reflect.Type, out map[reflect.Type]bool) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		namedStructs(t.Elem(), out)
	case reflect.Struct:
		if t.Name() != "" && strings.Contains(t.PkgPath(), "compute_pipes") {
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
