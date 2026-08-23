# Projections — generated, and committed as evidence

`cpipes-contract templates --project <dir>` writes these. They are the output of M.4 and
M.5, not an input to anything: the generator reads `templates/*.template.json` and its
bindings, and the IDE reads the documents.

**Committed rather than generated on demand**, for the reason the fixtures beside them
are: they are what `ui_refresh` runs through their validation layers, and a review of a
generator is much harder than a review of what it emits. Regenerate after any change to
`project.py` and commit the diff with it — a change in the generator that does not move
these files is a change with no observable effect.

## Four files per template, and the fourth is ours alone

**It was two until M.5, and two did not load** (I-84). `FlowStore.load` reads
`<key>.uf.json` *and* `<key>.ua.json` in one `Promise.all` and throws before it reaches
the escape registry, so a pair fails at *read* rather than at the missing registration
M.4 predicted.

| File | Whose | What it is |
|---|---|---|
| `<t>.uf.json` | UserFlow | The state graph. One state per **fill**, not per hole. |
| `<t>.form.json` | UserFlow | One form per state. |
| `<t>.ua.json` | UserFlow | One action of one step: the `cpipesTemplateApply` escape. |
| `<t>.apply.json` | **ours** | The skeleton and what the escape needs to substitute into it. |

**The fourth is deliberately not a UserFlow document.** No schema of theirs describes it
and nothing in `jetsclient_ide/src/userflow` reads it — only
`src/cpipes/templateApply.ts`, which this project owns. That is what keeps the
interpreter ignorant of templates, as the 2026-08-20 gate settled.

**The apply substitutes; it does not expand.** The generator emits the expanded config
with a marker wherever a collected value belongs, so the wizard's output is the
expander's output by construction. There is no expander in Go and none in TypeScript, and
writing a second one is the drift M.4 refused when it made the projection a consumer of
`expand`'s traversal rather than a second traversal beside it.

## What each projects to

| Template | States | Longest walk | What the walk is |
|---|---:|---:|---|
| `map_claim_load_stages` | 10 | 10 | one binding step, then three load stages, each a `ConditionalPipeSpec` and its nested channel |
| `qc_metrics` | 119 | 34 | one binding step, nine column mappings — each a variant choice and its form — and the partition writer |
| `qc_report` | 1 | 1 | its bindings, and nothing else: all eleven of its holes are loops, so it asks a filler for nothing (I-76) |

**The gap between 119 and 34 is the point rather than a cost.** A variant chooser
enumerates what an author *could* pick; the walk counts what they *must*. `ColumnMapping`
has eight variants, so nine fills of it produce nine choosers and seventy-two branch
states, of which an author visits nine.

**`qc_report` projects to one state and that is correct.** Its configuration lives in a
466-line bindings file, and eight of those bindings are contract-typed objects that were
never declared as holes (I-78). The projection reports them and renders none of them —
they are a template defect rather than a case to accommodate.

## `qc_metrics.demonstrated.pc.json` — M.5's evidence

**Not a projection: the config one came out of.** It is what
`jetsclient_ide/src/cpipes/templateApply.test.ts` produced by walking the `qc_metrics`
projection with the shipped engine, the shipped action interpreter and the shipped
`validateForm` — 24 steps, `select` chosen for all nine column mappings, every required
field filled and no optional one. `tests_project.py` validates it against the full cpipes
contract, which is the one layer no UserFlow schema can reach.

**Every value is the form key that produced it**, so the artefact reads as a map from
wizard step to config site. That is deliberate: it is committed to be read.

**The check earned itself on the first run.** The wizard emitted a `partition_writer`
operator with **no `type`** — the property is `const` in the contract, so the projection
treated it as already-supplied by a variant chooser, which is true of a union and false of
a single-concrete hole. All four flow-document layers passed that config; the contract is
what saw it.
