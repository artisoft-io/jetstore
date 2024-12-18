
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
  int res = 0;
  try {
    res = factory->load_database(rete_db_path, lookup_data_db_path);
  } catch(jets::rete_exception ex) {
    LOG(ERROR)<<"create_jetstore_hdl: ERROR while loading database '"<< rete_db_path<<"': "<<ex;
    return -1;
  } catch(...) {
    LOG(ERROR)<<"create_jetstore_hdl: Unknown ERROR while loading database" << rete_db_path;
    return -1;
  }

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
  factory->reset();
  delete factory;
  return 0;
}

int create_rdf_session(HJETS jets_hdl, HJRDF * handle )
{
  if(not jets_hdl or not handle) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(jets_hdl);
  if(not factory) {
    LOG(ERROR) << "create_rdf_session: ERROR NULL factory!";
    return -1;
  }

  auto mg = factory->get_meta_graph();
  if(not mg) {
    LOG(ERROR) << "::create_rdf_session: ERROR MetaGraph not found";
    return -1;
  }
  auto * rdf_session = jets::rdf::RDFSession::create_raw_ptr(mg);
  *handle = rdf_session;
  if(not rdf_session) return -1;
  return 0;
}

int delete_rdf_session(HJRDF hdl )
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  delete rdf_session;
  return 0;
}

int create_rete_session(HJETS jets_hdl, HJRDF rdf_hdl, char const * jetrule_name, HJRETE * handle )
{
  if(not jetrule_name or not jets_hdl or not rdf_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(jets_hdl);
  auto * rdf_session =  static_cast<RDFSession*>(rdf_hdl);
  if(not factory or not rdf_session) {
    LOG(ERROR) << "create_rete_session2: ERROR NULL factory for "<<jetrule_name;
    return -1;
  }
  auto ms = factory->get_rete_meta_store(jetrule_name);
  if(not ms) {
    LOG(ERROR) << "::create_rete_session: ERROR ReteMetaStore not found for main_rule file ";
    return -1;
  }
  auto * rete_session = new ReteSession(ms, rdf_session);
  if(not rete_session) return -1;
  *handle = rete_session;
  int res = rete_session->initialize();
  if(res) {
    LOG(ERROR) << "create_rete_session: ERROR while initializing rete session "<<
      ", code "<<res;
  }
  return res;
}

int delete_rete_session(HJRETE rete_session_hdl )
{
  if(not rete_session_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_session_hdl);
  rete_session->terminate();
  delete rete_session;
  return 0;
}

// Creating meta resources and literals
int create_meta_null(HJETS js_hdl, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  * handle = factory->get_rmgr()->get_null();
  return 0;
}
int create_meta_blanknode(HJETS js_hdl, int v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  * handle = factory->get_rmgr()->create_bnode(v);
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

  * handle = factory->get_rmgr()->create_resource(name);
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

  * handle = factory->get_rmgr()->get_resource(name);
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

  * handle = factory->get_rmgr()->create_literal(name);
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

  * handle = factory->get_rmgr()->create_literal(v);
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

  * handle = factory->get_rmgr()->create_literal(v);
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

  * handle = factory->get_rmgr()->create_literal(v);
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

  * handle = factory->get_rmgr()->create_literal(v);
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

  * handle = factory->get_rmgr()->create_literal(v);
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
  if(d.is_not_a_date()) return -2;
  * handle = factory->get_rmgr()->create_literal(d);
  return 0;
}

int create_meta_datetime(HJETS js_hdl, char const * v, HJR * handle)
{
  if(not js_hdl) return -1;
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  auto d = parse_datetime(v);
  if(d.is_not_a_date_time()) return -2;
  * handle = factory->get_rmgr()->create_literal(d);
  return 0;
}

int load_process_meta_triples(char const * jetrule_name, int is_rule_set, HJETS js_hdl)
{
  if(not jetrule_name) {
    LOG(ERROR) << "ERROR: jetrule_name is NULL in cwrapper";
    return -1;
  }
  if(not js_hdl) {
    LOG(ERROR) << "ERROR: js_hdl is NULL in cwrapper";
    return -1;
  }
  auto * factory =  static_cast<ReteMetaStoreFactory*>(js_hdl);
  return factory->load_meta_triples(jetrule_name, is_rule_set);
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
  auto mg = factory->get_meta_graph();
  if(not mg) {
    LOG(ERROR) << "insert_meta_graph: while get_meta_graph: ERROR MetaGraph not found";
    return -1;
  }
  return mg->insert(s, p, o);
}

// Creating resources and literals
int create_null(HJRDF hdl, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->meta_graph()->get_rmgr()->get_null();
  return 0;
}
int create_blanknode(HJRDF hdl, int v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_bnode(v);
  return 0;
}
int create_resource(HJRDF hdl, char const * name, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_resource(name);
  return 0;
}
int get_resource(HJRDF hdl, char const * name, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->get_resource(name);
  return 0;
}
int create_text(HJRDF hdl, char const * txt, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_literal(txt);
  return 0;
}
int create_int(HJRDF hdl, int v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_literal(v);
  return 0;
}
int create_uint(HJRDF hdl, uint v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_literal(v);
  return 0;
}
int create_long(HJRDF hdl, long v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_literal(v);
  return 0;
}
int create_ulong(HJRDF hdl, ulong v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_literal(v);
  return 0;
}
int create_double(HJRDF hdl, double v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  * handle = rdf_session->get_rmgr()->create_literal(v);
  return 0;
}
int create_date(HJRDF hdl, char const * v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto d = parse_date(v);
  if(d.is_not_a_date()) return -2;
  * handle = rdf_session->get_rmgr()->create_literal(d);
  return 0;
}
int create_datetime(HJRDF hdl, char const * v, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto d = parse_datetime(v);
  if(d.is_not_a_date_time()) return -2;
  * handle = rdf_session->get_rmgr()->create_literal(d);
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
  if(not handle or not v) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_named_resource_t   : *v = boost::get<NamedResource>(r)->name.data(); return 0;
  default: return -1;
  }
}
char const* get_resource_name2(HJR handle, int*v)
{
  if(not handle or not v) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_named_resource_t   : *v = 0; return boost::get<NamedResource>(r)->name.data();
  default: *v = -1; return nullptr;
  }
}

