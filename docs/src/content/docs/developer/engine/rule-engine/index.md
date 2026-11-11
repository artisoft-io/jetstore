---
title: "The C++ rule engine"
description: "JetStore's one rule engine, its packages, and how to read the pages on its semantics and their history."
kind: overview
sidebar:
  label: Overview
---

**JetStore has one rule engine, the C++ one.** Until October 2026 it had two: this C++ engine
(`jets/rete` and `jets/rdf`, built as `libjets.so` and reached through `jets/bridge` and
`jets/compute_pipes/jetrules_native_adaptor`), which every deployment ran, and a Go engine
(`jets/jetrules/rete` and `jets/jetrules/rdf`, reached through `jets/compute_pipes/jetrules_go_adaptor`).
The Go engine was retired, and its package's notes moved here with the retirement.

These pages record what you cannot see from the code in front of you: the engine's semantics and how
they came to be. Within a page, dated sections are newest first.

**How to read them.** They were written while both engines ran, and are kept as written apart from
tense: **what they say of the Go engine is history**, and a Go file or test they name is at the tag
`go-engine-final`, not in this tree. What they say of the C++ engine is current unless the section
says otherwise. Where the Go engine was the reference for a behaviour, the C++ engine's tests named
in the section are now its specification.

| Page | What it covers |
|---|---|
| [Retraction and truth maintenance](/docs/developer/engine/rule-engine/retraction/) | the one retraction behaviour, `Retract`, and the defects and switch the engine once kept |
| [Values and operators](/docs/developer/engine/rule-engine/values-and-operators/) | what sums, doubles, text, arithmetic, comparisons, ties, lookups and text read as a bool give |
| [The rete network at run time](/docs/developer/engine/rule-engine/rete-network/) | salience, a filter's re-entry, index removal, live iterators, vertex observations |
| [Limits, memory safety and the sanitizer build](/docs/developer/engine/rule-engine/limits-and-memory-safety/) | the default vertex visit limit, leaks and a crash fixed, the sanitizer gate |
| [The retired Go engine](/docs/developer/engine/rule-engine/go-engine/) | why there is one engine, and what differed between the two |

The operators the engine registers are listed in the generated
[JetRules operators](/docs/developer/reference/generated/jetrules-operators/) page.
