package compiler

// Test BuildReteNetwork and PostProcessJetruleModel

import (
	"encoding/json"
	"fmt"
	"strings"
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
	// R01 (s=50) and R02 (s=20) have the same antecedents. They share the
	// first two vertices and end on one each, so each fires at its own salience:
	// the head node, three antecedents and the second terminal, plus two
	// consequents. Until 2026-10-11 they shared the terminal too, 6 nodes, and
	// the vertex fired both at one salience (rules_unit_test_harness I-25).
	case len(jrCompiler.JetRuleModel().ReteNodes) != 7:
		t.Error("Expected 7 rete nodes, got", len(jrCompiler.JetRuleModel().ReteNodes))
	}
	assertEachRuleFiresAtItsSalience(t, jrCompiler)
	// t.Error("Done")
}

// assertEachRuleFiresAtItsSalience checks that every vertex a rule ends on
// carries that rule's salience and no other.
func assertEachRuleFiresAtItsSalience(t *testing.T, c *Compiler) {
	t.Helper()
	declared := make(map[string]int)
	for _, r := range c.JetRuleModel().Jetrules {
		declared[r.Name] = r.Salience
	}
	for _, n := range c.JetRuleModel().ReteNodes {
		for i, r := range n.Rules {
			if n.Salience[i] != declared[r] {
				t.Errorf("vertex %d: rule %s listed at s=%d, declared s=%d", n.Vertex, r, n.Salience[i], declared[r])
			}
			if n.Salience[i] != n.Salience[0] {
				t.Errorf("vertex %d fires rules at different saliences %v (%v)", n.Vertex, n.Salience, n.Rules)
			}
		}
	}
}

// Rules with the same antecedents and the same salience still share their last
// vertex; a third at another salience gets one of its own, sharing the prefix.
func TestJetRuleListener_TerminalVertexBySalience(t *testing.T) {
	c := NewCompiler("./testdata", "rete_test_salience.jr", false, false, true)
	err := c.CompileBuffer(`@JetCompilerDirective source_file = "rete_test_salience.jr";
	resource ex:Item = "ex:Item";
	resource ex:link = "ex:link";
	resource ex:a = "ex:a";
	resource ex:b = "ex:b";
	resource ex:c = "ex:c";
	[A, s=100]: (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:a ?y);
	[B, s=5]:   (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:b ?y);
	[C, s=100]: (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:c ?y);`)
	if err != nil {
		t.Fatal(err)
	}
	vertexOf := make(map[string]int)
	for _, n := range c.JetRuleModel().ReteNodes {
		for _, r := range n.Rules {
			vertexOf[r] = n.Vertex
		}
	}
	if vertexOf["A"] != vertexOf["C"] {
		t.Errorf("A and C (both s=100) end on vertices %d and %d, want one", vertexOf["A"], vertexOf["C"])
	}
	if vertexOf["A"] == vertexOf["B"] {
		t.Errorf("A (s=100) and B (s=5) share vertex %d", vertexOf["A"])
	}
	nodes := c.JetRuleModel().ReteNodes
	if nodes[vertexOf["A"]].ParentVertex != nodes[vertexOf["B"]].ParentVertex {
		t.Errorf("A and B do not share the prefix (?x rdf:type ex:Item)")
	}
	assertEachRuleFiresAtItsSalience(t, c)
}

