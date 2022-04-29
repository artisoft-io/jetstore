
#include <stdlib.h>
#include <stdint.h>
#include <utility>

#include "../rete/rete_err.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/jets_rete_cwrapper.h"
#include "rete_session.h"

using namespace jets::rete;
using namespace jets::rdf;

int create_jetstore_hdl( char const * rete_db_path, char const * lookup_data_db_path, HJETS * handle )
{
  if(not rete_db_path) return -1;
  auto * factory = new ReteMetaStoreFactory();
  *handle = factory;
  int res = factory->load_database(rete_db_path, lookup_data_db_path);
  if(res) {
    LOG(ERROR) << "create_jetstore_hdl: ERROR while loading database "<<
      rete_db_path<<", code "<<res;
  }
  return res;
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

// Creating meta resources and literals
int create_null(HJETS js_hdl, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  * handle = factory->meta_graph()->get_rmgr()->get_null();
  return 0;
}
int create_meta_blanknode(HJETS js_hdl, int v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  * handle = factory->meta_graph()->get_rmgr()->create_bnode(v);
  return 0;
}

int create_meta_resource(HJETS js_hdl, char const * name, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_resource: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_resource(name);
  return 0;
}

int get_meta_resource(HJETS js_hdl, char const * name, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_resource: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->get_resource(name);
  return 0;
}
int create_meta_text(HJETS js_hdl, char const * name, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_text: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_literal(name);
  return 0;
}
int create_meta_int(HJETS js_hdl, int v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_int: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_literal(v);
  return 0;
}
int create_meta_uint(HJETS js_hdl, uint v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_uint: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_literal(v);
  return 0;
}
int create_meta_long(HJETS js_hdl, long v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_long: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_literal(v);
  return 0;
}
int create_meta_ulong(HJETS js_hdl, ulong v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_ulong: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_literal(v);
  return 0;
}
int create_meta_double(HJETS js_hdl, double v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_double: ERROR NULL factory";
    return -1;
  }

  * handle = factory->meta_graph()->get_rmgr()->create_literal(v);
  return 0;
}
int create_meta_date(HJETS js_hdl, char const * v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  if(not factory) {
    LOG(ERROR) << "create_meta_date: ERROR NULL factory";
    return -1;
  }

  auto d = parse_date(v);
  * handle = factory->meta_graph()->get_rmgr()->create_literal(d);
  return 0;
}
int create_meta_datetime(HJETS js_hdl, char const * v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  auto d = parse_datetime(v);
  * handle = factory->meta_graph()->get_rmgr()->create_literal(d);
  return 0;
}

int insert_meta_graph(HJETS js_hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl)
{
  if(not js_hdl) {
    LOG(ERROR) << "ERROR: js_hdl is NULL in cwrapper";
    return -1;
  }
  if(not s_hdl or not p_hdl or not o_hdl) {
    LOG(ERROR) << "ERROR: r_hdl is NULL in cwrapper";
    return -1;
  }
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  return factory->get_meta_graph()->insert(s, p, o);
}

// Creating resources and literals
int create_blanknode(HJRETE rete_hdl, int v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_bnode(v);
  return 0;
}
int create_resource(HJRETE rete_hdl, char const * name, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_resource(name);
  return 0;
}
int get_resource(HJRETE rete_hdl, char const * name, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->get_resource(name);
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
int create_uint(HJRETE rete_hdl, uint v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(v);
  return 0;
}
int create_long(HJRETE rete_hdl, long v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(v);
  return 0;
}
int create_ulong(HJRETE rete_hdl, ulong v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(v);
  return 0;
}
int create_double(HJRETE rete_hdl, double v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(v);
  return 0;
}
int create_date(HJRETE rete_hdl, char const * v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto d = parse_date(v);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(d);
  return 0;
}
int create_datetime(HJRETE rete_hdl, char const * v, HJR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto d = parse_datetime(v);
  * handle = rete_session->rdf_session()->get_rmgr()->create_literal(d);
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
  case rdf_literal_date_t     : return rdf_literal_date_t;
  case rdf_literal_datetime_t : return rdf_literal_datetime_t;
  default: return -1;
  }
}

// Get the resource name and literal value
int get_resource_name(HJR handle, HSTR*v)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_named_resource_t   : *v = boost::get<NamedResource>(r)->name.data(); return 0;
  default: return -1;
  }
}
char const* go_get_resource_name(HJR handle)
{
  if(not handle) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_named_resource_t   : return boost::get<NamedResource>(r)->name.data();
  default: return nullptr;
  }
}

int get_int_literal(HJR handle, int*v)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_int32_t    : *v = boost::get<LInt32>(r)->data; return 0;
  default: return -1;
  }
}

int get_date_details(HJR hdl, int* year, int* month, int* day)
{
  if(not hdl) return -1;
  auto const* r =  static_cast<r_index>(hdl);
  switch (r->which()) {
  case rdf_literal_date_t: 
    {
      date const& d = boost::get<LDate>(r)->data; 
      if(d.is_not_a_date()) return -2;
      date::ymd_type ymd = d.year_month_day();
      *year = ymd.year;
      *month = ymd.month;
      *day = ymd.day;
    }
    return 0;
  default: return -1;
  }
}

char const* go_date_iso_string(HJR handle)
{
  if(not handle) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_date_t: 
    {
      date const& d = boost::get<LDate>(r)->data; 
      if(d.is_not_a_date()) return nullptr;
      return boost::gregorian::to_iso_extended_string(d).c_str();
    }
  default: return nullptr;
  }
}

char const* go_datetime_iso_string(HJR handle)
{
  if(not handle) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_datetime_t: 
    {
      datetime const& d = boost::get<LDatetime>(r)->data; 
      if(d.is_not_a_date_time()) return nullptr;
      return boost::posix_time::to_iso_extended_string(d).c_str();
    }
  default: return nullptr;
  }
}

int get_text_literal(HJR handle, HSTR*v)
{
  if(not handle) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_string_t   : *v = boost::get<LString>(r)->data.data(); return 0;
  default: return -1;
  }
}
char const* go_get_text_literal(HJR handle)
{
  if(not handle) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_string_t   : return boost::get<LString>(r)->data.data();
  default: return nullptr;
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
  //*
  std::cout<<"C: Calling rete_session execute_rules..."<<std::endl;
  return rete_session->execute_rules();
}

int dump_rdf_graph(HJRETE rete_hdl)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  std::cout << "RDF Session Contains:"<<std::endl;
  std::cout << rete_session->rdf_session()<<"---"<<std::endl;
  return 0;
}

int find_all(HJRETE rete_hdl, HJITERATOR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto * itor = rete_session->rdf_session()->new_find();
  *handle = itor;
  return 0;
}

int find(HJRETE rete_hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl, HJITERATOR * handle)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  auto * itor = rete_session->rdf_session()->new_find(s, p, o);
  *handle = itor;
  return 0;
}

int find_sp(HJRETE rete_hdl, HJR s_hdl, HJR p_hdl, HJITERATOR * handle)
{
  if(not rete_hdl or not s_hdl or not p_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto * itor = rete_session->rdf_session()->new_find(s, p);
  *handle = itor;
  return 0;
}

int find_object(HJRETE rete_hdl, HJR s_hdl, HJR p_hdl, HJR * handle)
{
  if(not rete_hdl or not s_hdl or not p_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto * obj = rete_session->rdf_session()->get_object(s, p);
  *handle = obj;
  return 0;
}

int find_s(HJRETE rete_hdl, HJR s_hdl, HJITERATOR * handle)
{
  if(not rete_hdl or not s_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto * itor = rete_session->rdf_session()->new_find(s);
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
