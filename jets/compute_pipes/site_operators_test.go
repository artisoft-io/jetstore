package compute_pipes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/pipesmodel"
)

// `BD.3`: a registered operator runs, an unregistered name still produces
// today's error, a site name colliding with a built-in loses to the built-in,
// and `OperatorEnv` exposes nothing beyond `BD.1`'s list. The last of those
// lives in pipesmodel, beside the interface it pins.

var siteTestColumns = []string{"member_id", "value"}

// siteTestRig is a two-channel registry and a builder over it, which is the
// smallest thing that can run a pipe transformation. It is the render and ollama
// tests' shape with everything specific to those operators removed.
type siteTestRig struct {
	ctx      *BuilderContext
	registry *ChannelRegistry
	source   *InputChannel
	out      *OutputChannel
	errorOut *OutputChannel
}

func newSiteTestRig(t *testing.T) *siteTestRig {
	t.Helper()
	columnsMap := make(map[string]int, len(siteTestColumns))
	for i, c := range siteTestColumns {
		columnsMap[c] = i
	}
	spec := &ChannelSpec{Name: "rows", Columns: siteTestColumns}
	spec.SetColumnsMap(&columnsMap)

	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			"rows.out": {Name: "rows.out", Channel: make(chan []any, 8), Columns: &columnsMap, Config: spec},
			"rows.errors": {Name: "rows.errors", Channel: make(chan []any, 8),
				Columns: &columnsMap, Config: spec},
		},
		ClosedChannels: make(map[string]bool),
	}
	rig := &siteTestRig{
		registry: registry,
		source:   &InputChannel{Name: "rows.in", Columns: &columnsMap, Config: spec},
		ctx: &BuilderContext{
			cpConfig:        &ComputePipesConfig{},
			channelRegistry: registry,
			sessionId:       "session-1",
			done:            make(chan struct{}),
			errCh:           make(chan error, 10),
			env:             map[string]any{"$ENTITY": "member", "nbr_partitions": 6},
		},
	}
	var err error
	if rig.out, err = registry.GetOutputChannel("rows.out"); err != nil {
		t.Fatal(err)
	}
	if rig.errorOut, err = registry.GetOutputChannel("rows.errors"); err != nil {
		t.Fatal(err)
	}
	return rig
}

func (r *siteTestRig) withOperators(operators map[string]SiteOperatorFactory) *siteTestRig {
	o := applyCPOptions([]CPOption{WithOperators(operators)})
	r.ctx.siteOperators = o.siteOperators
	return r
}

// doublingOperator is the smallest thing that is recognisably an operator: it
// reads one column, writes one record, and reports a bad one on the error
// channel when it has one. It exists to be observed.
type doublingOperator struct {
	env      pipesmodel.OperatorEnv
	args     *pipesmodel.OperatorArgs
	factor   int
	applied  int
	doneCall int
}

type doublingConfig struct {
	Factor int    `json:"factor"`
	Label  string `json:"label"`
}

func newDoublingOperator(env pipesmodel.OperatorEnv, args *pipesmodel.OperatorArgs) (pipesmodel.PipeTransformationEvaluator, error) {
	op := &doublingOperator{env: env, args: args, factor: 2}
	if len(args.Config) > 0 {
		var c doublingConfig
		if err := json.Unmarshal(args.Config, &c); err != nil {
			return nil, fmt.Errorf("site_config.config: %v", err)
		}
		if c.Factor > 0 {
			op.factor = c.Factor
		}
	}
	return op, nil
}

func (op *doublingOperator) Apply(input *[]any) error {
	op.applied++
	value, ok := (*input)[1].(int)
	if !ok {
		if op.args.ErrorChannel == nil {
			return fmt.Errorf("value is not an int and there is no error channel")
		}
		select {
		case op.args.ErrorChannel.Channel <- []any{(*input)[0], "value is not an int"}:
		case <-op.env.Done():
		}
		return nil
	}
	select {
	case op.args.Output.Channel <- []any{(*input)[0], value * op.factor}:
	case <-op.env.Done():
	}
	return nil
}

func (op *doublingOperator) Done() error { op.doneCall++; return nil }
func (op *doublingOperator) Finally()    {}

