#ifndef JETS_RETE_CWAPPER_H
#define JETS_RETE_CWAPPER_H

#include <stdint.h>
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

// A non-zero return with *handle set means the session was built and its initialize
// failed: get_rete_session_err_msg says why, and delete_rete_session releases it.
int create_rete_session( HJETS jets_hdl, HJRDF rdf_hdl, char const * jetrule_name, HJRETE * handle );
int delete_rete_session( HJRETE rete_session_hdl );
// The session's last error, or "" if none; valid until the session's next call
char const* get_rete_session_err_msg( HJRETE rete_session_hdl );
// What the session's network holds, per vertex: rows_now the rows of its beta
// relation at the call, rows_peak the most it held in this session, inferred and
// retracted the times its consequents were computed and retracted (the visits the
// vertex visit limit counts; 0 at a vertex with no consequent terms). Each array has room for n vertices. Returns the
// number of vertices of the network, filling the first min(n, that) of each; call
// it with n = 0 for the count alone. Free: every counter is kept by the session
// anyway; nothing is recorded for this call. -1 for a null handle or array.
int get_vertex_observations( HJRETE rete_session_hdl, int n, int64_t * rows_now,
  int64_t * rows_peak, int64_t * inferred, int64_t * retracted );

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
int create_meta_date_details(HJETS hdl, int year, int month, int day, HJR * handle);
int create_meta_datetime_details(HJETS hdl, int year, int month, int day, int hr, int min, int sec, HJR * handle);
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
int create_date_details(HJRDF hdl, int year, int month, int day, HJR * handle);
int create_datetime_details(HJRDF hdl, int year, int month, int day, int hr, int min, int sec, HJR * handle);
int create_datetime(HJRDF hdl, char const * v, HJR * handle);
char const* get_rdf_graph_txt(HJRDF hdl, int*v);

typedef void const* HSTR;

// Get the resource name and literal value
int get_resource_type(HJR handle);
int get_resource_name(HJR handle, HSTR*);
char const* get_resource_name2(HJR handle, int*);
int get_blank_node_key(HJR handle, int*);
int get_int_literal(HJR handle, int*);
// get_long_literal reads an int32 or int64 literal, and get_ulong_literal a
// uint32 or uint64 one, without narrowing to an int.
int get_long_literal(HJR handle, long*);
int get_ulong_literal(HJR handle, ulong*);
int get_double_literal(HJR handle, double*);
int get_text_literal(HJR handle, HSTR*);
char const* get_text_literal2(HJR handle, int*);
int get_date_details(HJR hdl, int* year, int* month, int* day);
int get_datetime_details(HJR hdl, int* year, int* month, int* day, int* hr, int* min, int* sec, int* frac);
// The ISO text of a date or datetime, written with its terminating zero into buf,
// of len bytes: 0; -1 for a null handle or buffer or another type; -2 for not a
// date; -4 when it does not fit (64 bytes always do)
int get_date_iso_string3(HJR handle, char* buf, int len);
int get_datetime_iso_string3(HJR handle, char* buf, int len);
// DEPRECATED, not safe from Go: the text is in a buffer of the calling thread,
// overwritten by its next ISO getter call (see jets_rete_cwrapper.cpp)
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

// An iterator over a session's graph, from find_all, find, find_s or find_sp.
// A session's graph is single-threaded, even when it is only read: creating,
// advancing and releasing an iterator write the graph's registry of live
// iterators, so one session must not be used from two threads at once.
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
