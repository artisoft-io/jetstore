---
title: "A site operator runs in one of three processes"
description: "Why everything the platform needs to know about a site operator must be readable from the document, and three consequences."
kind: explanation
sidebar:
  order: 7
---

**Established 2026-09-12, and every sentence below is the reason a design decision went
the way it did rather than a remark about the code.**

`WithOperators` is an option on `CoordinateComputePipes` (`actions_coordinate_cp.go`), which runs in
**`cp_node`**. The three cpipes lambda entries are separate binaries in separate processes
(`build_cpipes_lambdas.go`), and the functions that decide what a document *means*
before a worker starts all run in the other two:

| Runs in | Function |
|---|---|
| the starters | `ApplyAllConditionalTransformationSpec`, `SynthesizeDefaultErrorChannels`, `ValidatePipeSpecConfig` |
| the node | `CoordinateComputePipes`, `StartComputePipes`, `BuildPipeTransformationEvaluator` |

**So everything the platform needs to know about a site operator before the worker starts has to be
readable from the document.** That is why `site_config` is a typed `SiteOperatorSpec` with a named
`error_channel` (`pipes_model.go`) rather than an opaque blob the registry interprets, and why
`errorChannelConfig` (`actions_start_common.go`) has one arm keyed on `site_config` rather than on
the operator token — it is the only arm in that switch that is.

**Three consequences worth knowing before changing any of it.**

- **A mistyped site token is reported inside a running worker, not at startup.**
  `ValidatePipeSpecConfig`'s operator switch has no `default`, so an unknown `type` passes startup
  untouched and fails at `BuildPipeTransformationEvaluator`'s `default:`. Consulting the registry at
  startup would fix it and cannot be done: the starters do not have one. The only honest startup
  check is a schema one.
- **A site operator is never given a synthesised error channel.** `reportsRowLevelFailures` names
  six built-in tokens and a site token is not one of them, so the synthesis skips it. This is the one
  place that function and `errorChannelConfig` deliberately disagree, and the direction is the benign
  one: an authored channel is honoured and nothing is invented. JetStore cannot know whether an
  operator it knows nothing about will ever write a bad record, and the operator's author is the only
  party who does.
- **A configuration block on a `TransformationSpec` that has no field for it is dropped in silence.**
  The document crosses a process boundary as JSON twice — the starter marshals it into
  `jetsapi.cpipes_execution_status` and the node reads it back through `UnmarshalComputePipesConfig`,
  a plain `json.Unmarshal` with no `DisallowUnknownFields` anywhere in this package. No error, no log
  line. `json.RawMessage` is the field type that survives both hops verbatim without JetStore knowing
  the site's schema.

**And the registry is consulted in `default:` on purpose.** The eighteen built-in cases are tried
first, so a site cannot change what an existing `.pc.json` means. A colliding registration is kept
rather than refused — a deployment should not fail to start because a later JetStore release took its
name — and `WithOperators` logs the collision, because an operator that loses silently is
indistinguishable from one that was never registered.
