---
title: "One rule engine, the C++ one"
description: "Why JetStore runs its rules on the C++ engine alone, and what retiring the Go engine changed."
kind: explanation
---

*Decided 2026-10-22.*

## Context

JetStore had two rule engines. The C++ one (`jets/rete` and `jets/rdf`, reached through `jets/bridge`
and `jets/compute_pipes/jetrules_native_adaptor`) is the one every deployment ran:
`cpipes_native_server` and the native node Lambda use it. The Go one (`jets/jetrules/rete` and
`jets/jetrules/rdf`) was maintained beside it, and was what a developer often tested with.

Nothing held the two to the same behaviour. Operators were registered in two factories, each with
its own implementation, and they were measured to disagree: on aggregates, on how a double literal
was stored, and on six operator names that only one engine knew. So a rule could pass every test on
the Go engine and fail, or compute a different value, on the deployed one.

## Decision

Keep one engine, the C++ one, computing what it computed. A benchmark of both engines settled their
differences value by value, each one decided, and the Go engine was then deleted. The last commit
with it is tagged `go-engine-final`.

## Consequences

- The rule *model* stays in Go (`rete_meta_store_model.go` and `workspace_control.go` in
  `jets/jetrules/rete`, `ast.go` and `datetime_parser.go` in `jets/jetrules/rdf`): the compiler writes
  `workspace.db` with it and the pipelines read their configuration through it.
- `cp_node`, the node Lambda of `cpipesSM`, carries no engine. A `jetrules` step there fails with
  `NoRuleEngineOnNode` (`jets/compute_pipes/jetrules_interface.go`), and the starter refuses such a
  configuration before the run (`validateNoRulesOnCpipesSM`). A rule pipeline's process names
  `cpipesNativeSM`.
- A configuration asking for the Go engine (`use_jet_rules_go: true`) is refused with a message
  naming the retirement; `false` is accepted, so existing configurations need no change.
- The compiler checks every operator name against the C++ engine
  (`jets/compilerv2/compiler/operators.txt`, kept equal to `expr_operator_factory.h` by
  `jets/rete/operator_list_test.cc`), so an unknown operator is a compile error rather than a failure
  at run time.
- The C++ tests that had Go twins are now the engine's specification.

## Where the reason is recorded

- [The retired Go engine](/docs/developer/engine/rule-engine/go-engine/): *The Go engine is retired*
  for the decision, and *The Go and C++ engines are independently maintained and they disagree*
  (2026-09-09) for the divergences that made two engines a liability.
- Commit `d7607680`, *Delete the Go JetRules engine*, and the tag `go-engine-final`.
