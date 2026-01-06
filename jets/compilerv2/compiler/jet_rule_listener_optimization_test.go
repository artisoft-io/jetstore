package compiler

import (
	// "encoding/json"
	// "fmt"
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
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01, s=50, flag=\"healthcare\"]:(?x01 rdf:type abc:RuleConfig) -\u003e (?x01 OutputUnit int(1));":
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
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]:(?x01 rdf:type abc:RuleConfig).(?x01 OutputUnit int(0)) -\u003e (?x01 OutputUnit int(1));":
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
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]:(?x01 rdf:type abc:RuleConfig).(?x01 OutputUnit int(0)).(jets:client rdf:type ?x02).(jets:client ?x02 int(0)) -> (?x01 OutputUnit int(1));":
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
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]:(?x01 rdf:type abc:RuleConfig).(?x01 OutputUnit int(0)).(jets:client rdf:type ?x02).[(?x02 != abc:RuleConfig)].(jets:client ?x02 int(0)) -> (?x01 OutputUnit int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_Filter2(t *testing.T) {
	jrCompiler := NewCompiler("./testdata", "filter2.jr", false, false, false)
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
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().ReteNodes, "", " ")
	// fmt.Printf("** Rete Nodes: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[R01]:(?x01 rdf:type Class1).(?x01 p1 ?x02).[(?x01 != ?x02)].(?x03 rdf:type Class1).[(?x03 != ?x02)].(?x03 p1 ?x02) -> (?x01 p1 int(1));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_TermOrder1(t *testing.T) {
	jrCompiler := NewCompiler("./testdata", "term_order1.jr", false, false, true)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "term_order1.jr";
[EID_Member_Id_10]:
  (?config rdf:type nh_c:RuleConfig).
  (?config nh_c:eligibilityIdDelimitor ?d).
  (?e rdf:type nh:Eligibility).
  [?e exist_not nh:Member_Id]
-> 
  (?e eid1 ?d).
  (?e jets:ruleTag "Missing Member_Id")
;`)

	if err != nil {
		t.Fatal(err.Error())
	}
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().ReteNodes, "", " ")
	// fmt.Printf("** Rete Nodes: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[EID_Member_Id_10]:(?x01 rdf:type nh_c:RuleConfig).(?x01 nh_c:eligibilityIdDelimitor ?x02).(?x03 rdf:type nh:Eligibility).[(?x03 exist_not nh:Member_Id)] -> (?x03 eid1 ?x02).(?x03 jets:ruleTag text(Missing Member_Id));":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}

func TestJetRuleOptimizer_TermOrder2(t *testing.T) {
	jrCompiler := NewCompiler("./testdata", "term_order2.jr", false, false, true)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "term_order2.jr";
[MA_AuthInfoCopy01]: 
  (?ia rdf:type InputAuthorization).
  (?ia hasAuthorization ?auth).
  (iProperties authProperty2Copy ?property).
  (?ia ?property ?v)
-> 
  (?auth ?property ?v);`)

	if err != nil {
		t.Fatal(err.Error())
	}
	// var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().ReteNodes, "", " ")
	// fmt.Printf("** Rete Nodes: \n%v\n", string(b))
	switch {
		case jrCompiler.ErrorLog().Len() > 0:
			t.Error(jrCompiler.ErrorLog().String())
		case len(jrCompiler.JetRuleModel().Jetrules) != 1:
			t.Error("Expected 1 jetrule")
		case jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel != "[MA_AuthInfoCopy01]:(?x01 rdf:type InputAuthorization).(?x01 hasAuthorization ?x02).(iProperties authProperty2Copy ?x03).(?x01 ?x03 ?x04) -> (?x02 ?x03 ?x04);":
			t.Error("Unexpected normalized label:", jrCompiler.JetRuleModel().Jetrules[0].NormalizedLabel)
	}
	// t.Error("Done")
}
