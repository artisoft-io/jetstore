#ifndef JETS_RDF_GRAPH_CALLBACK_MGR_H
#define JETS_RDF_GRAPH_CALLBACK_MGR_H

#include <string>
#include <memory>
#include <list>

#include "jets/rdf/base_graph.h"
#include "jets/rdf/rdf_ast.h"

// Component to manage list of call backs to notify when triples are added or removed
// from BaseGraph. This is an abstract base class, the implementation class is
// defined in the rete package.
namespace jets::rdf {
// //////////////////////////////////////////////////////////////////////////////////////
// GraphCallbackManager class -- main class for managing callback functions on BaseGraph
// --------------------------------------------------------------------------------------
class GraphCallbackManager;
using GraphCallbackManagerPtr = std::shared_ptr<GraphCallbackManager>;

// BetaRelation making the rete network
class GraphCallbackManager {
 public:
  GraphCallbackManager() = default;
  virtual ~GraphCallbackManager() {}

  virtual void
  triple_inserted(r_index u, r_index v, r_index w)const=0;

  virtual void
  triple_deleted(r_index u, r_index v, r_index w)const=0;

 private:
};

} // namespace jets::rdf
#endif // JETS_RDF_GRAPH_CALLBACK_MGR_H
