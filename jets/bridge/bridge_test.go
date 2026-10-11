package bridge

import (
	"fmt"
	"testing"
)

// This file contains test cases for the bridge package
func TestGetInt(t *testing.T) {

	// load the workspace db
	workspaceDb := "test_data/bridge_test1.db"
	js, err := LoadJetRules("", workspaceDb, "")
	if err != nil {
		t.Errorf("while loading workspace db: %v", err)
	}

	// use the meta graph
	r, err := js.NewIntLiteral(5)
	if err != nil {
		t.Errorf("while calling NewIntLiteral: %v", err)
	}
	v, err := r.GetInt()
	if err != nil {
		t.Errorf("while calling GetInt: %v", err)
	}
	if v != 5 {
		t.Errorf("Did not get 5 back, got %d", v)
	}
}
func TestGetDateDetails(t *testing.T) {
	// load the workspace db
	workspaceDb := "test_data/bridge_test1.db"
	js, err := LoadJetRules("", workspaceDb, "")
	if err != nil {
		t.Errorf("while loading workspace db: %v", err)
	}

	// // use the meta graph
	// _, err = js.TestDateLiteral("JUST ANYTHING")
	// if err != nil {
	// 	t.Errorf("while calling NewDateLiteral: %v",err)
	// }
	// fmt.Println("SO WE GOT...")
	// // fmt.Println(r.AsText())

	r, err := js.NewDateLiteral("2022-05-02")
	if err != nil {
		t.Errorf("while calling NewDateLiteral: %v", err)
	}
	y, m, d, err := r.GetDateDetails()
	if err != nil {
		t.Errorf("while calling GetDateDetails: %v", err)
	}
	if y != 2022 || m != 5 || d != 2 {
		t.Errorf("did not get back 2022-05-02, got %d-%d-%d", y, m, d)
	}

	// Check for invalid date
	r, err = js.NewDateLiteral("2022-55-55")
	if err != ErrNotValidDate {
		t.Errorf("expecting ErrNotValidDate got: %v", err)
	}
	_, _, _, err = r.GetDateDetails()
	if err != ErrNotValidDate {
		t.Errorf("expecting ErrNotValidDate got: %v", err)
	}
}

// create_jetstore_hdl_with_mode is reachable from Go and loads a workspace in each of
// the four LEGACY retraction modes. What the modes do to a network is tested in
// jets/rete/retract_recorded_consequents_test.cc and loop_through_negation_test.cc;
// this only shows the C entry point links and accepts every combination.
func TestLoadJetRulesWithMode(t *testing.T) {
	workspaceDb := "test_data/bridge_test1.db"
	for _, mode := range []struct{ replay, erase bool }{
		{false, false}, {true, false}, {false, true}, {true, true},
	} {
		js, err := LoadJetRulesWithMode("", workspaceDb, "", mode.replay, mode.erase)
		if err != nil {
			t.Fatalf("mode %+v: while loading workspace db: %v", mode, err)
		}
		r, err := js.NewIntLiteral(5)
		if err != nil {
			t.Fatalf("mode %+v: NewIntLiteral: %v", mode, err)
		}
		if v, err := r.GetInt(); err != nil || v != 5 {
			t.Errorf("mode %+v: GetInt = %d, %v", mode, v, err)
		}
		if err := js.ReleaseJetRules(); err != nil {
			t.Errorf("mode %+v: ReleaseJetRules: %v", mode, err)
		}
	}
}

// A blank node's text carries its key, so two blank nodes read as two terms.
// AsText returned a bare "BN:" for every blank node until 2026-10-11, which made
// the rules unit test harness read all of a session's blank nodes, lookup rows
// among them, as one subject (rules_unit_test_harness I-43).
func TestBlankNodeAsText(t *testing.T) {
	js, err := LoadJetRules("", "test_data/bridge_test1.db", "")
	if err != nil {
		t.Fatalf("while loading workspace db: %v", err)
	}
	defer js.ReleaseJetRules()
	for _, key := range []int{1, 2, 4097} {
		r, err := js.NewBlankNode(key)
		if err != nil {
			t.Fatalf("NewBlankNode(%d): %v", key, err)
		}
		if k, err := r.GetBlankNodeKey(); err != nil || k != key {
			t.Errorf("GetBlankNodeKey = %d, %v; want %d", k, err, key)
		}
		want := fmt.Sprintf("BN:%d", key)
		if txt, err := r.AsText(); err != nil || txt != want {
			t.Errorf("AsText = %q, %v; want %q", txt, err, want)
		}
	}
	r, err := js.NewIntLiteral(5)
	if err != nil {
		t.Fatalf("NewIntLiteral: %v", err)
	}
	if _, err := r.GetBlankNodeKey(); err != ErrUnexpectedRdfType {
		t.Errorf("GetBlankNodeKey on an int literal: %v, want ErrUnexpectedRdfType", err)
	}
}
