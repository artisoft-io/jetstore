---
title: Web app reference
description: The source layout of jetsclient_ide, and the server contract the web application depends on.
kind: reference
sidebar:
  order: 1
---

## Layout

Every directory under `src/`, as of 2026-11-06. A user flow's own documents are not here: they are
workspace assets, under `jets/workspace_assets/`, installed into each workspace and read from there at
run time.

```
src/
  App.tsx                  routes and nav: every screen, flow and supervision page is mounted here
  main.tsx, base.ts        entry point; BASENAME, which must agree with vite's `base` and the Go prefix
  styles.css               the app's stylesheet: one palette, shared with the editor theme
  actions/                 the action grammar (schema.ts) and its interpreter, escapes and registry
  actions/fixtures/        the 455 Flutter constant values, frozen for the name-for-value check
  api/                     HttpClient (bearer auth, per-response token refresh) and workspace wrappers
  components/              file tree, login, register, and the copy-by-hand text dialog
  cpipes/                  the `cpipesTemplateApply` escape for projected template flows
  datatable/               the table document (table.ts), query builder, data table and its action bar
  datatable/fixtures/      the legacy Flutter-generated table and form corpus, and what it still builds
  datatable/tables/        the .tc.json tables the screens draw, bundled (a flow's tables are installed)
  detection/               the Detection screen: detector runs, and run-now
  editor/                  CodeMirror view, JetRules mode, file extension -> language, theme
  incidents/               the incident list and detail, over /agentic
  incidents/__fixtures__/  an incident as triage writes it, for the screen tests
  proposals/               the change-proposal staging screens, over /agentic
  runbooks/                the Runbooks and Run screens, over /runbook
  screens/                 the routed screens: Home, the Workspace IDE, FlowRunner, admin, Query Tool, …
  screens/documents/       the .ua.json and .form.json the screens run, bundled rather than installed
  screens/fixtures/        the legacy Flutter-generated screen corpus and reachability map
  screens/views/           the compiled views' tab declarations (data model, JetRules, lookups)
  shell/                   app shell: session, capabilities, notifications, prompts, title, favicon
  userflow/                the flow, form and document-set schemas, the engine, FlowStore, the validator
  userflow/fixtures/       the legacy Flutter-generated flow corpus, and what it still builds
  widgets/                 form widgets bound to form state: text input, dropdown, typeahead
```

## The server contract

Everything here mirrors Go source rather than a written specification, so when the server changes,
these are the things that break:

| This app | Server |
|---|---|
| `Authorization: Bearer <token>` | `ExtractToken`, `jets/user/token.go` |
| reads `token` from **every** response | `authh` mints a fresh one per request, `jets/apiserver/server.go` |
| `workspace_query_structure` | returns `{result_type, result_data}` of `wsfile.WorkspaceNode` |
| `get_workspace_file_content` / `save_workspace_file_content` | `jets/datatable/workspace_data_table_action.go` |
| `file_name` passed back **exactly as received** | the server url-escapes it when building the tree and unescapes it on read |
| local JSON check before save | `SaveWorkspaceFileContent` refuses a `.json` file that will not parse |

The refresh-per-response behaviour is the one most easily missed: a client that does not read `token`
out of each body will present a stale one and start getting 401s. The token is held in memory only,
never `localStorage`, so a reload costs one sign-in.
