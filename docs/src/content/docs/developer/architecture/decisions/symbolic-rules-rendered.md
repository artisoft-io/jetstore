---
title: "Rules written as JSON are rendered to JetRules source"
description: "Why JetRules written as JSON are rendered to source and compiled by the unchanged compiler, rather than entering the compiler after its parser."
kind: explanation
---

*Decided 2026-10-12.*

## Context

A local model asked to write JetRules loses structural tokens: rule labels, import lines. The same
model writes a whole pipeline configuration, which is JSON, intact far more often. So JetStore accepts
rules written as JSON (`jets/compilerv2/symbolic`), through the `rules` argument of the
`compile_rule_file` tool. The compiler's passes after the parser were measured to have no dependency
on the parse tree, so a JSON rule could have been fed to them directly.

## Decision

Render the JSON to JetRules source and compile it with the unchanged compiler. The renderer writes
every structural token itself (the brackets, the import line, the arrow, the terminator), so the
author never writes one. No string from the document reaches the source unchecked: an identifier is
matched against the lexer's, a literal against its type's grammar, an operator against the grammar's
operators, and a reserved word is escaped or refused.

## Consequences

- **The parser stays the authority on what a rule file is.** A renderer that drifts from the grammar
  produces text the parser refuses, loudly; a second entry point would have accepted what the grammar
  does not, quietly.
- `TestCorpusRoundTrip` compiles every main rule file the workspaces name, renders it from symbolic
  form, compiles the rendering, and requires the two to agree rule for rule. That is the drift check.
- `reserved.go` is held to `JetRule.g4` by `TestReservedMatchesGrammar`.
- The form refuses what the compiler accepts but an author should not write: a duplicate rule name,
  and the `i` property of the rules the compiler adds itself.
- The file suffix is `.jrs.json`, not `.jr.json`, which is the compiler's own output name and is
  ignored by every workspace's `.gitignore`.

## Where the reason is recorded

- The package documentation of `jets/compilerv2/symbolic` (`symbolic.go`).
- [JetRules as JSON](/docs/developer/engine/compiler/symbolic-rules/), *Why render rather than enter
  the compiler after the parser*.
- Commit `9420f9fa`, *compile_rule_file: a 'rules' arm that takes JetRules as JSON*.
