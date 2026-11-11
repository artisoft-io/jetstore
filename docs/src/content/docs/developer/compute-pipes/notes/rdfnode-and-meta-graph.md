---
title: "The two adaptors' RdfNode and the meta graph"
description: "Three differences between the Go and native adaptors: node equality, Value() for blank nodes and unsigned literals, and nodes made before the first rete session."
kind: explanation
sidebar:
  order: 10
---

**Recorded 2026-10-11.**

**Three differences between the Go and native adaptors, found by the rules unit test
harness and fixed together**. All three were
invisible to pipelines, which is why they lasted.

**`RdfNode.Equals` was false for a node and itself, on both engines.** The Go adaptor
compared `EQ`'s result with `rdf.TRUE()` by pointer, and `TRUE()` makes a new node on every
call. The native adaptor compared `*bridge.Resource` wrappers, and every read (`GetObject`,
an iterator's `GetSubject`) allocates a new one. **It became node identity on both**: the Go
`*rdf.Node`, and on C++ the engine's handle (`bridge.Resource.SameAs`), which is what it is
since the Go engine's retirement (October 2026). The C++ engine interns resources and literals, so identity is *same resource, or literal of the same type and
value*, which is what `Contains` and `FindSPO` match on. **There is no coercion**: `int 2`,
`uint 2` and `double 2` are three nodes, although the rules' `==` (`EQ`) says `int 2 ==
double 2`.

**Native `Value()` returned nil for a blank node or an unsigned literal**, because
`bridge.Resource.AsInterface` had no case for either. It now returns `rdf.BlankNode` and
`uint`, as the Go adaptor does. The same switch narrowed an int64 to a C `int`, through
`get_int_literal`; `GetInt` now reads `get_long_literal`. **Both getters are new C
symbols**, so a binary built from this commit needs a `libjets.so` built from it.

**On C++, a node made before the first rete session went into the meta graph.** The native
session's resource manager made its nodes through the *rete* session, and fell back to the
engine's meta resource manager while none existed. The meta graph locks when the first RDF
session opens, and the C++ manager refused by throwing, uncaught across cgo, so **the
process aborted**. A pipeline never met it because `executeRules` creates the first rete
session before asserting input; the harness met it at once. Now:

- the session's manager makes nodes through the RDF session, which is all the C calls ever
  needed (`create_resource(rdfs.hdl, …)`), so the order no longer matters;
- a locked C++ manager returns what it already holds and refuses only a new item, as the Go
  `ResourceManager` does (`insert_item`, `jets/rdf/r_manager.h`);
- every C entry point that writes to the meta graph catches and returns `-3`
  (`guard_meta`, `jets/rete/jets_rete_cwrapper.cpp`), so a refusal is an error in Go.

**What still holds: make the meta resources you need before the first RDF session.** `JetResources()` is created on first use through the meta graph, so
`RunRuleSets`' precondition stands; the consequence of breaking it went from an abort on
C++ to a refused node, whose `Value()` is nil (on both engines, while there were two).
