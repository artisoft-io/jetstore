---
title: "A node's document is a marshal of the struct"
description: "Why a bare json tag on a configuration struct puts a key the author never wrote into the document a node reads, and the corpus test that guards it."
kind: explanation
sidebar:
  order: 9
---

**Established 2026-09-27, by the first deployed run of a Python `cp_node`, and then measured over the
whole corpus.**

## The symptom

The Python node refused the document it was handed, `ConfigInvalid` with 49 validation errors, before
doing any work. The document is `cpipes_execution_status.cpipes_config_json`, and the Python node
validates it against the contract model, which has `extra="forbid"` throughout. Most of the 49 were
Pydantic reporting every branch of the operator union; the causes were two.

## The mechanism

A starter does not copy the authored `.pc.json` into that column. It unmarshals it into
`ComputePipesConfig`, builds a new one (`actions_start_sharding_cp.go`, `actions_start_reducing_cp.go`),
and `json.Marshal`s that. **So every field whose tag has neither `omitempty` nor `omitzero` comes back
as a key, carrying its zero value, whether or not the author wrote it**: `"type": ""` on every channel
config and on a conditional `then`, `"apply": null` on `merge_files`, an empty `output_channel` on
`jetrules`, `"name": ""` on output channels and columns. Go reads every one of those back as the zero
value it already was, which is why **no Go node ever noticed** and why nothing on the Go side could
have.

The second cause is a real runtime field rather than an artefact: `ClusterSpec.ShardingInfo`, which the
sharding starter sets on every run and which the contract's runtime model did not admit.

## What was measured

Every `.pc.json` of the four workspaces of the rule corpus, one document per step built the way a starter builds it, then
validated by the Python runtime model: **194 of 194 refused** before the change and **0 of 194** after.
The healthcare run only hit two of the five kinds because of which operators its step happens to use.

## The fix, and the guard

The tags were given `omitempty` (strings) or `omitzero` (slices, pointers, structs) on
`TransformationSpec.Type` and `.OutputChannel`, `PipeSpec.Apply`, `InputChannelConfig.Type`,
`OutputChannelConfig.Type` and `.Name`, `AnonymizeSpec.KeysOutputChannel`, and in `pipesmodel`
`ChannelSpec.Columns` and `TransformationColumnSpec.Name`. **`omitzero` rather than `omitempty` on a
slice is deliberate**: it drops a `nil` slice and keeps an author's explicit `[]`, so the document
stays faithful in both directions. The contract gained `ClusterSpecRuntime`
(`tools/cpipes_contract/cpipes_model.py`) for `sharding_info`.

`TestCorpusRuntimeDocumentAddsOnlyRuntimeKeys` (`runtime_document_corpus_test.go`) asserts the
invariant rather than the tags: for every step in the corpus, the starter-shaped document may add only
`common_runtime_args` and `cluster_config.sharding_info` to what the author wrote. **A new field with a
bare tag goes red there on the day it is added**, naming the file and the key path, instead of on the
day a Python step first reaches it. Run it with `JETS_PC_CORPUS_DIR` set and `-count=1`, because it
reads a submodule.

**The other bare `type` tags were left alone on purpose** — `Metric`, `LookupSpec`, `CsvSourceSpec`,
`SchemaProviderSpec`, `ReportCmdSpec`, `PipeSpec`, `FunctionTokenNode` — because the corpus always
writes them, so marshalling them adds nothing. If one of them becomes optional, the test above is what
will say so.
