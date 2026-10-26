package harness

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// InputTable is one CSV of an input session: a header naming data properties
// of the workspace, in any order, and rows of text cells. It is an entity-table
// export rather than a pipeline's input file (definition F26), so it is read by
// header and each row is asserted through compute_pipes.AssertTextRow, which
// casts every cell by the property's type in build/properties.json.
//
// A column named _0:<name> is a rule-local property, which a pipeline's input
// never carries and properties.json does not declare (I-20). Its cells are
// text unless the header gives a type, "_0:units as int", and they are
// asserted on the row's jets:key, which a table with one must supply.
type InputTable struct {
	File    string // as named in the test definition
	Columns []string
	Rows    [][]string
	Lines   []int // the line of each row in the file, for messages
	// Local are the _0: columns, in header order. Columns names each of them
	// as the header does, so Rows keeps one cell per header column; the cast
	// of the data properties skips them (dataCells).
	Local []LocalColumn
}

// LocalColumn is a _0: column of an input table.
type LocalColumn struct {
	Index int    // in Columns and in each row
	Name  string // the property, _0:<name>
	Type  string // a literal type of the grammar; text when the header names none
}

// localPrefix marks a rule-local resource: the compiler adds one for a name
// no declaration names (autoAddResources), and it is never a data property.
const localPrefix = "_0:"

// localTypes are the literal types a _0: column may name, the grammar's.
var localTypes = map[string]bool{
	"int": true, "uint": true, "long": true, "ulong": true, "double": true,
	"text": true, "date": true, "datetime": true, "bool": true,
}

// parseLocalColumn reads "_0:<name>" or "_0:<name> as <type>".
func parseLocalColumn(file string, index int, header string) (LocalColumn, error) {
	f := strings.Fields(header)
	switch {
	case len(f) == 1:
		return LocalColumn{Index: index, Name: f[0], Type: "text"}, nil
	case len(f) == 3 && f[1] == "as" && localTypes[f[2]]:
		return LocalColumn{Index: index, Name: f[0], Type: f[2]}, nil
	}
	return LocalColumn{}, fmt.Errorf(
		"input file %s: column '%s' is a rule-local property; write '%s<name>' or '%s<name> as <type>', the type one of int, uint, long, ulong, double, text, date, datetime, bool",
		file, header, localPrefix, localPrefix)
}

// ReadInputTable reads a CSV and checks its header against the workspace's
// data properties: a column that is not one is an error naming the file, the
// column and the closest property name (definition §3.3). Blank lines are
// skipped; every row must have one cell per column.
func ReadInputTable(path, name string) (*InputTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("input file %s: %w", name, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("input file %s is empty; it needs a header naming the data properties", name)
		}
		return nil, fmt.Errorf("input file %s: %w", name, err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}
	t := &InputTable{File: name, Columns: header}
	var dataColumns []string
	for i, c := range header {
		if !strings.HasPrefix(c, localPrefix) {
			dataColumns = append(dataColumns, c)
			continue
		}
		lc, err := parseLocalColumn(name, i, c)
		if err != nil {
			return nil, err
		}
		t.Local = append(t.Local, lc)
	}
	if err := checkColumns(name, header, dataColumns); err != nil {
		return nil, err
	}
	if len(t.Local) > 0 && !slices.Contains(header, "jets:key") {
		return nil, fmt.Errorf("input file %s: a %s column is asserted on the row's jets:key, and the file has no jets:key column",
			name, localPrefix)
	}
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("input file %s: %w", name, err)
		}
		line, _ := r.FieldPos(0)
		t.Rows = append(t.Rows, row)
		t.Lines = append(t.Lines, line)
	}
	return t, nil
}

// checkColumns checks every header column for a name and for being named once,
// and each of dataColumns for being a data property of the workspace.
func checkColumns(name string, columns, dataColumns []string) error {
	dpMap, err := compute_pipes.GetWorkspaceDataProperties()
	if err != nil {
		return fmt.Errorf("while reading the workspace's data properties: %w", err)
	}
	seen := make(map[string]bool, len(columns))
	for i, c := range columns {
		if c == "" {
			return fmt.Errorf("input file %s: column %d has no name", name, i+1)
		}
		// a _0: column is named by its property, whatever type it gives
		property := strings.Fields(c)[0]
		if seen[property] {
			return fmt.Errorf("input file %s: column '%s' appears twice", name, property)
		}
		seen[property] = true
	}
	var known []string
	for _, c := range dataColumns {
		if dpMap[c] != nil {
			continue
		}
		if known == nil {
			known = make([]string, 0, len(dpMap))
			for k := range dpMap {
				known = append(known, k)
			}
			sort.Strings(known)
		}
		if best := rulestest.ClosestName(c, known); best != "" {
			return fmt.Errorf("input file %s: column '%s' is not a data property of the workspace; did you mean '%s'?", name, c, best)
		}
		return fmt.Errorf("input file %s: column '%s' is not a data property of the workspace", name, c)
	}
	return nil
}

