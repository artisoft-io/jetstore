package rdf

import "iter"

// BaseGraph Iterator, synchronous
//
// The iterator runs on the caller's goroutine. All walks the graph's maps and
// calls the loop body once per matching triple; there are no goroutines and no
// channels, so there is nothing to release: a `break` in the loop body ends the
// walk, and an iterator that is never read costs nothing.
//
// It replaces an iterator of three goroutines per graph joined by unbuffered
// channels (rules_benchmark G1). That one leaked goroutines when it was closed
// before it was read (I-1), and its consumers could not tell "no triple" from
// "no triple yet" without blocking (I-2).
//
// A triple inserted or erased by the loop body, in the graph being walked, may
// or may not be returned by the walk in progress (the semantics of
// sync.Map.Range). The rete engine does not rely on either: consequents are
// inserted after a visit, not during it.
//
// For a caller that cannot be written as a range loop, Cursor gives the pull form.

type BaseGraphIterator struct {
	spin    byte
	u, v, w *Node
	// data is nil when the iterator matches nothing by construction, such as the
	// meta graph's when it is empty (see RdfSession.FindSPO).
	data UMapType
}

// Iterator over the BaseGraph, u, v, w can be nil
// The iterator returns the triple in (s, p, o) order based on the spin of the graph
func NewBaseGraphIterator(spin byte, u, v, w *Node, g UMapType) *BaseGraphIterator {
	return &BaseGraphIterator{spin: spin, u: u, v: v, w: w, data: g}
}

// All returns the matching triples, to use as: for t3 := range itor.All() { ... }
func (itor *BaseGraphIterator) All() iter.Seq[[3]*Node] {
	return func(yield func([3]*Node) bool) {
		itor.visit(yield)
	}
}

// IsEmpty returns true when no triple matches. It stops at the first match.
func (itor *BaseGraphIterator) IsEmpty() bool {
	return itor.visit(func([3]*Node) bool { return false })
}

// Cursor returns the pull form of this iterator.
func (itor *BaseGraphIterator) Cursor() TripleCursor {
	c := TripleCursor{n: 1}
	c.itors[0] = *itor
	return c
}

// visit calls yield for each matching triple. It returns false if yield returned
// false, so that the caller stops too.
func (itor *BaseGraphIterator) visit(yield func([3]*Node) bool) bool {
	if itor.data == nil {
		return true
	}
	if itor.u == nil {
		// Iterate over all elements
		cont := true
		itor.data.Range(func(node, data any) bool {
			cont = itor.visitU(node.(*Node), data.(VMapType), yield)
			return cont
		})
		return cont
	}
	// Single value for u
	data, _ := itor.data.Load(itor.u)
	if data == nil {
		return true
	}
	return itor.visitU(itor.u, data.(VMapType), yield)
}

// For the subject u, iterate over the predicates (the v's)
func (itor *BaseGraphIterator) visitU(u *Node, vmap VMapType, yield func([3]*Node) bool) bool {
	if itor.v == nil {
		// Iterate over all elements
		cont := true
		vmap.Range(func(node, data any) bool {
			cont = itor.visitUV(u, node.(*Node), data.(WSetType), yield)
			return cont
		})
		return cont
	}
	// Single value for v
	data, _ := vmap.Load(itor.v)
	if data == nil {
		return true
	}
	return itor.visitUV(u, itor.v, data.(WSetType), yield)
}

// For the subject, predicate pair, iterate over the objects (the w's)
func (itor *BaseGraphIterator) visitUV(u, v *Node, wset WSetType, yield func([3]*Node) bool) bool {
	if itor.w == nil {
		// Iterate over all elements
		cont := true
		wset.Range(func(node, _ any) bool {
			cont = yield(mapUVW2SPOArr(itor.spin, u, v, node.(*Node)))
			return cont
		})
		return cont
	}
	// Single value for w
	c, _ := wset.Load(itor.w)
	if c != nil && c.(int) > 0 {
		return yield(mapUVW2SPOArr(itor.spin, u, v, itor.w))
	}
	return true
}

