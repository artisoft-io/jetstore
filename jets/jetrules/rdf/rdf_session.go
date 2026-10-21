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
	metaGraph.Lock()
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

// GetObject returns the object of a triple (s, p, ?o), nil if there is none,
// looking in the meta, asserted and inferred graphs in that order.
func (rs *RdfSession) GetObject(s, p *Node) *Node {
	if o := rs.MetaGraph.GetObject(s, p); o != nil {
		return o
	}
	if o := rs.AssertedGraph.GetObject(s, p); o != nil {
		return o
	}
	return rs.InferredGraph.GetObject(s, p)
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
	return g.FindSPO(nil, nil, nil)
}

func (g *RdfSession) FindS(s *Node) *RdfSessionIterator {
	itor := &RdfSessionIterator{}
	if g.MetaGraph.Size() > 0 {
		itor.itors[0] = g.MetaGraph.spoGraph.iterator(s, nil, nil)
	}
	itor.itors[1] = g.AssertedGraph.spoGraph.iterator(s, nil, nil)
	itor.itors[2] = g.InferredGraph.spoGraph.iterator(s, nil, nil)
	return itor
}

// FindSP reads the spo index, also when s is nil, as RdfGraph.FindSP does.
func (g *RdfSession) FindSP(s, p *Node) *RdfSessionIterator {
	itor := &RdfSessionIterator{}
	if g.MetaGraph.Size() > 0 {
		itor.itors[0] = g.MetaGraph.spoGraph.iterator(s, p, nil)
	}
	itor.itors[1] = g.AssertedGraph.spoGraph.iterator(s, p, nil)
	itor.itors[2] = g.InferredGraph.spoGraph.iterator(s, p, nil)
	return itor
}

// FindSPO returns an iterator over the triples (s, p, o) of the three graphs, a nil
// node matching any.
//
// The meta graph is skipped when it is empty. Otherwise a bound first key costs it
// one map lookup, which is what an explicit skip would cost too.
func (g *RdfSession) FindSPO(s, p, o *Node) *RdfSessionIterator {
	itor := &RdfSessionIterator{}
	if g.MetaGraph.Size() > 0 {
		itor.itors[0] = g.MetaGraph.findSPO(s, p, o)
	}
	itor.itors[1] = g.AssertedGraph.findSPO(s, p, o)
	itor.itors[2] = g.InferredGraph.findSPO(s, p, o)
	return itor
}
