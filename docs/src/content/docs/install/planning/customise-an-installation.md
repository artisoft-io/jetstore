---
title: Customising an installation
description: What makes one installation of JetStore different from another (its workspace and images, the stack's variables, and a workspace's own Lambdas and operators) and where each lives.
kind: explanation
sidebar:
  order: 8
---

Every installation runs the same JetStore code. What differs between two installations lives in four
places, and none of them is a change to JetStore itself: the **workspace**, the **images** built from
it, the **stack's variables**, and any **Lambda or operator** the workspace brings. This page says
what each holds and where it is set, so that a change for one installation goes in the right place.

## The workspace

The workspace is a separate git repository holding everything that is specific to one client's data:

| What | Where in the workspace, for the example `jets_ws` |
|---|---|
| The data model: the classes and tables the rules and pipelines read and write | `data_model/` |
| The rules | `jet_rules/` |
| Lookup tables | `lookups/` |
| Pipeline configurations (`.pc.json`) | `pipes_config/` |
| The scripts that load its configuration rows into the system tables when the database is initialised | `process_config/` |
| Reports | `reports/` |
| The UI's table configurations and user flows | `table_configs/`, `user_flows/` |
| Runbooks | `runbooks/` |
| Go code for its own operators or Lambdas, if any | `go/` and `go.mod` |

A stack names its workspace at synth with `WORKSPACE`, `WORKSPACE_BRANCH` and `WORKSPACES_HOME`
([Required variables](/docs/install/reference/required-variables/)). JetStore installs its own assets
into the workspace and compiles it when the images are built; at start, the apiserver compares the
compiled workspace's version with its own and brings the workspace up to date. How a workspace is compiled, and what JetStore
installs into it, is in the Developer's Guide's [Workspaces](/docs/developer/workspace/) section.

**A change made in the running installation is an override, not a change to the workspace.** A file
saved through the Workspace IDE is stored in the database and takes precedence over the file in the
image until it is removed; see [Workspace overrides](/docs/developer/workspace/overrides/). To make it
permanent, commit it to the workspace repository and rebuild the images.

The configuration rows a client's data needs (which files are expected, which process runs on them)
are kept in the `jetsapi` system tables: loaded by the workspace's scripts, and changed through the UI.
See [Design of the system tables](/docs/developer/workspace/system-tables/).

## The images

The images a stack runs are the `_ws` images: JetStore's images with the compiled workspace added
(`Dockerfile.ui_service_ws`, `Dockerfile.cpipes_ws`, `Dockerfile.cpipes_native_lambda_ws`). So **each
installation has its own images**, rebuilt whenever either JetStore or the workspace changes, and the
stack names them with `JETS_IMAGE_TAG`, `CPIPES_IMAGE_TAG` and the image repositories. A workspace may
keep its own copies of these Dockerfiles; see [Images](/docs/install/reference/images/).

## The stack's variables

Everything about the shape of the deployment is a variable read at synth: the account, region and
network; the bucket and its prefixes; the database's capacity; the size of the compute-pipes task and
Lambdas; which optional components are built (the native and Python compute-pipes paths, the
inference server, the private API, SQS ingest, the purge, the agentic detection task); domain-key
hashing; pipeline throttling and notifications; the Pipeline Status screen's custom buttons; and
whether git is reachable. They are listed in [Required variables](/docs/install/reference/required-variables/),
[Environment variables](/docs/install/reference/environment-variables/) and
[Sizing an installation](/docs/install/planning/sizing/). Keep each installation's values with the
workspace or the deployment's own scripts; JetStore's repository holds none.

## A workspace's own Lambdas and operators

Where the stock behaviour is not enough, a workspace can bring its own code without forking JetStore.
Each is built against JetStore's packages and deployed by pointing the stack or the image build at it.

| What | How it is wired | Default when unset |
|---|---|---|
| A register-key Lambda that adds to, or takes over, a file's registration | `JETS_REGISTER_KEY_LAMBDA_ENTRY`: a site main built on `registerkey.Run` with a `registerkey.Hook` (`cdk/jetstore_one/lambdas/register_keys/registerkey/`) | the stock Lambda |
| A compute-pipes node Lambda with the workspace's own operators registered | `JETS_CPIPES_NODE_LAMBDA_ENTRY` (`stack/build_cpipes_lambdas.go`) | the stock Lambda |
| The same operators in the ECS task and the native node Lambda | the image build: `Dockerfile.cpipes_builder_ws`, with `JETS_CPIPES_TASK_ENTRY` and `JETS_CPIPES_NATIVE_LAMBDA_ENTRY`, selected by `CPIPES_BUILDER_IMAGE` | the stock binaries |
| Python operators | an image derived from the Python node Lambda's, with its own handler | the stock handler |
| An SQS ingest Lambda | `JETS_SQS_REGISTER_KEY_LAMBDA_ENTRY`, with `EXTERNAL_SQS_ARN` | not built |
| The private REST API's Lambda | `JETS_API_GATEWAY_LAMBDA_ENTRY` | not built |
| A run-reports Lambda for compute-pipes pipelines | `JETS_CPIPES_RUN_REPORTS_LAMBDA_ENTRY` | not built; `cpipesSM` uses the stock run-reports Lambda |

An operator present in one of the three compute-pipes processes and absent from another fails a step,
not a deployment, so a workspace with its own operators builds all three from the same code. Why, and
what that asks of an operator, is in
[A site operator runs in one of three processes](/docs/developer/compute-pipes/notes/site-operators/).

## The workspace's documentation

These guides describe JetStore. What is specific to one installation (its data model, its rules, the
way its client's files are customised) belongs in the workspace's repository. A workspace may carry
its own documentation pages, to be added beside these guides in that installation's copy; that is not
yet in place.
