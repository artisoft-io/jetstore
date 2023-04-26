#ifndef JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
#define JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H

#include <string>
#include <memory>
#include <list>

#include "../rdf/rdf_types.h"

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
      s_filter_(u_filter),
      p_filter_(v_filter),
      o_filter_(w_filter)
  {};
  ~ReteCallBackImpl()=default;

  ReteCallBackImpl(ReteCallBackImpl const& rhs) = default;
  ReteCallBackImpl(ReteCallBackImpl && rhs) = default;
  ReteCallBackImpl& operator=(ReteCallBackImpl const&) = default;

  // Implementation moved to rete_session.h
  void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const override;

  // Implementation moved to rete_session.h
  void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const override;

 private:
  ReteSession * rete_session_;
  int           vertex_;
  rdf::r_index  s_filter_;
  rdf::r_index  p_filter_;
  rdf::r_index  o_filter_;
};

inline rdf::ReteCallBackPtr
create_rete_callback(ReteSession * rete_session, int vertex,
  rdf::r_index s_filter, rdf::r_index p_filter, rdf::r_index o_filter)
{
  return std::make_shared<ReteCallBackImpl>(rete_session, vertex, s_filter, p_filter, o_filter);
}

class ReteCallBack4VisitorsImpl: public rdf::ReteCallBack {
 public:
  ReteCallBack4VisitorsImpl() = delete;
  ReteCallBack4VisitorsImpl(ReteSession * rete_session, int vertex,
    rdf::r_index p_filter)
    : ReteCallBack(),
      rete_session_(rete_session), 
      vertex_(vertex),
      p_filter_(p_filter)
  {};
  ~ReteCallBack4VisitorsImpl()=default;

  ReteCallBack4VisitorsImpl(ReteCallBack4VisitorsImpl const& rhs) = default;
  ReteCallBack4VisitorsImpl(ReteCallBack4VisitorsImpl && rhs) = default;
  ReteCallBack4VisitorsImpl& operator=(ReteCallBack4VisitorsImpl const&) = default;

  // Implementation moved to rete_session.h
  void
  triple_inserted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const override;

  // Implementation moved to rete_session.h
  void
  triple_deleted(rdf::r_index s, rdf::r_index p, rdf::r_index o)const override;

 private:
  ReteSession * rete_session_;
  int           vertex_;
  rdf::r_index  p_filter_;
};

inline rdf::ReteCallBackPtr
create_rete_callback_for_visitors(ReteSession * rete_session, int vertex, rdf::r_index p_filter)
{
  return std::make_shared<ReteCallBack4VisitorsImpl>(rete_session, vertex, p_filter);
}

} // namespace jets::rete
#endif // JETS_RETE_GRAPH_CALLBACK_MGR_IMPL_H
