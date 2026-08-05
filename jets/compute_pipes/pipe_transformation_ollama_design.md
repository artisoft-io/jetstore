# OllamaTransformationPipe — design

Compute Pipes operator that calls the Infer Server (Ollama) once per input record and
augments that record in place with values extracted from the model's response.

Implementation: `pipe_transformation_ollama.go`, transformation type `ollama`,
configuration `ollama_config` (`OllamaSpec`).

## 1. What the operator does

For each record arriving on the input channel:

1. Render a prompt from a template, substituting values from the record's columns.
2. POST it to the infer server (`/api/generate` or `/api/chat`, never streaming).
3. Extract values from the response with dot-notation paths and write them into
   **the same record**.
4. Forward that record to the output channel.

This is an *augmentation* pattern: nothing is copied, no new record is built. The
consequence — and the strongest constraint on the configuration — is that the input and
output channels must share one `ChannelSpec`, i.e. be declared with the same
`channel_spec_name`. The operator verifies this at build time rather than trusting it,
because a mismatch would otherwise show up as values landing in the wrong columns.

Row-level failures do not stop the pipeline: they are reported to an error channel
(the `process_errors` shape, as in the jetrules operator) and the row is passed through,
dropped, or escalated according to `on_error`.

## 2. Configuration

Added to `pipes_model.go`:

- `TransformationSpec.OllamaConfig *OllamaSpec` (`ollama_config`)
- `ComputePipesConfig.PromptTemplates []PromptTemplateSpec` (`prompt_templates`) — a
  top-level registry alongside `lookup_tables` and `schema_providers`, so a template can
  be shared by several steps and pipes.

### `OllamaSpec`

