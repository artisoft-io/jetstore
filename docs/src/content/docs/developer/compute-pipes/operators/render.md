---
title: "The render operator"
description: "Configuration, notation and failure model of the render operator, which applies a configured text template to a serialised entity column."
kind: reference
sidebar:
  order: 1
---

A `render` step reads a serialised entity from one column of a record, renders a text template
against it, and writes the text to another column of the same record. **The template is
configuration**: it lives in the pipeline's `text_templates` array, and nothing in the engine that
reads it knows what a briefing, a member or a medication is.

What the operator does, why it is designed this way, the worked example that ships and the measurement
it exists because of are in [How the render operator is designed](/docs/developer/compute-pipes/operators/render-design/).

## 2. Configuration

### 2.1 `render_config` — nine fields

`RenderSpec` (`RenderSpec`, `pipes_model.go`), reached from a step as `render_config`
(`RenderConfig`, `pipes_model.go`).

| Key | Default | Meaning, and what happens when it is absent |
|---|---|---|
| `comment` | — | Free text for the reader; ignored by JetStore. Every sibling spec in this file carries one |
| `template_name` | *required* | Names an entry of the top-level `text_templates`. Absent is a build failure; **there is no inline alternative** — see §2.4 |
| `input_column` | *required* | The column carrying the serialised entity. Not a column of the input channel is a build failure listing the columns |
| `output_column` | *required* | Where the rendered text is written. **May not equal `input_column`** |
| `input_encoding` | the channel's, else `json` | `json` or `toon`. An **override**, not the answer — §2.3 |
| `row_key_column` | — | Identifies the record on an error row. Absent means the error row carries no `row_jets_key` |
| `on_error` | `pass_through` | `pass_through`, `drop` or `fail`. **What pass-through means here is not obvious — read §4.4** |
| `max_error_count` | 20 | Caps what reaches the log and the error channel, counted per **error** rather than per record. A count below 1 is replaced by the default |
| `error_channel` | synthesised | `{name, channel_spec_name}`. A step that names none gets one from the default synthesis (`reportsRowLevelFailures`, `error_channel_default.go`), so the absence of this key is not the absence of error reporting |

**`output_column` may not equal `input_column`**, and the refusal is worth a line because the
alternative reads as harmless. The operator would overwrite the entity it rendered from, so a second
operator reading that column would find prose where a document is expected, and the failure would
land on the second step with a message about the data.

### 2.2 `text_templates` — five fields

`TextTemplateSpec` (`TextTemplateSpec`, `pipes_model.go`), an array at the configuration root
beside `prompt_templates`, `lookup_tables` and `schema_providers`.

| Key | Default | Meaning |
|---|---|---|
| `comment` | — | Free text for the reader |
| `key` | *required* | The name a step's `template_name` says. Two entries sharing one is a build failure |
| `width` | *required* | The column each paragraph is wrapped to. **Required rather than defaulted**: the shape of the output depends on it, and a silent default is a byte difference nobody is looking for |
| `elements` | *required* | The document body, §3 |
| `empty` | — | What the document says when no element said anything. Absent means a document with nothing to say renders as nothing |

**The indirection is `prompt_templates`' move repeated**, and copying it was the point rather than a
convenience: a named document at the configuration root can be shared by several steps and pipes,
and a name that is not there fails at build time rather than on a node.

**Every declared document is compiled at startup, whether or not a step names it**
(`validateTextTemplates`, `pipe_transformation_render.go`). The operator alone would leave a
hole: a second entry carrying an unclosed `each` would sit in the configuration unexamined until
somebody pointed a step at it, which is a different day and a different pull request. A template
document is a constant of the configuration, so every one of them is answerable before a record
arrives.

### 2.3 The input encoding is a property of the channel

