package rdf

import (
	"errors"
	"fmt"
	"log"
	"sort"
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

func (g *RdfGraph) Contains(s, p, o *Node) bool {
	return g.spoGraph.Contains(s, p, o)
}

func (g *RdfGraph) ContainsSP(s, p *Node) bool {
	return g.spoGraph.ContainsUV(s, p)
}

func (rs *RdfGraph) GetObject(s, p *Node) *Node {
	itor := rs.FindSP(s, p)
	defer itor.Done()
	for t3 := range itor.Itor {
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
		g.CallbackMgr.TripleInserted(s, p, o)
	}
	return inserted, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) erase_internal(s, p, o *Node) (bool, error) {
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
		g.CallbackMgr.TripleDeleted(s, p, o)
	}
	return erased, nil
}

// Return true if the triple is actually deleted from the RdfGraph.
func (g *RdfGraph) Erase(s, p, o *Node) (bool, error) {
	if g.isLocked {
		return false, ErrGraphLocked
	}
	if s != nil && p != nil && o != nil {
		return g.erase_internal(s, p, o)
	}
	var erased bool
	l := make([]*[3]*Node, 0)
	t3Itor := g.FindSPO(s, p, o)
	for t3 := range t3Itor.Itor {
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
		g.CallbackMgr.TripleDeleted(s, p, o)
	}
	return erased, nil
}

func (g *RdfGraph) ToTriples() []string {
	triples := make([]string, 0)
	t3Itor := g.Find()
	for t3 := range t3Itor.Itor {
		triples = append(triples, fmt.Sprintf("(%s, %s, %s)", t3[0], t3[1], t3[2]))
	}
	t3Itor.Done()
	sort.Slice(triples, func(i, j int) bool { return triples[i] < triples[j] })
	return triples
}
