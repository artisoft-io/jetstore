---
title: "The JetRules compiler"
description: "What the JetRules compiler does, and where each part of it is in jets/compilerv2."
kind: overview
sidebar:
  label: Overview
---

The compiler, in `jets/compilerv2`, turns a workspace's JetRules source (`.jr` files) into the RETE
network the rule engine runs, and writes it to the workspace's `workspace.db` (SQLite). The package
doc comments have the detail; these pages are the map.

- [How a workspace's rules are compiled](/docs/developer/engine/compiler/compilation/): read, parse,
  walk, build the network, analyse and save.
- [Reordering a rule's antecedents](/docs/developer/engine/compiler/antecedent-order/): the static
  score order, and the cost order computed from statistics.
- [JetRules as JSON](/docs/developer/engine/compiler/symbolic-rules/): the symbolic form, rendered to
  source for this compiler.

The operators a rule may call are listed in the generated
[JetRules operators](/docs/developer/reference/generated/jetrules-operators/) page.

## Where things are

Paths are relative to `jets/compilerv2/`.

| Path | What it holds |
|---|---|
| `main.go` | the command-line compiler for one rule file (`-f`, `-b`, `-s`) |
| `compiler/jet_rule_compiler.go` | `Compiler`: read, parse, walk, report |
| `compiler/rule_file_reader.go` | the main file and its imports, and the line mapping |
| `compiler/source_file_manager.go` | source file names and their keys in `workspace.db` |
| `compiler/jet_rule_listener*.go` | the model built from the parse tree, validation, optimization, the network |
| `compiler/jet_rule_listener_optimization.go` | the score order |
| `compiler/jet_rule_listener_cost_order.go` | the cost order |
| `compiler/workspace_db.go`, `workspace_db_helper.go` | writing `workspace.db` |
| `compiler/diagnostic.go`, `error_listener.go` | diagnostics, with authored file and line |
| `analyzer/` | the predicates a rule set reads and infers, and the rules a property depends on |
| `symbolic/` | JetRules written as JSON, rendered to source for this compiler ([JetRules as JSON](/docs/developer/engine/compiler/symbolic-rules/)) |
| `parser/` | generated from `compiler/JetRule.g4`; do not edit |
