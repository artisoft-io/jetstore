package harness

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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
type InputTable struct {
	File    string // as named in the test definition
	Columns []string
	Rows    [][]string
	Lines   []int // the line of each row in the file, for messages
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
	if err := checkColumns(name, header); err != nil {
		return nil, err
	}
	t := &InputTable{File: name, Columns: header}
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

func checkColumns(name string, columns []string) error {
	dpMap, err := compute_pipes.GetWorkspaceDataProperties()
	if err != nil {
		return fmt.Errorf("while reading the workspace's data properties: %w", err)
	}
	seen := make(map[string]bool, len(columns))
	var known []string
	for i, c := range columns {
		switch {
		case c == "":
			return fmt.Errorf("input file %s: column %d has no name", name, i+1)
		case seen[c]:
			return fmt.Errorf("input file %s: column '%s' appears twice", name, c)
		}
		seen[c] = true
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
				if err := compute_pipes.AssertTextRow(spec, rdfSession, t.Columns, row); err != nil {
					return fmt.Errorf("input file %s line %d: %w", t.File, t.Lines[i], err)
				}
			}
		}
		return nil
	}
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
