package rete

// The lookups' subjects, cache and misses, as the C++ engine has them
// (rules_benchmark I-15, I-31 and I-104; decided Q-62: C++ is correct and Go
// changes). C++ is jets/rete/lookup_sql_helper.{h,cc} and expr_op_others.h.

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// twoTables opens a lookup.db with two tables, lk_a and lk_b, that share the
// keys K1 and K2 and the column V, and a third, lk_n, keyed by a number. K2
// has two rows in each table.
func twoTables(t *testing.T) (*TestLookupTableManagerContext, *LookupTableManager) {
	t.Helper()
	ctx := NewTestLookupTableManagerContext()
	rm := ctx.reteSession.RdfSession.ResourceMgr
	mgr := openTwoTables(t, rm, ctx.metaGraph)
	ctx.reteSession.ms = &ReteMetaStore{ResourceMgr: rm, MetaGraph: ctx.metaGraph, LookupTables: mgr}
	return ctx, mgr
}

func openTwoTables(t *testing.T, rm *rdf.ResourceManager, metaGraph *rdf.RdfGraph) *LookupTableManager {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "lookup.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err = db.Exec(`CREATE TABLE "lk_a" (__key__ INTEGER, "jets:key" TEXT NOT NULL, "V" TEXT);
INSERT INTO "lk_a" VALUES (1, 'K1', 'a1'), (2, 'K2', 'a2'), (2, 'K2', 'a3');
CREATE TABLE "lk_b" (__key__ INTEGER, "jets:key" TEXT NOT NULL, "V" TEXT);
INSERT INTO "lk_b" VALUES (1, 'K1', 'b1'), (2, 'K2', 'b2'), (2, 'K2', 'b3');
CREATE TABLE "lk_n" (__key__ INTEGER, "jets:key" TEXT NOT NULL, "V" TEXT);
INSERT INTO "lk_n" VALUES (1, '7', 'seven');`); err != nil {
		t.Fatal(err)
	}
	rm.NewResource("V")
	mgr := &LookupTableManager{LookupTableMap: map[string]LookupTable{}}
	for _, name := range []string{"lk_a", "lk_b", "lk_n"} {
		tbl, err := NewLookupTableSqlite3(rm, metaGraph, &LookupTableNode{
			Name: name, Columns: []LookupTableColumn{{Name: "V", Type: "text"}}, Key: []string{"V"},
		}, db)
		if err != nil {
			t.Fatal(err)
		}
		mgr.LookupTableMap[name] = tbl
	}
	return mgr
}

// valuesOf returns the V of a lookup row, or of each row of a multi-lookup.
func valuesOf(sess *rdf.RdfSession, subject *rdf.Node, multi bool) map[string]bool {
	rm := sess.ResourceMgr
	v := rm.GetResource("V")
	got := map[string]bool{}
	if !multi {
		for t3 := range sess.FindSP(subject, v).All() {
			got[t3[2].String()] = true
		}
		return got
	}
	for r := range sess.FindSP(subject, rm.JetsResources.Jets__lookup_multi_rows).All() {
		for t3 := range sess.FindSP(r[2], v).All() {
			got[t3[2].String()] = true
		}
	}
	return got
}

// I-15: two tables looked up with the same key in one session were one subject,
// jets:lookup:<key>, so the second lookup read the first one's cached row. Each
// subject now names its table, as C++ names it (lookup_sql_helper.h:201):
// jets:<T>:<key>, for lookup and multi_lookup alike, and the cache triple is
// (<T> jets:lookup_row subject) (lookup_sql_helper.cc:105).
func TestLookupSubjectsNameTheirTable(t *testing.T) {
	ctx, mgr := twoTables(t)
	rs := ctx.reteSession
	sess := rs.RdfSession
	rm := sess.ResourceMgr
	lookupRow := rm.JetsResources.Jets__lookup_row
	for _, c := range []struct {
		multi bool
		key   string
		want  map[string]map[string]bool
	}{
		{false, "K1", map[string]map[string]bool{"lk_a": {"a1": true}, "lk_b": {"b1": true}}},
		{true, "K2", map[string]map[string]bool{"lk_a": {"a2": true, "a3": true}, "lk_b": {"b2": true, "b3": true}}},
	} {
		for _, name := range []string{"lk_a", "lk_b"} {
			key := c.key
			var row *rdf.Node
			var err error
			if c.multi {
				row, err = mgr.LookupTableMap[name].MultiLookup(rs, &name, &key)
			} else {
				row, err = mgr.LookupTableMap[name].Lookup(rs, &name, &key)
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := "jets:" + name + ":" + key; row.String() != want {
				t.Errorf("multi %v on %s: subject %s, want %s", c.multi, name, row, want)
			}
			if got := valuesOf(sess, row, c.multi); len(got) != len(c.want[name]) || !containsAll(got, c.want[name]) {
				t.Errorf("multi %v on %s key %s: values %v, want %v", c.multi, name, key, got, c.want[name])
			}
			if !sess.Contains(rm.GetResource(name), lookupRow, row) {
				t.Errorf("multi %v on %s: no cache triple (%s jets:lookup_row %s)", c.multi, name, name, row)
			}
		}
	}
}

func containsAll(got, want map[string]bool) bool {
	for k := range want {
		if !got[k] {
			return false
		}
	}
	return true
}

// The random lookups use C++'s subjects too (lookup_sql_helper.cc:165-167):
// jets:<T>:<n> for lookup_rand and jets:rand:<T>:<n> for multi_lookup_rand,
// where n is the __key__ drawn.
func TestLookupRandSubjectsNameTheirTable(t *testing.T) {
	ctx, mgr := twoTables(t)
	rs := ctx.reteSession
	sess := rs.RdfSession
	rm := sess.ResourceMgr
	name := "lk_a"
	tbl := mgr.LookupTableMap[name]
	for range 50 {
		row, err := tbl.LookupRand(rs, &name)
		if err != nil {
			t.Fatal(err)
		}
		if s := row.String(); s != "jets:lk_a:1" && s != "jets:lk_a:2" {
			t.Fatalf("lookup_rand subject %s, want jets:lk_a:1 or jets:lk_a:2", s)
		}
		multi, err := tbl.MultiLookupRand(rs, &name)
		if err != nil {
			t.Fatal(err)
		}
		if s := multi.String(); s != "jets:rand:lk_a:1" && s != "jets:rand:lk_a:2" {
			t.Fatalf("multi_lookup_rand subject %s, want jets:rand:lk_a:1 or jets:rand:lk_a:2", s)
		}
		for _, r := range []*rdf.Node{row, multi} {
			if !sess.Contains(rm.GetResource(name), rm.JetsResources.Jets__lookup_row, r) {
				t.Fatalf("no cache triple (lk_a jets:lookup_row %s)", r)
			}
		}
		if !sess.ContainsSP(multi, rm.JetsResources.Jets__lookup_multi_rows) {
			t.Fatalf("multi_lookup_rand %s has no rows", multi)
		}
	}
}

// I-31: a multi_lookup miss returned an empty subject and cached it, where C++
// returns null and writes nothing (lookup_sql_helper.cc:99-103), as a lookup
// miss does on both engines.
func TestMultiLookupMissIsNull(t *testing.T) {
	ctx, mgr := twoTables(t)
	rs := ctx.reteSession
	sess := rs.RdfSession
	name, key := "lk_a", "NONE"
	before := sess.Size()
	for _, multi := range []bool{false, true} {
		var row *rdf.Node
		var err error
		if multi {
			row, err = mgr.LookupTableMap[name].MultiLookup(rs, &name, &key)
		} else {
			row, err = mgr.LookupTableMap[name].Lookup(rs, &name, &key)
		}
		if err != nil {
			t.Fatal(err)
		}
		if row != rdf.Null() {
			t.Errorf("multi %v miss: got %s, want null", multi, row)
		}
	}
	if after := sess.Size(); after != before {
		t.Errorf("a miss wrote %d triples", after-before)
	}
}

// The rule form of I-31: a rule that tests a miss with (?x p null) fires for a
// multi_lookup as it does for a lookup.
//
//	v1  (?s src ?k)    ->  (?s p multi_lookup(lk_a, ?k))
//	v2  (?x p null)    ->  (?x missed yes)
func TestMultiLookupMissFiresNullRule(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	metaGraph := rdf.NewMetaRdfGraph(rm)
	mgr := openTwoTables(t, rm, metaGraph)
	src, p, missed, yes, lkA := rm.NewResource("src"), rm.NewResource("p"), rm.NewResource("missed"), rm.NewResource("yes"), rm.NewResource("lk_a")

	ml := NewExprBinaryOp(NewExprCst(lkA), NewMultiLookupOp(), NewExprBindedVar(1, "?k"))
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?s", "?k"}))
	nv2 := NewNodeVertex(2, head, false, 100, nil, "v2", []string{"r2"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?x"}))
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: src}, &FVariable{variable: "?k"}, nv1, true, "(?s src ?k)"),
		NewAlphaNode(&FVariable{variable: "?x"}, &FConstant{node: p}, &FConstant{node: rdf.Null()}, nv2, true, "(?x p null)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: p}, &FExpression{expression: ml}, nv1, false, "(?s p multi_lookup(lk_a, ?k))"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: missed}, &FConstant{node: yes}, nv2, false, "(?x missed yes)"),
	}
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an, LegacyRetraction)
		}
	}
	config := map[string]string{"$max_rule_exec": "1000"}
	ms, err := NewReteMetaStore(rm, metaGraph, mgr, alphaNodes, []*NodeVertex{head, nv1, nv2}, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := rdf.NewRdfSession(rm, metaGraph)
	rs := NewReteSession(sess)
	rs.Initialize(ms)
	defer rs.Done()
	srm := sess.ResourceMgr
	hit, miss := srm.NewResource("hit"), srm.NewResource("miss")
	sess.Insert(hit, src, srm.NewTextLiteral("K2"))
	sess.Insert(miss, src, srm.NewTextLiteral("NONE"))
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	if !sess.Contains(miss, p, rdf.Null()) {
		t.Errorf("(miss p null) not inferred; miss's p is %v", sess.GetObject(miss, p))
	}
	if !sess.Contains(miss, missed, yes) {
		t.Errorf("(?x p null) did not fire for a multi_lookup miss")
	}
	if sess.Contains(hit, missed, yes) {
		t.Errorf("(?x p null) fired for a multi_lookup hit")
	}
	if got := sess.GetObject(hit, p); got == nil || got.String() != "jets:lk_a:K2" {
		t.Errorf("(hit p ?) is %v, want jets:lk_a:K2", got)
	}
}

// I-104: an integer key, signed or unsigned, is written as text, as C++'s
// LookupVisitor does with std::to_string (expr_op_others.h:35-38), where Go
// panicked. A resource key is
// its name, as on C++. A null key gives no result, unchanged (C++ throws).
func TestLookupKeyConversion(t *testing.T) {
	ctx, _ := twoTables(t)
	rs := ctx.reteSession
	rm := rs.RdfSession.ResourceMgr
	tbl := rm.NewResource("lk_n")
	for _, multi := range []bool{false, true} {
		op := NewLookupOp()
		if multi {
			op = NewMultiLookupOp()
		}
		for _, key := range []*rdf.Node{rm.NewIntLiteral(7), rm.NewUIntLiteral(7), rm.NewResource("7")} {
			row := op.Eval(rs, nil, tbl, key)
			if row == nil || row.String() != "jets:lk_n:7" {
				t.Errorf("multi %v, key %v: got %v, want jets:lk_n:7", multi, key, row)
			}
		}
		if row := op.Eval(rs, nil, tbl, rdf.Null()); row != nil {
			t.Errorf("multi %v, null key: got %v, want nil", multi, row)
		}
	}
}
