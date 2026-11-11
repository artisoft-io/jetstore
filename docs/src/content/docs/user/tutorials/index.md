---
title: Tutorials
description: Two tutorials that take a newcomer through JetStore's main tasks once, end to end, with the jets_ws workspace and its client CI.
kind: overview
sidebar:
  label: Overview
  order: 0
---

A tutorial teaches by doing: it walks you through a whole task once, from start to finish, and says a
little about what is happening as you go. Do them in this order. Once you have, the
[how-to guides](/docs/user/how-to/) are the place to come back to for a single task.

| Tutorial | What you do |
|---|---|
| [Your first pipeline](/docs/user/tutorials/first-pipeline/) | follow a new eligibility file from its arrival to the patient profiles a pipeline builds from it |
| [Your first rule change](/docs/user/tutorials/first-rule-change/) | change a rule in the Workspace IDE, compile the workspace and run the rule's unit test |

Both use the `jets_ws` workspace and its client `CI`, whose configuration ships with the workspace,
and the small invented set of patients the rest of this guide uses. The second needs the
`workspace_ide` capability; the first needs `run_pipelines` and `workspace_ide`.
