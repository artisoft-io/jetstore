---
title: "How a workspace's rules are compiled"
description: "The steps the JetRules compiler takes for each main rule file, from reading the source to writing workspace.db."
kind: explanation
sidebar:
  order: 1
---

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
   - **optimized** ([Reordering a rule's antecedents](/docs/developer/engine/compiler/antecedent-order/)): its antecedents reordered, unless it is invalid or sets `o=false`.
4. **Build the network** (`BuildReteNetwork`, `jet_rule_listener_rete.go`). Once every rule is read,
   the rules' antecedents are joined into one network. Rules that begin with the same terms, by
   normalized label, share those nodes, and each rule's consequents hang off its last antecedent.
   That last node is shared only by rules of the same salience (`terminalReteNode`), so that each
   rule fires at the salience it declares.
   Salience is then checked: an aggregate in a consequent must fire after the rules that feed it.
5. **Analyze and save.** `analyzer/` lists the predicates the rule set reads and the ones it infers, and
   `workspace_db.go` (`SaveJetRuleModel`) writes the model and the network to `workspace.db`.
   `-s` on the command line (`main.go`) also writes the model as JSON.

   Everything saved for a main rule file is keyed by it: its network, its triples and its rules
   carry its `workspace_control` key in `source_file_key`, which is how the engine loads one main
   rule file's network. A rule in a file that two main rule files include is therefore saved under
   each. Two things record where the rules came from: `jet_rules.authored_file_key` is the file a
   rule is written in (the main rule file or an included file; a class's generated `ci_` rule is
   its class's file), and `main_support_files` lists every file the main rule file includes,
   directly or through another included file, in the order they were read. Every included file
   has a `workspace_control` row, with `is_main` unset unless it is a main rule file itself.

The workspace compile then builds `lookup.db` from the lookup tables and archives what the
pipelines and reports fetch (`workspace.tgz`, `reports.tgz`).

## String literals

A string in a rule file is written in double quotes. It is read the same way wherever it appears: a
`text`, `date`, `datetime` or `bool` declaration, a resource or volatile resource value, a rule's
terms (`"..."` or `text("...")`), a `triple(...)`, a compiler directive, a rule sequence's rule sets,
and a lookup table's file, key and column names. Inside the quotes a backslash starts an escape
(`UnquoteString`, `compiler/string_literal.go`):

| Written | Stored |
|---|---|
| `\"` | a double quote |
| `\\` | a backslash |
| `\t`, `\n`, `\r` | a tab, a line feed, a carriage return |

**Any other backslash is kept, with the character after it**, so a regular expression is written with
single backslashes: `"^(\d\d\d)"` stores `^(\d\d\d)`, which is what `literal_regex` receives. To
store a backslash before `"`, `\`, `t`, `n` or `r`, double it: `"\\d"` stores `\d`, and `"a\\tb"`
stores a backslash followed by `t`. A rule property (`[R1, description="..."]`) is the exception: the
compiler keeps its source text, quotes and escapes included.

**Changed 2026-11-13.** The compiler used to remove the quotes and nothing else, so `"\""` was stored
as a backslash and a quote, `"a\\b"` kept both backslashes, and `"\t"` was a backslash and a `t`.
A workspace whose strings contain `\"`, `\\`, `\t`, `\n` or `\r` compiles to different values
after the change; one that writes a backslash only before other characters, as regular expressions
usually do, compiles to the same `workspace.db`.