int get_int_literal(HJR handle, int*v)
{
  if(not handle or not v) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_int32_t    : 
    *v = boost::get<LInt32>(r)->data;
    return 0;
  default: return -1;
  }
}

int get_double_literal(HJR handle, double*v)
{
  if(not handle or not v) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_double_t    : 
    *v = boost::get<LDouble>(r)->data;
    return 0;
  default: return -1;
  }
}

int get_date_details(HJR hdl, int* year, int* month, int* day)
{
  if(not hdl or not year or not month or not day) return -1;
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

// frac part is in nanosecond to match go's time resolution
int get_datetime_details(HJR hdl, int* year, int* month, int* day, int* hr, int* min, int* sec, int* frac)
{
  if(not hdl or not year or not month or not day) return -1;
  if(not hr or not min or not sec or not frac) return -1;
  auto const* r =  static_cast<r_index>(hdl);
  switch (r->which()) {
  case rdf_literal_datetime_t: 
    {
      datetime const& dt = boost::get<LDatetime>(r)->data;       
      if(dt.is_not_a_date_time()) return -2;
      date dd = dt.date();
      date::ymd_type ymd = dd.year_month_day();
      *year = ymd.year;
      *month = ymd.month;
      *day = ymd.day;
      auto dur = dt.time_of_day();
      *hr = dur.hours();
      *min = dur.minutes();
      *sec = dur.seconds();
      *frac= dur.fractional_seconds()*pow(10, 9-time_duration::num_fractional_digits());
    }
    return 0;
  default: return -1;
  }
}

int get_date_iso_string(HJR handle, HSTR*v)
{
  if(not handle or not v) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_date_t   : 
    {
      date const& d = boost::get<LDate>(r)->data;
      if(d.is_not_a_date()) return -2;
      *v = boost::gregorian::to_iso_extended_string(d).c_str(); 
      return 0;
    }
  default: return -1;
  }
}
char const* get_date_iso_string2(HJR handle, int*v)
{
  if(not handle or not v) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_date_t   : 
    {
      date const& d = boost::get<LDate>(r)->data;
      if(d.is_not_a_date()) {
        *v = -2;
        return nullptr;
      }
      *v = 0;
      return boost::gregorian::to_iso_extended_string(d).c_str();
    }
  default: *v = -1; return nullptr;
  }
}
int get_datetime_iso_string(HJR handle, HSTR*v)
{
  if(not handle or not v) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_datetime_t   : 
    {
      datetime const& d = boost::get<LDatetime>(r)->data;
      if(d.is_not_a_date_time()) return -2;
      *v = boost::posix_time::to_iso_extended_string(d).c_str(); 
      return 0;
    }
  default: return -1;
  }
}
char const* get_datetime_iso_string2(HJR handle, int*v)
{
  if(not handle or not v) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_datetime_t   : 
    {
      datetime const& d = boost::get<LDatetime>(r)->data;
      if(d.is_not_a_date_time()) {
        *v = -2;
        return  nullptr;
      }
      *v = 0; 
      return boost::posix_time::to_iso_extended_string(d).c_str();
    }
  default: *v = -1; return nullptr;
  }
}

