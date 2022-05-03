package bridge

import (
	"testing"
)

// This file contains test cases for the bridge package
func TestGetInt(t *testing.T) {
	
	// load the workspace db
	workspaceDb := "test_data/bridge_test1.db"
	js, err := LoadJetRules(workspaceDb, "")
	if err != nil {
		t.Errorf("while loading workspace db: %v", err)
	}

	// use the meta graph
	r, err := js.NewIntLiteral(5)
	if err != nil {
		t.Errorf("while calling NewIntLiteral: %v",err)
	}
	v, err := r.GetInt()
	if err != nil {
		t.Errorf("while calling GetInt: %v",err)
	}
	if v != 5 {
		t.Errorf("Did not get 5 back, got %d", v)
	}
}