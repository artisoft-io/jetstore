package rdf

import "iter"

// RdfSession Iterator, the concatenation of the iterators of the session's rdf
// graphs, in the order meta, asserted, inferred. Synchronous, see
// BaseGraphIterator: no goroutine, nothing to release.

type RdfSessionIterator struct {
	itors [3]BaseGraphIterator
}

func NewRdfSessionIterator(metaItor, assertedItor, inferredItor *BaseGraphIterator) *RdfSessionIterator {
	if metaItor == nil || assertedItor == nil || inferredItor == nil {
		return nil
	}
	return &RdfSessionIterator{itors: [3]BaseGraphIterator{*metaItor, *assertedItor, *inferredItor}}
}

// All returns the matching triples, to use as: for t3 := range itor.All() { ... }
func (itor *RdfSessionIterator) All() iter.Seq[[3]*Node] {
	return func(yield func([3]*Node) bool) {
		itor.visit(yield)
	}
}

// IsEmpty returns true when no triple matches. It stops at the first match.
func (itor *RdfSessionIterator) IsEmpty() bool {
	return itor.visit(func([3]*Node) bool { return false })
}

// Cursor returns the pull form of this iterator.
func (itor *RdfSessionIterator) Cursor() TripleCursor {
	return TripleCursor{itors: itor.itors, n: 3}
}

func (itor *RdfSessionIterator) visit(yield func([3]*Node) bool) bool {
	return itor.itors[0].visit(yield) &&
		itor.itors[1].visit(yield) &&
		itor.itors[2].visit(yield)
}
