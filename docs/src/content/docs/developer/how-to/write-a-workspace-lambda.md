---
title: Write a Lambda for a workspace
description: Replace one of JetStore's stock Lambdas with a workspace's own handler, or add one of the optional Lambdas, and get the stack to build it.
kind: how-to
sidebar:
  order: 2
---

A workspace does not add Lambdas of its own to the stack. What the stack offers is **five
variables that name a Lambda's source directory**, each read at synth time, and a workspace supplies
the handler they point at. The handler is a Go `main` package in the workspace repository, built
against the `jetstore` Go module.

This was carried out on 2026-11-11 with a scratch workspace module: a register-key handler was
written, built under a Go workspace, and bundled by a `cdk synth` of the stack. The synth then stopped
at the AWS account lookup, as it does without credentials, so nothing was deployed. Step 2's script was
run the same way, with a synth given a cached availability-zone lookup, which bundled the handler and
completed.

## The five Lambdas a workspace can supply

They are declared in the comment block *Expected Env Variables* in `cdk/jetstore_one/jetstore_one.go`
and resolved in `cdk/jetstore_one/stack/`.

| Variable | Lambda | Unset |
|---|---|---|
| `JETS_REGISTER_KEY_LAMBDA_ENTRY` | `registerKeyV2`, the main ingest path (`build_registerkey_lambdas.go`) | the stock `lambdas/register_keys/register_keys_v2` |
| `JETS_CPIPES_NODE_LAMBDA_ENTRY` | `CpipesNodeLambda`, the compute-pipes node (`build_cpipes_lambdas.go`) | the stock `lambdas/compute_pipes/cp_node` |
| `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY` | a register-key Lambda triggered from SQS (`build_registerkey_lambdas.go`) | not built |
| `JETS_API_GATEWAY_LAMBDA_ENTRY` | the API Gateway Lambda (`build_api_lambdas.go`) | not built |
| `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` | `CpipesRunReportsLambda`, the run-reports step of the compute-pipes state machine (`build_lambdas.go`, `build_cpipes_sm.go`) | not built; the state machine uses the stock `RunReportsLambda` |

**The first two default and the other three gate**, and the difference is what an unset variable
means. The register-key and node Lambdas run in every deployment, so an unset or empty variable builds
JetStore's own entry (`lambdaEntryOrDefault` in `stack/stack_model.go`; the defaults are
`registerKeyLambdaDefaultEntry` and `cpipesNodeLambdaDefaultEntry`). The other three name a Lambda
only a site has, so without the variable it is not in the stack at all. When
`CpipesRunReportsLambda` is built, the compute-pipes state machine runs it in place of
`RunReportsLambda`.

Every other Go Lambda of the stack (the status update, the secret rotation, the purge, the two
compute-pipes starters, the stock run reports) takes its source from `cdk/jetstore_one/lambdas/` and
nothing else. The native and Python compute-pipes node Lambdas are container images
(`NewDockerImageFunction`), with no source path to redirect.

## 1. Write the handler

Put it in the workspace repository, by convention under `go/lambdas/<name>/`, with a `go.mod` at the
workspace root. Start from the stock `main` the Lambda would otherwise run, and change only what the
site needs.

**The register-key Lambda has a hook for this**, so the site's `main` is the stock one with an
argument. `registerkey.Run` (`cdk/jetstore_one/lambdas/register_keys/registerkey/registerkey.go`) takes
a `registerkey.Hook`, whose `BeforeRegister` is called with the file key's components before each
registration (`hook.go`). It may add components, report the event as a schema event, or return
`done` to handle the event itself; the stock `main` passes `nil`. The handler used here skipped one
kind of file and left the rest to the stock path:

```go
package main

import (
	"context"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/cdk/jetstore_one/lambdas/register_keys/registerkey"
	"github.com/artisoft-io/jetstore/jets/datatable"
)

type siteHook struct{}

func (siteHook) BeforeRegister(ctx context.Context, dtCtx *datatable.DataTableContext,
	fileKey string, components map[string]any, token string) (bool, bool, error) {
	if strings.HasSuffix(fileKey, ".tmp") {
		log.Println("skipping a temporary file:", fileKey)
		return false, true, nil
	}
	return false, false, nil
}

func main() {
	registerkey.Run(siteHook{})
}
```

