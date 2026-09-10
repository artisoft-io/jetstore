package rete_test

// OPERATOR NAMES: THE CLASS OF DEFECT NOTHING WAS CHECKING.
//
// An operator name reaches the engine as an opaque string. The ANTLR lexer has no
// token for `sum_values` or any of its siblings -- `binaryOp` and `unaryOp` both
// reduce to `Identifier` (`binaryOp`, jets/compilerv2/compiler/JetRule.g4:241) -- the
// compiler carries the name through as `ExpressionNode.Op`
// (`ExpressionNode`, jets/jetrules/rete/rete_meta_store_model.go:172) and writes it to
// `workspace.db` unexamined, and no validator whitelists it. So a typo in a `.jr` file
// COMPILES CLEANLY and throws when the rule fires: `create_binary_expr`
// (jets/rete/expr_operator_factory.h) raises, `CreateBinaryOperator`
// (`CreateBinaryOperator`, jets/jetrules/rete/expr_operator_factory.go:5) returns nil.
//
// The package README says the same thing from the author's side -- *adding an operator
// is exactly two registrations* -- and the two tests here are the two halves of that
// sentence turned into assertions:
//
//   - TestCorpusOperatorNamesResolve compiles every rule set of every workspace and
//     asserts each operator name it finds resolves in the Go factory. That is the
//     typo case, caught at test time instead of at rule-execution time.
//
//   - TestBothFactoriesRegisterTheSameOperators reads the two factories and asserts
//     they register the same names. That is the *one of two registrations* case: an
//     operator added to one engine and not the other compiles and runs on one
//     deployment and throws on the other, and the C++ engine is the one that ships
//     (`Dockerfile.cpipes:46`, `jets/cmds/cpipes_native_server/main.go`).
//
// THE C++ HALF OF THE FIRST TEST IS A GTEST, NOT A GO TEST, and the two share a file.
// C++ cannot compile a `.jr`, so this test writes the corpus operator names to
// `jets/rete/test_data/corpus_operators.txt` and `CorpusOperatorNames` in
// `jets/rete/expr_operator_factory_test.cc` asserts each one resolves in
// `create_binary_expr` / `create_unary_expr`. The manifest is checked in and this test
// FAILS when it drifts, rather than rewriting it silently -- run with
// JETS_UPDATE_OPERATOR_MANIFEST=1 to regenerate it after adding an operator to a rule.
//
// WHY package rete_test. Collecting the names means running the compiler, and
// `jets/compilerv2/compiler` imports this package. An external test package can import
// a package that imports the package under test; an internal one cannot.
//
// -count=1: this test reads rule files that live OUTSIDE this Go module (the four
// workspaces are submodules of the parent repository), so nothing in the test cache key
// changes when a rule changes. A cached `ok` here is indistinguishable from a pass.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// The manifest is under jets/rete/ rather than beside this file because the C++ test
// is the one that cannot generate it: CMake copies jets/rete/test_data next to the
// jets_test binary after every build (jets/CMakeLists.txt, the POST_BUILD custom
// command), which is how a gtest reaches it with a relative path.
const operatorManifest = "../../rete/test_data/corpus_operators.txt"

// corpusWorkspaces are the four rule workspaces the counts in this repository's
// documents are drawn from. They are submodules of the parent repository, so a plain
// checkout of jetstore alone has none of them and this test skips.
var corpusWorkspaces = []string{"jets_ws", "usi_ws", "walrus_ws", "cedargate_ws"}

// opUse records where an operator name was seen, so a failure names a rule file rather
// than only the offending word.
type opUse struct {
	binary map[string][]string
	unary  map[string][]string
}

func newOpUse() *opUse {
	return &opUse{binary: make(map[string][]string), unary: make(map[string][]string)}
}

func (u *opUse) walk(nd *rete.ExpressionNode, where string) {
	if nd == nil {
		return
	}
	switch nd.Type {
	case "binary":
		u.binary[nd.Op] = append(u.binary[nd.Op], where)
	case "unary":
		u.unary[nd.Op] = append(u.unary[nd.Op], where)
	}
	u.walk(nd.Arg, where)
	u.walk(nd.Lhs, where)
	u.walk(nd.Rhs, where)
}

