package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

func TestJetRuleListener_SimpleFile(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "simple.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fmt.Printf("** Generated model has %d compiler directives, %d classes, %d rules, %d resources, %d lookup tables\n",
		len(jrCompiler.JetRuleModel().CompilerDirectives),
		len(jrCompiler.JetRuleModel().Classes),
		len(jrCompiler.JetRuleModel().Jetrules),
		len(jrCompiler.JetRuleModel().Resources),
		len(jrCompiler.JetRuleModel().LookupTables))
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Classes(t *testing.T) {
	// delete file "./testdata/workspace.db" if it exists
	if _, err := os.Stat("./testdata/workspace.db"); err == nil {
		if err := os.Remove("./testdata/workspace.db"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	jrCompiler, err := CompileJetRuleFiles("./testdata", "classes.jr", true, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Classes, "", " ")
	fmt.Printf("** Classes: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.listener.currentRuleVarByValue, "", " ")
	fmt.Printf("** Variable by Value: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	jrCompiler.listener.PostProcessJetruleModel()
	fmt.Printf("** owl:Thing sub classes: \n%v\n", jrCompiler.listener.classesByName["owl:Thing"].SubClasses)
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Tables(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "tables.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Classes, "", " ")
	fmt.Printf("** Classes: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Tables, "", " ")
	fmt.Printf("** Tables: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.listener.currentRuleVarByValue, "", " ")
	// fmt.Printf("** Variable by Value: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	jrCompiler.listener.PostProcessJetruleModel()
	fmt.Printf("** owl:Thing sub classes: \n%v\n", jrCompiler.listener.classesByName["owl:Thing"].SubClasses)
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_JetRuleConfig(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "jetrule_config.jr", true, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().JetstoreConfig, "", " ")
	fmt.Printf("** Jetstore Config: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_RuleSequence(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "rule_sequence.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().RuleSequences, "", " ")
	fmt.Printf("** Rule Sequences: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Resources(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "resources.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Resources_err1(t *testing.T) {

	jrCompiler := NewCompiler("./testdata", "resources_err1.jr", false, true, false)
	// The fixture is invalid by design: the compile is expected to report
	// errors, and returning them is not a test failure.
	err := jrCompiler.Compile()
	if err == nil {
		t.Error("expected the compile to fail on an invalid fixture")
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	assertErrorLog(t, jrCompiler.ErrorLog().String(),
		"resource Id conflict for Id 'always'",
		"resource Id conflict for Id 'never'")
}

func TestJetRuleListener_Lookup(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "lookup.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().LookupTables, "", " ")
	fmt.Printf("** Lookup table: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_JetRule0(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "jetrule0.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_JetRule_err1(t *testing.T) {

	jrCompiler := NewCompiler("./testdata", "jetrule_err1.jr", false, true, false)
	// The fixture is invalid by design: the compile is expected to report
	// errors, and returning them is not a test failure.
	err := jrCompiler.Compile()
	if err == nil {
		t.Error("expected the compile to fail on an invalid fixture")
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	assertErrorLog(t, jrCompiler.ErrorLog().String(),
		"antecedent must have at least one of subject, predicate, object as variable",
		"consequent subject variable var|?config not found in antecedents")
}

func TestJetRuleListener_JetRule_err2(t *testing.T) {

	jrCompiler := NewCompiler("./testdata", "jetrule_err2.jr", false, true, false)
	// The fixture is invalid by design: the compile is expected to report
	// errors, and returning them is not a test failure.
	err := jrCompiler.Compile()
	if err == nil {
		t.Error("expected the compile to fail on an invalid fixture")
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	assertErrorLog(t, jrCompiler.ErrorLog().String(),
		"consequent object expression variable var|?v1 not found in antecedents")
}

func TestJetRuleListener_InlineLiteral(t *testing.T) {

	jrCompiler := NewCompiler("./testdata", "jetrule.jr", false, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "rete_test1.jr";
	resource abc:RuleConfig = "abc:RuleConfig";

	[EID_Relationship_Code_10]:
  (?e rdf:type abc:RuleConfig)
-> 
  (?e jets:ruleTag "Missing Relationship_Code");`)
	if err != nil {
		t.Fatal(err.Error())
	}

	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())

	// This test expected an error, from a time when a literal written inline in
	// a consequent was not supported. It is: the corpus is full of them
	// (workspaces/usi_ws alone has hundreds of (?x hc_usi:meetRule "...")), and
	// the compiler records the literal as an anonymous inline text resource
	// which the consequent then refers to. What the test checks now is that.
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error(jrCompiler.ErrorLog().String())
	}
	var inline *rete.ResourceNode
	for i, r := range jrCompiler.JetRuleModel().Resources {
		if r.Inline && r.Type == "text" && r.Value == "Missing Relationship_Code" {
			inline = jrCompiler.JetRuleModel().Resources[i]
		}
	}
	if inline == nil {
		t.Fatal("the inline literal is not in the model as an inline text resource")
	}
	if inline.Id != "" {
		t.Errorf("an inline literal is anonymous; this one has id %q", inline.Id)
	}
	if len(jrCompiler.JetRuleModel().Jetrules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(jrCompiler.JetRuleModel().Jetrules))
	}
	consequents := jrCompiler.JetRuleModel().Jetrules[0].Consequents
	if len(consequents) != 1 ||
		consequents[0].NormalizedLabel != "(?x01 jets:ruleTag text(Missing Relationship_Code))" {
		t.Errorf("the consequent does not refer to the literal: %+v", consequents)
	}
}

// assertErrorLog fails unless every wanted diagnostic is in the compiler's
// error log, printing the log when it is not — a fixture that stops producing
// the error it exists for should say which error went missing.
func assertErrorLog(t *testing.T, errorLog string, want ...string) {
	t.Helper()
	if errorLog == "" {
		t.Errorf("expected the compile to report errors, the error log is empty")
		return
	}
	for _, w := range want {
		if !strings.Contains(errorLog, w) {
			t.Errorf("the error log does not report %q:\n%s", w, errorLog)
		}
	}
}

func TestJetRuleListener_Triples(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "triples.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Triples, "", " ")
	fmt.Printf("** Triples: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error("unexpected errors found")
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Triples_err1(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "triples_err1.jr", false, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Triples, "", " ")
	fmt.Printf("** Triples: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error("unexpected error:", jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Triples_autoadd(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "triples_autoadd.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Triples, "", " ")
	fmt.Printf("** Triples: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Error("unexpected error:", jrCompiler.ErrorLog().String())
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_EscR(t *testing.T) {
	s := EscR("rdf:\"type\"")
	if s != "rdf:type" {
		t.Errorf("Unexpected result for rdf:\"type\": %s", s)
	}
	s = EscR("\"rdf:\"type\"\"")
	if s != "\"rdf:\"type\"\"" {
		t.Errorf("Unexpected result for \"rdf:\"type\"\": %s", s)
	}
	s = EscR("ex:SomeClass")
	if s != "ex:SomeClass" {
		t.Errorf("Unexpected result for ex:SomeClass: %s", s)
	}
	s = EscR("localVar")
	if s != "localVar" {
		t.Errorf("Unexpected result for localVar: %s", s)
	}
	s = EscR("\"XYZ\"")
	if s != "\"XYZ\"" {
		t.Errorf("Unexpected result for \"XYZ\": %s", s)
	}
}

func TestJetRuleListener_ParseObjectAtom(t *testing.T) {
	jrCompiler := NewCompiler("./testdata", "jetrule.jr", false, true, false)
	s := jrCompiler.listener
	atom := s.parseObjectAtom("?clm", "")
	if atom.Type != "var" ||
		atom.Value != "?clm" {
		t.Errorf("Unexpected result for ?clm: %v", atom)
	}

	atom = s.parseObjectAtom("ex:SomeClass", "")
	if atom.Type != "identifier" ||
		atom.Id != "ex:SomeClass" {
		t.Errorf("Unexpected result for ex:SomeClass: %v", atom)
	}

	atom = s.parseObjectAtom("localVar", "")
	if atom.Type != "identifier" ||
		atom.Id != "localVar" {
		t.Errorf("Unexpected result for localVar: %v", atom)
	}

	atom = s.parseObjectAtom("\"XYZ\"", "")
	if atom.Type != "text" ||
		atom.Value != "XYZ" {
		t.Errorf("Unexpected result for XYZ: %v", atom)
	}

	atom = s.parseObjectAtom("text(\"XYZ\")", "")
	if atom.Type != "text" ||
		atom.Value != "XYZ" {
		t.Errorf("Unexpected result for text(\"XYZ\"): %v", atom)
	}

	atom = s.parseObjectAtom("xyz(\"XYZ\")", "")
	if atom != nil {
		t.Errorf("Unexpected nil result for xyz(\"XYZ\"): %v", atom)
	}

	// special case seen when using literal_regex operator
	atom = s.parseObjectAtom("\"^(XYZ)\"", "")
	if atom.Type != "text" ||
		atom.Value != "^(XYZ)" {
		t.Errorf("Unexpected result for ^(XYZ): %v", atom)
	}

	atom = s.parseObjectAtom("1", "")
	if atom.Type != "int" ||
		atom.Value != "1" {
		t.Errorf("Unexpected result for 1: %v", atom)
	}

	atom = s.parseObjectAtom("-10", "")
	if atom.Type != "int" ||
		atom.Value != "-10" {
		t.Errorf("Unexpected result for -10: %v", atom)
	}

	atom = s.parseObjectAtom("+1.0", "")
	if atom.Type != "double" ||
		atom.Value != "+1.0" {
		t.Errorf("Unexpected result for +1.0: %v", atom)
	}

	atom = s.parseObjectAtom("int(1)", "")
	if atom.Type != "int" ||
		atom.Value != "1" {
		t.Errorf("Unexpected result for int(1): %v", atom)
	}

	atom = s.parseObjectAtom("bool(1)", "")
	if atom.Type != "bool" ||
		atom.Value != "1" {
		t.Errorf("Unexpected result for bool(1): %v", atom)
	}

	atom = s.parseObjectAtom("", "true")
	if atom.Type != "keyword" ||
		atom.Value != "true" {
		t.Errorf("Unexpected result for true: %v", atom)
	}

}

// `deleted` in the model reaches the table column as a tombstone.
//
// **The whole point is that nothing else drops a domain column.** A column that
// merely stops appearing in the model is reported as deprecated and left alone,
// because diffing the model against the database would make a bad config cost
// data — the same rule `jets_schema.json` already applies to the JetStore
// tables. So this flag is the only path, and it has to survive the grammar, the
// listener and MakeTableFromClass to be one.
func TestJetRuleListener_DeletedColumns(t *testing.T) {
	jrCompiler, err := CompileJetRuleFiles("./testdata", "deleted_columns.jr", false, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jrCompiler.ErrorLog().Len() > 0 {
		t.Fatal(jrCompiler.ErrorLog().String())
	}

	var table *rete.TableNode
	for _, tbl := range jrCompiler.JetRuleModel().Tables {
		if tbl.TableName == "Retired" {
			table = tbl
		}
	}
	if table == nil {
		t.Fatalf("no table for class Retired; got %d tables", len(jrCompiler.JetRuleModel().Tables))
	}

	deleted := make(map[string]bool)
	for _, c := range table.Columns {
		deleted[c.ColumnName] = c.Deleted
	}
	for name, want := range map[string]bool{
		"liveText":          false,
		"liveArray":         false,
		"retiredText":       true,
		"retiredArray":      true,
		"liveObject":        false,
		"staleObjectColumn": true,
	} {
		got, present := deleted[name]
		if !present {
			t.Errorf("%s missing from the table columns", name)
			continue
		}
		if got != want {
			t.Errorf("%s: Deleted = %v, want %v", name, got, want)
		}
	}
	// `deleted` must not disturb the flags beside it: a tombstone on an array
	// property is still an array, and on an object property still an object.
	for _, c := range table.Columns {
		switch c.ColumnName {
		case "retiredArray":
			if !c.AsArray {
				t.Error("retiredArray lost AsArray next to deleted")
			}
		case "staleObjectColumn":
			if !c.IsObject || !c.AsArray {
				t.Errorf("staleObjectColumn lost its flags next to deleted: %+v", c)
			}
		}
	}
}
