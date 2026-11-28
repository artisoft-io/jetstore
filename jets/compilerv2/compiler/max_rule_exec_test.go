package compiler

import (
	"errors"
	"strings"
	"testing"
)

// $max_rule_exec in a main rule file's config block is a compile error at its line,
// naming the triple that sets the limit it was meant to: nothing read it, so an author
// who set it had no limit and no message. The other config items compile, and the
// refused one is not stored in jetstore_config.
func TestMaxRuleExecIsRefusedNamingTheTriple(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.jr", strings.Join([]string{
		"# main line 1",
		"main {",
		"  $max_looping = 5,",
		"  $max_rule_exec = 5000,",
		"  $input_types = [acme:A]",
		"};",
		"resource acme:A = \"acme:A\";",
	}, "\n"))

	c := NewCompiler(dir, "main.jr", false, false, true)
	err := c.Compile()
	var cerr *CompilationError
	if !errors.As(err, &cerr) {
		t.Fatalf("expected a *CompilationError, got %v", err)
	}
	log := c.ErrorLog().String()
	if want := "error in main.jr at line 4: " + MaxRuleExecRefused; !strings.Contains(log, want) {
		t.Errorf("error log lacks %q:\n%s", want, log)
	}
	if !strings.Contains(MaxRuleExecRefused, "triple(jets:iState, jets:max_vertex_visits,") {
		t.Errorf("the message does not name the triple: %q", MaxRuleExecRefused)
	}
	located := false
	for _, d := range cerr.Errors() {
		if d.File == "main.jr" && d.Line == 4 {
			located = true
		}
	}
	if !located {
		t.Errorf("want a diagnostic located in main.jr at line 4, got %v", cerr.Errors())
	}
	if _, ok := c.JetRuleModel().JetstoreConfig["$max_rule_exec"]; ok {
		t.Errorf("$max_rule_exec was stored in jetstore_config: %v", c.JetRuleModel().JetstoreConfig)
	}
	if got := c.JetRuleModel().JetstoreConfig["$max_looping"]; got != "5" {
		t.Errorf("$max_looping = %q, want 5", got)
	}
}

// Without it the same file compiles.
func TestConfigWithoutMaxRuleExecCompiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.jr", strings.Join([]string{
		"main {",
		"  $max_looping = 5,",
		"  $input_types = [acme:A]",
		"};",
		"resource acme:A = \"acme:A\";",
		"triple(jets:iState, jets:max_vertex_visits, 1000000);",
	}, "\n"))
	c := NewCompiler(dir, "main.jr", false, false, true)
	if err := c.Compile(); err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, c.ErrorLog().String())
	}
}