The column the operator reads was written by a `column_encodings` entry on the channel spec, which
already says whether it is json or toon (`ColumnEncodingSpec`, `pipes_model.go`). **So the
operator reads it off the channel and `input_encoding` is an override** for the case where nothing
says — a column arriving from a file, or from a `map_record` (`resolveRenderInputEncoding`,
`pipe_transformation_render.go`).

Asking an author to declare it on the step as well would be two declarations that can disagree.
Where both are present and do disagree, **the build fails**, both being constants of the
configuration:

```
error: render_config input_encoding is 'json' and the channel 'briefings.in' encodes the column
'briefing_input' as 'toon'; they are both constants of the configuration and one of them is wrong
```

Letting one win was the alternative, and it means every record of the run fails to decode with a
message about the data. **A `briefing_prose` column is a build failure too**, from the same lookup
for nothing: that column is already rendered prose and there is no document to render from.

### 2.4 A complete step

The fragment below is real: it decodes into a `ComputePipesConfig`, passes `validateTextTemplates`,
and the record beneath it was rendered by the operator built from it.

```json
{
  "text_templates": [
    {
      "comment": "One paragraph per thing a representative can act on.",
      "key": "member_briefing",
      "width": 78,
      "elements": [
        {
          "comment": "The notice. The engine has no idea what a notice is: it is a substitution carrying a require.",
          "text": "{{Disclaimer require 'the entity carries no Disclaimer'}}"
        },
        {
          "comment": "The body. The fallback is this group's, so the notice stays above it.",
          "empty": "No claims activity on record for this member.",
          "elements": [
            {
              "when": "Event_Count > 0",
              "text": "{{Event_Count|words|sentence}} {{Event_Count|plural: 'visit'}} on record{{with has_Events[]|latest_by: Service_Date}}, most recently {{Service_Date|d_MMMM}}{{end}}."
            }
          ]
        }
      ]
    }
  ],
  "pipes_config": [
    {
      "type": "fan_out",
      "input_channel": { "name": "briefings.in", "channel_spec_name": "briefings" },
      "apply": [
        {
          "type": "render",
          "render_config": {
            "template_name": "member_briefing",
            "input_column": "briefing_input",
            "output_column": "briefing",
            "row_key_column": "member_key",
            "on_error": "pass_through"
          },
          "output_channel": { "name": "briefings.out", "channel_spec_name": "briefings" }
        }
      ]
    }
  ]
}
```

Over a `briefing_input` column carrying

```json
{"Disclaimer": "Informational only. Not medical advice.",
 "Event_Count": 2,
 "has_Events": [{"Service_Date": "2025-06-10"}, {"Service_Date": "2025-08-14"}]}
```

the `briefing` column comes out as

```
Informational only. Not medical advice.

Two visits on record, most recently 14 August.
```

and over an entity carrying only the disclaimer, as

```
Informational only. Not medical advice.

No claims activity on record for this member.
```

**`on_error` is written out rather than inherited**, which is a habit rather than a requirement: a
setting nobody wrote is still deciding something, and this one decides what happens to a record the
template refused.

---

## 3. The notation

A template document is an **ordered array of elements, each carrying inline markup** — not one
string. The choice is consequential and the argument is worth the paragraph, because the opposite is
what a reader pictures.

Order is data rather than position in text, so moving an element is moving an array entry.
Conditional emission is a property of an element rather than of a span, so an element that produces
nothing takes its blank line with it by construction. The wrap is per paragraph and collapses
whitespace, so in a single string the only available paragraph delimiter would itself be
whitespace. And what a single string buys — that it reads like a template — is kept where it is
true: inside an element, words and substitutions genuinely interleave, and
`Conditions on record: {{...}}.` is a sentence with a hole in it and should look like one. **The
split is structure above the paragraph, text inside it.**

```
document := { "comment": "...", "key": "...", "width": <positive int>,
              "elements": [ <element>, ... ], "empty": "<markup>" }

element  := paragraph | group

paragraph := { "comment": "...", "when": "<predicate>", "text": "<markup>" }

group     := { "comment": "...", "when": "<predicate>",
               "elements": [ <element>, ... ], "empty": "<markup>" }
```

