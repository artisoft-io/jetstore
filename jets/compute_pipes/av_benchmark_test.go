package compute_pipes

// The `AV` backend benchmark: latency, throughput and cold start, over the
// evaluation population, through the infer operator's own worker pool
// (agentic_ai Phase 7 `AV.1` and `AV.2`).
//
// # It drives the operator rather than the server
//
// A benchmark that posts to `/v1/chat/completions` in a loop measures the
// server. What `AV` reports has to be the deployed path, so this runs the real
// `infer` operator: its worker pool, its retries, its timeouts, its output
// mapping. **`pool_size` therefore comes from `patient_profile.pc.json`** and is
// not a flag of this file - which is the whole point, because `pool_size: 4` is
// a property of the pipeline rather than of the experiment.
//
// Everything else comes from the document too: the system prompt, the prompt
// template (resolved by name out of `prompt_templates`, as the operator does),
// the model, and the sampling `options` that `I-575` added. A benchmark that
// restated any of them would drift from the pipeline the moment the pipeline
// changed, which is exactly what the `AT.5` harness did on the day the operator
// gained `options` (`I-588`).
//
// # What it does not do, said plainly
//
// **It does not run a rule session, a loader or a writer.** The records carry a
// prompt captured by `TestCaptureTheEvaluationPromptsForAV`, and the channel is
// synthetic: the columns are exactly the one the template reads and the ones the
// output mapping writes, because the real `Briefing` channel is
// `direct_properties_only` and computes its columns from the rdf class at run
// time. **So a number here is the cost of the inference step and not of the
// pipeline**, and no figure taken from it should be reported as the latter.
//
// # Latency and throughput come from different runs, deliberately
//
// The operator hands a record to a worker and the worker returns it when the
// call is done, so what this file can observe is **completion times**, not
// service times. At `pool_size: 1` the gap between completions is the per-record
// latency; above it, it is not. So:
//
//   - **latency** is read off a `pool_size: 1` run;
//   - **throughput** is read off the configured pool, which is 4.
//
// **The operator's own summary line is the authoritative latency and token
// figure**, not this file's arithmetic. It logs `N records, N calls, N errors, N
// retries, avg latency Xms, prompt tokens X, eval tokens X` when the pipe
// finishes, measured inside the worker around the call itself. What this file
// adds is the wall clock and the completion profile, which the operator does not
// keep. Where the two are comparable they agree: 2996ms average against a p50 of
// 3.11s on the first pool-1 run.
//
// That split is not a workaround. `I-586` established that greedy decoding does
// not survive continuous batching, so an accuracy figure has to come from pool 1
// anyway - and this makes the latency figure come from the same place, which
// means one run answers both and the pool-4 run is left measuring the thing it
// is good for.
//
// # Warm-up is discarded and reported separately
//
// The first requests to a freshly started server are not steady state: weights
// page in, CUDA graphs are captured, allocators settle. A single "warm" figure
// hides that, and the two arms of `AV` will be measured minutes apart on a
// cluster that can host only one of them - one having served all day, the other
// seconds old. So a fixed warm-up pass runs first and is **discarded**, its
// first completion is reported on its own, and both arms get the same treatment.

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	avPromptColumn = "cintel:Briefing_Input"
	avChannelName  = "briefing"
)

type avPromptRecord struct {
	MemberID string `json:"member_id"`
	Toon     string `json:"toon"`
}

// avRun is one timed pass over the population.
type avRun struct {
	label       string
	poolSize    int
	records     int
	wall        time.Duration
	firstDone   time.Duration
	completions []time.Duration // offset from the start of the pass, one per record
	outputs     int
	errors      int
}

