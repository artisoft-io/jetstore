#ifndef JETS_RETE_CWAPPER_H
#define JETS_RETE_CWAPPER_H

#ifdef __cplusplus
extern "C"
{
#endif
// Opaque types that we'll use as handles
// using HJETS = void*;
typedef void* HJETS;

int create_jetstore_hdl( char const * rete_db_path, HJETS * handle );
HJETS go_create_jetstore_hdl( char const * rete_db_path);
int delete_jetstore_hdl( HJETS handle );

// using HJRETE = void*;
typedef void* HJRETE;

int create_rete_session( HJETS jets_hdl, char const * jetrule_name, HJRETE * handle );
int delete_rete_session( HJRETE rete_session_hdl );

// using HJR = void const*;
typedef void* HJR;

// Creating resources and literals
int create_resource(HJRETE rete_hdl, char const * name, HJR * handle);
int create_text(HJRETE rete_hdl, char const * txt, HJR * handle);
int create_int(HJRETE rete_hdl, int v, HJR * handle);

// using HSTR = void const*;
typedef void* HSTR;

// Get the resource name and literal value
int get_resource_type(HJR handle);
int get_resource_name(HJR handle, HSTR*);
int get_int_literal(HJR handle, int*);
int get_text_literal(HJR handle, HSTR*);

// main functions
int insert(HJRETE rete_hdl, HJR s, HJR p, HJR o);
int contains(HJRETE rete_hdl, HJR s, HJR p, HJR o);
int execute_rules(HJRETE rete_hdl);

// using HJITERATOR = void*;
typedef void* HJITERATOR;

int find_all(HJRETE rete_hdl, HJITERATOR * handle);
// int find_asserted(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// int find_inferred(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
int is_end(HJITERATOR handle);
int next(HJITERATOR handle);
int get_subject(HJITERATOR itor_hdl, HJR * handle);
int get_predicate(HJITERATOR itor_hdl, HJR * handle);
int get_object(HJITERATOR itor_hdl, HJR * handle);
int dispose(HJITERATOR itor_hdl);

#ifdef __cplusplus
}
#endif

#endif // JETS_RETE_CWAPPER_H
