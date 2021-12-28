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
template<class T>
class ReteCallBack {
 public:
  ReteCallBack() = delete;
  ReteCallBack(ReteSession<T> * rete_session, int vertex)
    : rete_session_(rete_session), vertex_(vertex) {};
  ~ReteCallBack()=default;

  ReteCallBack(ReteCallBack const& rhs) = default;
  ReteCallBack(ReteCallBack && rhs) = default;
  ReteCallBack& operator=(ReteCallBack const&) = default;

  inline void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
  {
    this->rete_session_->triple_inserted(this->vertex_, s, p, o);
  }

  inline void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const
  {
    this->rete_session_->triple_deleted(this->vertex_, s, p, o);
  }

 private:
  ReteSession<T> * rete_session_;
  int              vertex_;
};

// ReteCallBackList: Container for ReteCallBack
template<class T>
using ReteCallBackList = std::list<ReteCallBack<T>>;

// //////////////////////////////////////////////////////////////////////////////////////
// ReteGraphCallbackMgr class -- main class for managing callback functions on BaseGraph
// --------------------------------------------------------------------------------------
// Implementation class for rdf::GraphCallbackManager
template<class T>
class ReteGraphCallbackMgr: public rdf::GraphCallbackManager {
 public:
  ReteGraphCallbackMgr() = delete;
  // ReteGraphCallbackMgr(ReteCallBackList<T> callbacks) 
  //   : rdf::GraphCallbackManager(), callbacks_(callbacks) {}
  ReteGraphCallbackMgr(ReteCallBackList<T> const& callbacks) 
    : rdf::GraphCallbackManager(), callbacks_(callbacks) {}
  ReteGraphCallbackMgr(ReteCallBackList<T> && callbacks) 
    : rdf::GraphCallbackManager(), callbacks_(std::forward<ReteCallBackList<T>>(callbacks)) {}

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
  ReteCallBackList<T> callbacks_;
};

// template<class T>
// inline rdf::GraphCallbackManagerPtr create_graph_callback(ReteCallBackList<T> callbacks)
// {
//   return std::make_shared<ReteGraphCallbackMgr<T>>(callbacks);
// }

template<class T>
inline rdf::GraphCallbackManagerPtr create_graph_callback(ReteCallBackList<T> const& callbacks)
{
  return std::make_shared<ReteGraphCallbackMgr<T>>(callbacks);
}

template<class T>
inline rdf::GraphCallbackManagerPtr create_graph_callback(ReteCallBackList<T> && callbacks)
{
  return std::make_shared<ReteGraphCallbackMgr<T>>(std::forward<ReteCallBackList<T>>(callbacks));
}

} // namespace jets::rete
#endif // JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