// Criterion 94, first half: a registered site operator runs in a pipeline.
//
// It goes through BuildPipeTransformationEvaluator rather than calling the
// factory: what is under test is the dispatch, the argument assembly and the
// environment adapter, and calling the factory directly would exercise none of
// them.
func TestRegisteredSiteOperatorRuns(t *testing.T) {
	rig := newSiteTestRig(t).withOperators(map[string]SiteOperatorFactory{
		"site_double": newDoublingOperator,
	})
	spec := &TransformationSpec{
		Type:          "site_double",
		Comment:       "doubles the value column",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
		SiteConfig: &SiteOperatorSpec{
			ErrorChannel:  &OutputChannelConfig{Name: "rows.errors", SpecName: "rows"},
			MaxErrorCount: 7,
			Config:        json.RawMessage(`{"factor":3,"label":"triple"}`),
		},
	}

	pipe, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec)
	if err != nil {
		t.Fatalf("building the site operator: %v", err)
	}
	if pipe == nil {
		t.Fatal("the dispatch returned a nil evaluator for a registered operator")
	}
	op, ok := pipe.(*doublingOperator)
	if !ok {
		t.Fatalf("the dispatch built a %T", pipe)
	}

	// The argument assembly, which is what §12.4 decided.
	if op.args.Type != "site_double" || op.args.Comment != "doubles the value column" {
		t.Errorf("args carry type %q comment %q", op.args.Type, op.args.Comment)
	}
	if op.args.Output == nil || op.args.Output.Name != "rows.out" {
		t.Errorf("output channel is %v, want the resolved rows.out", op.args.Output)
	}
	if op.args.ErrorChannel == nil || op.args.ErrorChannel.Name != "rows.errors" {
		t.Errorf("error channel is %v, want the resolved rows.errors", op.args.ErrorChannel)
	}
	if op.args.MaxErrorCount != 7 {
		t.Errorf("max_error_count is %d, want 7", op.args.MaxErrorCount)
	}
	if op.factor != 3 {
		t.Errorf("the factory decoded factor %d from site_config.config, want 3", op.factor)
	}

	// And it runs.
	var wg sync.WaitGroup
	var output, errors [][]any
	wg.Add(2)
	go func() {
		defer wg.Done()
		for r := range rig.registry.ComputeChannels["rows.out"].Channel {
			output = append(output, r)
		}
	}()
	go func() {
		defer wg.Done()
		for r := range rig.registry.ComputeChannels["rows.errors"].Channel {
			errors = append(errors, r)
		}
	}()
	for _, record := range [][]any{{"m1", 21}, {"m2", "not an int"}, {"m3", 1}} {
		if err := pipe.Apply(&record); err != nil {
			t.Errorf("Apply: %v", err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	pipe.Finally()
	rig.registry.CloseChannel("rows.out")
	rig.registry.CloseChannel("rows.errors")
	wg.Wait()

	if len(output) != 2 || output[0][1] != 63 || output[1][1] != 3 {
		t.Errorf("output records are %v, want m1=63 and m3=3", output)
	}
	if len(errors) != 1 || errors[0][0] != "m2" {
		t.Errorf("error records are %v, want one for m2", errors)
	}
}

// Criterion 94, second half: an unknown name produces the message it produces
// today, and the *message* is the assertion rather than the fact of an error.
// A registry that is consulted in `default:` is also the thing most likely to
// change what an author sees when they mistype an operator, and this is the only
// diagnosis they get -- `ValidatePipeSpecConfig`'s operator switch has no
// default, so a bad token reaches a running worker before it is reported
// (`I-779`).
func TestUnregisteredOperatorProducesTheMessageItAlwaysDid(t *testing.T) {
	const want = "error: unknown TransformationSpec type: no_such_operator"
	spec := &TransformationSpec{
		Type:          "no_such_operator",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
	}

	// With no registry at all, which is every stock build.
	bare := newSiteTestRig(t)
	if _, err := bare.ctx.BuildPipeTransformationEvaluator(bare.source, nil, nil, spec); err == nil ||
		err.Error() != want {
		t.Errorf("with no registry: %v, want %q", err, want)
	}

	// And with a registry that does not hold the name, which is the case the
	// registry could have changed and must not.
	populated := newSiteTestRig(t).withOperators(map[string]SiteOperatorFactory{
		"site_double": newDoublingOperator,
	})
	if _, err := populated.ctx.BuildPipeTransformationEvaluator(populated.source, nil, nil, spec); err == nil ||
		err.Error() != want {
		t.Errorf("with a populated registry: %v, want %q", err, want)
	}
}

// Criterion 95: a site operator cannot shadow a built-in, demonstrated rather
// than asserted.
//
// The demonstration is that the built-in is what comes back. Asserting that the
// factory was not *called* would pass for a second reason -- an error before the
// dispatch reached either -- so the test registers a factory that fails loudly
// if it is ever reached and then checks the type of what was built.
func TestSiteOperatorCannotShadowABuiltin(t *testing.T) {
	var shadowCalled bool
	shadow := func(env pipesmodel.OperatorEnv, args *pipesmodel.OperatorArgs) (pipesmodel.PipeTransformationEvaluator, error) {
		shadowCalled = true
		return nil, fmt.Errorf("the site factory for %q was reached", args.Type)
	}
	rig := newSiteTestRig(t).withOperators(map[string]SiteOperatorFactory{"distinct": shadow})

	spec := &TransformationSpec{
		Type:           "distinct",
		DistinctConfig: &DistinctSpec{DistinctOn: []string{"member_id"}},
		OutputChannel:  OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
	}
	pipe, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec)
	if err != nil {
		t.Fatalf("building distinct with a shadowing registration: %v", err)
	}
	if shadowCalled {
		t.Error("the site factory was called for a built-in token")
	}
	if _, ok := pipe.(*DistinctTransformationPipe); !ok {
		t.Errorf("the dispatch built a %T for 'distinct', want the built-in", pipe)
	}
}

// The collision is registered and logged rather than refused, and the two halves
// of that are separate claims. WithOperators keeps the entry -- refusing it would
// mean a deployment cannot register a factory whose name a later JetStore release
// happens to take, and would turn a shadowed operator into a startup failure --
// and it warns, because losing silently is the failure mode this whole design
// exists to avoid.
func TestShadowingRegistrationIsKeptAndWarned(t *testing.T) {
	o := applyCPOptions([]CPOption{WithOperators(map[string]SiteOperatorFactory{
		"distinct":    newDoublingOperator,
		"site_double": newDoublingOperator,
	})})
	if len(o.siteOperators) != 2 {
		t.Errorf("registry holds %d operators, want both", len(o.siteOperators))
	}
	if o.siteOperators["distinct"] == nil {
		t.Error("the shadowing registration was dropped; it should be kept and never reached")
	}
}

// An empty name can never be dispatched to and a nil factory would panic at the
// call, so both are refused at registration where there is still somebody to
// tell.
func TestWithOperatorsRefusesWhatCannotWork(t *testing.T) {
	o := applyCPOptions([]CPOption{WithOperators(map[string]SiteOperatorFactory{
		"":            newDoublingOperator,
		"nil_factory": nil,
		"ok":          newDoublingOperator,
	})})
	if len(o.siteOperators) != 1 || o.siteOperators["ok"] == nil {
		t.Errorf("registry is %v, want only 'ok'", o.siteOperators)
	}
}

// Later calls merge, and a repeated name is last-wins. A site main composing two
// operator libraries is the case; nothing decides between them but the order the
// main wrote.
func TestWithOperatorsMerges(t *testing.T) {
	first := func(env pipesmodel.OperatorEnv, args *pipesmodel.OperatorArgs) (pipesmodel.PipeTransformationEvaluator, error) {
		return nil, fmt.Errorf("first")
	}
	o := applyCPOptions([]CPOption{
		WithOperators(map[string]SiteOperatorFactory{"a": first, "b": first}),
		WithOperators(map[string]SiteOperatorFactory{"a": newDoublingOperator}),
	})
	if len(o.siteOperators) != 2 {
		t.Fatalf("registry holds %d operators, want 2", len(o.siteOperators))
	}
	_, err := o.siteOperators["a"](nil, &pipesmodel.OperatorArgs{})
	if err != nil {
		t.Errorf("'a' is still the first registration: %v", err)
	}
}

// A factory that returns (nil, nil) would be silently skipped for the whole run:
// the builder uses a nil evaluator to mean "this step's `when` was false, do not
// apply it". Refuse it where it happens, with the operator named.
func TestNilEvaluatorFromAFactoryIsAnError(t *testing.T) {
	rig := newSiteTestRig(t).withOperators(map[string]SiteOperatorFactory{
		"site_nil": func(env pipesmodel.OperatorEnv, args *pipesmodel.OperatorArgs) (pipesmodel.PipeTransformationEvaluator, error) {
			return nil, nil
		},
	})
	spec := &TransformationSpec{Type: "site_nil",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"}}
	_, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec)
	if err == nil || !strings.Contains(err.Error(), "site_nil") {
		t.Errorf("building an operator that returns (nil, nil) gave %v", err)
	}
}

// The `when` condition is evaluated before the dispatch, so a site operator's
// factory is not reached when it is false -- and the caller gets the nil
// evaluator that means "skip", not the error above. This is the one place the
// two nils are both legitimate and they must not be confused.
func TestFalseWhenSkipsTheSiteFactory(t *testing.T) {
	called := false
	rig := newSiteTestRig(t).withOperators(map[string]SiteOperatorFactory{
		"site_double": func(env pipesmodel.OperatorEnv, args *pipesmodel.OperatorArgs) (pipesmodel.PipeTransformationEvaluator, error) {
			called = true
			return newDoublingOperator(env, args)
		},
	})
	spec := &TransformationSpec{
		Type:          "site_double",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
		When:          &ExpressionNode{Type: "value", Expr: "0"},
	}
	pipe, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec)
	if err != nil {
		t.Fatalf("building with a false `when`: %v", err)
	}
	if pipe != nil {
		t.Errorf("a false `when` built a %T, want nil", pipe)
	}
	if called {
		t.Error("the site factory was reached despite a false `when`")
	}
}

// A site operator that names an error channel nothing registered fails at build
// time with the channel named, rather than at the first bad record.
func TestSiteErrorChannelIsResolvedAtBuildTime(t *testing.T) {
	rig := newSiteTestRig(t).withOperators(map[string]SiteOperatorFactory{
		"site_double": newDoublingOperator,
	})
	for _, tc := range []struct {
		name string
		ec   *OutputChannelConfig
		want string
	}{
		{"no name", &OutputChannelConfig{SpecName: "rows"}, "name cannot be empty"},
		{"no spec name", &OutputChannelConfig{Name: "rows.errors"}, "spec name cannot be empty"},
		{"unregistered", &OutputChannelConfig{Name: "nowhere", SpecName: "rows"}, "nowhere"},
	} {
		spec := &TransformationSpec{Type: "site_double",
			OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
			SiteConfig:    &SiteOperatorSpec{ErrorChannel: tc.ec}}
		_, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want an error mentioning %q", tc.name, err, tc.want)
		}
	}
}

// The environment adapter, method by method. Five of the six are on §12.1's
// measurement and the sixth, IsDebugMode, is the one method admitted ahead of
// its evidence; all six are here because the adapter is what a site actually
// calls and a wrong one is not a compile error.
func TestOperatorEnvAdapter(t *testing.T) {
	rig := newSiteTestRig(t)
	rig.ctx.cpConfig.ClusterConfig = &ClusterSpec{IsDebugMode: true}
	env := operatorEnv{ctx: rig.ctx}

	if env.SessionId() != "session-1" {
		t.Errorf("SessionId is %q", env.SessionId())
	}
	if !env.IsDebugMode() {
		t.Error("IsDebugMode is false with the cluster config saying otherwise")
	}
	if got := env.Substitute("s3://bucket/$ENTITY/rows"); got != "s3://bucket/member/rows" {
		t.Errorf("Substitute gave %q", got)
	}
	// The half of the map Substitute cannot reach: it is keyed bare *and*
	// dollar-prefixed, and ReplaceEnvVars only ever matches the dollar keys.
	if v, ok := env.EnvValue("nbr_partitions"); !ok || v != 6 {
		t.Errorf("EnvValue(nbr_partitions) = %v, %v", v, ok)
	}
	if _, ok := env.EnvValue("no_such_key"); ok {
		t.Error("EnvValue reported a key that is not in the map")
	}
	select {
	case <-env.Done():
		t.Error("Done is already closed")
	default:
	}
	close(rig.ctx.done)
	select {
	case <-env.Done():
	default:
		t.Error("Done did not observe the builder's done channel closing")
	}

	// ColumnEvaluator delegates to the builder's own, so the authored column
	// vocabulary means the same thing in a site operator as in a built-in.
	value := "7"
	ce, err := env.ColumnEvaluator(rig.source, rig.out, &TransformationColumnSpec{
		Type: "value", Name: "value", Expr: &value,
	})
	if err != nil {
		t.Fatalf("ColumnEvaluator: %v", err)
	}
	record := make([]any, len(siteTestColumns))
	input := []any{"m1", 1}
	if err := ce.Update(&record, &input); err != nil {
		t.Fatal(err)
	}
	// int64(7) rather than the string "7": a `value` expression is *parsed*, and
	// which Go type it parses to is exactly the column vocabulary a site
	// operator would otherwise have to reimplement and get subtly wrong.
	if record[1] != int64(7) {
		t.Errorf("the evaluator wrote %v (%T) into the value column, want int64(7)", record[1], record[1])
	}
}

