package jetrules_go_adaptor

import (
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

type TripleIteratorGo struct {
	iterator *rdf.BaseGraphIterator
	sesIter  *rdf.RdfSessionIterator
	Itor     chan [3]*rdf.Node
	value    [3]*rdf.Node
	isEnd    bool
}

func NewTripleIteratorGo(itor *rdf.BaseGraphIterator, sesIter *rdf.RdfSessionIterator) *TripleIteratorGo {
	t3Itor := &TripleIteratorGo{
		iterator: itor,
		sesIter:  sesIter,
	}
	switch {
	case itor != nil:
		t3Itor.Itor = itor.Itor
	case sesIter != nil:
		t3Itor.Itor = sesIter.Itor

	}
	// move to the first value
	t3Itor.Next()
	return t3Itor
}