**Nothing distinguishes a paragraph from a group but which field is present** (`compileElement`,
`jets/agentic/template/compile.go`). An object carrying both `text` and `elements`, or neither,
is a compile failure. `when` is optional on both and absent means always.

### 3.1 Elements, groups and the empty case

A **paragraph** emits when its `when` holds and its rendered text is not blank. A **group** emits its
children in array order, each as its own paragraph; if none of them emitted and the group carries an
`empty`, the group emits that as one paragraph instead (`render`,
`jets/agentic/template/render.go`).

**A group whose `when` does not hold emits nothing at all, its fallback included.** The alternative
reading — that the fallback stands in for a suppressed group — would make `when` mean two things
depending on whether an `empty` is present.

**A document is a group**, so the fallback is one recursive concept rather than a document-level
special case (`compileGroup`, `jets/agentic/template/compile.go`). That is not tidiness: it is what lets a template put
a notice and a rule *above* a body that may have nothing to say. A flat element list cannot express
it, because the notice always emits and a document-level fallback would then never fire.

**Paragraphs are joined with one blank line and the separator is not configurable.** It is what the
artefact this notation generalises does, a configurable one buys nothing, and it is one more thing
to get wrong.

### 3.2 Paths

Paths are `briefing.Selector`, imported unchanged (`ParseSelector`,
`jets/agentic/briefing/selector.go`). `selector := segment ( "." segment )*` and
`segment := name [ "[]" ]`.

The reuse is the point rather than a convenience, and one rule is why. **A `[]` segment over a
scalar yields the scalar** (`Resolve`, `jets/agentic/briefing/selector.go`), because a multi-valued property holding
one value is serialised as a bare scalar — so a traversal insisting on an array is wrong about
exactly the record a guardrail is least entitled to be wrong about. A second implementation of the
walk would be a second place to hold that rule, and it is the one that would be got wrong.

What the import also buys is three refusals for nothing. **A path in a template document is a string
constant, so all three fire at compile time and none can reach a record**: an empty segment, a `[]`
anywhere but at the end of a segment, and a **numeric** segment. The last is load-bearing —
`events.0.code` is valid `output_mapping` notation and reads as though it works here.

**A path that is not there resolves to nothing rather than to an error**, which is what makes the
render total and is §4.3 below.

### 3.3 Markup — four forms and a modifier

| Form | What it is |
|---|---|
| `{{ chain }}` | substitution: the first value the chain yields, as text, or the empty string |
| `{{if pred}} … {{end}}` | the span emitted when the predicate holds |
| `{{with chain}} … {{end}}` | the span emitted when the chain yields a node, rendered against that node |
| `{{each chain sep: 'a' last: 'b'}} … {{end}}` | the span once per node, the results joined |
| `{{ chain require 'message' }}` | a substitution that also asserts |

**`if`, `with` and `each` are block forms and not constructs**, on a working definition worth
stating because a reader counting names should get the same number twice: *a construct is a total
function from a value to a value; a block form decides whether and how often a span is emitted.*
`require` is a modifier on the same definition — it changes no value and produces none.

**`with` is what makes a guard free.** Writing the same chain out once per substitution and guarding
each occurrence by hand is what the notation's own source sketch did, and it got it wrong: a span
scoped to a chain that yields nothing emits nothing, so `Most recent contact: , 5 May.` cannot
happen (`withSpan.render`, `jets/agentic/template/render.go`).

**`each` joins with `join`**, so a span that renders blank is skipped rather than leaving a
separator with nothing on one side of it (`eachSpan.render`, `jets/agentic/template/render.go`). Both `sep` and `last`
are required and are not defaulted, for `width`'s reason: a template that wants one separator
throughout writes the same literal twice, which is visible.

