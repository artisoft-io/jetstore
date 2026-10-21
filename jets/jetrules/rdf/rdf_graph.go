package rdf

import (
	"slices"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
)

// RDFGraph is a fully indexed rdf graph with type (*Node, *Node, *Node)
// It is made of 3 BaseGraph, each with a different spin: 's'po, 'p'os, 'o'sp

var ErrGraphLocked = errors.New("error: RdfGraph is locked, cannot mutate")
var ErrNilNode = errors.New("error: cannot insert/erase/retract triple with nil *Node in RdfGraph")

type RdfGraph struct {
	// isLocked is set by Lock, which every NewRdfSession calls on the meta graph that
	// all the sessions of a pool share, so it is atomic (rules_benchmark I-5).
	isLocked    atomic.Bool
	spoGraph    *BaseGraph
	posGraph    *BaseGraph
	ospGraph    *BaseGraph
	CallbackMgr *CallbackManager
	RootRm      *ResourceManager
}

func NewRdfGraph(graphType string) *RdfGraph {
	return &RdfGraph{
		spoGraph:    NewBaseGraph(graphType, 's'),
		posGraph:    NewBaseGraph(graphType, 'p'),
		ospGraph:    NewBaseGraph(graphType, 'o'),
		CallbackMgr: NewCallbackManager(),
	}
}

func NewMetaRdfGraph(rootRm *ResourceManager) *RdfGraph {
	return &RdfGraph{
		spoGraph:    NewBaseGraph("META", 's'),
		posGraph:    NewBaseGraph("META", 'p'),
		ospGraph:    NewBaseGraph("META", 'o'),
		CallbackMgr: NewCallbackManager(),
		RootRm:      rootRm,
	}
}

func (g *RdfGraph) Size() int {
	return g.spoGraph.Size()
}

func (g *RdfGraph) IsLocked() bool {
	return g.isLocked.Load()
}

// Lock makes the graph read-only. It is safe to call from concurrent sessions, and
// writes only the first time, so that sessions sharing a meta graph do not all write
// to one cache line.
func (g *RdfGraph) Lock() {
	if !g.isLocked.Load() {
		g.isLocked.Store(true)
	}
}

func (g *RdfGraph) Contains(s, p, o *Node) bool {
	return g.spoGraph.Contains(s, p, o)
}

// GetRefCount returns the reference count of triple (s, p, o), 0 when it is not in
// the graph.
func (g *RdfGraph) GetRefCount(s, p, o *Node) int {
	return g.spoGraph.GetRefCount(s, p, o)
}

func (g *RdfGraph) ContainsSP(s, p *Node) bool {
	return g.spoGraph.ContainsUV(s, p)
}

// GetObject returns the object of a triple (s, p, ?o), nil if there is none. When
// there are several, which one is returned is not defined.
func (g *RdfGraph) GetObject(s, p *Node) *Node {
	itor := g.spoGraph.iterator(s, p, nil)
	for t3 := range itor.All() {
		return t3[2]
	}
	return nil
}

func (g *RdfGraph) Find() *BaseGraphIterator {
	return g.spoGraph.Find()
}

func (g *RdfGraph) FindSP(s, p *Node) *BaseGraphIterator {
	return g.spoGraph.FindUV(s, p)
}

func (g *RdfGraph) FindS(s *Node) *BaseGraphIterator {
	return g.spoGraph.FindU(s)
}

func (g *RdfGraph) FindSPO(s, p, o *Node) *BaseGraphIterator {
	itor := g.findSPO(s, p, o)
	return &itor
}

// findSPO returns the iterator over (s, p, o), a nil node matching any, by value,
// on the index whose bound keys come first.
func (g *RdfGraph) findSPO(s, p, o *Node) BaseGraphIterator {
	switch {
	case s == nil && p == nil && o == nil:
		// case (*, *, *)
		return g.spoGraph.iterator(nil, nil, nil)

	case s != nil && p == nil && o == nil:
		// case (s, *, *)
		return g.spoGraph.iterator(s, nil, nil)

	case s != nil && p != nil && o == nil:
		// case (s, p, *)
		return g.spoGraph.iterator(s, p, nil)

	case s != nil && p != nil && o != nil:
		// case (s, p, o)
		return g.spoGraph.iterator(s, p, o)

	case s == nil && p != nil && o == nil:
		// case (*, p, *)
		return g.posGraph.iterator(p, nil, nil)

	case s == nil && p != nil && o != nil:
		// case (*, p, o)
		return g.posGraph.iterator(p, o, nil)

	case s == nil && p == nil && o != nil:
		// case (*, *, o)
		return g.ospGraph.iterator(o, nil, nil)

	default:
		// case (s, *, o)
		return g.ospGraph.iterator(o, s, nil)
	}
}

// Returns true if the triple is actually inserted (was not present in graph).
// Otherwise the reference count is increased by 1.
func (g *RdfGraph) Insert(s, p, o *Node) (bool, error) {
	if g.isLocked.Load() {
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
		g.CallbackMgr.TripleInserted(s, p, o)
	}
	return inserted, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) erase_internal(s, p, o *Node) (bool, error) {
	if g.isLocked.Load() {
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
		g.CallbackMgr.TripleDeleted(s, p, o)
	}
	return erased, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) Erase(s, p, o *Node) (bool, error) {
	if g.isLocked.Load() {
		return false, ErrGraphLocked
	}
	if s != nil && p != nil && o != nil {
		return g.erase_internal(s, p, o)
	}
	var erased bool
	l := make([]*[3]*Node, 0)
	t3Itor := g.findSPO(s, p, o)
	for t3 := range t3Itor.All() {
		l = append(l, &t3)
	}
	for _, t3 := range l {
		b, err := g.erase_internal((*t3)[0], (*t3)[1], (*t3)[2])
		if err != nil {
			return false, err
		}
		erased = erased || b
	}
	return erased, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) Retract(s, p, o *Node) (bool, error) {
	if g.isLocked.Load() {
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
		g.CallbackMgr.TripleDeleted(s, p, o)
	}
	return erased, nil
}

func (g *RdfGraph) ToTriples() []string {
	triples := make([]string, 0)
	t3Itor := g.Find()
	for t3 := range t3Itor.All() {
		triples = append(triples, fmt.Sprintf("(%s, %s, %s)", t3[0], t3[1], t3[2]))
	}
	slices.Sort(triples)
	return triples
}
