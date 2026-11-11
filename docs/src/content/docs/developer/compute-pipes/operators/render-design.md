---
title: "How the render operator is designed"
description: "What the render operator does, why its input is a serialised column, the worked example that ships, and the measurement it exists because of."
kind: explanation
sidebar:
  order: 2
---

**The operator *renders*; what it applies is a *text template*.** That sentence is here rather than
further down because `template` is an overloaded word in this repository and a reader who arrived
from the wrong directory should find out immediately:

| If you came looking for | You want |
|---|---|
| a **configuration** template that projects into a `.pc.json` — `qc_metrics.template.json` and its siblings | `tools/cpipes_contract/templates/`, and the escape body at `jetsclient_ide/src/cpipes/templateApply.ts`. Nothing on this page. |
| a **prompt** template for a model call | `prompt_templates` and the infer operators — [the inference operators page, §2.3](/docs/developer/compute-pipes/operators/infer/#23-where-a-prompt-is-declared) |
| a **text** template that turns a record into prose | this page, and [the render operator reference](/docs/developer/compute-pipes/operators/render/) |

The token is `render` and not `template` for exactly that reason (`RenderOperatorType`,
`pipe_transformation_render.go`). The array is `text_templates` and not `templates` because
`prompt_templates` is already qualified (`TextTemplates`, `pipes_model.go`) — a bare `templates`
would have been the one unqualified template array in the configuration, and the one that collides.
`template` keeps meaning the artefact and `render` names the act, so neither word does two jobs.

| Document | Covers |
|---|---|
| **this page** | What the operator does, the worked example that ships, and the measurement the operator exists because of |
| [The render operator](/docs/developer/compute-pipes/operators/render/) | Its configuration, the notation and the failure model |
| `jets/agentic/template/doc.go` | The engine's own doc block: what it refuses to know, and why its `Render` has no error to return |
| [The inference operators](/docs/developer/compute-pipes/operators/infer/) | The operator this one is the deterministic alternative to; the measurements §6 below quotes are in [What the inference measurements showed](/docs/developer/compute-pipes/operators/infer-measurements/) |

---

## 1. What the operator does

For each record arriving on the input channel:

1. Read the input column and decode it — json or toon — to a `map[string]any` (`documentOf`,
   `pipe_transformation_render.go`).
2. Render the compiled template against that document (`Render`,
   `jets/agentic/template/render.go`).
3. Write the text to the output column of **the same record** and forward it (`Apply`,
   `pipe_transformation_render.go`).

**This is the augmentation pattern, so the input and output channels must share one `ChannelSpec`**,
and the operator checks it with the infer operators' own function and their own message
(`validateInferChannels`, `pipe_transformation_infer.go`). A record shorter than the channel is
grown with nils before the column is assigned rather than written past its end: short rows are real
here, which is what `pad_short_rows_with_nulls` exists for.

**There is no worker pool, and the absence is deliberate.** The infer operators pool because a
record costs a network round trip; a render costs a walk over a compiled span list, with no lookup,
no parsing and no allocation of the template. A pool would buy contention on the output channel and
a second place for render state to be shared. The compiled template is immutable and safe for
concurrent use (`Template`, `jets/agentic/template/compile.go`) — which is a property worth
having so that one compiled template serves every worker node, not a reason to spawn goroutines
inside one operator.

### 1.1 Why the input is a column and not an entity

The briefing this operator generalises is rendered today *inside* the jetrules column encoder, from
`extractAsEntity`'s map, when a `column_encodings` entry names `entity_encoding: "briefing_prose"`
(`EncodeColumnData`, `jetrules_extract_entity.go`). **An operator cannot hang there.** What
crosses a channel is a record — a `[]any` of column values — and the RDF session that map was walked
from is gone by the time any downstream operator sees anything.

So the input is the serialised entity as a column, decoded back to a document. **Decoding costs
nothing**, because the engine was written for both shapes: from `extractAsEntity` a value is a
string, an int, a uint, a float64 or a `time.Time`; from a decoded json or toon document every
number is a float64 and every date is text (`scalar`, `jets/agentic/template/value.go`), and
`asInt` truncates a float for that reason (`asInt`, `jets/agentic/template/value.go`).

**The consequence worth stating is that this operator and the infer operator have the same input
contract**: one channel, one column, one serialised entity. That is what lets two versions of a
pipeline differ in their operator block and in nothing else — which is the discipline that makes a
comparison between a model arm and a template arm mean anything.

---

## 5. The worked example that ships

`jets/agentic/briefing/briefingtmpl/patient_profile_briefing.json` is the `patient_profile` briefing
as a template document — the one hand-edited copy, embedded into the binary (`Document`,
`jets/agentic/briefing/briefingtmpl/briefingtmpl.go`) and inlined into the pipeline's
`text_templates` array, with a test comparing the two.

**Three elements at the root**: a notice, a rule, and a group carrying the body — inside which one
further group holds the three body paragraphs under a single guard. **The two most briefing-shaped
things in the artefact turned out to be a substitution and a literal** — the notice is
`{{Briefing_Disclaimer require '…'}}` and the rule is 68 hyphens — which is the strongest evidence
available that the engine needs no concept of either. The medications element, with its comment
elided:

```json
{
  "when": "has_Briefing_Pharmacy_Events[]|count > 0",
  "text": "Medications: {{each has_Briefing_Pharmacy_Events[] sep: '; ' last: '; and '}}{{Drug_Name require 'a pharmacy event of the briefing carries no Drug_Name; …'}}{{if Maintenance == 'Y' or Maintenance == 'y'}}, a maintenance medication{{end}}{{if Fill_Count != ''}}, {{Fill_Count|words}} {{Fill_Count|plural: 'fill'}}{{if Fill_Date[]|max|d_MMMM != ''}}, most recently {{Fill_Date[]|max|d_MMMM}}{{end}}{{end}}{{end}}."
}
```

and the whole document over the reference entity renders, byte for byte:

```
Informational only. Prepared from claims data for a call-centre
representative, who is not a clinician. This is not medical advice, not a
diagnosis and not a treatment recommendation, and it must not be used to make
or support a clinical decision. Downstream use is governed by the service
agreement.

--------------------------------------------------------------------

Two medical visits on record, between 10 June and 14 August 2025. Most recent
contact: Independent Laboratory, 14 August.

Conditions on record: Chronic viral hepatitis C, Alcohol dependence and
Cellulitis.

Medications: lisinopril, a maintenance medication, three fills, most recently
24 August; and traMADol HCl, one fill, most recently 2 July.
```

**Three things in that element are decisions rather than transcription**, and the file records all
three in its own `comment` fields, because once the document is inlined in a `.pc.json` the
configuration is the only place the argument for a predicate can live.

**The gate counts the event list rather than reading the projected `Pharmacy_Event_Count`**, because
the renderer it reproduces returns early only when the count is not positive *and* the list is empty
— so guarding on the count alone would emit `Medications: .` for a count with no list.
**`Maintenance` is matched against both cases**, `== 'Y' or == 'y'`, which is `strings.EqualFold`
over a trimmed value written out rather than approximated. **And `Fill_Count` falls back to the
number of fill dates** when the property is absent, which is the same fallback the volume element
makes for `Medical_Event_Count` — and there the fallback is load-bearing: without it a member with
events and no count fails every gate and the document says *No claims activity on record for this
member*, an affirmative false statement rather than an omission.

The cost of that fallback is visible in the document and is worth seeing before writing one: *this
property, or the length of that list if it is absent* is two complementary spans, so the clause is
written twice where the code it reproduces had one function. It is the notation declining an
eleventh construct and paying for it in the configuration, which is where a reader can see it.

**A worked example a test does not run is prose**, so the equivalence between this document and the
hand-written renderer it generalises is asserted on every run, over transcribed fixtures, byte for
byte, in `jets/agentic/briefing/briefingtmpl/equivalence_test.go`.

---

## 6. What this operator is for

**A deterministic renderer was measured more accurate than a 3B model at writing the same briefing,
over the same fact set, and that is why this operator exists.** Measured 2026-09-09/10 against the
`patient_profile` pipeline over a **curated 22-member population**; the full write-up is
[What the inference measurements showed, §3.5](/docs/developer/compute-pipes/operators/infer-measurements/#35-a-template-beat-the-model-and-that-is-the-finding-to-carry).

Both arms read **one briefing node** and differed only in the renderer:

> **The template produced a correct briefing for 22 of 22, in every one of three runs. The model
> produced at least one statement not true of the fact set for 10 of 22.**

**The countable failure is one-directional.** Six members carry no maintenance drug at all; the model
described all six as being on maintenance medication in **17 of 18 member-runs**. `N` becomes `Y` and
never the reverse, across three runs, which is what makes it a property rather than noise. The
template arm's maintenance test — a Go function then, this document's `{{if Maintenance == 'Y' or
Maintenance == 'y'}}` now — was right 22 of 22.

**And it happened under the flag built to prevent it.** The briefing carries *"Adherence_Ratio is
applicable only for maintenance drugs"*, and that sentence was in the prompt in **7 of the 7 cases**
where the model misread the thing it explains. A guardrail on the input does not guard the reader.

**The one thing the model did that a template cannot** — join a medication list to a diagnosis —
occurred **twice in 66 member-runs**, and was an unlicensed inference both times.

### 6.1 What that does not establish

- **Not that a model cannot write a briefing.** This model, this pipeline, this population did not.
  One model at 3B, 22 members, three runs.
- **Not that this notation generalises.** The whole evidence for a domain-free renderer is **one**
  briefing over **one** entity of **one** workspace's domain model, expressed as configuration. That
  is strictly better than the same briefing expressed as Go, and it is not the same thing as a second
  consumer. **The test an interface's author cannot run is the first extension by somebody else**,
  and it has not happened yet.
- **Not that a parser is free.** The renderer it generalises refused a template engine in terms, on
  the argument that a mistyped filter name is a failure *of the renderer* rather than of the data.
  That argument was about being a control in an experiment and lapses for a product; the property it
  protected is bought back by compiling at build time, which [§4.1](/docs/developer/compute-pipes/operators/render/#41-every-failure-the-engine-admits-is-a-compile-failure--with-the-three-exceptions-of-43) is the evidence for and
  [§4.3](/docs/developer/compute-pipes/operators/render/#43-three-things-that-are-not-compile-failures) is the price of.
- **Nothing here is a recommendation to prefer this operator over an infer step** for work that is
  not a projection of its input. Ask what the model adds that a deterministic renderer cannot; where
  the answer is *phrasing*, the template is more accurate and free.

### 6.2 Two things to watch in a shipped template

**The count of `require` modifiers against the number of properties whose absence would change the
artefact's meaning.** That is [N3](/docs/developer/compute-pipes/operators/render/#43-three-things-that-are-not-compile-failures)'s only instrument, and a template with no `require` at all is the
hazard rather than a clean one.

**The count of constructs, and separately the count of block forms.** Eleven constructs is a finding
worth writing down; fifteen means the notation has become the code it replaced with a parser in
front of it. The construct count is the number people watch, and **a fourth block form is the same
growth arriving where nobody is counting**.
