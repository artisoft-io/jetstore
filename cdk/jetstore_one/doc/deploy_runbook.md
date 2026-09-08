# Deploy runbook

Everything the stack needs is an environment variable read at synth time. This is the preflight, the
deploy, and the two cases the defaults do not cover: the first deploy of a new stack, and a second
stack sharing an existing VPC.

The authoritative variable list is the comment block at `jetstore_one.go:506`. This document covers
what is **required**, what fails **loudly**, and what fails **silently**.

## 1. Prerequisites

| | |
|---|---|
| CDK bootstrap | `utils/cdk_bootstrap.sh` (runs `cdk bootstrap $CDK_BOOTSTRAP_ARG`), once per account/region |
| Images in ECR | `jetstore` (UI + run reports) and `cpipes`; plus `infer` and the cpipes native lambda image if those are enabled |
| Local toolchain | Go (the app is `go run jetstore_one.go`), the `cdk` CLI, and AWS credentials for the target account |
| Network reachability | Synth performs a live `GET https://api.github.com/meta` when `JETS_GIT_ACCESS` names github, and **panics if it fails** |

The Lambda bundles are compiled from source at synth by `awslambdago`, so a broken Go build in
`jets/` fails the deploy, not just the image build.

**That row is about synth and only about synth: `JETS_GIT_ACCESS` does not reach a container.** It is
read once, by `NewGitAccessSecurityGroup` (`stack/jetstore_github.go:71`), to open egress to the
providers it names, and it is in no task definition's environment map. So it decides whether a git
operation *could* reach a host, not whether the apiserver attempts one, and a stack deployed without
it still runs the git commands the Workspace Registry screen offers. The runtime switch is
`JETS_NO_GIT_ACCESS` (section 8), and the two are worth keeping apart because "deployed without
`JETS_GIT_ACCESS`" reads as a statement about behaviour and is a statement about the network.

## 2. Preflight — what `main` refuses to run without

These are checked together and reported as a list before CDK is reached; any one of them panics with
`Terminated due to missing or invalid env variables`.

| Variable(s) | Condition |
|---|---|
| `AWS_ACCOUNT`, `AWS_REGION` | always |
| `AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK`, `AWS_PREFIX_LIST_S3` | always — region-specific prefix list ids |
| `JETS_s3_INPUT_PREFIX`, `JETS_s3_OUTPUT_PREFIX` | always |
| `WORKSPACES_HOME`, `WORKSPACE`, `WORKSPACE_BRANCH` | always |
| `JETS_ECR_REPO_ARN`, `JETS_IMAGE_TAG` | always |
| `JETS_CERT_ARN` | when `JETS_ELB_MODE=public` |
| `JETS_ELB_INTERNET_FACING` | when `JETS_ELB_MODE=public`; must be exactly `true` or `false` |
| `JETS_DB_MIN_CAPACITY`, `JETS_DB_MAX_CAPACITY`, `JETS_CPU_UTILIZATION_ALARM_THRESHOLD` | only if set — must parse as a float |

**Warnings, not failures**: no `JETS_DOMAIN_KEY_HASH_ALGO`/`JETS_DOMAIN_KEY_HASH_SEED` means domain
keys are not hashed; setting exactly one of `JETS_STACK_ID` / `JETS_STACK_SUFFIX` is reported as
probably unintended.

### Required but *not* preflighted

These are not in the check above and fail later, which is worth knowing because the error names a
construct rather than a variable:

- **`CPIPES_ECR_REPO_ARN` and `CPIPES_IMAGE_TAG`** — the cpipes image is resolved unconditionally at
  `jetstore_one.go:316`. An empty repo ARN fails inside CDK's ARN parsing, not with a JetStore message.
- **`INFER_ECR_REPO_ARN` and `INFER_IMAGE_TAG`** — `log.Fatal` from `InferEcrRepoArn()` /
  `InferImageTag()` when `BUILD_INFER_SERVICE` is on. Deliberately not defaulted to the JetStore
  image: that would deploy a container with no `ollama` binary and fail only in the container log.
- **`JETS_INFER_MODEL`** — `log.Fatal` from `InferModel()`, but **only when `INFER_BACKEND=vllm`**.
  vLLM serves the single model it was started with, so there is nothing to default to.

### Choosing the infer backend

`INFER_BACKEND` is `ollama` unless set, and the stack it synthesises with the variable absent is
the one it synthesised before the variable existed. Setting it to `vllm` requires two things
together, and **nothing checks that they agree**:

1. `INFER_IMAGE_TAG` naming an image built from `dockerfiles/Dockerfile.infer_service_vllm`
   rather than `Dockerfile.infer_service`. A mismatch is a task that starts and cannot exec its
   server — visible only in the container log.
