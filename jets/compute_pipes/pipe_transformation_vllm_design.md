# VllmTransformationPipe — design

Compute Pipes operator that calls a vLLM server's OpenAI-compatible api once per input
record and augments that record in place with values extracted from the model's response.

Implementation: `pipe_transformation_vllm.go`, transformation type `vllm`, configuration
`vllm_config` (`VllmSpec`).

**It is the ollama operator's sibling, not a fork of it.** The operator shell, the worker
pool, the prompt template, the response mapping, the retry policy, the cost guard and the
`on_error` handling are the shared inference plumbing in `pipe_transformation_infer.go`,
reached through the `inferBackend` seam. **This document does not repeat any of it** —
[`pipe_transformation_infer_readme.md`](pipe_transformation_infer_readme.md) is where the
shared half is documented. What is here is what is different: the OpenAI request and
response shapes, the translation of `response_format` into what vLLM constrains generation
with, and the two failure modes that are this backend's own.

> **Do not read the shared half out of the ollama design doc.** That document was written
> on 2026-08-10 and the seam was extracted on 2026-08-16, so it describes a
> `pipe_transformation_ollama.go` that held the pool, the templating and the mapping. That
> file is **339 lines and holds only the backend** now. The ollama doc carries a note
> saying so; the parts that moved are here and in the infer readme.

**Most documents should not name this operator at all.** `type: infer` with
`infer_config.backend` resolves to `ollama` or `vllm` at startup, which is what lets one
document serve both — see
[`pipe_transformation_infer_readme.md`](pipe_transformation_infer_readme.md).

## 1. What is the same, in one table

Everything below is the shared plumbing, documented in
[`pipe_transformation_infer_readme.md`](pipe_transformation_infer_readme.md) — identical
for `ollama`, `vllm` and `embed` because it is one implementation rather than three.

| Concern | Where to read it |
|---|---|
| In-place augmentation; input and output must share one `ChannelSpec` | infer readme §2 |
| Prompt templating: `$ENV` at build time, `{{column}}` and `{{@record}}` per record | vllm prompt doc §2 |
| `output_mapping`: `source`, `path`, `as_rdf_type`, `default`, `required` | infer readme §2 |
| Worker pool, `pool_size`, order preservation at 1 | infer readme §2 |
| Retry with doubling backoff; circuit breaker on a server reported down | infer readme §2 |
| `max_input_count` cost guard, `on_error`, `max_error_count`, error channel | infer readme §2 |
| URL resolution: `server.url` → `$JETS_INFER_URL` → `JETS_INFER_URL` | infer readme §2, **and §4 below** |

**Only the last row has a twist for this backend**, and it is the one most likely to cost
an afternoon: the fallback points at Ollama.

## 2. Configuration

### `VllmSpec`

Only the keys that are this backend's own are listed. Every other key is
`InferCommonSpec`, embedded anonymously so the wire shape is identical to
`ollama_config`'s — `prompt_template`, `system_prompt`, `response_format`,
`output_mapping`, `pool_size`, `request_timeout_sec`, `on_error`, `error_channel` and the
rest all mean exactly what they mean there.

| Key | Default | Meaning |
|---|---|---|
| `model` | *required* | The model the server serves, e.g. `granite4.1:3b` |
| `api` | `chat` | `chat` → `/v1/chat/completions`, `completions` → `/v1/completions` |
| `structured_output` | `json_schema` | Which arm carries a schema: `guided_json` or `json_schema` |
| `options` | — | Sampling parameters, **merged into the request body at the top level** |
| `server.url`, `server.headers` | — | As ollama |

**Three keys of `ollama_config` do not exist here**, because the OpenAI api has no
equivalent: `keep_alive`, `think`, and ollama's nesting of sampling parameters under
`options`. The `infer` operator logs a line when a document carries one against the vLLM
backend rather than refusing it — see the infer readme.

### `api`, and why a system prompt constrains it

`chat` is the default because an instruct model needs its chat template, and the prompt
templates this operator is configured with are instructions.

**A `system_prompt` with `api: completions` is a build-time error.** `/v1/completions` has
no message roles, so the system prompt has nowhere to go; folding it into the prompt would
be a silent reinterpretation of what the author wrote.

### `options` is top-level, and the reserved keys are refused

This is the OpenAI api's shape rather than a choice: `temperature`, `max_tokens`, `top_p`
and vLLM's own extensions are **peers of `model`**, not members of a nested object. So the
`options` map is merged into the request body rather than nested in it.

