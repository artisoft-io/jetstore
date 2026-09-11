# The inference operators — shared plumbing, backend selection, and what we measured

Three Compute Pipes operators call a model server once per input record and augment that
record in place: `ollama`, `vllm` and `embed`. **They are one implementation with three
backends**, not three operators, and this document covers the half they share plus the
abstract `infer` type that chooses between them.

| Document | Covers |
|---|---|
| **this file** | The `infer` type and backend resolution; the shared plumbing; **the Phase 7 measurements and what they recommend** |
| [`pipe_transformation_ollama_design.md`](pipe_transformation_ollama_design.md) | The ollama backend: `keep_alive`, `think`, the generate/chat shapes |
| [`pipe_transformation_ollama_prompt.md`](pipe_transformation_ollama_prompt.md) | The ollama wire payloads, and the Infer Server Admin screen |
| [`pipe_transformation_vllm_design.md`](pipe_transformation_vllm_design.md) | The vLLM backend: guided decoding, the OpenAI shapes |
| [`pipe_transformation_vllm_prompt.md`](pipe_transformation_vllm_prompt.md) | vLLM prompt configuration and wire payloads |

---

## 1. Use `type: infer`, not the backend directly

**A document should name the abstract operator and let the deployment pick the server.**

```json
{
  "type": "infer",
  "infer_config": {
    "backend": "$INFER_BACKEND",
    "model": "granite4.1:3b",
    "prompt_template_name": "classify_claim",
    "output_mapping": [ { "column": "claim_category", "path": "category" } ]
  },
  "output_channel": { "name": "claims.out", "channel_spec_name": "claims" }
}
```

`ResolveInferBackend` rewrites every `type: infer` step into the concrete operator its
backend names, **before anything downstream sees it**. The validator, the executors and
`SelectActiveOutputTable` all see an `ollama` or a `vllm` step and need no knowledge that
the abstract type exists.

| `backend` | Resolves to |
|---|---|
| `"ollama"`, or **empty, or an unset env var** | `type: ollama`, `ollama_config` |
| `"vllm"` | `type: vllm`, `vllm_config` |
| anything else | build-time error naming the two |

**Empty means ollama deliberately**: that is what every document in the corpus was before
this type existed, so a deployment that says nothing keeps the behaviour it had.

`$INFER_BACKEND` is copied from the process environment by `shardingInitializeCpipes`. The
lookup is an **exact key match** rather than a substitution scan, so `$INFER_BACKEND`
cannot be partially matched by a longer key sharing its prefix.

### Ordering

Call it **after** `ApplyAllConditionalTransformationSpec` and **before**
`SynthesizeDefaultErrorChannels`. Not a preference: the conditional pass may introduce or
replace an infer step, and the error-channel synthesis dispatches on the operator type and
would not recognise `infer`.

### Keys that are inert on the chosen backend are logged, not refused

`keep_alive` and `think` are ollama's; `structured_output` is vLLM's. A document carrying
one against the other backend gets a log line:

```
infer_config: backend vllm does not read think, keep_alive; the key is inert on this backend
```

**Logged rather than refused**, because a document written to serve both backends is
expected to carry both operators' specialized keys, and refusing one would make the
abstraction useless. **Silence would be worse than either** — an author who misspells a
backend name and loses their structured output should be able to find out from the log.

The abstract config is **consumed rather than kept** beside its resolution: leaving it set
would give the document two states that can disagree.

## 2. The shared plumbing

`pipe_transformation_infer.go` holds everything that is not backend-specific, and **never
names a backend**. A backend supplies the `inferBackend` seam — `BuildRequest` and
`CallOnce` — plus the labels used in log and error messages, so the messages stay identical
to what each operator emitted before the extraction.

**This section is the operator.** The backend documents cover only the request and response
shapes on the wire.

### 2.1 What the operator does

For each record arriving on the input channel:

1. Render a prompt from a template, substituting values from the record's columns.
2. Call the model server, never streaming.
3. Extract values from the response with dot-notation paths and write them into **the same
   record**.
4. Forward that record to the output channel.

**This is an augmentation pattern: nothing is copied, no new record is built.** The
consequence — and the strongest constraint on the configuration — is that the input and
output channels must share one `ChannelSpec`, i.e. be declared with the same
`channel_spec_name`. The operator verifies this at build time by pointer equality rather
than trusting it, because a mismatch would otherwise show up as values landing in the wrong
columns. On failure it falls back to a name-by-name comparison and reports which columns
diverge.

