# Projections — generated, and committed as evidence

`cpipes-contract templates --project <dir>` writes these. They are the output of M.4,
not an input to anything: the generator reads `templates/*.template.json` and its
bindings, and the IDE reads the pair.

**Committed rather than generated on demand**, for the reason the fixtures beside them
are: the pair is what `ui_refresh` runs through their four validation layers, and a
review of a generator is much harder than a review of what it emits. Regenerate after
any change to `project.py` and commit the diff with it — a change in the generator that
does not move these files is a change with no observable effect.

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
