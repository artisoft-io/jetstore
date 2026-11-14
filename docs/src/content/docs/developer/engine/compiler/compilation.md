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
   diagnostic names the file and line the author wrote. Each line goes into the buffer as written,
   without the carriage return of a CRLF line end, so a string literal written across lines keeps
   its spaces and tabs (see [String literals](#string-literals)).
2. **Parse** (`JetRule.g4`, `parser/`). An ANTLR grammar; `parser/` is generated from it by
   `compiler/build.sh`.
3. **Walk** (`jet_rule_listener*.go`). A listener over the parse tree builds the model: resources and
   literals, classes and their properties, lookup tables, rule sequences and the rules. When a rule
   ends (`ExitJetRuleStmt`) it is, in order:
   - given its properties: `o` (optimization, on unless `o=false`) and `s` (salience), either of
     which may be written in quotes (see [Rule properties](#rule-properties));
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

Once every main rule file is compiled, the workspace compile saves the `rule_sequences` of
`workspace_control.json` (`SaveRuleSequences`): one `rule_sequences` row per sequence, and in
`main_rule_sets` the main rule files it runs, in order, each by its `workspace_control` key. A
sequence's `source_file_key` is the file it is declared in, `workspace_control.json`, which gets a
`workspace_control` row of its own, with `is_main` unset, when the workspace has a rule sequence.
A sequence naming a file that is not a compiled main rule file fails the compile.

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
stores a backslash followed by `t`. A rule property (`[R1, description="..."]`) is kept as written;
see [Rule properties](#rule-properties).

**A cast's value may hold parentheses.** In `text("...")`, `date("...")`, `int(...)` and the other
casts, the type is the word before the first `(` and the value is everything up to the closing `)`,
so `text("a(b")` is the text `a(b`. A cast the compiler cannot read is a compile error at its line.

**A resource name may be written with its name in quotes**, `acme:"has value"` or `rdf:"type"`, when
the name is a keyword or holds characters a name cannot (`EscR`, `jet_rule_listener_utility.go`):
the quotes are removed, and `acme:"has value"` names `acme:has value`. This applies to a name only.
A string literal or a cast in a term is read as this section describes, so `text("\"x:")` is the
text `"x:`.

**A string may be written across lines**, and keeps the line breaks and the spaces and tabs around
them: a literal opened with `"first ` on one line and closed with `   second"` on the next is the
text `first `, a line feed, then `   second`. A line end is a line feed whether the file has LF or
CRLF line ends. To write a tab or a line break without breaking the line, use `\t` or `\n`. A line
that begins or ends with whitespace the parser does not skip itself, such as a form feed or a
no-break space, is still trimmed of all of it, as every line used to be; inside a literal that
removes the spaces at that line's ends too.

**Changed 2026-11-14.** A cast's value was split on every `(`, so `text("a(b")` was read as nothing
and the term holding it was dropped without a message: an antecedent or a triple disappeared, and a
consequent or a filter lost its value. And every line was trimmed, so a literal written across lines
lost the spaces and tabs at the end of each line and at the start of the next. Neither changes what
a workspace that avoided both compiles to.

**Changed 2026-11-14, again.** The quotes of a name written `prefix:"name"` were removed from the
whole text of a term before it was read: an antecedent's subject, predicate and object, a
consequent's subject and predicate, and a triple's three. So a string literal or a cast there that
held `:"` lost every double quote, and `text("\"x:")` was the text `\x:`. A filter and a
consequent's value were read as written. And a consequent whose subject or predicate could not be
read, which happens when the parser recovers from a missing token as in `(?x  1)`, stopped the
compiler with a nil pointer instead of reporting the syntax error; it is now an error at its line
too. A workspace with no such literal compiles to the same `workspace.db`.

**And an antecedent or a triple with a term the parser's recovery left empty**, as in
`(?x  acme:A)` or `triple(acme:s, , 1)`, is now an error at its line too, naming the term and the
part that could not be read; until then it was dropped with only the syntax error to say so. A term
already refused with its own error, such as a bool literal the compiler cannot read, is not reported
a second time.

## Rule properties

A rule's properties are written in its header, `[R1, o=false, s=10, description="..."]`, each a
string, `true`, `false` or an integer. The compiler keeps each **as written**, quotes and escapes
included: in the rule's label and in `workspace.db`'s `rule_properties`, which is where the workspace
guide reads a rule's `description` from. The guide reads a quoted property with the escapes of a
string literal, as below, and shows a line break in it as a space
([Document a workspace](/docs/developer/how-to/document-a-workspace/#2-describe-the-rules)).

**It reads a property it acts on as the value it stands for** (`RuleProperty`,
`jet_rule_listener_validation.go`): a quoted value is read as its string, with the escapes of a
string literal, so `o="false"` is `o=false` and `s="10"` is `s=10`. The properties it acts on are
`o` (optimization: off for `false`, `f` or `0`, in any case), `s` (salience: an integer, refused
otherwise), `aggregate_check` (the aggregate salience check: off for `false`, `f` or `0`) and `i`,
which it writes itself, as `i=true`, on a class's generated `ci_` rule. Any other property, such as
`description` or `flag`, is kept for whoever reads it.

**Changed 2026-11-14.** The compiler used to read a property with its quotes, so `o="false"` left
optimization on, `aggregate_check="false"` left the check on, and `s="10"` was an invalid salience. A
workspace that writes these unquoted compiles to the same `workspace.db`.

**Changed 2026-11-13.** The compiler used to remove the quotes and nothing else, so `"\""` was stored
as a backslash and a quote, `"a\\b"` kept both backslashes, and `"\t"` was a backslash and a `t`.
A workspace whose strings contain `\"`, `\\`, `\t`, `\n` or `\r` compiles to different values
after the change; one that writes a backslash only before other characters, as regular expressions
usually do, compiles to the same `workspace.db`.
