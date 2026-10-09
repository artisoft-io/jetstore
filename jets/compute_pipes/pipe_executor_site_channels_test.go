package compute_pipes

import (
	"maps"
	"slices"
	"testing"
)

// P9-I13: a channel declared in `site_config.output_channels` is registered by
// StartComputePipes and resolved onto OperatorArgs.Outputs, and until P9-T04 it
// was closed by nothing.
//
// **Why that is a defect and not a shortfall.** Every reader of a compute
// channel is `for row := range ch`, so a channel nobody closes is a reader that
// never returns: twelve partition writers fed by one site operator would each
// block forever, and the run would hang rather than fail. It is the class
// P4-I43 names one level out -- a component whose own tests pass and which
// reaches no working path -- because the resolution half was complete and the
// close half made it unusable.
//
// These two tests run the executors rather than reading them, which is the half
// the AST test in site_operators_test.go cannot do: a file can mention
// siteOutputChannelConfigs and still not close what it answers.

// sortedKeys is the deterministic order a findings list is reported in. A map's
// iteration order is not a fact about the package under test, and a test that
// reported in it would produce a different message on different runs.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

// siteChannelRig extends newSiteTestRig with the two things an executor needs
// and a builder alone does not: a cluster config to read the debug flag off,
// and a source channel that is closed, so the pipe's `range` returns at once.
//
// The extra channels are named for what they are: the step's own output channel
// and two the site step declares.
func siteChannelRig(t *testing.T) *siteTestRig {
	t.Helper()
	rig := newSiteTestRig(t)
	rig.ctx.cpConfig.ClusterConfig = &ClusterSpec{}
	spec := rig.registry.ComputeChannels["rows.out"].Config
	columns := rig.registry.ComputeChannels["rows.out"].Columns
	for _, name := range []string{"rows.a", "rows.b"} {
		rig.registry.ComputeChannels[name] = &Channel{
			Name: name, Channel: make(chan []any, 8), Columns: columns, Config: spec}
	}
	rows := make(chan []any)
	close(rows)
	rig.source.Channel = rows
	return rig
}

// The step under test: one site operator writing its own output channel and the
// two it declares. Built from the document's shape rather than from a factory,
// because what closes a channel is the *declaration* and no executor consults
// the registry of operators to decide.
func siteChannelStep(declare bool) *PipeSpec {
	step := TransformationSpec{
		Type:          "site_double",
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
		SiteConfig:    &SiteOperatorSpec{},
	}
	if declare {
		step.SiteConfig.OutputChannels = []OutputChannelConfig{
			{Name: "rows.a", SpecName: "rows"},
			{Name: "rows.b", SpecName: "rows"},
		}
	}
	return &PipeSpec{Type: "fan_out", Apply: []TransformationSpec{step}}
}

func TestFanOutClosesASiteStepsDeclaredChannels(t *testing.T) {
	for _, tc := range []struct {
		name    string
		declare bool
		want    []string
	}{
		// The negative is asserted beside the positive rather than assumed: a
		// document declaring no output_channels must take exactly the path it
		// took before this arm existed, which is the step's own channel and
		// nothing else. Without it, an arm that closed every channel in the
		// registry would satisfy the positive case.
		{"declaring none", false, []string{"rows.out"}},
		{"declaring two", true, []string{"rows.a", "rows.b", "rows.out"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := siteChannelRig(t)
			resultCh := make(chan ComputePipesResult, 4)
			rig.ctx.StartFanOutPipe(siteChannelStep(tc.declare), rig.source, resultCh)
			got := sortedKeys(rig.registry.ClosedChannels)
			if !slices.Equal(got, tc.want) {
				t.Errorf("closed %v, want %v", got, tc.want)
			}
			// Closed in the registry's bookkeeping is not the same as closed: a
			// reader blocks on the channel, not on the map.
			for _, name := range tc.want {
				select {
				case _, open := <-rig.registry.ComputeChannels[name].Channel:
					if open {
						t.Errorf("%s yielded a record; the rig sends none", name)
					}
				default:
					t.Errorf("%s is not closed: a reader of it would block forever", name)
				}
			}
		})
	}
}

func TestSplitterClosesASiteStepsDeclaredChannels(t *testing.T) {
	for _, tc := range []struct {
		name    string
		declare bool
		want    []string
	}{
		{"declaring none", false, []string{"rows.out"}},
		{"declaring two", true, []string{"rows.a", "rows.b", "rows.out"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := siteChannelRig(t)
			spec := siteChannelStep(tc.declare)
			spec.Type = "splitter"
			// The smallest splitter_config that is not refused. Which key it
			// splits on is immaterial here -- the source is empty, so no channel
			// handler is ever started and what is under test is the deferred
			// close pass.
			spec.SplitterConfig = &SplitterSpec{DefaultSplitterValue: "all"}
			resultCh := make(chan ComputePipesResult, 4)
			rig.ctx.StartSplitterPipe(spec, rig.source, resultCh)
			select {
			case err := <-rig.ctx.errCh:
				t.Fatalf("the splitter reported an error: %v", err)
			default:
			}
			got := sortedKeys(rig.registry.ClosedChannels)
			if !slices.Equal(got, tc.want) {
				t.Errorf("closed %v, want %v", got, tc.want)
			}
		})
	}
}

// The guard on the same three lines P9-I15 reports unguarded in the splitter:
// `clustering` and `map_record` are dereferenced there without a nil check,
// where fan_out checks. **That panic is out of scope and is deliberately not
// repaired here**; this asserts only that the site arm did not extend it -- a
// built-in carrying a site_config is answered nil by siteOutputChannelConfigs,
// so the new arm dereferences nothing a document can make nil.
func TestTheSiteCloseArmDereferencesNothingOnABuiltin(t *testing.T) {
	for _, token := range []string{"map_record", "clustering", "jetrules"} {
		spec := &TransformationSpec{Type: token,
			SiteConfig: &SiteOperatorSpec{OutputChannels: []OutputChannelConfig{
				{Name: "rows.a", SpecName: "rows"}}}}
		if got := siteOutputChannelConfigs(spec); got != nil {
			t.Errorf("%s: siteOutputChannelConfigs answered %d channel(s) for a "+
				"built-in; the close pass would register channels for a step the "+
				"dispatch never builds", token, len(got))
		}
	}
}
