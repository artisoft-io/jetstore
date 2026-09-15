package pipesmodel

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The whole point of this package is what it does *not* reach. `compute_pipes`
// imports Apache Arrow, the AWS SDK, pgx, snappy, chardet, xlsxreader and the
// jsii runtime; a site operator implementing a three-method interface should
// inherit none of it.
//
// **Criterion 90 said "no AWS, Arrow, pgx or jsii". `BC.1` made it stronger by
// measurement** — the surface needs no `jets/agentic/template` either — so this
// asserts the stronger property: **standard library only**. A named deny list
// would pass the day somebody adds the dependency nobody thought to name.
//
// It is also the guard on the finding that set the surface at 13 types rather
// than 19. `OutputChannelConfig` was dropped because a factory receives the
// resolved `*OutputChannel` and never the spec, and dropping it is what removed
// `ParquetSchemaInfo` from the closure — a type holding an unexported
// `*arrow.Schema`, which cannot cross without bringing Arrow with it. Put
// `OutputChannelConfig` back and this test is how you find out.
func TestPackageImportsStandardLibraryOnly(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("globbing the package: %v", err)
	}
	checked := 0
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", f, err)
		}
		checked++
		for _, imp := range af.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			// A standard library path has no dot in its first segment: "fmt",
			// "go/parser", "encoding/json". Everything else is a module path.
			if strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
				t.Errorf("%s imports %q; this package must reach the standard library only", f, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no files parsed, so this test asserted nothing")
	}
}

// The surface is 18 named types, and the number is load-bearing rather than
// decorative: each one is a commitment to whoever writes a site operator, and
// R-149 says a public surface is hard to shrink. A type arriving here should be
// a decision somebody took, not a field somebody added to a struct that already
// crossed.
//
// **Eighteen since `Q-148` on 2026-09-15**, which added RowLevelError: the
// per-record half of an error row, which a site fills and OperatorEnv.ReportError
// completes. It is three strings and it is on the surface because a site names it
// at every call site -- the alternative shapes, a parameter list or an `error`,
// were weighed in plan §22 and neither survives a fourth column.
//
// **Thirteen until `BD.2`**, which added the four the operator contract needs:
// OperatorEnv, OperatorArgs, SiteOperatorFactory, and
// TransformationColumnEvaluator, the last moving out of `compute_pipes` because
// it is OperatorEnv.ColumnEvaluator's return type. The name carries the count on
// purpose -- editing a literal is a smaller act than renaming a test, and the
// point of the number is that moving it should feel like a decision.
//
// **It reads every non-test file in the package, and it read `model.go` alone
// until `BD.2`.** That was a file-scoped assertion wearing a package-scoped
// name: `operator.go` would have added four types to the surface with the test
// still green and still reporting thirteen. Found by writing the second file,
// which is the only way a single-file corpus ever finds it.
func TestSurfaceIsEighteenTypes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("globbing the package: %v", err)
	}
	got := map[string]bool{}
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, line := range strings.Split(string(src), "\n") {
			// Column zero only: a type declared inside a function body is not
			// part of the package's surface.
			if strings.HasPrefix(line, "type ") {
				got[strings.Fields(line)[1]] = true
			}
		}
	}
	if scanned < 2 {
		t.Fatalf("scanned %d file(s); the package has more than one and this test is meant to read them all", scanned)
	}
	want := []string{
		"CaseExpression", "ChannelSpec", "ColumnEncodingSpec", "DomainKeyInfo",
		"DomainKeysSpec", "ExpressionNode", "HashExpression", "InputChannel",
		"LookupColumnSpec", "MapExpression", "OperatorArgs", "OperatorEnv",
		"OutputChannel", "PipeTransformationEvaluator", "RowLevelError",
		"SiteOperatorFactory", "TransformationColumnEvaluator",
		"TransformationColumnSpec",
	}
	if len(got) != len(want) {
		t.Errorf("surface is %d types, want %d: %v", len(got), len(want), got)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("%s is missing from the surface", w)
		}
	}
}
