# `user_flows` — the wizards JetStore ships for its own templates

`install_workspace_assets` writes these into every workspace, beside `data_model` and
`pipes_config`. They are the third asset group and the first whose contents are written
by a generator rather than by a person.

## What is here

`cpipes-contract templates --project jets/workspace_assets/user_flows` emits four
documents per template in `tools/cpipes_contract/templates/`:

| File | Whose schema | What it is |
|---|---|---|
| `<t>.uf.json` | `jets/userflow` | The state graph. One state per **fill**, not per hole. |
| `<t>.form.json` | `jets/userflow` | One form per state. |
| `<t>.ua.json` | `jets/userflow` | One action of one step: the `cpipesTemplateApply` escape. |
| `<t>.apply.json` | **none — it is ours** | The skeleton, and what the escape substitutes into it. |

The fourth is deliberately not a UserFlow document. Nothing under
`jetsclient_ide/src/userflow` reads it; only `jetsclient_ide/src/cpipes/templateApply.ts`
does, which is what keeps the interpreter ignorant of templates.

`FlowStore.load` reads the first three together (`jetsclient_ide/src/userflow/store.ts`,
`FlowStore.load`) and the escape reads the fourth at the end of the walk. A set with three
of the four does not run, which is why the embed glob names four suffixes.

## Why these are JetStore's rather than the workspace's

**Because a template is.** `qc_metrics.template.json` lives in this repository and is
versioned with the engine that runs what it expands to; a wizard generated from it is that
template rendered for a person, not content a workspace authored. Committing one into a
workspace would say the workspace wrote it, and the next regeneration would silently
disagree.

The same question was settled once already, one directory over: `data_model/jets_agentic.jr`
is emitted by `jets-agentic generate` from a model in `tools/`, and it is installed here
with a `DO NOT EDIT` header rather than committed as workspace content (A21.4). This is
that shape a second time.

**And the escape is resolved by the build, not by the workspace.** A `.ua.json` naming
`cpipesTemplateApply` loads only where `productionRegistry` carries that name; `FlowStore`
refuses the whole set otherwise. An artefact whose runnability is decided by the
application build belongs to the build.

## No ownership token, and it is not the `pipes_config` reason

`pipes_config` carries no token because a `.pc.json` should not have one. These carry none
because they **cannot**: all three UserFlow schemas set `additionalProperties: false` at
their root, so a bookkeeping field is refused on the save path — and the browser's Zod
objects are not strict, so it would be stripped there rather than reported. The manifest
(`jets_assets_manifest.json`) is therefore the whole of the guard's evidence, as it is for
`pipes_config`.

## Regenerating

    cd tools/cpipes_contract
    python -m cpipes_contract templates --project ../../jets/workspace_assets/user_flows

Commit the diff with whatever caused it. A change to `project.py` or to a template that
does not move these files is a change with no observable effect — which is why they are
committed rather than generated at build time, and it is the same argument the fixtures
beside the templates are committed under.

**What is not here is a `--check`.** `jets-agentic generate --check` asserts that
`data_model` holds what the generator would emit; nothing asserts that of this directory.
That is I-61's second asymmetry arriving at the moment it was raised for, and the check is
owed to `cpipes-contract` rather than to the installer.

## `qc_metrics.demonstrated.pc.json` is not here

It stayed at `tools/cpipes_contract/projections/`, because it is a config rather than a
document: the one the wizard produced when the shipped engine walked this projection. It is
evidence for the generator and would be wrong to install into a workspace's `pipes_config`.