// collect compiles one workspace's rule sets and records every operator name.
//
// The rule sets come from workspace_control.json rather than from a `*_main.jr` glob,
// and the difference is not cosmetic: usi_ws names 34 rule sets of which 32 end in
// `_Main1.jr`, `_Main2.jr` or `_Main3.jr`. A glob for `_main.jr` finds two of them.
func (u *opUse) collect(t *testing.T, label, base string) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(base, "workspace_control.json"))
	if err != nil {
		t.Fatalf("%s: reading workspace_control.json: %v", label, err)
	}
	var ctl struct {
		RuleSets []string `json:"rule_sets"`
	}
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatalf("%s: parsing workspace_control.json: %v", label, err)
	}
	if len(ctl.RuleSets) == 0 {
		t.Fatalf("%s: workspace_control.json names no rule_sets", label)
	}
	for _, mainRuleFile := range ctl.RuleSets {
		c := compiler.NewCompiler(base, mainRuleFile, false, false, true)
		if err := c.Compile(); err != nil {
			// A rule set that does not compile is a failure of this test rather than
			// something to skip past: the operator names of a file that did not compile
			// are simply absent, and the assertion below would then be vacuous for it.
			t.Fatalf("%s: compiling %s: %v\n%s", label, mainRuleFile, err, c.ErrorLog().String())
		}
		where := label + "/" + mainRuleFile
		for _, rt := range c.JetRuleModel().ReteNodes {
			u.walk(rt.ObjectExpr, where)
			u.walk(rt.Filter, where)
		}
	}
	return len(ctl.RuleSets)
}

// corpusRoots returns the workspace directories to compile, and the reason each one
// that is missing is missing.
func corpusRoots(t *testing.T) (roots map[string]string, missing []string) {
	t.Helper()
	home := os.Getenv("WORKSPACES_HOME")
	if home == "" {
		// The layout this repository is normally checked out in: a submodule of
		// jetstore_agentic_ai, whose workspaces/ sits beside it.
		home = filepath.Join("..", "..", "..", "..", "workspaces")
	}
	roots = make(map[string]string)
	for _, ws := range corpusWorkspaces {
		dir := filepath.Join(home, ws)
		if _, err := os.Stat(filepath.Join(dir, "workspace_control.json")); err != nil {
			missing = append(missing, ws)
			continue
		}
		roots[ws] = dir
	}
	return roots, missing
}

func TestCorpusOperatorNamesResolve(t *testing.T) {
	uses := newOpUse()

	// jets/jetrules/test_ws is in this repository and is always available, so the
	// assertion below is never vacuous even when the workspaces are not checked out.
	nRuleSets := uses.collect(t, "test_ws", filepath.Join("..", "test_ws"))

	roots, missing := corpusRoots(t)
	for _, ws := range corpusWorkspaces {
		if dir, ok := roots[ws]; ok {
			nRuleSets += uses.collect(t, ws, dir)
		}
	}

	ctx := &rete.ReteBuilderContext{}
	for _, op := range sortedKeys(uses.binary) {
		if ctx.CreateBinaryOperator(op) == nil {
			t.Errorf("binary operator %q does not resolve in CreateBinaryOperator; used in %s",
				op, strings.Join(dedup(uses.binary[op]), ", "))
		}
	}
	for _, op := range sortedKeys(uses.unary) {
		if ctx.CreateUnaryOperator(op) == nil {
			t.Errorf("unary operator %q does not resolve in CreateUnaryOperator; used in %s",
				op, strings.Join(dedup(uses.unary[op]), ", "))
		}
	}
	t.Logf("%d rule sets compiled, %d binary and %d unary operator names",
		nRuleSets, len(uses.binary), len(uses.unary))

	// The manifest is the corpus as a whole, so it can only be checked when the whole
	// corpus is on disk. With a workspace missing, the names found are a subset and the
	// most that can be said is that the manifest contains them.
	checkManifest(t, uses, len(missing) == 0)
	if len(missing) > 0 {
		t.Logf("workspaces not checked out, operator names collected from the rest: %s",
			strings.Join(missing, ", "))
	}
}