// assertTables asserts every row of every table into rdfSession. It is the
// assertInput argument of RunRuleSets, which calls it once the first rete
// session exists, the pipeline's order. Until 2026-10-11 C++ needed that order:
// the native session made its nodes through the meta graph before a rete
// session existed (I-15).
func assertTables(spec *compute_pipes.JetrulesSpec, tables []*InputTable, isDebug bool) func(compute_pipes.JetRdfSession) error {
	return func(rdfSession compute_pipes.JetRdfSession) error {
		for _, t := range tables {
			for i, row := range t.Rows {
				if isDebug {
					log.Printf("rules_unit_test: asserting %s line %d: %s", t.File, t.Lines[i], zipRow(t.Columns, row))
				}
				columns, cells := t.dataCells(row)
				if err := compute_pipes.AssertTextRow(spec, rdfSession, columns, cells); err != nil {
					return fmt.Errorf("input file %s line %d: %w", t.File, t.Lines[i], err)
				}
				if err := t.assertLocal(rdfSession, row); err != nil {
					return fmt.Errorf("input file %s line %d: %w", t.File, t.Lines[i], err)
				}
			}
		}
		return nil
	}
}

// dataCells is the row without its _0: cells, with the columns that name them.
func (t *InputTable) dataCells(row []string) (columns, cells []string) {
	if len(t.Local) == 0 {
		return t.Columns, row
	}
	local := make(map[int]bool, len(t.Local))
	for _, lc := range t.Local {
		local[lc.Index] = true
	}
	for i, c := range t.Columns {
		if !local[i] {
			columns = append(columns, c)
			if i < len(row) {
				cells = append(cells, row[i])
			}
		}
	}
	return columns, cells
}

// assertLocal asserts the row's _0: cells on its jets:key, each cast by its
// column's type as a data property's cell is cast by the property's
// (castToRdfTypeFromTxt, through CastToRdfTxtFnc). A blank cell asserts
// nothing, and so does an empty array literal, {}; an array literal asserts one
// triple per value.
func (t *InputTable) assertLocal(rdfSession compute_pipes.JetRdfSession, row []string) error {
	if len(t.Local) == 0 {
		return nil
	}
	keyAt := slices.Index(t.Columns, "jets:key")
	key := strings.TrimSpace(row[keyAt])
	rm := rdfSession.GetResourceManager()
	for _, lc := range t.Local {
		cell := row[lc.Index]
		if strings.TrimSpace(cell) == "" {
			continue
		}
		if key == "" {
			return fmt.Errorf("column '%s' has a value and jets:key is blank", lc.Name)
		}
		value, err := compute_pipes.NewCastToRdfTxtFnc(lc.Name, lc.Type, nil).Cast(cell)
		if err != nil {
			return fmt.Errorf("column '%s' value '%s' is not a %s: %v", lc.Name, cell, lc.Type, err)
		}
		// {} casts to nil, and asserts nothing, as a data property's {} does
		if value == nil {
			continue
		}
		values, isArray := value.([]any)
		if !isArray {
			values = []any{value}
		}
		subject, predicate := rm.NewResource(key), rm.NewResource(lc.Name)
		for _, v := range values {
			node, err := compute_pipes.NewRdfNode(v, rm)
			if err != nil {
				return fmt.Errorf("column '%s': %v", lc.Name, err)
			}
			if err := rdfSession.Insert(subject, predicate, node); err != nil {
				return err
			}
		}
	}
	return nil
}

func zipRow(columns, cells []string) string {
	var b strings.Builder
	for i, c := range columns {
		if i < len(cells) && cells[i] != "" {
			fmt.Fprintf(&b, "%s=%q ", c, cells[i])
		}
	}
	return strings.TrimSpace(b.String())
}
