# Infer Server — GPU memory and Ollama tuning

Measured against a running `jetstore-infer-service` task with `granite4.1:3b`, Ollama 0.32.5.
First written 2026-08-04 against `g5.xlarge`; **re-measured 2026-08-20 on `g6e.2xlarge`**, which is
the instance these numbers now describe. Every figure below comes from `tools/infer_capacity` run
against the deployed task, not from a formula.

This file covers **runtime memory behaviour** only. For how the service is assembled — the ASG,
the persistent EBS volume, the lifecycle hook, GPU scheduling and the AMI contract — see the
"Infer Server (GPU inference)" section of [CLAUDE.md](CLAUDE.md).

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
per request**. The largest prompt the authoring loop actually builds is 13.5k (measured at F.6), so
that is 3.6× headroom.

| combination | VRAM | per request | |
|---|---|---|---|
| **98304, 2 parallel, 1 model** | **32.26 GiB** | **49 152** | **current** |
| 98304, 4 parallel, 1 model | 32.26 GiB | 24 576 | safe; restores E.8's K to 4 |
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
interacts with `OLLAMA_NUM_PARALLEL`. An earlier revision of this file asserted the division from
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
was never varied, because doing so needs a deployment. See I-47.

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

## Scaling the service

The service is scaled on demand and is normally left at 0:

```bash
CLUSTER=$(aws ecs list-clusters --query 'clusterArns[]' --output text \
  | tr '\t' '\n' | grep -i jetstore | head -1)
aws ecs update-service --cluster "$CLUSTER" \
  --service jetstore-infer-service --desired-count 1   # or 0 to stop
```

**A stack deploy leaves this count alone.** `DesiredCount` is deliberately omitted from the
CloudFormation template (see `InferDesiredCount` in `stack/stack_model.go`), because
CloudFormation only manages the property when it is present. Running stays running, stopped
stays stopped.

It was previously pinned to `0`, which meant every stack update — including updates that had
nothing to do with the infer service — reset the count and stopped a running task mid-use. The
symptom is a 503 from the load balancer with no corresponding error anywhere in the infer logs,
because the task was stopped deliberately rather than having failed.

The one case that still needs an explicit value is the **first** deploy of a new stack: with the
property absent, ECS defaults a brand new service to 1 and a GPU instance starts as soon as the
stack comes up. Set `INFER_DESIRED_COUNT=0` for that initial deploy, then leave it unset.

> **Not yet verified.** The `INFER_DESIRED_COUNT=0` path has only been reasoned through, not
> exercised — it can only be tested on a brand new stack, since ECS applies the "default a new
> service to 1" behaviour at create time only. The preservation behaviour on *update* is
> verified (see below). Confirm this the next time a new environment is stood up.

## Stopping it completely

Scaling the service to 0 stops the task but **leaves the `g5.xlarge` running**, which is where
almost all of the cost is. The instance belongs to the `InferASG` auto-scaling group, not to the
ECS service, so it has to be scaled down separately:

```bash
CLUSTER=JetstoreOneStack-ecsCluster15812518-mXSyN5XGIXmN
ASG=$(aws autoscaling describe-auto-scaling-groups \
  --query "AutoScalingGroups[?contains(AutoScalingGroupName,'InferASG')].AutoScalingGroupName" \
  --output text)

# 1. stop the task
aws ecs update-service --cluster "$CLUSTER" \
  --service jetstore-infer-service --desired-count 0

# 2. terminate the GPU instance
aws autoscaling set-desired-capacity --auto-scaling-group-name "$ASG" --desired-capacity 0
```

Step 2 is about *when*, not whether. The capacity provider has managed scaling enabled with
`DisableScaleIn: false` and managed termination protection off, so ECS will scale the ASG to 0 on
its own once the last task stops — but that runs off a CloudWatch target-tracking alarm and takes
roughly 15 minutes. Setting the desired capacity explicitly terminates it immediately. It is also
stable: with no tasks pending, managed scaling has nothing to scale back out for.

To start it again, reverse the order — capacity first, then the task:

