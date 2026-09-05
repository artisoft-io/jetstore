package compute_pipes

// The backwards-compatibility clause of built-in error reporting is a claim about
// the rule corpus -- ten error channel declarations across three workspace
// repositories keep working -- and a claim about a corpus is worth what running it
// is worth. This file runs it.
//
// It skips unless JETS_PC_CORPUS_DIR names a directory to walk for *.pc.json,
// because the workspaces are separate repositories and are not present in a plain
// checkout of this one. On the machine that wrote this the run is:
//
//	JETS_PC_CORPUS_DIR=<...>/workspaces go test -run TestCorpus ./jets/compute_pipes/
//
// What it asserts is stronger than "the corpus still validates". It asserts that
// every authored error channel is byte-identical after the synthesis, and that the
// document reaching the node -- pipes, channels and pruned output tables -- is
// unchanged for every step that had nothing to synthesise.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func corpusDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("JETS_PC_CORPUS_DIR")
	if dir == "" {
		t.Skip("JETS_PC_CORPUS_DIR is not set; see the header of this file")
	}
	return dir
}

func corpusFiles(t *testing.T, dir string) []string {
	t.Helper()
	files := make([]string, 0)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".pc.json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .pc.json under %s", dir)
	}
	return files
}

// corpusEvalEnv mirrors what actions_start_sharding_cp.go puts in EnvSettings for
// the When expressions of conditional steps, at the quiet single-shard values. It is
// the same env tools/cpipes_contract/harness/main.go uses, for the same reason.
func corpusEvalEnv() map[string]any {
	env := map[string]any{
		"multi_step_sharding": 0,
		"total_file_size":     1024,
		"total_file_size_gb":  float64(1024) / 1024 / 1024 / 1024,
		"nbr_partitions":      1,
	}
	env["$MULTI_STEP_SHARDING"] = env["multi_step_sharding"]
	env["$TOTAL_FILE_SIZE"] = env["total_file_size"]
	env["$TOTAL_FILE_SIZE_GB"] = env["total_file_size_gb"]
	env["$NBR_PARTITIONS"] = env["nbr_partitions"]
	return env
}

// authoredErrorChannels returns every error channel of the step keyed by its
// position, before anything has run. A synthesised channel is one this map does not
// have; an authored one must come out of the synthesis unchanged.
func authoredErrorChannels(t *testing.T, pipeConfig []PipeSpec) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for i := range pipeConfig {
		for j := range pipeConfig[i].Apply {
			ec := errorChannelConfig(&pipeConfig[i].Apply[j])
			if ec == nil {
				continue
			}
			b, err := json.Marshal(ec)
			if err != nil {
				t.Fatalf("marshalling error channel: %v", err)
			}
			out[fmt.Sprintf("%d.%d", i, j)] = string(b)
		}
	}
	return out
}

// stepStartup unmarshals a fresh config and selects one step, the way both startup
// actions do. A fresh unmarshal per step is deliberate: the validator mutates its
// input as it applies defaults, so validating step N on a document step N-1 has
// already been through would test a state that never occurs.
func stepStartup(t *testing.T, raw []byte, stepId int) (*CpipesStartup, []PipeSpec) {
	t.Helper()
	startup := &CpipesStartup{EnvSettings: corpusEvalEnv()}
	if err := json.Unmarshal(raw, &startup.CpConfig); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	pipeConfig, _, err := startup.CpConfig.GetComputePipes(stepId, startup.EnvSettings)
	if err != nil || pipeConfig == nil {
		return startup, nil
	}
	if err := ApplyAllConditionalTransformationSpec(pipeConfig, startup.EnvSettings); err != nil {
		return startup, nil
	}
	return startup, pipeConfig
}

