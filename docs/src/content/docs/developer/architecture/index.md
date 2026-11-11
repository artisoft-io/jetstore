---
title: "Architecture"
description: "How JetStore is put together: its components, how a file becomes output, the decisions behind the design, and where the deeper explanation pages sit."
kind: overview
sidebar:
  label: Overview
  order: 0
---

This section is the map of JetStore for an engineer: what the parts are, how a file travels through
them, and why the design is the way it is. The pages elsewhere in the Developer's Guide go deeper into
one part each; this one says where they fit.

| Page | What it covers |
|---|---|
| [Components and how a file becomes output](/docs/developer/architecture/components/) | the apiserver, the web app, the register-key Lambda, the compute-pipes Lambdas and ECS tasks, the state machines, the C++ rule engine and its bridge, PostgreSQL and S3; and a file traced from the bucket to its output files |
| [Decision records](/docs/developer/architecture/decisions/) | the design decisions already made, each with its context, its consequences and where its reason is recorded |

## Where the deeper pages sit

Each part of the [components page](/docs/developer/architecture/components/) has explanation pages of
its own:

| Part | Pages |
|---|---|
| The stack as deployed | [Architecture](/docs/install/planning/architecture/) and [Stack components](/docs/install/reference/stack-components/) in the Installation Guide: every service, Lambda, task and state machine, and what gates it |
| From a file to a running pipeline | [How a file starts a pipeline](/docs/install/planning/ingest-data-flow/): the register-key Lambda, the registry tables and the choice of state machine, step by step |
| The pipeline engine | [Compute pipes](/docs/developer/compute-pipes/): the operators, the [run manifest](/docs/developer/compute-pipes/run-manifest/), and the [engineering notes](/docs/developer/compute-pipes/notes/) on how a run behaves across processes |
| The rule engine and the compiler | [The C++ rule engine](/docs/developer/engine/rule-engine/) and [The JetRules compiler](/docs/developer/engine/compiler/): how rules are compiled into `workspace.db` and run |
| The workspace | [Workspaces](/docs/developer/workspace/): how a workspace is [compiled](/docs/developer/workspace/compiling/), [seeded onto a compute node](/docs/developer/workspace/compute-node-workspace/) and [overridden](/docs/developer/workspace/overrides/), and how a compiled workspace [reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/) |
| The web app | [How the web app is served and built](/docs/developer/testing/web-app/serving/): the apiserver's static handler and the UI image |
| The database | [Design of the system tables](/docs/developer/workspace/system-tables/) and the generated [system tables reference](/docs/developer/reference/generated/system-tables/) |
| The pipeline configuration | [The compute pipes contract](/docs/developer/tools/cpipes-contract/) and the generated [pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/) |