**The compute-pipes node Lambda's site `main` is the stock `cp_node` plus one option**:
`compute_pipes.WithOperators(...)` on its `CoordinateComputePipes` call, which registers the site's
own operators. `jets_ws/go/` holds a reference operator and its registry, and its readme gives the
change. See [A site operator runs in one of three processes](/docs/developer/compute-pipes/notes/site-operators/)
before writing one.

## 2. Build it against JetStore

A workspace module does not `require` the `jetstore` module. **The JetStore import resolves through a
Go workspace that lists both repositories**, in the first directory above both checkouts.
`utils/compose_workspace_go_work.sh` writes it:

```bash
utils/compose_workspace_go_work.sh <jetstore checkout> <workspace checkout>
cd <workspace checkout>
go build -o /tmp/bootstrap ./go/lambdas/register_keys
```

The script finds that directory, runs `go work init` there with both checkouts as relative paths, and
then checks that a build in the workspace uses this `go.work` and resolves
`github.com/artisoft-io/jetstore` to the checkout given. A `go.work` already there that uses both is left
alone. It changes nothing, and says why, when the workspace has no `go.mod` at its root, when one
checkout is inside the other, when a `go.work` nearer the workspace would be found first, or when the
directory already holds a `go.work` that does not use both. With `--check` it only checks, and exits 1
when the composition is missing, so a deploy script can run it before `cdk synth`.

**Run it rather than `go work init` by hand when any directory above has a `go.work` of its own.**
`go work init` then takes that file as the one to create and stops with `go.work already exists`; the
script names the file to write.

A build outside the composition fails: with `GOWORK=off` the build above gives `no required module
provides package github.com/artisoft-io/jetstore/...`.

## 3. Point the stack at it

Set the variable to the handler's directory and synthesise as usual. The stack bundles every Go Lambda
from source at synth time with `awslambdago.NewGoFunction`, so the handler is compiled then, not
before:

```bash
export JETS_REGISTER_KEY_LAMBDA_ENTRY=<workspaces home>/example_ws/go/lambdas/register_keys
cd cdk/jetstore_one && cdk synth
```

A relative path resolves against the directory `cdk` runs in, which is `cdk/jetstore_one`
(`cpipesNodeLambdaDefaultEntry`'s comment), so an absolute path is the safer form. The bundler finds the
handler's module by walking up from the entry and builds it under the `go.work` above it. Here the
synth bundled `registerKeyV2` from the scratch module: `go version -m` on the bundled `bootstrap` named
`example.com/example_ws/go/lambdas/register_keys` as its main package, beside the stock
`.../lambdas/compute_pipes/cp_node` and the other stock entries.

**The synth log is the only place to see which entry was built** for the two defaulting variables:
the template does not change shape, and `main` in `jetstore_one.go` prints each variable as it reads
it. Read it before the diff.

## What the code does not provide

**The stack does not compose the Go workspace that step 2 needs.** It requires that the handler's
module resolve `github.com/artisoft-io/jetstore`, and leaves it to whatever runs `cdk` for an
installation to run `utils/compose_workspace_go_work.sh` first, or `--check` it. The images compose one
themselves:
`dockerfiles/Dockerfile.cpipes_builder_ws` runs `go work init` over the JetStore source and the
workspace at `/`, and builds the site's entries for the compute-pipes ECS task and the native node
Lambda (`JETS_CPIPES_TASK_ENTRY`, `JETS_CPIPES_NATIVE_LAMBDA_ENTRY`). Those two are image build
arguments, not synth variables, and refuse a workspace with no `go.mod` at its root.

**And a workspace cannot add a Lambda the stack does not name.** A sixth variable is a change to
`cdk/jetstore_one`, in this repository.

What else an installation can change, and where, is in
[Customising an installation](/docs/install/planning/customise-an-installation/); building the
workspace's images is in [Build and deploy a workspace](/docs/install/how-to/build-and-deploy-a-workspace/).
