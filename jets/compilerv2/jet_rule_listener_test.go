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

	fmt.Printf("** Generated model has %d rules, %d resources, %d lookup tables\n",
		len(listener.jetRuleModel.Jetrules),
		len(listener.jetRuleModel.Resources),
		len(listener.jetRuleModel.LookupTables))

	t.Error("Done")
}