Row-level failures do not stop the pipeline: they are reported to an error channel (the
`process_errors` shape, as in the jetrules operator) and the row is passed through, dropped,
or escalated according to `on_error`.

### 2.2 Configuration — `InferCommonSpec`

Every shared key lives in `InferCommonSpec`, embedded **anonymously** in `InferSpec`,
`OllamaSpec` and `VllmSpec` — so `encoding/json` field promotion gives all three the same
wire shape, and a shared key is one field rather than three copies of a list.

| Key | Default | Meaning |
|---|---|---|
| `prompt_template` | — | Inline template; mutually exclusive with `prompt_template_name` |
| `prompt_template_name` | — | Key into the top-level `prompt_templates` registry |
| `system_prompt` | — | System message |
| `response_format` | — | `"json"` or a JSON schema. **What each backend does with it differs** — see the backend docs |
| `output_mapping` | *required* | Response → column mapping, §2.5 |
| `disable_strip_code_fences` | false | Code fences are stripped before parsing unless this is set |
| `pool_size` | 1 | Concurrent in-flight requests |
| `request_timeout_sec` | 120 | Per attempt |
| `connect_timeout_sec` | 10 | TCP + TLS handshake |
| `max_retry` | 2 | On timeout, connection error, 429 and 5xx. **A pointer**: unset means the default, an explicit `0` disables retries |
| `retry_wait_sec` | 2 | Doubled per attempt |
| `max_input_count` | 0 (unlimited) | Cost guard: past this count rows pass through **uncalled** |
| `on_error` | `pass_through` | `pass_through`, `drop`, or `fail` |
| `max_error_count` | 50 | Cap on rows written to the error channel |
| `row_key_column` | — | Column identifying the row in error reports (`row_jets_key`) |
| `is_debug` | false | Log prompt and response per row |
| `error_channel` | — | `{name, channel_spec_name}`, `process_errors` shape |

**`on_error` records whether it was defaulted**, in an unexported field that never
round-trips through json. From the moment the default is applied, an unset `on_error` and an
explicit `on_error: pass_through` are the same string — and the difference decides whether a
stopped infer server may be overruled and take the pipeline down.

**`provenance_schema_name` also lives here** and is read by `infer_provenance.go`.

### 2.3 Where a prompt is declared

Two places, and **exactly one of them per operator**:

| Config | Where | Key |
|---|---|---|
| Inline | the operator's config element | `prompt_template` |
| Named | `ComputePipesConfig` (top level, beside `lookup_tables` and `schema_providers`) | `prompt_templates[].key`, referenced by `prompt_template_name` |

A named template is a `PromptTemplateSpec`:

```json
{
  "key": "classify_claim",
  "template": "Diagnosis: {{diagnosis}}\n",
  "system_prompt": "You are a claims classification assistant.",
  "response_format": "json"
}
```

`system_prompt` and `response_format` on the template are **defaults**; the operator's own
values win when both are set (`resolveInferTemplate`). The registry exists so several steps
and pipes can share one prompt; it carries the settings that belong with the prompt text
rather than with the step.

**That adoption is why the vLLM backend has a `prepare()` step and the others do not.** A
backend that reads `response_format` once, at build time, has to read it *after* the named
template has been adopted — see `inferBackendPreparer` and the vLLM design doc §6.

### 2.4 The two kinds of placeholder

| Syntax | Substituted from | When | Applies to |
|---|---|---|---|
| `$VAR`, `${VAR}` | the cpipes env | once, at build time | `prompt_template`, `system_prompt`, `server.url` |
| `{{column_name}}` | the record's value for that column | per record, from a compiled segment list | the prompt template only |
| `{{@record}}` | the whole record as a JSON object | per record | the prompt template only |

**Env vars resolve first, at build time.** `utils.ReplaceEnvVars` replaces each env key
wherever it appears, so the key must be written **exactly as registered, braces included** —
a template naming `$REQUEST_ID` does not pick up the env entry `${REQUEST_ID}`, and vice
versa. Substitution repeats until no `$` remains or five passes have run, so an env value may
itself contain env keys. The cpipes env is fixed for the life of a node, so re-resolving per
record would be waste.

