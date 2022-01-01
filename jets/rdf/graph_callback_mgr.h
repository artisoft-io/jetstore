#ifndef JETS_RDF_GRAPH_CALLBACK_MGR_H
#define JETS_RDF_GRAPH_CALLBACK_MGR_H

#include <string>
#include <memory>
#include <list>

#include "jets/rdf/rdf_ast.h"

// Component to manage list of call backs to notify when triples are added or removed
// from BaseGraph. This is an abstract base class, the implementation class is
// defined in the rete package.
namespace jets::rdf {
// //////////////////////////////////////////////////////////////////////////////////////
// ReteCallBack - virtual base class for a alpha node callback from the inferred graph
// --------------------------------------------------------------------------------------
class ReteCallBack {
 public:
  ReteCallBack() {};
  virtual ~ReteCallBack() {};

  ReteCallBack(ReteCallBack const& rhs) = default;
  ReteCallBack(ReteCallBack && rhs) = default;
  ReteCallBack& operator=(ReteCallBack const&) = default;

  // Implemented by rete::ReteCallBackImpl
  virtual void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const=0;

  // Implemented by rete::ReteCallBackImpl
  virtual void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const=0;
};

using ReteCallBackPtr = std::shared_ptr<ReteCallBack>;

// ReteCallBackList: Container for ReteCallBackImpl
using ReteCallBackList = std::list<ReteCallBackPtr>;

// //////////////////////////////////////////////////////////////////////////////////////
// GraphCallbackManager class -- main class for managing callback functions on BaseGraph
// --------------------------------------------------------------------------------------
class GraphCallbackManager;
using GraphCallbackManagerPtr = std::shared_ptr<GraphCallbackManager>;

// BetaRelation making the rete network
class GraphCallbackManager {
 public:
  GraphCallbackManager(): callbacks_() {};
  virtual ~GraphCallbackManager() {}

  inline void
  add_callback(ReteCallBackPtr cp)
  {
    this->callbacks_.push_back(cp);
  }

  inline void
  clear_callbacks()
  {
    this->callbacks_.clear();
  }

  inline void
  triple_inserted(r_index u, r_index v, r_index w)const
  {
    for(auto const& cp: this->callbacks_)
    {
      cp->triple_inserted(u, v, w);
    }
  }

  inline void
  triple_deleted(r_index u, r_index v, r_index w)const
  {
    for(auto const& cp: this->callbacks_)
    {
      cp->triple_deleted(u, v, w);
    }
  }

 private:
  ReteCallBackList callbacks_;
};

inline GraphCallbackManagerPtr
create_graph_callback_mgr() {
  return std::make_shared<GraphCallbackManager>();
}

} // namespace jets::rdf
#endif // JETS_RDF_GRAPH_CALLBACK_MGR_H
