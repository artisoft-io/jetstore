package compute_pipes

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/optstats"
)

// statsNodeForTest is a named resource or a text literal, identified by its type
// and text (it has no NodeKey).
type statsNodeForTest struct{ typ, text string }

func (n statsNodeForTest) Hdle() any                 { return n }
func (n statsNodeForTest) IsNil() bool               { return false }
func (n statsNodeForTest) Value() any                { return n.text }
func (n statsNodeForTest) Type() string              { return n.typ }
func (n statsNodeForTest) Equals(other RdfNode) bool { return other != nil && other.Hdle() == any(n) }
func (n statsNodeForTest) String() string            { return n.text }

type statsResourcesForTest struct{ JetResourceManager }

func (statsResourcesForTest) NewResource(name string) RdfNode {
	return statsNodeForTest{"named_resource", name}
}
func (statsResourcesForTest) NewTextLiteral(text string) RdfNode {
	return statsNodeForTest{"string", text}
}

// statsSessionForTest holds triples, and implements what Count calls.
type statsSessionForTest struct {
	JetRdfSession
	triples [][3]RdfNode
}

func (s *statsSessionForTest) GetResourceManager() JetResourceManager { return statsResourcesForTest{} }
func (s *statsSessionForTest) Find() TripleIterator                   { return &statsIteratorForTest{triples: s.triples} }

type statsIteratorForTest struct {
	triples [][3]RdfNode
	i       int
}

func (it *statsIteratorForTest) IsEnd() bool           { return it.i >= len(it.triples) }
func (it *statsIteratorForTest) Next() bool            { it.i++; return !it.IsEnd() }
func (it *statsIteratorForTest) GetSubject() RdfNode   { return it.triples[it.i][0] }
func (it *statsIteratorForTest) GetPredicate() RdfNode { return it.triples[it.i][1] }
func (it *statsIteratorForTest) GetObject() RdfNode    { return it.triples[it.i][2] }
func (it *statsIteratorForTest) Release() error        { return nil }

// A main rule set with no antecedent, a rule file with no rule, counts its
// sessions and nothing else, rather than failing the step; a rule set that is no
// main rule set of workspace.db is still an error.
func TestOptimizerStatsRuleSetWithoutAntecedents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE workspace_control (key INTEGER PRIMARY KEY, source_file_name TEXT, is_main INTEGER)`,
		`CREATE TABLE resources (key INTEGER PRIMARY KEY, type TEXT, value TEXT)`,
		`CREATE TABLE rete_nodes (key INTEGER PRIMARY KEY, vertex INTEGER, type TEXT,
		   subject_key INTEGER, predicate_key INTEGER, object_key INTEGER, source_file_key INTEGER)`,
		`INSERT INTO workspace_control VALUES (1, 'jet_rules/rules.jr', 1), (2, 'jet_rules/no_rules.jr', 1), (3, 'jet_rules/imported.jr', 0)`,
		`INSERT INTO resources VALUES (10, 'var', NULL), (11, 'resource', 'rdf:type'), (12, 'resource', 'acme:Claim')`,
		`INSERT INTO rete_nodes VALUES (100, 0, 'head_node', NULL, NULL, NULL, 1), (101, 1, 'antecedent', 10, 11, 12, 1),
		   (102, 1, 'consequent', 10, 11, 12, 1), (200, 0, 'head_node', NULL, NULL, NULL, 2)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	st, err := LoadOptimizerStats(path)
	if err != nil {
		t.Fatal(err)
	}
	claim := statsNodeForTest{"named_resource", "acme:Claim"}
	typ := statsNodeForTest{"named_resource", "rdf:type"}
	sess := &statsSessionForTest{triples: [][3]RdfNode{
		{statsNodeForTest{"named_resource", "acme:c1"}, typ, claim},
		{statsNodeForTest{"named_resource", "acme:c2"}, typ, claim},
	}}
	for _, rs := range []string{"jet_rules/rules.jr", "jet_rules/no_rules.jr", "jet_rules/no_rules.jr"} {
		ss := st.NewSession()
		if err := ss.Count(rs, sess); err != nil {
			t.Fatalf("%s: %v", rs, err)
		}
		if err := ss.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	ss := st.NewSession()
	if err := ss.Count("jet_rules/imported.jr", sess); err == nil || !strings.Contains(err.Error(), "not a main rule set") {
		t.Errorf("a rule file that is not main: %v", err)
	}
	if ss.Commit() == nil {
		t.Errorf("a session with a counting error committed")
	}
	// A session counted and never committed, as a failed one is, adds nothing.
	dropped := st.NewSession()
	if err := dropped.Count("jet_rules/rules.jr", sess); err != nil {
		t.Fatal(err)
	}
	f := st.File()
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	if rs := f.ForRuleSet("jet_rules/no_rules.jr"); rs == nil || rs.Sessions != 2 || len(rs.Predicates) != 0 {
		t.Errorf("no_rules.jr: %+v, want 2 sessions and no predicate", rs)
	}
	rs := f.ForRuleSet("jet_rules/rules.jr")
	if rs == nil || rs.Sessions != 1 || *rs.Predicates["rdf:type"] != (optstats.Predicate{Triples: 2, Subjects: 2, Objects: 1}) ||
		rs.ObjectConstants["rdf:type"]["acme:Claim"] != 2 {
		t.Errorf("rules.jr: %+v", rs)
	}
	if _, err := optstats.Marshal(f); err != nil {
		t.Errorf("the file does not encode: %v", err)
	}

	// Commit is all or none: a session whose second rule set the statistics
	// refuse (here, rules.jr counted against another network) adds neither.
	other := optstats.NewRuleSet()
	other.Sessions = 1
	other.Predicates["acme:other"] = &optstats.Predicate{}
	st2, err := LoadOptimizerStats(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st2.Add("jet_rules/rules.jr", other); err != nil {
		t.Fatal(err)
	}
	ss2 := st2.NewSession()
	for _, rs := range []string{"jet_rules/no_rules.jr", "jet_rules/rules.jr"} {
		if err := ss2.Count(rs, sess); err != nil {
			t.Fatal(err)
		}
	}
	if err := ss2.Commit(); err == nil || !strings.Contains(err.Error(), "different networks") {
		t.Errorf("Commit: %v", err)
	}
	if got := st2.File(); got.ForRuleSet("jet_rules/no_rules.jr") != nil || got.ForRuleSet("jet_rules/rules.jr").Sessions != 1 {
		t.Errorf("a refused Commit added part of the session: %+v", got.RuleSets)
	}
}
