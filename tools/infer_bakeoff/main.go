// AG.2: the backend bake-off, measured as first-shot schema validity.
//
// **What this measures, and why it is not a throughput benchmark.** vLLM was
// wanted for two reasons: it constrains the decoder with the output schema, and
// its batching was expected to return a valid answer on the first attempt. So
// the primary figure here is **the fraction of requests whose first response
// validates against the schema**, per operator, with denominators and no
// aggregate (decision 13); wall-clock to a first *valid* answer is secondary,
// and tokens per second is a component of that rather than a headline. A
// backend that answers correctly on the first attempt at half the tokens per
// second is the better backend for this workload, and a report led by tok/s
// would obscure it (plan §20.2, §20.8).
//
// **The metric has a predicted value, which is what makes this an experiment.**
// Under true constrained decoding a non-conformant response is not unlikely, it
// is ungrammatical — so arm 3's predicted score is **100%**, and any shortfall
// is a finding about the wiring rather than about the model. The Ollama side has
// a published prior in the other direction: I-41 measured the deployed shape at
// nine parseable, non-conformant fragments out of nine.
//
// **Three arms, because two would confound the result** (plan §20.3), and the
// third turned into two on contact with a server:
//
//	1   ollama_format     Ollama, the schema in `format` only        the deployed shape
//	2   ollama_prompt     Ollama, `format` *and* I-180's remedy      the unported fix
//	3a  vllm_guided       vLLM, `guided_json`                        the operator's DEFAULT arm
//	3b  vllm_json_schema  vLLM, a named `response_format`, strict    the operator's other arm
//
// **3a and 3b are the two arms the operator already carries** and translates
// from the same `response_format` property (vllmStructuredOutput,
// jets/compute_pipes/pipe_transformation_vllm.go). Measuring only one of them
// would have reported whichever the pinned server happens to honour, and on
// vLLM 0.28.0 the answer is that `guided_json` is accepted and discarded while
// the OpenAI-compatible arm constrains: 0 of 24 against 23 of 24, same server,
// same model, same prompts, one request field different (plan §24.5).
//
// Arm 2 is the one that makes the result mean something. Benchmarking arm 1
// against arm 3 cannot attribute a vLLM win between *constrained decoding beats
// prompting* and *we never ported the fix*: F422 says the Go client puts the
// schema in `format` and never in the prompt, and I-41 measured that at 0 of 9
// while the same model with the schema rendered as TypeScript and one worked
// example per variant scored 9 of 9. If arm 2 closes most of the gap to arm 3,
// vLLM is wanted for parallelism and not for conformance, and those have
// different deployment answers.
//
// **The arms differ in exactly one thing each.** Arm 2 is arm 1 plus prompt
// material — same client, same `format`, same schema, same case list. Arm 3 is
// arm 1's prompt against a different server. So arm 2 minus arm 1 is the prompt
// and arm 3 minus arm 1 is the backend.
//
// **Every arm is judged by one validator in this program**, not by whatever the
// client it went through happened to think. Arms 1 and 2 go through
// jets/agentic/infer, which validates too; its verdict is compared with this
// one and a disagreement is printed rather than absorbed.
//
// **The backend each arm reached is probed and asserted, never assumed.** On the
// machine this was written for, two different servers listen on port 11434 —
// Ollama on 127.0.0.1 and an ssh tunnel to the deployed vLLM on [::1] — so
// `localhost` names both and Go's dialer may prefer either. Worse, Ollama serves
// an OpenAI-compatible shim, so `/v1/models` answers on both and an arm 3
// pointed at Ollama by accident would send `guided_json`, have it silently
// ignored, and report an unconstrained Ollama run as a vLLM measurement. The
// discriminator is `/api/tags`, which vLLM does not serve.
//
//	go run ./tools/infer_bakeoff -root .. -arms 1,2 -control
//	go run ./tools/infer_bakeoff -root .. -arms 3a,3b -vllm-host 'http://[::1]:11434'
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/eval"
	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/prompt"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The arms, by the names the report prints. The numbers are plan §20.3's.
const (
	armOllamaFormat = "ollama_format"
	armOllamaPrompt = "ollama_prompt"
	armVllmGuided   = "vllm_guided"
	armVllmSchema   = "vllm_json_schema"
)

// The two arms of vLLM structured output, named as the operator names them
// (vllmStructuredGuidedJson and vllmStructuredJsonSchema,
// jets/compute_pipes/pipe_transformation_vllm.go). Which one a server honours
// is a property of its vLLM version rather than of the configuration, which is
// the operator's own stated reason for making it a property — and is exactly
// what arm 3b is here to measure.
const (
	structuredGuidedJson = "guided_json"
	structuredJsonSchema = "json_schema"
)

