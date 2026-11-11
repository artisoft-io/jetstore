---
title: Inference server memory
description: How the optional GPU inference server's Ollama process uses GPU and host memory, the defaults the stack sets, and how to choose the context, parallelism and loaded-model settings together.
kind: explanation
sidebar:
  order: 6
---

This page covers how the inference server uses memory when it runs Ollama (`INFER_BACKEND` unset or
`ollama`), and the settings that control it. The defaults are read from
`cdk/jetstore_one/stack/build_infer_service.go` (`inferContainerEnvironment`, through
`InferEnvOrDefault` in `stack/stack_model.go`) and `cdk/jetstore_one/stack/build_infer_ec2.go`. The
measurements were taken with `tools/infer_capacity` against a deployed `jetstore-infer-service` task
running `granite4.1:3b` on Ollama 0.32.5, on the default instance type, `g6e.2xlarge`.

For how the service is assembled (the auto scaling group, the persistent EBS volume, the lifecycle
hook and GPU scheduling) see the
[infer service](/docs/install/reference/stack-components/#infer-service-gpu-optional) component; for
the AMI, [Build the inference server AMI](/docs/install/how-to/build-the-infer-ami/); to start and
stop it, [Start and stop the inference server](/docs/install/how-to/start-and-stop-the-infer-server/).
For the vLLM backend, see [Choosing the infer backend](/docs/install/reference/required-variables/#choosing-the-infer-backend):
its tuning variables have no defaults and none of the figures below apply to it.

## The defaults

Each Ollama variable is overridable at synth time by an environment variable of the same name; the
value is copied into the container definition. The instance and memory settings are synth variables
too.

| Variable | Default | Set in |
|---|---|---|
| `OLLAMA_CONTEXT_LENGTH` | `98304` | `inferContainerEnvironment`, `build_infer_service.go` |
| `OLLAMA_NUM_PARALLEL` | `2` | `inferContainerEnvironment`, `build_infer_service.go` |
| `OLLAMA_MAX_LOADED_MODELS` | `1` | `inferContainerEnvironment`, `build_infer_service.go` |
| `OLLAMA_KEEP_ALIVE` | `30m` | `inferContainerEnvironment`, `build_infer_service.go` |
| `INFER_EC2_INSTANCE_TYPE` | `g6e.2xlarge` (one L40S: 48 GiB of VRAM, 64 GiB of host RAM) | `BuildInferEc2`, `build_infer_ec2.go` |
| `INFER_MEM_LIMIT_MB` | `52224` (1024 × 51, about 80% of the instance's host RAM) | `InferMemLimitMB`, `stack_model.go` |
| `INFER_ROOT_VOLUME_GB` | `50` | `BuildInferEc2`, `build_infer_ec2.go` |
| `INFER_VOLUME_THROUGHPUT_MIBPS` | `500` (125 to 1000) for the 100 GiB persistent model volume | `BuildInferEc2`, `build_infer_ec2.go` |

`OLLAMA_KEEP_ALIVE` must carry a unit: Ollama reads a bare integer as seconds, so `30` would unload
the model after 30 seconds idle. `OLLAMA_MODELS` and `OLLAMA_HOST` are fixed by the stack, not
overridable: the weights go on the persistent volume, under `JETS_TEMP_DATA`, and the server listens
on port 11434.

With the defaults, each in-flight request gets 98304 / 2 = 49152 tokens of context, and the model
uses 32.26 GiB of the card's 48 GiB.

## Why the context length is the dangerous setting

Three properties of Ollama combine, and none of them is obvious from its error messages.

**The KV cache is allocated in full when the model loads, sized from `OLLAMA_CONTEXT_LENGTH`, not from
the actual prompt.** A 4k-token prompt against a 128k context still pays for 128k of cache. The
weights are the small part: `granite4.1:3b` is a 2.1 GB download.

**`INFER_MEM_LIMIT_MB` cannot protect against an oversized cache.** It becomes `MemoryLimitMiB` on the
container definition, a cgroup limit on host RAM. Ollama's memory planner does not see it: it sizes
the cache against the VRAM and host RAM the host reports. The limit protects the instance from the
container, not the container from itself.

**A value above the model's maximum is clamped, not rejected.** `granite4.1:3b`'s maximum is 131072;
a larger setting is silently reduced to it.

When the cache does not fit in VRAM, Ollama spills the rest to host RAM. On a smaller card the result
is either a model that runs more slowly, with nothing logged, or a model load that fails outright
when the spill exceeds the host's RAM. On the `g5.xlarge` the service used before (24 GiB of VRAM,
16 GiB of host RAM), a context of 256000, clamped to 131072, asked for 22 GiB of host RAM and every
model load failed:

```
ggml_aligned_malloc: insufficient memory (attempted to allocate 22528.00 MB)
...
llama_init_from_model: failed to initialize the context: failed to allocate buffer for kv cache
```

The service stayed up and healthy throughout; only the model loads failed.

## Measured cache cost on the default instance

`granite4.1:3b` on `g6e.2xlarge`, read from `size` and `size_vram` in Ollama's `/api/ps`:

| Context (`num_ctx`) | Total | In VRAM | Tokens per second |
|---|---|---|---|
| 32k | 12.32 GiB | 12.32 GiB | 202.7 |
| 48k | 17.39 GiB | 17.39 GiB | 203.0 |
| 64k | 22.45 GiB | 22.45 GiB | 202.7 |
| 128k | 42.14 GiB | 42.14 GiB | 202.6 |

Nothing spills at any context this model supports, and throughput does not change across the range.
The cache costs about 0.31 MiB per token on a base of about 2.3 GiB; that is a property of the model,
not of the card. **For one model of this size, the model's own maximum is the limit, not the card.**

## Choosing the three settings together

- `OLLAMA_CONTEXT_LENGTH` sizes the cache.
- `OLLAMA_NUM_PARALLEL` **divides** it between concurrent requests and does not change the VRAM it
  costs: each request gets `OLLAMA_CONTEXT_LENGTH / OLLAMA_NUM_PARALLEL` tokens.
- `OLLAMA_MAX_LOADED_MODELS` **multiplies** it: each loaded model has its own cache.

They are one choice, not three.

| Combination | VRAM | Tokens per request | |
|---|---|---|---|
| 98304, 2 parallel, 1 model | 32.26 GiB | 49152 | the default |
| 98304, 4 parallel, 1 model | 32.26 GiB | 24576 | fits; four requests at a time |
| 131072, 4 parallel, 1 model | 42.14 GiB | 32768 | fits, near the card's limit |
| 98304, 2 parallel, 2 models | about 64.5 GiB | 49152 | **over 48 GiB: spills silently** |
| 49152, 2 parallel, 2 models | about 34.8 GiB | 24576 | what two models cost |

The division was established with no per-request override: with `OLLAMA_CONTEXT_LENGTH=98304` and
`OLLAMA_NUM_PARALLEL=2`, `/api/ps` reported 32.26 GiB and `context_length 98304` after a plain request,
the figure a single 98304-token request costs. `tools/infer_capacity` sets `num_ctx` on each request,
which overrides `OLLAMA_CONTEXT_LENGTH`, so its sweep alone cannot show how the two variables interact.
The two-model rows are arithmetic, not readings: `OLLAMA_MAX_LOADED_MODELS` has not been varied on a
deployment.

**Raising `OLLAMA_MAX_LOADED_MODELS` to 2 needs a lower `OLLAMA_CONTEXT_LENGTH` in the same change.**
Ollama loads a model only when it is first requested, so an inconsistent pair works until a second
model is used, and then spills rather than failing.

**Lowering the context below the prompts the pipelines send truncates them.** A context of 32768 with
4 parallel requests gives each request 8192 tokens; check the largest prompt your pipelines build
before choosing.

## Verification

To check the settings on a running server, send it a chat request directly, on port 11434 (Ollama's
`/api/chat`), with a prompt about the size of the largest your pipelines build, and then read
`/api/ps`: `size` and `size_vram` should be equal, and `context_length` should be
`OLLAMA_CONTEXT_LENGTH`. Send the same request `OLLAMA_NUM_PARALLEL` times at once to check that the
concurrent requests are served together and the memory does not change. Judge the time from the
second request, not the first; see below.

## A deploy keeps the service's scale

The infer service's `DesiredCount` is left out of the template unless `INFER_DESIRED_COUNT` is set
(`InferDesiredCount`, `stack_model.go`). CloudFormation then leaves the count alone on an update, so a
running service stays running, with its model still loaded, and a stopped one stays stopped.

## Diagnosing memory problems

**A slow first request after a model load is normal.** Loading the model and warming up can take tens
of seconds; judge speed from a later request.

**Read `size_vram` from `/api/ps` to see whether the GPU is used.** When it equals `size`, the model is
entirely in VRAM. When it is 0, the task was placed without a GPU, which points at the ECS GPU
inventory or the AMI (see [Build the inference server AMI](/docs/install/how-to/build-the-infer-ami/)),
not at these settings. When it is between the two, the cache is spilling and the context is too large.

**A `ggml_aligned_malloc` failure naming a CPU buffer is a context-size problem, not a host-RAM
problem.** Raising `INFER_MEM_LIMIT_MB` or choosing an instance with more RAM treats the symptom: the
allocation reached host RAM only because it had already overflowed the GPU.
