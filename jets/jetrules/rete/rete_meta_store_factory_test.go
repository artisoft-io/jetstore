package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func TestReteMetaStoreFactory(t *testing.T) {
	workspaceHome = "/home/michel/projects/repos/jetstore/jets/jetrules"
	wprefix = "test_ws"
	factory, err := NewReteMetaStoreFactory("jet_rules/map_eligibility_main.jr")
	if err != nil {
		t.Fatalf("while calling NewReteMetaStoreFactory: %v", err)
	}
	if factory == nil {
		t.Fatalf("error: NewReteMetaStoreFactory returned nil")
	}
	if factory.ResourceMgr == nil || factory.MetaStoreLookup == nil {
		t.Fatalf("error: ReteMetaStoreFactory has nil ResourceMgr or MetaStoreLookup")
	}
	reteMetaStore := factory.MetaStoreLookup["jet_rules/map_eligibility_main.jr"]
	if reteMetaStore == nil {
		t.Fatalf("error: ReteMetaStoreFactory has nil ReteMetaStore for jet_rules/map_eligibility_main.jr")
	}

	// Create the session
	metaMgr := rdf.NewResourceManager(nil)
	metaGraph := rdf.NewRdfGraph("META")
	rdfSession := rdf.NewRdfSession(metaMgr, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	// rm := rdfSession.ResourceMgr
	reteSession := NewReteSession(rdfSession)
	reteSession.Initialize(reteMetaStore)
	if reteSession.maxVertexVisits != 150 {
		t.Error("Expecting max_looping = 150")
	}
}