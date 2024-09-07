package rdf

import "sync"

// Class BaseGraph is an rdf graph
//
// Class to manage a triple graph. The natural indexing to the graph is (u, v, w)
// which is it's natural order. The natural indexing allow to iterate the element
// according to: (u, v, *), (u, *, *), (*, *, *).
// In order to have the complementary indexes, a spin property indicate the maping
// of the indexes:
//
//	's': (u, v, w)  =>  (s, p, o)
//	'p': (u, v, w)  =>  (p, o, s)
//	'o': (u, v, w)  =>  (o, s, p)
//
// The graph structure, representing the triples:
//
//	(u, v, w) implemented as MAP(u, MAP(v, SET(WNode)))

// Using sync.Map ranther than regular map due to race condition
// created by the use of async channels
// type WSetType = map[*Node]int
// type VMapType = map[*Node]WSetType
// type UMapType = map[*Node]VMapType
type WSetType = *sync.Map
type VMapType = *sync.Map
type UMapType = *sync.Map

type BaseGraph struct {
	GraphType string
	spin      byte
	size      int
	data      UMapType
}

func NewBaseGraph(graphType string, spin byte) *BaseGraph {
	return &BaseGraph{
		GraphType: graphType,
		spin:      spin,
		data:      new(sync.Map),
	}
}

func (g *BaseGraph) Size() int {
	return g.size
}

func (g *BaseGraph) Clear() {
	g.data.Clear()
}

// returns true if (u, v, w) exist, false otherwise.
func (g *BaseGraph) Contains(u, v, w *Node) bool {
	return g.GetRefCount(u, v, w) > 0
}

// returns true if (s, p, o) exist with (spo => uvw) mapping, false otherwise.
func (g *BaseGraph) ContainsSPO(s, p, o *Node) bool {
	u, v, w := mapSPO2UVW(g.spin, s, p, o)
	return g.Contains(u, v, w)
}

// returns true if (u, v, *) exist, false otherwise.
func (g *BaseGraph) ContainsUV(u, v *Node) bool {
	vmap, _ := g.data.Load(u)
	if vmap == nil {
		return false
	}
	wmap, _ := vmap.(*sync.Map).Load(v)
	if wmap == nil {
		return false
	}
	m := wmap.(*sync.Map)
	hasValue := false
	m.Range(func(key, value any) bool {
		hasValue = true
		return false
	})
	return hasValue
}

// returns an Iterator over all the triples in the graph
func (g *BaseGraph) Find() *BaseGraphIterator {
	return NewBaseGraphIterator(g.spin, nil, nil, nil, g.data)
}

// returns an Iterator over the triples (u, *, *) in the graph
func (g *BaseGraph) FindU(u *Node) *BaseGraphIterator {
	return NewBaseGraphIterator(g.spin, u, nil, nil, g.data)
}

// returns an Iterator over the triples (u, v, *) in the graph
func (g *BaseGraph) FindUV(u, v *Node) *BaseGraphIterator {
	return NewBaseGraphIterator(g.spin, u, v, nil, g.data)
}

// returns an Iterator over the triples (u, v, w) in the graph
// this iterator will return at most one triple
// This func is for completeness
func (g *BaseGraph) FindUVW(u, v, w *Node) *BaseGraphIterator {
	return NewBaseGraphIterator(g.spin, u, v, w, g.data)
}

// Used by `rule_term` to determine if an inferred triple will
// be removed as result of retract call.
// Returns the reference count associated with the triple (u, v, w).
func (g *BaseGraph) GetRefCount(u, v, w *Node) int {
	vmap, _ := g.data.Load(u)
	if vmap == nil {
		return 0
	}
	wmap, _ := vmap.(*sync.Map).Load(v)
	if wmap == nil {
		return 0
	}
	m := wmap.(*sync.Map)
	c, _ := m.Load(w)
	if c == nil {
		return 0
	}
	return c.(int)
}

// Insert triple (u, v, w) into the graph.
// Reference count increase by 1 if triple already in graph.
// Returns true if the triple is actually inserted (was not already in the graph).
func (g *BaseGraph) Insert(u, v, w *Node) bool {
	if u == nil || v == nil || w == nil {
		return false
	}
	vv, _ := g.data.Load(u)
	if vv == nil {
		vv = new(sync.Map)
		g.data.Store(u, vv)
	}
	vmap := vv.(*sync.Map)
	ww, _ := vmap.Load(v)
	if ww == nil {
		ww = new(sync.Map)
		vmap.Store(v, ww)
	}
	wmap := ww.(*sync.Map)
	var count int
	c, _ := wmap.Load(w)
	if c != nil {
		count = c.(int)
	} else {
		g.size += 1
	}
	count += 1
	wmap.Store(w, count)
	return count == 1
}

// Remove the triple (u, v, w) from the graph.
// Returns true if it's actually removed from the graph.
func (g *BaseGraph) Erase(u, v, w *Node) bool {
	if u == nil || v == nil || w == nil {
		return false
	}
	vmap, _ := g.data.Load((u))
	if vmap == nil {
		return false
	}
	ww, _ := vmap.(*sync.Map).Load(v)
	if ww == nil {
		return false
	}
	wmap := ww.(*sync.Map)
	_, ok := wmap.Load(w)
	if ok {
		wmap.Delete(w)
		g.size -= 1
	}
	return ok
}

// Retract the triple (u, v, w) from the graph.
// The triple is removed if the reference count becomes zero.
// Returns true if it's actually removed from the graph.
func (g *BaseGraph) Retract(u, v, w *Node) bool {
	if u == nil || v == nil || w == nil {
		return false
	}
	vmap, _ := g.data.Load(u)
	if vmap == nil {
		return false
	}
	ww, _ := vmap.(*sync.Map).Load(v)
	if ww == nil {
		return false
	}
	wmap := ww.(*sync.Map)
	cc, _ := wmap.Load(w)
	if cc == nil {
		return false
	}
	c := cc.(int)
	if c < 2 {
		wmap.Delete(w)
		g.size -= 1
	} else {
		wmap.Store(w, c - 1)
	}
	return c < 2
}