The env carries `$FILE_KEY`, `$SESSIONID`, `$PROCESS_NAME`, `$PATH_FILE_KEY`,
`$NAME_FILE_KEY`, `$DATE_FILE_KEY`, `$FULL_INPUT_FILE_KEY`, `$INPUT_BUCKET`,
`$MAIN_SCHEMA_NAME`, `${REQUEST_ID}`, plus everything declared in the `context` section
(`PrepareCpipesEnv`, `actions_start_common.go`).

**Column placeholders compile at build time and render per record.** `{{col}}` becomes a
position in the input channel's column map; rendering is a `strings.Builder` walk with no map
lookup or regex per record. Whitespace inside the braces is trimmed, so `{{ diagnosis }}` and
`{{diagnosis}}` are the same placeholder.

**A `{{col}}` naming no column of the input channel is a build-time error** listing the
available columns (up to 40). This is deliberate: the alternative is discovering the typo
after spending GPU-seconds on a prompt with a hole in it.

**Only `{{` opens a placeholder**, so a JSON skeleton can be written into a template as-is.

Three asymmetries worth knowing:

- **`system_prompt` gets env substitution but not column substitution.** It is one string for
  the life of the node; a `{{col}}` left in it reaches the model verbatim.
- **`response_format` and `options` get no substitution at all.** They are raw JSON passed
  straight through.
- **Order matters, once.** Env substitution runs *before* the template is compiled, so an env
  value containing `{{...}}` is itself compiled as a placeholder — and fails the build if it
  names no column.

**An env placeholder matching no env key is not an error.** `ReplaceEnvVars` leaves unknown
`$…` text alone and it reaches the model as written. A prompt arriving at the server with a
literal `$CLIENT` in it means the key was never registered — check the `context` section and
the exact spelling, `$X` against `${X}`.

### 2.5 Response mapping

`output_mapping` entries:

