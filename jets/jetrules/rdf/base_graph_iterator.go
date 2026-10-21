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
// Writes during a walk. The loop body may insert into or erase from the graph
// being walked, on the same goroutine. The walk stays well defined and finishes,
// and a triple inserted or erased by the loop body may or may not be returned by
// the walk in progress. The graph's maps are plain Go maps (G2), so this is the
// language's rule for a write during a range on the same goroutine: an entry
// deleted before the walk reaches it is not produced, and an entry added may or
// may not be. It was sync.Map.Range's contract before G2, and it is no weaker.
// An erase that empties an inner map deletes that map too (I-11); a walk already
// inside it finishes on the detached map, which is empty. Which triples a write
// makes visible is not specified, and nothing may depend on it. What the iterator
// does not support is a write from another goroutine during a walk: with plain
// maps that is a data race, which the runtime may report as a fatal error.
//
// The rete engine does write during a walk. Consequents are inserted after a
// visit (ComputeConsequentTriples), but a filter is evaluated inside
// VisitReteGraph's range over FindMatchingTriples, and four operators insert into
// the inferred graph when they are evaluated: lookup and multi_lookup
// (LookupTableSqlite3, lookup_table_sqlite3.go, InsertInferred of the row, its
// columns and the cache entry), range (RangeOp.Eval, expr_operator_r_range.go)
// and create_entity (CreateEntityOp.Eval, expr_operator_r_create_entity.go). A
// rule that used one in an antecedent's filter would write while the iterator is
// live. The four rule workspaces rules_benchmark compiles use them only in
// consequents (checked on their compiled networks, 2026-10-21), but nothing
// prevents the other use (rules_benchmark G13), so the guarantee above is the
// one to keep when the graph's maps change.
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
func (itor *BaseGraphIterator) Cursor() *TripleCursor {
	c := &TripleCursor{n: 1}
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
		for u, vmap := range itor.data {
			if !itor.visitU(u, vmap, yield) {
				return false
			}
		}
		return true
	}
	// Single value for u
	vmap := itor.data[itor.u]
	if vmap == nil {
		return true
	}
	return itor.visitU(itor.u, vmap, yield)
}

// For the subject u, iterate over the predicates (the v's)
func (itor *BaseGraphIterator) visitU(u *Node, vmap VMapType, yield func([3]*Node) bool) bool {
	if itor.v == nil {
		// Iterate over all elements
		for v, wset := range vmap {
			if !itor.visitUV(u, v, wset, yield) {
				return false
			}
		}
		return true
	}
	// Single value for v
	wset := vmap[itor.v]
	if wset == nil {
		return true
	}
	return itor.visitUV(u, itor.v, wset, yield)
}

// For the subject, predicate pair, iterate over the objects (the w's)
func (itor *BaseGraphIterator) visitUV(u, v *Node, wset WSetType, yield func([3]*Node) bool) bool {
	if itor.w == nil {
		// Iterate over all elements
		for w := range wset {
			if !yield(mapUVW2SPOArr(itor.spin, u, v, w)) {
				return false
			}
		}
		return true
	}
	// Single value for w
	if wset[itor.w] > 0 {
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
// It is lazy across graphs and across map entries, and it snapshots one level at
// a time. It descends one map level at a time: a bound key is looked up when the
// cursor gets to it, and an unbound level's keys are copied when the cursor
// enters it. So stopping after the first triple does not read the rest of the
// match beyond the levels already entered, and a session's later graphs are not
// read until the earlier ones are done. It never holds the matches themselves,
// only keys.
//
// The snapshot has two consequences:
//   - memory: entering an unbound level holds that whole level's keys, so it is
//     O(level), not O(1). Find() holds every subject of the graph it is in, and
//     FindSPO(nil, rdf:type, class) every entity of the class, as one slice of
//     pointers;
//   - visibility: a key read into a level is returned even if its triple is
//     erased before the cursor gets to it, and a key inserted into a level after
//     it was read is not returned. Extraction reads the session after the rules
//     have run, so it never writes while a cursor is open.
//
// It runs on the caller's goroutine and holds no resource, so a cursor that is
// dropped before its end needs no release.
//
// Use it through the pointer Cursor returns, and do not copy a TripleCursor: a
// copy made after Next shares its key buffers with the original, so advancing
// either corrupts the other.
type TripleCursor struct {
	itors [3]BaseGraphIterator
	// n iterators; the current one is itors[gi-1]
	n, gi int

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
			c.value = mapUVW2SPOArr(c.itors[c.gi-1].spin, c.u, c.v, w)
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
			c.gi++
			c.enterGraph()

		default:
			c.u, c.v, c.vmap = nil, nil, nil
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
	it := &c.itors[c.gi-1]
	switch {
	case it.data == nil:
	case it.u != nil:
		c.uOne[0] = it.u
		c.us = c.uOne[:]
	default:
		for u := range it.data {
			c.us = append(c.us, u)
		}
	}
}

// enterU reads the v level under c.u.
func (c *TripleCursor) enterU() {
	c.vs, c.vi = c.vs[:0], 0
	c.ws, c.wi = c.ws[:0], 0
	c.vmap = c.itors[c.gi-1].data[c.u]
	if c.vmap == nil {
		return
	}
	if v := c.itors[c.gi-1].v; v != nil {
		c.vOne[0] = v
		c.vs = c.vOne[:]
		return
	}
	for v := range c.vmap {
		c.vs = append(c.vs, v)
	}
}

// enterUV reads the w level under (c.u, c.v).
func (c *TripleCursor) enterUV() {
	c.ws, c.wi = c.ws[:0], 0
	wset := c.vmap[c.v]
	if wset == nil {
		return
	}
	if w := c.itors[c.gi-1].w; w != nil {
		if wset[w] > 0 {
			c.wOne[0] = w
			c.ws = c.wOne[:]
		}
		return
	}
	for w := range wset {
		c.ws = append(c.ws, w)
	}
}
