#ifndef JETS_RDF_CONTAINERS_TYPE_H
#define JETS_RDF_CONTAINERS_TYPE_H

#include <cstddef>
#include <string_view>
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
// --------------------------------------------------------------------------------------
// The map is keyed by Rptr and compared by value. Its hash and equality are transparent,
// so a lookup can be made with the value itself, without allocating an Rptr: with an
// RdfAstType (a number, a date, a blank node), or with an RdfStringKey, which views the
// text of a resource or of a string literal. An Rptr is built only when the value is new.
/////////////////////////////////////////////////////////////////////////////////////////

// The text of a named resource or of a string literal, borrowed for a lookup.
// `which` is rdf_named_resource_t or rdf_literal_string_t.
struct RdfStringKey {
  int which;
  std::string_view text;

  // As rdf_ast_hash_value hashes the std::string it views; absl hashes a std::string
  // and a std::string_view of the same text alike.
  template <typename H>
  friend H AbslHashValue(H h, RdfStringKey const& k) {
    return H::combine(std::move(h), k.text);
  }
};

// An RdfAstType borrowed for hashing, hashed as an Rptr to it would be
struct RdfAstRef {
  RdfAstType const* v;

  template <typename H>
  friend H AbslHashValue(H h, RdfAstRef const& r) {
    return rdf_ast_hash_value(std::move(h), *r.v);
  }
};

struct RdfAstHash {
  using is_transparent = void;
  std::size_t operator()(Rptr const& p) const { return absl::Hash<RdfAstRef>{}(RdfAstRef{p.get()}); }
  std::size_t operator()(RdfAstType const& v) const { return absl::Hash<RdfAstRef>{}(RdfAstRef{&v}); }
  std::size_t operator()(RdfStringKey const& k) const { return absl::Hash<RdfStringKey>{}(k); }
};

struct RdfAstEq {
  using is_transparent = void;
  bool operator()(Rptr const& lhs, Rptr const& rhs) const { return *lhs == *rhs; }
  bool operator()(Rptr const& lhs, RdfAstType const& rhs) const { return *lhs == rhs; }
  bool operator()(RdfAstType const& lhs, Rptr const& rhs) const { return lhs == *rhs; }
  bool operator()(Rptr const& lhs, RdfStringKey const& rhs) const { return equals(*lhs, rhs); }
  bool operator()(RdfStringKey const& lhs, Rptr const& rhs) const { return equals(*rhs, lhs); }

  static bool equals(RdfAstType const& v, RdfStringKey const& k)
  {
    if(v.which() != k.which) return false;
    if(k.which == rdf_named_resource_t) return boost::get<NamedResource>(v).name == k.text;
    return boost::get<LString>(v).data == k.text;
  }
};
using LiteralDataStlMap = std::unordered_map<Rptr, r_index, RdfAstHash, RdfAstEq>;

/////////////////////////////////////////////////////////////////////////////////////////
// ResourceManager Canonical Container Types - used in ResourceManager class
// --------------------------------------------------------------------------------------
using LiteralDataMap = LiteralDataStlMap;

} // namespace jets::rdf
#endif // JETS_RDF_CONTAINERS_TYPE_H
