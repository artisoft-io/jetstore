
#include <stdlib.h>
#include <stdint.h>
#include <utility>

#include "jets/rete/rete_err.h"
#include "jets/rete/rete_meta_store_factory.h"
#include "jets/rete/jets_rete_cwrapper.h"
#include "rete_session.h"

using namespace jets::rete;

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

  // //TRY 1
  // auto rete_session = factory->create_rete_session(jetrule_name);
  // if(not rete_session) {
  //   LOG(ERROR) << "create_rete_session: ERROR NULL rete_session for "<<jetrule_name;
  //   return -1;
  // }
  // std::cout<<"RETE SESSION CREATED!"<<std::endl;
  // *handle = rete_session.get();
  // //TRY 1

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
  std::cout<<"RETE SESSION INIT DONE XX"<<std::endl;
  return res;
}

int delete_rete_session(  HJRETE rete_session_hdl )
{
  // //TRY 1
  // if(not jets_hdl or not rete_session_hdl) return -1;
  // auto * factory =  static_cast<ReteMetaStoreFactory*>(jets_hdl);
  // if(not factory) {
  //   LOG(ERROR) << "delete_rete_session: ERROR NULL factory";
  //   return -1;
  // }

  // auto * rete_session =  static_cast<ReteSession*>(rete_session_hdl);
  // return factory->delete_rete_session(rete_session);
  // //TRY 1
  std::cout<<"RETE SESSION INIT DONE YYY"<<std::endl;
  if(not rete_session_hdl) return -1;

  auto * rete_session =  static_cast<ReteSession*>(rete_session_hdl);
  
  std::cout<<"RETE SESSION INIT DONE ZZZ1"<<std::endl;
  delete rete_session;
  std::cout<<"RETE SESSION INIT DONE ZZZ"<<std::endl;
  return 0;
}

// using HJRETE = void*;

// int create_rete_session( HJETS jets_hdl, char const * rete_db_path, HJRETE * handle );
// int delete_rete_session( HJRETE * rete_hdl );

// struct HJR;
// typedef struct HJR HJR;

// // Creating resources and literals
// int create_resource(HJRETE * rete_hdl, char const * name, HJR ** handle);
// int create_text(HJRETE * rete_hdl, char const * txt, HJR ** handle);
// int create_int(HJRETE * rete_hdl, int v, HJR ** handle);
// // Get the resource name and literal value
// char const* get_resource_name(HJR * handle);
// int get_int_literal(HJR * handle); // errors?
// char const* get_text_literal(HJR * handle); // errors?

// int insert(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o);
// bool contains(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o);
// int execute_rules(HJRETE * rete_hdl);

// struct HJITERATOR;
// typedef struct HJITERATOR HJITERATOR;

// int find(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// int find_asserted(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// int find_inferred(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// bool is_end(HJITERATOR * handle);
// bool next(HJITERATOR * handle);
// int get_subject(HJITERATOR * itor_hdl, HJR ** handle);
// int get_predicate(HJITERATOR * itor_hdl, HJR ** handle);
// int get_object(HJITERATOR * itor_hdl, HJR ** handle);
// int dispose(HJITERATOR * itor_hdl);