2. `JETS_INFER_MODEL` naming what vLLM serves. `JETS_INFER_SERVED_MODEL_NAME` is the alias a
   pipeline configuration's `model` property may use instead of the full repository id; the four
   `JETS_VLLM_*` variables are optional tuning and are left to vLLM's own defaults when unset.

Both arms use the same service, ASG, capacity provider, target group, port and `JETS_INFER_URL`,
so switching is a task-definition revision. Two things do not carry over:

- **The infer admin screen's model actions are Ollama's** — `/api/pull`, `/api/ps`, `/api/show`,
  `/api/delete` (`inferActions`, `../../../jets/apiserver/api_infer_server.go`). vLLM serves none
  of them; expect the screen's model list and pull to fail against a vLLM task.
- **A deploy of this service does not rotate.** `GpuCount` is 1 on a single-GPU instance, so the
  replacement task cannot be scheduled beside the one it replaces; the service is stopped by hand
  first. Two backend arms therefore cost two of those stops, which is an argument for running them
  in one sitting.
- **`JETS_VPC_ID`, `JETS_VPC_ENDPOINTS_SG_ID`** — `log.Fatal` from the lookup helpers when the
  imported-VPC path is taken. See §5.

### Fails silently — no error, no resource

**The private REST API is skipped, with only a log line, if any of three things is missing**:
`JETS_API_GATEWAY_LAMBDA_ENTRY`, a resolved API Gateway VPC endpoint, or
`JETS_API_GATEWAY_EXEC_ROLE_NAME` (`build_api_lambdas.go:25`, `:31`, `:140`). The deploy succeeds and
the API simply is not there. If you expected an API, check for `JetsApiUrl` in the outputs rather
than trusting the exit status.

Other conditional components behave the same way by design, and their absence is normal:
`RETENTION_DAYS` (purge data), `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` (SQS ingest),
`JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY`, `DEPLOY_CPIPES_NATIVE`, `BASTION_HOST_KEYPAIR_NAME`.

## 3. Deploy

Both scripts `cd ./cdk/jetstore_one` first, so run them from the repository root:

```bash
./utils/diff_jetstore_one.sh
```

```bash
./utils/deploy_jetstore_one.sh
```

They are `cdk diff` and `cdk deploy --require-approval never`. **Always diff first** — with
`--require-approval never` there is no IAM/security-group confirmation prompt to catch a surprise.

Synth logs every environment variable it read. Check that log before the diff: a typo'd optional
variable is invisible otherwise, because an unset optional simply produces a component that is not
built.

## 4. First deploy of a new stack

Three defaults are tuned for updating an existing stack and are wrong exactly once.

**Set `INFER_DESIRED_COUNT=0`** if `BUILD_INFER_SERVICE` is on. Leaving it unset omits `DesiredCount`
from the template, which is what makes later deploys preserve whatever scale the service is at. But
with the property absent, ECS defaults a *brand new* service to 1 — so a GPU instance launches as
soon as the stack comes up. On an existing stack, leave it unset (`stack_model.go:242`).

**The database and the stack are both protected from deletion on creation**:
`TerminationProtection` on the stack and `DeletionProtection` on the Aurora cluster. A first deploy
that you intend to tear down and retry needs both lifted first — see §7.

**The bucket's removal policy is decided at creation.** With `JETS_BUCKET_NAME` unset the stack
creates a bucket with `RemovalPolicy_DESTROY` and `AutoDeleteObjects`; with it set, the bucket is
imported and never destroyed. Switching later does not migrate data.

After the first deploy, seed the database and the workspace through the UI or `update_db`; the stack
creates the cluster but no JetStore schema.

## 5. A second stack sharing the VPC

Set `JETS_STACK_ID` and `JETS_STACK_SUFFIX` **together** — the first names the CloudFormation stack,
the second is appended by `props.MkId` to every resource whose name must be unique in the account.

Then take three values from the first stack's outputs (see [`stack_outputs.md`](stack_outputs.md)):

| Set on the second stack | From the first stack's output |
|---|---|
| `JETS_VPC_ID` | `JetStoreVpcID` |
| `JETS_VPC_ENDPOINTS_SG_ID` | `VpcEndpointsSGID` |
| `JETS_API_GATEWAY_VPC_ENDPOINT_ID` | `ApiGatewayVpcEndpointId` — only if deploying the private API |

With `JETS_VPC_ID` set, five variables become inert: `JETS_NBR_NAT_GATEWAY`,
`JETS_VPC_INTERNET_GATEWAY`, `JETS_VPC_CIDR`, `AWS_PREFIX_LIST_ROUTE53_HEALTH_CHECK` and
`AWS_PREFIX_LIST_S3`. The last two are still required by the preflight even though nothing consumes
them on this path.

