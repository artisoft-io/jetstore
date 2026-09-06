package compute_pipes

import "testing"

// A conditional_config `then` carrying a type replaces the host operator outright,
// where a type-less one merges fields. TestApplyAllConditionalTransformationSpec
// covers the merge half only, and the merge half cannot reach an infer operator at
// all: MergeTransformationSpec copies no OllamaConfig, VllmConfig or EmbedConfig.
// So swapping an infer backend is possible only through replacement, and this is
// the test for it.
func TestConditionalReplacesOllamaWithVllm(t *testing.T) {
	build := func() []PipeSpec {
		return []PipeSpec{{Apply: []TransformationSpec{{
			Type: "ollama",
			OllamaConfig: &OllamaSpec{
				Model: "granite4.1:3b",
				InferCommonSpec: InferCommonSpec{
					PromptTemplateName:   "patient_briefing",
					ProvenanceSchemaName: "patient_briefing",
				},
			},
			OutputChannel: OutputChannelConfig{Name: "Briefing.out"},
			ConditionalConfig: []*ConditionalTransformationSpec{{
				When: ExpressionNode{
					Lhs: &ExpressionNode{Type: "select", Expr: "$INFER_BACKEND"},
					Op:  "==",
					Rhs: &ExpressionNode{Type: "value", Expr: "'vllm'"},
				},
				Then: TransformationSpec{
					Type: "vllm",
					VllmConfig: &VllmSpec{
						Model: "granite4.1:3b",
						InferCommonSpec: InferCommonSpec{
							PromptTemplateName:   "patient_briefing",
							ProvenanceSchemaName: "patient_briefing",
						},
					},
					OutputChannel: OutputChannelConfig{Name: "Briefing.out"},
				},
			}},
		}}}}
	}

	for _, tc := range []struct {
		name, backend, wantType string
	}{
		{"set to vllm replaces the operator", "vllm", "vllm"},
		{"unset leaves the host in place", "", "ollama"},
		{"set to ollama leaves the host in place", "ollama", "ollama"},
		{"an unrelated value leaves the host in place", "sglang", "ollama"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pipeConfig := build()
			env := map[string]any{"$INFER_BACKEND": tc.backend}
			if err := ApplyAllConditionalTransformationSpec(pipeConfig, env); err != nil {
				t.Fatalf("ApplyAllConditionalTransformationSpec: %v", err)
			}
			got := pipeConfig[0].Apply[0]
			if got.Type != tc.wantType {
				t.Fatalf("got operator type %q, want %q", got.Type, tc.wantType)
			}
			// The guardrail must survive the swap. It does because both operators
			// embed InferCommonSpec rather than declaring parallel copies.
			var provenance, model string
			switch {
			case got.VllmConfig != nil:
				provenance, model = got.VllmConfig.ProvenanceSchemaName, got.VllmConfig.Model
			case got.OllamaConfig != nil:
				provenance, model = got.OllamaConfig.ProvenanceSchemaName, got.OllamaConfig.Model
			default:
				t.Fatal("neither ollama_config nor vllm_config is set")
			}
			if provenance != "patient_briefing" {
				t.Errorf("provenance_schema_name = %q, want it carried across the swap", provenance)
			}
			if model != "granite4.1:3b" {
				t.Errorf("model = %q, want it carried across the swap", model)
			}
			if got.OutputChannel.Name != "Briefing.out" {
				t.Errorf("output_channel = %q, want it carried across the swap", got.OutputChannel.Name)
			}
			if tc.wantType == "vllm" && got.OllamaConfig != nil {
				t.Error("replacement left the host's ollama_config behind")
			}
		})
	}
}

// An absent key, rather than an empty value, is what a run started before
// INFER_BACKEND was plumbed replays out of cpipes_startup_json. It must not error.
func TestConditionalInferBackendAbsentFromEnv(t *testing.T) {
	pipeConfig := []PipeSpec{{Apply: []TransformationSpec{{
		Type:         "ollama",
		OllamaConfig: &OllamaSpec{Model: "granite4.1:3b"},
		ConditionalConfig: []*ConditionalTransformationSpec{{
			When: ExpressionNode{
				Lhs: &ExpressionNode{Type: "select", Expr: "$INFER_BACKEND"},
				Op:  "==",
				Rhs: &ExpressionNode{Type: "value", Expr: "'vllm'"},
			},
			Then: TransformationSpec{Type: "vllm", VllmConfig: &VllmSpec{Model: "granite4.1:3b"}},
		}},
	}}}}
	if err := ApplyAllConditionalTransformationSpec(pipeConfig, map[string]any{}); err != nil {
		t.Fatalf("an absent $INFER_BACKEND must not be an error: %v", err)
	}
	if got := pipeConfig[0].Apply[0].Type; got != "ollama" {
		t.Fatalf("got %q, want the host operator to remain", got)
	}
}
