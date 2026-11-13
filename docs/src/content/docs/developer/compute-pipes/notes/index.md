---
title: "Engineering notes"
description: "Package-level discoveries about compute pipes: facts you cannot see from the code you are reading, each of which took real digging to establish."
kind: overview
sidebar:
  label: "Overview"
  order: 0
---

**Package-level discoveries.** Not API documentation — that belongs in the code, in doc
comments where `go doc` will show it. This section is for the facts you cannot see from the code you are
reading: how a mechanism behaves across process boundaries, what an environment variable is really
deciding, why an obvious-looking change is wrong. **Anything that took real digging to establish, and
that the next person would otherwise have to dig for again.**

Add a page here when you find one. The bar is *would I have saved a day by reading this?* — not *is this
interesting?* Cite the file and the identifier so the entry can be checked rather than believed, say what was
**measured** as opposed to reasoned, and date it, because a fact about deployment wiring goes stale
without announcing it.

In the order they were recorded:

- [How a compiled workspace reaches a running process](/docs/developer/compute-pipes/notes/workspace-version-chain/)
- [What a cpipes cold start fetches](/docs/developer/compute-pipes/notes/cold-start-fetch/)
- [The jetrules caches and the cold start of 2026-08-31](/docs/developer/compute-pipes/notes/jetrules-cache-race/)
- [Object properties are graph structure, not columns](/docs/developer/compute-pipes/notes/object-properties/)
- [The inference operators, and what a GPU week bought](/docs/developer/compute-pipes/notes/inference-gpu-week/)
- [The render operator, and the word template meaning two things](/docs/developer/compute-pipes/notes/render-and-template/)
- [A site operator runs in one of three processes](/docs/developer/compute-pipes/notes/site-operators/)
- [Context overwrites the schema event, and an unresolved bucket writes successfully](/docs/developer/compute-pipes/notes/context-and-buckets/)
- [A node's document is a marshal of the struct](/docs/developer/compute-pipes/notes/runtime-document-json-tags/)
- [The two adaptors' RdfNode and the meta graph](/docs/developer/compute-pipes/notes/rdfnode-and-meta-graph/)
- [The keys on a jetrules error row](/docs/developer/compute-pipes/notes/jetrules-error-row-keys/)
- [A workspace override and the version that names it](/docs/developer/compute-pipes/notes/workspace-override-version/)
- [A cumulative file, and why its flag is read at start](/docs/developer/compute-pipes/notes/cumulative-files/)
- [A map column: empty, rejected and absent values](/docs/developer/compute-pipes/notes/map-column-values/)
- [Reading a stage folder: the latest session, and the columns it was written with](/docs/developer/compute-pipes/notes/stage-session-reads/)
- The rule optimizer's statistics, 2026-10-24: a jetrules step can count, over its sessions, what
  the rule optimizer orders by; see [The rule optimizer's statistics](/docs/developer/compute-pipes/optimizer-statistics/).
