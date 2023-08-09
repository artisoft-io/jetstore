#ifndef JETS_RDF_R_RESOURCES_H
#define JETS_RDF_R_RESOURCES_H

#include <string>

#include "../rdf/rdf_err.h"
#include "../rdf/rdf_ast.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rdf {
class RManager;
using RManagerPtr = std::shared_ptr<RManager>;

/////////////////////////////////////////////////////////////////////////////////////////
// JetsResources is a cache of resources for rete exper
struct JetsResources {

  void
  initialize(RManager * rmgr);

  inline bool
  is_initialized()const
  {
    if(not this->jets__entity_property) return false;
    return true;
  }

  r_index jets__client{nullptr};
  r_index jets__completed{nullptr};
  r_index jets__entity_property{nullptr};
  r_index jets__exception{nullptr};
  r_index jets__from{nullptr};
  r_index jets__input_record{nullptr};
  r_index jets__istate{nullptr};
  r_index jets__key{nullptr};
  r_index jets__length{nullptr};
  r_index jets__lookup_multi_rows{nullptr};
  r_index jets__lookup_row{nullptr};
  r_index jets__loop{nullptr};
  r_index jets__operator{nullptr};
  r_index jets__org{nullptr};
  r_index jets__replace_chars{nullptr};
  r_index jets__replace_with{nullptr};
  r_index jets__source_period_sequence{nullptr};
  r_index jets__state{nullptr};
  r_index jets__value_property{nullptr};
  r_index rdf__type{nullptr};

};

} // namespace jets::rdf
#endif // JETS_RDF_R_RESOURCES_H
