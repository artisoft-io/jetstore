package rdf

import (
	"github.com/artisoft-io/jetstore/jets/bridgego"
)

type RdfResources struct {
	jets__client                  *bridgego.Resource
	jets__completed               *bridgego.Resource
	jets__currentSourcePeriod     *bridgego.Resource
	jets__currentSourcePeriodDate *bridgego.Resource
	jets__exception               *bridgego.Resource
	jets__input_record            *bridgego.Resource
	jets__istate                  *bridgego.Resource
	jets__key                     *bridgego.Resource
	jets__loop                    *bridgego.Resource
	jets__org                     *bridgego.Resource
	jets__source_period_sequence  *bridgego.Resource
	jets__sourcePeriodType        *bridgego.Resource
	jets__state                   *bridgego.Resource
	rdf__type                     *bridgego.Resource
}

func NewRdfResources(js *bridgego.JetStore) *RdfResources {
	var ri RdfResources
	ri.jets__client, _ = js.GetResource("jets:client")
	ri.jets__completed, _ = js.GetResource("jets:completed")
	ri.jets__currentSourcePeriod, _ = js.GetResource("jets:currentSourcePeriod")
	ri.jets__currentSourcePeriodDate, _ = js.GetResource("jets:currentSourcePeriodDate")
	ri.jets__exception, _ = js.GetResource("jets:exception")
	ri.jets__input_record, _ = js.GetResource("jets:InputRecord")
	ri.jets__istate, _ = js.GetResource("jets:iState")
	ri.jets__key, _ = js.GetResource("jets:key")
	ri.jets__loop, _ = js.GetResource("jets:loop")
	ri.jets__org, _ = js.GetResource("jets:org")
	ri.jets__source_period_sequence, _ = js.GetResource("jets:source_period_sequence")
	ri.jets__sourcePeriodType, _ = js.GetResource("jets:sourcePeriodType")
	ri.jets__state, _ = js.GetResource("jets:State")
	ri.rdf__type, _ = js.GetResource("rdf:type")
	return &ri
}