**Literals are written with single quotes**, and there is no escape for a quote inside one. The body
lives in a JSON string, so double quotes would need escaping on every `join`, every `plural` and
every equality; that cost is taken once and there is no case for paying it twice. What it cannot
express is a literal containing an apostrophe.

**`if`, `with`, `each`, `end` and `require` are reserved**, so a property spelled like one of them
cannot open a substitution. The escape is the notation's own idiom — a dotted path, or `[]`, neither
of which a keyword can carry.

### 3.4 The chain

**A chain alternates paths and constructs, separated by a vertical bar.**
`has_Events[]|latest_by: Service_Date|Care_Setting` is a chain of three: address a list, fold it to
one node, address into that node.

Which links may be paths is a rule the specification left open and the implementation had to settle,
because *an unknown construct name is a compile failure* and *any bare link may be a path* cannot
both hold — if any name is a path then no name is unknown, and `{{Order_Count|wordz}}` is a property
nobody has, rendering the empty string. The rule (`parseChain`, `jets/agentic/template/chain.go`):

- **the first link is a path**, there being no value before it but the context node;
- **a link directly after a path is a construct**, because a path after a path is a path — `a.b`
  rather than `a|b` — so an unrecognised name there is refused rather than silently resolving to
  nothing;
- **a link after a construct** is the construct of that name if there is one, and a path otherwise,
  which is what lets a chain address into the node `latest_by` yields.

The cost is one collision: a property spelled exactly like a construct cannot be a bare link after a
construct. It is reachable as `count[]` or as a dotted path, which no construct can carry.

### 3.5 The ten constructs

There is no eleventh, and adding one is the risk this notation is watched for: the fastest way to
make a template reproduce a hand-written renderer is to add whatever construct the next sentence
needs, and the result compiles, passes every test, and is the old code with a parser in front of it.
Every construct is **total**, and where totality costs something the table says what it does instead
of failing (`constructs`, `jets/agentic/template/construct.go`).

| Construct | Shape | Takes | Gives | Instead of failing |
|---|---|---|---|---|
| `words` | first | a number | text | spells 0 to 999 and renders anything else as digits; a value that is not a number at all renders as written (`words`, `jets/agentic/template/construct.go`) |
| `sentence` | first | text | text | returns the empty string unchanged (`sentence`, `jets/agentic/template/construct.go`) |
| `plural` | first | a number, one quoted word | text | appends `s` for any count but one (`plural`, `jets/agentic/template/construct.go`) |
| `d_MMMM` | first | a date | text | the empty string for a value that does not read as a date (`dMMMM`, `jets/agentic/template/construct.go`) |
| `d_MMMM_yyyy` | first | a date | text | the same (`dMMMMyyyy`, `jets/agentic/template/construct.go`) |
| `latest_by` | fold | a list of nodes, one path | a node | yields nothing when no node carries a readable date; resolves a tie to the first (`latestBy`, `jets/agentic/template/construct.go`) |
| `strip_code` | map | text | text | returns a value with no leading parenthesised code unchanged (`stripCode`, `jets/agentic/template/construct.go`) |
| `join` | fold | a list of text, two quoted separators | text | no items to `""`, one item to itself, an empty item skipped (`join`, `jets/agentic/template/construct.go`) |
| `count` | fold | any list | a number | an absent path is a list of none, so the answer is zero |
| `max` | fold | a list of dates | a date | skips values that are not dates; yields nothing when none is (`maxDate`, `jets/agentic/template/construct.go`) |

**The `Shape` column is what makes a chain checkable before a record arrives**, and it is the column
that does the most work. `first` takes the first value the chain yields and discards the rest — a
substitution is not a list operation. `map` applies to every value and yields a list of the same
length. `fold` consumes the list and yields one thing.

So `Condition_Summary[]|strip_code|join: ', ', ' and '` type-checks and
`Condition_Summary[]|join: ', ', ' and '|strip_code` does not, and the second is a compile failure
rather than a surprise. **Both `map` and `fold` need a list**, and the escape when you have a scalar
is two characters:

```
{{Drug_Name|strip_code}}      compile failure: "strip_code" is a map and is applied to a single value
{{Drug_Name[]|strip_code}}    correct on every record, because [] over a scalar yields the scalar
```

**That is the rule most likely to refuse a template a reader thinks is fine**, and it is kept rather
than relaxed, because relaxing a shape rule retires a whole class of compile failure rather than one
flag — and because it is what catches the typo it exists for: `Fill_Date|max` where `Fill_Date[]|max`
was meant would silently take one date and call it the latest.

**One direction widens and the rest do not** (`accepts`, `jets/agentic/template/construct.go`). Every document value
renders as text, so a text-taking construct over a number or a date is a no-op rather than a
mistake. Nothing else widens: a number is not recoverable from `d_MMMM`'s output. **And a chain of
paths alone carries no judgement at all**, because a path says nothing about what it addressed — so
the check refuses a contradiction and never an absence of information.

### 3.6 Predicates

```
pred := term ( ("and" | "or") term )*    with parentheses; `and` binds tighter than `or`
term := chain op literal
```

**The two families of operator read the value differently, and that is the whole of what an author
has to remember** (`eval`, `jets/agentic/template/predicate.go`):

| Operators | Read the chain's first value as | Take on the right |
|---|---|---|
| `==`, `!=` | trimmed **text** | a quoted literal |
| `>`, `>=`, `<`, `<=` | a **number**, and are false when it does not read as one | a bare number |

So `Fill_Count != ''` is *the property is there and is not blank*, and `Event_Count > 0` is *it is
there and reads as a number greater than zero*. A quoted literal after `>` is refused and a bare
word after `==` is refused, rather than either being coerced: an unquoted word on the right of an
equality would be a path compared against a path, which is a silently different question.

**Parentheses are in the grammar rather than left out.** Without them `a > 0 or b > 0 and c > 0`
groups as `a > 0 or (b > 0 and c > 0)`, and a template that means the other thing has no way to say
so. An unbalanced parenthesis is a compile failure; a precedence trap is not catchable at all.

**There is no negation**, and it is an absence rather than an oversight. `not` is neither a
construct nor a block form — it is a change to the grammar — and what it would buy is the ability to
say *this value does not read as a number*, which is a projection defect rather than a state worth
branching on.

### 3.7 Whitespace, and what a markup boundary decides

**Whitespace in a template is not whitespace in the output.** Every paragraph is wrapped to the
document's `width`, and the wrap collapses runs of whitespace through `strings.Fields` (`wrap`,
`jets/agentic/template/construct.go`). A line break inside a `"text"` value is a space; two spaces are one.

**So the only thing a markup boundary decides is whether there is a space there at all** — which is
what lets `{{if Care_Setting != ''}}{{Care_Setting}}, {{end}}{{Service_Date|d_MMMM}}.` render
correctly both with the setting and without it. Write `{{if …}}, between` closed up against what
precedes it; a stray space before a comma survives into the output as one.

**A word longer than the width takes a line of its own rather than being split**, so the wrap is
total over any input and never invents a hyphen.

---

## 4. The failure model

This is the operator's most unusual property and the reason the notation was specified before it was
built.

### 4.1 Every failure the *engine* admits is a compile failure — with the three exceptions of §4.3

**The design that enforces it is a signature rather than a convention.** `Compile` is the only
function in `jets/agentic/template` that returns an error (`Compile`,
`jets/agentic/template/compile.go`), and `Render` returns `(string, []Violation)` with no error
at all (`Render`, `jets/agentic/template/render.go`). A later maintainer cannot introduce a
render-time failure without changing a signature and every call site. The claim is the compiler's to
enforce, which is the only form of it worth making: *every failure is a build failure* is a property
of a whole package and nothing else in Go checks it.