The keys the operator sets itself are **refused at build time**:

```
model, stream, messages, prompt, guided_json, response_format
```

Refused rather than merged, because either precedence is wrong: letting the config win
silently redirects the operator to another model, and letting the operator win silently
discards what the author wrote.

```
error: vllm_config options cannot set 'model', the operator sets it itself (reserved:
model, stream, messages, prompt, guided_json, response_format). Sampling parameters such
as temperature, max_tokens and top_p are what options is for
```

## 3. `guided_json` is not `format`

**This is the whole of the request difference between the two backends.**

Ollama takes a schema in `format` and treats it as best effort. vLLM constrains the
decoder itself, and the field it takes the schema in depends on which arm the server
supports. The operator's configuration is the **same `response_format` property the ollama
operator has**, and the translation happens once at build time — so a `.pc.json` moves
between the two operators by changing the type token and the config element, not by
rewriting the schema.

| `response_format` | `structured_output` | What goes on the wire | `constrained` |
|---|---|---|---|
| absent | — | nothing | false |
| `"json"` | — | `response_format: {"type":"json_object"}` | true |
| a schema document | `json_schema` *(default)* | `response_format: {"type":"json_schema","json_schema":{"name":"response","schema":…,"strict":true}}` | true |
| a schema document | `guided_json` | `guided_json: …` | true |

`strict: true` is what makes the `json_schema` arm the equivalent of `guided_json` rather
than a hint. `name` is the constant `response`: the OpenAI shape requires one and nothing
reads it back.

**Which arm a server accepts is a property of its vLLM version rather than of the
pipeline**, which is why this is configurable at all.

### The default is `json_schema`, and it was measured rather than preferred

**vLLM v0.28.0 — the version `Dockerfile.infer_service_vllm` pins — accepts `guided_json`
and discards it.** At temperature 0 with a fixed seed, the same request with and without
the field returns a byte-identical answer, sha256 and all, while
`response_format: {"type":"json_schema"}` carrying the same schema returns a conformant
one. Measured at **0 of 24 against 23 of 24** (agentic_ai `AG.2`).

**It is not specific to that field.** `guided_regex`, `guided_choice` and an invented field
all return 200 with an unconstrained answer, so this server **discards unknown top-level
request fields as a class and says nothing**. A default that is inert is worse than one
that is wrong, because the operator reports success on an unconstrained answer.

**This default is version-dependent, and that is a real cost rather than a caveat.** It is
correct against the pinned image and would be wrong against a version where the
OpenAI-compatible arm is the weaker one. An author who needs the other arm sets
`structured_output` explicitly, which is why this is a default rather than a removal of the
`guided_json` path. *Detecting the discard rather than defaulting around it is `Q-82`.*

## 4. The two failure modes that are this backend's own

### A truncated constrained generation is a row-level error

With the decoder constrained to a schema, a response that stopped at the token ceiling is
**invalid json by construction**. The shared plumbing would report that as a parse failure
suggesting `response_format` — which is already set, and is not the problem.

So the backend checks `finish_reason` and says what actually happened:

```
error: the model stopped at the token ceiling with generation constrained by a schema, so
the response is truncated and cannot be valid json; raise max_tokens in
vllm_config.options.
```

It **fails the record rather than the attempt**: retrying the same call reproduces it.

And it fires **only when generation was constrained** — an unconstrained call producing
free text may legitimately be cut short, so the same `finish_reason` is not an error there.

### A 404 is almost always the url, and the operator says so

**`JETS_INFER_URL` points at Ollama in the deployed stack.** URL resolution is shared with
the ollama and embed operators, and the deployment sets that variable to the infer service,
which serves Ollama on 11434 (`build_infer_service.go`). **Ollama answers `/api/*`, not
`/v1/*`.**

So a `vllm` operator relying on the fallback reaches a live server that has never heard of
the route, and a bare `404 Not Found` reads like a missing model rather than a misdirected
operator. `vllmExplainError` annotates it:

```
(note: <url> is an OpenAI-compatible route that only a vLLM server serves - check that
vllm_config.server.url points at vLLM and not at the Ollama infer server, which
JETS_INFER_URL names and which answers /api/* instead)
```

**`server.url` is therefore in practice required for this operator today.**

## 5. Response handling

`vllmApiResponse` covers both routes: `choices[].message.content` for chat,
`choices[].text` for completions. It implements `inferResponse`, so every `output_mapping`
`source` behaves as it does for ollama — with one difference worth knowing.

