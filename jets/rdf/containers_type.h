#ifndef JETS_RDF_CONTAINERS_TYPE_H
#define JETS_RDF_CONTAINERS_TYPE_H

#include <unordered_set>
#include <unordered_map>

#include "absl/container/flat_hash_map.h"
#include "absl/container/flat_hash_set.h"
#include "absl/hash/hash.h"

#include "../rdf/rdf_ast.h"
#include "../rdf/w_node.h"

namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
// BaseGraph Canonical Container Types - these are used in BaseGraph class
// --------------------------------------------------------------------------------------
// The graph index MAP(u, MAP(v, SET(w))) is held in open-addressing (absl "swiss")
// tables. They store their elements inline, so a lookup touches one array rather
// than a chain of nodes, and a set of one element -- the common case for (s, p) in
// the spo index -- is stored inside the set object and allocates nothing.
//
// What the graph relies on, and what it does not:
//  - No pointer or reference to an element is kept beyond one BaseGraph call, so
//    the elements need no pointer stability and node_hash_* is not needed.
//  - Iterators into these tables ARE kept, by BaseGraphIterator, across calls, and
//    any insert can rehash a table and invalidate them. BaseGraph therefore freezes
//    its live iterators before every change of structure (an element inserted or
//    removed, an inner table created or erased): see BaseGraph::freeze_iterators and
//    BaseGraphIterator::freeze. A reference-count change is not a change of structure.
//  - Iteration order is unspecified, as it was with std::unordered_*: it depends on
//    the r_index addresses and on absl::Hash's per-process seed.
using WSetType = absl::flat_hash_set<WNode>;
using VMapType = absl::flat_hash_map<r_index, WSetType>;
using UMapType = absl::flat_hash_map<r_index, VMapType>;

/////////////////////////////////////////////////////////////////////////////////////////
// ResourceManager Container for all literal data => r_index mapping
// STL CONTAINERS -- suffix STL
/////////////////////////////////////////////////////////////////////////////////////////
struct RdfAstEq {
  bool operator()( const Rptr& lhs, const Rptr& rhs ) const
  {
    return *lhs == *rhs;
  }
};
using LiteralDataStlMap = std::unordered_map<Rptr, r_index, absl::Hash<Rptr>, RdfAstEq>;

/////////////////////////////////////////////////////////////////////////////////////////
// ResourceManager Canonical Container Types - used in ResourceManager class
// --------------------------------------------------------------------------------------
using LiteralDataMap = LiteralDataStlMap;

} // namespace jets::rdf
#endif // JETS_RDF_CONTAINERS_TYPE_H