// TestAVBenchmark times the infer operator over the evaluation population.
//
// Gated on JETS_AV_PROMPTS_FILE and JETS_AV_URL so an ordinary run makes no
// network call. JETS_AV_BACKEND selects vllm (default) or ollama;
// JETS_AV_POOL overrides the pool size from the document, which is what the
// pool-1 latency pass needs; JETS_AV_WARMUP sets the warm-up pass size.
func TestAVBenchmark(t *testing.T) {
	promptsFile := os.Getenv("JETS_AV_PROMPTS_FILE")
	url := os.Getenv("JETS_AV_URL")
	if promptsFile == "" || url == "" {
		t.Skip("JETS_AV_PROMPTS_FILE and JETS_AV_URL are not set; see the header of this file")
	}
	configFile := os.Getenv("JETS_AV_CONFIG")
	if configFile == "" {
		t.Fatal("JETS_AV_CONFIG must name the patient_profile.pc.json the operator is configured by")
	}
	backend := os.Getenv("JETS_AV_BACKEND")
	if backend == "" {
		backend = "vllm"
	}

	prompts := avLoadPrompts(t, promptsFile)
	cpConfig, inferConfig := avLoadConfig(t, configFile)

	// The pool size the document sets, unless the caller is taking the latency
	// pass. Reported either way, so a number always says which pool produced it.
	poolSize := avCommonOf(t, backend, inferConfig).PoolSize
	if v := os.Getenv("JETS_AV_POOL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("JETS_AV_POOL must be a positive integer, got %q", v)
		}
		poolSize = n
	}
	warmup := len(prompts)
	if v := os.Getenv("JETS_AV_WARMUP"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			t.Fatalf("JETS_AV_WARMUP must be a non-negative integer, got %q", v)
		}
		warmup = n
	}

	t.Logf("backend=%s url=%s pool_size=%d prompts=%d warmup=%d", backend, url, poolSize, len(prompts), warmup)

	var warm *avRun
	if warmup > 0 {
		n := warmup
		if n > len(prompts) {
			n = len(prompts)
		}
		warm = avTimeOnePass(t, "warm-up (discarded)", backend, url, cpConfig, inferConfig, poolSize, prompts[:n])
		avReport(t, warm)
	}
	timed := avTimeOnePass(t, "timed", backend, url, cpConfig, inferConfig, poolSize, prompts)
	avReport(t, timed)
}

// avTimeOnePass runs the operator once over the given prompts and times it.
func avTimeOnePass(t *testing.T, label, backend, url string, cpConfig *ComputePipesConfig,
	inferConfig json.RawMessage, poolSize int, prompts []avPromptRecord) *avRun {
	t.Helper()

	// The channel carries the prompt column and the columns the output mapping
	// writes; see the header on why it is synthetic.
	common := avCommonOf(t, backend, inferConfig)
	columns := []string{avPromptColumn}
	for _, m := range common.OutputMapping {
		columns = append(columns, m.Column)
	}
	columnsMap := make(map[string]int, len(columns))
	for i, c := range columns {
		columnsMap[c] = i
	}
	channelSpec := &ChannelSpec{Name: avChannelName, Columns: columns, columnsMap: &columnsMap}

	outName := avChannelName + ".out"
	registry := &ChannelRegistry{
		ComputeChannels: map[string]*Channel{
			outName: {Name: outName, Channel: make(chan []any), Columns: &columnsMap, Config: channelSpec},
		},
		ClosedChannels: make(map[string]bool),
	}
	source := &InputChannel{Name: avChannelName + ".in", Columns: &columnsMap, Config: channelSpec}
	builderContext := &BuilderContext{
		cpConfig:        cpConfig,
		channelRegistry: registry,
		done:            make(chan struct{}),
		errCh:           make(chan error, 64),
		env:             make(map[string]any),
	}
	outputCh, err := registry.GetOutputChannel(outName)
	if err != nil {
		t.Fatal(err)
	}

	spec := avTransformationSpec(t, backend, url, inferConfig, poolSize)
	var pipe PipeTransformationEvaluator
	switch backend {
	case "vllm":
		pipe, err = builderContext.NewVllmTransformationPipe(source, outputCh, spec)
	case "ollama":
		pipe, err = builderContext.NewOllamaTransformationPipe(source, outputCh, spec)
	default:
		t.Fatalf("JETS_AV_BACKEND must be vllm or ollama, got %q", backend)
	}
	if err != nil {
		t.Fatalf("building the %s operator: %v", backend, err)
	}

	run := &avRun{label: label, poolSize: poolSize, records: len(prompts)}
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range registry.ComputeChannels[outName].Channel {
			run.completions = append(run.completions, time.Since(start))
			run.outputs++
		}
	}()

	for i := range prompts {
		record := make([]any, len(columns))
		record[0] = prompts[i].Toon
		if err := pipe.Apply(&record); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatalf("Done: %v", err)
	}
	pipe.Finally()
	run.wall = time.Since(start)
	registry.CloseChannel(outName)
	wg.Wait()

	close(builderContext.errCh)
	for range builderContext.errCh {
		run.errors++
	}
	if len(run.completions) > 0 {
		run.firstDone = run.completions[0]
	}
	return run
}