// A nil environment map is what a builder constructed without one has, and
// several tests in this package do exactly that. Neither accessor may panic.
func TestOperatorEnvAdapterToleratesANilEnvironment(t *testing.T) {
	env := operatorEnv{ctx: &BuilderContext{}}
	if _, ok := env.EnvValue("anything"); ok {
		t.Error("EnvValue found a key in a nil map")
	}
	if got := env.Substitute("$ENTITY"); got != "$ENTITY" {
		t.Errorf("Substitute over a nil map gave %q", got)
	}
	if env.IsDebugMode() {
		t.Error("IsDebugMode is true with no cpipes config at all")
	}
	// And with a config whose ClusterConfig pointer is nil, which is what
	// `&ComputePipesConfig{}` is and what a dozen tests in this package build.
	if (operatorEnv{ctx: &BuilderContext{cpConfig: &ComputePipesConfig{}}}).IsDebugMode() {
		t.Error("IsDebugMode is true with a nil cluster config")
	}
}

// `builtinOperatorTypes` is a second list of what the dispatch already knows,
// and a second list is how `reportsRowLevelFailures` and `errorChannelConfig`
// could have drifted apart. It is load-bearing twice over -- WithOperators warns
// from it and errorChannelConfig decides from it -- so it is parsed out of the
// switch rather than trusted.
func TestBuiltinOperatorTypesMatchesTheDispatch(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "pipes_runtime_model.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if d, ok := n.(*ast.FuncDecl); ok && d.Name.Name == "BuildPipeTransformationEvaluator" {
			fn = d
			return false
		}
		return true
	})
	if fn == nil {
		t.Fatal("BuildPipeTransformationEvaluator not found; this test is asserting nothing")
	}

	// The last type switch in the function body is the operator dispatch; the
	// `when` check above it is an if.
	dispatched := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok || sw.Tag == nil {
			return true
		}
		sel, ok := sw.Tag.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Type" {
			return true
		}
		for _, stmt := range sw.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expr := range clause.List {
				switch e := expr.(type) {
				case *ast.BasicLit:
					dispatched[strings.Trim(e.Value, `"`)] = true
				case *ast.Ident:
					// RenderOperatorType, a named constant.
					if e.Name == "RenderOperatorType" {
						dispatched[RenderOperatorType] = true
					} else {
						t.Errorf("the dispatch has a case on identifier %s that this test cannot resolve", e.Name)
					}
				}
			}
		}
		return false
	})

	if len(dispatched) == 0 {
		t.Fatal("parsed no cases out of the dispatch")
	}
	for token := range dispatched {
		if !builtinOperatorTypes[token] {
			t.Errorf("the dispatch handles %q and builtinOperatorTypes does not; "+
				"a site could register it and shadow nothing while believing it had", token)
		}
	}
	for token := range builtinOperatorTypes {
		if !dispatched[token] {
			t.Errorf("builtinOperatorTypes names %q and the dispatch does not; "+
				"WithOperators would warn about a collision that does not exist", token)
		}
	}
}

// errorChannelConfig answers for a site operator from the document, because the
// three callers that make an error channel exist all run in a process with no
// registry. The arm is keyed on site_config rather than on the token, which is
// the only arm in that function that is.
func TestErrorChannelConfigReadsSiteConfig(t *testing.T) {
	ec := &OutputChannelConfig{Name: "rows.errors", SpecName: "process_errors"}
	withChannel := &TransformationSpec{Type: "site_double", SiteConfig: &SiteOperatorSpec{ErrorChannel: ec}}
	if got := errorChannelConfig(withChannel); got != ec {
		t.Errorf("errorChannelConfig gave %v, want the site's channel", got)
	}
	without := &TransformationSpec{Type: "site_double", SiteConfig: &SiteOperatorSpec{}}
	if got := errorChannelConfig(without); got != nil {
		t.Errorf("errorChannelConfig gave %v for a site operator that named none", got)
	}
	none := &TransformationSpec{Type: "site_double"}
	if got := errorChannelConfig(none); got != nil {
		t.Errorf("errorChannelConfig gave %v for a site operator with no site_config", got)
	}
	// A built-in's own error channel is not displaced by a stray site_config,
	// which cannot be authored (validateSiteOperatorSpec refuses it) and would
	// otherwise send an error row to a channel the operator never opened.
	builtinEC := &OutputChannelConfig{Name: "map.errors", SpecName: "process_errors"}
	confused := &TransformationSpec{Type: "map_record",
		MapRecordConfig: &MapRecordSpec{ErrorChannel: builtinEC},
		SiteConfig:      &SiteOperatorSpec{ErrorChannel: ec}}
	if got := errorChannelConfig(confused); got != builtinEC {
		t.Errorf("errorChannelConfig gave %v for a map_record carrying a site_config, want the built-in's", got)
	}
}

// A site operator is never given a synthesised error channel, and that is the
// one deliberate asymmetry between errorChannelConfig and
// reportsRowLevelFailures -- whose agreement the package otherwise tests.
//
// The asymmetry is one-directional and benign in the direction it runs. The
// harmful direction is reportsRowLevelFailures saying yes where
// errorChannelConfig has no arm: synthesis would set a channel the operator
// cannot read back. This is the reverse -- an authored channel honoured, no
// synthesis -- and it is what JetStore not knowing a site operator's failure
// modes actually means.
func TestSiteOperatorGetsNoSynthesisedErrorChannel(t *testing.T) {
	t.Setenv("JETS_DEFAULT_ERROR_REPORTING", "")
	cpConfig := &ComputePipesConfig{}
	pipeConfig := []PipeSpec{{Apply: []TransformationSpec{
		{Type: "site_double"},
		{Type: "site_double", SiteConfig: &SiteOperatorSpec{}},
		{Type: "site_double", SiteConfig: &SiteOperatorSpec{
			ErrorChannel: &OutputChannelConfig{Name: "site.errors", SpecName: "process_errors"}}},
		{Type: "map_record"},
	}}}

	n := SynthesizeDefaultErrorChannels(cpConfig, pipeConfig)
	if n != 1 {
		t.Fatalf("synthesised %d channels, want 1 -- the map_record and nothing else", n)
	}
	for i := range 3 {
		if reportsRowLevelFailures(pipeConfig[0].Apply[i].Type) {
			t.Errorf("apply[%d]: reportsRowLevelFailures says yes for a site token", i)
		}
	}
	if ec := errorChannelConfig(&pipeConfig[0].Apply[2]); ec == nil || ec.Name != "site.errors" {
		t.Errorf("the authored site error channel is %v, want site.errors", ec)
	}
	for _, i := range []int{0, 1} {
		if ec := errorChannelConfig(&pipeConfig[0].Apply[i]); ec != nil {
			t.Errorf("apply[%d] was given %v; a site operator that named none gets none", i, ec)
		}
	}
}

