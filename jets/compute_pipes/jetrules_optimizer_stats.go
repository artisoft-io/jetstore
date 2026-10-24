package compute_pipes

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/optstats"
	_ "modernc.org/sqlite"
)

// The rule optimizer's statistics, counted from sessions' graphs (jets/optstats,
// rules_benchmark Phase 9). rules_bench -stats and a jetrules step's
// optimizer_statistics option both count with OptimizerStats, after each rule set
// of each session, from the RuleSetsTimer's RuleSetDone.
//
// What is counted is taken from the network the run loaded: workspace.db's
// antecedents of each main rule set, positive and negated. Each constant of an
// antecedent is made a node through the session's resource manager, as the
// columns of an output channel are (columnReader), and its key is rendered from
// that node; triples are matched to it by the node's identity key. So a rule's
// constant and the graph's node are one node, and their keys agree by
// construction.
//
// The graph counted is everything the session's Find returns when the rule set
// ends: the meta graph, the asserted and the inferred triples, each triple once.
// That is what the rule set's antecedents match.

// statsTerm is one term of an antecedent as workspace.db holds it: kind is the
// resources table's type, "" for a variable or a resource without a value.
type statsTerm struct {
	kind, value string
}

type statsAntecedent struct {
	s, p, o statsTerm
}

// OptimizerStats counts the statistics of the rule sets of one workspace over
// any number of sessions. It is safe for concurrent use by the workers of a pool.
type OptimizerStats struct {
	antecedents map[string][]statsAntecedent
	acc         *optstats.Accumulator
}

// LoadOptimizerStats reads the antecedents of every main rule set of the
// workspace.db at path, which is the workspace the engine loaded
// (workspace.WorkspaceDbPath()).
func LoadOptimizerStats(path string) (*OptimizerStats, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("optimizer statistics: workspace.db: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT wc.source_file_name,
	    coalesce(s.type, ''), coalesce(s.value, ''), coalesce(p.type, ''), coalesce(p.value, ''),
	    coalesce(o.type, ''), coalesce(o.value, '')
	  FROM workspace_control wc
	  JOIN rete_nodes rn ON rn.source_file_key = wc.key AND rn.type = 'antecedent'
	  LEFT JOIN resources s ON s.key = rn.subject_key
	  LEFT JOIN resources p ON p.key = rn.predicate_key
	  LEFT JOIN resources o ON o.key = rn.object_key
	  WHERE wc.is_main = 1
	  ORDER BY wc.source_file_name, rn.vertex`)
	if err != nil {
		return nil, fmt.Errorf("optimizer statistics: workspace.db %s: %w", path, err)
	}
	defer rows.Close()
	st := &OptimizerStats{antecedents: map[string][]statsAntecedent{}, acc: optstats.NewAccumulator()}
	for rows.Next() {
		var ruleSet string
		var a statsAntecedent
		if err := rows.Scan(&ruleSet, &a.s.kind, &a.s.value, &a.p.kind, &a.p.value, &a.o.kind, &a.o.value); err != nil {
			return nil, err
		}
		st.antecedents[ruleSet] = append(st.antecedents[ruleSet], a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return st, nil
}

// CountRuleSet counts rdfSession's graph for ruleSet, a main rule file as the
// workspace control names it, and adds it to the statistics as one session. Call
// it when the rule set has ended, before the next one runs.
func (st *OptimizerStats) CountRuleSet(ruleSet string, rdfSession JetRdfSession) error {
	ants, ok := st.antecedents[ruleSet]
	if !ok {
		return fmt.Errorf("optimizer statistics: rule set %s has no antecedent in workspace.db", ruleSet)
	}
	rm := rdfSession.GetResourceManager()
	ids := map[string]any{}
	key := func(t statsTerm) string {
		n := statsNode(rm, t)
		if n == nil || n.IsNil() {
			return ""
		}
		k, ok := statsTermKey(n)
		if !ok {
			return ""
		}
		ids[k] = statsIdentity(n)
		return k
	}
	wanted := optstats.NewWanted()
	for _, a := range ants {
		wanted.AddAntecedent(key(a.s), key(a.p), key(a.o))
	}
	counter, err := optstats.NewCounter(wanted, func(k string) (any, bool) {
		id, ok := ids[k]
		return id, ok
	})
	if err != nil {
		return fmt.Errorf("optimizer statistics: rule set %s: %v", ruleSet, err)
	}
	itor := rdfSession.Find()
	if itor == nil {
		return fmt.Errorf("optimizer statistics: rule set %s: the session's Find returned no iterator", ruleSet)
	}
	defer itor.Release()
	pk, _ := itor.(PredicateKeyer)
	// Find walks the meta, asserted and inferred graphs, so one triple can come
	// twice; each counted triple is counted once.
	seen := map[[3]any]struct{}{}
	for !itor.IsEnd() {
		var p any
		if pk != nil {
			p = pk.PredicateKey()
		} else {
			p = statsIdentity(itor.GetPredicate())
		}
		if counter.Wants(p) {
			t := [3]any{statsIdentity(itor.GetSubject()), p, statsIdentity(itor.GetObject())}
			if _, dup := seen[t]; !dup {
				seen[t] = struct{}{}
				counter.Add(t[0], t[1], t[2])
			}
		}
		itor.Next()
	}
	return st.acc.Add(ruleSet, counter.Session())
}

// File is the statistics counted so far.
func (st *OptimizerStats) File() *optstats.File {
	return st.acc.File()
}

// Len is the number of rule sets with statistics.
func (st *OptimizerStats) Len() int {
	return st.acc.Len()
}

// statsNode makes the node of a constant that the statistics count, through the
// session's resource manager; nil for any other term (optstats.TermKey).
func statsNode(rm JetResourceManager, t statsTerm) RdfNode {
	switch t.kind {
	case optstats.KindResource:
		if t.value == "" {
			return nil
		}
		return rm.NewResource(t.value)
	case optstats.KindVolatileResource:
		if t.value == "" {
			return nil
		}
		return rm.NewResource(optstats.VolatileName(t.value))
	case optstats.KindText:
		return rm.NewTextLiteral(t.value)
	case optstats.KindInt:
		v, err := strconv.Atoi(t.value)
		if err != nil {
			return nil
		}
		return rm.NewIntLiteral(v)
	}
	return nil
}

// statsTermKey renders a node as the statistics file keys it: a resource by
// its name, text quoted, an int as N^^int. Other nodes are not counted.
func statsTermKey(n RdfNode) (string, bool) {
	switch n.Type() {
	case "named_resource":
		if name, ok := GetRdfNodeValue(n).(string); ok {
			return optstats.TermKey(optstats.KindResource, name)
		}
	case "string":
		if v, ok := n.Value().(string); ok {
			return optstats.TermKey(optstats.KindText, v)
		}
	case "int", "int32", "int64":
		if v, ok := GetRdfNodeValue(n).(int); ok {
			return optstats.TermKey(optstats.KindInt, strconv.Itoa(v))
		}
	}
	return "", false
}

// statsIdentity is a node's identity: its identity key, or, for an engine whose
// nodes have none, its type and text.
func statsIdentity(n RdfNode) any {
	if k := nodeKey(n); k != nil {
		return k
	}
	if n == nil {
		return nil
	}
	return n.Type() + "\x00" + n.String()
}
