---
title: "What the inference measurements showed"
description: "Measurements of the inference operators over one pipeline and one population: backend choice, throughput, cold start, encoding, and a template against a model."
kind: explanation
sidebar:
  order: 4
---

**Measured 2026-09-09/10.**

**This section exists because these numbers are expensive to produce and easy to lose.**
They were measured against the `patient_profile` briefing pipeline over a **curated
22-member population**, and every figure below carries that scope.

**Read all of it as narrow.** One model at 3B, one pipeline, one population, and the runs
named per result. It licenses *this model, on this pipeline, over this population, did* —
never *a model cannot*.

## 3.1 The backend choice is a cost and latency decision, not a quality one

**The strongest result in the set, and the most useful.** Three arms — vLLM bf16 eager,
vLLM bf16 with CUDA graphs, ollama `Q4_K_M` — over the same 22 members and the same fact
set:

> **Every *decidable* accuracy category is identical across all three.** The approximate
> ones vary by one to four, inside their own brackets and inside the run-to-run spread
> already measured within a single arm.

So the model's errors are a property of **the model and the prompt** rather than of the
serving stack — across a 4-bit and a 16-bit checkpoint, two servers and two execution
modes. **Choose a backend on throughput, latency and cold start.**

## 3.2 The two backends cross, and the crossing is at the pool the pipeline runs

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

## 3.3 Cold start: vLLM 127 s, ollama ~20 s — and the biggest term was the disk

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

## 3.4 Encode entities as JSON, not TOON

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

## 3.5 A template beat the model, and that is the finding to carry

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

**That template is an operator now, and this paragraph is where a reader will look for it.**
`type: render` applies a configured text template to a serialised entity column — the same input
contract as an infer step, so two versions of a pipeline can differ in the operator block and in
nothing else. See
[the render operator](/docs/developer/compute-pipes/operators/render/). **What it does not
do is settle which arm a deployment runs**: the arms differ on cost, latency and what a briefing is
for, and §3.7 below still bounds what the 22-of-22 result licenses.

## 3.6 Recommendations

1. **Name `type: infer` and set the backend from the deployment.** Everything in §3.1 says
   the document should not care which server runs.
2. **Prefer a template where the output is a projection of the input.** Ask what the model
   adds that a deterministic renderer cannot; if the answer is *phrasing*, the template is
   more accurate and free — and since 2026-09-11 it is a configured operator rather than Go,
   [the render operator](/docs/developer/compute-pipes/operators/render/). **Run a
   deterministic control beside any model arm** — without one, §3.5 is not measurable.
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
   and no warning — see [the vLLM backend, §3](/docs/developer/compute-pipes/operators/infer-vllm/#3-guided_json-is-not-format).
8. **Do not read a passing corpus test as evidence without `-count=1`.** These pipelines
   read workspace files that live outside the Go module, so nothing in the test cache key
   changes when a workspace moves, and a stale `ok` is indistinguishable from a real one.

## 3.7 What these numbers do not establish

- **Not that TOON is a worse encoding in general** — one model, one population, one run.
- **Not that a model cannot write a briefing** — this model, this pipeline, this population
  did not.
- **Not a throughput claim without a stated concurrency.** The arms cross; any figure here
  is meaningless without its pool size.
- **Neither backend was measured at its best.** vLLM was tuned and ollama still ran without
  flash attention, which is the same unfairness reversed rather than removed.
- **The two arms did not serve identical weights.** Same model *name*, different artefacts —
  bf16 against `Q4_K_M`.