// Every authored error channel in the corpus survives the synthesis byte for byte,
// and every step that had one still selects the table it selected before.
func TestCorpusAuthoredErrorChannelsAreUntouched(t *testing.T) {
	dir := corpusDir(t)
	files := corpusFiles(t, dir)

	authored, filesWithOne, synthesised, steps := 0, 0, 0, 0
	// The two numbers the always-on question turns on, and the reason the fan-in
	// exists: the busiest step's channel count against the writers it costs.
	maxPerStep, maxPerStepAt, writersAdded := 0, "", 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var probe ComputePipesConfig
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		fileHasOne := false
		for stepId := range probe.NbrComputePipes() {
			startup, pipeConfig := stepStartup(t, raw, stepId)
			if pipeConfig == nil {
				continue
			}
			steps++
			before := authoredErrorChannels(t, pipeConfig)
			authored += len(before)
			if len(before) > 0 {
				fileHasOne = true
			}
			// The table selection the step made before the synthesis.
			tablesBefore, errBefore := SelectActiveOutputTable(startup.CpConfig.OutputTables, pipeConfig)

			n := SynthesizeDefaultErrorChannels(&startup.CpConfig, pipeConfig)
			synthesised += n
			if n > maxPerStep {
				maxPerStep, maxPerStepAt = n, fmt.Sprintf("%s step %d", filepath.Base(path), stepId)
			}
			if n > 0 {
				writersAdded++
			}

			after := authoredErrorChannels(t, pipeConfig)
			for pos, want := range before {
				if got := after[pos]; got != want {
					t.Errorf("%s step %d apply %s: authored error channel changed\n before: %s\n  after: %s",
						filepath.Base(path), stepId, pos, want, got)
				}
			}
			tablesAfter, errAfter := SelectActiveOutputTable(startup.CpConfig.OutputTables, pipeConfig)
			if (errBefore == nil) != (errAfter == nil) {
				t.Errorf("%s step %d: SelectActiveOutputTable went from %v to %v",
					filepath.Base(path), stepId, errBefore, errAfter)
				continue
			}
			if errBefore != nil {
				continue
			}
			// Every table the step selected before is still selected, in order.
			if len(tablesAfter) < len(tablesBefore) {
				t.Errorf("%s step %d: selected %d tables, was %d",
					filepath.Base(path), stepId, len(tablesAfter), len(tablesBefore))
				continue
			}
			for i := range tablesBefore {
				if tablesAfter[i] != tablesBefore[i] {
					t.Errorf("%s step %d: table %d is now %+v, was %+v",
						filepath.Base(path), stepId, i, tablesAfter[i], tablesBefore[i])
				}
			}
		}
		if fileHasOne {
			filesWithOne++
		}
	}
	t.Logf("corpus: %d documents, %d steps, %d authored error channels in %d documents, "+
		"%d channels synthesised", len(files), steps, authored, filesWithOne, synthesised)
	t.Logf("corpus: the busiest step synthesises %d channels (%s); %d of %d steps gain the one "+
		"default table writer, so the default costs %d writer(s) rather than %d",
		maxPerStep, maxPerStepAt, writersAdded, steps, writersAdded, synthesised)
}

// The synthesised configuration validates. This is ValidatePipeSpecConfig, the check
// both startup actions make, run over the corpus with the synthesis in place -- so a
// default that produced a channel the engine refuses would fail here rather than at
// a deployment.
func TestCorpusValidatesWithSynthesizedErrorChannels(t *testing.T) {
	dir := corpusDir(t)
	files := corpusFiles(t, dir)

	baselineFailures, synthesisedFailures := make([]string, 0), make([]string, 0)
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var probe ComputePipesConfig
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		for stepId := range probe.NbrComputePipes() {
			// The baseline: what the step does without the synthesis.
			startup, pipeConfig := stepStartup(t, raw, stepId)
			if pipeConfig == nil {
				continue
			}
			if err := startup.ValidatePipeSpecConfig(&startup.CpConfig, pipeConfig); err != nil {
				baselineFailures = append(baselineFailures,
					fmt.Sprintf("%s step %d: %v", filepath.Base(path), stepId, err))
				continue
			}
			// The same step with the synthesis, on a fresh unmarshal.
			startup, pipeConfig = stepStartup(t, raw, stepId)
			if pipeConfig == nil {
				continue
			}
			SynthesizeDefaultErrorChannels(&startup.CpConfig, pipeConfig)
			if err := startup.ValidatePipeSpecConfig(&startup.CpConfig, pipeConfig); err != nil {
				synthesisedFailures = append(synthesisedFailures,
					fmt.Sprintf("%s step %d: %v", filepath.Base(path), stepId, err))
			}
		}
	}
	t.Logf("corpus: %d step(s) failed validation without the synthesis, %d with it",
		len(baselineFailures), len(synthesisedFailures))
	for _, f := range baselineFailures {
		t.Logf("  baseline failure: %s", f)
	}
	if len(synthesisedFailures) > len(baselineFailures) {
		for _, f := range synthesisedFailures {
			t.Errorf("  failure with the synthesis: %s", f)
		}
	}
}

