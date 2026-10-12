package rete

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

type TestLookupTableManagerContext struct {
	resourceManager *rdf.ResourceManager
	metaGraph       *rdf.RdfGraph
	reteSession     *ReteSession
}

func NewTestLookupTableManagerContext() *TestLookupTableManagerContext {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		log.Println("error: nil returned by NewResourceManager")
		return nil
	}
	metaGraph := rdf.NewRdfGraph("META")
	if metaGraph == nil {
		log.Println("error: nil returned by NewRdfGraph")
		return nil
	}
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		log.Println("error: nil returned by NewRdfSession")
		return nil
	}
	reteSession := NewReteSession(rdfSession)
	if reteSession == nil {
		log.Println("error: nil returned by NewReteSession")
		return nil
	}
	return &TestLookupTableManagerContext{
		resourceManager: rm,
		metaGraph:       metaGraph,
		reteSession:     reteSession,
	}
}

// useLookupFixture writes a lookup.db holding the tables these tests read into
// a temporary workspace, and points the package at it for the test's duration.
// They used to read a lookup.db under one developer's checkout, which had none
// of these tables (jetstore_maintenance_02 I-42).
//
// The schema is the packager's (jets/workspace/lookup_tables.go), which this
// package cannot import. firstKey is the first __key__: the current packager
// numbers from 1 and the earlier one numbered from 0, and the random lookups
// must work for both.
func useLookupFixture(t *testing.T, firstKey int) {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "test_ws"), 0755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "test_ws", "lookup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	type table struct {
		name    string
		columns string
		rows    [][]any // the jets:key first, then the columns
	}
	tables := []table{
		{"common_zipcode", `"ZIP_CITY" TEXT, "ZIP_COUNTY" TEXT, "SC_ID" TEXT, "ZIP_STATE" TEXT, "ZIP" TEXT`, [][]any{
			{"07960", "MORRISTOWN", "MORRIS", "34", "NJ", "07960"},
			{"10001", "NEW YORK", "NEW YORK", "36", "NY", "10001"},
			{"60601", "CHICAGO", "COOK", "17", "IL", "60601"},
		}},
		// TYPE repeats a key, which multi_lookup_rand returns as several rows.
		{"IM_Exclusion_Type", `"PRIORITY" INTEGER, "TYPE" TEXT`, [][]any{
			{"DIAGNOSIS", 1, "DIAGNOSIS"},
			{"MODALITY", 2, "MODALITY"},
			{"PROCEDURE", 3, "PROCEDURE"},
			{"PROCEDURE", 4, "PROCEDURE"},
		}},
	}
	for _, tbl := range tables {
		stmt := fmt.Sprintf(`CREATE TABLE %q (__key__ INTEGER, "jets:key" TEXT NOT NULL, %s);
CREATE INDEX %q ON %q ("jets:key");
CREATE INDEX %q ON %q (__key__);`, tbl.name, tbl.columns, tbl.name+"_idx", tbl.name, tbl.name+"_key_idx", tbl.name)
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
		// __key__ numbers each distinct jets:key, in order, as the packager does.
		keys := map[string]int{}
		for _, row := range tbl.rows {
			jetsKey := row[0].(string)
			if _, ok := keys[jetsKey]; !ok {
				keys[jetsKey] = firstKey + len(keys)
			}
			args := append([]any{keys[jetsKey]}, row...)
			marks := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
			if _, err := db.Exec(fmt.Sprintf(`INSERT INTO %q VALUES (%s)`, tbl.name, marks), args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	savedHome, savedPrefix := workspaceHome, wprefix
	workspaceHome, wprefix = home, "test_ws"
	t.Cleanup(func() { workspaceHome, wprefix = savedHome, savedPrefix })
}

func TestLookupTable1(t *testing.T) {
	useLookupFixture(t, 1)
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []*LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name: "ZIP_CITY", Type: "text"},
					{Name: "ZIP_COUNTY", Type: "text"},
					{Name: "SC_ID", Type: "text"},
					{Name: "ZIP_STATE", Type: "text"},
					{Name: "ZIP", Type: "text"},
				},
				CsvFile: "lookups/common/zipcode.csv",
				Key:     []string{"ZIP"},
				Name:    "common_zipcode",
				Type:    "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("ZIP_CITY")
	rm.NewResource("ZIP_COUNTY")
	rm.NewResource("SC_ID")
	rm.NewResource("ZIP_STATE")
	rm.NewResource("ZIP")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["common_zipcode"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for common_zipcode")
	}
	name := "common_zipcode"
	key := "07960"
	row, err := lookupTable.Lookup(ctx.reteSession, &name, &key)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key 07960, err: %v", err)
	}
	if row.String() != "jets:lookup:07960" {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.Contains(row, rm.GetResource("ZIP"), rm.GetLiteral("07960")) {
		t.Errorf("triple not found")
	}
	log.Println("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}
func TestLookupTable2(t *testing.T) {
	useLookupFixture(t, 1)
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []*LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name: "PRIORITY", Type: "int"},
					{Name: "TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key:     []string{"TYPE"},
				Name:    "IM_Exclusion_Type",
				Type:    "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for IM_Exclusion_Type")
	}
	name := "IM_Exclusion_Type"
	key := "MODALITY"
	row, err := lookupTable.Lookup(ctx.reteSession, &name, &key)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key MODALITY, err: %v", err)
	}
	if row.String() != "jets:lookup:MODALITY" {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.Contains(row, rm.GetResource("PRIORITY"), rm.GetLiteral(2)) {
		t.Errorf("triple not found")
	}
	t.Log("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}

func TestLookupRand1(t *testing.T) {
	useLookupFixture(t, 1)
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []*LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name: "PRIORITY", Type: "int"},
					{Name: "TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key:     []string{"TYPE"},
				Name:    "IM_Exclusion_Type",
				Type:    "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for IM_Exclusion_Type")
	}
	name := "IM_Exclusion_Type"
	row, err := lookupTable.LookupRand(ctx.reteSession, &name)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key MODALITY, err: %v", err)
	}
	if !strings.HasPrefix(row.String(), "jets:lookup:rand:") {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.ContainsSP(row, rm.GetResource("PRIORITY")) {
		t.Errorf("triple not found")
	}
	t.Log("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}

func TestLookupMultiRand1(t *testing.T) {
	useLookupFixture(t, 1)
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []*LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name: "PRIORITY", Type: "int"},
					{Name: "TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key:     []string{"TYPE"},
				Name:    "IM_Exclusion_Type",
				Type:    "lookup",
			},
		},
	}
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	if lookupMgr == nil {
		t.Fatalf("error: NewLookupTableManager returned nil")
	}
	if len(lookupMgr.LookupTableMap) == 0 {
		t.Fatalf("error: LookupTableManager has nil or empty LookupTableMap")
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	if lookupTable == nil {
		t.Fatalf("error: LookupTableManager has nil lookupTable for IM_Exclusion_Type")
	}
	name := "IM_Exclusion_Type"
	row, err := lookupTable.MultiLookupRand(ctx.reteSession, &name)
	if err != nil || row == nil {
		t.Fatalf("error: LookupTable returned nil for key MODALITY, err: %v", err)
	}
	if !strings.HasPrefix(row.String(), "jets:lookup:multi:rand:") {
		t.Errorf("row key not matching")
	}
	if !ctx.reteSession.RdfSession.Contains(rm.GetResource("jets:cache:IM_Exclusion_Type"), rm.GetResource("jets:lookup_multi_rows"), row) {
		t.Errorf("triple not found")
	}
	t.Log("Got row", row.String())
	itor := ctx.reteSession.RdfSession.Find()
	log.Println("The graph contains:")
	for t3 := range itor.Itor {
		log.Println(t3)
	}
	itor.Done()
}