// A site_config on a built-in token is refused at startup, which is where an
// author can still be told. It is the only thing about a site operator that the
// starter process can check at all: the registry is an argument to
// CoordinateComputePipes and the starters never call it, so a mistyped site
// token is reported by the dispatch inside a running worker (`I-779`).
func TestSiteConfigOnABuiltinIsRefused(t *testing.T) {
	err := validateSiteOperatorSpec(&TransformationSpec{Type: "distinct", SiteConfig: &SiteOperatorSpec{}})
	if err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Errorf("validateSiteOperatorSpec gave %v for a distinct carrying a site_config", err)
	}
	if err := validateSiteOperatorSpec(&TransformationSpec{Type: "site_double",
		SiteConfig: &SiteOperatorSpec{}}); err != nil {
		t.Errorf("validateSiteOperatorSpec refused a site operator: %v", err)
	}
	if err := validateSiteOperatorSpec(&TransformationSpec{Type: "distinct"}); err != nil {
		t.Errorf("validateSiteOperatorSpec refused a plain built-in: %v", err)
	}
}

// The configuration reaches the factory verbatim across the hop it actually
// makes: the starter marshals the whole document into
// jetsapi.cpipes_execution_status and the node reads it back with
// UnmarshalComputePipesConfig, which is a plain json.Unmarshal.
//
// Without the SiteConfig field this test is what failure looks like: it decodes
// cleanly and loses the block, with no error and no log line (`I-777`).
func TestSiteConfigSurvivesTheDocumentRoundTrip(t *testing.T) {
	const authored = `{
	  "cluster_config": {},
	  "pipes_config": [{
	    "type": "fan_out",
	    "input_channel": {"name": "input_row"},
	    "apply": [{
	      "type": "cgt_scrub",
	      "output_channel": {"name": "scrubbed", "channel_spec_name": "claim_row"},
	      "site_config": {
	        "error_channel": {"name": "scrub_errors", "channel_spec_name": "jetsapi_process_errors"},
	        "max_error_count": 5,
	        "config": {"policy": "strict", "columns": ["member_id", "ssn"]}
	      }
	    }]
	  }]
	}`
	// The hop the starter makes: marshal the parsed document, then read it back.
	first, err := UnmarshalComputePipesConfig(&[]string{authored}[0])
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	roundTripped := string(encoded)
	cpConfig, err := UnmarshalComputePipesConfig(&roundTripped)
	if err != nil {
		t.Fatal(err)
	}

	spec := &cpConfig.PipesConfig[0].Apply[0]
	if spec.SiteConfig == nil {
		t.Fatal("site_config was dropped by the round trip")
	}
	if spec.SiteConfig.MaxErrorCount != 5 {
		t.Errorf("max_error_count is %d, want 5", spec.SiteConfig.MaxErrorCount)
	}
	if spec.SiteConfig.ErrorChannel == nil || spec.SiteConfig.ErrorChannel.Name != "scrub_errors" {
		t.Errorf("error_channel is %v", spec.SiteConfig.ErrorChannel)
	}
	var decoded struct {
		Policy  string   `json:"policy"`
		Columns []string `json:"columns"`
	}
	if err := json.Unmarshal(spec.SiteConfig.Config, &decoded); err != nil {
		t.Fatalf("the site's own config did not survive: %v", err)
	}
	if decoded.Policy != "strict" || len(decoded.Columns) != 2 {
		t.Errorf("the site's config decoded to %+v", decoded)
	}
}

// A stock document is byte-identical through the same hop, which is what
// `omitzero` on the new field buys and is worth asserting rather than assuming:
// a field marshalling as `"site_config": null` on every one of the corpus's
// documents would be a change to every `.pc.json` JetStore ever writes back.
func TestSiteConfigIsAbsentFromADocumentThatNamesNone(t *testing.T) {
	spec := TransformationSpec{Type: "distinct", DistinctConfig: &DistinctSpec{}}
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "site_config") {
		t.Errorf("a step naming no site_config marshals as %s", encoded)
	}
}

// TestReservedOperatorTypesCoversTheResolvedAway pins the relationship between
// the two sets rather than their contents.
//
// `builtinOperatorTypes` is checked against the dispatch by
// TestBuiltinOperatorTypesMatchesTheDispatch, and that test is why `infer` cannot
// simply be added to it: `infer` is not a dispatch case and must not become one,
// because ResolveInferBackend rewrites it away before the graph is built.
//
// What this asserts is the containment and the reason for the gap. If a future
// token is resolved away the same way, the second loop fails until it is added to
// reservedOperatorTypes -- which is the point, since the failure it prevents is
// silent: a site factory registered under a rewritten token is never called and
// no collision warning fires.
func TestReservedOperatorTypesCoversTheResolvedAway(t *testing.T) {
	for token := range builtinOperatorTypes {
		if !reservedOperatorTypes[token] {
			t.Errorf("builtinOperatorTypes names %q and reservedOperatorTypes does not; "+
				"a site could register a built-in's token without a warning", token)
		}
	}
	if !reservedOperatorTypes["infer"] {
		t.Error(`reservedOperatorTypes does not name "infer"; ResolveInferBackend rewrites ` +
			`that token before the dispatch sees it, so a site operator registered under it ` +
			`is silently never called`)
	}
	if builtinOperatorTypes["infer"] {
		t.Error(`builtinOperatorTypes names "infer", but the dispatch has no such case; ` +
			`TestBuiltinOperatorTypesMatchesTheDispatch should already be failing`)
	}
	// The set difference is the resolved-away tokens, and there is one today. A
	// bare count would be a literal to keep in step; this names what it expects.
	for token := range reservedOperatorTypes {
		if !builtinOperatorTypes[token] && token != "infer" {
			t.Errorf("reservedOperatorTypes names %q, which is neither dispatched nor a known "+
				"resolved-away token; add it to this test's list with the pass that consumes it", token)
		}
	}
}

// `Q-148`: the error-row helper fills the five columns a site operator cannot
// reach, and the test reads them off a real *BuilderContext* rather than a
// fixture that agrees with the assertion.
//
// **This assertion moved here from the site**, which is the second-order
// consequence of choosing the helper over a `NodeId()` accessor and is worth a
// line. `demo_observe` used to assemble the row itself and assert its columns in
// `jets_ws`; a site test can now only assert that the operator *called*
// ReportError, because what lands in the row is JetStore's. So the column
// assertion belongs where the behaviour is, and a site operator that never
// touches a row is no longer evidence about whether a row is correct.
func TestReportErrorFillsTheColumnsASiteCannotReach(t *testing.T) {
	columns := append(append([]string{}, legacyProcessErrorColumns...), ProcessErrorDiscriminatorColumns...)
	outCh, ch := newTestErrorChannel("site.errors", columns)

	ctx := &BuilderContext{
		peKey:     4242,
		sessionId: "session-1",
		nodeId:    3,
		done:      make(chan struct{}),
		cpConfig: &ComputePipesConfig{
			CommonRuntimeArgs: &ComputePipesCommonArgs{MainInputStepId: "reducing02"},
		},
	}
	env := operatorEnv{ctx: ctx, operatorType: "demo_observe"}
	env.ReportError(outCh, RowLevelError{
		ErrorMessage: "value is not an int",
		InputColumn:  "amount",
		RowJetsKey:   "row-7",
	})

	row := <-ch
	got := map[string]any{}
	for name, pos := range *outCh.Columns {
		got[name] = row[pos]
	}
	// The five JetStore fills. shard_id and pipeline_execution_status_key are
	// `I-807` and `I-808`: NOT NULL and unreachable, and nullable and orphaned.
	if got["shard_id"] != 3 {
		t.Errorf("shard_id: got %v, want 3 -- a site operator used to write a placeholder 0 here", got["shard_id"])
	}
	if got["pipeline_execution_status_key"] != int64(4242) {
		t.Errorf("pipeline_execution_status_key: got %v, want 4242 -- the row joins to its run through this",
			got["pipeline_execution_status_key"])
	}
	if got["cpipes_step_id"] != "reducing02" {
		t.Errorf("cpipes_step_id: got %v, want reducing02", got["cpipes_step_id"])
	}
	if got["session_id"] != "session-1" {
		t.Errorf("session_id: got %v", got["session_id"])
	}
	// operator_type comes off the authored spec rather than off anything the
	// site passes, so a site operator cannot report under another name.
	if got["operator_type"] != "demo_observe" {
		t.Errorf("operator_type: got %v, want demo_observe", got["operator_type"])
	}
	if got["error_channel"] != "site.errors" {
		t.Errorf("error_channel: got %v", got["error_channel"])
	}

	// The three the site fills, two of them as sql.NullString.
	if got["error_message"] != "value is not an int" {
		t.Errorf("error_message: got %v", got["error_message"])
	}
	if v, ok := got["input_column"].(sql.NullString); !ok || !v.Valid || v.String != "amount" {
		t.Errorf("input_column: got %v", got["input_column"])
	}
	if v, ok := got["row_jets_key"].(sql.NullString); !ok || !v.Valid || v.String != "row-7" {
		t.Errorf("row_jets_key: got %v", got["row_jets_key"])
	}
	// And the rete columns, which no site operator has a session for.
	if v, ok := got["rete_session_triples"].(sql.NullString); !ok || v.Valid {
		t.Errorf("rete_session_triples should be NULL for a site operator, got %v", got["rete_session_triples"])
	}
}