| Key | Meaning |
|---|---|
| `column` | Output column to fill; must exist in the shared `ChannelSpec` |
| `source` | `response` (default, model text parsed as JSON when `path` is set), `raw_response` (text verbatim), `envelope` (a field of the server's response envelope), `thinking`, `model_name` |
| `path` | Dot notation over the parsed JSON: `summary`, `codes.0.icd10`, `detail.score` |
| `as_rdf_type` | Cast via `CastToRdfType` |
| `default` | Used when the path is absent or null |
| `required` | Absent value ⇒ row-level error |

**A path that resolves to nothing, with no `default` and not `required`, leaves the column as
it was.** Clearing it would destroy the input value whenever a mapping targets an existing
column, and the usual case — a column added to the channel spec for the model to fill — is
null either way. A JSON object or array lands in the column as JSON text, since a column
value cannot be a map.

**`source: envelope` is the one mapping that is backend-specific.** The envelope is the
server's, not the operator's: ollama's token counts are `eval_count` and `prompt_eval_count`,
vLLM's are `usage.completion_tokens` and `usage.prompt_tokens`. **A mapping copied between
backends without changing the `path` resolves to nothing and — by the rule above — silently
leaves the column alone.** Set `required: true` if that matters.

**Dot notation rather than JSONPath.** No JSONPath library is vendored, and a small walker
over `map[string]any` / `[]any` (a numeric segment indexes an array) covers what the mapping
needs. A full expression language would slot in behind the same `path` field.

### 2.6 Runtime

**Always a worker pool, default size 1.** One code path instead of two: `Apply` hands the
record to the pool's task channel and returns; workers do the call, mutate their record, and
write it out. With `pool_size: 1` a single FIFO worker preserves record order; **above 1,
order is not preserved**. The task channel is buffered at 1, for back-pressure.

Each worker gets **its own** set of `spec.Columns` evaluators — those carry state and are not
safe to share across goroutines. They are built eagerly in the constructor so a bad column
spec fails at build rather than inside a worker.

`Finally()` closes the task channel, waits for the pool, then closes the error channel. **The
ordering matters**: `StartFanOutPipe` calls `Finally()` on every evaluator *before* its
deferred block closes the output channels, so waiting here is what keeps a worker from
writing into a closed channel. It also logs the run summary — rows, calls, errors, latency,
token counts.

Per record, in the worker:

1. Past `max_input_count` → pass through untouched (a cost guard, not a filter).
2. Render the prompt and build the request through the backend's `BuildRequest`.
3. Call with a context cancelled by `ctx.done`, so an aborting pipeline does not leave rows
   blocked on a 120 s timeout.
4. Retry with doubling backoff on timeout / connection error / 429 / 5xx; **a 4xx fails the
   row immediately**.
5. Extract the text, strip code fences, parse JSON once, apply each mapping.
6. **Grow the record to the channel's column count with nils before assigning** — short rows
   are real in this codebase (`pad_short_rows_with_nulls` exists for that reason) and an
   in-place write past the end would panic.
7. Run the `spec.Columns` evaluators over the same record, so model output can be
   post-processed with the existing `case` / `hash` / `map` machinery.
8. Send the record to the output channel.

### 2.7 The circuit breaker

A server reported down opens a **30-second window** rather than having every remaining row
pay its full retry sequence. One record's exhausted retries are already proof enough about
the server, so the rest skip the call.

**It is not a latch, and that is a deliberate choice about the failure that actually
happens.** The infer server is an ECS service on a single GPU instance, so a deploy stops and
restarts it — and a pipeline running across that window would otherwise fail every record
after the outage, including the ones the server came back in time to answer. The cooldown
bounds the hammering without deciding the server is gone forever. There is no explicit close:
a probe that succeeds simply does not extend the window.

`noteServerDown` is monotonic under concurrency — several workers may exhaust their retries
at once, and the latest deadline wins because it is the one that learnt most recently.

This is why `on_error` remembering whether it was defaulted matters: a stopped server is the
one failure an invisible `pass_through` should not get to turn into a silently empty result.

### 2.8 URL resolution

`server.url` (after cpipes env substitution) → `$JETS_INFER_URL` in the cpipes env → the
`JETS_INFER_URL` OS environment variable — the same variable the apiserver's Infer Server
Admin screen uses. Absent all three, the operator fails at build time with a message naming
both configuration routes.

**`JETS_INFER_URL` names the Ollama infer service in the deployed stack**, which serves
Ollama on 11434. That is correct for the ollama and embed backends and **wrong for vLLM**,
which needs `server.url` — see the vLLM design doc §4.

The operator never starts the infer server. `awsi.StartInferServer` exists and a pipeline
*could* call it, but auto-starting GPU capacity is a cost decision belonging to whoever runs
the pipeline, not to an operator. A stopped server fails fast with a message that says so.

### 2.9 Build-time validation

In the shared builder:

- Input and output channels share one `ChannelSpec` (§2.1).
- Every `output_mapping.column` exists in that channel.
- Exactly one of `prompt_template` / `prompt_template_name`, a named template exists, the
  template is not empty, every `{{col}}` names a column, and every placeholder is terminated.
- `pool_size >= 1`; `on_error` in range; a resolvable server url.

Each backend adds its own — the model, the api, and whatever its request shape requires.

In `CpipesStartup.ValidatePipeSpecConfig` (`actions_start_common.go`), across every operator
of a step:

- **No two operators may declare the same error channel**, and an error channel name may not
  also be some operator's output channel. The operator that owns an error channel closes it
  in `Finally()`; a second writer would then panic on a closed channel, or lose its rows to a
  channel closed early. This covers `map_record`, `jetrules` and the inference operators
  alike.

Error channels are handled **once for every operator** rather than per operator type, keyed
on `errorChannelConfig` — registration, validation and the uniqueness rule all read from that
one list. That closed a latent gap in `map_record`, whose error channel was neither registered
nor closed; no workspace config used it, which is why it had gone unnoticed.

### 2.10 Seeing what happened

`is_debug: true` logs the fully rendered prompt and the raw response for **every** record:

```
OllamaTransformationPipe prompt: Classify the claim below. …
OllamaTransformationPipe response (412ms, 37 eval tokens): {"model":"granite4.1:3b", …}
```

Per record — so pair it with `max_input_count: 5` when debugging against real data. That caps
how many records reach the model at all, and the rest pass through untouched.

### 2.11 Deliberately not built

- **Prompt-hash response cache.** Repeated values in a column are common and a cache could cut
  GPU time by an order of magnitude, but it changes failure semantics (a cached error? a
  cached partial?) and deserves its own pass.
- **Batching several records per prompt.** Better tokens-per-row, much worse error
  attribution.

## 3. What we measured — agentic_ai Phase 7, 2026-09-09/10

**This section exists because these numbers are expensive to produce and easy to lose.**
They were measured against the `patient_profile` briefing pipeline over a **curated
22-member population**, and every figure below carries that scope.

**Read all of it as narrow.** One model at 3B, one pipeline, one population, and the runs
named per result. It licenses *this model, on this pipeline, over this population, did* —
never *a model cannot*. Full write-up: `projects/agentic_ai/plan/phase7_plan.md` §1.23 to
§1.28 in the `jetstore_agentic_ai` repository.

### 3.1 The backend choice is a cost and latency decision, not a quality one

**The strongest result in the set, and the most useful.** Three arms — vLLM bf16 eager,
vLLM bf16 with CUDA graphs, ollama `Q4_K_M` — over the same 22 members and the same fact
set:

> **Every *decidable* accuracy category is identical across all three.** The approximate
> ones vary by one to four, inside their own brackets and inside the run-to-run spread
> already measured within a single arm.

So the model's errors are a property of **the model and the prompt** rather than of the
serving stack — across a 4-bit and a 16-bit checkpoint, two servers and two execution
modes. **Choose a backend on throughput, latency and cold start.**

### 3.2 The two backends cross, and the crossing is at the pool the pipeline runs

| | vLLM (graphs) | ollama Q4 |
|---|---|---|
| pool 1, operator average latency | 1387 ms | **968 ms** |
| pool 1, throughput | 0.720 rec/s | **1.032 rec/s** |
| pool 2, throughput | 1.293 rec/s | **1.450 rec/s** |
| **pool 4, throughput** | **2.450 rec/s** | 2.161 rec/s |
| pool 1 → 4 scaling | **3.40x** | 2.09x |
| latency rise, pool 1 → 4 | **10%** | 79% |

ollama is 1.43x faster at pool 1, 1.12x at pool 2, and **vLLM is 1.13x faster at pool 4** —
which is the pool `patient_profile.pc.json` sets. A 4-bit model losing on throughput to a
bf16 one is **continuous batching earning its keep**.

**vLLM absorbs concurrency almost free and ollama does not**, which the latency row shows
without dividing anything.

**Removing `--enforce-eager` was worth 2.16x** at run time and cost only ~6 s at startup.
If you run vLLM, do not run it eager.

### 3.3 Cold start: vLLM 127 s, ollama ~20 s — and the biggest term was the disk

| Phase | Elapsed |
|---|---|
| backend invoked | 2.0 s |
| Python and torch import | 16.9 s |
| banner to EngineCore init | 16.8 s |
| **weight load, 6.34 GiB** | **51.0 s** |
| `torch.compile` | 18.1 s |
| KV cache profiling and graph capture | ~11 s |
| **total, container start to serving** | **127.1 s** |

Against ollama's ~19.6 s to a first answer. **vLLM is 6.5x slower to scale from zero.**

**But the dominant term was not the backend.** 6.34 GiB in 51.0 s is **124 MiB/s**;
ollama's ~2.1 GiB in 17.3 s is **~120 MiB/s**. Both backends read at the same rate, and
that rate was **gp3's unprovisioned baseline**. Neither server is slow at loading — the
volume was.

**Fixed 2026-09-10**: the infer persistent volume is provisioned at 500 MiB/s
(`INFER_VOLUME_THROUGHPUT_MIBPS`, `cdk/jetstore_one/stack/build_infer_ec2.go`), which takes
that 51 s to about 13 s. **The figures above are from before that change.**

The 127 s was also a **cold-cache** start: `torch.compile` writes to
`/jetsdata/.../torch_compile_cache`, so a later start should skip most of the 18.1 s.

### 3.4 Encode entities as JSON, not TOON

Measured over the same 22 members with **one entity map encoded both ways and both arms
scored against the same fact set** — the arms differ in the bytes sent and in nothing else.

| Category | TOON | JSON |
|---|---|---|
| maintenance misstated, drug level | 18 / 76 | **10 / 76** |
| conditions omitted | 26 / 92 | **16 / 92** |
| untrue of the fact set | 13 / 22 | **9 / 22** |
| adherence attributed to a drug carrying none | 13 / 76 | **6 / 76** |
| advisory language | **1 / 22** | 6 / 22 |
| **prompt tokens** | **30,394** | 31,255 |

**TOON's compactness is 2.8%** — 861 tokens over 22 records — and it costs accuracy on five
of nine categories. TOON wins one, advisory language, which is real and does not offset
five.

**The whole difference lives in the expanded stratum.** Where a record's lists carry
multi-valued fields or mixed key sets, the two diverge **eightfold** on maintenance errors;
where the record serialises tabular they are equivalent. And the economics run the wrong
way within one population: TOON saves 4.5% where it costs nothing and **1.2% where it costs
a great deal**.

**The mechanism is unexplained**, and is recorded as such rather than dressed in a story:
the expanded form is the *less* compact and *more* redundant of the two, which is not the
direction a compactness argument predicts.

`patient_profile.pc.json` was switched to `entity_encoding: json` on 2026-09-10.

### 3.5 A template beat the model, and that is the finding to carry

**The question was whether a model adds anything a template cannot write.** Over 22 members
and three runs, with both arms reading **one briefing node** and differing only in the
renderer:

> **The template produced a correct briefing for 22 of 22, in every run. The model produced
> at least one statement not true of the fact set for 10 of 22.**

The one thing the model did that a template cannot — join a medication list to a diagnosis
— occurred **twice in 66 member-runs** and was an unlicensed inference both times.

**The countable failure is one-directional.** Six members carry no maintenance drug at all;
the model described all six as being on maintenance medication in **17 of 18 member-runs**.
`N` becomes `Y` and never the reverse, across three runs, which is what makes it a property
rather than noise. The template's `{{if Maintenance == "Y"}}` was right 22 of 22.

**And it happened under the flag built to prevent it.** The briefing carries
*"Adherence_Ratio is applicable only for maintenance drugs"*, and that sentence was in the
prompt in **7 of the 7 cases** where the model misread the thing it explains. **A guardrail
on the input does not guard the reader.**

**A second one worth knowing about any numeric field you hand a model.** The adherence
ratio takes exactly two values over the whole population — `0.00` on 36 of 47 maintenance
drugs and `1.00` on the other 11, nothing between, an integer division. The model surfaced
it in **22 of 22** briefings and gave it an evaluative reading in **11 of 22**
(*"one hundred percent adherence"*, *"indicating no reported adherence"*). One member on
five maintenance drugs, filling most months of the year, was described as at 0%.

**The model is the reader who supplies the threshold.** A number that reads as a grade will
be graded.

### 3.6 Recommendations

1. **Name `type: infer` and set the backend from the deployment.** Everything in §3.1 says
   the document should not care which server runs.
2. **Prefer a template where the output is a projection of the input.** Ask what the model
   adds that a deterministic renderer cannot; if the answer is *phrasing*, the template is
   more accurate and free. **Run a deterministic control beside any model arm** — without
   one, §3.5 is not measurable.
3. **Encode entities as JSON** (§3.4) unless something specific argues otherwise.
4. **If you deploy vLLM: run it with CUDA graphs, set `pool_size` to 4 or more, and expect
   a slow scale-from-zero.** If the service idles at zero and wakes often, ollama's ~20 s
   against 127 s may outweigh 13% of throughput.
5. **Provision the model volume's throughput** before concluding a backend is slow to start
   (§3.3).
6. **Do not hand a model a derived number and expect it to be quoted rather than judged**
   (§3.5). Withhold it, or give it the inputs it is computed from.
7. **Set `response_format` and, on vLLM, leave `structured_output` at its default.** An
   accepted-and-discarded guided-decoding field returns an unconstrained answer with a 200
   and no warning — see the vLLM design doc §3.
8. **Do not read a passing corpus test as evidence without `-count=1`.** These pipelines
   read workspace files that live outside the Go module, so nothing in the test cache key
   changes when a workspace moves, and a stale `ok` is indistinguishable from a real one.

### 3.7 What these numbers do not establish

- **Not that TOON is a worse encoding in general** — one model, one population, one run.
- **Not that a model cannot write a briefing** — this model, this pipeline, this population
  did not.
- **Not a throughput claim without a stated concurrency.** The arms cross; any figure here
  is meaningless without its pool size.
- **Neither backend was measured at its best.** vLLM was tuned and ollama still ran without
  flash attention, which is the same unfairness reversed rather than removed.
- **The two arms did not serve identical weights.** Same model *name*, different artefacts —
  bf16 against `Q4_K_M`.