The mechanism is not new here. `compileInferPromptTemplate` already walks a prompt's placeholders
against the input channel's columns and fails the build naming the unknown one
(`compileInferPromptTemplate`, `pipe_transformation_infer.go`). A template is a constant of the
pipeline configuration, so every question about how it is spelled is answerable before a record
arrives.

Twelve failures, each naming the construct and its position — the reader of the error is the author
of the template. The messages below are verbatim.

| | Failure | Example message |
|---|---|---|
| C1 | an unknown key on the document, a group or a paragraph | `while reading a text template: while reading a template element: json: unknown field "wen"` |
| C2 | an element carrying both `text` and `elements` or neither; a paragraph carrying `empty`; no `key`; an empty `elements` | `elements[0]: carries both text and elements; an element is a paragraph or a group` |
| C3 | `width` absent, not an integer, or not positive | `width is absent or is not a positive integer; it is the column each paragraph is wrapped to and is required rather than defaulted` |
| C4 | `{{` with no `}}`, or `}}` with no `{{` | `elements[0].text at offset 6: '{{' is never closed; a substitution ends with '}}'` |
| C5 | a block form with no `{{end}}`, or an `{{end}}` closing nothing | `elements[0].text at offset 0: {{if}} is never closed; a block ends with {{end}}` |
| C6 | an unknown construct name | `chain "Event_Count\|wordz": "wordz" is not a construct; the constructs are count, d_MMMM, d_MMMM_yyyy, join, latest_by, max, plural, sentence, strip_code, words` |
| C7 | a construct given the wrong number of arguments | `"plural" takes 1 argument(s) and is given 0, as "plural"` |
| C8 | a construct given the wrong **kind** of argument | `"latest_by" takes a path as argument 1 and is given the quoted literal 'Service_Date'` |
| C9 | a construct applied to a chain of the wrong shape | `chain "Fill_Date\|max\|d_MMMM": "max" is a fold and is applied to a single value; a fold needs a list, which a path writes with []` |
| C10 | a path that does not parse | `selector "events.0.code": "0" is an array index; a provenance selector addresses every element with [] rather than one by position` |
| C11 | a malformed predicate | `predicate "Care_Setting == Telehealth": "==" compares text and needs a quoted literal on its right, not "Telehealth"` |
| C12 | `require` with no message, or on anything but a substitution | `{{if}} carries a 'require'; require is a modifier on a substitution, which is the only form that reads a value` |

**C1 is strict decoding and it is the one place strictness costs something** (`decodeStrict`,
`jets/agentic/template/spec.go`): `comment` had to be added to the document types as a field,
because every other object in a `.pc.json` carries one and a template that could not would be an
inconsistency a reader reads as a mistake. What it buys is `wen` written for `when`, which is
otherwise a template that silently never fires.

**C9 is the one that can refuse a template a reader thinks is fine**, because it is the only check
that reasons about what a chain *means* rather than about how it is spelled. If it is ever relaxed,
this table loses a row and the change belongs here rather than quietly in the code.

### 4.2 The operator's own build failures

Three of them are the operator's rather than the engine's, and two more fall out of the same
reasoning for nothing. All are answered at build time.

| | Failure | Message |
|---|---|---|
| C13 | two entries of `text_templates` sharing a `key` | `text_templates carries more than one entry with the key 'member_briefing'; a step naming it would render whichever came first` |
| C14 | a step naming a template `text_templates` does not carry | `render_config refers to template_name 'no_such_template' which is not defined in text_templates, available templates are: member_briefing` |
| C15 | a step naming an input or output column the channel does not have | `render_config input_column 'x' is not a column of the input channel 'briefings.in', available columns are: …` |
| — | `input_column` and `output_column` naming the same column | `the operator would overwrite the entity it rendered from` |
| — | an `input_encoding` the step and the channel disagree about | §2.3 |

