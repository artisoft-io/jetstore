package compiler

import (
	"testing"
)

func TestCompiler1(t *testing.T) {
	jrCompiler := NewCompiler("/home/michel/projects/repos/usi_ws", "jet_rules/main/MSK/1_MSK_Mapping_SM_Main2.jr", false, false, true)
	err := jrCompiler.Compile()
	if err != nil {
		t.Error(err.Error())
	}
	// t.Error("Done")
}