**`source: envelope` reads OpenAI's envelope, not Ollama's.** The token counts are
`usage.prompt_tokens` and `usage.completion_tokens`, not `prompt_eval_count` and
`eval_count`. A mapping copied between the two backends without changing the path will
resolve to nothing, and — per the shared mapping rule — **leave the column as it was**
rather than fail. Set `required: true` on a token-count mapping if that matters.

Three responses are caught before the mappings run, so the message names the cause rather
than a missing path:

- **an error reported with a 2xx**, the way ollama reports an unknown model;
- **an empty `choices` list** — a successful response the mappings cannot use;
- **a body that will not parse**, reported with the first 500 bytes.

## 6. Build-time validation

In the constructor, in addition to everything the shared builder checks:

- `model` non-empty; `api` in range.
- `system_prompt` requires `api: chat`.
- `response_format` is the string `"json"` or a json schema document; `structured_output`
  is `guided_json` or `json_schema`.
- No `options` key collides with the reserved list.
- A resolvable server url.

**The request base is built once, at build time**, by `vllmRequestBase` — not per record.
The cost saving is secondary; what matters is that an unusable `response_format` or a
colliding option becomes a **build-time error**, which is where every other configuration
failure of this operator is reported.

**This is why the backend implements `inferBackendPreparer`.** `prepare()` runs after every
step that writes to `InferCommonSpec` has run — including the one that adopts a named
template's `response_format`. Building the base in the constructor instead would read an
empty `response_format` and send an unconstrained request: the model would be free to omit
a required field and free to keep generating, which is a mapping error on some records and
a timeout across enough of them. **The ollama backend has no equivalent** because it reads
`config.ResponseFormat` per request, by which time the adoption has happened.

## 7. Integration points

| File | Change |
|---|---|
| `pipes_model.go` | `VllmSpec`; `TransformationSpec.VllmConfig`. `OllamaServerSpec` and `InferMappingSpec` are reused rather than duplicated |
| `pipe_transformation_infer.go` | The shared plumbing and the `inferBackend` / `inferBackendPreparer` seams |
| `pipe_transformation_vllm.go` | The backend |
| `resolve_infer_backend.go` | `type: infer` → this operator when the backend is `vllm` |
| `pipes_runtime_model.go` | `case "vllm"` in `BuildPipeTransformationEvaluator` |
| `actions_start_common.go` | `case "vllm"` validation; error channel registration is shared and needs nothing per backend |
| `dockerfiles/Dockerfile.infer_service_vllm` | The deployable image, with a pinned vLLM version |

## 8. Example

```json
{
  "type": "vllm",
  "vllm_config": {
    "model": "granite4.1:3b",
    "server": { "url": "http://vllm-host:8000" },
    "structured_output": "json_schema",
    "options": { "temperature": 0, "seed": 42, "max_tokens": 512 },
    "system_prompt": "You are a claims classification assistant.",
    "prompt_template_name": "classify_claim",
    "response_format": {
      "type": "object",
      "properties": {
        "category":   { "type": "string" },
        "confidence": { "type": "number" }
      },
      "required": ["category", "confidence"]
    },
    "pool_size": 4,
    "row_key_column": "claim_id",
    "output_mapping": [
      { "column": "claim_category",   "path": "category" },
      { "column": "claim_confidence", "path": "confidence", "as_rdf_type": "double" },
      { "column": "infer_tokens",     "source": "envelope", "path": "usage.completion_tokens", "as_rdf_type": "int" }
    ],
    "error_channel": { "name": "process_errors.out", "channel_spec_name": "process_errors" }
  },
  "output_channel": { "name": "claims.out", "channel_spec_name": "claims" }
}
```

`server.url` is set explicitly rather than left to `JETS_INFER_URL` — see §4.

See [`pipe_transformation_vllm_prompt.md`](pipe_transformation_vllm_prompt.md) for the
exact body this puts on the wire.

## 9. Deliberately not built

- **Detecting a discarded guided-decoding field.** The `AG.2` measurement says a server can
  accept and ignore one silently; the operator defaults around that rather than detecting
  it. `Q-82`.
- **Streaming.** The operator needs the complete response before it can map it.
- **Embeddings.** `/v1/embeddings` produces a vector rather than a set of columns; that is
  the `embed` operator.
- **Choosing `structured_output` from the server's version.** The server reports its
  version, and reading it would remove the version-dependent default of §3. It is not done
  because the probe belongs with the detection of `Q-82` rather than beside it.