| Key | Default | Meaning |
|---|---|---|
| `model` | *required* | Ollama model tag, e.g. `llama3.1:8b` |
| `api` | `generate` | `generate` or `chat` |
| `prompt_template` | — | Inline template; mutually exclusive with `prompt_template_name` |
| `prompt_template_name` | — | Key into `prompt_templates` |
| `system_prompt` | — | System message |
| `response_format` | — | Ollama `format`: `"json"` or a JSON schema; raw passthrough |
| `options` | — | Ollama `options`: `temperature`, `num_ctx`, `seed`, `num_predict`… raw passthrough |
| `keep_alive` | `30m` | Keeps the model resident between rows — the single most important throughput setting |
| `think` | — | Reasoning models |
| `server.url` | — | See URL resolution below |
| `server.headers` | — | Extra request headers |
| `output_mapping` | *required* | Response → column mapping, see below |
| `disable_strip_code_fences` | false | Code fences (```` ```json ````) are stripped before parsing unless this is set |
| `pool_size` | 1 | Concurrent in-flight requests |
| `request_timeout_sec` | 120 | Per attempt |
| `connect_timeout_sec` | 10 | TCP + TLS handshake |
| `max_retry` | 2 | On timeout, connection error, 429 and 5xx. A pointer: unset means the default, an explicit `0` disables retries |
| `retry_wait_sec` | 2 | Doubled per attempt |
| `max_input_count` | 0 (unlimited) | Cost guard: past this count rows pass through **uncalled** |
| `on_error` | `pass_through` | `pass_through`, `drop`, or `fail` |
| `max_error_count` | 50 | Cap on rows written to the error channel |
| `row_key_column` | — | Column identifying the row in error reports (`row_jets_key`) |
| `is_debug` | false | Log prompt and response per row |
| `error_channel` | — | `{name, channel_spec_name}`, `process_errors` shape |

### URL resolution

`server.url` (after cpipes env substitution) → `$JETS_INFER_URL` in the cpipes env →
the `JETS_INFER_URL` OS environment variable — the same variable the apiserver's Infer
Server Admin screen uses. Absent all three, the operator fails at build time with a
message naming both configuration routes.

The operator never starts the infer server. `awsi.StartInferServer` exists and a pipeline
*could* call it, but auto-starting GPU capacity is a cost decision belonging to whoever
runs the pipeline, not to an operator. A stopped server fails fast with a message that
says so.

## 3. Prompt templating

Two substitution layers, each resolved as early as it can be:

**Env, once at build time.** `utils.ReplaceEnvVars` handles `$CLIENT`, `$SESSIONID`,
`${PERIOD_ID}` and anything declared under `context` — the same syntax every other
operator uses. The cpipes env is fixed for the life of a node, so re-resolving per row
would be pure waste.

**Columns, compiled at build time and rendered per row.** `{{column_name}}` placeholders
compile to a segment list (literal | column position) against the input channel's column
map; rendering is then a `strings.Builder` walk with no map lookups or regexes per row.

A `{{col}}` naming no column in the input channel is a **build-time error** listing the
available columns. This is deliberate: the alternative is discovering the typo after
spending GPU-seconds on a prompt with a hole in it.

One reserved placeholder, `{{@record}}`, expands to the whole record as a JSON object —
the shape most useful for prompting.

## 4. Response mapping

`output_mapping` entries:

| Key | Meaning |
|---|---|
| `column` | Output column to fill; must exist in the shared `ChannelSpec` |
| `source` | `response` (default, model text parsed as JSON when `path` is set), `raw_response` (text verbatim), `envelope` (a field of the Ollama API envelope: `eval_count`, `total_duration`, `model`…), `thinking` |
| `path` | Dot notation over the parsed JSON: `summary`, `codes.0.icd10`, `detail.score` |
| `as_rdf_type` | Cast via `CastToRdfType` |
| `default` | Used when the path is absent or null |
| `required` | Absent value ⇒ row-level error |

A path that resolves to nothing, with no `default` and not `required`, **leaves the column
as it was**. Clearing it would destroy the input value whenever a mapping targets an
existing column, and the usual case — a column added to the channel spec for the model to
fill — is null either way. A JSON object or array lands in the column as JSON text, since
a column value cannot be a map.

**Dot notation rather than JSONPath.** No JSONPath library is vendored, and a small
walker over `map[string]any` / `[]any` (a numeric segment indexes an array) covers what
the mapping needs. If a full expression language is ever wanted, it slots in behind the
same `path` field.

## 5. Runtime

**Always a worker pool, default size 1.** One code path instead of two: `Apply` hands the
record to `WorkersTaskCh` and returns; workers do the HTTP call, mutate their record, and
write it out. With `pool_size: 1` a single FIFO worker preserves record order; above 1,
order is not preserved and the config doc says so. The task channel is buffered at 1, for
back-pressure, as `JrPoolManager` does.

Each worker gets **its own** set of `spec.Columns` evaluators — those carry state and are
not safe to share across goroutines. They are all built eagerly in the constructor so a
bad column spec fails at build rather than inside a worker.

`Finally()` closes the task channel, waits for the pool, then closes the error channel —
mirroring the jetrules operator. The ordering matters: `StartFanOutPipe` calls `Finally()`
on every evaluator *before* its deferred block closes the output channels, so waiting here
is what keeps a worker from writing into a closed channel. It also logs the run summary
(rows, calls, errors, latency, token counts).

Per record, in the worker:

1. Past `max_input_count` → pass through untouched (a cost guard, not a filter).
2. Render prompt, build request (`stream:false`, model, format, options, keep_alive).
3. POST with a context cancelled by `ctx.done`, so an aborting pipeline does not leave
   rows blocked on a 120s timeout.
4. Retry with doubling backoff on timeout / connection error / 429 / 5xx; a 4xx fails the
   row immediately.
5. Extract text (`response` for generate, `message.content` for chat), strip code fences,
   parse JSON once, apply each mapping.
6. **Grow the record to the channel's column count with nils before assigning** — short
   rows are real in this codebase (`pad_short_rows_with_nulls` exists for that reason) and
   an in-place write past the end would panic.
7. Run the `spec.Columns` evaluators over the same record, so model output can be
   post-processed with the existing `case` / `hash` / `map` machinery.
8. Send the record to the output channel.

## 6. Build-time validation

In the constructor:

- `outputCh.Config == source.Config` (pointer equality — `compute_pipes.go` maps every
  channel name to the same `*ChannelSpec` instance, so this holds exactly when both
  channels name the same `channel_spec_name`). On failure it falls back to a name-by-name
  comparison and reports which columns diverge.
- Every `output_mapping.column` exists in the channel.
- Model non-empty; `api`, `on_error` in range; `pool_size >= 1`; a resolvable server URL;
  exactly one of `prompt_template` / `prompt_template_name`, and a named template must
  exist.

In `CpipesStartup.ValidatePipeSpecConfig` (`actions_start_common.go`), across every
operator of a step:

- **No two operators may declare the same error channel**, and an error channel name may
  not also be some operator's output channel. The operator that owns an error channel
  closes it in `Finally()`; a second writer would then panic on a closed channel, or lose
  its rows to a channel closed early. This covers `map_record`, `jetrules` and `ollama`
  alike. Verified against every `*.pc.json` in the workspaces repo: no existing config
  trips either rule.

## 7. Integration points

| File | Change |
|---|---|
| `pipes_model.go` | `OllamaSpec`, `OllamaServerSpec`, `OllamaMappingSpec`, `PromptTemplateSpec`; `TransformationSpec.OllamaConfig`; `ComputePipesConfig.PromptTemplates` |
| `pipes_runtime_model.go` | `case "ollama"` in `BuildPipeTransformationEvaluator` |
| `compute_pipes.go` | Register the error channel of every operator that has one — without it the channel never enters `channelsInUse` and `GetOutputChannel` fails at build |
| `actions_start_common.go` | `case "ollama"` validation, error channel validation for every operator that has one, and the step-wide error channel uniqueness check |
| `pipe_transformation_map_record.go` | Close the error channel in `Finally` (see below) |
| `pipe_transformation_ollama.go` | The operator |
| `pipe_transformation_ollama_test.go` | Tests |
| `cdk/jetstore_one/stack/build_ecs_tasks.go`, `build_cpipes_lambdas.go`, `build_elb.go` | Give the cpipes tasks and lambdas `JETS_INFER_URL` |

The fan_out and splitter executors need no change: the main output channel is declared in
`spec.OutputChannel` so it is already in their close set, and the error channel is closed
by `Finally()`.

Error channels are handled once for every operator rather than per operator type, keyed on
`errorChannelConfig` (`actions_start_common.go`) — registration, validation and the
uniqueness rule all read from that one list. This closed a latent gap in `map_record`,
whose error channel was neither registered (so `GetOutputChannel` would have failed at
build) nor closed (so the pipe reading it would have hung). No workspace config used it,
which is why it had gone unnoticed; `map_record` now closes it in `Finally` the way
`jetrules` and `ollama` do.

## 8. Deployment note

`JETS_INFER_URL` was set only on the UI container. Compute Pipes runs in ECS tasks and
Lambdas, which never saw it, so the operator would have required an explicit `server.url`
in every config. `build_elb.go` now sets it on the components that execute the operators
too — `CpipesContainerDef`, `CpipesNodeLambda`, and `CpipesNativeNodeLambda` when cpipes
native is deployed — inside the existing `DoBuildInferServer()` guard. `BuildELB` runs
last in `jetstore_one.go`, so all three exist by then.

## 9. Example

```json
{
  "prompt_templates": [
    {
      "key": "classify_claim",
      "template": "Classify the claim below.\nReturn JSON: {\"category\": <string>, \"confidence\": <0..1>}\n\nDiagnosis: {{diagnosis_text}}\nProcedure: {{procedure_code}}\n",
      "response_format": "json"
    }
  ],
  "channels": [
    { "name": "claims", "same_columns_as_input": true },
    { "name": "process_errors",
      "columns": ["pipeline_execution_status_key", "session_id", "grouping_key",
                  "row_jets_key", "input_column", "error_message",
                  "rete_session_saved", "rete_session_triples", "shard_id"] }
  ],
  "pipes_config": [
    {
      "type": "fan_out",
      "input_channel": { "name": "claims.in" },
      "apply": [
        {
          "type": "ollama",
          "ollama_config": {
            "model": "llama3.1:8b",
            "prompt_template_name": "classify_claim",
            "pool_size": 4,
            "request_timeout_sec": 90,
            "row_key_column": "claim_id",
            "output_mapping": [
              { "column": "claim_category",   "path": "category" },
              { "column": "claim_confidence", "path": "confidence", "as_rdf_type": "double" },
              { "column": "infer_tokens",     "source": "envelope", "path": "eval_count", "as_rdf_type": "int" }
            ],
            "error_channel": { "name": "process_errors.out", "channel_spec_name": "process_errors" }
          },
          "output_channel": { "name": "claims.out", "channel_spec_name": "claims" }
        }
      ]
    }
  ]
}
```

`claims.in` and `claims.out` both name `channel_spec_name: "claims"` — that shared spec is
what makes the in-place augmentation legal.

## 10. Deliberately not built

- **Prompt-hash response cache.** Repeated values in a column are common and a cache could
  cut GPU time by an order of magnitude, but it changes failure semantics (a cached error?
  a cached partial?) and deserves its own pass.
- **Batching several records per prompt.** Better tokens-per-row, much worse error
  attribution.
- **Embeddings (`/api/embed`).** Its output is a vector, not a set of columns; that wants a
  different operator.
