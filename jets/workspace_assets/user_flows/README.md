# `user_flows` — the flows JetStore ships

**Two kinds of document since 2026-08-26, and the title said one until then.** This
directory held only the wizards generated from JetStore's own templates; the eleven
flows ported out of the Flutter app were moved in beside them, so what the directory
is now is *every user flow JetStore owns*, generated or not.

`install_workspace_assets` writes these into every workspace, beside `data_model`,
`pipes_config` and `table_configs`. They are the third asset group and the first whose
contents are written by a generator rather than by a person — which is still true of
the projections and is not the whole of the directory any more.

A flow that draws a table needs that table's `.tc.json`, which is installed by the
`table_configs` group; see its README. `actions.md` here is the note on the `.ua.json`
documents, moved with them.

## What is here

**The eleven ported flows**, three documents each — `<key>.uf.json`, `<key>.ua.json`,
`<key>.form.json` — keyed by the flow key the React router serves them under and the
Flutter app hands off to (`jetsclient/lib/routes/migrated_user_flows.dart`). Two of the
three are emitted rather than hand-written: `jetsclient_ide/src/userflow/schema.test.ts`
writes the `.uf.json` from the Flutter corpus under `UPDATE_SCHEMA=1`, and
`jetsclient_ide/src/datatable/table.test.ts` writes their tables the same way. The
`.ua.json` and `.form.json` are authored.

**The three projected templates**, four documents each.
`cpipes-contract templates --project jets/workspace_assets/user_flows` emits them from
the templates in `tools/cpipes_contract/templates/`:

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

**For the projections, because a template is.** `qc_metrics.template.json` lives in this repository and is
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

**For the eleven, that last argument is the whole of it, and it is the stronger half.**
They are the application's own screens expressed as documents — every escape name they
use is registered in `jetsclient_ide/src/actions/registry.ts` and resolved when the
bundle is built, so a workspace that owned them could not change one without the change
being refused at load. Two of their three documents are emitted from the Flutter corpus
as well, which puts them in the same position as `data_model/jets_agentic.jr`: output of
a generator that lives in this repository.

**What a client does when they need one changed and cannot wait.** Edit it in the
workspace IDE and leave it there. The save is recorded in `jetsapi.workspace_changes` and
re-applied over the installed file at every container start, and `install_workspace_assets`
runs against a fresh copy of the workspace *repository* — so an override is invisible to
the guard and survives until it is deleted. What breaks that is committing it: the commit
action is `git add -A` and promotes the override into the repository, where the next
install refuses it by name. So: override freely, commit deliberately.

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
