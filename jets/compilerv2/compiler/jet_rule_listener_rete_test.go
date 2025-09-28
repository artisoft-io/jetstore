package compiler

// Test BuildReteNetwork and PostProcessJetruleModel

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestJetRuleListener_BuildReteNetwork1(t *testing.T) {
	jrCompiler := NewCompiler("./testdata", "rete_test1.jr", true, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "rete_test1.jr";
	resource abc:RuleConfig = "abc:RuleConfig";
	resource OutputUnit = "abc:OutputUnit";
	resource RelatedTo = "abc:RelatedTo";

	[R01, s=50, flag="healthcare"]:
	(?config rdf:type abc:RuleConfig).
	(?config OutputUnit 0).
	(?config RelatedTo ?x1)
	->
	(?config OutputUnit 1);

	[R02, s=20, flag="finance"]:
	(?config rdf:type abc:RuleConfig).
	(?config OutputUnit 0).
	(?config RelatedTo ?x2)
	->
	(?config OutputUnit 2);`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	fmt.Printf("** Jet Rules: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().ReteNodes, "", " ")
	fmt.Printf("** Rete Nodes: \n%v\n", string(b))
	switch {
	case jrCompiler.ErrorLog().Len() > 0:
		t.Error(jrCompiler.ErrorLog().String())
	case len(jrCompiler.JetRuleModel().Jetrules) != 2:
		t.Error("Expected 2 jetrules")
	case len(jrCompiler.JetRuleModel().ReteNodes) != 8:
		t.Error("Expected 8 rete nodes, got", len(jrCompiler.JetRuleModel().ReteNodes))
	}
	t.Error("Done")
}