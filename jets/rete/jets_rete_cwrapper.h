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

int create_rete_session( HJETS jets_hdl, char const * jetrule_name, HJRETE * handle );
int delete_rete_session( HJRETE rete_session_hdl );

typedef void const* HJR;

// Creating meta resources and literals
int create_null(HJETS js_hdl, HJR * handle);
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

int insert_meta_graph(HJETS js_hdl, HJR s, HJR p, HJR o);

// Creating resources and literals
int create_blanknode(HJRETE rete_hdl, int v, HJR * handle);
int create_resource(HJRETE rete_hdl, char const * name, HJR * handle);
int get_resource(HJRETE rete_hdl, char const * name, HJR * handle);
int create_text(HJRETE rete_hdl, char const * name, HJR * handle);
int create_int(HJRETE rete_hdl, int v, HJR * handle);
int create_uint(HJRETE rete_hdl, uint v, HJR * handle);
int create_long(HJRETE rete_hdl, long v, HJR * handle);
int create_ulong(HJRETE rete_hdl, ulong v, HJR * handle);
int create_double(HJRETE rete_hdl, double v, HJR * handle);
int create_date(HJRETE rete_hdl, char const * v, HJR * handle);
int create_datetime(HJRETE rete_hdl, char const * v, HJR * handle);

typedef void const* HSTR;

// Get the resource name and literal value
int get_resource_type(HJR handle);
int get_resource_name(HJR handle, HSTR*);
char const* go_get_resource_name(HJR handle);
int get_int_literal(HJR handle, int*);
int get_text_literal(HJR handle, HSTR*);
char const* go_get_text_literal(HJR handle);

// main functions
int insert(HJRETE rete_hdl, HJR s, HJR p, HJR o);
int contains(HJRETE rete_hdl, HJR s, HJR p, HJR o);
int execute_rules(HJRETE rete_hdl);

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

int say_hello();
int say_hello3(char const* name);
void say_hello0();

#ifdef __cplusplus
}
#endif

#endif // JETS_RETE_CWAPPER_H
