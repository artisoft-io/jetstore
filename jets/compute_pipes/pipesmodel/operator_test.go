package pipesmodel

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// Criterion 93: OperatorEnv exposes exactly the list `BD.1` decided, and this
// test fails if it grows.
//
// **Six until `Q-148` on 2026-09-15, seven since**, and the growth is the point
// rather than an exception to it. The seventh is ReportError, and the test is
// what made adding it a decision somebody took: the list was edited in the same
// commit as the interface, with plan §22 as the argument. A method that arrives
// without this test going red is a method nobody weighed.
//
// **It pins full method signatures rather than method names**, and the
// difference is the whole reason it is written with reflection instead of the
// source parse its sibling in imports_test.go uses. A name list catches a method
// added and misses a parameter widened -- `EnvValue(string) (any, bool)`
// becoming `EnvValue(string) (any, error)`, or ColumnEvaluator taking a
// `*TransformationSpec` instead of a `*TransformationColumnSpec`. Either would
// hand a site operator something §12.6 withheld, under a name the list already
// approves of.
//
// Mutation-checked the way `BC.3` checked its two: a seventh method added to the
// interface fails it by name, and a widened parameter fails it by signature.
//
// The comparison runs both ways on purpose. A one-way check would let the
// interface grow as long as the six were still there, which is exactly the
// failure R-149 names: a public surface is hard to shrink, so the moment to
// notice a method is the moment it is added.
func TestOperatorEnvExposesExactlyTheDecidedList(t *testing.T) {
	typ := reflect.TypeOf((*OperatorEnv)(nil)).Elem()
	if typ.Kind() != reflect.Interface {
		t.Fatalf("OperatorEnv is a %s, not an interface", typ.Kind())
	}

	got := map[string]string{}
	for i := range typ.NumMethod() {
		m := typ.Method(i)
		got[m.Name] = m.Type.String()
	}

	want := map[string]string{
		"Done":       "func() <-chan struct {}",
		"Substitute": "func(string) string",
		"EnvValue":   "func(string) (interface {}, bool)",
		"ColumnEvaluator": "func(*pipesmodel.InputChannel, *pipesmodel.OutputChannel, " +
			"*pipesmodel.TransformationColumnSpec) (pipesmodel.TransformationColumnEvaluator, error)",
		"SessionId":   "func() string",
		"IsDebugMode": "func() bool",
		"ReportError": "func(*pipesmodel.OutputChannel, pipesmodel.RowLevelError)",
	}

	for name, wantSig := range want {
		gotSig, ok := got[name]
		switch {
		case !ok:
			t.Errorf("OperatorEnv has lost %s; removing a method breaks every site that consumes it", name)
		case gotSig != wantSig:
			t.Errorf("OperatorEnv.%s is %s, want %s", name, gotSig, wantSig)
		}
	}
	for name, gotSig := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("OperatorEnv has gained %s %s; §12.6 withheld everything not on the list, "+
				"so adding one is a decision to record rather than a field to add", name, gotSig)
		}
	}
	if len(got) != len(want) {
		names := make([]string, 0, len(got))
		for name := range got {
			names = append(names, name)
		}
		sort.Strings(names)
		t.Errorf("OperatorEnv has %d methods, want %d: %v", len(got), len(want), names)
	}
}

// OperatorArgs is written by JetStore and read by a site, so a field added to it
// breaks nobody -- which is exactly why it needs pinning too. The asymmetry that
// makes widening safe is the same one that makes widening invisible.
//
// The three that are deliberately absent are the load-bearing part: `when` and
// `conditional_config` are consumed before a factory can be reached, and
// `*TransformationSpec` itself never crosses.
//
// **Nine until Phase 9's P9-T01, ten since.** `Outputs` is the first of the two
// extensions D-202 rules, and it is ErrorChannel's move repeated: the builder
// resolves what the step itself declared and hands the result over, so §12.6's
// argument is intact -- the operator still cannot reach `channelRegistry` and
// still names nothing its own step did not. That it arrives as a *field* here
// rather than as a method on OperatorEnv is the whole of the shape: this struct
// is written by JetStore and read by a site, so widening it breaks nobody, and
// the interface is not.
func TestOperatorArgsCarriesExactlyTheDecidedFields(t *testing.T) {
	typ := reflect.TypeOf(OperatorArgs{})
	got := map[string]string{}
	for i := range typ.NumField() {
		f := typ.Field(i)
		got[f.Name] = f.Type.String()
	}
	want := map[string]string{
		"Type":          "string",
		"Comment":       "string",
		"NewRecord":     "bool",
		"Columns":       "[]pipesmodel.TransformationColumnSpec",
		"Source":        "*pipesmodel.InputChannel",
		"Output":        "*pipesmodel.OutputChannel",
		"Outputs":       "[]*pipesmodel.OutputChannel",
		"ErrorChannel":  "*pipesmodel.OutputChannel",
		"MaxErrorCount": "int",
		"Config":        "json.RawMessage",
	}
	for name, wantType := range want {
		gotType, ok := got[name]
		switch {
		case !ok:
			t.Errorf("OperatorArgs has lost %s", name)
		case gotType != wantType:
			t.Errorf("OperatorArgs.%s is %s, want %s", name, gotType, wantType)
		}
	}
	for name, gotType := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("OperatorArgs has gained %s %s; record the decision", name, gotType)
		}
	}
}

