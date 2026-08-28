// P.1: give the eval harness a caller.
//
// **What this is.** One compile-pass run end to end — load the live corpus,
// split it at file level, cut a case by mutation, drive the Phase-1 authoring
// loop against it, and report through eval.Report under decision 13's rules.
// Every part of that existed before this file and none of them had ever met:
// LoadCorpus, SplitFiles, Coverage and MakeCase had no consumer outside
// eval_test.go, and jets/agentic/agent had no non-test consumer anywhere in the
// repository (F32, F111). Criterion 20 was met by the harness satisfying its
// own contract, which is what the criterion asked and is less than a
// measurement.
//
// **What one run measures, and it is less than the number suggests.** A
// compile-pass figure from this program is one model, one prompt, one context
// length, one split and one repair budget. J.2 moved a hole from 0/9 to 9/9
// with four prompt changes and no model change, so the first number out of this
// harness measures the wiring at least as much as the model — which is why the
// report names the model and the case source, and why there is no aggregate to
// quote.
//
// **The wall this hit, and why it is the finding rather than the number.**
// A mutation case's ground truth is a whole transformation instance, so the
// schema that constrains generation is one operator's TransformationSpec — and
// on the emitted cpipes contract every one of the sixteen closes over the same
// shared substrate of channel specs and column evaluators, at 28,277 to 30,542
// estimated tokens (F112). Task.validate calls prompt.Fits, which reserves
// 8,192 of a 32,768 window for the instruction and the answer, so **at the
// deployed context length the loop refuses all 134 held-out cases before a
// model is called** — every operator in the corpus, for the same reason.
//
// **And the budget prompt.Fits enforces is not one this serving path spends
// (F113).** The schema rides in Ollama's `format` field, where it is compiled
// into a sampling grammar; it is never sent as messages. Measured against the
// running server on 2026-08-27: an 84-byte schema and a 119,038-byte schema
// both report prompt_eval_count of 30 for the same prompt, and 23 for another.
// So the two budgets are separate and this program keeps them separate —
// -plan-budget is what prompt.Fits is told, -num-ctx is what the server is
// asked to serve. Raising the first is how a case gets asked at all; the second
// stays at the deployed window, because that is the honest number to run at.
//
//	# what the deployed configuration measures: nothing, and it says why
//	go run ./tools/compile_pass -root ..
//
//	# ask anyway, with the served window left where the deployment has it
//	go run ./tools/compile_pass -root .. -plan-budget 40960 -model qwen2.5:0.5b
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/agent"
	"github.com/artisoft-io/jetstore/jets/agentic/eval"
	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/prompt"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
)

const systemPrompt = "You are a JetStore Compute Pipes configuration author. " +
	"You answer with one JSON transformation specification and nothing else — no prose, " +
	"no markdown fence, no explanation."

// instructionBudgetShare is how much of the reserved window the instruction may
// take. The rest of the reserve is the answer, and an answer that cannot be
// finished is a case lost to formatting rather than to authoring.
const instructionBudgetShare = 2

