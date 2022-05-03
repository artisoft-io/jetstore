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
func TestGetDateDetails(t *testing.T) {
	
	// load the workspace db
	workspaceDb := "test_data/bridge_test1.db"
	js, err := LoadJetRules(workspaceDb, "")
	if err != nil {
		t.Errorf("while loading workspace db: %v", err)
	}

	// use the meta graph
	r, err := js.NewDateLiteral("2022-05-02")
	if err != nil {
		t.Errorf("while calling NewDateLiteral: %v",err)
	}
	y, m, d, err := r.GetDateDetails()
	if err != nil {
		t.Errorf("while calling GetDateDetails: %v",err)
	}
	if y!=2022 || m !=5 || d!=2 {
		t.Errorf("did not get back 2022-05=02, got %d-%d-%d", y, m, d)
	}

	// Check for invalid date
	r, err = js.NewDateLiteral("2022-55-55")
	if err != nil {
		t.Errorf("while calling NewDateLiteral: %v",err)
	}
	_, _, _, err = r.GetDateDetails()
	if err != NotValidDate {
		t.Errorf("expecting NotValidDate got: %v",err)
	}
}