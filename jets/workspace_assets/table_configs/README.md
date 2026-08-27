# `table_configs` — the table documents JetStore's flows draw

`install_workspace_assets` writes these into every workspace beside `user_flows`, and
`FlowStore.loadTables` reads them from there by the key a form's `dataTable` field
names. They are the fourth asset group, added 2026-08-26 with the eleven ported flows.

## Why a group of its own rather than a fourth suffix under `user_flows`

Because the destination differs. `AssetGroup.Dir` is both where an asset lives here and
where it is installed inside the workspace, and a table document is read from
`table_configs/` — the convention `tablePath` states in
`jetsclient_ide/src/datatable/table.ts`. A `.tc.json` installed into `user_flows/` would
sit where nothing looks for it.

It is also the honest shape: **a table document is keyed by table, not by flow.** Two
flows may draw the same one, which is why there is no `<flow>.tc.json` and why editing
one in place is the thing the ownership guard exists to catch.

## What is here, and what is deliberately not

The 35 tables the eleven flows' forms name, computed from those forms rather than
chosen — the walk is `tableKeysOf` in `jetsclient_ide/src/userflow/store.ts`.

**The tables a *screen* draws are not here.** They stay in
`jetsclient_ide/src/datatable/tables/`, imported into the React bundle and never read
from a workspace. That is 29 documents, and the split is by consumer: a flow reads its
tables through the workspace, a screen has its own compiled in.

**One document is in both places.** `pipelineExecStatusTable` is drawn by `homeFiltersUF`
from the workspace and by the Home screen from the bundle, so it is committed twice.
`jetsclient_ide/src/datatable/table.test.ts` writes both copies from one emitter and
asserts they agree; `sharedTableDocuments.test.ts` asserts it directly, because two
copies kept in step by nothing is the failure this repository keeps finding. Whether the
Home screen should read it from the workspace instead — leaving one copy — is open.

## Regenerating

These are emitted from the Flutter corpus, not hand-written:

    cd jetsclient_ide
    UPDATE_SCHEMA=1 npm run emit-schema

which writes each document to whichever directory it is committed in. Commit the diff
with whatever caused it.