```bash
aws autoscaling set-desired-capacity --auto-scaling-group-name "$ASG" --desired-capacity 1
aws ecs update-service --cluster "$CLUSTER" \
  --service jetstore-infer-service --desired-count 1
```

Scaling only the service also works, since managed scaling launches an instance when a task
cannot be placed, but it is slower: the alarm has to fire before the instance even begins booting.

### From Go

`awsi.StartInferServer` / `awsi.StopInferServer` (`jets/awsi/infer_server.go`) do both steps
in the right order. They are idempotent — starting an already running server returns
`changed=false` rather than an error — and they return as soon as both scale changes are
requested, without waiting for the instance or the model.

The CDK stack sets `JETS_ECS_CLUSTER_NAME`, `JETS_INFER_SERVICE_NAME` and
`JETS_INFER_ASG_NAME` on the UI container whenever the infer server is part of the stack, and
every `InferServerTarget` field falls back to its variable, so from the apiserver:

```go
changed, err := awsi.StartInferServer(ctx, awsi.InferServerTarget{})
```

The variables being absent is how the apiserver can tell "infer server not deployed" from
"deployed but stopped" — with `BUILD_INFER_SERVICE` unset, none of the three are present and
the corresponding IAM permissions are not granted either.

`awsi.GetInferServerStatus` reports the same thing back: `running`, `stopped`, or the
`starting` / `stopping` transitions, derived from the desired counts against what is actually
placed. It reads only, through the two describes the scaling calls already make, so it needs
no permissions beyond theirs.

### From the UI

**Workspace IDE → Infer Server Admin** drives all of this from a screen. It needs the
`infer_server_admin` capability, which is checked in the apiserver
(`jets/apiserver/api_infer_server.go`), not only in the UI — the disabled menu button is
presentation, the endpoint is the enforcement point. The capability is seeded for
`knowledge_engineer` in `jets/jets_init_db.sql`.

The screen posts a request envelope to `/inferServer`. The envelope names an action and the
server maps it to a route; it never accepts a path, which would make the apiserver an open
proxy into the VPC.

```json
{ "action": "pull_model", "body": { "model": "granite4.1:3b", "stream": false } }
```

| `action` | Effect |
|---|---|
| `server_status` | `GetInferServerStatus` |
| `start_server` / `stop_server` | `StartInferServer` / `StopInferServer`, then re-reads status |
| `list_models` | `GET /api/ps` |
| `pull_model` | `POST /api/pull` |
| `show_model` | `POST /api/show` |
| `delete_model` | `DELETE /api/delete` |

Set `"stream": false` on a pull: Ollama otherwise streams NDJSON progress, which the screen
cannot render. The proxy allows 20 minutes, matching the ALB idle timeout — enough for a model
this size, possibly not for a much larger one.

The apiserver reaches Ollama at `JETS_INFER_URL`, which `BuildELB` sets on the UI container
(the load balancer does not exist yet when `BuildUiService` runs). Its absence means the stack
was built without the infer server, and the proxy actions say so rather than failing to connect.

**What still costs money after this.** The 100 GiB gp3 persistent volume
(`vol-02db94957c76c43cb`, ~$8/month) is deliberately retained — it holds the model weights, and
deleting it means re-pulling them on next start. The instance's 50 GiB root volume has
`DeleteOnTermination: true` and disappears with the instance. Everything else in the stack (ELB,
NAT gateway, RDS) is not infer-specific.

## Verification

With `num_ctx: 32768`, against the live service:

- **Single request** — `data/unit_test_data/cintel/ollama_unit_test_chat.json` in the
  `jets_ws` workspace: 4,407-token prompt, 467-token response, 7.1s end to end.
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
placed without a GPU — which points at the ECS GPU inventory or the AMI (see CLAUDE.md), not at
these settings. When it is between the two, the cache is spilling and the context is too large.

**A `ggml_aligned_malloc` failure naming a CPU buffer is a context-size problem, not a host-RAM
problem.** Raising `INFER_MEM_LIMIT_MB` or moving to a larger-RAM instance treats the symptom;
the allocation only reached host RAM because it had already overflowed the GPU.
