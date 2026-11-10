---
title: "Workspaces"
description: "How a workspace is compiled, seeded onto a compute node and overridden, the system tables it configures, and the assets JetStore installs into it."
kind: overview
sidebar:
  label: Overview
---

A workspace is the repository of a client's rules, data model, pipeline configurations, reports and
user flows. JetStore compiles it, bakes it into its images, and seeds it onto the nodes that run the
pipelines. These pages cover what happens to a workspace on the way, from the `jets/workspace` and
`jets/workspace_assets` packages:

| Page | What it covers |
|---|---|
| [Compiling a workspace](/docs/developer/workspace/compiling/) | the flow-document validation a compile runs first, when `lookup.db` is rebuilt, and where the compile sits in a schema migration |
| [The workspace on a compute node](/docs/developer/workspace/compute-node-workspace/) | how the native lambda seeds its baked workspace, the directory modes that broke it, and what the seed leaves out |
| [Workspace overrides](/docs/developer/workspace/overrides/) | files saved through the Workspace IDE, and how they are removed |
| [Design of the system tables](/docs/developer/workspace/system-tables/) | what each `jetsapi` table is for and how the configuration tables relate |
| [Workspace assets JetStore installs](/docs/developer/workspace/assets/) | the data model, pipeline configurations, user flows and table documents every workspace receives |

How the rules themselves are compiled is in [The JetRules compiler](/docs/developer/engine/compiler/);
the format of a workspace's runbooks is in [The workspace runbook format](/docs/developer/engine/runbook-format/).