// An unset field is SQL NULL rather than the empty string, which is what the
// built-ins do by setting a column only when they have a value for it. A triage
// query filtering on `input_column is not null` must not match every site row.
func TestReportErrorLeavesUnsetFieldsNull(t *testing.T) {
	columns := append(append([]string{}, legacyProcessErrorColumns...), ProcessErrorDiscriminatorColumns...)
	outCh, ch := newTestErrorChannel("site.errors", columns)
	env := operatorEnv{ctx: &BuilderContext{done: make(chan struct{})}, operatorType: "demo_observe"}
	env.ReportError(outCh, RowLevelError{ErrorMessage: "boom"})

	row := <-ch
	for _, name := range []string{"input_column", "row_jets_key", "grouping_key"} {
		v, ok := row[(*outCh.Columns)[name]].(sql.NullString)
		if !ok || v.Valid {
			t.Errorf("%s: got %v, want an invalid sql.NullString", name, row[(*outCh.Columns)[name]])
		}
	}
}

// **The nil channel is the half a `NodeId()` accessor could not have absorbed.**
// ErrorChannel is nil when the step authored none, which is documented and
// legitimate, so a site operator calls this unconditionally. All three nils are
// absorbed: the channel itself, its spec (which sizes the row) and its column
// map (which places the columns).
func TestReportErrorOnAnUnauthoredChannelIsANoOp(t *testing.T) {
	env := operatorEnv{ctx: &BuilderContext{done: make(chan struct{})}, operatorType: "demo_observe"}
	columns := map[string]int{"error_message": 0}
	for _, c := range []struct {
		name string
		ch   *OutputChannel
	}{
		{"nil channel", nil},
		{"nil spec", &OutputChannel{Name: "x", Channel: make(chan []any, 1), Columns: &columns}},
		{"nil columns", &OutputChannel{Name: "x", Channel: make(chan []any, 1),
			Config: &ChannelSpec{Name: "x", Columns: []string{"error_message"}}}},
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: ReportError panicked: %v", c.name, r)
				}
			}()
			env.ReportError(c.ch, RowLevelError{ErrorMessage: "boom"})
		}()
	}
}

// A channel spec written before the discriminator columns existed still gets a
// row, and the columns it does not declare are dropped rather than written over
// column zero. That is `setColumn`'s comma-ok form, and reimplementing it is
// what a site operator no longer has to do.
func TestReportErrorIsAdditiveOverAnOlderChannelSpec(t *testing.T) {
	outCh, ch := newTestErrorChannel("site.errors", legacyProcessErrorColumns)
	env := operatorEnv{ctx: &BuilderContext{nodeId: 2, done: make(chan struct{})},
		operatorType: "demo_observe"}
	env.ReportError(outCh, RowLevelError{ErrorMessage: "boom"})

	row := <-ch
	if len(row) != len(legacyProcessErrorColumns) {
		t.Fatalf("row is %d wide, want %d", len(row), len(legacyProcessErrorColumns))
	}
	if row[0] != int64(0) {
		t.Errorf("pipeline_execution_status_key at column 0 is %v; an unguarded map lookup "+
			"would have written the undeclared discriminators over it", row[0])
	}
	if row[(*outCh.Columns)["error_message"]] != "boom" {
		t.Errorf("error_message: got %v", row[(*outCh.Columns)["error_message"]])
	}
}

// The helper selects on the builder's done channel, so an operator reporting an
// error into a full channel during shutdown does not block the shutdown it is
// being asked to take part in. The site no longer writes that select itself.
func TestReportErrorIsInterruptedByDone(t *testing.T) {
	columns := []string{"error_message"}
	columnsMap := map[string]int{"error_message": 0}
	done := make(chan struct{})
	outCh := &OutputChannel{
		Name:    "site.errors",
		Channel: make(chan []any), // unbuffered and undrained
		Columns: &columnsMap,
		Config:  &ChannelSpec{Name: "site.errors", Columns: columns},
	}
	env := operatorEnv{ctx: &BuilderContext{done: done}, operatorType: "demo_observe"}
	close(done)
	returned := make(chan struct{})
	go func() {
		env.ReportError(outCh, RowLevelError{ErrorMessage: "boom"})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("ReportError blocked on an undrained channel with done closed")
	}
}

// ---------------------------------------------------------------------------
// P9-T01: `site_config.output_channels` and OperatorArgs.Outputs.
//
// The extension is ErrorChannel's move pluralised, so these tests are the
// error channel's tests pluralised: resolved at build time, refused when it
// cannot work, absent when nothing is declared, and surviving the document's
// two JSON hops. The one they add is the §12.6 assertion, which is about what
// the operator is *not* given.
// ---------------------------------------------------------------------------

// withChannels adds compute channels to the rig's registry, so a test can
// declare more output channels than the two every rig starts with.
func (r *siteTestRig) withChannels(t *testing.T, names ...string) *siteTestRig {
	t.Helper()
	columnsMap := make(map[string]int, len(siteTestColumns))
	for i, c := range siteTestColumns {
		columnsMap[c] = i
	}
	spec := &ChannelSpec{Name: "rows", Columns: siteTestColumns}
	spec.SetColumnsMap(&columnsMap)
	for _, name := range names {
		r.registry.ComputeChannels[name] = &Channel{
			Name: name, Channel: make(chan []any, 8), Columns: &columnsMap, Config: spec}
	}
	return r
}

// siteOutputsSpec is a site step declaring `output_channels` by name, with the
// step's own output_channel set to rows.out the way every other test here has it.
func siteOutputsSpec(names ...string) *TransformationSpec {
	channels := make([]OutputChannelConfig, 0, len(names))
	for _, name := range names {
		channels = append(channels, OutputChannelConfig{Name: name, SpecName: "rows"})
	}
	return &TransformationSpec{
		Type:          "site_double",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
		SiteConfig:    &SiteOperatorSpec{OutputChannels: channels},
	}
}

// capturingFactory records the args it was handed and returns an operator that
// does nothing, which is all these tests need: what is under test is the
// assembly, not the operator.
type capturingOperator struct{ args *pipesmodel.OperatorArgs }

func (op *capturingOperator) Apply(input *[]any) error { return nil }
func (op *capturingOperator) Done() error              { return nil }
func (op *capturingOperator) Finally()                 {}

func capturingFactory(captured **pipesmodel.OperatorArgs) SiteOperatorFactory {
	return func(env pipesmodel.OperatorEnv, args *pipesmodel.OperatorArgs) (pipesmodel.PipeTransformationEvaluator, error) {
		*captured = args
		return &capturingOperator{args: args}, nil
	}
}

// Criterion: a site step's declared output channels are resolved by the builder
// and handed over, in the order they were authored.
//
// It goes through BuildPipeTransformationEvaluator rather than calling
// siteOperatorArgs, for the reason TestRegisteredSiteOperatorRuns gives: what is
// under test is the argument assembly on the path a pipeline actually takes.
func TestSiteOutputChannelsAreResolvedAtBuildTime(t *testing.T) {
	var captured *pipesmodel.OperatorArgs
	rig := newSiteTestRig(t).withChannels(t, "rows.a", "rows.b", "rows.c").
		withOperators(map[string]SiteOperatorFactory{"site_double": capturingFactory(&captured)})

	// Authored out of registry order, so a test that passed by accident of map
	// iteration would have to pass by accident twice.
	spec := siteOutputsSpec("rows.c", "rows.a", "rows.b")
	if _, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec); err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("the factory was never called")
	}
	if len(captured.Outputs) != 3 {
		t.Fatalf("Outputs has %d channels, want 3", len(captured.Outputs))
	}
	for i, want := range []string{"rows.c", "rows.a", "rows.b"} {
		if captured.Outputs[i] == nil {
			t.Fatalf("Outputs[%d] is nil", i)
		}
		if captured.Outputs[i].Name != want {
			t.Errorf("Outputs[%d] is %q, want %q (authored order is the operator's handle)",
				i, captured.Outputs[i].Name, want)
		}
		// The registry's own channel, not a copy: a copy would give the operator
		// a channel nothing else reads from.
		registered, err := rig.registry.GetOutputChannel(want)
		if err != nil {
			t.Fatal(err)
		}
		if captured.Outputs[i].Channel != registered.Channel {
			t.Errorf("Outputs[%d] is not the registry's channel for %q", i, want)
		}
	}
	// The step's own output_channel is untouched and is not in the list.
	if captured.Output == nil || captured.Output.Name != "rows.out" {
		t.Errorf("Output is %v, want the step's own rows.out", captured.Output)
	}
	for i, out := range captured.Outputs {
		if out.Name == "rows.out" {
			t.Errorf("Outputs[%d] is the step's own output_channel; Outputs never contains Output", i)
		}
	}
}

