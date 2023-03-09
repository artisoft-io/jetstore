#ifndef JETS_RETE_CWAPPER_H
#define JETS_RETE_CWAPPER_H

#include <stdlib.h>

#ifdef __cplusplus
extern "C"
{
#endif
// Opaque types that we'll use as handles
typedef void* HJETS;

int create_jetstore_hdl( char const * rete_db_path, char const * lookup_data_db_path, HJETS * handle );
int delete_jetstore_hdl( HJETS handle );

typedef void* HJRETE;
typedef void* HJRDF;

// Create sessions
// ---------------
int create_rdf_session( HJETS jets_hdl, HJRDF * handle );
int delete_rdf_session( HJRDF rdf_session_hdl );

int create_rete_session( HJETS jets_hdl, HJRDF rdf_hdl, char const * jetrule_name, HJRETE * handle );
int delete_rete_session( HJRETE rete_session_hdl );

typedef void const* HJR;

// Creating meta resources and literals
// ------------------------------------
int create_meta_null(HJETS js_hdl, HJR * handle);
int create_meta_blanknode(HJETS js_hdl, int v, HJR * handle);
int create_meta_resource(HJETS js_hdl, char const * name, HJR * handle);
int get_meta_resource(HJETS js_hdl, char const * name, HJR * handle);
int create_meta_text(HJETS js_hdl, char const * name, HJR * handle);
int create_meta_int(HJETS js_hdl, int v, HJR * handle);
int create_meta_uint(HJETS js_hdl, uint v, HJR * handle);
int create_meta_long(HJETS js_hdl, long v, HJR * handle);
int create_meta_ulong(HJETS js_hdl, ulong v, HJR * handle);
int create_meta_double(HJETS js_hdl, double v, HJR * handle);
int create_meta_date(HJETS js_hdl, char const * v, HJR * handle);
int create_meta_datetime(HJETS js_hdl, char const * v, HJR * handle);

int load_process_meta_triples(char const * jetrule_name, int is_rule_set, HJETS js_hdl);
int insert_meta_graph(HJETS js_hdl, HJR s, HJR p, HJR o);

// rdf session methods
// -------------------
// Creating resources and literals
int create_null(HJRDF hdl, HJR * handle);
int create_blanknode(HJRDF hdl, int v, HJR * handle);
int create_resource(HJRDF hdl, char const * name, HJR * handle);
int get_resource(HJRDF hdl, char const * name, HJR * handle);
int create_text(HJRDF hdl, char const * name, HJR * handle);
int create_int(HJRDF hdl, int v, HJR * handle);
int create_uint(HJRDF hdl, uint v, HJR * handle);
int create_long(HJRDF hdl, long v, HJR * handle);
int create_ulong(HJRDF hdl, ulong v, HJR * handle);
int create_double(HJRDF hdl, double v, HJR * handle);
int create_date(HJRDF hdl, char const * v, HJR * handle);
int create_datetime(HJRDF hdl, char const * v, HJR * handle);

typedef void const* HSTR;

// Get the resource name and literal value
int get_resource_type(HJR handle);
int get_resource_name(HJR handle, HSTR*);
char const* get_resource_name2(HJR handle, int*);
int get_int_literal(HJR handle, int*);
int get_double_literal(HJR handle, double*);
int get_text_literal(HJR handle, HSTR*);
char const* get_text_literal2(HJR handle, int*);
int get_date_details(HJR hdl, int* year, int* month, int* day);
int get_datetime_details(HJR hdl, int* year, int* month, int* day, int* hr, int* min, int* sec, int* frac);
int get_date_iso_string(HJR handle, HSTR*);
int get_datetime_iso_string(HJR handle, HSTR*);
char const* get_date_iso_string2(HJR handle, int*);
char const* get_datetime_iso_string2(HJR handle, int*);


// rete session functions
int insert(HJRDF hdl, HJR s, HJR p, HJR o);
int contains(HJRDF hdl, HJR s, HJR p, HJR o);
int contains_sp(HJRDF hdl, HJR s_hdl, HJR p_hdl);
int erase(HJRDF hdl, HJR s, HJR p, HJR o);
int execute_rules(HJRETE rete_hdl);
char const* execute_rules2(HJRETE rete_hdl, int*v);
int dump_rdf_graph(HJRDF hdl);

typedef void* HJITERATOR;

int find_all(HJRDF hdl, HJITERATOR * handle);
int find(HJRDF hdl, HJR s, HJR p, HJR o, HJITERATOR * handle);
int find_s(HJRDF hdl, HJR s, HJITERATOR * handle);
int find_sp(HJRDF hdl, HJR s, HJR p, HJITERATOR * handle);
int find_object(HJRDF hdl, HJR s, HJR p, HJR * handle);
// int find_asserted(HJRDF hdl, HJR s, HJR p, HJR o, HJITERATOR * handle);
// int find_inferred(HJRDF hdl, HJR s, HJR p, HJR o, HJITERATOR * handle);
int is_end(HJITERATOR handle);
int next(HJITERATOR handle);
int get_subject(HJITERATOR itor_hdl, HJR * handle);
int get_predicate(HJITERATOR itor_hdl, HJR * handle);
int get_object(HJITERATOR itor_hdl, HJR * handle);
int dispose(HJITERATOR itor_hdl);

int say_hello();
int say_hello3(char const* name);
void say_hello0();

#ifdef __cplusplus
}
#endif

#endif // JETS_RETE_CWAPPER_H
