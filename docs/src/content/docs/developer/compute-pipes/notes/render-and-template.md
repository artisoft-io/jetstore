---
title: "The render operator, and the word template meaning two things"
description: "Why the operator is called render and its array text_templates, and three properties of its failure model worth knowing first."
kind: explanation
sidebar:
  order: 6
---

**Added 2026-09-11.** Full documentation:
[the render operator](/docs/developer/compute-pipes/operators/render/) — the configuration, the notation and the failure model —
and [how the render operator is designed](/docs/developer/compute-pipes/operators/render-design/), with the measurement behind it.

**`render` applies a *text* template to a record, and `template` already meant something else here.**
`tools/cpipes_contract/templates/` holds the **configuration** templates, which project into a
`.pc.json`; `prompt_templates` holds the prompts an infer step sends a model. So the operator token
is `render`, its array is `text_templates` (`TextTemplates`, `pipes_model.go`), and the sentence
to carry is **the operator renders; what it applies is a text template**. A reader meeting
`"type": "template"` in a `pipes_config` would have had no way to tell which sense was meant.

**Every failure the template engine admits is a *compile* failure, with three exceptions it names.**
`Compile` is the only function of `jets/agentic/template` that returns an error and `Render` returns
none at all (`Render`, `jets/agentic/template/render.go`), so a render-time failure cannot be
added without changing a signature and every call site. The three that are not compile failures — a
`require` a record violates, a prefixed entity, and a value of the wrong runtime kind, which renders
empty and silently — are enumerated rather than defined away, and the third is a cost rather than a
feature.

**`on_error: pass_through` leaves the output column unwritten, and that is not what the word means
elsewhere.** For `map_record`, `jetrules` and the infer operators there is nothing to write, so the
two readings coincide; a render is total, so *the record continues carrying that text* is an equally
natural reading and it is the one that puts a briefing an author declared invalid in front of a
reader under the **default** policy. The rendered text reaches nobody under any of the three
policies.

**The input is a serialised entity column, not an entity.** The briefing this generalises renders
inside the jetrules column encoder (`EncodeColumnData`, `jetrules_extract_entity.go`), and an
operator cannot hang there — the RDF session is gone by the time a record crosses a channel. The
consequence is that `render` and `infer` have the **same input contract**, which is what lets two
versions of a pipeline differ in their operator block and in nothing else.
