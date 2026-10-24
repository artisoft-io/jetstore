package compute_pipes

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/optstats"
	_ "modernc.org/sqlite"
)

// The rule optimizer's statistics, counted from sessions' graphs (jets/optstats,
// rules_benchmark Phase 9). rules_bench -stats and a jetrules step's
// optimizer_statistics option both count with OptimizerStats, after each rule set
// of each session: from RuleSetsTimer's RuleSetDone in rules_bench, and from
// RuleSetsConfig.AfterRuleSet in a pipeline.
//
// A session's counts are collected per session (OptimizerStatsSession) and added
// to the statistics only when the whole session has ended without error: a
// session that fails contributes nothing.
//
// What is counted is taken from the network the run loaded: workspace.db's
// antecedents of each main rule set, positive and negated. Each constant of an
// antecedent is made a node through the session's resource manager, as the
// columns of an output channel are (columnReader), and its key is rendered from
// that node; triples are matched to it by the node's identity key. So a rule's
// constant and the graph's node are one node, and their keys agree by
// construction.
//
// In a pipeline, the jetrules step's optimizer_statistics option makes each node
// count over its sessions, its pool's workers sharing one OptimizerStats, and
// write one partial file when they are done (writeOptimizerStatsPartial). The
// count is made from the worker's AfterRuleSet, which a pipeline always installs
// (reportRuleSet), so the option adds no hook and reads no clock: off, it costs a
// nil check per rule set.
//
// The graph counted is everything the session's Find returns when the rule set
// ends: the meta graph, the asserted and the inferred triples. That is what the
// rule set's antecedents match. The three graphs never hold one triple twice,
// so each comes once: an asserted insert is refused when the meta graph holds
// the triple and erases it from the inferred graph, an inferred insert is
// refused when the meta or the asserted graph holds it (RDFSession insert and
// insert_inferred, jets/rdf/rdf_session.h), and the meta graph is locked once a
// session opens.

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
// (workspace.WorkspaceDbPath()). A main rule set with no antecedent, a rule file
// with no rule, is listed with none.
func LoadOptimizerStats(path string) (*OptimizerStats, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("optimizer statistics: workspace.db: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT wc.source_file_name, rn.key IS NOT NULL,
	    coalesce(s.type, ''), coalesce(s.value, ''), coalesce(p.type, ''), coalesce(p.value, ''),
	    coalesce(o.type, ''), coalesce(o.value, '')
	  FROM workspace_control wc
	  LEFT JOIN rete_nodes rn ON rn.source_file_key = wc.key AND rn.type = 'antecedent'
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
		var isAntecedent bool
		var a statsAntecedent
		if err := rows.Scan(&ruleSet, &isAntecedent, &a.s.kind, &a.s.value, &a.p.kind, &a.p.value, &a.o.kind, &a.o.value); err != nil {
			return nil, err
		}
		if !isAntecedent {
			st.antecedents[ruleSet] = []statsAntecedent{}
			continue
		}
		st.antecedents[ruleSet] = append(st.antecedents[ruleSet], a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return st, nil
}

// Count counts rdfSession's graph for ruleSet, a main rule file as the workspace
// control names it, as one session's statistics, and adds them to nothing. Call
// it when the rule set has ended, before the next one runs. A rule set with no
// antecedent counts the session and nothing else, its predicates empty. A rule
// set workspace.db does not have as a main rule file is an error: the run and the
// statistics would describe different workspaces.
func (st *OptimizerStats) Count(ruleSet string, rdfSession JetRdfSession) (*optstats.RuleSet, error) {
	ants, ok := st.antecedents[ruleSet]
	if !ok {
		return nil, fmt.Errorf("optimizer statistics: rule set %s is not a main rule set of workspace.db", ruleSet)
	}
	if len(ants) == 0 {
		rs := optstats.NewRuleSet()
		rs.Sessions = 1
		return rs, nil
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
		return nil, fmt.Errorf("optimizer statistics: rule set %s: %v", ruleSet, err)
	}
	itor := rdfSession.Find()
	if itor == nil {
		return nil, fmt.Errorf("optimizer statistics: rule set %s: the session's Find returned no iterator", ruleSet)
	}
	defer itor.Release()
	pk, _ := itor.(PredicateKeyer)
	for !itor.IsEnd() {
		var p any
		if pk != nil {
			p = pk.PredicateKey()
		} else {
			p = statsIdentity(itor.GetPredicate())
		}
		if counter.Wants(p) {
			counter.Add(statsIdentity(itor.GetSubject()), p, statsIdentity(itor.GetObject()))
		}
		itor.Next()
	}
	return counter.Session(), nil
}

// Add adds one session's statistics for ruleSet, as Count gives them.
func (st *OptimizerStats) Add(ruleSet string, rs *optstats.RuleSet) error {
	return st.acc.Add(ruleSet, rs)
}

// NewSession is a collector of one session's statistics, rule set by rule set,
// which adds them to st only when the session commits.
func (st *OptimizerStats) NewSession() *OptimizerStatsSession {
	return &OptimizerStatsSession{stats: st}
}

// File is the statistics counted so far.
func (st *OptimizerStats) File() *optstats.File {
	return st.acc.File()
}

// Len is the number of rule sets with statistics.
func (st *OptimizerStats) Len() int {
	return st.acc.Len()
}

// OptimizerStatsSession collects one session's statistics. Count each rule set
// as it ends, then Commit when the session has ended without error, or drop the
// collector: a failed session contributes nothing. It is used by one goroutine.
type OptimizerStatsSession struct {
	stats    *OptimizerStats
	ruleSets []string
	counts   []*optstats.RuleSet
	err      error
}

// Count counts rdfSession's graph for ruleSet into the session. After an error it
// counts nothing more, and Err reports the error.
func (s *OptimizerStatsSession) Count(ruleSet string, rdfSession JetRdfSession) error {
	if s.err != nil {
		return s.err
	}
	rs, err := s.stats.Count(ruleSet, rdfSession)
	if err != nil {
		s.err = err
		return err
	}
	s.ruleSets = append(s.ruleSets, ruleSet)
	s.counts = append(s.counts, rs)
	return nil
}

// Err is the first error counting the session.
func (s *OptimizerStatsSession) Err() error {
	return s.err
}

// Commit adds the session's statistics, each rule set it counted as one session.
// A session with a counting error is refused.
func (s *OptimizerStatsSession) Commit() error {
	if s.err != nil {
		return s.err
	}
	for i, rs := range s.counts {
		if err := s.stats.Add(s.ruleSets[i], rs); err != nil {
			return err
		}
	}
	s.counts, s.ruleSets = nil, nil
	return nil
}

// OptimizerStatsUploader is awsi.UploadBufToS3, indirected so that a test can see
// a node's partial file without an AWS account. Nothing outside a test assigns it.
var OptimizerStatsUploader = awsi.UploadBufToS3

// writeOptimizerStatsPartial writes the node's statistics as its partial file,
// optstats.PartialKey, in the jetstore bucket. A node that ran no session writes a
// file with no rule set, which merges as nothing.
func (ctx *BuilderContext) writeOptimizerStatsPartial(st *OptimizerStats) error {
	buf, err := optstats.Marshal(st.File())
	if err != nil {
		return fmt.Errorf("optimizer statistics: %v", err)
	}
	key := optstats.PartialKey(awsi.JetStoreStagePrefix(), ctx.processName, ctx.sessionId, ctx.jetsPartition, ctx.nodeId)
	if err := OptimizerStatsUploader("", key, buf); err != nil {
		return fmt.Errorf("optimizer statistics: while writing %s: %v", key, err)
	}
	log.Printf("optimizer statistics: wrote %s, %d rule sets", key, st.Len())
	return nil
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
		// workspace.db holds a volatile resource's name with one _0: trimmed, and the
		// engine always prefixes it when it loads the rules
		// (jets/rete/rete_meta_store_factory.cc), so a declared "_0:x" is _0:_0:x
		// in the graph. This mirrors the factory: never optstats.VolatileName, which
		// is for the compiler's value.
		return rm.NewResource(optstats.VolatilePrefix + t.value)
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
