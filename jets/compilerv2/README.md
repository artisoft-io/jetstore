# `jets/compilerv2` — the JetRules compiler

The compiler turns a workspace's JetRules source (`.jr` files) into the RETE network the rule engine
runs, and writes it to the workspace's `workspace.db` (SQLite). Added 2026-10-26; the package doc comments have the
detail, and this file is the map.

---

## 1. The compilation process

A workspace is compiled by `jets/workspace` (`compileWorkspaceV2`), which runs this compiler once for
each **main rule file**, the `rule_sets` of `workspace_control.json`, in sorted order so that two
compiles of one workspace write the same `workspace.db`. For each main rule file:

1. **Read** (`rule_file_reader.go`). The main file and every file it imports, recursively, are read
   into one buffer. The reader keeps the mapping from buffer lines to authored files, so a
   diagnostic names the file and line the author wrote.
2. **Parse** (`JetRule.g4`, `parser/`). An ANTLR grammar; `parser/` is generated from it by
   `compiler/build.sh`.
3. **Walk** (`jet_rule_listener*.go`). A listener over the parse tree builds the model: resources and
   literals, classes and their properties, lookup tables, rule sequences and the rules. When a rule
   ends (`ExitJetRuleStmt`) it is, in order:
   - given its properties: `o` (optimization, on unless `o=false`) and `s` (salience);
   - **validated** (`ValidateJetruleNode`). Every antecedent has a variable. A filter reads only
     variables bound by its antecedent or earlier ones, and a negated antecedent only variables
     bound before it. A consequent uses only variables the antecedents bind. Every operator is one
     the engine registers, and no filter writes to the graph;
   - labelled, with the authored and normalized forms of each term;
   - **optimized** (§2): its antecedents reordered, unless it is invalid or sets `o=false`.
4. **Build the network** (`BuildReteNetwork`, `jet_rule_listener_rete.go`). Once every rule is read,
   the rules' antecedents are joined into one network. Rules that begin with the same terms, by
   normalized label, share those nodes, and each rule's consequents hang off its last antecedent.
   That last node is shared only by rules of the same salience (`terminalReteNode`), so that each
   rule fires at the salience it declares.
   Salience is then checked: an aggregate in a consequent must fire after the rules that feed it.
5. **Analyze and save.** `analyzer/` lists the predicates the rule set reads and the ones it infers, and
   `workspace_db.go` (`SaveJetRuleModel`) writes the model and the network to `workspace.db`.
   `-s` on the command line (`main.go`) also writes the model as JSON.

The workspace compile then builds `lookup.db` from the lookup tables and archives what the
pipelines and reports fetch (`workspace.tgz`, `reports.tgz`).

## 2. The static optimization: reordering a rule's antecedents

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

## 3. The dynamic optimization: the cost order, from statistics

**The score knows the shape of each term and nothing about the data.** When the workspace holds
statistics, the compiler orders the rules they cover by the rows each antecedent is **estimated**
to add, from counts of what the sessions' graphs actually hold (`jet_rule_listener_cost_order.go`).

- **The files.** `jet_rules/optimizer_statistics.json`, committed, and one file per statistics
  process, `jet_rules/<process>.optimizer_statistics.json` (`optstats.ReadWorkspace`). A process's
  file takes precedence for its rule sets. The same rule set in two processes' files gets no
  statistics, so it keeps the score order, and the compile warns, naming both files.
- **When it applies.** A rule is cost-ordered when a file has its rule set and covers every
  positive antecedent's predicate, and the rule does not set `o=false`. Every other rule keeps the
  score order of §2. Without a file nothing changes: `workspace.db` is byte-identical.
- **The same guarantees as §2**: negated terms after their bindings, stable ties, filters
  reattached, every reordered rule validated again.
- **What it did is logged.** Each rule set gets a line in the compile log: `Cost order: rule set …,
  statistics …: N rules cost-ordered (M differ from the score order); fell back to the score
  order: …`.
  `JETS_COST_ORDER_TRACE=1` lists each rule's order and its estimated rows.
- **The format** is `jets/optstats`'s. A file is produced either by a bench run
  (`rules_bench -stats`, [`jets/cmds/rules_bench/README.md`](../cmds/rules_bench/README.md)) or by a
  pipeline on a deployed stack: a jetrules step that sets `optimizer_statistics`, merged into the
  workspace, as that process's own file, by the `merge_optimizer_statistics` report. **How to
  produce the files, run the processes, and keep the statistics across deployments:
  [`jets/compute_pipes/optimizer_statistics_readme.md`](../compute_pipes/optimizer_statistics_readme.md).**

Measure a workspace before committing its statistics. The cost order holds fewer rows wherever the
estimate is right, but a rule set can run slower under it; leave such a rule set out of the file,
and it keeps the score order.

## 4. Where things are

| Path | What it holds |
|---|---|
| `main.go` | the command-line compiler for one rule file (`-f`, `-b`, `-s`) |
| `compiler/jet_rule_compiler.go` | `Compiler`: read, parse, walk, report |
| `compiler/rule_file_reader.go` | the main file and its imports, and the line mapping |
| `compiler/source_file_manager.go` | source file names and their keys in `workspace.db` |
| `compiler/jet_rule_listener*.go` | the model built from the parse tree, validation, optimization, the network |
| `compiler/jet_rule_listener_optimization.go` | the score order (§2) |
| `compiler/jet_rule_listener_cost_order.go` | the cost order (§3) |
| `compiler/workspace_db.go`, `workspace_db_helper.go` | writing `workspace.db` |
| `compiler/diagnostic.go`, `error_listener.go` | diagnostics, with authored file and line |
| `analyzer/` | the predicates a rule set reads and infers, and the rules a property depends on |
| `symbolic/` | JetRules written as JSON, rendered to source for this compiler ([`symbolic/README.md`](symbolic/README.md)) |
| `parser/` | generated from `compiler/JetRule.g4`; do not edit |
