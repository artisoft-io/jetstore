package rdf

import (
	"errors"
	"log"
)

// RDFGraph is a fully indexed rdf graph with type (*Node, *Node, *Node)
// It is made of 3 BaseGraph, each with a different spin: 's'po, 'p'os, 'o'sp

var ErrGraphLocked = errors.New("error: RdfGraph is locked, cannot mutate")
var ErrNilNode = errors.New("error: cannot insert/erase/retract triple with nil *Node in RdfGraph")

type RdfGraph struct {
	isLocked    bool
	spoGraph    *BaseGraph
	posGraph    *BaseGraph
	ospGraph    *BaseGraph
	callbackMgr *CallbackManager
}

func NewRdfGraph(graphType string) *RdfGraph {
	return &RdfGraph{
		spoGraph:    NewBaseGraph(graphType, 's'),
		posGraph:    NewBaseGraph(graphType, 'p'),
		ospGraph:    NewBaseGraph(graphType, 'o'),
		callbackMgr: NewCallbackManager(),
	}
}

func (g *RdfGraph) Size() int {
	return g.spoGraph.Size()
}

func (g *RdfGraph) Contains(s, p, o *Node) bool {
	return g.spoGraph.Contains(s, p, o)
}

func (g *RdfGraph) ContainsSP(s, p *Node) bool {
	return g.spoGraph.ContainsUV(s, p)
}

func (g *RdfGraph) Find() *BaseGraphIterator {
	return g.spoGraph.Find()
}

func (g *RdfGraph) FindSP(s, p *Node) *BaseGraphIterator {
	return g.spoGraph.FindUV(s, p)
}

func (g *RdfGraph) FindSPO(s, p, o *Node) *BaseGraphIterator {
	switch {
	case s == nil && p == nil && o == nil:
		// case (*, *, *)
		return g.spoGraph.Find()

	case s != nil && p == nil && o == nil:
		// case (s, *, *)
		return g.spoGraph.FindU(s)

	case s != nil && p != nil && o == nil:
		// case (s, p, *)
		return g.spoGraph.FindUV(s, p)

	case s != nil && p != nil && o != nil:
		// case (s, p, o)
		return g.spoGraph.FindUVW(s, p, o)

	case s == nil && p != nil && o == nil:
		// case (*, p, *)
		return g.posGraph.FindU(p)

	case s == nil && p != nil && o != nil:
		// case (*, p, o)
		return g.posGraph.FindUV(p, o)

	case s == nil && p == nil && o != nil:
		// case (*, *, o)
		return g.ospGraph.FindU(o)

	case s != nil && p == nil && o != nil:
		// case (s, *, o)
		return g.ospGraph.FindUV(o, s)
	}
	return nil
}

// Returns true if the triple is actually inserted (was not present in graph).
// Otherwise the reference count is increased by 1.
func (g *RdfGraph) Insert(s, p, o *Node) (bool, error) {
	if g.isLocked {
		return false, ErrGraphLocked
	}
	if s == nil || p == nil || o == nil {
		log.Printf("error: insert called with NULL *Node to RdfGraph: (%s, %s, %s)", s, p, o)
		return false, ErrNilNode
	}
	inserted := g.spoGraph.Insert(s, p, o)
	g.posGraph.Insert(p, o, s)
	g.ospGraph.Insert(o, s, p)
	if inserted {
		// Notify the callback mgr
		g.callbackMgr.TripleInserted(s, p, o)
	}
	return inserted, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) Erase(s, p, o *Node) (bool, error) {
	if g.isLocked {
		return false, ErrGraphLocked
	}
	if s == nil || p == nil || o == nil {
		log.Printf("error: erase called with NULL *Node to RdfGraph: (%s, %s, %s)", s, p, o)
		return false, ErrNilNode
	}
	erased := g.spoGraph.Erase(s, p, o)
	g.posGraph.Erase(p, o, s)
	g.ospGraph.Erase(o, s, p)
	if erased {
		// Notify callback mgr
		g.callbackMgr.TripleDeleted(s, p, o)
	}
	return erased, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) Retract(s, p, o *Node) (bool, error) {
	if g.isLocked {
		return false, ErrGraphLocked
	}
	if s == nil || p == nil || o == nil {
		log.Printf("error: retract called with NULL *Node to RdfGraph: (%s, %s, %s)", s, p, o)
		return false, ErrNilNode
	}
	erased := g.spoGraph.Retract(s, p, o)
	g.posGraph.Erase(p, o, s)
	g.ospGraph.Erase(o, s, p)
	if erased {
		// Notify callback mgr
		g.callbackMgr.TripleDeleted(s, p, o)
	}
	return erased, nil
}
