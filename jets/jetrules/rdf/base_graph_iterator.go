package rdf

import "iter"

// BaseGraph Iterator, synchronous
//
// The iterator runs on the caller's goroutine: All walks the graph's maps with
// range and calls the loop body once per matching triple. There are no
// goroutines and no channels, so there is nothing to release when the caller
// stops early: a `break` in the loop body ends the walk.
//
// The graph's maps are plain maps, so a write made by the loop body to the
// graph being walked has the semantics Go defines for a write to a map during
// a range on the same goroutine: an entry erased before it is reached is not
// returned, an entry inserted may or may not be returned, and a reference
// count change does not affect the walk, which only reads the keys.

type BaseGraphIterator struct {
	spin    byte
	u, v, w *Node
	data    UMapType
}

// Iterator over the BaseGraph, u, v, w can be nil
// The iterator returns the triple in (s, p, o) order based on the spin of the graph
func NewBaseGraphIterator(spin byte, u, v, w *Node, g UMapType) *BaseGraphIterator {
	return &BaseGraphIterator{
		spin: spin,
		u:    u,
		v:    v,
		w:    w,
		data: g,
	}
}

// All returns the matching triples, to use as: for t3 := range itor.All() { ... }
func (itor *BaseGraphIterator) All() iter.Seq[[3]*Node] {
	return func(yield func([3]*Node) bool) {
		itor.visit(yield)
	}
}

// IsEmpty returns true when no triple matches.
func (itor *BaseGraphIterator) IsEmpty() bool {
	for range itor.All() {
		return false
	}
	return true
}

// visit calls yield for each matching triple, returns false if yield returned false.
func (itor *BaseGraphIterator) visit(yield func([3]*Node) bool) bool {
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
