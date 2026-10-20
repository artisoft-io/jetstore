package jetrules_go_adaptor

import (
	"slices"

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

// TripleIteratorGo is the pull iterator of compute_pipes.TripleIterator.
// The rdf iterators are synchronous and push the triples to a loop body, they
// have no Next(), so the matching triples are collected when the iterator is
// created and Next() walks the collected triples.
type TripleIteratorGo struct {
	triples [][3]*rdf.Node
	pos     int
	value   [3]*rdf.Node
	isEnd   bool
}

func NewTripleIteratorGo(itor *rdf.BaseGraphIterator, sesIter *rdf.RdfSessionIterator) *TripleIteratorGo {
	t3Itor := &TripleIteratorGo{}
	switch {
	case itor != nil:
		t3Itor.triples = slices.Collect(itor.All())
	case sesIter != nil:
		t3Itor.triples = slices.Collect(sesIter.All())
	}
	// move to the first value
	t3Itor.Next()
	return t3Itor
}
