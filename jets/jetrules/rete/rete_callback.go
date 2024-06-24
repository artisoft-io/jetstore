package rete

import "github.com/artisoft-io/jetstore/jets/jetrules/rdf"

// Rete Calback Implementation

// forFilterTerm is true when the callback if for a rule filter term
type ReteCallback struct {
	reteSession   *ReteSession
	vertex        int
	sFilter       *rdf.Node
	pFilter       *rdf.Node
	oFilter       *rdf.Node
	forFilterTerm bool
}

func NewReteCallback(rs *ReteSession, vertex int, s, p, o *rdf.Node) rdf.NotificationCallback {
	return &ReteCallback{
		reteSession: rs,
		vertex: vertex,
		sFilter: s,
		pFilter: p,
		oFilter: o,
		forFilterTerm: false,
	}
}

func NewReteCallbackForFilter(rs *ReteSession, vertex int, p *rdf.Node) rdf.NotificationCallback {
	return &ReteCallback{
		reteSession: rs,
		vertex: vertex,
		pFilter: p,
		forFilterTerm: true,
	}
}

func (cm *ReteCallback) TripleInserted(s, p, o *rdf.Node) {
	XXX
}

func (cm *ReteCallback) TripleDeleted(s, p, o *rdf.Node) {
	XXX
}

