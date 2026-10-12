package jetrules_native_adaptor

import (
	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// This file contains the adaptor code to implement JetrulesInterface for the Native rule engine.

// Implementation types for cpipes facade types
type JetRulesFactoryNative struct {
}

func NewJetRulesFactory() compute_pipes.JetRulesFactory {
	return &JetRulesFactoryNative{}
}

type JetRuleEngineNative struct {
	factory      *JetRulesFactoryNative
	jetResources *compute_pipes.JetResources
	processName  string
	js           *bridge.JetStore
	isDebug      bool
	// retractionMode is the LEGACY retraction mode the networks were built in
	retractionMode compute_pipes.RetractionMode
}

type RdfNodeNative struct {
	node *bridge.Resource
}

type JetResourceManagerNative struct {
	js *bridge.JetStore // metadata resources
	// session-based resources, made through the RDF session: through the rete
	// session they fell back to js until the first rete session existed, and the
	// meta graph is locked by then (rules_unit_test_harness I-15)
	rdfs *bridge.RDFSession
}

type JetRdfSessionNative struct {
	re            *JetRuleEngineNative
	rdfSession    *bridge.RDFSession
	rs            *bridge.ReteSession
	insertCounter int
	isDebug       bool
}

type JetReteSessionNative struct {
	rdfSession          *bridge.RDFSession
	reteSession         *bridge.ReteSession
	rdfSessionHdl       *JetRdfSessionNative
	ruleset             string
	executeErrorCounter int
	executeCounter      int
}

type TripleIteratorNative struct {
	null     *bridge.Resource
	iterator *bridge.RSIterator
}

func NewTripleIteratorNative(itor *bridge.RSIterator, null *bridge.Resource) *TripleIteratorNative {
	t3Itor := &TripleIteratorNative{
		iterator: itor,
		null:     null,
	}
	return t3Itor
}
