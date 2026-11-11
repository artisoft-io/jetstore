---
title: Add a screen or a user flow
description: Add a screen to the web app with its route, menu entry and help page, or a user flow as three documents, and run the checks that hold each to the rest.
kind: how-to
sidebar:
  order: 5
---

The web app (`jetsclient_ide/`) shows two kinds of thing. A **screen** is a React component with a
route of its own. A **user flow** is a wizard written as documents, not code: a flow (`.uf.json`), its
actions (`.ua.json`) and its forms (`.form.json`), run by the flow runner at `/flow/<key>`. Prefer a
flow when the task is a sequence of forms that end in server actions; write a screen when it needs a
layout or behaviour of its own.

Both were carried out on 2026-11-11: a route with no help entry was added to `App.tsx` and the help
map's test run, and a new flow was copied into a scratch workspace and run through the workspace
validator, valid and then broken. Both changes were then discarded. The web app's whole suite was
run first, unchanged: `npm test` in `jetsclient_ide`, 96 files and 1,836 tests passing, 26 skipped.

## Add a screen

### 1. Write the component and its route

Write the screen under `jetsclient_ide/src/screens/` (or a directory of its own, as `proposals/`,
`incidents/` and `runbooks/` are), taking the `ApiClient` as a prop as the others do. Then add a
`<Route>` for it inside the shell's route in `jetsclient_ide/src/App.tsx`:

```tsx
<Route path="fileLoaderStatus" element={<FileLoaderStatus api={api} />} />
```

Routes are relative to the shell at `/`. Keep `path="*"` last.

### 2. Give it a menu entry

The top bar is the `NAV` array in `App.tsx`, and the flows menu is `FLOW_MENU`. An entry is
`{ to, label, capability? }`:

```tsx
{ to: "/query-tool", label: "Query Tool", capability: QUERY_TOOL },
```

**`capability` hides a top-bar entry** from a user who does not hold it, and **disables a flows-menu
entry** with a title naming it (`NavItem` and `FlowMenuItem` in `jetsclient_ide/src/shell/AppShell.tsx`).
The menu is only what is shown: the server checks the capability on every action the screen calls,
and the entry should name the capability those actions require. A detail screen reached from another
one, such as a proposal from the list, has a route and no entry.

### 3. Give it a help page

The *Help* button opens the page of the documentation site that `HELP_TOPICS` in
`jetsclient_ide/src/shell/helpTopics.ts` names for the current route. Add a row with the route
exactly as `App.tsx` writes it:

```ts
{ route: "fileLoaderStatus", page: "user/screens/file-loader-status/" },
```

