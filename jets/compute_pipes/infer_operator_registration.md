# Registering a second inference operator — the 15b checklist

Written at the landing of 15a (2026-08-16), verified against the code the same
day. This is what a `vllm` (or any second) inference operator must touch, and
why that is not a 15a deficiency: every cpipes operator pays this —
`jetrules`, `clustering` and `map_record` sit in the same switches, so a
`vllm` arm alongside them is the convention rather than a fork.

**Exercised 2026-08-20 by the `embed` operator, and the six rows below were
right.** `pipe_transformation_embed.go` is ~340 lines of genuinely embeddings-
specific code, the six registration points cost about five lines each, and
nothing in `pipe_transformation_infer.go` had to change — the seam took a
backend whose response carries no text at all. What the checklist *understated*
is the last paragraph, the contract chain: see the note at the end.

**Exercised again 2026-09-04 by the `vllm` operator — the one this document was
written for — and the six rows were right a second time.**
`pipe_transformation_vllm.go` is ~430 lines, the five Go registration points
cost three to seven lines each, and `pipe_transformation_infer.go` is again
untouched: the seam took a backend whose request shape, guided-decoding field,
sampling-parameter placement and error envelope all differ from Ollama's.

Two things this run adds to the rows below.

**The row 1 wording was a prediction and it held verbatim.** It names
`VllmConfig *VllmSpec` with json key `vllm_config` and `InferCommonSpec`
embedded anonymously, written before the field existed; that is what
`pipes_model.go` now declares.

**There is a seventh site, and it is the one a reader following the table would
miss.** A tree-wide search for the `"ollama"` token, excluding tests and
`tools/cpipes_contract/`, returns exactly the five Go switch arms rows 2–6 name
— so the *hand-edited* surface is enumerable rather than merely enumerated. But
searching for `ollama_config` instead returns one more file in this package:
**`cpipes_contract_data.go`, which is generated (`cpipes-contract gofile`) and
checked in**, and which carries a `TransformationSpec/<token>` entry per
operator.

**It already existed when this checklist was written, which is the part worth
knowing.** `cpipes_contract_data.go` landed at `a01fdb53` on 2026-08-16 at
21:19; this document landed at `a5c0f848` thirty minutes later, and does not
name it. So the omission is not staleness catching up with a list — the list was
incomplete on the evening it was verified. The `embed` operator did regenerate
the file (`e52d3c8f`, 2026-08-20) and `TransformationSpec/embed` is in it, so
the step was taken without the checklist asking for it, and the omission has
never yet cost anything. Nothing in the runtime reads
`CpipesContract` today — only its own spot-check test — so a token missing from
it breaks nothing and announces nothing, which is why it is worth a row's worth
of prose here rather than being left to the contract-chain paragraph that covers
it only by implication.

**`vllm` is absent from it, deliberately**: it is downstream of the matrix, and
the matrix was not regenerated (below). What that costs is narrow and should be
stated rather than assumed. The *runtime* validation path is registered — row 3
is `ValidatePipeSpecConfig`, which is what `jets/agentic/tools`'
`validate_cpipes_config` wraps — so a `vllm` step is validated like any other.
What is not registered is the *projected* contract: `cpipes_schema.json` and
this table do not mention the operator, so a model authoring a `.pc.json` from
the projection cannot reach it.

**The contract-chain paragraph below has been overtaken, and the `vllm`
operator did not regenerate the matrix.** Its warning is that a change to
`pipes_model.go` costs a mass re-approval; measured on the pinned commit
**before** this operator's field was added, `cpipes-contract check --code`
already reported **443 `evidence_ref` citations that no longer name their
field** — `MapExpression.Comment` cited at a line inside `HashExpression.String`,
`EmbedSpec` cited eight lines above its own declaration, and so on. The
re-approval the paragraph asks for has not been paid for the changes since
2026-08-20, so the matrix is adrift by considerably more than one operator's
worth, and adding 50 correct rows to it would not make it a document a reviewer
could trust. Recorded as the agentic_ai project's **I-271**; the repair is the
citation scheme the paragraph already proposes, not another `--restamp`.

What the operator does NOT have to write is everything in
`pipe_transformation_infer.go`: the operator shell, the worker pool and its
counters, the on_error policy, the prompt template compile/render with the
build-time `{{col}}` check, the response mapping compile/apply, the retry with
backoff, the cost guard, the channel validation and the call context. The
backend supplies the `inferBackend` seam — `BuildRequest` and `CallOnce` —
plus an `inferResponse` (`Text`, `Tokens`, `ModelName`) and the `inferLabels`
wording. `pipe_transformation_ollama.go` is the worked example: ~330 lines,
all of them genuinely Ollama-specific.

| # | Registration point | What it registers |
|---|---|---|
| 1 | `pipes_model.go` — `TransformationSpec` | the `VllmConfig *VllmSpec` field (json `vllm_config`); the spec embeds `InferCommonSpec` anonymously so the shared configuration and its wire shape come for free |
| 2 | `pipes_runtime_model.go:320` | the DAG-build dispatch: a `case "vllm"` calling the operator's constructor, which ends in `ctx.newInferTransformationPipe(...)` |
| 3 | `actions_start_common.go:1019` | config validation and defaults for the operator (the ollama arm validates presence and pool size) |
| 4 | `actions_start_common.go:1088` | `errorChannelConfig`: the operator's error channel, read from the promoted `InferCommonSpec.ErrorChannel` |
| 5 | `pipe_executor_fan_out.go:55` | the extra-output-channel switch (error channel ownership on close) |
| 6 | `pipe_executor_splitter.go:60` | same, for the splitter executor |

And the contract chain (`tools/cpipes_contract/`): a new operator token means
new matrix rows (a `types.csv` row per token, `fields.csv` rows for the spec —
the `InferCommonSpec` rows carry over by embedding, as `FileConfig`'s do), a
`generate`/`schema`/`gofile` regeneration, and the corpus/negative gates.

**That paragraph is the expensive one, and not for the reason it gives.** The
`embed` operator added 63 field rows, 2 type rows and 3 constraint rows, which
is the predicted cost and is fine. What it also did was invalidate **711
already-reviewed rows** — because `evidence_ref` is an absolute `file:line`
citation, `evidence_ref` is inside the review fingerprint, and adding one field
to `TransformationSpec` shifts every line below it in `pipes_model.go`. The
matrix then reports 695 rows as *changed since reviewed* when nothing a reviewer
looked at has changed at all.

The mechanical repair is an exact old→new line map (`difflib` over the `HEAD`
blob against the working file) applied to `evidence_ref`, followed by
`stamp --restamp`. That works, and it is what was done. But `--restamp` exists
to record an explicit human re-approval, and using it on 695 rows because a
struct gained a field spends the review mechanism to pay for a line number.

**So: any change to `pipes_model.go` costs a mass re-approval until the
citations stop being line numbers.** Two ways out, neither attempted here —
cite a symbol (`pipes_model.go:TransformationSpec.EmbedConfig`) and resolve it
at check time, or exclude `evidence_ref` from the fingerprint and check the
citation on its own axis, the way `harness` already is. Recorded as debt rather
than fixed, because it is matrix-shaped work and this was an operator.

The one place this surface could shrink is rows 5–6: both switches re-derive
what `errorChannelConfig` already answers because they need a superset
(jetrules' `OutputChannels`, clustering's `CorrelationOutputChannel`, and the
error channels). A helper answering that superset would let both switches
disappear. It is operator-shaped work, not backend-shaped, so it was noticed
and deliberately declined during 15a (plan §8.4).
