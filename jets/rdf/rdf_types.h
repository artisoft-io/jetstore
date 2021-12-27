#ifndef JETS_RDF_STL_TYPES_H
#define JETS_RDF_STL_TYPES_H

#include <string>
#include <memory>
#include <unordered_set>
#include <unordered_map>

#include "absl/hash/hash.h"

#include "jets/rdf/rdf_ast.h"
#include "jets/rdf/base_graph.h"
#include "jets/rdf/graph_callback_mgr.h"
#include "jets/rdf/base_graph_iterator.h"
#include "jets/rdf/r_manager.h"
#include "jets/rdf/rdf_graph.h"
#include "jets/rdf/rdf_session_iterator.h"
#include "jets/rdf/rdf_session.h"

namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
// base graph container type for u, v, w collections
// STL CONTAINERS -- suffix STL
/////////////////////////////////////////////////////////////////////////////////////////
using WSetStl = std::unordered_set<WNode<r_index>, absl::Hash<WNode<r_index>>>;
using VMapStl = std::unordered_map<r_index, WSetStl, absl::Hash<r_index>>;
using UMapStl = std::unordered_map<r_index, VMapStl, absl::Hash<r_index>>;

/////////////////////////////////////////////////////////////////////////////////////////
// Define some BaseGraph configuration
using BaseGraphStlImpl = BaseGraph<UMapStl, VMapStl, WSetStl, BaseGraphIterator<UMapStl, VMapStl, WSetStl>>;
using BaseGraphStlPtr = std::shared_ptr<BaseGraphStlImpl>;

// Graph Factory constructor
BaseGraphStlPtr create_stl_base_graph(char const spin);
/////////////////////////////////////////////////////////////////////////////////////////
// Container for all literal data => r_index mapping
// STL CONTAINERS -- suffix STL
/////////////////////////////////////////////////////////////////////////////////////////
using LD2RIndexMap = std::unordered_map<Rptr, r_index, absl::Hash<Rptr>>;

/////////////////////////////////////////////////////////////////////////////////////////
// RManager -- with STL containers
using RManagerStlImpl = RManager<LD2RIndexMap>;
using RManagerStlPtr = RManagerPtr<LD2RIndexMap>;

/////////////////////////////////////////////////////////////////////////////////////////
// RDFGraph  -- with STL containers -- suffix STL
using RDFGraphStlImpl = RDFGraph<BaseGraphStlImpl, RManagerStlImpl>;
using RDFGraphStlPtr = RDFGraphPtr<BaseGraphStlImpl, RManagerStlImpl>;
RDFGraphStlPtr create_stl_rdf_graph(RManagerStlPtr meta_mgr=RManagerStlPtr());

/////////////////////////////////////////////////////////////////////////////////////////
// RDFSession  -- with STL containers -- suffix STL
using RDFSessionStlImpl = RDFSession<RDFGraphStlImpl>;
using RDFSessionStlPtr = RDFSessionPtr<RDFGraphStlImpl>;
RDFSessionStlPtr create_stl_rdf_session(RDFGraphStlPtr meta_mgr=RDFGraphStlPtr());

} // namespace jets::rdf
#endif // JETS_RDF_STL_TYPES_H