// checkManifest compares the corpus against the checked-in list the C++ test reads.
func checkManifest(t *testing.T, uses *opUse, complete bool) {
	t.Helper()
	want := renderManifest(uses)
	if os.Getenv("JETS_UPDATE_OPERATOR_MANIFEST") != "" {
		if !complete {
			t.Fatalf("refusing to regenerate %s from a partial corpus; check out the "+
				"workspaces first", operatorManifest)
		}
		if err := os.WriteFile(operatorManifest, []byte(want), 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("regenerated %s", operatorManifest)
		return
	}
	raw, err := os.ReadFile(operatorManifest)
	if err != nil {
		t.Fatalf("reading %s: %v", operatorManifest, err)
	}
	inManifest := parseManifest(string(raw))
	for _, kind := range []string{"binary", "unary"} {
		set := uses.binary
		if kind == "unary" {
			set = uses.unary
		}
		for _, op := range sortedKeys(set) {
			if !slices.Contains(inManifest[kind], op) {
				t.Errorf("%s operator %q is used by the corpus (%s) and is not in %s; "+
					"rerun with JETS_UPDATE_OPERATOR_MANIFEST=1 so the C++ factory is "+
					"checked against it too",
					kind, op, dedup(set[op])[0], operatorManifest)
			}
		}
	}
	if !complete {
		return
	}
	if string(raw) != want {
		t.Errorf("%s is stale against the corpus; rerun with "+
			"JETS_UPDATE_OPERATOR_MANIFEST=1", operatorManifest)
	}
}

func renderManifest(uses *opUse) string {
	var b strings.Builder
	b.WriteString(`# Operator names used by the compiled rule corpus.
#
# GENERATED. Do not hand edit: TestCorpusOperatorNamesResolve
# (jets/jetrules/rete/operator_registration_test.go) compiles every rule set of every
# workspace and fails when this file drifts from what it finds. Regenerate with
#
#   JETS_UPDATE_OPERATOR_MANIFEST=1 go test -count=1 \
#     -run TestCorpusOperatorNamesResolve ./jets/jetrules/rete/
#
# It exists because the C++ engine cannot compile a .jr file and therefore cannot
# collect these names for itself. CorpusOperatorNames in expr_operator_factory_test.cc
# reads it and asserts each name resolves in create_binary_expr / create_unary_expr.
# CMake copies this directory next to the jets_test binary after every build.
`)
	for _, op := range sortedKeys(uses.binary) {
		fmt.Fprintf(&b, "binary %s\n", op)
	}
	for _, op := range sortedKeys(uses.unary) {
		fmt.Fprintf(&b, "unary %s\n", op)
	}
	return b.String()
}

func parseManifest(s string) map[string][]string {
	out := map[string][]string{"binary": nil, "unary": nil}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, op, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		out[kind] = append(out[kind], op)
	}
	return out
}

// THE SECOND HALF: the two factories must register the same names.
//
// This one reads source rather than running anything, because there is no other way to
// ask a Go switch or a chain of C++ ifs what it accepts. That makes it a textual test,
// and the trade is deliberate: the two factories are written in one rigid form each --
// `case "name":` and `if(op == "name")` -- and a formatting change breaks this test
// loudly rather than letting a divergence through quietly.
//
// KNOWN DIVERGENCES ARE LISTED, NOT TOLERATED SILENTLY. The six below were measured on
// 2026-09-09 and none of them appears anywhere in the corpus, which is the only reason
// they have not cost anyone a production failure. A seventh will fail this test.
var knownFactoryDivergences = map[string]string{
	// binary
	"apply_regex": "go: an alias for literal_regex; C++ has literal_regex alone",
	"min_head_of": "go only: NewMinMaxOp(true, true); C++ has no head form of min_of",
	"max_head_of": "go only: NewMinMaxOp(false, true); C++ has no head form of max_of",
	// unary
	"to_date":         "go only: NewToDateOp; C++ has to_timestamp and no to_date",
	"to_datetime":     "go only: NewToDatetimeOp; C++ has to_timestamp and no to_datetime",
	"raise_exception": "C++ only: RaiseExceptionVisitor; the Go engine has no counterpart",
}

