
#include <stdlib.h>
#include <stdint.h>
#include <utility>

#include "../rete/rete_err.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/jets_rete_cwrapper.h"
#include "rete_session.h"

using namespace jets::rete;
using namespace jets::rdf;

int create_jetstore_hdl( char const * rete_db_path, HJETS * handle )
{
  if(not rete_db_path) return -1;
  auto * factory = new ReteMetaStoreFactory();
  *handle = factory;
  int res = factory->load_database(rete_db_path);
  if(res) {
    LOG(ERROR) << "create_jetstore_hdl: ERROR while loading database "<<
      rete_db_path<<", code "<<res;
  }
  return res;
}

HJETS go_create_jetstore_hdl( char const * rete_db_path)
{
  if(not rete_db_path) return nullptr;
  auto * factory = new ReteMetaStoreFactory();
  int res = factory->load_database(rete_db_path);
  if(res) {
    LOG(ERROR) << "go_create_jetstore_hdl: ERROR while loading database "<<
      rete_db_path<<", code "<<res;
    return nullptr;
  }
  return factory;
}

int delete_jetstore_hdl( HJETS handle )
{
  if(not handle) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(handle);
  delete factory;
  return 0;
}

int create_rete_session( HJETS jets_hdl, char const * jetrule_name, HJRETE * handle )
{
  if(not jetrule_name) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(jets_hdl);
  if(not factory) {
    LOG(ERROR) << "create_rete_session: ERROR NULL factory for "<<jetrule_name;
    return -1;
  }

  auto ms = factory->get_rete_meta_store(jetrule_name);
  if(not ms) {
    LOG(ERROR) << "::create_rete_session: ERROR ReteMetaStore not found for main_rule file ";
    return -1;
  }
  auto rdf_session = jets::rdf::create_rdf_session(ms->get_meta_graph());
  auto * rete_session = new ReteSession(ms, rdf_session);
  *handle = rete_session;
  int res = rete_session->initialize();
  if(res) {
    LOG(ERROR) << "create_rete_session: ERROR while initializing rete session "<<
      ", code "<<res;
  }
  return res;
}

int delete_rete_session(  HJRETE rete_session_hdl )
{
  if(not rete_session_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_session_hdl);
  delete rete_session;
  return 0;
}

// Creating resources and literals
int create_resource(HJRETE rete_hdl, char const * name, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_resource(name);
  return 0;
}
int create_text(HJRETE rete_hdl, char const * txt, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(txt);
  return 0;
}
int create_int(HJRETE rete_hdl, int v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(v);
  return 0;
}

// Get the resource name and literal value
int get_resource_type(HJR handle)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_null_t             : return rdf_null_t;
  case rdf_blank_node_t       : return rdf_blank_node_t;
  case rdf_named_resource_t   : return rdf_named_resource_t;
  case rdf_literal_int32_t    : return rdf_literal_int32_t;
  case rdf_literal_uint32_t   : return rdf_literal_uint32_t;
  case rdf_literal_int64_t    : return rdf_literal_int64_t;
  case rdf_literal_uint64_t   : return rdf_literal_uint64_t;
  case rdf_literal_double_t   : return rdf_literal_double_t;
  case rdf_literal_string_t   : return rdf_literal_string_t;
  default: return -1;
  }
}

// Get the resource name and literal value
int get_resource_name(HJR handle, HSTR*v)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_null_t             : return -1;
  case rdf_blank_node_t       : return -1;
  case rdf_named_resource_t   : *v = boost::get<NamedResource>(r)->name.data(); return 0;
  case rdf_literal_int32_t    : return -1;
  case rdf_literal_uint32_t   : return -1;
  case rdf_literal_int64_t    : return -1;
  case rdf_literal_uint64_t   : return -1;
  case rdf_literal_double_t   : return -1;
  case rdf_literal_string_t   : return -1;
  default: return -1;
  }
}

int get_int_literal(HJR handle, int*v)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_null_t             : return -1;
  case rdf_blank_node_t       : return -1;
  case rdf_named_resource_t   : return -1;
  case rdf_literal_int32_t    : *v = boost::get<LInt32>(r)->data; return 0;
  case rdf_literal_uint32_t   : return -1;
  case rdf_literal_int64_t    : return -1;
  case rdf_literal_uint64_t   : return -1;
  case rdf_literal_double_t   : return -1;
  case rdf_literal_string_t   : return -1;
  default: return -1;
  }
}

int get_text_literal(HJR handle, HSTR*v)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_null_t             : return -1;
  case rdf_blank_node_t       : return -1;
  case rdf_named_resource_t   : return -1;
  case rdf_literal_int32_t    : return -1;
  case rdf_literal_uint32_t   : return -1;
  case rdf_literal_int64_t    : return -1;
  case rdf_literal_uint64_t   : return -1;
  case rdf_literal_double_t   : return -1;
  case rdf_literal_string_t   : *v = boost::get<LString>(r)->data.data(); return 0;
  default: return -1;
  }
}

int insert(HJRETE rete_hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl)
{
  if(not rete_hdl) return -1;
  if(not s_hdl or not p_hdl or not o_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  return rete_session->rdf_session()->insert(s, p, o);
}

int contains(HJRETE rete_hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl)
{
  if(not rete_hdl) return -1;
  if(not s_hdl or not p_hdl or not o_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  return rete_session->rdf_session()->contains(s, p, o);
}

int execute_rules(HJRETE rete_hdl)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  return rete_session->execute_rules();
}

int find_all(HJRETE rete_hdl, HJITERATOR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto * itor = rete_session->rdf_session()->new_find();
  *handle = itor;
  return 0;
}

int is_end(HJITERATOR handle)
{
  if(not handle) return -1;
  auto * itor =  static_cast<ReteSession::Iterator*>(handle);
  return itor->is_end();  
}

int next(HJITERATOR handle)
{
  if(not handle) return -1;
  auto * itor =  static_cast<ReteSession::Iterator*>(handle);
  return itor->next();  
}

int get_subject(HJITERATOR itor_hdl, HJR * handle)
{
  if(not handle) return -1;
  auto * itor =  static_cast<ReteSession::Iterator*>(itor_hdl);
  *handle = itor->get_subject();
  return 0;
}

int get_predicate(HJITERATOR itor_hdl, HJR * handle)
{
  if(not handle) return -1;
  auto * itor =  static_cast<ReteSession::Iterator*>(itor_hdl);
  *handle = itor->get_predicate();
  return 0;
}

int get_object(HJITERATOR itor_hdl, HJR * handle)
{
  if(not handle) return -1;
  auto * itor =  static_cast<ReteSession::Iterator*>(itor_hdl);
  *handle = itor->get_object();
  return 0;
}

int dispose(HJITERATOR handle)
{
  if(not handle) return -1;
  auto * itor =  static_cast<ReteSession::Iterator*>(handle);
  delete itor;
  return 0;
}

// int find_asserted(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// int find_inferred(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
