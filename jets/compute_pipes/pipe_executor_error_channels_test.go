package compute_pipes

import (
	"slices"
	"testing"
)

// Both executors close every operator's error channel, whichever way it came to
// exist: named by the author in the pipeline config, or synthesised by
// SynthesizeDefaultErrorChannels when the author named none. And they close
// nothing extra when there is none.
//
// Found 2026-10-11 running Patient_Profile_Template: the close pass was a
// per-type list naming no `render` and no site operator, so a render step's
// error channel -- synthesised, since that pipeline names none -- was closed by
// nothing, and the process_errors writer it feeds waited for an EOF that never
// came. The run hung after every other channel had closed. The executors now
// read errorChannelConfig, the one function the synthesis and the channel
// registry already read.

// errorChannelStep is one step of the given operator, writing rows.out, with an
// error channel or without one. Built from the document's shape: what closes a
// channel is the declaration, and the source is empty, so whether the operator
// builds is not what is under test.
func errorChannelStep(operator string, withErrorChannel bool) *PipeSpec {
	var ec *OutputChannelConfig
	if withErrorChannel {
		ec = &OutputChannelConfig{Name: "rows.errors", SpecName: "rows"}
	}
	step := TransformationSpec{
		Type:          operator,
		OutputChannel: OutputChannelConfig{Name: "rows.out", SpecName: "rows"},
	}
	switch operator {
	case RenderOperatorType:
		step.RenderConfig = &RenderSpec{ErrorChannel: ec}
	case "map_record":
		step.MapRecordConfig = &MapRecordSpec{ErrorChannel: ec}
	default:
		step.SiteConfig = &SiteOperatorSpec{ErrorChannel: ec}
	}
	return &PipeSpec{Type: "fan_out", Apply: []TransformationSpec{step}}
}

var errorChannelCases = []struct {
	name     string
	operator string
	withEC   bool
	want     []string
}{
	{"render with an error channel", RenderOperatorType, true, []string{"rows.errors", "rows.out"}},
	{"render without one", RenderOperatorType, false, []string{"rows.out"}},
	{"a site operator with an error channel", "site_double", true, []string{"rows.errors", "rows.out"}},
	{"a site operator without one", "site_double", false, []string{"rows.out"}},
	// The operators the old list named, so the move to errorChannelConfig is
	// shown to keep them rather than assumed to.
	{"map_record with an error channel", "map_record", true, []string{"rows.errors", "rows.out"}},
	{"map_record without one", "map_record", false, []string{"rows.out"}},
}

func TestFanOutClosesEveryOperatorsErrorChannel(t *testing.T) {
	for _, tc := range errorChannelCases {
		t.Run(tc.name, func(t *testing.T) {
			rig := siteChannelRig(t)
			resultCh := make(chan ComputePipesResult, 4)
			rig.ctx.StartFanOutPipe(errorChannelStep(tc.operator, tc.withEC), rig.source, resultCh)
			got := sortedKeys(rig.registry.ClosedChannels)
			if !slices.Equal(got, tc.want) {
				t.Errorf("closed %v, want %v", got, tc.want)
			}
			// Closed in the bookkeeping is not closed: a reader blocks on the
			// channel itself.
			if tc.withEC {
				select {
				case <-rig.registry.ComputeChannels["rows.errors"].Channel:
				default:
					t.Error("rows.errors is not closed: its writer would wait forever")
				}
			}
		})
	}
}

func TestSplitterClosesEveryOperatorsErrorChannel(t *testing.T) {
	for _, tc := range errorChannelCases {
		t.Run(tc.name, func(t *testing.T) {
			rig := siteChannelRig(t)
			spec := errorChannelStep(tc.operator, tc.withEC)
			spec.Type = "splitter"
			spec.SplitterConfig = &SplitterSpec{DefaultSplitterValue: "all"}
			resultCh := make(chan ComputePipesResult, 4)
			rig.ctx.StartSplitterPipe(spec, rig.source, resultCh)
			got := sortedKeys(rig.registry.ClosedChannels)
			if !slices.Equal(got, tc.want) {
				t.Errorf("closed %v, want %v", got, tc.want)
			}
		})
	}
}
