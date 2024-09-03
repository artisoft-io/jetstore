package rete

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

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
	cm.tripleUpdated(s, p, o, true)
}

func (cm *ReteCallback) TripleDeleted(s, p, o *rdf.Node) {
	cm.tripleUpdated(s, p, o, false)
}

func (cm *ReteCallback) tripleUpdated(s, p, o *rdf.Node, isInserted bool) {
  if cm.sFilter!= nil && cm.sFilter!=s {
		return
	}
  if cm.pFilter!= nil && cm.pFilter!=p {
		return
	}
  if cm.oFilter!= nil && cm.oFilter!=o {
		return
	}

	if cm.forFilterTerm {
		cm.reteSession.TripleUpdatedForFilter(cm.vertex, s, p, o, isInserted)
	} else {
		cm.reteSession.TripleUpdated(cm.vertex, s, p, o, isInserted)
	}
}

