package compiler

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestJetRuleOptimizer_JetRule0(t *testing.T) {
	jrCompiler := NewCompiler("", "jetrule0.jr", false, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "jetrule0.jr";
		resource abc:RuleConfig = "abc:RuleConfig";
		resource OutputUnit = "abc:OutputUnit";

		[R01, s=50, flag="healthcare"]:
		(?config rdf:type abc:RuleConfig)
		->
		(?config OutputUnit 1);`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01, s=50, flag=\"healthcare\"]: (?x1 rdf:type abc:RuleConfig) -\u003e (?x1 OutputUnit int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_JetRule1(t *testing.T) {
	jrCompiler := NewCompiler("", "jetrule1.jr", false, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "jetrule1.jr";
		resource abc:RuleConfig = "abc:RuleConfig";
		resource OutputUnit = "abc:OutputUnit";

		[R01]:
		(?config OutputUnit 0).
		(?config rdf:type abc:RuleConfig)
		->
		(?config OutputUnit 1);`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]: (?x1 rdf:type abc:RuleConfig).(?x1 OutputUnit int(0)) -\u003e (?x1 OutputUnit int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_JetRule2(t *testing.T) {
	jrCompiler := NewCompiler("", "jetrule2.jr", false, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "jetrule2.jr";
		resource abc:RuleConfig = "abc:RuleConfig";
		resource OutputUnit = "abc:OutputUnit";

		[R01]:
		(jets:client rdf:type ?p).
		(jets:client ?p 0).
		(?config OutputUnit 0).
		(?config rdf:type abc:RuleConfig)
		->
		(?config OutputUnit 1);`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]: (?x1 rdf:type abc:RuleConfig).(jets:client rdf:type ?x2).(?x1 OutputUnit int(0)).(jets:client ?x2 int(0)) -\u003e (?x1 OutputUnit int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_Filter1(t *testing.T) {
	jrCompiler := NewCompiler("", "filter1.jr", false, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "filter1.jr";
		resource abc:RuleConfig = "abc:RuleConfig";
		resource OutputUnit = "abc:OutputUnit";

		[R01]:
		(jets:client rdf:type ?p).
		(jets:client ?p 0).
		(?config OutputUnit 0).
		(?config rdf:type abc:RuleConfig).
		[?p != abc:RuleConfig]
		->
		(?config OutputUnit 1);`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]: (?x1 rdf:type abc:RuleConfig).(jets:client rdf:type ?x2).[(?x2 != abc:RuleConfig)].(?x1 OutputUnit int(0)).(jets:client ?x2 int(0)) -\u003e (?x1 OutputUnit int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_Filter2(t *testing.T) {
	jrCompiler := NewCompiler("", "filter2.jr", false, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "filter2.jr";
		resource p1 = "p1";
		resource Class1 = "Class1";

		[R01]:
		(?a rdf:type Class1).
		(?b rdf:type Class1).
		(?a p1 ?c).
		[?a != ?c]
		(?b p1 ?c).
		[?b != ?c]
		->
		(?a p1 1);`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]: (?x1 rdf:type Class1).(?x2 rdf:type Class1).(?x1 p1 ?x3).[((?x2 != ?x3) and (?x1 != ?x3))].(?x2 p1 ?x3) -\u003e (?x1 p1 int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	t.Error("Done")
}