Three things worth knowing about what is and is not shared:

- **Only the endpoint security group is shared.** `RdsAccessSg` and `InternetAccessSg` are created
  per stack, so each stack keeps its own database boundary and its own Aurora cluster.
- **The CIDR is read back from the imported VPC** (`jetstore_vpc.go:46`), so you do not pass it.
- **There is no CloudFormation dependency between the stacks.** No output carries an `ExportName`;
  the coupling is values copied at synth. The two stacks can be updated and deleted independently,
  and equally, deleting the first stack's VPC out from under the second is not prevented.

Give the second stack its own `JETS_BUCKET_NAME` or its own S3 prefixes. Two stacks sharing a bucket
*and* an input prefix will both receive the same S3 events and both register the same file keys.

## 6. Post-deploy checks

```bash
aws cloudformation describe-stacks --stack-name "${JETS_STACK_ID:-JetstoreOneStack}" --query 'Stacks[0].Outputs'
```

- `UiListenerUrl` responds; the ALB health check is `/healthcheck/status` on container port 8443.
- The UI service has a running task; a task that starts and dies is usually the workspace copy in
  `cbooter` failing, visible in the `UiContainerLogGroup`.
- If the API was expected, `JetsApiUrl` is present.
- If infer was expected, `InferListenerUrl` is present and `PersistentVolumeId` names a volume.

## 7. Teardown

`cdk destroy` will not complete while the guards are in place. In order: disable stack
`TerminationProtection`, disable Aurora `DeletionProtection`, then destroy. Two things survive
regardless and must be removed by hand if you mean to: the **infer persistent volume**
(`RemovalPolicy_RETAIN`, named by the `PersistentVolumeId` output) and an **imported bucket**
(`JETS_BUCKET_NAME`). A stack-created bucket is emptied and deleted with the stack.

## 8. Running against a local checkout, and why `JETS_NO_GIT_ACCESS=1` belongs there

This section is about a workstation rather than a stack. It sits in the deploy runbook because this
is the document that says which environment variable decides what, and the setting below is one a
developer wants **before** the first run rather than after the first surprise.

**Locally, `WORKSPACES_HOME` is a tree of submodules of the developer's own repository.** JetStore's
checkout on the `jets_ai` branch puts the code at `jetstore_ai/` and the workspaces at `workspaces/`,
so `${WORKSPACES_HOME}/${WORKSPACE}` is a working tree of the repository the developer is editing in,
not a copy the server owns. Two consequences follow, and the second is the expensive one.

**The Workspace Registry screen does not render.** An uninitialised submodule is an existing
directory that is not a repository, so the directory-absent branch of `GetStatus`
(`jets/datatable/git/workspace_git.go`) does not catch it: `git rev-parse` exits 128 with
`fatal: not a git repository`, and the read action turns that into a 400 for the whole request. One
workspace nobody initialised blanks the screen for the others as well.

**A button in a local UI can write into the developer's repository.** The git operations in that
package go through one helper, `runGit`, which sets `cmd.Dir` to `${WORKSPACES_HOME}/${WORKSPACE}`.
`UpdateLocalWorkspace` runs `switch`, `pull` and `push`; `CommitLocalWorkspace` runs `add -A`,
`commit` and `push`. So pressing *Update Local Workspace* in a UI started for an unrelated reason can
move the developer's HEAD, stage everything below it, and push it.

**And that failure hides itself, which is the argument for setting the variable in advance.** A
submodule moved under you leaves a detached HEAD with `git status` reporting **clean** about a tree
you did not ask for. The signals an ordinary check produces all say nothing happened, so the cost is
not a visible error but an afternoon spent working out where the work went.

**So set `JETS_NO_GIT_ACCESS=1` in the local environment file**, alongside `WORKSPACES_HOME`,
`WORKSPACE` and `WORKSPACE_BRANCH`. Truthy is `1`, `true`, `yes` or `on`, case-insensitive and
trimmed; anything else, including unset and including the empty string, leaves git integration on
(`jets/datatable/git/no_git_access.go`, which is the authority on the semantics). With it set, the
registry screen reports its status from the file system without shelling out to git, and the write
actions return a notice naming the variable instead of acting. The apiserver states at startup which
way the switch went, so a misread setting is visible in the first few lines of the log rather than in
its consequences.

**It is not a local-only workaround.** The same variable serves a site with no route to a
source-control host, which is the deployment it was added for: no `JETS_GIT_ACCESS`, no
`WORKSPACE_URI`, and a workspace baked into the image with no `.git` in it. Local development is the
second consumer and the one that reaches a developer's own files.
