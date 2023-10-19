package rdf

import "github.com/artisoft-io/jetstore/jets/bridge"


type RdfResources struct {
	jets__client                     *bridge.Resource
	jets__completed                  *bridge.Resource
	jets__currentSourcePeriod        *bridge.Resource
	jets__currentSourcePeriodDate    *bridge.Resource
	jets__exception                  *bridge.Resource
	jets__input_record               *bridge.Resource
	jets__istate                     *bridge.Resource
	jets__key                        *bridge.Resource
	jets__loop                       *bridge.Resource
	jets__org                        *bridge.Resource
	jets__source_period_sequence     *bridge.Resource
	jets__sourcePeriodType           *bridge.Resource
	jets__state                      *bridge.Resource
	rdf__type                        *bridge.Resource
}

func NewRdfResources(js *bridge.JetStore) *RdfResources {
	var ri RdfResources 
	ri.jets__client,_ = js.GetResource("jets:client")
	ri.jets__completed,_ = js.GetResource("jets:completed")
	ri.jets__currentSourcePeriod,_ = js.GetResource("jets:currentSourcePeriod")
	ri.jets__currentSourcePeriodDate,_ = js.GetResource("jets:currentSourcePeriodDate")
	ri.jets__exception,_ = js.GetResource("jets:exception")
	ri.jets__input_record,_ = js.GetResource("jets:InputRecord")
	ri.jets__istate,_ = js.GetResource("jets:iState")
	ri.jets__key,_ = js.GetResource("jets:key")
	ri.jets__loop,_ = js.GetResource("jets:loop")
	ri.jets__org,_ = js.GetResource("jets:org")
	ri.jets__source_period_sequence,_ = js.GetResource("jets:source_period_sequence")
	ri.jets__sourcePeriodType,_ = js.GetResource("jets:sourcePeriodType")
	ri.jets__state,_ = js.GetResource("jets:State")
	ri.rdf__type,_ = js.GetResource("rdf:type")
	return &ri
}
