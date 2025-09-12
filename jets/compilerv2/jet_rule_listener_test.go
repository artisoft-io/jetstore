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
