#ifndef JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
#define JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H

#include <string>
#include <memory>
#include <list>

#include "expr.h"
#include "jets/rdf/rdf_types.h"

// This file contains implementation classes for rdf::GraphCallbackManager
namespace jets::rete {
// ReteCallBack class implementating the callback for triple inserted and deleted
class ReteCallBack {
 public:
  ReteCallBack() = delete;
  ReteCallBack(ReteSession * rete_session, int vertex)
    : rete_session_(rete_session), vertex_(vertex) {};
  ~ReteCallBack()=default;

  ReteCallBack(ReteCallBack const& rhs) = default;
  ReteCallBack(ReteCallBack && rhs) = default;
  ReteCallBack& operator=(ReteCallBack const&) = default;

  // Implementation moved to rete_session.h
  void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const;

  // Implementation moved to rete_session.h
  void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const;

 private:
  ReteSession * rete_session_;
  int           vertex_;
};

// ReteCallBackList: Container for ReteCallBack
using ReteCallBackList = std::list<ReteCallBack>;

// //////////////////////////////////////////////////////////////////////////////////////
// ReteGraphCallbackMgr class -- main class for managing callback functions on BaseGraph
// --------------------------------------------------------------------------------------
// Implementation class for rdf::GraphCallbackManager
class ReteGraphCallbackMgr: public rdf::GraphCallbackManager {
 public:
  ReteGraphCallbackMgr() = delete;
  // ReteGraphCallbackMgr(ReteCallBackList callbacks) 
  //   : rdf::GraphCallbackManager(), callbacks_(callbacks) {}
  ReteGraphCallbackMgr(ReteCallBackList const& callbacks) 
    : rdf::GraphCallbackManager(), callbacks_(callbacks) {}
  ReteGraphCallbackMgr(ReteCallBackList && callbacks) 
    : rdf::GraphCallbackManager(), 
      callbacks_(std::forward<ReteCallBackList>(callbacks)) {}

  virtual ~ReteGraphCallbackMgr() {}

  void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const override
  {
    for(auto & callback: this->callbacks_) {
      callback.triple_inserted(s, p, o);
    }
  }

  virtual void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const override
  {
    for(auto & callback: this->callbacks_) {
      callback.triple_deleted(s, p, o);
    }
  }

 private:
  ReteCallBackList callbacks_;
};

inline rdf::GraphCallbackManagerPtr create_graph_callback(ReteCallBackList const& callbacks)
{
  return std::make_shared<ReteGraphCallbackMgr>(callbacks);
}

inline rdf::GraphCallbackManagerPtr create_graph_callback(ReteCallBackList && callbacks)
{
  return std::make_shared<ReteGraphCallbackMgr>(std::forward<ReteCallBackList>(callbacks));
}

} // namespace jets::rete
#endif // JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