func TestJetRuleListener_BuildReteNetwork2(t *testing.T) {
	jrCompiler := NewCompiler("./testdata", "rete_test2.jr", true, true, false)
	err := jrCompiler.CompileBuffer(`@JetCompilerDirective source_file = "rete_test2.jr";
		@JetCompilerDirective extract_resources_from_rules = "true";
		[Savings01]: 
			(?stateNdc rdf:type wrs:StateNdc).
			(?stateNdc wrs:hasDrugInterchanges ?drugInterchange).
			(?stateNdc wrs:Sum_Total_Paid_Amount ?paid).
			(?stateNdc wrs:Sum_Quantity_Dispensed ?qty).
			(?drugInterchange wrs:Target_Drug_Cost ?targetCost)
		-> 
			(?drugInterchange wrs:Rx_Savings (?paid - (?qty * ?targetCost))).
			(?drugInterchange jets:ruleTag "Interchange");	
			
		[Savings02]: 
			(?stateNdc rdf:type wrs:StateNdc).
			(?stateNdc wrs:hasDrugInterchanges ?drugInterchange).
			(?stateNdc wrs:Sum_Total_Paid_Amount ?paid).
			(?stateNdc wrs:Sum_Quantity_Dispensed ?qty).
			(?drugInterchange wrs:Target_Drug_Cost ?targetCost1).
			(?drugInterchange wrs:Target_Drug_Cost ?targetCost2)
		-> 
			(?drugInterchange wrs:Rx_Savings (?paid - (?qty * ?targetCost1)/?targetCost2)).
			(?drugInterchange jets:ruleTag "Interchange");
	`)
	if err != nil {
		t.Fatal(err.Error())
	}
	var b []byte
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Resources, "", " ")
	// fmt.Printf("** Resources: \n%v\n", string(b))
	// b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().Jetrules, "", " ")
	// fmt.Printf("** Jet Rules: \n%v\n", string(b))
	b, _ = json.MarshalIndent(jrCompiler.JetRuleModel().ReteNodes, "", " ")
	fmt.Printf("** Rete Nodes: \n%v\n", string(b))
	// Eleven nodes, not the twelve this test used to expect: a head node, six
	// antecedent nodes and four consequents. The two rules share the first four
	// antecedents *and* the fifth — Savings01's (?drugInterchange
	// wrs:Target_Drug_Cost ?targetCost) and Savings02's ... ?targetCost1
	// normalise to the same node, (?x02 wrs:Target_Drug_Cost ?x05), because a
	// variable's name is local to its rule. Savings02 then extends that node
	// with its second Target_Drug_Cost binding. Sharing the common prefix is
	// what a RETE network is for, so counting twelve was counting a network
	// that had failed to share.
	switch {
	case jrCompiler.ErrorLog().Len() > 0:
		t.Error(jrCompiler.ErrorLog().String())
	case len(jrCompiler.JetRuleModel().Jetrules) != 2:
		t.Error("Expected 2 jetrules")
	case len(jrCompiler.JetRuleModel().ReteNodes) != 11:
		t.Error("Expected 11 rete nodes, got", len(jrCompiler.JetRuleModel().ReteNodes))
	}
	// The shape behind the count: the antecedents form one chain of six, and
	// each rule's two consequents hang off the vertex where that rule ends -
	// Savings01 at 5, Savings02 at 6.
	antecedents, consequents := 0, map[int]int{}
	for _, node := range jrCompiler.JetRuleModel().ReteNodes {
		switch node.Type {
		case "antecedent":
			antecedents++
		case "consequent":
			consequents[node.Vertex]++
		}
	}
	if antecedents != 6 {
		t.Errorf("expected 6 antecedent nodes (4 shared, 1 shared fifth, 1 for Savings02), got %d", antecedents)
	}
	if consequents[5] != 2 || consequents[6] != 2 {
		t.Errorf("expected both rules to end at vertices 5 and 6 with two consequents each, got %v", consequents)
	}
	// t.Error("Done")
}

// compileSalienceCase compiles rules over a small header and returns the
// compile error, if any, and the warnings logged.
func compileSalienceCase(t *testing.T, rules string) (error, []string) {
	t.Helper()
	c := NewCompiler("./testdata", "rete_test_salience_warn.jr", false, false, true)
	err := c.CompileBuffer(`@JetCompilerDirective source_file = "rete_test_salience_warn.jr";
	resource ex:Item = "ex:Item";
	resource ex:link = "ex:link";
	resource ex:a = "ex:a";
	resource ex:b = "ex:b";
` + rules)
	var warnings []string
	for _, d := range c.Diagnostics() {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "same antecedents") {
			warnings = append(warnings, d.Message)
		}
	}
	return err, warnings
}

// A rule copied to change its salience, with the original kept: the triple
// exists from the higher salience, whatever the copy declares. A warning, and
// the compile succeeds.
func TestJetRuleListener_SameConsequentAtTwoSaliencesWarns(t *testing.T) {
	err, warnings := compileSalienceCase(t, `
	[Early, s=100]: (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:a ?y);
	[Late, s=5]:    (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:a ?y).(?x ex:b ?y);`)
	if err != nil {
		t.Fatalf("a warning must not fail the compile: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %d: %v", len(warnings), warnings)
	}
	for _, want := range []string{"Early (", "s=100)", "Late (", "s=5)", "ex:a"} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning does not mention %q: %s", want, warnings[0])
		}
	}
	if strings.Contains(warnings[0], "ex:b") {
		t.Errorf("ex:b is inferred by one rule only and must not be named: %s", warnings[0])
	}
}

// Different consequents, or the same salience (one shared vertex), is no warning.
func TestJetRuleListener_SameConsequentAtTwoSaliencesQuiet(t *testing.T) {
	for name, rules := range map[string]string{
		"different consequents": `
	[Early, s=100]: (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:a ?y);
	[Late, s=5]:    (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:b ?y);`,
		"same salience": `
	[One, s=10]: (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:a ?y);
	[Two, s=10]: (?x rdf:type ex:Item).(?x ex:link ?y) -> (?x ex:a ?y);`,
	} {
		err, warnings := compileSalienceCase(t, rules)
		if err != nil || len(warnings) != 0 {
			t.Errorf("%s: expected a clean compile, got %v, %v", name, err, warnings)
		}
	}
}