// `Config` is json.RawMessage and not `map[string]any`, and the difference is
// that the site owns its own schema: it unmarshals into a typed struct of its
// own and validates it, which is the whole benefit of the platform not knowing
// what is in there (`I-777`).
//
// The property under test is that the bytes survive verbatim. The document
// crosses a process boundary as JSON twice before a factory sees it, and a field
// type that re-encodes what it decoded would not necessarily round-trip key
// order, number formatting or an unknown key.
func TestOperatorArgsConfigSurvivesAJSONRoundTrip(t *testing.T) {
	const authored = `{"policy":"strict","columns":["member_id","ssn"],"threshold":0.9500}`
	type siteConfig struct {
		Config json.RawMessage `json:"config"`
	}
	encoded, err := json.Marshal(siteConfig{Config: json.RawMessage(authored)})
	if err != nil {
		t.Fatal(err)
	}
	var back siteConfig
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}
	args := OperatorArgs{Config: back.Config}
	if string(args.Config) != authored {
		t.Errorf("config after a round trip is %s, want %s", args.Config, authored)
	}
	// And the site decodes it into whatever type it likes.
	var decoded struct {
		Policy    string   `json:"policy"`
		Columns   []string `json:"columns"`
		Threshold float64  `json:"threshold"`
	}
	if err := json.Unmarshal(args.Config, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Policy != "strict" || len(decoded.Columns) != 2 || decoded.Threshold != 0.95 {
		t.Errorf("site decode gave %+v", decoded)
	}
}

// The test-double remedy of `I-781`, compiled rather than described. A site that
// implements OperatorEnv is broken by a seventh method; a site that *embeds* it
// is not, and gets a panic on first use of the method it did not write instead
// of a build failure it cannot fix without a release.
type embeddingFake struct {
	OperatorEnv // nil: any method not overridden below panics on call
	done        chan struct{}
}

func (f *embeddingFake) Done() <-chan struct{} { return f.done }

func TestEmbeddingFakeSatisfiesOperatorEnv(t *testing.T) {
	done := make(chan struct{})
	var env OperatorEnv = &embeddingFake{done: done}
	if env.Done() != (<-chan struct{})(done) {
		t.Error("the embedded fake did not return its own done channel")
	}
	defer func() {
		if recover() == nil {
			t.Error("a method the fake did not override should panic on use, not return a zero value")
		}
	}()
	_ = env.SessionId()
}

// RowLevelError is written by the site and read by JetStore, which is the
// reverse of OperatorArgs -- so *removing* a field here breaks a site and adding
// one does not, and the pinning is against silent shrinkage as much as growth.
//
// The three are what JetStore's own operators set on a ProcessError, measured
// against every caller of NewProcessError on 2026-09-15. Two columns of
// jetsapi.process_errors are deliberately not fields: `grouping_key`, which no
// caller sets, and the two rete_session columns, which belong to a rules session
// no site operator has.
func TestRowLevelErrorCarriesExactlyTheDecidedFields(t *testing.T) {
	typ := reflect.TypeOf(RowLevelError{})
	got := map[string]string{}
	for i := range typ.NumField() {
		f := typ.Field(i)
		got[f.Name] = f.Type.String()
	}
	want := map[string]string{
		"ErrorMessage": "string",
		"InputColumn":  "string",
		"RowJetsKey":   "string",
	}
	for name, wantType := range want {
		gotType, ok := got[name]
		switch {
		case !ok:
			t.Errorf("RowLevelError has lost %s; a site that set it no longer compiles", name)
		case gotType != wantType:
			t.Errorf("RowLevelError.%s is %s, want %s", name, gotType, wantType)
		}
	}
	for name, gotType := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("RowLevelError has gained %s %s; a column offered to a site operator is a "+
				"decision to record rather than a field to add", name, gotType)
		}
	}
}