// avReport prints one pass. Every figure names the pool it came from, because a
// latency taken at pool 4 and one taken at pool 1 are different quantities.
func avReport(t *testing.T, r *avRun) {
	t.Helper()
	t.Logf("--- %s: %d records at pool_size %d", r.label, r.records, r.poolSize)
	t.Logf("    wall %.2fs, %d completed, %d pipeline errors", r.wall.Seconds(), r.outputs, r.errors)
	if r.outputs == 0 {
		return
	}
	t.Logf("    throughput %.3f records/s", float64(r.outputs)/r.wall.Seconds())
	t.Logf("    first completion at %.2fs", r.firstDone.Seconds())

	// Inter-completion gaps. At pool 1 this is the per-record latency; above it,
	// it is a service rate and is labelled as one.
	gaps := make([]time.Duration, 0, len(r.completions))
	prev := time.Duration(0)
	for _, c := range r.completions {
		gaps = append(gaps, c-prev)
		prev = c
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	kind := "inter-completion gap"
	if r.poolSize == 1 {
		kind = "per-record latency"
	}
	t.Logf("    %s: p50 %.2fs, p95 %.2fs, max %.2fs", kind,
		gaps[len(gaps)/2].Seconds(), gaps[(len(gaps)*95)/100].Seconds(), gaps[len(gaps)-1].Seconds())
}

func avLoadPrompts(t *testing.T, path string) []avPromptRecord {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var prompts []avPromptRecord
	if err := json.Unmarshal(b, &prompts); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(prompts) == 0 {
		t.Fatalf("%s carries no prompts", path)
	}
	return prompts
}

// avLoadConfig returns the whole cpipes config, so the operator resolves its
// prompt template by name exactly as it does in the pipeline, and the raw
// infer_config it is configured by.
func avLoadConfig(t *testing.T, path string) (*ComputePipesConfig, json.RawMessage) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	cpConfig := &ComputePipesConfig{}
	if err := json.Unmarshal(b, cpConfig); err != nil {
		t.Fatalf("parsing %s into a ComputePipesConfig: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	raw := avFindInferConfig(doc)
	if raw == nil {
		t.Fatalf("%s has no infer_config; the benchmark configures the operator from the document", path)
	}
	return cpConfig, raw
}

// avFindInferConfig walks the document for the infer step's config. It matches on
// the key rather than on the operator type, because ResolveInferBackend rewrites
// the type before anything else sees it and the document may carry either name.
func avFindInferConfig(n any) json.RawMessage {
	switch v := n.(type) {
	case map[string]any:
		if cfg, ok := v["infer_config"]; ok {
			if b, err := json.Marshal(cfg); err == nil {
				return b
			}
		}
		for k, c := range v {
			if k == "comment" {
				continue
			}
			if r := avFindInferConfig(c); r != nil {
				return r
			}
		}
	case []any:
		for _, c := range v {
			if r := avFindInferConfig(c); r != nil {
				return r
			}
		}
	}
	return nil
}

func avCommonOf(t *testing.T, backend string, raw json.RawMessage) *InferCommonSpec {
	t.Helper()
	switch backend {
	case "ollama":
		spec := &OllamaSpec{}
		if err := json.Unmarshal(raw, spec); err != nil {
			t.Fatalf("parsing infer_config as an OllamaSpec: %v", err)
		}
		return &spec.InferCommonSpec
	default:
		spec := &VllmSpec{}
		if err := json.Unmarshal(raw, spec); err != nil {
			t.Fatalf("parsing infer_config as a VllmSpec: %v", err)
		}
		return &spec.InferCommonSpec
	}
}

// avTransformationSpec builds the operator spec from the document, overriding
// only the server url and the pool size - the two things the benchmark is
// entitled to set, since the first is where the backend happens to be and the
// second is the variable being reported.
func avTransformationSpec(t *testing.T, backend, url string, raw json.RawMessage, poolSize int) *TransformationSpec {
	t.Helper()
	switch backend {
	case "ollama":
		spec := &OllamaSpec{}
		if err := json.Unmarshal(raw, spec); err != nil {
			t.Fatalf("parsing infer_config as an OllamaSpec: %v", err)
		}
		spec.Server = &OllamaServerSpec{Url: url}
		spec.PoolSize = poolSize
		spec.ErrorChannel = nil
		return &TransformationSpec{Type: "ollama", OllamaConfig: spec}
	default:
		spec := &VllmSpec{}
		if err := json.Unmarshal(raw, spec); err != nil {
			t.Fatalf("parsing infer_config as a VllmSpec: %v", err)
		}
		spec.Server = &OllamaServerSpec{Url: url}
		spec.PoolSize = poolSize
		spec.ErrorChannel = nil
		return &TransformationSpec{Type: "vllm", VllmConfig: spec}
	}
}
