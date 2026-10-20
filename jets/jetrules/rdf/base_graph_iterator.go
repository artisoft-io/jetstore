package rdf

import (
	"iter"
	"sync"
)

// BaseGraph Iterator, synchronous
//
// The iterator runs on the caller's goroutine: All walks the graph's maps with
// Range and calls the loop body once per matching triple. There are no
// goroutines and no channels, so there is nothing to release when the caller
// stops early: a `break` in the loop body ends the walk.
//
// A triple inserted or erased by the loop body, in the graph being walked, may
// or may not be returned by the walk in progress (the semantics of
// sync.Map.Range).

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
		cont := true
		itor.data.Range(func(node, data any) bool {
			cont = itor.visitU(node.(*Node), data.(*sync.Map), yield)
			return cont
		})
		return cont
	}
	// Single value for u
	data, _ := itor.data.Load(itor.u)
	if data == nil {
		return true
	}
	return itor.visitU(itor.u, data.(*sync.Map), yield)
}

// For the subject u, iterate over the predicates (the v's)
func (itor *BaseGraphIterator) visitU(u *Node, vmap VMapType, yield func([3]*Node) bool) bool {
	if itor.v == nil {
		// Iterate over all elements
		cont := true
		vmap.Range(func(node, data any) bool {
			cont = itor.visitUV(u, node.(*Node), data.(*sync.Map), yield)
			return cont
		})
		return cont
	}
	// Single value for v
	data, _ := vmap.Load(itor.v)
	if data == nil {
		return true
	}
	return itor.visitUV(u, itor.v, data.(*sync.Map), yield)
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