**The last two carry no number because the specification names three and the code has five.** They
are not extensions of the language; they are the same reasoning applied twice more — two constants of
the configuration contradicting each other is decidable before a record arrives, and refusing it costs
one comparison. Numbering them here would invent identifiers the specification that owns C1 to C15 does not
have.

C13 is checked in **two** places — at startup over every declared document, and again in the
operator (`resolveRenderTemplate`, `pipe_transformation_render.go`). That is deliberate
redundancy rather than an oversight: a configuration that reaches a node by some path the startup
validation did not cover still may not silently pick one of two documents.

### 4.3 Three things that are **not** compile failures

The claim in §4.1's heading is worth exactly as much as this list, so none of the three is glossed.

**N1 — a `require` that a record violates.** It is decidable only against data. It *is* a run-time
failure of the step, and calling it anything else would be the kind of wording this repository's
editorial stance exists to prevent. What is bought is that the diagnosis is a **value** rather than a
control-flow exit (`Violation`, `jets/agentic/template/render.go`), so it costs the render nothing and the compiler
still enforces the signature. A violation carries the template's own message, the chain it was
written on, and — inside an `each` — which item it was about, because the message is a constant of
the template and the iteration is not:

```
a fill carries no Drug_Name (Drug_Name, item 1, at elements[0].text at offset 58)
```

**N2 — an entity whose root properties still carry a model prefix.** A property of the record. The
operator deliberately does **not** check it, and the reason is worth knowing because the check looks
free: `ParseSelector` refuses an empty segment, a numeric segment and a stray bracket, and **a colon
is none of those** — so `cintel:Medical_Events` is a legal path and a template written against an
unstripped entity is a legal template. An unconditional refusal would refuse a configuration that
works. What replaces it is already in the notation: when the root keys are prefixed every path
resolves to nothing, and a `require` on the first substitution fires. The diagnosis is weaker than a
dedicated check — it says *the entity carries no Disclaimer* rather than *the column needs
`remove_model_prefixes`* — and it is not silent, which is the property that mattered.

**N3 — a chain whose value is of the wrong runtime kind.** A service date that does not parse, a
count that is not a number. **This one is not a failure at all, and that is a cost rather than a
feature.** The construct renders the empty string, the enclosing span disappears, and nothing says
anything:

```
template  Seen {{Last_Seen|d_MMMM}}. Count {{Event_Count|words}}.
record    {"Last_Seen": "not a date", "Event_Count": 3}
output    Seen . Count three.
```

**The two halves of that line fail differently, and neither says so.** A date construct over a value
that is not a date yields the empty string and the clause collapses; `words` over a value that is not
a number renders the value **as written**, so a count of `"many"` reads *Count many.* Both are
deliberate — each construct states in §3.5 what it does instead of failing — and neither is
distinguishable, in the output, from a template that meant it.

The answer in the notation is `require` — **but only where an author wrote one**. A template
carrying no `require` at all is exactly this hazard, and it passes every compile check. Worse, the
notation cannot always express the assertion you want: `require` is a modifier on a substitution, so
**a template can only assert a property it also writes**, and a property read inside an element
gated on that same property is unassertable.

**A path resolving to nothing is not on this list**, because it is not a failure: it renders the
empty form the template specifies, and the notation adds nothing to what the selector already does.

### 4.4 `on_error`, and what `pass_through` means here

**A violated `require` fails the row**, rather than the column being written with a note beside it.
The engine leaves the cost to the caller, and this caller decides it costs the artefact, on the
grounds the hand-written renderer stated for its own refusals: *prose delivered without its notice is
the one output worse than no output* (`Render`, `jets/agentic/briefing/prose/prose.go`). The
third option — write the rendered text anyway and report beside it — is the only one under which an
artefact its author declared invalid reaches a reader, which is why it is refused rather than offered
as a setting.

A failed row is reported on the error channel and then governed by `on_error` (`reportError`, `pipe_transformation_render.go`, then `applyOnError`,
`pipe_transformation_render.go`), which is `map_record`'s vocabulary and shape:

