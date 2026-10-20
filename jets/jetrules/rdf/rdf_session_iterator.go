package rdf

import "iter"

// RdfSession Iterator, it's the concatenation of the iterators of the session's
// rdf graphs, in the order: meta, asserted, inferred.
// Synchronous, see BaseGraphIterator.

type RdfSessionIterator struct {
	metaItor, assertedItor, inferredItor *BaseGraphIterator
}

func NewRdfSessionIterator(metaItor, assertedItor, inferredItor *BaseGraphIterator) *RdfSessionIterator {
	if metaItor == nil || assertedItor == nil || inferredItor == nil {
		return nil
	}
	return &RdfSessionIterator{
		metaItor:     metaItor,
		assertedItor: assertedItor,
		inferredItor: inferredItor,
	}
}

// All returns the matching triples, to use as: for t3 := range itor.All() { ... }
func (itor *RdfSessionIterator) All() iter.Seq[[3]*Node] {
	return func(yield func([3]*Node) bool) {
		_ = itor.metaItor.visit(yield) &&
			itor.assertedItor.visit(yield) &&
			itor.inferredItor.visit(yield)
	}
}

// IsEmpty returns true when no triple matches.
func (itor *RdfSessionIterator) IsEmpty() bool {
	for range itor.All() {
		return false
	}
	return true
}
