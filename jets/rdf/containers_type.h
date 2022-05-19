#ifndef JETS_RDF_CONTAINERS_TYPE_H
#define JETS_RDF_CONTAINERS_TYPE_H

#include <unordered_set>
#include <unordered_map>

#include "absl/hash/hash.h"

#include "../rdf/rdf_ast.h"
#include "../rdf/w_node.h"

namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
// base graph container type for u, v, w collections
// STL CONTAINERS -- suffix STL
/////////////////////////////////////////////////////////////////////////////////////////
using WSetStl = std::unordered_set<WNode, absl::Hash<WNode>>;
using VMapStl = std::unordered_map<r_index, WSetStl, absl::Hash<r_index>>;
using UMapStl = std::unordered_map<r_index, VMapStl, absl::Hash<r_index>>;

/////////////////////////////////////////////////////////////////////////////////////////
// BaseGraph Canonical Container Types - these are used in BaseGraph class
// --------------------------------------------------------------------------------------
using WSetType = WSetStl;
using VMapType = VMapStl;
using UMapType = UMapStl;

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
