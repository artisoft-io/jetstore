---
title: User flows
description: How a JetStore user flow takes you through a task in steps, its Previous, Cancel and Next buttons, where it returns when it ends, and a page for each flow.
kind: overview
sidebar:
  label: Overview
  order: 0
---

A **user flow** is a screen that takes you through one task in steps: choose a pipeline
configuration, then its input, then confirm, for example. Each step is one form, often a single
table to choose from, and the flow keeps your choices from step to step. The flows are part of the
workspace (see [Workspaces](/docs/user/concepts/workspaces/#what-a-workspace-holds)), so an
installation's workspace may add its own.

## What every flow has

- **Its title**, at the top, and under it **the step you are on**, such as *Select a pipeline
  configuration*.
- **Previous**, to go back a step, keeping what you chose. Disabled on the first step, and on the
  step a flow was opened at when it was opened part way through.
- **Cancel**, to leave without doing anything.
- **Next**, to go on. A step whose choice is required says so, under its table or field,
  when you press *Next* without one.
- On the last step, instead of *Next*, **the button that does the task**: *Start Pipeline & Done*,
  *Load Files & Done*, *Confirm*. Nothing is changed before you press it.

**When a flow ends, or you cancel it, it returns to the screen you opened it from**, or to Home when
it has nowhere else to go. Its address is `flow/<key>`, where the key names the flow's documents in
the workspace, such as `startPipelineUF`.

Choosing a flow from the *JetStore Workspace* menu starts it again from its first step, even if you
are already in it.

## The flows in this section

| Flow | What it does | Opened from |
|---|---|---|
| [Pipeline Execution Status Filters](/docs/user/screens/flows/pipeline-execution-status-filters/) | chooses which runs Home shows | Home, *Set Filters* |
| [Start Pipeline](/docs/user/screens/flows/start-pipeline/) | starts a pipeline by hand | Home, *Start Pipeline* |
| [Load Files](/docs/user/screens/flows/load-files/) | loads files into their staging table | the *Source Mapping* flow, *Load Data* |
| [File Mapping Configuration](/docs/user/screens/flows/file-mapping-configuration/) | the worksheet that maps a source's columns onto the data model | the *Source Mapping* flow, *Configure Mapping* |
| [Load Client Configurations](/docs/user/screens/flows/load-client-configurations/) | loads clients' configuration from the workspace into the database | the *Workspaces* screen, *Load Client Config* |