// The backends a probe can identify.
const (
	backendOllama  = "ollama"
	backendOpenAI  = "openai-compatible"
	backendUnknown = "unknown"
)

func main() {
	root := flag.String("root", "..", "the checkout holding workspaces/*/pipes_config (the parent repo, not jetstore_ai)")
	schemaPath := flag.String("schema", "tools/cpipes_contract/cpipes_schema.json", "the emitted cpipes contract")
	bundlePath := flag.String("bundles", "tools/cpipes_contract/matrix/bundle_members.csv",
		"the authored bundle membership; the bundle is the unit a hole addresses, not the flat leaf")
	arms := flag.String("arms", "1,2", "which arms to run: 1 (ollama_format), 2 (ollama_prompt), 3 or 3a (vllm_guided), 3b (vllm_json_schema)")
	ollamaHost := flag.String("ollama-host", "http://127.0.0.1:11434",
		"the Ollama server for arms 1 and 2 — an address literal, never a name (see the package comment)")
	vllmHost := flag.String("vllm-host", "http://[::1]:11434",
		"the vLLM server for arm 3 — an address literal, never a name")
	model := flag.String("model", "granite4.1:3b", "the model tag to ask for; name it beside every figure (Q-38)")
	vllmModel := flag.String("vllm-model", "", "the model tag arm 3 asks for, when the vLLM server names it differently; empty means -model")
	everyNth := flag.Int("every-nth", 3, "hold out every nth file")
	perOperator := flag.Int("per-operator", 3, "how many held-out cases to take per operator; 0 means all")
	maxCases := flag.Int("max-cases", 0, "overall cap on cases per arm, 0 for no cap")
	shots := flag.Int("shots", 4, "worked examples arm 2 carries, drawn from the training split only")
	numCtx := flag.Int("num-ctx", prompt.DefaultContextTokens, "num_ctx the Ollama arms ask the server for")
	temperature := flag.Float64("temperature", 0, "sampling temperature; 0 and a fixed seed, so a rerun is a rerun (F118)")
	seed := flag.Int("seed", 1, "sampling seed")
	maxTokens := flag.Int("max-tokens", 4096, "ceiling on the answer, applied to every arm — vLLM's max_tokens and Ollama's num_predict")
	timeout := flag.Duration("timeout", 10*time.Minute, "bound on one model call")
	control := flag.Bool("control", false, "run the first arm again at the end, as a repeated-arm control (plan §20.7)")
	verbose := flag.Bool("v", false, "print the answer and the removed instance for every case")
	dryRun := flag.Bool("dry-run", false, "build every prompt, probe every server, and call no model")
	out := flag.String("out", "", "also write the run as JSON here")
	flag.Parse()

	if err := run(config{
		root: *root, schemaPath: *schemaPath, bundlePath: *bundlePath, arms: *arms,
		ollamaHost: *ollamaHost, vllmHost: *vllmHost, model: *model, vllmModel: *vllmModel,
		everyNth: *everyNth, perOperator: *perOperator, maxCases: *maxCases, shots: *shots,
		numCtx: *numCtx, temperature: *temperature, seed: *seed, timeout: *timeout,
		maxTokens: *maxTokens,
		control:   *control, verbose: *verbose, dryRun: *dryRun, out: *out,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type config struct {
	root, schemaPath, bundlePath, arms     string
	ollamaHost, vllmHost, model, vllmModel string
	everyNth, perOperator, maxCases, shots int
	numCtx, seed, maxTokens                int
	temperature                            float64
	timeout                                time.Duration
	control, verbose, dryRun               bool
	out                                    string
}

// attempt is one model call: what was asked, what came back, and whether the
// first response satisfied the schema.
type attempt struct {
	Arm      string `json:"arm"`
	Pass     int    `json:"pass"`
	Operator string `json:"operator"`
	File     string `json:"file"`
	Index    int    `json:"index"`
	// Valid is the measurement: the *first* response validated against the
	// schema the request carried.
	Valid bool `json:"valid"`
	// Transport separates "the call did not happen" from "the answer was
	// wrong". A 503 from a load balancer with no healthy target is a wiring
	// problem, and counting it as a non-conformant answer would put it in the
	// column that is supposed to measure the model.
	Transport bool `json:"transport_error,omitempty"`
	// Ceiling records that generation stopped at the token ceiling. It is a
	// *measurement* rather than a transport failure — a constrained generation
	// that ran to the ceiling did not produce a valid answer, and counting it
	// out of the denominator would flatter the arm that ran away.
	Ceiling      bool    `json:"ceiling,omitempty"`
	Err          string  `json:"error,omitempty"`
	Seconds      float64 `json:"seconds"`
	PromptTokens int     `json:"prompt_tokens"`
	EvalTokens   int     `json:"eval_tokens"`
	ServedModel  string  `json:"served_model,omitempty"`
	PromptChars  int     `json:"prompt_chars"`
	// ClientDisagreed records the case where the client that made the call and
	// this program's own validator reached different verdicts on one answer.
	ClientDisagreed bool `json:"client_disagreed,omitempty"`
}

// armRun is one arm's whole result. It carries counts and never a rate: a rate
// over operators of different difficulty measures the mix.
type armRun struct {
	Arm      string    `json:"arm"`
	Pass     int       `json:"pass"`
	Host     string    `json:"host"`
	Backend  string    `json:"backend"`
	Detail   string    `json:"backend_detail"`
	Model    string    `json:"model_asked"`
	Attempts []attempt `json:"attempts"`
	Skipped  string    `json:"skipped,omitempty"`
}

type runReport struct {
	Started     string            `json:"started"`
	Model       string            `json:"model_asked"`
	NumCtx      int               `json:"num_ctx"`
	Temperature float64           `json:"temperature"`
	Seed        int               `json:"seed"`
	EveryNth    int               `json:"every_nth"`
	Shots       int               `json:"shots"`
	LiveByOp    map[string]int    `json:"live_instances_by_operator"`
	HeldByOp    map[string]int    `json:"held_out_instances_by_operator"`
	Refusals    map[string]string `json:"refusals,omitempty"`
	Arms        []armRun          `json:"arms"`
}

func run(c config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	contract, err := os.ReadFile(c.schemaPath)
	if err != nil {
		return fmt.Errorf("reading the contract: %w", err)
	}
	bundles, err := bundleOf(c.bundlePath)
	if err != nil {
		return err
	}

	corpus, err := eval.LoadCorpus(c.root)
	if err != nil {
		return err
	}
	split, err := corpus.SplitFiles(c.everyNth)
	if err != nil {
		return err
	}
	live := corpus.ByOperator()
	held := corpus.Coverage(split)
	fmt.Printf("corpus: %d live files, %d instances; holding out every %d file (%d held out, %d train)\n",
		len(corpus.Files), len(corpus.Instances), c.everyNth, len(split.HeldOut), len(split.Train))

	// One sub-schema per operator, resolved once. The unit a hole addresses is
	// the operator's *bundle* and not the flat leaf (F112): the leaf's
	// conditional_config re-admits every operator's config and blows the
	// window, and the bundle ranges that field to itself.
	schemas := map[string]json.RawMessage{}
	compiled := map[string]*jsonschema.Schema{}
	typescript := map[string]string{}
	refusals := map[string]string{}
	for op := range live {
		name, ok := bundles[op]
		if !ok {
			refusals[op] = "no bundle row in bundle_members.csv; the flat leaf does not fit the window"
			continue
		}
		sub, err := prompt.Subschema(contract, name)
		if err != nil {
			refusals[op] = err.Error()
			continue
		}
		if err := prompt.Fits(sub, c.numCtx, 0); err != nil {
			refusals[op] = fmt.Sprintf("%s is ~%d tokens against a %d-token window",
				name, prompt.EstimateTokens(sub), c.numCtx)
			continue
		}
		s, err := compileOne(sub)
		if err != nil {
			return fmt.Errorf("compiling the schema for %s: %w", op, err)
		}
		ts, err := prompt.AsTypeScript(sub, name)
		if err != nil {
			return fmt.Errorf("rendering %s as TypeScript: %w", op, err)
		}
		schemas[op], compiled[op], typescript[op] = sub, s, ts
	}
	for op, why := range refusals {
		fmt.Printf("  %-18s not asked — %s\n", op, why)
	}

	cases, err := caseList(c, corpus, split, schemas)
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return errors.New("the split held out no case this run can ask for")
	}
	fmt.Printf("cases: %d, from %d held-out files\n\n", len(cases), len(split.HeldOut))

	// Worked examples come from the *training* side only. Few-shot from the
	// library is legitimate and few-shot from the target is not: an example
	// drawn from the held-out file would hand over the answer.
	examples, err := examplesByOperator(c, corpus, split)
	if err != nil {
		return err
	}

	rep := runReport{
		Started: time.Now().UTC().Format(time.RFC3339), Model: c.model, NumCtx: c.numCtx,
		Temperature: c.temperature, Seed: c.seed, EveryNth: c.everyNth, Shots: c.shots,
		LiveByOp: live, HeldByOp: held, Refusals: refusals,
	}

	order, err := armOrder(c.arms, c.control)
	if err != nil {
		return err
	}
	for _, a := range order {
		r := runArm(ctx, c, a.name, a.pass, cases, schemas, compiled, typescript, examples)
		rep.Arms = append(rep.Arms, r)
		if ctx.Err() != nil {
			fmt.Println("\ninterrupted; reporting what was measured")
			break
		}
	}

	fmt.Print("\n" + render(&rep, live, held))
	if c.out != "" {
		b, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(c.out, append(b, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Printf("\nwrote %s\n", c.out)
	}
	return nil
}

type armPass struct {
	name string
	pass int
}

// armOrder turns -arms into the sequence of passes to run, appending the
// repeated first arm when -control is set.
//
// **The control is the plan's, and its reading is stated there:** if the two
// passes of the repeated arm differ by more than the arms differ from each
// other, the run establishes nothing.
func armOrder(spec string, control bool) ([]armPass, error) {
	names := map[string]string{
		"1": armOllamaFormat, armOllamaFormat: armOllamaFormat,
		"2": armOllamaPrompt, armOllamaPrompt: armOllamaPrompt,
		"3": armVllmGuided, "3a": armVllmGuided, armVllmGuided: armVllmGuided,
		"3b": armVllmSchema, armVllmSchema: armVllmSchema,
	}
	var out []armPass
	seen := map[string]int{}
	for _, tok := range strings.Split(spec, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		n, ok := names[tok]
		if !ok {
			return nil, fmt.Errorf("unknown arm %q; the arms are 1, 2, 3a and 3b", tok)
		}
		seen[n]++
		out = append(out, armPass{name: n, pass: seen[n]})
	}
	if len(out) == 0 {
		return nil, errors.New("no arms named")
	}
	if control {
		first := out[0].name
		seen[first]++
		out = append(out, armPass{name: first, pass: seen[first]})
	}
	return out, nil
}

// caseList picks the held-out instances to ask for, stratified by operator.
//
// **Stratified rather than capped in corpus order**, because the figure is per
// operator: two operators are most of the corpus, so a flat cap measures those
// two and reports every other operator as not run.
func caseList(c config, corpus *eval.Corpus, split *eval.Split, schemas map[string]json.RawMessage) ([]*eval.Case, error) {
	heldFile := map[string]bool{}
	for _, f := range split.HeldOut {
		heldFile[f] = true
	}
	taken := map[string]int{}
	files := map[string][]byte{}
	var out []*eval.Case
	for _, inst := range corpus.Instances {
		if !heldFile[inst.File] || len(schemas[inst.Operator]) == 0 {
			continue
		}
		if c.perOperator > 0 && taken[inst.Operator] >= c.perOperator {
			continue
		}
		if c.maxCases > 0 && len(out) >= c.maxCases {
			break
		}
		raw, ok := files[inst.File]
		if !ok {
			var err error
			raw, err = os.ReadFile(filepath.Join(c.root, inst.File))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", inst.File, err)
			}
			files[inst.File] = raw
		}
		cs, err := eval.MakeCase(raw, inst)
		if err != nil {
			return nil, fmt.Errorf("making a case from %s: %w", inst.File, err)
		}
		taken[inst.Operator]++
		out = append(out, cs)
	}
	return out, nil
}

// examplesByOperator renders arm 2's worked examples, one operator at a time.
//
// **One per file before a second from any file**, which is the shape-diversity
// rule the Python picker uses one level down: three instances out of one config
// teach less than one each from three.
func examplesByOperator(c config, corpus *eval.Corpus, split *eval.Split) (map[string]string, error) {
	if c.shots <= 0 {
		return map[string]string{}, nil
	}
	trainFile := map[string]bool{}
	for _, f := range split.Train {
		trainFile[f] = true
	}
	byOpFile := map[string]map[string][]json.RawMessage{}
	files := map[string][]byte{}
	for _, inst := range corpus.Instances {
		if !trainFile[inst.File] {
			continue
		}
		raw, ok := files[inst.File]
		if !ok {
			var err error
			raw, err = os.ReadFile(filepath.Join(c.root, inst.File))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", inst.File, err)
			}
			files[inst.File] = raw
		}
		cs, err := eval.MakeCase(raw, inst)
		if err != nil {
			continue
		}
		if byOpFile[inst.Operator] == nil {
			byOpFile[inst.Operator] = map[string][]json.RawMessage{}
		}
		byOpFile[inst.Operator][inst.File] = append(byOpFile[inst.Operator][inst.File], cs.Expected)
	}

	out := map[string]string{}
	for op, byFile := range byOpFile {
		names := make([]string, 0, len(byFile))
		for f := range byFile {
			names = append(names, f)
		}
		sort.Strings(names)
		var picked []string
		for round := 0; len(picked) < c.shots; round++ {
			progressed := false
			for _, f := range names {
				if round >= len(byFile[f]) {
					continue
				}
				progressed = true
				var pretty bytes.Buffer
				if err := json.Indent(&pretty, byFile[f][round], "", " "); err != nil {
					continue
				}
				picked = append(picked, fmt.Sprintf("// from %s\n%s", f, pretty.String()))
				if len(picked) >= c.shots {
					break
				}
			}
			if !progressed {
				break
			}
		}
		if len(picked) > 0 {
			out[op] = strings.Join(picked, "\n")
		}
	}
	return out, nil
}

// remedy is I-180's, assembled: the schema as TypeScript declarations, the legal
// discriminator token named, and worked examples one per file.
//
// **Three changes, because I-41 measured all three and only the third moved the
// number to 9 of 9.** Its table is the reason arm 2 is not simply "the JSON
// Schema pasted into the prompt" — P.1 ran that in Phase 3 and the figure moved
// by nothing.
func remedy(op, ts, examples string, count int) string {
	var b strings.Builder
	b.WriteString("\n\nThese are the types you must produce, as TypeScript declarations:\n\n```typescript\n")
	b.WriteString(ts)
	b.WriteString("```\n\n")
	fmt.Fprintf(&b, "The `type` field must be exactly: %q.", op)
	if examples != "" {
		fmt.Fprintf(&b, "\n\nHere are %d real examples from existing JetStore configurations. "+
			"Follow their field names exactly — note which fields each one requires:\n\n```json\n%s\n```",
			count, examples)
	}
	return b.String()
}

func runArm(ctx context.Context, c config, arm string, pass int, cases []*eval.Case,
	schemas map[string]json.RawMessage, compiled map[string]*jsonschema.Schema,
	typescript, examples map[string]string) armRun {

	host, want := c.ollamaHost, backendOllama
	model := c.model
	if isVllm(arm) {
		host, want = c.vllmHost, backendOpenAI
		if c.vllmModel != "" {
			model = c.vllmModel
		}
	}
	r := armRun{Arm: arm, Pass: pass, Host: host, Model: model}

	label := arm
	if pass > 1 {
		label = fmt.Sprintf("%s (pass %d)", arm, pass)
	}
	fmt.Printf("── %s ── %s\n", label, host)

	// **Probe before measuring, and refuse on a mismatch.** Ollama serves an
	// OpenAI-compatible shim, so an arm 3 pointed at Ollama would answer
	// happily, ignore `guided_json`, and report an unconstrained run as a
	// constrained one. /api/tags is the discriminator: vLLM does not serve it.
	backend, detail := probe(ctx, host, c.timeout)
	r.Backend, r.Detail = backend, detail
	fmt.Printf("   backend: %s — %s\n", backend, detail)
	switch {
	case backend == backendUnknown:
		r.Skipped = fmt.Sprintf("no inference server answered at %s (%s)", host, detail)
	case want == backendOllama && backend != backendOllama:
		r.Skipped = fmt.Sprintf("this arm needs Ollama and %s answered %s", host, backend)
	case want == backendOpenAI && backend == backendOllama:
		r.Skipped = fmt.Sprintf("this arm needs vLLM and %s is Ollama; guided_json would be ignored "+
			"and the run would report an unconstrained answer as a constrained one", host)
	}
	if r.Skipped != "" {
		fmt.Printf("   skipped: %s\n", r.Skipped)
		return r
	}
	if c.dryRun {
		r.Skipped = "dry run"
	}

	client := &infer.Client{
		Host: host, Model: model, RequestTimeout: c.timeout, MaxRetry: -1,
		Options: map[string]any{"num_ctx": c.numCtx, "temperature": c.temperature, "seed": c.seed,
			"num_predict": c.maxTokens},
	}
	structured := structuredGuidedJson
	if arm == armVllmSchema {
		structured = structuredJsonSchema
	}
	vc := &vllmClient{host: host, model: model, timeout: c.timeout, structured: structured,
		temperature: c.temperature, seed: c.seed, maxTokens: c.maxTokens,
		http: &http.Client{Timeout: c.timeout}}

	for _, cs := range cases {
		if ctx.Err() != nil {
			break
		}
		extra := ""
		if arm == armOllamaPrompt {
			ex := examples[cs.Operator]
			extra = remedy(cs.Operator, typescript[cs.Operator], ex, strings.Count(ex, "// from "))
		}
		instruction, err := eval.Instruction(cs, c.numCtx, nil, extra)
		if err != nil {
			r.Attempts = append(r.Attempts, attempt{Arm: arm, Pass: pass, Operator: cs.Operator,
				File: cs.File, Index: cs.Hole.Index, Transport: true, Err: err.Error()})
			continue
		}
		a := attempt{Arm: arm, Pass: pass, Operator: cs.Operator, File: cs.File,
			Index: cs.Hole.Index, PromptChars: len(instruction) + len(eval.SystemPrompt)}
		if c.dryRun {
			r.Attempts = append(r.Attempts, a)
			continue
		}

		began := time.Now()
		var content string
		var clientSaidValid bool
		if isVllm(arm) {
			content, a.PromptTokens, a.EvalTokens, a.ServedModel, a.Ceiling, err =
				vc.call(ctx, eval.SystemPrompt, instruction, schemas[cs.Operator])
			if err != nil {
				a.Transport, a.Err = true, err.Error()
			}
			if a.Ceiling {
				// A constrained generation that ran to the ceiling is a failed
				// first shot with a name, not a call that did not happen.
				a.Err = "generation stopped at the token ceiling (finish_reason=length)"
			}
		} else {
			resp, cerr := client.Chat(ctx, &infer.Request{
				System: eval.SystemPrompt, User: instruction, Schema: schemas[cs.Operator],
			})
			var se *infer.SchemaError
			switch {
			case cerr == nil:
				content, a.PromptTokens, a.EvalTokens, a.ServedModel = resp.Content,
					resp.PromptTokens, resp.EvalTokens, resp.ModelName
				clientSaidValid = true
			case errors.As(cerr, &se):
				// The model answered and the answer was rejected. That is a
				// measurement, not a failure of the run.
				content, a.PromptTokens, a.EvalTokens = se.Content, se.PromptTokens, se.EvalTokens
				a.Err = firstLine(se.Err.Error())
			default:
				a.Transport, a.Err = true, firstLine(cerr.Error())
			}
		}
		a.Seconds = time.Since(began).Seconds()

		if !a.Transport {
			a.Valid = validates(compiled[cs.Operator], content, &a)
			if !isVllm(arm) && clientSaidValid != a.Valid {
				a.ClientDisagreed = true
			}
		}
		r.Attempts = append(r.Attempts, a)

		mark := "invalid"
		switch {
		case a.Transport:
			mark = "ERROR  "
		case a.Valid:
			mark = "valid  "
		}
		fmt.Printf("   %-18s %-40s %s %6.1fs %5dp %5de  %s\n", cs.Operator, short(cs.File), mark,
			a.Seconds, a.PromptTokens, a.EvalTokens, truncate(a.Err, 70))
		if c.verbose {
			fmt.Printf("       answered: %s\n", truncate(strings.TrimSpace(content), 400))
			fmt.Printf("       removed:  %s\n", truncate(string(cs.Expected), 400))
		}
	}
	return r
}

// validates is the one verdict every arm is judged by, whatever client carried
// the call.
func validates(schema *jsonschema.Schema, content string, a *attempt) bool {
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		if a.Err == "" {
			a.Err = "not valid JSON: " + firstLine(err.Error())
		}
		return false
	}
	if err := schema.Validate(value); err != nil {
		if a.Err == "" {
			// **The whole failure, squashed, rather than its first line.** A
			// jsonschema/v6 error opens with the document url and puts the
			// cause on the lines after it, so a first-line reading records the
			// same string for every failure in the run and classifies nothing.
			a.Err = truncate(squash(err.Error()), 300)
		}
		return false
	}
	return true
}

func compileOne(doc []byte) (*jsonschema.Schema, error) {
	var parsed any
	if err := json.Unmarshal(doc, &parsed); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", parsed); err != nil {
		return nil, err
	}
	return c.Compile("schema.json")
}

