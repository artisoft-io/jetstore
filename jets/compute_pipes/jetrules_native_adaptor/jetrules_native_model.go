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
}

type RdfNodeNative struct {
	node *bridge.Resource
}

type JetResourceManagerNative struct {
	js *bridge.JetStore    // metadata resources
	rs *bridge.ReteSession // session-based resources
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