and write the page under `docs/src/content/docs/user/screens/`, with its screenshot spec under
`docs/screenshots/specs/` (see [Taking the documentation's screenshots](/docs/developer/contributing/screenshots/)).
A detail route shares its list's page.

**`helpTopics.test.ts` holds the map to the other two**: it reads every `path="…"` of a `<Route>` in
`App.tsx`, and fails when a route has no row, when a row names a route that is gone, when a page has no
file under `docs/src/content/docs/`, and when a route other than `*` is mapped to the User Guide's
landing page. With a `scratchScreen` route added and no row, it failed here with:

```text
FAIL  src/shell/helpTopics.test.ts > the help map's routes > maps every route of App.tsx
AssertionError: expected [ 'scratchScreen' ] to deeply equal []
```

### 4. Test and build

```bash
cd jetsclient_ide
npx vitest run src/shell/helpTopics.test.ts   # the help map, 46 tests here
npm test                                      # the whole suite
npm run build                                 # type-checks, and is the real check
```

Write the screen's own tests beside it, rendering it against a stubbed `ApiClient` as the existing
`*.test.tsx` files do. [Develop and test the web app](/docs/developer/testing/web-app/develop-and-test/)
covers running it against a local apiserver.

## Add a user flow

### 1. Write the three documents

A flow is three files with one key, in the `user_flows/` folder:

| File | Holds | Schema |
|---|---|---|
| `<key>.uf.json` | the states, each naming a form and an action, and the transitions between them | `jets/userflow/schema/userflow.schema.json` |
| `<key>.form.json` | the forms the states name | `form.schema.json` |
| `<key>.ua.json` | the actions the states and forms name | `action.schema.json` |

**Where the folder is decides whose flow it is.** JetStore's own flows are in
`jets/workspace_assets/user_flows/`, and `install_workspace_assets` writes them into every workspace;
a flow only one installation needs goes in that workspace's own `user_flows/`. Either way the runner
reads the documents from the workspace: a flow exists for a deployment **when its documents are in the
workspace** (`jetsclient_ide/src/userflow/routing.ts`). What JetStore ships is described in
[The user flows JetStore ships](/docs/developer/workspace/assets/user-flows/), and the actions in
[Flow actions](/docs/developer/workspace/assets/flow-actions/).

Start from the smallest shipped flow that does something like what you need and change it.
`workspacePullUF` is three states and a branch. Give the `.uf.json` a `title`: the runner shows it as the
flow's heading, and a titled JetStore flow must have a help page (below).

**An action that runs code names an escape**, and the escape must be registered in the build,
`productionRegistry` in `jetsclient_ide/src/actions/registry.ts`. A flow whose escape is not there
refuses to load, so a workspace flow can use only the escapes JetStore already has.

The schemas are emitted from the TypeScript in `jetsclient_ide/src/userflow/` and
`src/actions/` (`npm run emit-schema`), and `jets/userflow/schema/` holds committed copies for Go:
`TestSchemaCopiesMatchSource` fails when they differ. Edit the TypeScript, never the copies.

### 2. Validate it

A flow is checked three times, by two implementations:

| When | What | Where |
|---|---|---|
| saved in the Workspace IDE | each document against its schema, and the flow's states and transitions | Go, `jets/userflow`, through the save path's validator table |
| compiled | the documents against each other: forms and actions named by the flow exist, tables' actions resolve, every escape is in the build | the bundled validator `jets_validate_workspace.mjs`, run by `CompileWorkspace` (`jets/workspace/asset_validation.go`) |
| loaded | the same set check, before the runner starts | `FlowStore.load`, `jetsclient_ide/src/userflow/store.ts` |

Run the compile-time check yourself on a workspace folder. Build the validator with
`npm run build:validator` in `jetsclient_ide`, then:

```bash
node jetsclient_ide/dist-validator/jets_validate_workspace.mjs <workspace folder>
```

It exits 0 with no error, 1 with an error, and 2 when it cannot validate at all. Here a copy of
`workspacePullUF` as `scratchUF`, beside JetStore's shipped flows and tables, gave `0 error(s), 0
warning(s)`. With one state's `formConfig` changed to a form that does not exist it gave:

```text
ERROR user_flows/scratchUF.uf.json#/states/confirm/formConfig: state "confirm" names form "noSuchForm", which the form document does not define
```

and exit 1. A compile of a workspace with that flow fails the same way.

The Go half has its own tests: `go test -count=1 ./jets/userflow/` passed here, and
`TestShippingDocumentsPassTheSaveCheck` in `jets/datatable` runs every shipped `.uf.json` through the
save check and counts them, so **a new JetStore flow changes the count it expects**.

### 3. Give a JetStore flow its help page and menu entry

For a flow in `jets/workspace_assets/user_flows/`:

- add it to `FLOW_TOPICS` in `helpTopics.ts`, with a page under `user/screens/flows/` and a screenshot
  spec. `helpTopics.test.ts` fails on a shipped flow with a `title` and no row. A workspace's own flow
  needs no row: its help opens the flows overview.
- add it to `FLOW_MENU` in `App.tsx` if users start it from the menu, as
  `{ to: "/flow/<key>", label: "…" }`.
