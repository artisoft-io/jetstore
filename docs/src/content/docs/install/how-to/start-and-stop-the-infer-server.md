---
title: Start and stop the inference server
description: Scale the GPU inference service and its instance up and down from the command line, from Go, or from the Infer Server Admin screen, and what still costs money when it is stopped.
kind: how-to
sidebar:
  order: 12
---

The inference server is optional; the stack builds it only with `BUILD_INFER_SERVICE` set (see the
[infer service](/docs/install/reference/stack-components/#infer-service-gpu-optional) component). How
it uses memory, and how to choose its settings, is in
[Inference server memory](/docs/install/planning/infer-server-memory/).

## Scaling the service

The service is scaled on demand and is normally left at 0:

```bash
CLUSTER=$(aws ecs list-clusters --query 'clusterArns[]' --output text \
  | tr '\t' '\n' | grep -i jetstore | head -1)
aws ecs update-service --cluster "$CLUSTER" \
  --service jetstore-infer-service --desired-count 1   # or 0 to stop
```

**A stack deploy never starts or stops the service.** On a stack that already holds it,
`DesiredCount` is left out of the CloudFormation template, because CloudFormation manages the
property only when it is present: running stays running, stopped stays stopped. A new service is
created at 0. Synth tells the two apart by reading the deployed stack (`InferDesiredCount` in
`stack/infer_desired_count.go`); if it cannot read the stack it creates or sets the service at 0,
and says so in the synth log. `INFER_DESIRED_COUNT` overrides it.

It was previously pinned to `0`, which meant every stack update — including updates that had
nothing to do with the infer service — reset the count and stopped a running task mid-use. The
symptom is a 503 from the load balancer with no corresponding error anywhere in the infer logs,
because the task was stopped deliberately rather than having failed.

Left out on a **new** service, the property made ECS create it at 1. That did not launch a GPU
instance, as this page used to say: the service uses the EC2 launch type rather than the capacity
provider, so managed scaling never scales the auto scaling group up for it. The task could not be
placed and the deploy waited on the service until its count was set to 0 by hand (seen on a new
stack, 2026-11-24). Fixed in 4.1.19 by creating a new service at 0.

**To run a new task definition, stop the server and start it again after the deploy.** A deploy
that changes the task definition of a *running* service still asks ECS for a rolling replacement,
which cannot place a second task on the instance's one GPU: stop the server before such a deploy.

## Stopping it completely

Scaling the service to 0 stops the task but **leaves the GPU instance running** (by default a `g6e.2xlarge`, `INFER_EC2_INSTANCE_TYPE`), which is where
almost all of the cost is. The instance belongs to the `InferASG` auto-scaling group, not to the
ECS service, so it has to be scaled down separately:

```bash
CLUSTER=<your-ecs-cluster-name>
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
(named by the stack's `PersistentVolumeId` output, for example `vol-0123456789abcdef0`; ~$8/month) is deliberately retained — it holds the model weights, and
deleting it means re-pulling them on next start. The instance's 50 GiB root volume has
`DeleteOnTermination: true` and disappears with the instance. Everything else in the stack (ELB,
NAT gateway, RDS) is not infer-specific.

