package rdf

import (
	"errors"
	"log"
)

// RdfSession is the working memory used by the rule engine
// rdf session is composed of 3 rdf graphs:
//    - meta graph that is read-only and shared among sessions
//    - asserted graph containing the triples asserted from the input source.
//    - inferred graph containing the inferred triples from rule engine.

var ErrNilNodeRdfSession = errors.New("error: cannot insert/erase/retract triple with nil *Node in RdfSession")

type RdfSession struct {
	ResourceMgr   *ResourceManager
	MetaGraph     *RdfGraph
	AssertedGraph *RdfGraph
	InferredGraph *RdfGraph
}

func NewRdfSession(rootRm *ResourceManager, metaGraph *RdfGraph) *RdfSession {
	if rootRm == nil || metaGraph == nil {
		return nil
	}
	metaGraph.isLocked = true
	return &RdfSession{
		ResourceMgr:   NewResourceManager(rootRm),
		MetaGraph:     metaGraph,
		AssertedGraph: NewRdfGraph("ASSERTED"),
		InferredGraph: NewRdfGraph("INFERRED"),
	}
}

func (rs *RdfSession) Size() int {
	return rs.MetaGraph.Size() + rs.AssertedGraph.Size() + rs.InferredGraph.Size()
}

func (rs *RdfSession) Contains(s, p, o *Node) bool {
	return rs.MetaGraph.Contains(s, p, o) || rs.AssertedGraph.Contains(s, p, o) || 
		rs.InferredGraph.Contains(s, p, o)
}

func (rs *RdfSession) ContainsSP(s, p *Node) bool {
	return rs.MetaGraph.ContainsSP(s, p) || rs.AssertedGraph.ContainsSP(s, p) || 
		rs.InferredGraph.ContainsSP(s, p)
}

func (rs *RdfSession) GetObject(s, p *Node) *Node {
	itor := rs.FindSP(s, p)
	defer itor.Done()
	for t3 := range itor.Itor {
		return t3[2]
	}
	return nil
}

// Asserting a triple to rdf session, returns true if actually inserted
func (rs *RdfSession) Insert(s, p, o *Node) (bool, error) {
	if s == nil || p == nil || o == nil {
		log.Printf("error: insert called with NULL *Node to RdfSession: (%s, %s, %s)", s, p, o)
		return false, ErrNilNodeRdfSession
	}
	if rs.MetaGraph.Contains(s, p, o) {
		return false, nil
	}
	var b bool
	var err error
	if _, err = rs.InferredGraph.erase_internal(s, p, o); err != nil {
		return false, err
	}
	if b, err = rs.AssertedGraph.Insert(s, p, o); err != nil {
		return false, err
	}
	return b, nil
}

// Insert an inferred triple to rdf session, returns true if actually inserted
func (rs *RdfSession) InsertInferred(s, p, o *Node) (bool, error) {
	if s == nil || p == nil {
		log.Printf("error: InsertInferred called with NULL *Node to RdfSession: (%s, %s, %s)", s, p, o)
		return false, ErrNilNodeRdfSession
	}
	if o == nil {
		o = Null()
	}
	if rs.MetaGraph.Contains(s, p, o) || rs.AssertedGraph.Contains(s, p, o) {
		return false, nil
	}
	// //DEV**
	// log.Printf("InsertInferred: %s", ToString(&[3]*Node{s, p, o}))
	return rs.InferredGraph.Insert(s, p, o)
}

// Delete triple from rdf session
func (rs *RdfSession) Erase(s, p, o *Node) (bool, error) {
	var b1, b2 bool
	var err error
	if b1, err = rs.AssertedGraph.Erase(s, p, o); err != nil {
		return false, err
	}
	if b2, err = rs.InferredGraph.Erase(s, p, o); err != nil {
		return false, err
	}
	return b1 || b2, nil
}

// Retract triple from inferred graph
func (rs *RdfSession) Retract(s, p, o *Node) (bool, error) {
	if s == nil || p == nil || o == nil {
		log.Printf("error: Retract called with NULL *Node to RdfSession: (%s, %s, %s)", s, p, o)
		return false, ErrNilNodeRdfSession
	}
	// //DEV**
	// log.Printf("Retract: %s", ToString(&[3]*Node{s, p, o}))
	return rs.InferredGraph.Retract(s, p, o)
}

func (g *RdfSession) Find() *RdfSessionIterator {
	return NewRdfSessionIterator(g.MetaGraph.Find(), g.AssertedGraph.Find(), g.InferredGraph.Find())
}

func (g *RdfSession) FindS(s *Node) *RdfSessionIterator {
	return NewRdfSessionIterator(g.MetaGraph.FindS(s), g.AssertedGraph.FindS(s), g.InferredGraph.FindS(s))
}

func (g *RdfSession) FindSP(s, p *Node) *RdfSessionIterator {
	return NewRdfSessionIterator(g.MetaGraph.FindSP(s, p), g.AssertedGraph.FindSP(s, p), g.InferredGraph.FindSP(s, p))
}

func (g *RdfSession) FindSPO(s, p, o *Node) *RdfSessionIterator {
	// //**
	// log.Printf("RdfSession.FindSPO(%s, %s, %s)", s, p, o)
	return NewRdfSessionIterator(g.MetaGraph.FindSPO(s, p, o), g.AssertedGraph.FindSPO(s, p, o), g.InferredGraph.FindSPO(s, p, o))
}