func main() {
	root := flag.String("root", ".", "the checkout holding workspaces/*/pipes_config (the parent repo, not jetstore_ai)")
	schemaPath := flag.String("schema", "tools/cpipes_contract/cpipes_schema.json", "the emitted cpipes contract")
	bundlePath := flag.String("bundles", "tools/cpipes_contract/matrix/bundle_members.csv",
		"the authored bundle membership; the bundle is the unit a hole addresses, not the flat leaf")
	host := flag.String("host", "http://localhost:11434", "inference server")
	model := flag.String("model", "granite4.1:3b", "model tag")
	everyNth := flag.Int("every", 3, "hold out every nth file (decision 13's split is at file level)")
	maxCases := flag.Int("max-cases", 0, "stop after this many attempted cases; 0 means all of them")
	iterations := flag.Int("iterations", 3, "propose-verify-repair iterations per case")
	wall := flag.Duration("wall", 0, "wall-clock cap per case; 0 is unbounded")
	timeout := flag.Duration("timeout", 300*time.Second, "per-call timeout")
	planBudget := flag.Int("plan-budget", prompt.DefaultContextTokens,
		"the window prompt.Fits is told the task has; NOT the served window — the schema rides in "+
			"`format` and costs no prompt tokens (F113)")
	numCtx := flag.Int("num-ctx", prompt.DefaultContextTokens,
		"the context length the server is asked to serve (num_ctx)")
	temperature := flag.Float64("temperature", 0,
		"sampling temperature; 0 is greedy decoding, which is what makes a run comparable with itself")
	seed := flag.Int("seed", 1, "sampling seed, sent with the temperature")
	schemaInPrompt := flag.Bool("schema-in-prompt", false,
		"also put the sub-schema in the instruction. The Phase-1 loop sends it only as `format`, "+
			"which I-41 measured as not constraining the answer at all")
	era := flag.String("era", string(eval.EraPreTemplates), "which side of the gap-19 template transition this run is")
	dryRun := flag.Bool("dry-run", false, "build the cases and print the inventory; call no model and publish no figure")
	selfCheck := flag.Bool("self-check", false,
		"put each case's own ground truth back through Fill and the verifier, and call no model. "+
			"A case whose known-correct answer does not compile is a broken case")
	verbose := flag.Bool("v", false, "print every case")
	flag.Parse()

	if err := run(*root, *schemaPath, *bundlePath, *host, *model, *era, *everyNth, *maxCases, *iterations,
		*planBudget, *numCtx, *temperature, *seed, *schemaInPrompt, *wall, *timeout, *dryRun,
		*selfCheck, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "compile_pass:", err)
		os.Exit(1)
	}
}

