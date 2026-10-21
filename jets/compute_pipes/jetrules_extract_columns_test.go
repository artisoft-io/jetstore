package compute_pipes_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// Extraction reads a subject's columns in one pass over its triples (rules_benchmark
// G11). This runs that pass and the extraction it replaced, one FindSP per column, on
// the same Go session and requires the same rows, cell for cell and in the same order.
//
// The session holds what test_looping_main.jr infers, three lp:Person with inferred
// lp:name values, and, asserted on top: more lp:name values, so one column's values
// come from two graphs; twenty xr:tag values; two ints on a single-valued column
// (set to null) and two texts on another (kept as an array); and a child entity
// for the JSON-encoded column. The columns add a duplicate, one with no value,
// rdf:type and jets:source_period_sequence.
func TestExtractionPassMatchesOneFindSPPerColumn(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		engine, s := goSession(t, loopingRuleFile)
		if o := compute_pipes.RunRuleSets(s, engine, &compute_pipes.RuleSetsConfig{}, assertCase("ok")); o.Err != nil {
			t.Fatal(o.Err)
		}
		rm := s.GetResourceManager()
		res := rm.NewResource
		var persons []compute_pipes.RdfNode
		itor := s.FindSPO(nil, s.JetResources().Rdf__type, res("lp:Person"))
		for !itor.IsEnd() {
			persons = append(persons, itor.GetSubject())
			itor.Next()
		}
		itor.Release()
		if len(persons) != 3 {
			t.Fatalf("%d lp:Person inferred, want 3", len(persons))
		}
		insert := func(subject compute_pipes.RdfNode, p string, o compute_pipes.RdfNode) {
			t.Helper()
			if err := s.Insert(subject, res(p), o); err != nil {
				t.Fatal(err)
			}
		}
		for i, p := range persons {
			for k := range 3 {
				insert(p, "lp:name", rm.NewTextLiteral(fmt.Sprintf("asserted_%d_%d", i, k)))
			}
			for k := range 20 {
				insert(p, "xr:tag", rm.NewTextLiteral(fmt.Sprintf("tag_%02d", k)))
			}
			insert(p, "xr:count", rm.NewIntLiteral(1))
			insert(p, "xr:count", rm.NewIntLiteral(2))
			insert(p, "xr:note", rm.NewTextLiteral("a"))
			insert(p, "xr:note", rm.NewTextLiteral("b"))
			child := res(fmt.Sprintf("child_%d", i))
			insert(child, "xr:label", rm.NewTextLiteral("child"))
			insert(p, "xr:child", child)
		}

		columns := []string{"jets:key", "rdf:type", "lp:name", "xr:tag", "lp:name", "xr:count", "xr:note",
			"xr:absent", "jets:source_period_sequence", "xr:as_json"}
		pass, each, keyed := compute_pipes.ExtractRowsBothWaysForTest(s, "lp:Person", columns,
			map[string]bool{"lp:name": true, "xr:tag": true},
			map[string]string{"xr:count": "int", "xr:note": "text"},
			map[string]*compute_pipes.JrSpecialColumnEncoding{
				"xr:as_json": {Config: &compute_pipes.ColumnEncodingSpec{Column: "xr:as_json", EntityEncoding: "json"}},
			})
		if !keyed {
			t.Fatalf("the Go engine's nodes have no identity key: the pass was not used")
		}
		if !reflect.DeepEqual(pass, each) {
			for i := range pass {
				for c := range columns {
					if !reflect.DeepEqual(pass[i][c], each[i][c]) {
						t.Errorf("row %d column %s: pass %#v, one FindSP per column %#v", i, columns[c], pass[i][c], each[i][c])
					}
				}
			}
			t.FailNow()
		}

		// And the rows are what the columns ask for, so the comparison is not of two
		// empty extractions.
		hasPrefix := func(list any, prefix string) bool {
			l, _ := list.([]any)
			return slices.ContainsFunc(l, func(v any) bool {
				s, _ := v.(string)
				return len(s) >= len(prefix) && s[:len(prefix)] == prefix
			})
		}
		for i, row := range pass {
			if !hasPrefix(row[2], "person_") || !hasPrefix(row[2], "asserted_") {
				t.Errorf("row %d lp:name %v: want inferred and asserted names", i, row[2])
			}
			if !reflect.DeepEqual(row[2], row[4]) {
				t.Errorf("row %d: the duplicated column differs: %v, %v", i, row[2], row[4])
			}
			if tags, _ := row[3].([]any); len(tags) != 20 {
				t.Errorf("row %d xr:tag %v: want 20 values", i, row[3])
			}
			if !reflect.DeepEqual(row[1], []any{"lp:Person"}) || row[5] != nil || row[7] != nil || row[8] != 1 {
				t.Errorf("row %d rdf:type %v, xr:count %v, xr:absent %v, source period %v", i, row[1], row[5], row[7], row[8])
			}
			if notes, _ := row[6].([]any); len(notes) != 2 {
				t.Errorf("row %d xr:note %v: want both texts", i, row[6])
			}
			if js, _ := row[9].(string); len(js) == 0 || !slices.Contains([]byte(js), '{') {
				t.Errorf("row %d xr:as_json %v: want a JSON object", i, row[9])
			}
		}
	})
}
