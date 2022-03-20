
#include "r_resources.h"
#include "r_manager.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rdf {

void
JetsResources::initialize(RManager * rmgr)
{
  if(this->is_initialized()) return;
  this->jets__entity_property = rmgr->create_resource("jets:entity_property");
  this->jets__value_property  = rmgr->create_resource("jets:value_property");
  this->jets__key             = rmgr->create_resource("jets:key");
}

} // namespace jets::rdf
