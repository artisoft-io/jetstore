package compute_pipes

import "testing"

func inferStep(c *InferSpec) []PipeSpec {
	return []PipeSpec{{Apply: []TransformationSpec{
		{Type: "map_record", MapRecordConfig: &MapRecordSpec{}},
		{Type: "infer", InferConfig: c, OutputChannel: OutputChannelConfig{Name: "Briefing.out"}},
	}}}
}

func commonSpec() InferCommonSpec {
	return InferCommonSpec{
		PromptTemplateName:   "patient_briefing",
		ProvenanceSchemaName: "patient_briefing",
		PoolSize:             4,
		OutputMapping:        []InferMappingSpec{{Column: "cintel:Briefing_Document", Source: "raw_response"}},
		ErrorChannel:         &OutputChannelConfig{Name: "infer_errors.out"},
	}
}

func TestResolveInferBackendSelectsTheOperator(t *testing.T) {
	yes := true
	for _, tc := range []struct {
		name, backend string
		env           map[string]any
		wantType      string
	}{
		{"an env var naming vllm", "$INFER_BACKEND", map[string]any{"$INFER_BACKEND": "vllm"}, "vllm"},
		{"an env var naming ollama", "$INFER_BACKEND", map[string]any{"$INFER_BACKEND": "ollama"}, "ollama"},
		{"an env var set to empty", "$INFER_BACKEND", map[string]any{"$INFER_BACKEND": ""}, "ollama"},
		{"an env var absent from env", "$INFER_BACKEND", map[string]any{}, "ollama"},
		{"a literal backend", "vllm", nil, "vllm"},
		{"no backend at all", "", nil, "ollama"},
		{"case and space are not significant", "  VLLM ", nil, "vllm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pipeConfig := inferStep(&InferSpec{
				Backend: tc.backend, Model: "granite4.1:3b", Think: &yes,
				StructuredOutput: "json_schema", InferCommonSpec: commonSpec(),
			})
			if err := ResolveInferBackend(pipeConfig, tc.env); err != nil {
				t.Fatalf("ResolveInferBackend: %v", err)
			}
			got := pipeConfig[0].Apply[1]
			if got.Type != tc.wantType {
				t.Fatalf("got type %q, want %q", got.Type, tc.wantType)
			}
			if got.InferConfig != nil {
				t.Error("infer_config survived resolution; the document would have two states that can disagree")
			}
			// The shared configuration is carried, not re-declared.
			var common InferCommonSpec
			switch tc.wantType {
			case "ollama":
				if got.OllamaConfig == nil {
					t.Fatal("no ollama_config")
				}
				if got.VllmConfig != nil {
					t.Error("vllm_config was also set")
				}
				if got.OllamaConfig.Think == nil {
					t.Error("think is ollama's and was dropped")
				}
				common = got.OllamaConfig.InferCommonSpec
			case "vllm":
				if got.VllmConfig == nil {
					t.Fatal("no vllm_config")
				}
				if got.OllamaConfig != nil {
					t.Error("ollama_config was also set")
				}
				if got.VllmConfig.StructuredOutput != "json_schema" {
					t.Error("structured_output is vllm's and was dropped")
				}
				common = got.VllmConfig.InferCommonSpec
			}
			if common.ProvenanceSchemaName != "patient_briefing" {
				t.Error("provenance_schema_name did not survive resolution")
			}
			if len(common.OutputMapping) != 1 {
				t.Error("output_mapping did not survive resolution")
			}
			if common.ErrorChannel == nil {
				t.Error("error_channel did not survive resolution")
			}
			// A step that is not an infer step is untouched.
			if pipeConfig[0].Apply[0].Type != "map_record" {
				t.Error("a neighbouring operator was rewritten")
			}
		})
	}
}

func TestResolveInferBackendRejectsWhatItCannotResolve(t *testing.T) {
	t.Run("an unknown backend fails loudly", func(t *testing.T) {
		pipeConfig := inferStep(&InferSpec{Backend: "sglang", Model: "m"})
		err := ResolveInferBackend(pipeConfig, nil)
		if err == nil {
			t.Fatal("want an error naming the unknown backend")
		}
		t.Log(err)
	})
	t.Run("infer without infer_config fails loudly", func(t *testing.T) {
		pipeConfig := []PipeSpec{{Apply: []TransformationSpec{{Type: "infer"}}}}
		if err := ResolveInferBackend(pipeConfig, nil); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("an env var naming an unknown backend fails loudly", func(t *testing.T) {
		pipeConfig := inferStep(&InferSpec{Backend: "$INFER_BACKEND", Model: "m"})
		err := ResolveInferBackend(pipeConfig, map[string]any{"$INFER_BACKEND": "vLLM2"})
		if err == nil {
			t.Fatal("want an error: a misspelt backend must not silently become ollama")
		}
	})
}

// A config carrying no infer step at all must be left exactly as it was, since the
// pass runs over every pipeline whether or not it uses the abstraction.
func TestResolveInferBackendLeavesOtherConfigsAlone(t *testing.T) {
	pipeConfig := []PipeSpec{{Apply: []TransformationSpec{
		{Type: "ollama", OllamaConfig: &OllamaSpec{Model: "granite4.1:3b"}},
		{Type: "vllm", VllmConfig: &VllmSpec{Model: "granite4.1:3b"}},
	}}}
	if err := ResolveInferBackend(pipeConfig, nil); err != nil {
		t.Fatalf("ResolveInferBackend: %v", err)
	}
	if pipeConfig[0].Apply[0].Type != "ollama" || pipeConfig[0].Apply[1].Type != "vllm" {
		t.Fatal("an already-concrete operator was rewritten")
	}
}