int get_text_literal(HJR handle, HSTR*v)
{
  if(not handle or not v) return -1;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_string_t   : *v = boost::get<LString>(r)->data.data(); return 0;
  default: return -1;
  }
}
char const* get_text_literal2(HJR handle, int*v)
{
  if(not handle or not v) return nullptr;
  auto const* r =  static_cast<r_index>(handle);
  switch (r->which()) {
  case rdf_literal_string_t   : *v = 0; return boost::get<LString>(r)->data.data();
  default: *v = -1; return nullptr;
  }
}

int insert(HJRDF hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl)
{
  if(not hdl) return -1;
  if(not s_hdl or not p_hdl or not o_hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  return rdf_session->insert(s, p, o);
}

int contains(HJRDF hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl)
{
  if(not hdl) return -1;
  if(not s_hdl or not p_hdl or not o_hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  return rdf_session->contains(s, p, o);
}

int contains_sp(HJRDF hdl, HJR s_hdl, HJR p_hdl)
{
  if(not hdl) return -1;
  if(not s_hdl or not p_hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  return rdf_session->contains_sp(s, p);
}

int erase(HJRDF hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  return rdf_session->erase(s, p, o);
}

int execute_rules(HJRETE rete_hdl)
{
  if(not rete_hdl) return -1;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  return rete_session->execute_rules();
}
char const* execute_rules2(HJRETE rete_hdl, int*v)
{
  if(not rete_hdl or not v) return nullptr;
  auto * rete_session =  static_cast<ReteSession*>(rete_hdl);
  return rete_session->execute_rules2(v);
}

// returns the rdf graph as list of triples (text buffer)
char const* get_rdf_graph_txt(HJRDF hdl, int*v)
{
  if(not hdl or not v) return nullptr;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  return rdf_session->get_graph_buf(v);
}

int dump_rdf_graph(HJRDF hdl)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  std::cout << "Meta Graph Contains "<<rdf_session->meta_graph()->size()<<" triples (c++ version):"<<std::endl;
  std::cout << rdf_session->meta_graph()<<"---"<<std::endl;
  std::cout << "Asserted Graph Contains "<<rdf_session->asserted_graph()->size()<<" triples (c++ version):"<<std::endl;
  std::cout << rdf_session->asserted_graph()<<"---"<<std::endl;
  std::cout << "Inferred Graph Contains "<<rdf_session->inferred_graph()->size()<<" triples (c++ version):"<<std::endl;
  std::cout << rdf_session->inferred_graph()<<"---"<<std::endl;
  std::cout << "The Meta Graph Contains:"<<rdf_session->meta_graph()->size()<<" triples"<<std::endl;
  return 0;
}

int find_all(HJRDF hdl, HJITERATOR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto * itor = rdf_session->new_find();
  *handle = itor;
  return 0;
}

int find(HJRDF hdl, HJR s_hdl, HJR p_hdl, HJR o_hdl, HJITERATOR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto const* o =  static_cast<r_index>(o_hdl);
  auto * itor = rdf_session->new_find(s, p, o);
  *handle = itor;
  return 0;
}

int find_sp(HJRDF hdl, HJR s_hdl, HJR p_hdl, HJITERATOR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto * itor = rdf_session->new_find(s, p);
  *handle = itor;
  return 0;
}

int find_object(HJRDF hdl, HJR s_hdl, HJR p_hdl, HJR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto const* p =  static_cast<r_index>(p_hdl);
  auto * obj = rdf_session->get_object(s, p);
  *handle = obj;
  return 0;
}

int find_s(HJRDF hdl, HJR s_hdl, HJITERATOR * handle)
{
  if(not hdl) return -1;
  auto * rdf_session =  static_cast<RDFSession*>(hdl);
  auto const* s =  static_cast<r_index>(s_hdl);
  auto * itor = rdf_session->new_find(s);
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
