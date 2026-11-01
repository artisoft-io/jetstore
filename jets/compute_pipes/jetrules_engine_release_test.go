package compute_pipes

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// THE JETRULES STEP RELEASES THE ENGINE IT BUILT, ON EVERY PATH.
//
// The native engine holds sqlite connections to workspace.db and lookup.db until it is
// released, and a cp_node lambda builds one per jetrules step per invocation in a
// process that serves many invocations. Until this test the step never released it, so
// a warm execution environment kept every engine's descriptors and a burst of runs
// failed on "too many open files". The descriptors themselves are counted against the
// real engine in jetrules_native_adaptor (engine_release_fd_test.go); this test holds
// the step to releasing what it builds, once, on success and on failure.

type countingEngine struct {
	JetRuleEngine
	released int
}

func (e *countingEngine) Release() error {
	e.released++
	return nil
}

type countingFactory struct{ engines []*countingEngine }

func (f *countingFactory) JetRulesName() string { return "counting" }
func (f *countingFactory) ClearCache() bool     { return true }
func (f *countingFactory) NewJetRuleEngine(_ *pgxpool.Pool, _ string, _ bool) (JetRuleEngine, error) {
	e := &countingEngine{}
	f.engines = append(f.engines, e)
	return e, nil
}

type countingProxy struct{ factory *countingFactory }

func (p countingProxy) GetNativeFactory() JetRulesFactory { return p.factory }

func TestJetrulesStepReleasesItsEngineWhenItFailsToBuild(t *testing.T) {
	factory := &countingFactory{}
	ctx := &BuilderContext{
		cpConfig: &ComputePipesConfig{
			CommonRuntimeArgs: &ComputePipesCommonArgs{},
			PipesConfig:       []PipeSpec{{}},
		},
		channelRegistry: &ChannelRegistry{ComputeChannels: map[string]*Channel{}},
		jetRules:        countingProxy{factory},
		env:             map[string]any{},
	}
	source := &InputChannel{Config: &ChannelSpec{}}
	// Many builds in one process, as a warm lambda serves many invocations: every
	// engine is released, each once.
	const invocations = 100
	for range invocations {
		spec := &TransformationSpec{Type: "jetrules", JetrulesConfig: &JetrulesSpec{
			ProcessName:    "p",
			OutputChannels: []OutputChannelConfig{{Name: "no_such_channel"}},
		}}
		_, err := ctx.NewJetrulesTransformationPipe(source, nil, spec)
		if err == nil || !strings.Contains(err.Error(), "no_such_channel") {
			t.Fatalf("want the missing output channel error, got %v", err)
		}
	}
	if len(factory.engines) != invocations {
		t.Fatalf("built %d engines, want %d", len(factory.engines), invocations)
	}
	for i, e := range factory.engines {
		if e.released != 1 {
			t.Errorf("engine %d released %d times, want 1", i, e.released)
		}
	}
}

func TestJetrulesStepReleasesItsEngineInFinally(t *testing.T) {
	engine := &countingEngine{}
	pipe := &JetrulesTransformationPipe{ruleEngine: engine}
	pipe.Finally()
	pipe.Finally() // a second call releases nothing
	if engine.released != 1 {
		t.Errorf("engine released %d times, want 1", engine.released)
	}
}
