package symbolic

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// The corpus round trip is the form's drift check, and the schema's: every
// document it produces is validated against rule_file.schema.json, so the
// published schema cannot refuse a rule the corpus holds.
//
// The corpus round trip is the form's drift check, and the reason it is
// written against the real workspaces rather than a fixture: a fixture checks
// the constructs its author thought of, and the corpus checks the ones that
// exist.
//
// For every main rule file a workspace names in workspace_control.json:
//
//  1. compile it as the workspace would, and keep its rules;
//  2. take those rules to symbolic form (FromModel), through JSON and back;
//  3. render them (Render), and put the rendering in place of the rules in
//     the combined buffer the compiler parses;
//  4. compile that buffer, and require the two compiles to agree rule for
//     rule — name, salience, optimisation, properties, validity, and every
//     antecedent's and consequent's normalised label.
//
// Normalised labels name variables by position and resources by id, so they
// compare the rules and not the keys, which differ between the two compiles
// by construction. The rule-level label is not compared because its property
// order is a map iteration (makeRuleLabel); the properties are compared as a
// map instead.
//
// The workspaces are not this repository's, so the test reads them where the
// parent checkout has them and skips when it does not. Run it with -count=1:
// they live outside the Go module and the test cache does not see them move.

func corpusRoot() string {
	if dir := os.Getenv("JETS_SYMBOLIC_CORPUS_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("..", "..", "..", "..", "workspaces")
}

func TestCorpusRoundTrip(t *testing.T) {
	for _, ws := range []string{"cedargate_ws", "jets_ws", "usi_ws", "walrus_ws"} {
		t.Run(ws, func(t *testing.T) {
			src := filepath.Join(corpusRoot(), ws)
			control, err := os.ReadFile(filepath.Join(src, "workspace_control.json"))
			if err != nil {
				t.Skipf("workspace %s is not available here (%v)", ws, err)
			}
			var wc struct {
				RuleSets []string `json:"rule_sets"`
			}
			if err := json.Unmarshal(control, &wc); err != nil {
				t.Fatal(err)
			}
			if len(wc.RuleSets) == 0 {
				t.Fatalf("%s names no rule sets", ws)
			}
			dir := stageWorkspace(t, src)
			total := 0
			for _, main := range wc.RuleSets {
				total += roundTrip(t, dir, main)
			}
			t.Logf("%s: %d rules over %d main files round-tripped", ws, total, len(wc.RuleSets))
		})
	}
}

// stageWorkspace copies a workspace's rule sources and installs the
// JetStore-owned assets they import, which a fresh checkout does not carry.
func stageWorkspace(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"data_model", "jet_rules", "lookups"} {
		from := filepath.Join(src, sub)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := os.CopyFS(filepath.Join(dir, sub), os.DirFS(from)); err != nil {
			t.Fatalf("copying %s: %v", sub, err)
		}
	}
	if _, err := workspace_assets.Install(dir, workspace_assets.Options{Force: true}); err != nil {
		t.Fatalf("installing workspace assets: %v", err)
	}
	return dir
}

func readUnder(base, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(base, name))
	return string(data), err
}

