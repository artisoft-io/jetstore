#ifndef JETS_RETE_TYPES_H
#define JETS_RETE_TYPES_H

#include "jets/rdf/rdf_types.h"

#include "jets/rete/rete_err.h"
#include "jets/rete/node_vertex.h"
#include "jets/rete/beta_row_initializer.h"
#include "jets/rete/beta_row.h"
#include "jets/rete/beta_row_iterator.h"
#include "jets/rete/beta_relation.h"
#include "jets/rete/alpha_node.h"
#include "jets/rete/expr.h"
#include "jets/rete/rete_meta_store.h"
#include "jets/rete/graph_callback_mgr_impl.h"
#include "jets/rete/expr_impl.h"
#include "jets/rete/rete_session.h"

namespace jets::rete {
/////////////////////////////////////////////////////////////////////////////////////////
// ReteMetaStore with  RDFSession  -- with STL containers -- suffix STL
using ReteMetaStoreStl = ReteMetaStore<rdf::RDFSessionStlImpl>;
using ReteMetaStoreStlPtr = ReteMetaStorePtr<rdf::RDFSessionStlImpl>;

using ReteSessionStl = ReteSession<rdf::RDFSessionStlImpl>;
using ReteSessionStlPtr = ReteSessionPtr<rdf::RDFSessionStlImpl>;

} // namespace jets::rete
#endif // JETS_RETE_TYPES_H
