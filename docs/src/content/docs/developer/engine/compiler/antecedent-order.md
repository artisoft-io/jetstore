---
title: "Reordering a rule's antecedents"
description: "The compiler's static optimization by score, and its dynamic optimization by estimated rows from optimizer statistics."
kind: explanation
sidebar:
  order: 2
---

## The static optimization: the score order

**The order of a rule's antecedents decides how many partial matches the engine holds, never what the
rule computes.** Each antecedent is a node of the network, and each node holds the rows that match
the antecedents up to it. A selective term placed early keeps every later node small, and a term
that binds nothing new, placed early, multiplies them.

`OptimizeJetruleNode` (`jet_rule_listener_optimization.go`) reorders each valid rule, greedily:

- **The next term is the one with the best score** (`evaluateScore`), given the variables the terms
  already placed bind: the sum of a score for its subject, its predicate and its object.

  | | bound variable | constant | unbound variable |
  |---|---|---|---|
  | subject | 240 | 100 | 20 |
  | predicate | 200 | 140, and 240 for `rdf:type` | 0 |
  | object | 100 | 120 | 20 |

  So the terms that score highest join on a bound subject, or anchor on `rdf:type` with a constant
  class.
- **A negated term binds nothing**, so it is placed only once the terms before it bind every
  variable it reads.
- **Ties keep the authored order**: the sort is stable.
- **Filters are reattached** to the first antecedent at which all their variables are bound, several
  joined with `and`.
- **The reordered rule is validated again**, as the authored one was. An order that would read a
  variable before binding it is a compile error, not a silent change.

A rule that sets `o=false` keeps its written order: the author's choice, for a rule hand-ordered
better than the heuristic does it.

## The dynamic optimization: the cost order, from statistics

**The score knows the shape of each term and nothing about the data.** When the workspace holds
statistics, the compiler orders the rules they cover by the rows each antecedent is **estimated**
to add, from counts of what the sessions' graphs actually hold (`jet_rule_listener_cost_order.go`).

- **The files.** `jet_rules/optimizer_statistics.json`, committed, and one file per statistics
  process, `jet_rules/<process>.optimizer_statistics.json` (`optstats.ReadWorkspace`). A process's
  file takes precedence for its rule sets. The same rule set in two processes' files gets no
  statistics, so it keeps the score order, and the compile warns, naming both files.
- **When it applies.** A rule is cost-ordered when a file has its rule set and covers every
  positive antecedent's predicate, and the rule does not set `o=false`. Every other rule keeps the
  score order above. Without a file nothing changes: `workspace.db` is byte-identical.
- **The same guarantees as the score order**: negated terms after their bindings, stable ties, filters
  reattached, every reordered rule validated again.
- **What it did is logged.** Each rule set gets a line in the compile log: `Cost order: rule set …,
  statistics …: N rules cost-ordered (M differ from the score order); fell back to the score
  order: …`.
  `JETS_COST_ORDER_TRACE=1` lists each rule's order and its estimated rows.
- **The format** is `jets/optstats`'s. A file is produced either by a bench run
  (`rules_bench -stats`; see the `rules_bench` command's documentation) or by a
  pipeline on a deployed stack: a jetrules step that sets `optimizer_statistics`, merged into the
  workspace, as that process's own file, by the `merge_optimizer_statistics` report. **How to
  produce the files, run the processes, and keep the statistics across deployments: the optimizer
  statistics page of the compute pipes documentation.**

Measure a workspace before committing its statistics. The cost order holds fewer rows wherever the
estimate is right, but a rule set can run slower under it; leave such a rule set out of the file,
and it keeps the score order.