func exclusionTypeLookup(t *testing.T) (*TestLookupTableManagerContext, LookupTable) {
	t.Helper()
	ctx := NewTestLookupTableManagerContext()
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	mgr, err := NewLookupTableManager(rm, ctx.metaGraph, &JetruleModel{LookupTables: []*LookupTableNode{{
		Columns: []LookupTableColumn{{Name: "PRIORITY", Type: "int"}, {Name: "TYPE", Type: "text"}},
		Key:     []string{"TYPE"},
		Name:    "IM_Exclusion_Type",
		Type:    "lookup",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, mgr.LookupTableMap["IM_Exclusion_Type"]
}

// lookup_rand and multi_lookup_rand draw a __key__ in [MIN, MAX]. They drew
// from [0, MAX) in Go, which asked for a key the packager never writes (it
// numbers from 1), returned Null for it, and never reached the last row.
func TestLookupRandReachesEveryKeyAndNeverNull(t *testing.T) {
	for _, firstKey := range []int{0, 1} {
		t.Run(fmt.Sprintf("keys from %d", firstKey), func(t *testing.T) {
			useLookupFixture(t, firstKey)
			ctx, tbl := exclusionTypeLookup(t)
			name := "IM_Exclusion_Type"
			want := map[string]bool{}
			for k := firstKey; k < firstKey+3; k++ {
				want[fmt.Sprint(k)] = true
			}
			seen, seenMulti := map[string]bool{}, map[string]bool{}
			for range 300 {
				row, err := tbl.LookupRand(ctx.reteSession, &name)
				if err != nil {
					t.Fatal(err)
				}
				key, ok := strings.CutPrefix(row.String(), "jets:lookup:rand:")
				if !ok {
					t.Fatalf("lookup_rand returned %s", row.String())
				}
				seen[key] = true

				multi, err := tbl.MultiLookupRand(ctx.reteSession, &name)
				if err != nil {
					t.Fatal(err)
				}
				mkey, ok := strings.CutPrefix(multi.String(), "jets:lookup:multi:rand:")
				if !ok {
					t.Fatalf("multi_lookup_rand returned %s", multi.String())
				}
				if !ctx.reteSession.RdfSession.ContainsSP(multi, ctx.reteSession.RdfSession.ResourceMgr.JetsResources.Jets__lookup_multi_rows) {
					t.Fatalf("multi_lookup_rand key %s returned no rows", mkey)
				}
				seenMulti[mkey] = true
			}
			for k := range want {
				if !seen[k] || !seenMulti[k] {
					t.Errorf("key %s never drawn (lookup_rand %v, multi_lookup_rand %v)", k, seen, seenMulti)
				}
			}
			for k := range seen {
				if !want[k] {
					t.Errorf("lookup_rand drew key %s, outside the table", k)
				}
			}
			for k := range seenMulti {
				if !want[k] {
					t.Errorf("multi_lookup_rand drew key %s, outside the table", k)
				}
			}
		})
	}
}

// An empty table gives Null rather than a panic (rand.Int63n(0)) or a failure
// to open the table (MAX of no rows is NULL); a one-row table gives that row.
func TestLookupRandOnEmptyAndOneRowTables(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "lookup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE "empty_lk" (__key__ INTEGER, "jets:key" TEXT NOT NULL, "V" TEXT);
CREATE TABLE "one_lk" (__key__ INTEGER, "jets:key" TEXT NOT NULL, "V" TEXT);
INSERT INTO "one_lk" VALUES (1, 'A', 'only');`); err != nil {
		t.Fatal(err)
	}
	ctx := NewTestLookupTableManagerContext()
	rm := ctx.reteSession.RdfSession.ResourceMgr
	rm.NewResource("V")
	for _, c := range []struct {
		name, wantPrefix string
	}{{"empty_lk", ""}, {"one_lk", "jets:lookup:rand:1"}} {
		tbl, err := NewLookupTableSqlite3(rm, ctx.metaGraph, &LookupTableNode{
			Name: c.name, Columns: []LookupTableColumn{{Name: "V", Type: "text"}}, Key: []string{"V"},
		}, db)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		name := c.name
		for range 20 {
			row, err := tbl.LookupRand(ctx.reteSession, &name)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			multi, err := tbl.MultiLookupRand(ctx.reteSession, &name)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if c.wantPrefix == "" {
				if row != rdf.Null() || multi != rdf.Null() {
					t.Fatalf("%s: got %s and %s, want Null", c.name, row, multi)
				}
				continue
			}
			if row.String() != c.wantPrefix {
				t.Fatalf("%s: got %s, want %s", c.name, row, c.wantPrefix)
			}
		}
	}
}

// TestLookupServesRowId checks that every served row carries jets:row_id, its
// position in the table, so that rows sharing a jets:key stay apart.
func TestLookupServesRowId(t *testing.T) {
	useLookupFixture(t, 1)
	ctx := NewTestLookupTableManagerContext()
	jetRuleModel := &JetruleModel{
		LookupTables: []*LookupTableNode{
			{
				Columns: []LookupTableColumn{
					{Name: "PRIORITY", Type: "int"},
					{Name: "TYPE", Type: "text"},
				},
				CsvFile: "lookups/IM/Exclusion_Type.csv",
				Key:     []string{"TYPE"},
				Name:    "IM_Exclusion_Type",
				Type:    "lookup",
			},
		},
	}
	sess := ctx.reteSession.RdfSession
	rm := sess.ResourceMgr
	rm.NewResource("PRIORITY")
	rm.NewResource("TYPE")
	lookupMgr, err := NewLookupTableManager(rm, ctx.metaGraph, jetRuleModel)
	if err != nil {
		t.Fatalf("while calling NewLookupTableManager: %v", err)
	}
	lookupTable := lookupMgr.LookupTableMap["IM_Exclusion_Type"]
	rowId := rm.JetsResources.Jets__row_id
	name := "IM_Exclusion_Type"

	// The fixture's rows are DIAGNOSIS, MODALITY, PROCEDURE, PROCEDURE.
	key := "MODALITY"
	row, err := lookupTable.Lookup(ctx.reteSession, &name, &key)
	if err != nil || row == nil {
		t.Fatalf("Lookup MODALITY: %v", err)
	}
	if !sess.Contains(row, rowId, rm.NewIntLiteral(2)) {
		t.Errorf("lookup row for MODALITY: want (%s jets:row_id 2), got %v", row, sess.GetObject(row, rowId))
	}

	key = "PROCEDURE"
	multi, err := lookupTable.MultiLookup(ctx.reteSession, &name, &key)
	if err != nil || multi == nil {
		t.Fatalf("MultiLookup PROCEDURE: %v", err)
	}
	// Each row's jets:row_id against its PRIORITY, which the fixture gives as 3 and 4.
	got := map[int]int{}
	itor := sess.FindSP(multi, rm.JetsResources.Jets__lookup_multi_rows)
	for t3 := range itor.Itor {
		id, priority := sess.GetObject(t3[2], rowId), sess.GetObject(t3[2], rm.GetResource("PRIORITY"))
		if id == nil || priority == nil {
			t.Errorf("multi lookup row %s: jets:row_id %v, PRIORITY %v", t3[2], id, priority)
			continue
		}
		got[id.Value.(int)] = priority.Value.(int)
	}
	itor.Done()
	if len(got) != 2 || got[3] != 3 || got[4] != 4 {
		t.Errorf("multi lookup rows for PROCEDURE: want jets:row_id 3 and 4 on PRIORITY 3 and 4, got %v", got)
	}
}
