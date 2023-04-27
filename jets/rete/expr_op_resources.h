#ifndef JETS_RETE_EXPR_OP_RESOURCES_H
#define JETS_RETE_EXPR_OP_RESOURCES_H

#include <cctype>
#include <cstdint>
#include <type_traits>
#include <algorithm>
#include <string>
#include <memory>
#include <utility>
#include <regex>

#include <boost/numeric/conversion/cast.hpp>

#include "../rdf/rdf_types.h"
#include "../rete/rete_err.h"
#include "../rete/beta_row.h"
#include "../rete/rete_session.h"

// This file contains basic operator used in rule expression 
// see ExprUnaryOp and ExprBinaryOp classes.
namespace jets::rete {

inline int setup_callback_for_visitors(ReteSession * rs, int vertex, ExprBase::ExprDataType && rhs)
{
  auto * rdf_session_p = rs->rdf_session();
  auto p_filter = get_resource(rs, std::forward<ExprBase::ExprDataType>(rhs));
  if(not p_filter) {
      return 0;
  }
  auto cb = create_rete_callback_for_visitors(rs, vertex, p_filter);
  rdf_session_p->asserted_graph()->register_callback(cb);
  rdf_session_p->inferred_graph()->register_callback(cb);
  return 0;
}

using RDFTTYPE = rdf::RdfAstType;

// CreateEntityVisitor
// --------------------------------------------------------------------------------------
struct CreateEntityVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CreateEntityVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  CreateEntityVisitor(): rs(nullptr), br(nullptr) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for create_entity: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return rdf::NamedResource{ this->create_entity(lhs.data==0 ? "":std::to_string(lhs.data)) };}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return rdf::NamedResource{ this->create_entity(lhs.data==0 ? "":std::to_string(lhs.data)) };}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::NamedResource{ this->create_entity(lhs.data==0 ? "":std::to_string(lhs.data)) };}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return rdf::NamedResource{ this->create_entity(lhs.data==0 ? "":std::to_string(lhs.data)) };}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return rdf::NamedResource{ this->create_entity(lhs.data==0 ? "":std::to_string(lhs.data)) };}
  RDFTTYPE operator()(rdf::LString lhs)const{return rdf::NamedResource{ this->create_entity(lhs.data) };}

  std::string
  create_entity(std::string key)const
  {
    auto sess = this->rs->rdf_session();
    auto rmgr = sess->rmgr();
    if(key.empty()) key = rdf::create_uuid();
    auto entity = rmgr->create_resource(key);
    auto jets_key = rmgr->jets()->jets__key;
    sess->insert_inferred(entity, jets_key, rmgr->create_literal(key));
    return key;
  }

  ReteSession * rs;
  BetaRow const* br;
};

// CreateLiteralVisitor
// --------------------------------------------------------------------------------------
struct CreateLiteralVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CreateLiteralVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  CreateLiteralVisitor(): rs(nullptr), br(nullptr) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for create_literal: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LString lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDate lhs)const{return lhs;}
  RDFTTYPE operator()(rdf::LDatetime lhs)const{return lhs;}

  ReteSession * rs;
  BetaRow const* br;
};

// CreateResourceVisitor
// --------------------------------------------------------------------------------------
struct CreateResourceVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CreateResourceVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  CreateResourceVisitor(): rs(nullptr), br(nullptr) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for create_resource: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32 lhs)const{return rdf::NamedResource(std::to_string(lhs.data));}
  RDFTTYPE operator()(rdf::LString lhs)const{return rdf::NamedResource(lhs.data);}

  ReteSession * rs;
  BetaRow const* br;
};

// CreateUUIDResourceVisitor
// --------------------------------------------------------------------------------------
struct CreateUUIDResourceVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  CreateUUIDResourceVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  CreateUUIDResourceVisitor(): rs(nullptr), br(nullptr) {}
  template<class T> RDFTTYPE operator()(T lhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for create_uuid_resource: ("<<lhs<<")");};

  RDFTTYPE operator()(rdf::LInt32  lhs)const{return rdf::NamedResource(rdf::create_uuid());}
  RDFTTYPE operator()(rdf::LUInt32 lhs)const{return rdf::NamedResource(rdf::create_uuid());}
  RDFTTYPE operator()(rdf::LInt64  lhs)const{return rdf::NamedResource(rdf::create_uuid());}
  RDFTTYPE operator()(rdf::LUInt64 lhs)const{return rdf::NamedResource(rdf::create_uuid());}
  RDFTTYPE operator()(rdf::LDouble lhs)const{return rdf::NamedResource(rdf::create_uuid());}
  RDFTTYPE operator()(rdf::LString lhs)const{return rdf::NamedResource(rdf::create_uuid());}

  ReteSession * rs;
  BetaRow const* br;
};

// ExistVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct ExistVisitor: public boost::static_visitor<RDFTTYPE>
{
  ExistVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for exist: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::LInt32(0);
    auto objp = sess->get_object(pr.first, pr.second);
    return rdf::LInt32(objp != nullptr);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"ExistVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";
    return setup_callback_for_visitors(rs, vertex, std::forward<ExprBase::ExprDataType>(rhs));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// ExistNotVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct ExistNotVisitor: public boost::static_visitor<RDFTTYPE>
{
  ExistNotVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for exist_not: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::LInt32(1);
    auto objp = sess->get_object(pr.first, pr.second);
    return rdf::LInt32(objp == nullptr);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"ExistNotVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";
    return setup_callback_for_visitors(rs, vertex, std::forward<ExprBase::ExprDataType>(rhs));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// SizeOfVisitor * Add truth maintenance
// --------------------------------------------------------------------------------------
struct SizeOfVisitor: public boost::static_visitor<RDFTTYPE>
{
  SizeOfVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}
  template<class T, class U> RDFTTYPE operator()(T lhs, U rhs) const {if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("Invalid arguments for size_of: ("<<lhs<<", "<<rhs<<")");};

  RDFTTYPE operator()(rdf::NamedResource lhs, rdf::NamedResource rhs)const
  {
    auto * sess = this->rs->rdf_session();
    auto pr = get_resources(sess->rmgr(), std::move(lhs.name), std::move(rhs.name));
    if(not pr.first or not pr.second) return rdf::LInt32(0);
    auto itor = sess->find(pr.first, pr.second, rdf::make_any());
    int size = 0;
    while(not itor.is_end()) {
      ++size;
      itor.next();
    }
    return rdf::LInt32(size);
  }

  int
  register_callback(int vertex, ExprBase::ExprDataType && lhs, ExprBase::ExprDataType && rhs)const
  {
    VLOG(40)<<"SizeOfVisitor::register callback for vertex "<<vertex<<" with pattern (*,"<<rhs<<",*)";
    return setup_callback_for_visitors(rs, vertex, std::forward<ExprBase::ExprDataType>(rhs));
  }

  ReteSession * rs;
  BetaRow const* br;
};

// IsLiteralVisitor
// --------------------------------------------------------------------------------------
struct IsLiteralVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  IsLiteralVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}

  RDFTTYPE operator()(rdf::RDFNull       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::BlankNode     lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::NamedResource lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LInt32        lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LUInt32       lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LInt64        lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LUInt64       lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LDouble       lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LString       lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LDate         lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LDatetime     lhs)const{return rdf::LInt32(1);}

  ReteSession * rs;
  BetaRow const* br;
};

// IsNullVisitor
// --------------------------------------------------------------------------------------
struct IsNullVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  IsNullVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}

  RDFTTYPE operator()(rdf::RDFNull       lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::BlankNode     lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::NamedResource lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LInt32        lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LUInt32       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LInt64        lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LUInt64       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LDouble       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LString       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LDate         lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LDatetime     lhs)const{return rdf::LInt32(0);}

  ReteSession * rs;
  BetaRow const* br;
};

// IsResourceVisitor
// --------------------------------------------------------------------------------------
struct IsResourceVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  IsResourceVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}

  RDFTTYPE operator()(rdf::RDFNull       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::BlankNode     lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::NamedResource lhs)const{return rdf::LInt32(1);}
  RDFTTYPE operator()(rdf::LInt32        lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LUInt32       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LInt64        lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LUInt64       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LDouble       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LString       lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LDate         lhs)const{return rdf::LInt32(0);}
  RDFTTYPE operator()(rdf::LDatetime     lhs)const{return rdf::LInt32(0);}

  ReteSession * rs;
  BetaRow const* br;
};

// RaiseExceptionVisitor
// --------------------------------------------------------------------------------------
struct RaiseExceptionVisitor: public boost::static_visitor<RDFTTYPE>, public NoCallbackNeeded
{
  RaiseExceptionVisitor(ReteSession * rs, BetaRow const* br): rs(rs), br(br) {}

  RDFTTYPE operator()(rdf::RDFNull       lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("null");}
  RDFTTYPE operator()(rdf::BlankNode     lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION("blank-node");}
  RDFTTYPE operator()(rdf::NamedResource lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.name);}
  RDFTTYPE operator()(rdf::LInt32        lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LUInt32       lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LInt64        lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LUInt64       lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LDouble       lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LString       lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LDate         lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}
  RDFTTYPE operator()(rdf::LDatetime     lhs)const{if(br==nullptr) return rdf::Null(); else RETE_EXCEPTION(lhs.data);}

  ReteSession * rs;
  BetaRow const* br;
};


} // namespace jets::rete
#endif // JETS_RETE_EXPR_OP_RESOURCES_H
