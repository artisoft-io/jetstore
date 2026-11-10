---
title: "Operators"
description: "The compute-pipes operators documented here: render, and the inference operators with their two backends."
kind: overview
sidebar:
  label: "Overview"
  order: 0
---

An operator is one entry of a step's `apply` list: it reads records from its input channel and writes
them to its output channel. Every operator's fields are in the generated
[pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/); these
pages cover the operators whose behaviour needs more than a field list.

| Page | Kind | Covers |
|---|---|---|
| [The render operator](/docs/developer/compute-pipes/operators/render/) | reference | A configured text template over a serialised entity: configuration, notation, failure model |
| [How the render operator is designed](/docs/developer/compute-pipes/operators/render-design/) | explanation | Why its input is a column, the worked example that ships, and the measurement it exists because of |
| [The inference operators](/docs/developer/compute-pipes/operators/infer/) | reference | The `infer` type and backend selection, and the plumbing `ollama`, `vllm` and `embed` share |
| [What the inference measurements showed](/docs/developer/compute-pipes/operators/infer-measurements/) | explanation | Backend choice, throughput, cold start, encoding, and a template against a model |
| [The ollama backend](/docs/developer/compute-pipes/operators/infer-ollama/) | reference | The ollama backend's own keys, request and response |
| [Ollama wire payloads](/docs/developer/compute-pipes/operators/infer-ollama-payloads/) | reference | The exact ollama request bodies, and the Infer Server Admin screen's envelope |
| [The vLLM backend](/docs/developer/compute-pipes/operators/infer-vllm/) | reference | The vLLM backend's own keys, guided decoding, and its two failure modes |
| [vLLM prompt configuration](/docs/developer/compute-pipes/operators/infer-vllm-payloads/) | reference | The exact vLLM request bodies and build-time errors |

File names without a directory are in `jets/compute_pipes/`.
