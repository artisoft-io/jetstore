package compute_pipes

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestMissingDiscriminatorColumns(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		want    []string
	}{
		{
			name:    "a spec written before the widening",
			columns: legacyProcessErrorColumns,
			want:    []string{"cpipes_step_id", "error_channel", "operator_type"},
		},
		{
			name:    "a spec that declares all three",
			columns: append(append([]string{}, legacyProcessErrorColumns...), ProcessErrorDiscriminatorColumns...),
			want:    nil,
		},
		{
			name:    "a spec that declares some of them",
			columns: append(append([]string{}, legacyProcessErrorColumns...), "operator_type"),
			want:    []string{"cpipes_step_id", "error_channel"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := missingDiscriminatorColumns(&ChannelSpec{Name: "process_errors", Columns: tc.columns})
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// pipeConfigWithErrorChannel is one step applying one jetrules operator that
// reports to an error channel named after the given spec.
func pipeConfigWithErrorChannel(specName string) []PipeSpec {
	return []PipeSpec{{
		Apply: []TransformationSpec{{
			Type: "jetrules",
			JetrulesConfig: &JetrulesSpec{
				ErrorChannel: &OutputChannelConfig{Name: "process_errors.out", SpecName: specName},
			},
		}},
	}}
}

// The check warns and does not fail, because ten live configurations are in exactly
// this state and refusing them would trade a diagnostic they never had for a run
// they depend on.
func TestWarnMissingErrorChannelDiscriminators(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	cpConfig := &ComputePipesConfig{
		Channels: []ChannelSpec{{Name: "process_errors", Columns: legacyProcessErrorColumns}},
	}
	pipeConfig := pipeConfigWithErrorChannel("process_errors")
	if err := validateErrorChannelSpecs(cpConfig, pipeConfig); err != nil {
		t.Fatalf("the check must not fail the configuration: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"process_errors.out", "jetrules", "cpipes_step_id", "operator_type"} {
		if !strings.Contains(out, want) {
			t.Errorf("the warning should name %q, got: %s", want, out)
		}
	}
}

func TestNoWarningWhenTheSpecDeclaresThem(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	cpConfig := &ComputePipesConfig{
		Channels: []ChannelSpec{{
			Name:    "process_errors",
			Columns: append(append([]string{}, legacyProcessErrorColumns...), ProcessErrorDiscriminatorColumns...),
		}},
	}
	if err := validateErrorChannelSpecs(cpConfig, pipeConfigWithErrorChannel("process_errors")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "error channel") {
		t.Errorf("expected no warning, got: %s", buf.String())
	}
}

// An error channel that names no spec falls back to the channel's own name, and a
// spec that cannot be found is skipped rather than reported: whether a named spec
// exists is validateOutputChConfig's question, already asked.
func TestUnknownSpecIsSkipped(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	cpConfig := &ComputePipesConfig{Channels: []ChannelSpec{}}
	if err := validateErrorChannelSpecs(cpConfig, pipeConfigWithErrorChannel("")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "error channel") {
		t.Errorf("expected no warning for an unknown spec, got: %s", buf.String())
	}
}

// The single-writer rule still fails, and still first: the warning must not
// displace the error validateErrorChannels exists to raise.
func TestSingleWriterRuleStillFails(t *testing.T) {
	cpConfig := &ComputePipesConfig{
		Channels: []ChannelSpec{{Name: "process_errors", Columns: legacyProcessErrorColumns}},
	}
	pipeConfig := []PipeSpec{{
		Apply: []TransformationSpec{
			{
				Type: "jetrules",
				JetrulesConfig: &JetrulesSpec{
					ErrorChannel: &OutputChannelConfig{Name: "shared.out", SpecName: "process_errors"},
				},
			},
			{
				Type: "map_record",
				MapRecordConfig: &MapRecordSpec{
					ErrorChannel: &OutputChannelConfig{Name: "shared.out", SpecName: "process_errors"},
				},
			},
		},
	}}
	err := validateErrorChannelSpecs(cpConfig, pipeConfig)
	if err == nil || !strings.Contains(err.Error(), "must have its own error channel") {
		t.Errorf("expected the single-writer error, got: %v", err)
	}
}