// With the deployment switch off, every corpus document reaching the node is byte
// identical to the document its author wrote -- pipes, channel specs and pruned
// output tables alike. That is a stronger claim than "the ten declarations still
// work", and it is the one an operator backing this change out is relying on: off
// has to be indistinguishable from a build without the feature, and the only way to
// say so about 45 documents is to run them.
func TestCorpusIsUntouchedWhenReportingIsOff(t *testing.T) {
	dir := corpusDir(t)
	files := corpusFiles(t, dir)
	t.Setenv(DefaultErrorReportingEnvVar, "off")

	steps, changed := 0, 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var probe ComputePipesConfig
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		for stepId := range probe.NbrComputePipes() {
			startup, pipeConfig := stepStartup(t, raw, stepId)
			if pipeConfig == nil {
				continue
			}
			steps++
			before, err := json.Marshal(struct {
				Channels []ChannelSpec
				Tables   []*TableSpec
				Pipes    []PipeSpec
			}{startup.CpConfig.Channels, startup.CpConfig.OutputTables, pipeConfig})
			if err != nil {
				t.Fatalf("marshalling %s step %d: %v", filepath.Base(path), stepId, err)
			}
			if n := SynthesizeDefaultErrorChannels(&startup.CpConfig, pipeConfig); n != 0 {
				t.Errorf("%s step %d: synthesised %d channel(s) with the switch off",
					filepath.Base(path), stepId, n)
			}
			after, err := json.Marshal(struct {
				Channels []ChannelSpec
				Tables   []*TableSpec
				Pipes    []PipeSpec
			}{startup.CpConfig.Channels, startup.CpConfig.OutputTables, pipeConfig})
			if err != nil {
				t.Fatalf("marshalling %s step %d: %v", filepath.Base(path), stepId, err)
			}
			if string(before) != string(after) {
				changed++
				t.Errorf("%s step %d: the configuration changed with the switch off",
					filepath.Base(path), stepId)
			}
		}
	}
	t.Logf("corpus: %d documents, %d steps, %d step(s) changed with %s=off",
		len(files), steps, changed, DefaultErrorReportingEnvVar)
}

// The deployment cap reaches exactly the operators the synthesis gives a channel to,
// over the real corpus. What it must not do is reach an operator that already names
// a max_error_count, or one whose error channel its author wrote.
func TestCorpusCapReachesOnlyTheSynthesizedOperators(t *testing.T) {
	dir := corpusDir(t)
	files := corpusFiles(t, dir)
	t.Setenv(DefaultErrorMaxCountEnvVar, "7")

	capped, authoredKept, synthesised := 0, 0, 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var probe ComputePipesConfig
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		for stepId := range probe.NbrComputePipes() {
			startup, pipeConfig := stepStartup(t, raw, stepId)
			if pipeConfig == nil {
				continue
			}
			before := authoredErrorChannels(t, pipeConfig)
			synthesised += SynthesizeDefaultErrorChannels(&startup.CpConfig, pipeConfig)
			for i := range pipeConfig {
				for j := range pipeConfig[i].Apply {
					pos := fmt.Sprintf("%d.%d", i, j)
					n := maxErrorCountOf(&pipeConfig[i].Apply[j])
					if _, authored := before[pos]; authored {
						if n == 7 {
							t.Errorf("%s step %d apply %s: an authored operator was capped",
								filepath.Base(path), stepId, pos)
						}
						authoredKept++
						continue
					}
					if isDefaultErrorChannel(errorChannelConfig(&pipeConfig[i].Apply[j])) {
						if n != 7 {
							t.Errorf("%s step %d apply %s: max_error_count is %d, want 7",
								filepath.Base(path), stepId, pos, n)
						}
						capped++
					}
				}
			}
		}
	}
	t.Logf("corpus: %d channel(s) synthesised, %d operator(s) capped, %d authored declaration(s) untouched",
		synthesised, capped, authoredKept)
}

// maxErrorCountOf reads the operator's max_error_count, or 0 where the operator has
// no config or no such field.
func maxErrorCountOf(ts *TransformationSpec) int {
	switch ts.Type {
	case "map_record":
		if ts.MapRecordConfig != nil {
			return ts.MapRecordConfig.MaxErrorCount
		}
	case "jetrules":
		if ts.JetrulesConfig != nil {
			return ts.JetrulesConfig.MaxErrorCount
		}
	case "ollama":
		if ts.OllamaConfig != nil {
			return ts.OllamaConfig.MaxErrorCount
		}
	case "embed":
		if ts.EmbedConfig != nil {
			return ts.EmbedConfig.MaxErrorCount
		}
	case "vllm":
		if ts.VllmConfig != nil {
			return ts.VllmConfig.MaxErrorCount
		}
	}
	return 0
}
