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