// TripleCursor is the pull form of BaseGraphIterator and RdfSessionIterator, for
// a caller that reads one triple at a time rather than in a range loop: the
// compute_pipes adaptor's TripleIterator, which extraction uses.
//
//	c := itor.Cursor()
//	for c.Next() {
//		t3 := c.Value()
//	}
//
// It is lazy. It descends one map level at a time and holds, at each level, the
// keys of the one map it is inside: a bound key is looked up when the cursor gets
// to it, and an unbound level is read when the cursor enters it. So stopping
// after the first triple costs the first triple, not the whole match, and a
// session's later graphs are not read at all. It never holds the matches
// themselves, only keys, at most one level's worth per level.
//
// It runs on the caller's goroutine and holds no resource, so a cursor that is
// dropped before its end needs no release.
//
// A key read into a level is returned even if its triple is erased before the
// cursor gets to it, and a key inserted into a level after it was read is not
// returned. Extraction reads the session after the rules have run, so it never
// writes while a cursor is open.
type TripleCursor struct {
	itors [3]BaseGraphIterator
	n, gi int
	it    *BaseGraphIterator

	u, v       *Node
	vmap       VMapType
	us, vs, ws []*Node
	ui, vi, wi int
	// one key per level, so a bound key needs no allocation
	uOne, vOne, wOne [1]*Node

	value [3]*Node
}

// Next moves to the next triple and returns true, or returns false at the end.
func (c *TripleCursor) Next() bool {
	for {
		switch {
		case c.wi < len(c.ws):
			w := c.ws[c.wi]
			c.wi++
			c.value = mapUVW2SPOArr(c.it.spin, c.u, c.v, w)
			return true

		case c.vi < len(c.vs):
			c.v = c.vs[c.vi]
			c.vi++
			c.enterUV()

		case c.ui < len(c.us):
			c.u = c.us[c.ui]
			c.ui++
			c.enterU()

		case c.gi < c.n:
			c.it = &c.itors[c.gi]
			c.gi++
			c.enterGraph()

		default:
			c.it, c.u, c.v, c.vmap = nil, nil, nil, nil
			c.us, c.vs, c.ws = nil, nil, nil
			return false
		}
	}
}

// Value returns the current triple, after Next returned true.
func (c *TripleCursor) Value() [3]*Node {
	return c.value
}

// enterGraph reads the u level of the current iterator.
func (c *TripleCursor) enterGraph() {
	c.us, c.ui = c.us[:0], 0
	c.vs, c.vi = c.vs[:0], 0
	c.ws, c.wi = c.ws[:0], 0
	it := c.it
	switch {
	case it.data == nil:
	case it.u != nil:
		c.uOne[0] = it.u
		c.us = c.uOne[:]
	default:
		it.data.Range(func(node, _ any) bool {
			c.us = append(c.us, node.(*Node))
			return true
		})
	}
}

// enterU reads the v level under c.u.
func (c *TripleCursor) enterU() {
	c.vs, c.vi = c.vs[:0], 0
	c.ws, c.wi = c.ws[:0], 0
	data, _ := c.it.data.Load(c.u)
	if data == nil {
		c.vmap = nil
		return
	}
	c.vmap = data.(VMapType)
	if c.it.v != nil {
		c.vOne[0] = c.it.v
		c.vs = c.vOne[:]
		return
	}
	c.vmap.Range(func(node, _ any) bool {
		c.vs = append(c.vs, node.(*Node))
		return true
	})
}

// enterUV reads the w level under (c.u, c.v).
func (c *TripleCursor) enterUV() {
	c.ws, c.wi = c.ws[:0], 0
	data, _ := c.vmap.Load(c.v)
	if data == nil {
		return
	}
	wset := data.(WSetType)
	if c.it.w != nil {
		if n, _ := wset.Load(c.it.w); n != nil && n.(int) > 0 {
			c.wOne[0] = c.it.w
			c.ws = c.wOne[:]
		}
		return
	}
	wset.Range(func(node, _ any) bool {
		c.ws = append(c.ws, node.(*Node))
		return true
	})
}