func roundTrip(t *testing.T, dir, main string) int {
	t.Helper()
	reader := compiler.NewRuleFileReader(dir, main, readUnder)
	combined, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("%s: reading: %v", main, err)
	}

	first := compiler.NewCompiler(dir, main, false, false, true)
	if err := first.CompileBuffer(combined); err != nil {
		t.Fatalf("%s: the original does not compile: %v", main, err)
	}
	original := first.JetRuleModel()
	if len(authored(original)) == 0 {
		return 0
	}

	sym, err := FromModel(original, nil)
	if err != nil {
		t.Fatalf("%s: FromModel: %v", main, err)
	}
	// The form refuses two rules of one name, which the compiler accepts and
	// usi_ws has (its assessment counts 29 of 743). Rename the later ones so
	// the rendering compiles, and compare by position rather than by name.
	seen := map[string]int{}
	for i := range sym.Rules {
		name := sym.Rules[i].Name
		if n := seen[name]; n > 0 {
			sym.Rules[i].Name = fmt.Sprintf("%s__dup%d", name, n)
		}
		seen[name]++
	}
	// Through JSON, so what is compiled is what an author would have sent.
	data, err := json.Marshal(sym)
	if err != nil {
		t.Fatalf("%s: marshal: %v", main, err)
	}
	if err := validateAgainstSchema(t, data); err != nil {
		t.Fatalf("%s: the schema refuses a document the corpus produced: %v", main, err)
	}
	sym, err = Parse(data)
	if err != nil {
		t.Fatalf("%s: parse: %v", main, err)
	}
	rendered, err := Render(sym)
	if err != nil {
		t.Fatalf("%s: render: %v", main, err)
	}

	stripped := stripRules(t, combined)
	second := compiler.NewCompiler(dir, main, false, false, true)
	if err := second.CompileBuffer(stripped + "\n" + rendered.Source); err != nil {
		t.Fatalf("%s: the rendering does not compile: %v\n%s", main, err, firstLines(second.ErrorLog().String(), 20))
	}
	return compareRules(t, main, original, second.JetRuleModel())
}

// stripRules blanks every rule statement out of a combined buffer, keeping its
// line breaks, so the buffer keeps its declarations and loses its rules.
func stripRules(t *testing.T, combined string) string {
	t.Helper()
	lexer := parser.NewJetRuleLexer(antlr.NewInputStream(combined))
	p := parser.NewJetRuleParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	p.RemoveErrorListeners()
	collector := &ruleSpans{}
	antlr.ParseTreeWalkerDefault.Walk(collector, p.Jetrule())
	runes := []rune(combined)
	for _, span := range collector.spans {
		for i := span[0]; i <= span[1] && i < len(runes); i++ {
			if runes[i] != '\n' {
				runes[i] = ' '
			}
		}
	}
	return string(runes)
}

type ruleSpans struct {
	*parser.BaseJetRuleListener
	spans [][2]int
}

func (r *ruleSpans) ExitJetRuleStmt(ctx *parser.JetRuleStmtContext) {
	r.spans = append(r.spans, [2]int{ctx.GetStart().GetStart(), ctx.GetStop().GetStop()})
}

// compareRules compares the authored rules of two compiles in order. The
// rendering keeps the source's rule order, so position identifies a rule even
// where a name does not.
func compareRules(t *testing.T, main string, want, got *rete.JetruleModel) int {
	t.Helper()
	w, g := authored(want), authored(got)
	if len(w) != len(g) {
		t.Errorf("%s: %d rules compiled from the rendering, %d from the source", main, len(g), len(w))
		return 0
	}
	for i := range w {
		if !strings.HasPrefix(g[i].Name, w[i].Name) {
			t.Errorf("%s: rule %d is %s in the source and %s in the rendering", main, i, w[i].Name, g[i].Name)
			continue
		}
		if a, b := ruleSignature(w[i]), ruleSignature(g[i]); a != b {
			t.Errorf("%s: rule %s differs\n source:    %s\n rendering: %s", main, w[i].Name, a, b)
		}
	}
	return len(w)
}

func authored(m *rete.JetruleModel) []*rete.JetruleNode {
	var out []*rete.JetruleNode
	for _, r := range m.Jetrules {
		if !IsGenerated(r) {
			out = append(out, r)
		}
	}
	return out
}

func ruleSignature(r *rete.JetruleNode) string {
	b := &strings.Builder{}
	props := maps.Clone(r.Properties)
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(b, "s=%d o=%t valid=%t", r.Salience, r.Optimization, r.IsValid)
	for _, k := range keys {
		fmt.Fprintf(b, " %s=%s", k, props[k])
	}
	for _, a := range r.Antecedents {
		b.WriteString(" | " + a.NormalizedLabel)
	}
	b.WriteString(" ->")
	for _, c := range r.Consequents {
		b.WriteString(" | " + c.NormalizedLabel)
	}
	return b.String()
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