func run(root, schemaPath, bundlePath, host, model, era string, everyNth, maxCases, iterations, planBudget, numCtx int,
	temperature float64, seed int, schemaInPrompt bool, wall, timeout time.Duration,
	dryRun, selfCheck, verbose bool) error {

	contract, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("reading the cpipes contract: %w", err)
	}
	specNames, err := transformationSpecNames(contract, bundlePath)
	if err != nil {
		return err
	}

	corpus, err := eval.LoadCorpus(root)
	if err != nil {
		return err
	}
	split, err := corpus.SplitFiles(everyNth)
	if err != nil {
		return err
	}
	coverage := corpus.Coverage(split)
	live := corpus.ByOperator()

	fmt.Printf("corpus: %d live files, %d transformation instances, %d operators\n",
		len(corpus.Files), len(corpus.Instances), len(live))
	fmt.Printf("split:  %d held out, %d train (1 file in %d)\n\n",
		len(split.HeldOut), len(split.Train), everyNth)

	// **Every operator the contract names, not only those the corpus uses.**
	// Decision 13 reserves "untested" for an operator with no live instances,
	// and a report built from ByOperator alone can never say it: an operator
	// with nothing in the corpus is absent from that map, so it vanishes
	// instead of reporting the absence of a measurement. The contract is the
	// list of operators that exist; the corpus is the list that are used.
	known := map[string]int{}
	for op := range specNames {
		known[op] = live[op]
	}
	for op, n := range live {
		known[op] = n
	}

	// One subschema per operator, and the fit check that decides whether the
	// operator can be asked at all. Done once and up front rather than per
	// case: the answer is a property of the contract and the window, not of the
	// case, and finding it out 458 times would say the same thing 458 times.
	schemas, refusals := planOperators(contract, specNames, live, planBudget)
	for _, op := range sortedKeys(live) {
		if r := refusals[op]; r != "" {
			fmt.Printf("  %-18s cannot be asked — %s\n", op, r)
		}
	}
	if len(refusals) > 0 {
		fmt.Println()
	}

	held := map[string]bool{}
	for _, f := range split.HeldOut {
		held[f] = true
	}
	var candidates []eval.Instance
	for _, inst := range corpus.Instances {
		if held[inst.File] {
			candidates = append(candidates, inst)
		}
	}

	if dryRun {
		return inventory(root, candidates, coverage, known, refusals)
	}

	registry0, err := tools.DefaultRegistry()
	if err != nil {
		return fmt.Errorf("building the tool registry: %w", err)
	}
	if selfCheck {
		return selfCheckCases(root, candidates, refusals, registry0, verbose)
	}

	// **The sampler is pinned, and the reason is `instancesIn`'s own.** That
	// walk sorts its output because "a harness whose case list reshuffles
	// between runs cannot be compared with itself" — and the determinism
	// stopped at the corpus. At Ollama's default temperature the same 45 cases
	// gave two passes on one run and none on the next (F118), so the figure had
	// a sampling variance nobody had measured and no run could be repeated.
	// Greedy decoding with a fixed seed is the default here for that reason;
	// -temperature is how a caller measures the variance deliberately.
	//
	// **num_ctx is passed through, and leaving it out would have been the
	// quiet failure.** prompt.Fits decides against the window a caller *says*
	// it has; Ollama serves whatever num_ctx it was configured with and
	// truncates the rest without complaint. A fit check against 40,960 and a
	// server quietly serving 4,096 is precisely the silent truncation Fits
	// exists to prevent, arriving one layer below it.
	// **Every case is checked against its own ground truth before the model
	// sees it, and the ones that cannot be won are excluded from the
	// denominator (F120).** A case whose removed instance does not validate
	// when put back is not a case: no answer can pass it, including the right
	// one. Thirteen of the 134 are in that state — `anonymize_file.pc.json`
	// reads `$CGT_MULTIPART_OUTPUT` and `$MAIN_INPUT_ROW_COUNT` in its `when`
	// clauses and the verifier's fixed environment defines neither — and
	// counting them would have depressed every rate this harness publishes by
	// about a tenth, invisibly.
	unwinnable := map[string]int{}
	var winnable []eval.Instance
	{
		seenFiles := map[string][]byte{}
		for _, inst := range candidates {
			if refusals[inst.Operator] != "" {
				continue
			}
			raw, seen := seenFiles[inst.File]
			if !seen {
				var rerr error
				raw, rerr = os.ReadFile(filepath.Join(root, inst.File))
				if rerr != nil {
					return fmt.Errorf("reading %s: %w", inst.File, rerr)
				}
				seenFiles[inst.File] = raw
			}
			c, cerr := eval.MakeCase(raw, inst)
			if cerr != nil {
				return fmt.Errorf("making a case from %s: %w", inst.File, cerr)
			}
			okCase, verr := groundTruthValidates(c, registry0)
			if verr != nil {
				return verr
			}
			if !okCase {
				unwinnable[inst.Operator]++
				continue
			}
			winnable = append(winnable, inst)
		}
	}
	if n := totalOf(unwinnable); n > 0 {
		fmt.Printf("excluded: %d case(s) whose own ground truth does not validate — "+
			"no answer can pass them, so they are outside every denominator below\n\n", n)
	}
	candidates = winnable

	client := &infer.Client{
		Host: host, Model: model, RequestTimeout: timeout,
		Options: map[string]any{"num_ctx": numCtx, "temperature": temperature, "seed": seed},
	}
	registry := registry0

	// A cancelled run is interrupted rather than exhausted, and the loop
	// already knows the difference — it must not count against the model.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	attempted := map[string]int{}
	passed := map[string]int{}
	errored := map[string]int{}
	var interrupted, harnessErrors, neverVerified int

	// **What answered, not what was asked for.** ToolReport has recorded this
	// since J.2 and Report could not; a tag resolves to whatever build the
	// server holds. One tiny probe is the cheapest way to learn it through the
	// same client the run uses, and its failure is not fatal — the run then
	// reports the tag it asked for, which is the weaker claim and says so.
	served := model + " (as requested; the server did not say what answered)"
	if probe, err := client.Chat(ctx, &infer.Request{
		System: "You answer with JSON only.",
		User:   "Reply with {\"ok\":true}.",
		Schema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"]}`),
	}); err == nil && probe.ModelName != "" {
		served = probe.ModelName
	}

	files := map[string][]byte{}
	started := time.Now()
	for _, inst := range candidates {
		if maxCases > 0 && totalOf(attempted) >= maxCases {
			break
		}
		if refusals[inst.Operator] != "" {
			continue
		}
		raw, ok := files[inst.File]
		if !ok {
			raw, err = os.ReadFile(filepath.Join(root, inst.File))
			if err != nil {
				return fmt.Errorf("reading %s: %w", inst.File, err)
			}
			files[inst.File] = raw
		}
		c, err := eval.MakeCase(raw, inst)
		if err != nil {
			return fmt.Errorf("making a case from %s: %w", inst.File, err)
		}

		schema := schemas[inst.Operator]
		var inPrompt json.RawMessage
		if schemaInPrompt {
			inPrompt = schema
		}
		instruction, err := instructionFor(c, numCtx, inPrompt)
		if err != nil {
			return fmt.Errorf("building the instruction for %s: %w", inst.File, err)
		}
		task := &agent.Task{
			Instruction:   instruction,
			Schema:        schema,
			System:        systemPrompt,
			Verifier:      "validate_cpipes_config",
			VerifierArgs:  verifierArgsFor(c),
			ContextTokens: planBudget,
		}
		loop := &agent.Loop{
			Infer:    client,
			Registry: registry,
			Budget:   agent.Budget{MaxIterations: iterations, WallClock: wall},
			RunId:    fmt.Sprintf("p1-%s-%d", inst.Operator, totalOf(attempted)+totalOf(errored)),
			Actor:    "tools/compile_pass",
			Tier:     "assisted",
		}

		began := time.Now()
		result, err := loop.Run(ctx, task)
		took := time.Since(began).Round(time.Second)
		switch {
		case err != nil:
			// The run could not be conducted — an unreachable model, a
			// verifier that errored. That is not a measurement of authoring
			// and does not belong in the denominator; it is printed instead.
			harnessErrors++
			errored[inst.Operator]++
			fmt.Printf("  %-18s %-44s ERROR after %s: %v\n", inst.Operator, short(inst.File), took, err)
			if ctx.Err() != nil {
				return fmt.Errorf("interrupted")
			}
		case result.Outcome == agent.OutcomeInterrupted:
			interrupted++
		case result.Outcome == agent.OutcomeSucceeded:
			attempted[inst.Operator]++
			passed[inst.Operator]++
			fmt.Printf("  %-18s %-44s pass in %d iteration(s), %d tokens, %s\n",
				inst.Operator, short(inst.File), result.Iterations, result.TokenSpend, took)
			if verbose {
				// **What passed, not only that something did.** Compile-pass is
				// the gate decision 13 chose and it judges the config, not the
				// resemblance to what was removed — so an answer can pass while
				// being nothing like the instance it replaced, and a reader
				// sizing the number needs to see one.
				fmt.Printf("      answered: %s\n", strings.TrimSpace(string(result.Artifact)))
				fmt.Printf("      removed:  %s\n", truncateJSON(c.Expected, 400))
			}
		default:
			attempted[inst.Operator]++
			// **A run that exhausts with no diagnostics never reached the
			// verifier.** LastDiagnostics is the last *verdict*; it stays nil
			// when every iteration failed client-side schema validation
			// instead, so the config was never judged at all. That is the
			// difference between "the model wrote a bad config" and "the model
			// did not write a config", and I-41 says which to expect from a
			// prompt that carries no schema.
			why := ""
			if len(result.LastDiagnostics) == 0 {
				neverVerified++
				why = ", answer never satisfied the schema"
			}
			fmt.Printf("  %-18s %-44s %s after %d iteration(s), %d tokens, %s%s\n",
				inst.Operator, short(inst.File), result.Outcome, result.Iterations,
				result.TokenSpend, took, why)
			if verbose {
				for _, d := range result.LastDiagnostics {
					fmt.Printf("      %s\n", strings.TrimSpace(string(d)))
				}
			}
		}
		if ctx.Err() != nil {
			break
		}
	}

	report := &eval.Report{
		Era:          eval.Era(era),
		Model:        served,
		CaseSource:   caseSource(everyNth, planBudget, numCtx, temperature, seed, schemaInPrompt),
		HeldOutFiles: split.HeldOut,
	}
	for _, op := range sortedKeys(known) {
		res := eval.OperatorResult{
			Operator:      op,
			Attempted:     attempted[op],
			Passed:        passed[op],
			LiveInstances: known[op],
		}
		switch {
		case res.Attempted > 0:
		case res.Untested():
			// Nothing to say beyond the absence of a measurement, and the
			// report renders that itself.
		case refusals[op] != "":
			res.NotRun = refusals[op]
		case coverage[op] == 0:
			res.NotRun = "the split held out no instance of it"
		case unwinnable[op] > 0 && coverage[op] == unwinnable[op]:
			res.NotRun = fmt.Sprintf("all %d held-out case(s) are unwinnable: the removed instance "+
				"does not validate when put back", unwinnable[op])
		case maxCases > 0:
			res.NotRun = fmt.Sprintf("the run stopped at -max-cases=%d before reaching it", maxCases)
		case errored[op] > 0:
			res.NotRun = fmt.Sprintf("%d case(s) could not be conducted", errored[op])
		}
		report.Operators = append(report.Operators, res)
	}

	fmt.Println()
	fmt.Print(report.String())
	fmt.Printf("\nrun: %s, planned against %d tokens, served at num_ctx %d, temperature %g seed %d, "+
		"%d iteration budget\n",
		time.Since(started).Round(time.Second), planBudget, numCtx, temperature, seed, iterations)
	if harnessErrors > 0 {
		fmt.Printf("%d case(s) could not be conducted and are outside every denominator above\n", harnessErrors)
	}
	if interrupted > 0 {
		fmt.Printf("%d case(s) were interrupted and are outside every denominator above\n", interrupted)
	}
	if failed := totalOf(attempted) - totalOf(passed); failed > 0 {
		fmt.Printf("%d of %d failed case(s) never reached the verifier: every proposal failed "+
			"client-side schema validation, so no config was judged\n", neverVerified, failed)
	}
	if err := report.Validate(); err != nil {
		return err
	}
	return nil
}

