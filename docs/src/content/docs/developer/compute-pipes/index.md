---
title: "Compute pipes"
description: "The compute-pipes pipeline engine: its operators, the run manifest, the rule optimizer's statistics, running a pipeline locally, and engineering notes."
kind: overview
sidebar:
  label: "Overview"
  order: 0
---

Compute pipes is JetStore's pipeline engine. A pipeline configuration, a `.pc.json` in a workspace's
`pipes_config/`, declares channels and the operators that read and write them; the engine runs it in
sharding and reducing steps, on ECS tasks and Lambdas. The code is in `jets/compute_pipes/`, and a file
named on these pages without a directory is in that package.

- [Operators](/docs/developer/compute-pipes/operators/): the `render` operator, and the inference operators `infer`, `ollama`,
  `vllm` and `embed`.
- [The run manifest](/docs/developer/compute-pipes/run-manifest/): the document a completed run writes, describing each declared
  deliverable and what was written for it.
- [The rule optimizer's statistics](/docs/developer/compute-pipes/optimizer-statistics/): how a jetrules step counts what the rule
  optimizer orders by, and how the counts reach the workspace.
- [How-to guides](/docs/developer/compute-pipes/how-to/): adding an inference backend, running a pipeline locally, and collecting
  optimizer statistics.
- [Engineering notes](/docs/developer/compute-pipes/notes/): how the engine behaves across process boundaries, and the facts that
  took real digging to establish.

Every field of a pipeline configuration is listed in the generated
[pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/).
