# jetsclient_ide

The JetStore web application: a React + TypeScript single-page app built around
[CodeMirror 6](https://codemirror.net/), served by the Go apiserver at the root.

**The directory is named for what this was and not for what it is.** It began as
the Workspace IDE — one screen, mounted at `/ide/`, running beside the Flutter
client. Task X.1 retired that client on 2026-08-26 and X.2 moved this app to `/`,
so it is now the whole product: twenty screens, eleven user flows and the editor.
Renaming the directory would touch the Dockerfiles, the Go handler, the module
paths and every citation below, so it has not been done and is recorded rather
than left to be wondered about.

The editor is why it exists. The Flutter client's was a single `TextFormField`,
and Flutter lays a text field's whole document out on every frame rather than
virtualising by line, so cost grew with the size of the file instead of with the
part of it on screen — which is why `mapMenuEntry` refused to open anything at or
above 250,000 bytes, and why three real `.jr` files in the workspace corpus, the
largest 1.18 MB, could not be opened at all. CodeMirror virtualises by viewport,
so **this app has no size limit**; `src/api/workspace.test.ts` pins that with a
1.2 MB load.

## Citations into `jetsclient/`

**About 110 comments in this package cite `jetsclient/lib/...`, and that directory
no longer exists.** They are provenance rather than pointers: this app is a port,
and a comment saying which Dart function a behaviour was taken from is how a
reader checks that a reproduction is faithful — including the reproductions that
are deliberately *not* faithful, which are the ones worth arguing about.

They were not rewritten when the directory was deleted, because there is nothing
to rewrite them to. A line number cannot be repaired against a file that is gone,
and replacing 110 of them with "removed in X.1" would delete the information while
looking like maintenance.

To read one:

    git show b4cda702:jetsclient/lib/utils/constants.dart

`b4cda702` is the last commit that had the app — the parent of the deletion. Any
earlier commit works too; that one is quoted because it is the state every
citation here was written against.

**One of these citations is load-bearing rather than historical.** `constants.dart`
was read at test time to check that every key an action document names is a
declared constant's *value* — the name-for-value trap, which has caught this
project four times. Its 455 values are frozen in
`src/actions/fixtures/dart_constant_values.json`, extracted at that commit, and
the fixture explains what the check can no longer see.

## Layout

```
src/
  api/client.ts      HttpClient — bearer auth, per-response token refresh, 401 handling
  api/workspace.ts   typed wrappers over the workspace_* /dataTable actions
  editor/jetrules.ts CodeMirror mode for the JetRules DSL
  editor/language.ts file extension -> language
  editor/theme.ts    editor theme, driven by the same CSS variables as the shell
  editor/Editor.tsx  the CodeMirror view, wrapped for React
  components/        file tree, login
  App.tsx            shell: workspace picker, tabs, save
```

## The server contract

Everything here mirrors Go source rather than a written spec, so when the server
changes, these are the things that break:

| This app | Server |
|---|---|
| `Authorization: Bearer <token>` | `ExtractToken`, `jets/user/token.go` |
| reads `token` from **every** response | `authh` mints a fresh one per request, `jets/apiserver/server.go` |
| `workspace_query_structure` | returns `{result_type, result_data}` of `wsfile.WorkspaceNode` |
| `get_workspace_file_content` / `save_workspace_file_content` | `jets/datatable/workspace_data_table_action.go` |
| `file_name` passed back **exactly as received** | the server url-escapes it when building the tree and unescapes it on read |
| local JSON check before save | `SaveWorkspaceFileContent` refuses a `.json` file that will not parse |

The refresh-per-response behaviour is the one most easily missed: a client that
does not read `token` out of each body will present a stale one and start getting
401s. The token is held in memory only — never `localStorage` — so a reload costs
one sign-in.

## Build

```bash
npm install
npm run build       # tsc --noEmit && vite build  ->  dist/
npm test            # vitest
npm run typecheck
```

`dist/` is git-ignored. `npm run build` is the real check; `npm test` covers the
API client, the workspace wrappers, the language picker and the JetRules mode.

## Running it against a local apiserver

Two ways. Both talk to the same endpoints on the same origin, so the client code
does not know which one it is in.

### Served by the apiserver (what production does)

Point `IDE_APP_DEPLOYMENT_DIR` at the built bundle:

```bash
npm run build                     # the app bundle

go build -o /tmp/apiserver ./jets/apiserver

WORKSPACES_HOME=/home/michel/projects/repos/workspaces \
WORKSPACE=jets_ws \
IDE_APP_DEPLOYMENT_DIR=$PWD/jetsclient_ide/dist \
JETS_DSN='postgresql://...' \
API_SECRET='...' \
/tmp/apiserver -usingSshTunnel
```

Then open <http://localhost:8080/>. `-usingSshTunnel` is what puts the server on
`:8080` without TLS; without it the server listens on `:8443`.

**`WEB_APP_DEPLOYMENT_DIR` is gone** — X.1 deleted the flag with the Flutter
bundle it pointed at. A paragraph here used to explain that leaving it unset made
every Flutter url 404 while `/ide/` carried on, and that the asymmetry made it
look like the IDE had broken the other app. There is one bundle now, so there is
no asymmetry to be confused by.

The flag is `-IDE_APP_DEPLOYMENT_DIR`, and the environment variable of the same
name overrides it. If it points nowhere, every url answers 404 with a plain
message rather than failing obscurely, so an apiserver built without the bundle
still starts — and `TestRootIs404WithoutABundle` in `jets/apiserver` keeps that a
deployment error rather than a routing mystery.

### Vite dev server (hot reload)

```bash
npm run dev                                     # apiserver on :8080
JETS_API_ORIGIN=https://localhost:8443 npm run dev   # apiserver on :8443
```

Serves on <http://localhost:5173/> and proxies `/dataTable`, `/login` and the
other endpoints to the apiserver, so there is no CORS involved and the app still
sees a same-origin path. The proxy sets `secure: false` for the `:8443` case,
whose certificate is self-signed.

## In the container image

The bundle is built into `ui_service_ws` by the same three-image chain. It was
built alongside the Flutter one until X.1; the Flutter build step and the Flutter
toolchain in the base image went with it.

| Dockerfile | What it contributes |
|---|---|
| `Dockerfile.cpipes_base_builder` | Node.js 24.19.0, pinned to a release tarball under `/usr/local/node`, next to Go and Flutter |
| `Dockerfile.cpipes_builder` | copies `jetsclient_ide/` into the build image |
| `Dockerfile.ui_service_ws` | `npm ci && npm run build`, then copies `dist/` to `/usr/local/lib/ide` |

`/usr/local/lib/ide` is the default of `-IDE_APP_DEPLOYMENT_DIR`, so the image
sets no environment variable for it — the same arrangement `/usr/local/lib/web`
has with `-WEB_APP_DEPLOYMENT_DIR`. The variable still overrides it at runtime.

Two things this depends on, both easy to undo by accident:

**`package-lock.json` is committed**, against a repo-wide `.gitignore` rule that
excludes lock files everywhere else — there is an explicit negation for this one.
`npm ci` reads the lock and fails without it, so removing it from git breaks the
image build on any machine that builds from a fresh clone, while continuing to
work on a developer's, where the file is present but untracked.

**The bundle is not relocatable.** `/ide/` is baked into every asset url at build
time by vite's `base`, so serving it from another prefix is a rebuild, not a move.

## Serving

`jets/apiserver/static_ide.go` serves the bundle at `/` with **one** `PathPrefix`
handler. That is not tidiness: vite emits content-hashed file names, so the set of
assets changes on every build and cannot be enumerated in Go. The hashing is also
what makes the cache policy safe — `/assets/*` is immutable for a year,
`index.html` is `no-cache`.

Anything that is not a file falls back to `index.html`, so client-side routes
survive a reload. A *missing* asset 404s instead, because handing back the html
shell surfaces in the browser as "unexpected token '<'", which points nowhere near
the real cause.

**The registration is the last thing `Run` does, and that is load-bearing.**
gorilla/mux matches in registration order and this prefix matches everything, so
an api route registered *after* it would be shadowed — `/login` answered with the
html shell. `TestTheStaticRegistrationIsLastInServerGo` reads `server.go` and
asserts the order rather than trusting it, because the property is the position of
one line in a long function.

**The catch-all became necessary at X.1 and was not needed before.** `jetsclient`
never called `setPathUrlStrategy`, so Flutter web kept hash routing and every one
of its routes lived after the `#` — never sent to the server, which is why 64
enumerated asset routes and no fallback were enough for years. React uses real
paths, so a reload on any in-app route is a GET the server must answer.

## The JetRules mode

`src/editor/jetrules.ts` is a `StreamLanguage` ported from
`tools/vscode-jetrule/syntaxes/jetrule.tmLanguage.json`, which stays the reference
for the language. That grammar is small — fourteen top-level patterns — so porting
was cheaper than pulling in the TextMate runtime (`vscode-textmate` plus an
Oniguruma wasm build) purely to reuse it. **Keep the two in step:** something the
VS Code extension highlights and this does not is a bug in the mode.

A stream tokeniser, not a Lezer grammar, because highlighting is all this needs
today. Folding by rule, structural selection and go-to-definition all want a real
tree; that is the point to invest in Lezer, and this cannot give them one.

`.jr.sql` is checked before `.jr` and reads as SQL — the Go visitor serves both
from the same directories, and the wrong order sends every `.jr.sql` down the
JetRules branch.

## ~~Getting here from the Flutter app~~

There is no Flutter app. It carried a **Code Editor ↗** entry that opened `/ide/`
in a new tab, and a handoff that sent a user to `/ide/flow/<key>` for a migrated
flow; X.1 deleted both with the app they were in. The section is kept as a heading
because the arrangement is cited from the project documents and "it used to work
like this" is more useful to a reader than a missing section.

**The session was not shared between the two**, which meant signing in twice. That
wrinkle is gone by subtraction rather than by being fixed.

## What this does not do yet

**The beachhead paragraph that was here is retired.** It listed what remained in
the Flutter app — file creation, git operations, the workspace-registry screens,
the Query Tool, the section forms — and every one of those is now here: track C
ported fifteen screens, track F the eleven user flows, and X.4 registration.

What is still true is narrower and worth keeping: **the flows have not been driven
by a real user against a live apiserver.** They run end to end under jsdom against
a stubbed `fetch`, with everything below `ApiClient` real, and they load from an
installed workspace — but a browser, a database and a person are a different test
and that one has not been run.
