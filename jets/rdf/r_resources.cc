
#include "r_resources.h"
#include "r_manager.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rdf {

void
JetsResources::initialize(RManager * rmgr)
{
  if(this->is_initialized()) return;
  rmgr->insert_item(mkNull());
  this->jets__client                   = rmgr->create_resource("jets:client");
  this->jets__completed                = rmgr->create_resource("jets:completed");
  this->jets__entity_property          = rmgr->create_resource("jets:entity_property");
  this->jets__exception                = rmgr->create_resource("jets:exception");
  this->jets__input_record             = rmgr->create_resource("jets:InputRecord");
  this->jets__istate                   = rmgr->create_resource("jets:iState");
  this->jets__key                      = rmgr->create_resource("jets:key");
  this->jets__lookup_multi_rows        = rmgr->create_resource("jets:lookup_multi_rows");
  this->jets__lookup_row               = rmgr->create_resource("jets:lookup_row");
  this->jets__loop                     = rmgr->create_resource("jets:loop");
  this->jets__operator                 = rmgr->create_resource("jets:operator");
  this->jets__org                      = rmgr->create_resource("jets:org");
  this->jets__source_period_sequence   = rmgr->create_resource("jets:source_period_sequence");
  this->jets__state                    = rmgr->create_resource("jets:State");
  this->jets__value_property           = rmgr->create_resource("jets:value_property");
  this->rdf__type                      = rmgr->create_resource("rdf:type");
}

} // namespace jets::rdf