// The refusals, each with the message that names what the author has to fix.
// The duplicate is the one that is not ErrorChannel's: a name repeated inside
// the list hands the operator one channel at two indices with nothing to tell
// them apart (D-208).
func TestSiteOutputChannelsRefuseWhatCannotWork(t *testing.T) {
	rig := newSiteTestRig(t).withChannels(t, "rows.a").
		withOperators(map[string]SiteOperatorFactory{"site_double": newDoublingOperator})
	for _, tc := range []struct {
		name     string
		channels []OutputChannelConfig
		want     string
	}{
		{"no name", []OutputChannelConfig{{SpecName: "rows"}}, "name cannot be empty"},
		{"no spec name", []OutputChannelConfig{{Name: "rows.a"}}, "spec name cannot be empty"},
		{"unregistered", []OutputChannelConfig{{Name: "nowhere", SpecName: "rows"}}, "nowhere"},
		{"repeated", []OutputChannelConfig{
			{Name: "rows.a", SpecName: "rows"},
			{Name: "rows.a", SpecName: "rows"}}, "twice"},
	} {
		spec := &TransformationSpec{Type: "site_double",
			OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
			SiteConfig:    &SiteOperatorSpec{OutputChannels: tc.channels}}
		_, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want an error mentioning %q", tc.name, err, tc.want)
		}
	}
}

// The other half of the additive claim, asserted rather than assumed: a step
// declaring no output_channels is handed exactly what it was handed before the
// field existed. A nil Outputs rather than an empty slice, because the two mean
// different things to a `for range` reader only by accident and to a `== nil`
// reader on purpose.
func TestSiteOutputChannelsAreAbsentWhenNoneAreDeclared(t *testing.T) {
	for _, tc := range []struct {
		name       string
		siteConfig *SiteOperatorSpec
	}{
		{"no site_config at all", nil},
		{"a site_config naming none", &SiteOperatorSpec{MaxErrorCount: 3}},
		{"an empty list", &SiteOperatorSpec{OutputChannels: []OutputChannelConfig{}}},
	} {
		var captured *pipesmodel.OperatorArgs
		rig := newSiteTestRig(t).withOperators(
			map[string]SiteOperatorFactory{"site_double": capturingFactory(&captured)})
		spec := &TransformationSpec{Type: "site_double",
			OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
			SiteConfig:    tc.siteConfig}
		if _, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if captured == nil {
			t.Fatalf("%s: the factory was never called", tc.name)
		}
		if captured.Outputs != nil {
			t.Errorf("%s: Outputs is %v, want nil", tc.name, captured.Outputs)
		}
		if captured.Output == nil || captured.Output.Name != "rows.out" {
			t.Errorf("%s: Output is %v, want the step's own rows.out", tc.name, captured.Output)
		}
	}
}

// §12.6's argument, asserted: an operator receives what its own step declared
// and can name nothing else.
//
// **Both halves are derived rather than listed.** The registry half is measured
// against the registry itself -- whatever channels the rig holds, the operator
// holds only the ones its step named -- so it stays true as the rig grows. The
// interface half is reflected off OperatorEnv rather than read: no method of it
// returns a channel or a registry, under any name, which is the property that
// would have to break for an operator to reach one. A list of method names
// would pass the day somebody adds `Channel(name string)`.
func TestSiteOperatorIsGivenNoWayToNameAnotherChannel(t *testing.T) {
	var captured *pipesmodel.OperatorArgs
	rig := newSiteTestRig(t).withChannels(t, "rows.a", "rows.b", "rows.secret").
		withOperators(map[string]SiteOperatorFactory{"site_double": capturingFactory(&captured)})
	spec := siteOutputsSpec("rows.a")
	if _, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec); err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("the factory was never called")
	}
	declared := map[string]bool{"rows.a": true, "rows.out": true, "rows.errors": true}
	reachable := map[string]bool{}
	for _, out := range captured.Outputs {
		reachable[out.Name] = true
	}
	if captured.Output != nil {
		reachable[captured.Output.Name] = true
	}
	if captured.ErrorChannel != nil {
		reachable[captured.ErrorChannel.Name] = true
	}
	if len(rig.registry.ComputeChannels) <= len(reachable) {
		t.Fatalf("the registry holds %d channels and the operator reaches %d; this test asserts nothing "+
			"unless the registry holds channels the step did not declare",
			len(rig.registry.ComputeChannels), len(reachable))
	}
	for name := range reachable {
		if !declared[name] {
			t.Errorf("the operator reaches channel %q, which its step did not declare", name)
		}
	}
	// And nothing on the interface would let it ask for one.
	envType := reflect.TypeOf((*pipesmodel.OperatorEnv)(nil)).Elem()
	for i := range envType.NumMethod() {
		m := envType.Method(i)
		for j := range m.Type.NumOut() {
			out := m.Type.Out(j)
			name := out.String()
			if strings.Contains(name, "OutputChannel") || strings.Contains(name, "ChannelRegistry") {
				t.Errorf("OperatorEnv.%s returns %s; §12.6 withholds the registry precisely so an "+
					"operator cannot name a channel its step did not declare", m.Name, name)
			}
		}
	}
}

// siteOutputChannelConfigs is the one function that reads the declared channels
// off the document, and the three passes that make a channel exist read it. This
// is that function: what it answers for a site token, and the built-in guard it
// carries for errorChannelConfig's reason.
func TestSiteOutputChannelConfigsReadsTheDocument(t *testing.T) {
	declared := []OutputChannelConfig{
		{Name: "rows.a", SpecName: "rows"}, {Name: "rows.b", SpecName: "rows"}}
	for _, tc := range []struct {
		name string
		spec *TransformationSpec
		want []string
	}{
		{"a site token with a list", &TransformationSpec{Type: "site_double",
			SiteConfig: &SiteOperatorSpec{OutputChannels: declared}}, []string{"rows.a", "rows.b"}},
		{"a site token with no list", &TransformationSpec{Type: "site_double",
			SiteConfig: &SiteOperatorSpec{}}, nil},
		{"no site_config", &TransformationSpec{Type: "site_double"}, nil},
		// The guard: validateSiteOperatorSpec refuses this document, and until it
		// does, answering for it would register channels for a step the dispatch
		// is never going to build.
		{"a built-in carrying a site_config", &TransformationSpec{Type: "map_record",
			SiteConfig: &SiteOperatorSpec{OutputChannels: declared}}, nil},
		{"the resolved-away token", &TransformationSpec{Type: "infer",
			SiteConfig: &SiteOperatorSpec{OutputChannels: declared}}, nil},
	} {
		got := siteOutputChannelConfigs(tc.spec)
		names := make([]string, 0, len(got))
		for _, ch := range got {
			names = append(names, ch.Name)
		}
		if strings.Join(names, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: %v, want %v", tc.name, names, tc.want)
		}
	}
	// And it yields the document's own configs rather than copies, which is what
	// the registry construction needs: it registers what it is handed.
	spec := &TransformationSpec{Type: "site_double",
		SiteConfig: &SiteOperatorSpec{OutputChannels: []OutputChannelConfig{{Name: "rows.a", SpecName: "rows"}}}}
	got := siteOutputChannelConfigs(spec)
	if len(got) != 1 || got[0] != &spec.SiteConfig.OutputChannels[0] {
		t.Error("siteOutputChannelConfigs returned a copy rather than the document's own config")
	}
	// outputChannelConfigs carries them, which is how the validator sees them.
	all := outputChannelConfigs(spec)
	found := false
	for _, ch := range all {
		if ch.Name == "rows.a" {
			found = true
		}
	}
	if !found {
		t.Error("outputChannelConfigs does not carry a site operator's declared channels")
	}
}