var (
	goCaseRe   = regexp.MustCompile(`case\s+((?:"[^"]*"\s*,\s*)*"[^"]*")\s*:`)
	cppCaseRe  = regexp.MustCompile(`op\s*==\s*"([^"]*)"`)
	goStringRe = regexp.MustCompile(`"([^"]*)"`)
)

func TestBothFactoriesRegisterTheSameOperators(t *testing.T) {
	goBinary, goUnary := readGoFactory(t)
	cppBinary, cppUnary := readCppFactory(t)

	for _, pair := range []struct {
		kind     string
		go_, cpp []string
	}{
		{"binary", goBinary, cppBinary},
		{"unary", goUnary, cppUnary},
	} {
		for _, op := range pair.go_ {
			if slices.Contains(pair.cpp, op) {
				continue
			}
			if why, ok := knownFactoryDivergences[op]; ok {
				t.Logf("known divergence, %s %q -- %s", pair.kind, op, why)
				continue
			}
			t.Errorf("%s operator %q is registered in the Go factory and not in the C++ one; "+
				"the C++ engine is the one that ships, so a rule using it throws in production "+
				"(add the registration, or record it in knownFactoryDivergences with a reason)",
				pair.kind, op)
		}
		for _, op := range pair.cpp {
			if slices.Contains(pair.go_, op) {
				continue
			}
			if why, ok := knownFactoryDivergences[op]; ok {
				t.Logf("known divergence, %s %q -- %s", pair.kind, op, why)
				continue
			}
			t.Errorf("%s operator %q is registered in the C++ factory and not in the Go one "+
				"(add the registration, or record it in knownFactoryDivergences with a reason)",
				pair.kind, op)
		}
	}
	t.Logf("go: %d binary %d unary; c++: %d binary %d unary; %d known divergences",
		len(goBinary), len(goUnary), len(cppBinary), len(cppUnary), len(knownFactoryDivergences))
}

func readGoFactory(t *testing.T) (binary, unary []string) {
	t.Helper()
	src := readSource(t, "expr_operator_factory.go")
	bin, un := splitOn(t, src, "func (ctx *ReteBuilderContext) CreateBinaryOperator",
		"func (ctx *ReteBuilderContext) CreateUnaryOperator")
	collect := func(s string) []string {
		var out []string
		for _, m := range goCaseRe.FindAllStringSubmatch(s, -1) {
			for _, lit := range goStringRe.FindAllStringSubmatch(m[1], -1) {
				out = append(out, lit[1])
			}
		}
		sort.Strings(out)
		return out
	}
	return collect(bin), collect(un)
}

func readCppFactory(t *testing.T) (binary, unary []string) {
	t.Helper()
	src := stripCppComments(readSource(t, filepath.Join("..", "..", "rete", "expr_operator_factory.h")))
	bin, un := splitOn(t, src, "create_binary_expr(int key", "create_unary_expr(int key")
	collect := func(s string) []string {
		var out []string
		for _, m := range cppCaseRe.FindAllStringSubmatch(s, -1) {
			out = append(out, m[1])
		}
		sort.Strings(out)
		return out
	}
	return collect(bin), collect(un)
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}

// splitOn returns the text between first and second, and the text after second.
// It fails rather than returning an empty half, because an empty half would make the
// comparison above pass by finding nothing on one side.
func splitOn(t *testing.T, src, first, second string) (string, string) {
	t.Helper()
	i := strings.Index(src, first)
	j := strings.Index(src, second)
	if i < 0 || j < 0 || j <= i {
		t.Fatalf("cannot locate %q and %q in the factory source; this test reads source "+
			"text and the source has been reshaped", first, second)
	}
	return src[i:j], src[j:]
}

// stripCppComments removes // and /* */ comments, which matters here: the C++ factory
// carries two commented-out registrations (`to_type_of` and `cast_to`, behind a TODO)
// and counting them would report two divergences that do not exist.
func stripCppComments(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "//"):
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				return b.String()
			}
			i += j
		case strings.HasPrefix(s[i:], "/*"):
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return b.String()
			}
			i += j + 4
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func dedup(in []string) []string {
	out := slices.Clone(in)
	sort.Strings(out)
	return slices.Compact(out)
}
