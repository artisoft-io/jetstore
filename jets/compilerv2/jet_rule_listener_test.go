package main

import (
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

	// fmt.Printf("** Got Classes: \n%v\n", listener.jetRuleModel.Classes)
	t.Error("Done")
}