// A channel exists because some pass reads it off the document, and the passes
// that do are exactly the ones that already know jetrules writes channels its
// step's `output_channel` does not name.
//
// **The subject is derived from that fact rather than kept as a list here**
// (P3-I20): every non-test file mentioning `JetrulesConfig.OutputChannels` is a
// file that has to reckon with a plural output-channel declaration, so every one
// of them has to know about a site step's. A fourth such place added later joins
// this test by existing.
//
// **It was deliberately not satisfied by the two pipe executors when P9-T01
// wrote it**, and it said so by logging rather than failing. P9-T04 added the
// arm to both, so the report becomes the assertion: every file in the derived
// set must read the function. The count is asserted as well as the membership,
// because a glob that matched nothing and a package that satisfied everything
// produce the same silence.
func TestTheRegistryConstructionReadsTheSiteDeclaration(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("globbing the package: %v", err)
	}
	plural := map[string]bool{}
	reads := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		if strings.Contains(text, "JetrulesConfig.OutputChannels") {
			plural[f] = true
		}
		if strings.Contains(text, "siteOutputChannelConfigs(") {
			reads[f] = true
		}
	}
	if len(plural) == 0 {
		t.Fatal("no file mentions JetrulesConfig.OutputChannels; this test asserted nothing")
	}
	// The subject is three files: the validator's outputChannelConfigs and the two
	// pipe executors' close passes. Predicted at three and asserted, so that a
	// fourth place arriving is a failure here rather than a silence -- joining the
	// set is what a new plural-channel reader does, and this number is the only
	// thing that says somebody looked.
	if len(plural) != 3 {
		t.Errorf("the derived subject is %d file(s) %v, and it was three when this "+
			"assertion was written; a file joining or leaving the set has to be "+
			"reckoned with rather than absorbed", len(plural), sortedKeys(plural))
	}
	// What P9-T01 owns: the pass that registers channels.
	if !reads["compute_pipes.go"] {
		t.Error("the channel registry construction does not read siteOutputChannelConfigs; " +
			"a channel declared in site_config.output_channels would fail to resolve")
	}
	if !reads["actions_start_common.go"] {
		t.Error("outputChannelConfigs does not read siteOutputChannelConfigs; " +
			"the validator would not see a site operator's declared channels")
	}
	// What P9-T04 added: the two executors' close passes. Derived rather than
	// named, so the pair is not a list here -- a file that learns about jetrules'
	// plural channels has to learn about a site step's, and joins this demand by
	// existing.
	for _, f := range sortedKeys(plural) {
		if !reads[f] {
			t.Errorf("P9-I13: %s knows jetrules' plural output channels and not a site "+
				"step's; a channel declared in site_config.output_channels is registered "+
				"and resolved and closed by nothing, so its reader never sees EOF", f)
		}
	}
	// Keep the parser import honest: the glob above is the subject and a parse
	// failure would mean the file set is not what this test thinks it is.
	for f := range plural {
		if _, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly); err != nil {
			t.Errorf("parsing %s: %v", f, err)
		}
	}
}

// The document's two JSON hops, which is where a configuration block that is not
// a JSON-tagged field is dropped silently (I-777). The pair is
// TestSiteConfigSurvivesTheDocumentRoundTrip's, one field over.
func TestSiteOutputChannelsSurviveTheDocumentRoundTrip(t *testing.T) {
	const authored = `{
	  "cluster_config": {},
	  "pipes_config": [{
	    "type": "fan_out",
	    "input_channel": {"name": "input_row"},
	    "apply": [{
	      "type": "hc_generator",
	      "output_channel": {"name": "member", "channel_spec_name": "member_row"},
	      "site_config": {
	        "output_channels": [
	          {"name": "eligibility", "channel_spec_name": "eligibility_row"},
	          {"name": "claim_pharmacy", "channel_spec_name": "pharmacy_row"}
	        ]
	      }
	    }]
	  }]
	}`
	first, err := UnmarshalComputePipesConfig(&[]string{authored}[0])
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	roundTripped := string(encoded)
	cpConfig, err := UnmarshalComputePipesConfig(&roundTripped)
	if err != nil {
		t.Fatal(err)
	}
	spec := &cpConfig.PipesConfig[0].Apply[0]
	if spec.SiteConfig == nil {
		t.Fatal("site_config was dropped by the round trip")
	}
	if len(spec.SiteConfig.OutputChannels) != 2 {
		t.Fatalf("output_channels has %d entries after the round trip, want 2",
			len(spec.SiteConfig.OutputChannels))
	}
	for i, want := range []struct{ name, specName string }{
		{"eligibility", "eligibility_row"}, {"claim_pharmacy", "pharmacy_row"},
	} {
		got := spec.SiteConfig.OutputChannels[i]
		if got.Name != want.name || got.SpecName != want.specName {
			t.Errorf("output_channels[%d] is %s/%s, want %s/%s",
				i, got.Name, got.SpecName, want.name, want.specName)
		}
	}
	// And a site_config naming none does not grow the key, which is what
	// omitempty buys and is worth asserting rather than assuming.
	bare, err := json.Marshal(SiteOperatorSpec{MaxErrorCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "output_channels") {
		t.Errorf("a site_config naming no output_channels marshals as %s", bare)
	}
}

// ---------------------------------------------------------------------------
// P9-T02: `site_config.lookups` and OperatorArgs.Lookups.
//
// Built for the contract rather than for this corpus (D-202), so its first
// consumer is these tests: the corpus operator keeps its own reference-table
// package and does not use the extension. That makes the tests the only thing
// standing between the field and P4-I43's class -- a component whose own tests
// pass and which nothing on a run's path ever reaches -- so they exercise both
// ends: the starter pass that decides a table is loaded at all, and the builder
// pass that hands it over.
// ---------------------------------------------------------------------------

// siteLookupTableFake is the smallest thing that is a LookupTable. It exists to be
// handed over and recognised, which is all these tests ask of it.
type siteLookupTableFake struct {
	name string
	rows map[string]*[]any
}

func (t *siteLookupTableFake) Lookup(key *string) (*[]any, error) {
	if key == nil {
		return nil, fmt.Errorf("nil key")
	}
	return t.rows[*key], nil
}
func (t *siteLookupTableFake) LookupValue(row *[]any, columnName string) (any, error) {
	if row == nil || len(*row) == 0 {
		return nil, nil
	}
	return (*row)[0], nil
}
func (t *siteLookupTableFake) ColumnMap() map[string]int { return map[string]int{"value": 0} }
func (t *siteLookupTableFake) IsEmptyTable() bool        { return len(t.rows) == 0 }
func (t *siteLookupTableFake) Size() int64               { return int64(len(t.rows)) }

// withLookups gives the rig a lookup table manager holding the named tables.
func (r *siteTestRig) withLookups(names ...string) *siteTestRig {
	mgr := NewLookupTableManager(nil, nil, false)
	for _, name := range names {
		mgr.LookupTableMap[name] = &siteLookupTableFake{
			name: name, rows: map[string]*[]any{"k": {name}}}
	}
	r.ctx.lookupTableManager = mgr
	return r
}

func siteLookupsSpec(keys ...string) *TransformationSpec {
	return &TransformationSpec{
		Type:          "site_double",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
		SiteConfig:    &SiteOperatorSpec{Lookups: keys},
	}
}

// The declared tables are resolved and handed over, in the order authored, and
// what arrives is the manager's own table rather than a copy or a view.
func TestSiteLookupsAreResolvedAtBuildTime(t *testing.T) {
	var captured *pipesmodel.OperatorArgs
	rig := newSiteTestRig(t).withLookups("zip_to_state", "codes", "unused").
		withOperators(map[string]SiteOperatorFactory{"site_double": capturingFactory(&captured)})

	spec := siteLookupsSpec("codes", "zip_to_state")
	if _, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec); err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("the factory was never called")
	}
	if len(captured.Lookups) != 2 {
		t.Fatalf("Lookups has %d entries, want 2", len(captured.Lookups))
	}
	for i, want := range []string{"codes", "zip_to_state"} {
		got := captured.Lookups[i]
		if got == nil || got.Key != want {
			t.Fatalf("Lookups[%d] is %v, want key %q (authored order is the operator's handle)", i, got, want)
		}
		if got.Table != rig.ctx.lookupTableManager.LookupTableMap[want] {
			t.Errorf("Lookups[%d] is not the manager's own table for %q", i, want)
		}
		// And it reads: the site gets the object a built-in gets, so the keyed
		// read is the same keyed read.
		key := "k"
		row, err := got.Table.Lookup(&key)
		if err != nil || row == nil {
			t.Fatalf("Lookups[%d].Table.Lookup: %v, %v", i, row, err)
		}
		if value, err := got.Table.LookupValue(row, "value"); err != nil || value != want {
			t.Errorf("Lookups[%d] resolved to table %v, want %q", i, value, want)
		}
	}
	// A table the step did not name is not handed over, even though it is loaded.
	for _, l := range captured.Lookups {
		if l.Key == "unused" {
			t.Error("the operator was handed a table its step did not declare")
		}
	}
}

