---
title: Inference server memory
description: How the optional GPU inference server's Ollama process uses GPU and host memory, what was measured, and how to choose its context, parallelism and loaded-model settings together.
kind: explanation
sidebar:
  order: 6
---

Measured against a running `jetstore-infer-service` task with `granite4.1:3b`, Ollama 0.32.5.
First written 2026-08-04 against `g5.xlarge`; **re-measured 2026-08-20 on `g6e.2xlarge`**, which is
the instance these numbers now describe. Every figure below comes from `tools/infer_capacity` run
against the deployed task, not from a formula.

This page covers **runtime memory behaviour** only. For how the service is assembled — the ASG,
the persistent EBS volume, the lifecycle hook and GPU scheduling — see the
[infer service](/docs/install/reference/stack-components/#infer-service-gpu-optional) component; for
the AMI, [Build the inference server AMI](/docs/install/how-to/build-the-infer-ami/). To start and
stop the service, see [Start and stop the inference server](/docs/install/how-to/start-and-stop-the-infer-server/).

## The failure

The first chat request against a freshly started server — the one that loads the model into
memory — died before generating a token:

```
{"error":"llama-server process has terminated: exit status 1:
ggml_aligned_malloc: insufficient memory (attempted to allocate 22528.00 MB)
ggml_backend_cpu_buffer_type_alloc_buffer: failed to allocate buffer of size 23622320128
alloc_tensor_range: failed to allocate CPU buffer of size 23622320128
llama_init_from_model: failed to initialize the context: failed to allocate buffer for kv cache"}
```

Cause: `OLLAMA_CONTEXT_LENGTH` was set to `256000`.

## Why that setting is dangerous

Three properties of Ollama combine badly here, and none of them are obvious from the error text.

**The KV cache is allocated up front at model load, sized from `OLLAMA_CONTEXT_LENGTH` — never
from the actual prompt.** A 4k-token prompt against a 128k context still pays for 128k of cache.
The cost is not the model weights: `granite4.1:3b` is a 2.1 GB download.

**`MemoryLimitMiB` on the ECS container definition cannot protect against this.** It is a cgroup
limit on *host RAM*. It is invisible to Ollama's memory planner, which sizes the cache against
total VRAM plus total host RAM as the *host* reports them. No value of `INFER_MEM_LIMIT_MB` would
have prevented this failure. It bounds what the container may use; it does not tell Ollama what
to plan for.

**Over-large values are silently clamped, not rejected.** `256000` exceeds `granite4.1:3b`'s
131072 maximum, so Ollama clamped it to 131072 and proceeded. Confirmed by reproduction: a
request with no options and an explicit `num_ctx: 131072` fail with the *identical* 22528.00 MB
figure.

What then happens at 131072: the cache does not fit in the A10G's 24 GiB, so Ollama spills the
overflow to host RAM — and asks for 22 GiB of it on an instance with 16 GiB. `malloc` fails and
`llama-server` exits. The service stays up and healthy; every model load just fails.

## Measured KV cache cost

`granite4.1:3b` on **`g6e.2xlarge` (L40S, 48 GiB VRAM / 64 GiB host RAM)**, measured 2026-08-20
with `tools/infer_capacity`, which reads `size` and `size_vram` from `/api/ps` rather than
inferring the spill from throughput.

| `num_ctx` | Total | Resident in VRAM | tok/s | Result |
|---|---|---|---|---|
| 32k | 12.32 GiB | 12.32 GiB | 202.7 | fully GPU-resident |
| **48k** | **17.39 GiB** | **17.39 GiB** | **203.0** | **fully GPU-resident — current default** |
| 64k | 22.45 GiB | 22.45 GiB | 202.7 | fully GPU-resident |
| 128k | 42.14 GiB | 42.14 GiB | 202.6 | fully GPU-resident — granite's own maximum |

**Nothing spilled, at any context this model supports.** 131072 is the model's maximum and it sits
fully on the GPU with about 6 GiB to spare, so for a single model **the card is no longer the
constraint — the model is.** Throughput is flat at ~203 tok/s across the whole range.

The cache still costs **~0.31 MiB per token** on a ~2.3 GiB base. That is a property of the model
and did not change with the card; what changed is how much of it fits.

### What this replaced

The previous instance was `g5.xlarge` (A10G, 24 GiB VRAM / 16 GiB host RAM), where the ceiling was
between 56k and 60k and the spill was graded rather than a cliff — 68% of resident speed at 0.59 GiB
spilled, 38% at 2.26 GiB, with nothing logged either time. Those readings are gone from the table
because the instance is gone, but the *shape* of the failure is worth carrying: a model that has
spilled a little just looks slow.

## Choosing the three settings together

`OLLAMA_MAX_LOADED_MODELS` multiplies the VRAM cost. `OLLAMA_NUM_PARALLEL` **divides** the
per-request context and does not change the VRAM cost at all. `OLLAMA_CONTEXT_LENGTH` sizes the
window they act on. **They are one choice, not three.**

**Deployed 2026-08-20: `98304` / `2` parallel / `1` model** — 32.26 GiB resident, **49 152 tokens
per request**. The largest prompt the authoring loop actually builds is 13.5k, so
that is 3.6× headroom.

| combination | VRAM | per request | |
|---|---|---|---|
| **98304, 2 parallel, 1 model** | **32.26 GiB** | **49 152** | **current** |
| 98304, 4 parallel, 1 model | 32.26 GiB | 24 576 | safe; four requests at a time |
| 128k, 4 parallel, 1 model | 42.14 GiB | 32 768 | safe, near the ceiling |
| 98304, 2 parallel, **2 models** | **64.5 GiB** | 49 152 | **over 48 GiB — silent spill** |
| 49152, 2 parallel, 2 models | 34.78 GiB | 24 576 | what two models costs |

**Raising `OLLAMA_MAX_LOADED_MODELS` to 2 requires lowering `OLLAMA_CONTEXT_LENGTH` in the same
change.** Leaving them inconsistent is safe only while exactly one model exists, because Ollama does
not load a model until it is requested — so the failure arrives with the second model, as a spill
rather than a refusal.

### How the division was established, because the first attempt did not

`tools/infer_capacity` sets `num_ctx` **per request**, which overrides `OLLAMA_CONTEXT_LENGTH`
entirely. So the sweep measures the per-request path and says nothing about how the env var
interacts with `OLLAMA_NUM_PARALLEL`. An earlier revision of this page asserted the division from
the sweep; the sweep could not see it.

What settled it was a reading with no override at all — `/api/ps` immediately after a plain
generate:

```
size 32.26 GiB   size_vram 32.26 GiB   context_length 98304
```

against `OLLAMA_CONTEXT_LENGTH=98304`, `OLLAMA_NUM_PARALLEL=2`. That is the figure a *single*
98304-token request allocates, not twice it. Multiplying would have wanted ~120 GiB and refused to
start.

**One consequence worth keeping.** The previous default of `32768` with `OLLAMA_NUM_PARALLEL` at 4
was giving each request **8 192 tokens** — under the prompts the loop builds. That truncation was
latent and unnoticed, and it is gone rather than fixed.

**And one figure here is still arithmetic, not a reading**: the two-model rows. `OLLAMA_MAX_LOADED_MODELS`
was never varied, because doing so needs a deployment.

## Current defaults

Set in `cdk/jetstore_one/stack/build_infer_service.go`; each is overridable by an environment
variable of the same name at CDK synth time.

| Variable | Default | Rationale |
|---|---|---|
| `OLLAMA_CONTEXT_LENGTH` | `32768` | Fully GPU-resident, ~7x the production prompt size |
| `OLLAMA_MAX_LOADED_MODELS` | `1` | A second model of this size would not fit alongside the first |
| `OLLAMA_NUM_PARALLEL` | `4` | Verified to serve 4 concurrent requests from the 32k pool |
| `OLLAMA_KEEP_ALIVE` | `30m` | Must carry a unit — a bare integer is parsed as *seconds* |

`OLLAMA_CONTEXT_LENGTH` and `OLLAMA_MAX_LOADED_MODELS` multiply against the same fixed 24 GiB of
VRAM and must be chosen together. Raising either one requires lowering the other, or a larger
GPU. Two models at 32k want 24.6 GiB and will not fit.

## Verification

With `num_ctx: 32768`, against the live service:

- **Single request** — a test chat request: 4,407-token prompt, 467-token response, 7.1s end to end.
- **4 concurrent copies of that request** — 10.6s wall clock (vs 8.0s for one), all four
  returning complete responses. Memory flat at 12.32 GiB, fully in VRAM.
- **Warm generation** — ~166 tok/s.

Scale preservation across a deploy, with the service running at 1:

- **`cdk deploy` completed `UPDATE_COMPLETE` on the service, and the task was untouched** — same
  task id and same `startedAt` before and after, `desired`/`running` still 1, and no ECS service
  events logged during the update at all. The model stayed resident in VRAM, so the first
  post-deploy request served warm in 4.3s.

## Notes for the next person debugging this

**A slow first call is normal and is not a GPU problem.** The initial request after a model load
took 22s to evaluate 15 tokens; the same request warm took 10ms. Cold-start warmup, nothing more.
Do not diagnose from the first call.

**To check whether the GPU is actually being used, read `size_vram` from `/api/ps`,** not the
timings. When it equals `size`, the model is fully GPU-resident. When it is 0, the task was
placed without a GPU — which points at the ECS GPU inventory or the AMI (see
[Build the inference server AMI](/docs/install/how-to/build-the-infer-ami/)), not at
these settings. When it is between the two, the cache is spilling and the context is too large.

**A `ggml_aligned_malloc` failure naming a CPU buffer is a context-size problem, not a host-RAM
problem.** Raising `INFER_MEM_LIMIT_MB` or moving to a larger-RAM instance treats the symptom;
the allocation only reached host RAM because it had already overflowed the GPU.
