package rdf

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

type WSetType = map[*Node]int
type VMapType = map[*Node]WSetType
type UMapType = map[*Node]VMapType

type BaseGraph struct {
	GraphType string
	spin      byte
	size      int
	data      UMapType
}

func NewBaseGraph(graphType string, spin byte) *BaseGraph {
	return &BaseGraph{
		GraphType: graphType,
		spin: spin,
		data: make(UMapType, 100),
	}
}

func (g *BaseGraph) Size() int {
	return g.size
}

func (g *BaseGraph) Clear() {
	g.data = make(UMapType, 100)
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
	vmap := g.data[u]
	if vmap == nil {
		return false
	}
	wmap := vmap[v]
	if wmap == nil {
		return false
	}
	return len(wmap) > 0
}

// returns an Iterator over all the triples in the graph
func (g *BaseGraph) Find() (*BaseGraphIterator, error) {
	return NewBaseGraphIterator(nil, nil, &g.data)
}

// returns an Iterator over the triples (u, *, *) in the graph
func (g *BaseGraph) FindU(u *Node) (*BaseGraphIterator, error) {
	return NewBaseGraphIterator(u, nil, &g.data)
}

// returns an Iterator over the triples (u, v, *) in the graph
func (g *BaseGraph) FindUV(u, v *Node) (*BaseGraphIterator, error) {
	return NewBaseGraphIterator(u, v, &g.data)
}

// Used by `rule_term` to determine if an inferred triple will
// be removed as result of retract call.
// returns the reference count associated with the triple (u, v, w)
func (g *BaseGraph) GetRefCount(u, v, w *Node) int {
	vmap := g.data[u]
	if vmap == nil {
		return 0
	}
	wmap := vmap[v]
	if wmap == nil {
		return 0
	}
	return wmap[w]
}

// Insert triple (u, v, w) into the graph
// Reference count increase by 1 if tripople already in graph
// Returns true if the triple is actually inserted (was not already in the graph)
func (g *BaseGraph) Insert(u, v, w *Node) bool {
	if u == nil || v == nil || w == nil {
		return false
	}
	vmap := g.data[u]
	if vmap == nil {
		vmap = make(VMapType, 20)
		g.data[u] = vmap
	}
	wmap := vmap[v]
	if wmap == nil {
		wmap = make(WSetType)
		vmap[v] = wmap
	}
	c := wmap[w]
	wmap[w] = c + 1
	if c == 0 {
		g.size += 1
	}
	return c == 0
}

// Remove the triple (u, v, w) from the graph.
// Returns true if it's actually removed from the graph
func (g *BaseGraph) Erase(u, v, w *Node) bool {
	if u == nil || v == nil || w == nil {
		return false
	}
	vmap := g.data[u]
	if vmap == nil {
		return false
	}
	wmap := vmap[v]
	if wmap == nil {
		return false
	}
	_, ok := wmap[w]
	if ok {
		delete(wmap, w)
		g.size -= 1
	}
	if len(wmap) == 0 {
		delete(vmap, v)
	}
	if len(vmap) == 0 {
		delete(g.data, u)
	}
	return ok
}

// Retract the triple (u, v, w) from the graph.
// The triple is removed if the reference count becomes zero.
// Returns true if it's actually removed from the graph
func (g *BaseGraph) Retract(u, v, w *Node) bool {
	if u == nil || v == nil || w == nil {
		return false
	}
	vmap := g.data[u]
	if vmap == nil {
		return false
	}
	wmap := vmap[v]
	if wmap == nil {
		return false
	}
	c := wmap[w]
	if c < 2 {
		delete(wmap, w)
		g.size -= 1
		if len(wmap) == 0 {
			delete(vmap, v)
		}
		if len(vmap) == 0 {
			delete(g.data, u)
		}	
	} else {
		wmap[w] = c - 1
	}
	return c < 2	
}
