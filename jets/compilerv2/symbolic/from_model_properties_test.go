package symbolic

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// FromModel reads s, o and i as the compiler does: a quoted value as the
// string it stands for, so a rule written s="10" is the salience 10 rather than
// an error, and a rule the compiler accepts is one FromModel can read.
func TestFromModelReadsQuotedPropertiesAsTheCompilerDoes(t *testing.T) {
	c := compiler.NewCompiler(t.TempDir(), "p.jr", false, false, true)
	src := `[R, o="false", s="10", flag="x"]: (?x rdf:type hc:C) -> (?x hc:p 1);` + "\n"
	if err := c.CompileBuffer(src); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
	f, err := FromModel(c.JetRuleModel(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Rules) != 1 {
		t.Fatalf("%d rules, want 1", len(f.Rules))
	}
	r := f.Rules[0]
	if r.Salience == nil || *r.Salience != 10 {
		t.Errorf("salience %v, want 10", r.Salience)
	}
	if r.Optimization == nil || *r.Optimization {
		t.Errorf("optimization %v, want false", r.Optimization)
	}
	if v := r.Properties["flag"].V; v != "x" {
		t.Errorf("flag %v, want the string x", v)
	}

	if !IsGenerated(&rete.JetruleNode{Properties: map[string]string{"i": `"true"`}}) {
		t.Error(`i="true" is not read as a generated rule`)
	}
}
