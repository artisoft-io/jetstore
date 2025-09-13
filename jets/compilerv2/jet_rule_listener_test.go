package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestJetRuleListener_SimpleFile(t *testing.T) {

	listener, err := CompileJetRuleFiles("./testdata", "simple.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fmt.Printf("** Generated model has %d compiler directives, %d classes, %d rules, %d resources, %d lookup tables\n",
		len(listener.jetRuleModel.CompilerDirectives),
		len(listener.jetRuleModel.Classes),
		len(listener.jetRuleModel.Jetrules),
		len(listener.jetRuleModel.Resources),
		len(listener.jetRuleModel.LookupTables))
	t.Error("Done")
}

func TestJetRuleListener_Classes(t *testing.T) {

	listener, err := CompileJetRuleFiles("./testdata", "classes.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(listener.jetRuleModel.Classes, "", " ")
	fmt.Printf("** Classes: \n%v\n", string(b))
	t.Error("Done")
}

func TestJetRuleListener_RuleSequence(t *testing.T) {

	listener, err := CompileJetRuleFiles("./testdata", "rule_sequence.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(listener.jetRuleModel.RuleSequences, "", " ")
	fmt.Printf("** Rule Sequences: \n%v\n", string(b))
	t.Error("Done")
}

func TestJetRuleListener_Resources(t *testing.T) {

	listener, err := CompileJetRuleFiles("./testdata", "resources.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(listener.jetRuleModel.Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	t.Error("Done")
}

func TestJetRuleListener_Lookup(t *testing.T) {

	listener, err := CompileJetRuleFiles("./testdata", "lookup.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(listener.jetRuleModel.LookupTables, "", " ")
	fmt.Printf("** Lookup table: \n%v\n", string(b))
	t.Error("Done")
}

func TestJetRuleListener_JetRule(t *testing.T) {

	listener, err := CompileJetRuleFiles("./testdata", "jetrule.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(listener.jetRuleModel.Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(listener.jetRuleModel.Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	t.Error("Done")
}

func TestJetRuleListener_ParseObjectAtom(t *testing.T) {
	l := JetRuleListener{}
	atom := l.ParseObjectAtom("?clm", "")
	if atom == nil || atom.Type != "var" || 
		atom.Value != "?clm" || atom.Key == 0 {
		t.Errorf("Unexpected result for ?clm: %v", atom)
	}

	atom = l.ParseObjectAtom("ex:SomeClass", "")
	if atom == nil || atom.Type != "identifier" || 
		atom.Value != "ex:SomeClass" || atom.Key == 0 {
		t.Errorf("Unexpected result for ex:SomeClass: %v", atom)
	}	

	atom = l.ParseObjectAtom("localVar", "")
	if atom == nil || atom.Type != "identifier" || 
		atom.Value != "localVar" || atom.Key == 0 {
		t.Errorf("Unexpected result for localVar: %v", atom)
	}

	atom = l.ParseObjectAtom("\"XYZ\"", "")
	if atom == nil || atom.Type != "text" || 
		atom.Value != "XYZ" || atom.Key == 0 {
		t.Errorf("Unexpected result for XYZ: %v", atom)
	}

	atom = l.ParseObjectAtom("text(\"XYZ\")", "")
	if atom == nil || atom.Type != "text" || 
		atom.Value != "XYZ" || atom.Key == 0 {
		t.Errorf("Unexpected result for text(\"XYZ\"): %v", atom)
	}

	atom = l.ParseObjectAtom("1", "")
	if atom == nil || atom.Type != "int" || 
		atom.Value != "1" || atom.Key == 0 {
		t.Errorf("Unexpected result for 1: %v", atom)
	}

	atom = l.ParseObjectAtom("-10", "")
	if atom == nil || atom.Type != "int" || 
		atom.Value != "-10" || atom.Key == 0 {
		t.Errorf("Unexpected result for -10: %v", atom)
	}

	atom = l.ParseObjectAtom("+1.0", "")
	if atom == nil || atom.Type != "double" || 
		atom.Value != "+1.0" || atom.Key == 0 {
		t.Errorf("Unexpected result for +1.0: %v", atom)
	}

	atom = l.ParseObjectAtom("int(1)", "")
	if atom == nil || atom.Type != "int" || 
		atom.Value != "1" || atom.Key == 0 {
		t.Errorf("Unexpected result for int(1): %v", atom)
	}

	atom = l.ParseObjectAtom("bool(1)", "")
	if atom == nil || atom.Type != "bool" || 
		atom.Value != "1" || atom.Key == 0 {
		t.Errorf("Unexpected result for bool(1): %v", atom)
	}

	atom = l.ParseObjectAtom("", "true")
	if atom == nil || atom.Type != "keyword" || 
		atom.Value != "true" || atom.Key == 0 {
		t.Errorf("Unexpected result for true: %v", atom)
	}

}
