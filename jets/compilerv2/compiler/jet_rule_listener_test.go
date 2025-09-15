package compiler

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestJetRuleListener_SimpleFile(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "simple.jr", true)
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

	jrCompiler, err := CompileJetRuleFiles("./testdata", "classes.jr", true)
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

	jrCompiler, err := CompileJetRuleFiles("./testdata", "tables.jr", true)
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

func TestJetRuleListener_RuleSequence(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "rule_sequence.jr", true)
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

	jrCompiler, err := CompileJetRuleFiles("./testdata", "resources.jr", true)
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

	jrCompiler, err := CompileJetRuleFiles("./testdata", "resources_err1.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() == 0 {
		t.Error("Expected error but none found")
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_Lookup(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "lookup.jr", true)
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

	jrCompiler, err := CompileJetRuleFiles("./testdata", "jetrule0.jr", true)
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

	jrCompiler, err := CompileJetRuleFiles("./testdata", "jetrule_err1.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() == 0 {
		t.Error("Expected error but none found")
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_JetRule_err2(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "jetrule_err2.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() == 0 {
		t.Error("Expected error but none found")
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_JetRule(t *testing.T) {

	jrCompiler, err := CompileJetRuleFiles("./testdata", "jetrule.jr", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	fmt.Printf("** Error Log: \n%v\n", jrCompiler.ErrorLog().String())
	if jrCompiler.ErrorLog().Len() == 0 {
		t.Error("Expected error but none found")
	} else {
		// t.Error("Done")
	}
}

func TestJetRuleListener_ParseObjectAtom(t *testing.T) {
	atom := parseObjectAtom("?clm", "")
	if atom.Type != "var" ||
		atom.Value != "?clm" {
		t.Errorf("Unexpected result for ?clm: %v", atom)
	}

	atom = parseObjectAtom("ex:SomeClass", "")
	if atom.Type != "identifier" ||
		atom.Id != "ex:SomeClass" {
		t.Errorf("Unexpected result for ex:SomeClass: %v", atom)
	}

	atom = parseObjectAtom("localVar", "")
	if atom.Type != "identifier" ||
		atom.Id != "localVar" {
		t.Errorf("Unexpected result for localVar: %v", atom)
	}

	atom = parseObjectAtom("\"XYZ\"", "")
	if atom.Type != "text" ||
		atom.Value != "XYZ" {
		t.Errorf("Unexpected result for XYZ: %v", atom)
	}

	atom = parseObjectAtom("text(\"XYZ\")", "")
	if atom.Type != "text" ||
		atom.Value != "XYZ" {
		t.Errorf("Unexpected result for text(\"XYZ\"): %v", atom)
	}

	atom = parseObjectAtom("1", "")
	if atom.Type != "int" ||
		atom.Value != "1" {
		t.Errorf("Unexpected result for 1: %v", atom)
	}

	atom = parseObjectAtom("-10", "")
	if atom.Type != "int" ||
		atom.Value != "-10" {
		t.Errorf("Unexpected result for -10: %v", atom)
	}

	atom = parseObjectAtom("+1.0", "")
	if atom.Type != "double" ||
		atom.Value != "+1.0" {
		t.Errorf("Unexpected result for +1.0: %v", atom)
	}

	atom = parseObjectAtom("int(1)", "")
	if atom.Type != "int" ||
		atom.Value != "1" {
		t.Errorf("Unexpected result for int(1): %v", atom)
	}

	atom = parseObjectAtom("bool(1)", "")
	if atom.Type != "bool" ||
		atom.Value != "1" {
		t.Errorf("Unexpected result for bool(1): %v", atom)
	}

	atom = parseObjectAtom("", "true")
	if atom.Type != "keyword" ||
		atom.Value != "true" {
		t.Errorf("Unexpected result for true: %v", atom)
	}

}
