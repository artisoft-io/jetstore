package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// ReteSession type -- main session class for the rete network

type ReteSession struct {
	RdfSession *rdf.RdfSession
}

func NewReteSession(rdfSession *rdf.RdfSession) *ReteSession {
	return &ReteSession{
		RdfSession: rdfSession,
	}
}

func (rs *ReteSession) GetBetaRelation(vertex int) *BetaRelation {
	//*##* GetBetaRelation
	return nil
}

func (rs *ReteSession) TripleUpdated(vertex int, s, p, o *rdf.Node, isInserted bool) *BetaRelation {
	//*##* TripleUpdated
	return nil
}

func (rs *ReteSession) TripleUpdatedForFilter(vertex int, s, p, o *rdf.Node, isInserted bool) *BetaRelation {
	//*##* TripleUpdatedForFilter
	return nil
}
