# `jets/compilerv2/symbolic` — JetRules as JSON

The rules of a JetRule file written as a JSON document, rendered to JetRules
source and compiled by the unchanged compiler. It exists for an author that
reproduces JSON and not JetRules — a local model driving `compile_rule_file`
(jetstore_agentic_ai I-250, F176). The package doc comment (`symbolic.go`) has
the argument; this file has what cost digging. Newest first.

## What a small model did with it: the losses are in the envelope — 2026-10-12

Measured with `tools/tool_conformance` on `granite4.1:3b`, CPU, three repeats per run, six `rules`
probes that carry the same six rules as the six `rule_text` probes (full record:
jetstore_agentic_ai I-250). **Wherever the argument parsed, the rules arrived exactly as given — 83
calls of 83 over seven runs.** Everything lost was lost in how they were sent, and each argument shape
traded one such loss for the next:

| `compile_rule_file`'s `rules` argument offered as | what the model did |
|---|---|
| a whole document, with an inline JSON example in the description | sent it as a string in 14 of 28 calls, and in 11 of those turned the quotes around `?variables` into curly quotes — not JSON |
| a whole document, its shape in words | sent the document's own `rules` array as the argument in 10 of 12 calls: the argument and the field had one name |
| `rules` an array, `imports` beside it | put `{"imports": [...]}` inside the array, and copied the description's example path in as an import |
| the same, that last element read as the imports, no example | intact 12 of 16 and 10 of 15; every loss an import added to rules that need none |

So `ReadRulesArgument` (`jets/agentic/tools/compile.go`) accepts four forms, each with one reading, and
`tools/tool_conformance` judges a call by that function rather than by its own reading. **Two things
to know before changing the signature**: an example in a description is copied as data, and a field
name repeated one level down is collapsed. And the six probes are the ones the shape was fitted on.

**Translation is a different thing and fails.** Given JetRules *text*, the model answered through
`rules` 36 times; 32 were refused by the reader or `Render` — most often a rule name written
`[rule1]` — and none of the four that rendered changed a term. That is the case for refusing rather
than repairing: a brackets-and-all name is a loud error here and would be a silent one if stripped.

## The corpus round trip is the drift check, and what it found — 2026-10-12

`TestCorpusRoundTrip` compiles every main rule file the four workspaces name in
`workspace_control.json`, takes the rules to symbolic form (`FromModel`), renders
them, compiles the rendering in place of the original rules, and requires the
two compiles to agree rule for rule. **4,434 rule compiles over 53 main files**
on 2026-10-12, at cedargate_ws `eebe5bf`, jets_ws `0f19049`, usi_ws `57f2f2b`
and walrus_ws `dd13164` — a count of the workspaces, which this repository does
not control, so it is documentation and not an assertion. Run it with
`-count=1`: the workspaces live outside the Go module and the test cache does not
see them move. It skips a workspace that is not checked out beside
`jetstore_ai/`; `JETS_SYMBOLIC_CORPUS_DIR` points it elsewhere.

Three things it found, none visible from the grammar:

- **`T op unary(x)` is ambiguous.** The grammar admits a unary operator in three
  forms, one of them bare (`op arg`, `UnaryExprTerm3`), so
  `(T lookup create_literal(x))` parses as the unary `T lookup` combined by
  `create_literal` with `x`. The renderer writes `(op arg)`, which delimits
  itself — and is the form `usi_ws` already writes by hand. With `op(arg)`, 14
  rule compiles over four usi_ws main files differed; with `(op arg)`, none.
- **The compiler writes rules of its own.** `PostProcessClasses` adds one
  `ci_<class>` rule per class, marked `i=true`. They are not authored, the
  declarations regenerate them, and `FromModel` skips them; the form refuses an
  `i` property so an author cannot write one.
- **The compiler accepts two rules of one name; the form does not.** usi_ws has
  them (its assessment counts 29 of 743). A rule's name is what a diagnostic, a
  salience warning and a rete node's `rules` list identify it by, so a new
  authoring form refuses the duplicate. The round trip renames them and compares
  by position.

## Why `.jrs.json` and not `.jr.json` — 2026-10-12

I-250 proposed `.jr.json`. That is the name the compiler writes its output to
(`NewJetRuleListener`, `jet_rule_model.go`: the main file's name plus `.json`),
and every workspace `.gitignore` lists `*.jr.json`. An authored file of that name
would be ignored by git and overwritten by the next compile of a main file with
the same stem. The form is, for now, only the wire format of
`compile_rule_file`'s `rules` argument; nothing in a workspace reads a
`.jrs.json`, so the suffix is reserved rather than used.

## Why render rather than enter the compiler after the parser — 2026-10-12

I-250 measured that the compiler's post-parse passes have no parse-tree
dependency, so a JSON rule could be fed to them directly. Rendering was chosen
instead, because then the parser stays the authority: a renderer that drifts from
the grammar produces text the parser refuses, where a second entry point would
accept what the grammar does not. The renderer's own job is to make sure no
string from the document reaches the source unchecked — an id is matched against
the lexer's `Identifier`, a literal against its type's grammar, an operator
against `binaryOp`/`unaryOp`, and a reserved word is escaped (`ex:"date"`) or
refused. `reserved.go` is held to `JetRule.g4` by `TestReservedMatchesGrammar`.

## Literal values are the compiler's stored text, not unescaped — 2026-10-12

`StripQuotes` removes the quotes of a `STRING` and does not unescape it, so the
model stores `a\"b` for the source `"a\"b"`. A literal's `value` here is that
stored text, written back between quotes verbatim. Escaping it again would make
the round trip lose a backslash per pass.
