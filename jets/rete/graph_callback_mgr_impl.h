#ifndef JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
#define JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H

#include <string>
#include <memory>
#include <list>

#include "jets/rdf/rdf_types.h"

// This file contains implementation classes for rdf::GraphCallbackManager
namespace jets::rete {
class ReteSession;

// ReteCallBackImpl class implementating the callback for triple inserted and deleted
class ReteCallBackImpl: public rdf::ReteCallBack {
 public:
  ReteCallBackImpl() = delete;
  ReteCallBackImpl(ReteSession * rete_session, int vertex,
    rdf::r_index u_filter, rdf::r_index v_filter, rdf::r_index w_filter)
    : ReteCallBack(),
      rete_session_(rete_session), 
      vertex_(vertex),
      u_filter_(u_filter),
      v_filter_(v_filter),
      w_filter_(w_filter)
  {};
  ~ReteCallBackImpl()=default;

  ReteCallBackImpl(ReteCallBackImpl const& rhs) = default;
  ReteCallBackImpl(ReteCallBackImpl && rhs) = default;
  ReteCallBackImpl& operator=(ReteCallBackImpl const&) = default;

  // Implementation moved to rete_session.h
  void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const;

  // Implementation moved to rete_session.h
  void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const;

 private:
  ReteSession * rete_session_;
  int           vertex_;
  rdf::r_index  u_filter_;
  rdf::r_index  v_filter_;
  rdf::r_index  w_filter_;
};

inline rdf::ReteCallBackPtr
create_rete_callback(ReteSession * rete_session, int vertex,
  rdf::r_index u_filter, rdf::r_index v_filter, rdf::r_index w_filter)
{
  return std::make_shared<ReteCallBackImpl>(rete_session, vertex, u_filter, v_filter, w_filter);
}

} // namespace jets::rete
#endif // JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
