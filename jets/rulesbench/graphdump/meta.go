package graphdump

import (
	"fmt"
	"sync"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// engineSession is a session that names the engine it was opened on. Both
// adaptors' sessions implement it (JetRdfSessionGo, JetRdfSessionNative).
type engineSession interface {
	JetRuleEngine() compute_pipes.JetRuleEngine
}

// metaCache holds each engine's meta graph, read once. The meta graph is locked
// when an engine's first RDF session opens, so it cannot change afterwards. An
// entry keeps its engine reachable; an engine is built once per process in every
// caller this package has.
var metaCache sync.Map // compute_pipes.JetRuleEngine -> map[tripleKey]bool

// metaOf is the meta graph of sess's engine, by value.
func metaOf(sess compute_pipes.JetRdfSession) (map[tripleKey]bool, error) {
	es, ok := sess.(engineSession)
	if !ok {
		return nil, fmt.Errorf("graphdump: a %T cannot name its engine, so its meta graph cannot be left out; set IncludeMeta", sess)
	}
	engine := es.JetRuleEngine()
	if engine == nil {
		return nil, fmt.Errorf("graphdump: the session has no engine, so its meta graph cannot be left out; set IncludeMeta")
	}
	if m, ok := metaCache.Load(engine); ok {
		return m.(map[tripleKey]bool), nil
	}
	m, err := metaTriples(engine)
	if err != nil {
		return nil, err
	}
	actual, _ := metaCache.LoadOrStore(engine, m)
	return actual.(map[tripleKey]bool), nil
}

// metaTriples reads engine's meta graph: the triples of a new RDF session of the
// engine, which holds nothing else yet. Opening a session locks the meta graph
// (see compute_pipes.RunRuleSets), so this is called only from a dump, when the
// caller's own session is already open and the meta graph is complete.
func metaTriples(engine compute_pipes.JetRuleEngine) (map[tripleKey]bool, error) {
	sess, err := engine.NewRdfSession()
	if err != nil {
		return nil, fmt.Errorf("graphdump: while opening a session to read the meta graph: %w", err)
	}
	defer sess.Release()
	triples, err := readTriples(sess)
	if err != nil {
		return nil, fmt.Errorf("graphdump: while reading the meta graph: %w", err)
	}
	m := make(map[tripleKey]bool, len(triples))
	for _, t := range triples {
		m[keyOf(t)] = true
	}
	return m, nil
}
