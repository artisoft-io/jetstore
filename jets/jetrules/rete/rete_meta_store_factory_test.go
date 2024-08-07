package rete

import (
	"testing"
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
}