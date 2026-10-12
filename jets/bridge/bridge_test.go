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

// Long and unsigned literals read back whole. GetInt went through a C int, so an
// int64 above 2^31 was narrowed; AsText refused int64 and both unsigned types; and
// AsInterface, which RdfNodeNative.Value calls, had no case for unsigned literals
// or blank nodes (rules_unit_test_harness I-12).
func TestLongUnsignedAndBlankNodeValues(t *testing.T) {
	js, err := LoadJetRules("", "test_data/bridge_test1.db", "")
	if err != nil {
		t.Fatalf("while loading workspace db: %v", err)
	}
	defer js.ReleaseJetRules()

	const big = 1 << 40
	r, err := js.NewLongLiteral(big)
	if err != nil {
		t.Fatalf("NewLongLiteral: %v", err)
	}
	if v, err := r.GetInt(); err != nil || v != big {
		t.Errorf("long GetInt = %d, %v; want %d", v, err, big)
	}
	if v, err := r.AsInterface(""); err != nil || v != big {
		t.Errorf("long AsInterface = %v, %v; want %d", v, err, big)
	}
	if txt, err := r.AsText(); err != nil || txt != "1099511627776" {
		t.Errorf("long AsText = %q, %v", txt, err)
	}

	for _, c := range []struct {
		name string
		make func() (*Resource, error)
		want uint
	}{
		{"uint", func() (*Resource, error) { return js.NewUIntLiteral(7) }, 7},
		{"ulong", func() (*Resource, error) { return js.NewULongLiteral(1<<63 + 5) }, 1<<63 + 5},
	} {
		r, err := c.make()
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if v, err := r.GetUInt(); err != nil || v != c.want {
			t.Errorf("%s GetUInt = %d, %v; want %d", c.name, v, err, c.want)
		}
		if v, err := r.AsInterface(""); err != nil || v != c.want {
			t.Errorf("%s AsInterface = %v (%T), %v; want uint %d", c.name, v, v, err, c.want)
		}
		if txt, err := r.AsText(); err != nil || txt != fmt.Sprint(c.want) {
			t.Errorf("%s AsText = %q, %v", c.name, txt, err)
		}
		if _, err := r.AsInterface("text"); err == nil {
			t.Errorf("%s AsInterface(\"text\"): no type error", c.name)
		}
	}
	if _, err := r.GetUInt(); err == nil {
		t.Errorf("GetUInt on a signed literal: no error")
	}

	bn, err := js.NewBlankNode(3)
	if err != nil {
		t.Fatalf("NewBlankNode: %v", err)
	}
	if v, err := bn.AsInterface(""); err != nil || v != "BN:3" {
		t.Errorf("blank node AsInterface = %v, %v; want BN:3", v, err)
	}
}

// SameAs is identity on the engine's handle. Every call returns a new *Resource,
// so comparing those pointers, as RdfNodeNative.Equals did, was false for the
// same node (rules_unit_test_harness I-11).
func TestSameAs(t *testing.T) {
	js, err := LoadJetRules("", "test_data/bridge_test1.db", "")
	if err != nil {
		t.Fatalf("while loading workspace db: %v", err)
	}
	defer js.ReleaseJetRules()
	a, _ := js.NewResource("same_as_r")
	b, _ := js.NewResource("same_as_r")
	if a == b {
		t.Fatalf("two calls returned one *Resource; the test needs two")
	}
	if !a.SameAs(b) {
		t.Errorf("one resource read twice: SameAs false")
	}
	c, _ := js.NewResource("same_as_s")
	if a.SameAs(c) {
		t.Errorf("two resources: SameAs true")
	}
	i2, _ := js.NewIntLiteral(2)
	i2b, _ := js.NewIntLiteral(2)
	u2, _ := js.NewUIntLiteral(2)
	d2, _ := js.NewDoubleLiteral(2)
	if !i2.SameAs(i2b) {
		t.Errorf("int 2 twice: SameAs false")
	}
	if i2.SameAs(u2) || i2.SameAs(d2) {
		t.Errorf("int 2 is the same node as uint 2 or double 2; literals are typed")
	}
	if a.SameAs(nil) {
		t.Errorf("SameAs(nil) true")
	}
}

// The meta graph locks when the first RDF session opens. Creating a new meta
// resource after that threw an exception across cgo and aborted the process; it
// is now an error. What the meta graph already holds is still returned, as the Go
// engine's ResourceManager does (rules_unit_test_harness I-15).
func TestMetaGraphAfterSessionOpens(t *testing.T) {
	js, err := LoadJetRules("", "test_data/bridge_test1.db", "")
	if err != nil {
		t.Fatalf("while loading workspace db: %v", err)
	}
	defer js.ReleaseJetRules()
	before, err := js.NewResource("made_before_the_session")
	if err != nil {
		t.Fatalf("NewResource before the session: %v", err)
	}
	five, err := js.NewIntLiteral(5)
	if err != nil {
		t.Fatalf("NewIntLiteral before the session: %v", err)
	}
	rdfs, err := js.NewRDFSession()
	if err != nil {
		t.Fatalf("NewRDFSession: %v", err)
	}
	defer rdfs.ReleaseRDFSession()

	again, err := js.NewResource("made_before_the_session")
	if err != nil || !again.SameAs(before) {
		t.Errorf("an existing meta resource after the lock: %v, same %v", err, again.SameAs(before))
	}
	if r, err := js.NewIntLiteral(5); err != nil || !r.SameAs(five) {
		t.Errorf("an existing meta literal after the lock: %v", err)
	}
	if _, err := js.NewResource("made_after_the_session"); err == nil {
		t.Errorf("a new meta resource after the lock: no error")
	}
	if _, err := js.NewIntLiteral(424242); err == nil {
		t.Errorf("a new meta literal after the lock: no error")
	}
	if _, err := js.InsertRuleConfig(before, before, five); err == nil {
		t.Errorf("a meta graph insert after the lock: no error")
	}
}