// The refusals. The miss is the one that differs from what the built-ins do, and
// the comment on resolveSiteLookups says why: a nil LookupTable would turn a
// load failure into a nil dereference inside site code.
func TestSiteLookupsRefuseWhatCannotWork(t *testing.T) {
	for _, tc := range []struct {
		name        string
		keys        []string
		withManager bool
		want        string
	}{
		{"empty key", []string{""}, true, "cannot be empty"},
		{"repeated key", []string{"codes", "codes"}, true, "twice"},
		{"not loaded", []string{"nowhere"}, true, "is not loaded"},
		{"no manager at all", []string{"codes"}, false, "prepared none"},
	} {
		rig := newSiteTestRig(t).withOperators(
			map[string]SiteOperatorFactory{"site_double": newDoublingOperator})
		if tc.withManager {
			rig = rig.withLookups("codes")
		}
		_, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, siteLookupsSpec(tc.keys...))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want an error mentioning %q", tc.name, err, tc.want)
		}
	}
}

// A step declaring no lookups is handed exactly what it was handed before the
// field existed -- including when the node prepared no lookup tables at all,
// which is every pipeline in the corpus that uses none.
func TestSiteLookupsAreAbsentWhenNoneAreDeclared(t *testing.T) {
	for _, tc := range []struct {
		name       string
		siteConfig *SiteOperatorSpec
		manager    bool
	}{
		{"no site_config at all", nil, false},
		{"a site_config naming none", &SiteOperatorSpec{MaxErrorCount: 3}, false},
		{"an empty list", &SiteOperatorSpec{Lookups: []string{}}, false},
		{"none declared, tables loaded", &SiteOperatorSpec{}, true},
	} {
		var captured *pipesmodel.OperatorArgs
		rig := newSiteTestRig(t).withOperators(
			map[string]SiteOperatorFactory{"site_double": capturingFactory(&captured)})
		if tc.manager {
			rig = rig.withLookups("codes")
		}
		spec := &TransformationSpec{Type: "site_double",
			OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
			SiteConfig:    tc.siteConfig}
		if _, err := rig.ctx.BuildPipeTransformationEvaluator(rig.source, nil, nil, spec); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if captured == nil {
			t.Fatalf("%s: the factory was never called", tc.name)
		}
		if captured.Lookups != nil {
			t.Errorf("%s: Lookups is %v, want nil", tc.name, captured.Lookups)
		}
	}
}

// The starter half, and the half the resolution cannot stand without.
//
// SelectActiveLookupTable prunes `lookup_tables` to what some step references,
// in a process that knows nothing about site operators. A site operator's
// declared lookup survives that pruning, an undefined one is refused there by
// name, and a step declaring none prunes exactly as it did before -- which is
// the assertion that keeps this additive.
func TestSelectActiveLookupTableKeepsASiteOperatorsLookups(t *testing.T) {
	lookupConfig := []*LookupSpec{
		{Key: "zip_to_state", Type: "s3_csv_lookup"},
		{Key: "codes", Type: "s3_csv_lookup"},
		{Key: "unreferenced", Type: "s3_csv_lookup"},
	}
	siteStep := func(keys ...string) []PipeSpec {
		return []PipeSpec{{Apply: []TransformationSpec{{
			Type: "hc_generator", SiteConfig: &SiteOperatorSpec{Lookups: keys}}}}}
	}

	active, err := SelectActiveLookupTable(lookupConfig, siteStep("codes", "zip_to_state"))
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(active))
	for _, spec := range active {
		got = append(got, spec.Key)
	}
	if strings.Join(got, ",") != "codes,zip_to_state" {
		t.Errorf("active lookups are %v, want [codes zip_to_state]", got)
	}

	// An undefined name is refused here, by name, the way every other operator's
	// reference is -- rather than surfacing as a nil table two processes later.
	if _, err := SelectActiveLookupTable(lookupConfig, siteStep("nowhere")); err == nil ||
		!strings.Contains(err.Error(), "nowhere") {
		t.Errorf("an undefined site lookup gave %v, want an error naming it", err)
	}

	// A step declaring none prunes as it always did: nothing active.
	none, err := SelectActiveLookupTable(lookupConfig, siteStep())
	if err != nil || len(none) != 0 {
		t.Errorf("a site step declaring no lookups made %d table(s) active (%v)", len(none), err)
	}

	// And the built-in guard: a site_config on a built-in token answers nothing
	// here, because validateSiteOperatorSpec refuses that document and the
	// dispatch will never build it.
	builtin := []PipeSpec{{Apply: []TransformationSpec{{
		Type: "map_record", SiteConfig: &SiteOperatorSpec{Lookups: []string{"codes"}}}}}}
	active, err = SelectActiveLookupTable(lookupConfig, builtin)
	if err != nil || len(active) != 0 {
		t.Errorf("a built-in carrying a site_config made %d table(s) active (%v)", len(active), err)
	}
}

// The document's two JSON hops, where a block that is not a JSON-tagged field is
// dropped silently (I-777).
func TestSiteLookupsSurviveTheDocumentRoundTrip(t *testing.T) {
	const authored = `{
	  "cluster_config": {},
	  "lookup_tables": [{"key": "zip_to_state", "type": "s3_csv_lookup"}],
	  "pipes_config": [{
	    "type": "fan_out",
	    "input_channel": {"name": "input_row"},
	    "apply": [{
	      "type": "cgt_scrub",
	      "output_channel": {"name": "scrubbed", "channel_spec_name": "claim_row"},
	      "site_config": {"lookups": ["zip_to_state", "codes"]}
	    }]
	  }]
	}`
	first, err := UnmarshalComputePipesConfig(&[]string{authored}[0])
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	roundTripped := string(encoded)
	cpConfig, err := UnmarshalComputePipesConfig(&roundTripped)
	if err != nil {
		t.Fatal(err)
	}
	spec := &cpConfig.PipesConfig[0].Apply[0]
	if spec.SiteConfig == nil {
		t.Fatal("site_config was dropped by the round trip")
	}
	if strings.Join(spec.SiteConfig.Lookups, ",") != "zip_to_state,codes" {
		t.Errorf("lookups after the round trip are %v", spec.SiteConfig.Lookups)
	}
	bare, err := json.Marshal(SiteOperatorSpec{MaxErrorCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "lookups") {
		t.Errorf("a site_config naming no lookups marshals as %s", bare)
	}
}

// siteLookupKeys, and the built-in guard it carries for siteOutputChannelConfigs'
// reason.
func TestSiteLookupKeysReadsTheDocument(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *TransformationSpec
		want string
	}{
		{"a site token with a list", &TransformationSpec{Type: "site_double",
			SiteConfig: &SiteOperatorSpec{Lookups: []string{"a", "b"}}}, "a,b"},
		{"a site token with no list", &TransformationSpec{Type: "site_double",
			SiteConfig: &SiteOperatorSpec{}}, ""},
		{"no site_config", &TransformationSpec{Type: "site_double"}, ""},
		{"a built-in carrying a site_config", &TransformationSpec{Type: "anonymize",
			SiteConfig: &SiteOperatorSpec{Lookups: []string{"a"}}}, ""},
		{"the resolved-away token", &TransformationSpec{Type: "infer",
			SiteConfig: &SiteOperatorSpec{Lookups: []string{"a"}}}, ""},
	} {
		if got := strings.Join(siteLookupKeys(tc.spec), ","); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
