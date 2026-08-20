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