// caseSource is the provenance line the report may not be published without.
//
// **It says mutation, and that is the whole of P.1's provenance decision.**
// Plan §4 asked whether Case gains a provenance field or the two libraries stay
// apart; they stay apart (I-115, I-136), so what a reader needs is not a label
// on each case but a statement of the population a figure was measured over.
func caseSource(everyNth, planBudget, numCtx int, temperature float64, seed int, schemaInPrompt bool) string {
	where := "schema in `format` only, as the Phase-1 loop sends it"
	if schemaInPrompt {
		where = "schema in `format` and in the instruction"
	}
	return fmt.Sprintf(
		"mutation — one transformation instance removed from a working config, "+
			"workspaces/*/pipes_config/**, 1 file in %d held out, planned against %d tokens, "+
			"served at num_ctx %d, temperature %g seed %d, %s",
		everyNth, planBudget, numCtx, temperature, seed, where)
}

// transformationSpecNames maps an operator to the $defs entry a hole should
// address, which is its **bundle** and not the flat leaf.
//
// **Getting this wrong is the whole difference between a harness that runs and
// one that refuses every case, and the first version of this file got it
// wrong (F114).** The contract's own discriminator maps `map_record` to
// `TransformationSpecMapRecord`, which reads like the answer and is not: that
// leaf's `conditional_config` reaches `ConditionalTransformationSpec`, which
// re-admits every operator's config, so its closure is 61 definitions and
// ~28,766 tokens and `prompt.Fits` refuses it. `MapRecordPipe` is the same
// shape with the same eight properties and the same two required — and its
// `conditional_config` is ranged to its own bundle, so the closure is 25
// definitions and ~11,943 tokens and fits with room to spare.
//
// The mapping is read from `bundle_members.csv` rather than derived, and the
// derivation that looks available is a trap. Every bundle today carries a
// `type` const equal to its operator's token, so a scan would work — and it
// would work only because the tier is 1:1 with the leaves, which
// BUNDLES_SCHEMA.md names as a deliberate current choice that forecloses a
// hole offering a choice among related operators. The first bundle grouping
// two operators breaks the scan and not the CSV, and the CSV is
// authoritative by design because it expresses semantics the model cannot.
// An operator with no bundle row falls back to the leaf and is reported as
// doing so — `embed` is the one, added after the bundle layer was authored
// and never given a row (F119).
func transformationSpecNames(contract []byte, bundlePath string) (map[string]string, error) {
	var doc struct {
		Defs map[string]struct {
			Discriminator struct {
				Mapping map[string]string `json:"mapping"`
			} `json:"discriminator"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(contract, &doc); err != nil {
		return nil, fmt.Errorf("parsing the cpipes contract: %w", err)
	}
	mapping := doc.Defs["TransformationSpec"].Discriminator.Mapping
	if len(mapping) == 0 {
		return nil, errors.New("the contract's TransformationSpec has no discriminator mapping, " +
			"so there is no way to say which $defs entry an operator's spec is")
	}
	out := make(map[string]string, len(mapping))
	for op, ref := range mapping {
		out[op] = strings.TrimPrefix(ref, "#/$defs/")
	}
	bundles, err := bundleMembers(bundlePath)
	if err != nil {
		return nil, err
	}
	var defs struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(contract, &defs); err != nil {
		return nil, err
	}
	for op := range out {
		b, ok := bundles[op]
		if !ok {
			continue
		}
		if _, ok := defs.Defs[b]; !ok {
			// The CSV names a bundle the contract does not define. Say so
			// rather than silently falling back: the two are meant to be
			// regenerated together.
			return nil, fmt.Errorf("bundle_members.csv maps %q to %q and the contract has no such "+
				"$defs entry; the matrix and the schema are out of step", op, b)
		}
		out[op] = b
	}
	return out, nil
}

// bundleMembers reads the (bundle, type_token) pairs, keyed by operator.
func bundleMembers(path string) (map[string]string, error) {
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
		if i == 0 || len(r) < 2 {
			continue
		}
		bundle, token := r[0], r[1]
		// Only the TransformationSpec tier: the column bundles share their
		// tokens with the pipe operators (`select`, `value`, `case`) and
		// would overwrite them.
		if !strings.HasSuffix(bundle, "Pipe") {
			continue
		}
		out[token] = bundle
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s named no operator bundles", path)
	}
	return out, nil
}

// planOperators builds one sub-schema per operator the corpus holds and records
// why an operator cannot be asked when it cannot.
func planOperators(contract []byte, specNames map[string]string, live map[string]int,
	planBudget int) (map[string]json.RawMessage, map[string]string) {

	schemas := map[string]json.RawMessage{}
	refusals := map[string]string{}
	for op := range live {
		name, ok := specNames[op]
		if !ok {
			refusals[op] = "the contract's discriminator has no $defs entry for it"
			continue
		}
		sub, err := prompt.Subschema(contract, name)
		if err != nil {
			refusals[op] = fmt.Sprintf("its sub-schema could not be built: %v", err)
			continue
		}
		if err := prompt.Fits(sub, planBudget, 0); err != nil {
			refusals[op] = fmt.Sprintf("prompt.Fits refused it — %s is ~%d tokens against a "+
				"%d-token plan budget with %d reserved", name, prompt.EstimateTokens(sub),
				planBudget, prompt.DefaultReserveTokens)
			continue
		}
		schemas[op] = sub
	}
	return schemas, refusals
}

// instructionFor describes the hole without sending the whole config through
// it.
//
// **The config does not fit beside the schema and that is not a tuning
// problem.** A held-out config runs to tens of kilobytes and the operator's
// schema already takes most of the window, so the instruction carries the
// enclosing step — its channels, and its siblings reduced to their type — with
// the hole marked. A run that sent the whole document would measure a different
// task, so what was sent is recorded rather than left to be inferred.
func instructionFor(c *eval.Case, numCtx int, inPrompt json.RawMessage) (string, error) {
	// The served window is what the instruction competes for, and what else is
	// in it depends on where the schema goes: as `format` it costs nothing
	// (F113), in the instruction it costs its own size and the budget for the
	// context abstract shrinks by exactly that.
	budgetTokens := prompt.DefaultReserveTokens / instructionBudgetShare
	if numCtx > 0 {
		remaining := numCtx - prompt.EstimateTokens(inPrompt) - prompt.DefaultReserveTokens
		if remaining < budgetTokens {
			budgetTokens = remaining
		}
	}
	if budgetTokens < 256 {
		return "", fmt.Errorf("eval: %s: nothing left of the window for an instruction", c.File)
	}

	sketch, err := stepSketch(c)
	if err != nil {
		return "", err
	}
	rendered, kind := sketch, "the enclosing step, with its siblings reduced to their type"
	if prompt.EstimateTokens(rendered) > budgetTokens {
		rendered, err = json.Marshal(siblingTypes(c))
		if err != nil {
			return "", err
		}
		kind = "the sibling transformation types only — the enclosing step did not fit the window"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "A JetStore Compute Pipes configuration has one transformation removed from an "+
		"`apply` array. Write the transformation that belongs there.\n\n")
	fmt.Fprintf(&b, "File: %s\n", c.File)
	fmt.Fprintf(&b, "Operator type required: %q\n", c.Operator)
	fmt.Fprintf(&b, "Position in the apply array: %d\n\n", c.Hole.Index)
	fmt.Fprintf(&b, "Context (%s):\n%s\n\n", kind, rendered)
	if len(inPrompt) > 0 {
		fmt.Fprintf(&b, "This is the type you must produce, as a JSON Schema:\n\n```json\n%s\n```\n\n",
			inPrompt)
	}
	b.WriteString("Answer with the single transformation object. Its `type` must be exactly the " +
		"operator named above. Reference only channels and columns the context shows.")
	return b.String(), nil
}

// stepSketch is the object holding the `apply` array, with the array replaced
// by its siblings' types and a marker at the hole.
func stepSketch(c *eval.Case) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(c.Context, &doc); err != nil {
		return nil, err
	}
	if len(c.Hole.Path) == 0 {
		return nil, errors.New("the case has no path to a hole")
	}
	node := doc
	for _, s := range c.Hole.Path[:len(c.Hole.Path)-1] {
		switch {
		case s.Key != "":
			m, ok := node.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected an object at %q", s.Key)
			}
			node = m[s.Key]
		default:
			arr, ok := node.([]any)
			if !ok || s.Index >= len(arr) {
				return nil, fmt.Errorf("no element %d on the path to the hole", s.Index)
			}
			node = arr[s.Index]
		}
	}
	step, ok := node.(map[string]any)
	if !ok {
		return nil, errors.New("the hole's enclosing node is not an object")
	}
	out := map[string]any{}
	for k, v := range step {
		if k == c.Hole.Path[len(c.Hole.Path)-1].Key {
			out[k] = siblingTypes(c)
			continue
		}
		out[k] = v
	}
	return json.MarshalIndent(out, "", "  ")
}

// siblingTypes is the apply array as a list of operator types with the hole
// marked — the smallest description of where the answer goes.
func siblingTypes(c *eval.Case) []any {
	var doc any
	if err := json.Unmarshal(c.Context, &doc); err != nil {
		return nil
	}
	holder, err := navigateTo(doc, c.Hole.Path)
	if err != nil {
		return nil
	}
	arr, ok := holder.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(arr)+1)
	for i := 0; i <= len(arr); i++ {
		if i == c.Hole.Index {
			out = append(out, map[string]any{"MISSING — write this one": c.Operator})
		}
		if i < len(arr) {
			t := "?"
			if m, ok := arr[i].(map[string]any); ok {
				if s, ok := m["type"].(string); ok {
					t = s
				}
			}
			out = append(out, map[string]any{"type": t})
		}
	}
	return out
}

func navigateTo(node any, path []eval.Step) (any, error) {
	cur := node
	for _, s := range path {
		if s.Key != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected an object at %q", s.Key)
			}
			cur = m[s.Key]
			continue
		}
		arr, ok := cur.([]any)
		if !ok || s.Index >= len(arr) {
			return nil, fmt.Errorf("no element %d", s.Index)
		}
		cur = arr[s.Index]
	}
	return cur, nil
}

// verifierArgsFor is the wiring the corpus API had no way to express until
// Case.Fill: the gate is the startup validator over a whole config, and the
// model's answer is one transformation.
func verifierArgsFor(c *eval.Case) func(json.RawMessage) (json.RawMessage, error) {
	return func(artifact json.RawMessage) (json.RawMessage, error) {
		filled, err := c.Fill(artifact)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]json.RawMessage{"config": filled})
	}
}

// groundTruthValidates asks whether a case can be won at all: fill the hole
// with the instance that was removed and see whether the verifier accepts it.
func groundTruthValidates(c *eval.Case, registry *tools.Registry) (bool, error) {
	args, err := verifierArgsFor(c)(c.Expected)
	if err != nil {
		return false, fmt.Errorf("%s: filling the hole with its own ground truth: %w", c.File, err)
	}
	out, err := registry.Call(context.Background(), nil, "validate_cpipes_config", args)
	if err != nil {
		return false, fmt.Errorf("%s: the verifier failed: %w", c.File, err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return false, err
	}
	var verdict agent.Verdict
	if err := json.Unmarshal(encoded, &verdict); err != nil {
		return false, fmt.Errorf("%s: the verifier's report does not satisfy the verdict contract: %w",
			c.File, err)
	}
	return verdict.Valid, nil
}

// selfCheckCases puts every case's own ground truth back through Fill and the
// verifier, with no model in the loop.
//
// **It exists because the first full run never reached the verifier.** Every
// one of the 134 proposals failed client-side schema validation, so
// validate_cpipes_config was called zero times and the wiring from an answer to
// a verdict — Fill, the argument shape, the registry dispatch — went entirely
// unexercised by the run that was supposed to exercise it. Feeding the removed
// instance back is the strongest available check on that path short of a model
// that can write one: the answer is known-correct by construction, so a case
// that fails here is a broken case rather than a bad answer.
func selfCheckCases(root string, candidates []eval.Instance, refusals map[string]string,
	registry *tools.Registry, verbose bool) error {

	files := map[string][]byte{}
	ok, bad := 0, 0
	byOp := map[string]int{}
	for _, inst := range candidates {
		if refusals[inst.Operator] != "" {
			continue
		}
		raw, seen := files[inst.File]
		if !seen {
			var err error
			raw, err = os.ReadFile(filepath.Join(root, inst.File))
			if err != nil {
				return fmt.Errorf("reading %s: %w", inst.File, err)
			}
			files[inst.File] = raw
		}
		c, err := eval.MakeCase(raw, inst)
		if err != nil {
			return fmt.Errorf("making a case from %s: %w", inst.File, err)
		}
		args, err := verifierArgsFor(c)(c.Expected)
		if err != nil {
			return fmt.Errorf("%s: filling the hole with its own ground truth: %w", inst.File, err)
		}
		out, err := registry.Call(context.Background(), nil, "validate_cpipes_config", args)
		if err != nil {
			return fmt.Errorf("%s: the verifier failed: %w", inst.File, err)
		}
		encoded, err := json.Marshal(out)
		if err != nil {
			return err
		}
		var verdict agent.Verdict
		if err := json.Unmarshal(encoded, &verdict); err != nil {
			return fmt.Errorf("%s: the verifier's report does not satisfy the verdict contract: %w",
				inst.File, err)
		}
		if verdict.Valid {
			ok++
			byOp[inst.Operator]++
			continue
		}
		bad++
		fmt.Printf("  %-18s %-44s ground truth does not validate\n", inst.Operator, short(inst.File))
		if verbose {
			for _, d := range verdict.Diagnostics {
				fmt.Printf("      %s\n", strings.TrimSpace(string(d)))
			}
		}
	}
	fmt.Printf("\nself-check: %d of %d cases validate when filled with their own ground truth\n",
		ok, ok+bad)
	for _, op := range sortedKeys(byOp) {
		fmt.Printf("  %-18s %d\n", op, byOp[op])
	}
	fmt.Println("\nNo compile-pass figure is published from a self-check: no model was called, and " +
		"the answer was known-correct by construction.")
	if bad > 0 {
		return fmt.Errorf("%d case(s) do not validate with their own ground truth", bad)
	}
	return nil
}

// inventory prints what a run would attempt, and publishes no figure. A report
// with no model behind it is exactly the thing decision 13's rules refuse.
func inventory(root string, candidates []eval.Instance, coverage, live map[string]int,
	refusals map[string]string) error {

	askable, refused := 0, 0
	for _, inst := range candidates {
		if refusals[inst.Operator] != "" {
			refused++
			continue
		}
		askable++
	}
	fmt.Printf("held-out instances: %d\n", len(candidates))
	fmt.Printf("  askable at this context length: %d\n", askable)
	fmt.Printf("  refused before any model call:  %d\n\n", refused)
	fmt.Printf("%-18s %8s %8s  %s\n", "operator", "live", "held out", "status")
	for _, op := range sortedKeys(live) {
		status := "askable"
		switch {
		case live[op] == 0:
			status = "untested — no live instances in the corpus"
		case refusals[op] != "":
			status = "refused — " + refusals[op]
		case coverage[op] == 0:
			status = "the split held out no instance of it"
		}
		fmt.Printf("%-18s %8d %8d  %s\n", op, live[op], coverage[op], status)
	}
	fmt.Println("\nNo compile-pass figure is published from a dry run: no model was called, and a " +
		"report that cannot name what answered is one decision 13's rules refuse.")
	_ = root
	return nil
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func totalOf(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// truncateJSON keeps a ground-truth instance readable beside an answer without
// printing a kilobyte of column mappings.
func truncateJSON(raw json.RawMessage, n int) string {
	s := strings.TrimSpace(string(raw))
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("… (%d bytes)", len(s))
}

func short(file string) string {
	return filepath.Join(filepath.Base(filepath.Dir(filepath.Dir(file))), filepath.Base(file))
}
