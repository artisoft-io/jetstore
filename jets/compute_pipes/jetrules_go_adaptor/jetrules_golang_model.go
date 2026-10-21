package jetrules_go_adaptor

import (
	"sync"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains the adaptor code to implement JetrulesInterface for the Go rule engine.

// Implementation types for cpipes facade types
type JetRulesFactoryGo struct {
}

func NewJetRulesFactory() compute_pipes.JetRulesFactory {
	return &JetRulesFactoryGo{}
}

type JetRuleEngineGo struct {
	factory              *JetRulesFactoryGo
	jetResources         *compute_pipes.JetResources
	processName          string
	reteMetaStoreFactory *rete.ReteMetaStoreFactory
	isDebug              bool

	// jetResourcesOnce guards jetResources, which the sessions of a pool share
	jetResourcesOnce sync.Once
}

type RdfNodeGo struct {
	node *rdf.Node
}

type JetResourceManagerGo struct {
	rm *rdf.ResourceManager
}

type JetRdfSessionGo struct {
	re         *JetRuleEngineGo
	rdfSession *rdf.RdfSession
}

type JetReteSessionGo struct {
	rdfSession  *rdf.RdfSession
	reteSession *rete.ReteSession
}

// TripleIteratorGo is the pull iterator compute_pipes reads, over the Go engine's
// synchronous iterators. It is lazy (rdf.TripleCursor): each Next reads the next
// triple from the graph, so stopping early costs only what was read, and it holds
// no goroutine, so an iterator that is never released leaks nothing.
type TripleIteratorGo struct {
	cursor rdf.TripleCursor
	value  [3]*rdf.Node
	isEnd  bool
}

func NewTripleIteratorGo(itor *rdf.BaseGraphIterator, sesIter *rdf.RdfSessionIterator) *TripleIteratorGo {
	t3Itor := &TripleIteratorGo{}
	switch {
	case itor != nil:
		t3Itor.cursor = itor.Cursor()
	case sesIter != nil:
		t3Itor.cursor = sesIter.Cursor()
	}
	// move to the first value
	t3Itor.Next()
	return t3Itor
}
