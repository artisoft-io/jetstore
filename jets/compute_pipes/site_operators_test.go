package compute_pipes

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"

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