| `on_error` | What happens to the record |
|---|---|
| `pass_through` (default) | forwarded with **the output column left as it arrived** |
| `drop` | not forwarded |
| `fail` | the error is returned from `Apply` and the pipeline fails |

**The default needs the argument spelled out, because the word has two readings here and only one of
them is safe.** Everywhere else `pass_through` is used — `map_record`, `jetrules`, the infer
operators — the two coincide: there is nothing to write, so the record continues *without the
enrichment*. **A render is total**, so the operator is holding rendered text even for a record that
violated an assertion, and *the record continues carrying that text* is an equally natural reading
of the same word.

**It is the reading that reverses the guardrail the answer was chosen to preserve.** Under it a
partial briefing reaches a consumer under the **default** policy, having been reported to a table
nobody reads in time. So the operator implements the first reading: the rendered text reaches nobody
under any of the three policies, the default matches its siblings rather than being a special case a
reader must know about, and `drop` and `fail` keep meaning what they mean elsewhere. An author who
wants the row gone rather than merely un-enriched writes `drop`; one who wants the run stopped
writes `fail`.

The same three policies govern the other row-level failure, which is **the input column not decoding
to a document**. Blank is a failure rather than an empty document, and the distinction is not
pedantry: a null column is what a record gets when the step that was meant to serialise the entity
did not run, and rendering a template's `empty` form for it would report a member with nothing to
say. What the error row looks like, with `row_key_column` set:

```
row_jets_key   900123457
input_column   briefing_input
operator_type  render
error_message  render operator (template 'member_briefing'): the input column 'briefing_input'
               is not valid json: invalid character 'o' in literal null (expecting 'u')
```

**Errors are logged whether or not an error channel is configured**, and `max_error_count` caps both.
One row per **violation** rather than one per record, so an `each` over ten items that all fail says
which ten. **`max_error_count` counts those rows and not the records that wrote them**, so that
record spends ten of the budget — which is `map_record`'s behaviour as well, its own count being per
column evaluator. It caps reporting and nothing else: the eleventh failed record is still dropped,
passed through or fatal exactly as `on_error` says, silently.

**The record itself is forwarded once, whatever it did wrong** — which is the other count, and it is
the one that belongs to the policy rather than to the diagnosis. A record with two violations writes
two error rows and continues **once** under `pass_through`, nothing under `drop`, and nothing under
`fail`, which returns. **One input record leaves the step as at most one output record, under every
policy.**

**~~And that last sentence has a defect behind it~~ It had one, and this paragraph now describes the
repair rather than the workaround.** Written on 2026-09-11 while this page was being drafted, it read:
*`Apply` calls the reporting path once per violation, and that path is also where `pass_through`
forwards the record — so a record carrying two violations is forwarded twice under the default
policy*. Measured, not reasoned: a two-item `each` whose items both violate produced 2 error rows and
**2 identical output records**. It was recorded as a defect rather than repaired on the
spot, and the advice was to run such a template under `drop` or `fail` until it was fixed.

**Fixed the same day.** Reporting and policy were one function; they are two now — `reportError`
writes the row and counts it, `applyOnError` decides what the record costs, and `Apply` calls the
first once per violation and the second once per record. **The workaround is retired: the default is
safe for a template with any number of `require`s.** The regression test is
`TestRenderSendsAFailedRecordOnce`, which asserts the send count and the row count for all three
policies against a record that fails twice — the case the suite could not see before, because its only
multi-violation test ran under `drop`, and **a policy that forwards nothing cannot observe a
duplicate**.

**The fix corrected a second count in the same place, and it is worth naming because it moved in the
opposite direction.** Under `fail` the old loop returned at the first violation, so a record that
failed three ways reported **one** row before stopping the run; it now reports all three and then
fails. *One row per violation* had been a claim about `pass_through` and `drop` only, and nothing said
so.
