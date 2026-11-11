---
title: "Operators"
description: "Every operator type a pipeline configuration can name, one line each, with its configuration section and its own pages where it has them."
kind: overview
sidebar:
  label: "Overview"
  order: 0
---

An operator is one entry of a step's `apply` list: it reads records from its input channel and writes
them to its output channel. Its `type` names which operator it is. Every operator's fields are in the
generated [pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/#transformationspec);
this page lists the types, and the pages below it cover the operators whose behaviour needs more than a
field list.

File names without a directory are in `jets/compute_pipes/`.

## The operator types

A configuration can name 21 types: the variants of `TransformationSpec` (`pipes_model.go`) in the
contract matrix, `tools/cpipes_contract/matrix/types.csv`. Eighteen are built-in operators that
`BuildPipeTransformationEvaluator` (`pipes_runtime_model.go`) dispatches on; `infer` is rewritten into
one of them before the pipeline is built; and two rows, `~override` and `~site`, are not types at all
but describe what an entry means when its `type` is absent or unknown (see
[below](#the-override-and-site-rows)). Each type below links its section of the configuration
reference.

### Map, filter and order records

| Type | What it does |
|---|---|
| [`map_record`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecmaprecord) | Maps each input record onto the output channel's columns through its column transformations, optionally driven by a file-mapping table (`pipe_transformation_map_record.go`). |
| [`filter`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecfilter) | Keeps the records for which its `filter_config.when` expression is true, drops rows without the input channel's number of columns when `row_length_strict` is set, and caps the records kept at `max_output_records` (`pipe_transformation_filter.go`). |
| [`distinct`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecdistinct) | Keeps one record per value of a composite key (`pipe_transformation_distinct.go`). |
| [`sort`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecsort) | Sorts the input records in memory on a composite key or a domain key, and sends them on in that order (`pipe_transformation_sort.go`). |
| [`aggregate`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecaggregate) | Reduces all its input records to one output record, with aggregate column functions such as `sum`, `count` and `max` (`pipe_transformation_aggregate.go`). |

### Bundle records

| Type | What it does |
|---|---|
| [`group_by`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecgroupby) | Groups consecutive input records into bundles, by a composite key, a domain key or a fixed count; each output record is one bundle (`pipe_transformation_group_by.go`). |
| [`merge`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecmerge) | Merges records from several input channels, all sorted in the same order, into bundles on a grouping key (`pipe_transformation_merge.go`). |

### Run rules

| Type | What it does |
|---|---|
| [`jetrules`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecjetrules) | Runs each input record or bundle as a rule session on a pool of workers, and extracts the output records from the session by type, one output channel per type; it has no single `output_channel` (`pipe_transformation_jetrules.go`). |

### Profile and protect data

| Type | What it does |
|---|---|
| [`analyze`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecanalyze) | Profiles every column of the input and emits one record per column: total, null and distinct counts, lengths, and with its parse functions the minimum and maximum as dates, numbers or text (`pipe_transformation_analyze.go`). |
| [`high_freq`](/docs/developer/reference/generated/pipeline-configuration/#transformationspechighfreq) | Emits, for each selected column, the values in its top percentile by frequency (`pipe_transformation_high_freq.go`). |
| [`anonymize`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecanonymize) | Anonymises or de-identifies the columns an analysis lookup classifies as sensitive; in `anonymization` mode it can also emit the mapping from original to anonymised keys (`pipe_transformation_anonymize.go`). |
| [`shuffling`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecshuffling) | Builds a sample of synthetic records by shuffling values column by column, from a capped input sample and the columns a lookup retains (`pipe_transformation_shuffling.go`). |
| [`clustering`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecclustering) | Computes the correlation between sets of columns that a lookup table classifies; kept for future use, and not used by a production configuration today (`pipe_transformation_clustering.go`). |

### Call a model, render text

| Type | What it does |
|---|---|
| [`infer`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecinfer) | Calls an inference server once per record and adds values from the response to the record, on whichever backend the deployment names; `ResolveInferBackend` (`resolve_infer_backend.go`) rewrites it into an `ollama` or `vllm` step before the pipeline is built. See [the inference operators](/docs/developer/compute-pipes/operators/infer/). |
| [`ollama`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecollama) | The ollama backend of `infer`, also named directly: one generate or chat call per record, values extracted from the response (`pipe_transformation_ollama.go`). See [the ollama backend](/docs/developer/compute-pipes/operators/infer-ollama/). |
| [`vllm`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecvllm) | The vLLM backend of `infer`, also named directly: one call per record to a vLLM server's OpenAI-compatible API (`pipe_transformation_vllm.go`). See [the vLLM backend](/docs/developer/compute-pipes/operators/infer-vllm/). |
| [`embed`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecembed) | Calls the infer server's embeddings endpoint once per record and puts the vector on the record, through the same plumbing as `ollama` (`pipe_transformation_embed.go`). |
| [`render`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecrender) | Renders one text template per record over a serialised entity and adds the text to the record (`pipe_transformation_render.go`). See [the render operator](/docs/developer/compute-pipes/operators/render/). |

### Write partitions

| Type | What it does |
|---|---|
| [`partition_writer`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecpartitionwriter) | Writes the records of its input channel to partition files in the stage area, as CSV, Parquet or fixed-width, and records each partition it wrote (`pipe_transformation_partition_writer.go`). |

## The ~override and ~site rows

The contract matrix needs a row for every shape an `apply` entry can take, and two shapes have no
`type` value of their own. Their tokens start with `~` so that they cannot be mistaken for one.

- **[`~override`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecoverride)**
  is an entry with no `type`, and it is meaningful only as the `then` of a
  [`conditional_config`](/docs/developer/reference/generated/pipeline-configuration/#conditionaltransformationspec)
  item. When that item's `when` is true, `ApplyAllConditionalTransformationSpec`
  (`actions_start_common.go`) applies it to the operator that holds it: a `then` with no `type` is
  merged field by field into the host operator by `MergeTransformationSpec`, while a `then` with a
  `type` replaces the host altogether. No production configuration uses it today.
- **[`~site`](/docs/developer/reference/generated/pipeline-configuration/#transformationspecsite)** is an
  entry whose `type` names none of the built-in operators. `BuildPipeTransformationEvaluator` tries the
  built-in types first, then, in its `default` branch, looks the type up in the site operator registry
  (`buildSiteOperator`, `site_operators.go`): an operator a deployment registers with `WithOperators`
  rather than one JetStore ships. A type that is neither built in nor registered fails with *unknown
  TransformationSpec type*. A site operator registered under a built-in type, or under `infer`, is ignored with a warning: the built-in wins. See
  [where a site operator runs](/docs/developer/compute-pipes/notes/site-operators/).

## Pages in this section

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