// ---------------------------------------------------------------------------
// The vLLM arm's client
// ---------------------------------------------------------------------------

// vllmClient speaks the OpenAI-compatible chat api the way
// jets/compute_pipes/pipe_transformation_vllm.go does: `guided_json` carrying
// the schema itself, sampling parameters top-level rather than nested under
// `options`, and streaming off.
//
// **It is a second implementation of that envelope and that is worth saying
// out loud.** jets/agentic/infer is an /api/chat client and has no vLLM arm at
// all — the operator is the only vLLM caller in the tree — so an agentic-side
// harness has nothing to reuse. Q-13 asked whether the schema belongs in the
// shared seam; this file is one more piece of evidence that it is the envelope
// rather than the schema that keeps being rewritten.
type vllmClient struct {
	host, model string
	// structured selects which of the operator's two arms this client sends,
	// guided_json or a named json_schema response_format.
	structured  string
	timeout     time.Duration
	temperature float64
	seed        int
	maxTokens   int
	http        *http.Client
}

func (v *vllmClient) call(ctx context.Context, system, user string, schema json.RawMessage) (
	content string, promptTokens, evalTokens int, served string, ceiling bool, err error) {

	body := map[string]any{
		"model":  v.model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": v.temperature,
		"seed":        v.seed,
		"max_tokens":  v.maxTokens,
	}
	// The operator's translation, reproduced: guided_json takes the schema
	// itself, and the json_schema arm wraps it in a named response_format with
	// `strict` — which is what makes that arm the equivalent of guided_json
	// rather than a hint.
	if v.structured == structuredJsonSchema {
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": "response", "schema": json.RawMessage(schema), "strict": true,
			},
		}
	} else {
		body["guided_json"] = schema
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", 0, 0, "", false, err
	}
	callCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost,
		strings.TrimSuffix(v.host, "/")+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, 0, "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.http.Do(req)
	if err != nil {
		return "", 0, 0, "", false, fmt.Errorf("cannot reach the vLLM server at %s: %w", v.host, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, "", false, fmt.Errorf("the vLLM server returned %s: %s",
			resp.Status, truncate(strings.TrimSpace(string(raw)), 200))
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, 0, "", false, fmt.Errorf("while decoding the response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", out.Usage.PromptTokens, out.Usage.CompletionTokens, out.Model, false,
			errors.New("the vLLM server returned no choice")
	}
	// A constrained generation that stopped at the token ceiling is invalid by
	// construction, and saying so is not the same as saying the model wrote a
	// bad answer. The operator makes the same distinction.
	return out.Choices[0].Message.Content, out.Usage.PromptTokens, out.Usage.CompletionTokens,
		out.Model, out.Choices[0].FinishReason == "length", nil
}

// probe says which backend is listening, by asking for the one route that
// separates them.
//
// **/v1/models does not separate them.** Ollama serves an OpenAI-compatible
// shim and answers it with its own tags, so a probe built on that route calls
// Ollama vLLM. /api/tags is Ollama's own and vLLM 404s it.
func probe(ctx context.Context, host string, timeout time.Duration) (string, string) {
	client := &http.Client{Timeout: min(timeout, 30*time.Second)}
	get := func(path string) (int, []byte, error) {
		c, cancel := context.WithTimeout(ctx, min(timeout, 30*time.Second))
		defer cancel()
		req, err := http.NewRequestWithContext(c, http.MethodGet, strings.TrimSuffix(host, "/")+path, nil)
		if err != nil {
			return 0, nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
		return resp.StatusCode, b, nil
	}

	if code, body, err := get("/api/tags"); err == nil && code == http.StatusOK {
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &tags) == nil && len(tags.Models) > 0 {
			names := make([]string, 0, len(tags.Models))
			for _, m := range tags.Models {
				names = append(names, m.Name)
			}
			return backendOllama, "/api/tags served " + strings.Join(names, ", ")
		}
	}
	code, body, err := get("/v1/models")
	if err != nil {
		return backendUnknown, err.Error()
	}
	if code != http.StatusOK {
		return backendUnknown, fmt.Sprintf("/api/tags and /v1/models both refused; /v1/models said %d: %s",
			code, truncate(squash(string(body)), 160))
	}
	var models struct {
		Data []struct {
			ID          string `json:"id"`
			OwnedBy     string `json:"owned_by"`
			Root        string `json:"root"`
			MaxModelLen int    `json:"max_model_len"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &models)
	// **owned_by is the field that names the server in one word**, and it is
	// recorded rather than merely read: a swapped arm is then a loud difference
	// in the report instead of a plausible number.
	ids := make([]string, 0, len(models.Data))
	for _, m := range models.Data {
		s := m.ID
		if m.OwnedBy != "" {
			s += " (owned_by " + m.OwnedBy
			if m.Root != "" {
				s += ", root " + m.Root
			}
			if m.MaxModelLen > 0 {
				s += fmt.Sprintf(", max_model_len %d", m.MaxModelLen)
			}
			s += ")"
		}
		ids = append(ids, s)
	}
	detail := "/v1/models served " + strings.Join(ids, ", ") + "; /api/tags did not answer"
	if _, v, err := get("/version"); err == nil {
		detail += "; /version " + truncate(squash(string(v)), 60)
	}
	return backendOpenAI, detail
}

// ---------------------------------------------------------------------------
// The report
// ---------------------------------------------------------------------------

// render prints the run per operator and per arm, with denominators and no
// aggregate.
//
// **There is deliberately no total.** Decision 13 bans one, and here it would do
// specific damage: two operators are most of the corpus, so a total is those two
// wearing a coat, and the number that would travel is the one that cannot be
// read.
func render(rep *runReport, live, held map[string]int) string {
	var b strings.Builder
	b.WriteString("first-shot schema validity — the fraction of requests whose FIRST response\n")
	b.WriteString("validated against the schema the request carried. Per operator, with\n")
	b.WriteString("denominators. No aggregate (decision 13).\n\n")

	ops := map[string]bool{}
	for _, a := range rep.Arms {
		for _, at := range a.Attempts {
			ops[at.Operator] = true
		}
	}
	names := make([]string, 0, len(ops))
	for op := range ops {
		names = append(names, op)
	}
	sort.Strings(names)

	for _, a := range rep.Arms {
		label := a.Arm
		if a.Pass > 1 {
			label = fmt.Sprintf("%s (pass %d)", a.Arm, a.Pass)
		}
		fmt.Fprintf(&b, "%s — %s, %s, model %s\n", label, a.Host, a.Backend, a.Model)
		if a.Skipped != "" {
			fmt.Fprintf(&b, "  not run: %s\n\n", a.Skipped)
			continue
		}
		fmt.Fprintf(&b, "  %-18s %10s %12s %14s %12s\n", "operator", "valid/asked", "errors", "median s (valid)", "eval tok/s")
		for _, op := range names {
			var asked, valid, errs, ceil int
			var secs []float64
			var evalTok int
			var evalSecs float64
			for _, at := range a.Attempts {
				if at.Operator != op {
					continue
				}
				if at.Transport {
					errs++
					continue
				}
				asked++
				if at.Ceiling {
					ceil++
				}
				if at.Valid {
					valid++
					secs = append(secs, at.Seconds)
				}
				evalTok += at.EvalTokens
				evalSecs += at.Seconds
			}
			if asked == 0 && errs == 0 {
				continue
			}
			tps := "-"
			if evalSecs > 0 && evalTok > 0 {
				tps = fmt.Sprintf("%.1f", float64(evalTok)/evalSecs)
			}
			note := ""
			if ceil > 0 {
				note = fmt.Sprintf("  %d hit the token ceiling", ceil)
			}
			fmt.Fprintf(&b, "  %-18s %10s %12d %14s %12s%s\n", op,
				fmt.Sprintf("%d/%d", valid, asked), errs, medianOf(secs), tps, note)
		}
		fmt.Fprintf(&b, "\n")
	}

	b.WriteString("held-out instances by operator (the pool a denominator is drawn from):\n")
	for _, op := range sortedKeys(held) {
		fmt.Fprintf(&b, "  %-18s %3d held out of %d live\n", op, held[op], live[op])
	}
	b.WriteString("\nembed: no vLLM arm exists — it calls Ollama's /api/embed and the vLLM operator\n")
	b.WriteString("speaks /v1/chat/completions (F424). Its vLLM column is a structural zero rather\n")
	b.WriteString("than an unmeasured cell, and it is not a row above because it authors no\n")
	b.WriteString("transformation this corpus holds out.\n")

	// **The timing columns are within-arm readings and the cross-arm comparison
	// is refused rather than omitted.** A missing row reads as forgotten; this
	// one is a decision.
	// **The tok/s column is not a decode rate and must not be read as one.** It
	// is eval tokens over the whole wall clock of the call, so it carries the
	// prompt evaluation, the queue and every hop of the transport. A backend
	// reached through a tunnel and a load balancer is charged for both here.
	b.WriteString("\neval tok/s is eval tokens over the WHOLE wall clock of the call — prompt\n")
	b.WriteString("evaluation, queueing and transport included. It is not a decode rate, and on\n")
	b.WriteString("short answers it is dominated by the fixed cost of the request.\n")
	b.WriteString("\ntiming: the seconds and eval tok/s columns are comparable BETWEEN the two Ollama\n")
	b.WriteString("arms — same machine, same backend, same hardware, the remedy the only\n")
	b.WriteString("difference — and are NOT comparable between an Ollama arm and the vLLM arm.\n")
	b.WriteString("Where this run had them on different hardware, or reached one through a\n")
	b.WriteString("tunnel and a load balancer and the other over loopback, a cross-arm timing\n")
	b.WriteString("difference measures the hardware and the transport. First-shot validity is\n")
	b.WriteString("unaffected: whether a response satisfies a schema does not depend on where it\n")
	b.WriteString("was computed, which is why plan §20.2 made it the primary figure.\n")
	return b.String()
}

func medianOf(v []float64) string {
	if len(v) == 0 {
		return "-"
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	return fmt.Sprintf("%.1f", m)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// bundleOf reads the (bundle, type_token) pairs and keys them by operator, the
// same way tools/compile_pass does: only the TransformationSpec tier, because
// the column bundles share their tokens with the pipe operators.
func bundleOf(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading the bundle membership: %w", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	out := map[string]string{}
	for i, r := range rows {
		if i == 0 || len(r) < 2 || !strings.HasSuffix(r[0], "Pipe") {
			continue
		}
		out[r[1]] = r[0]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s named no operator bundles", path)
	}
	return out, nil
}

// isVllm says whether an arm talks to the vLLM server.
func isVllm(arm string) bool { return arm == armVllmGuided || arm == armVllmSchema }

func short(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) <= 2 {
		return p
	}
	return parts[1] + "/…/" + parts[len(parts)-1]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }
