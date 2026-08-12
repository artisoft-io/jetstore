# jetsclient_ide

The JetStore Workspace IDE: a React + TypeScript single-page app built around
[CodeMirror 6](https://codemirror.net/), served by the Go apiserver at `/ide/`.

It exists because the Flutter client's file editor is a single `TextFormField`.
Flutter lays a text field's whole document out on every frame rather than
virtualising by line, so cost grows with the size of the file instead of with the
part of it on screen — which is why `mapMenuEntry` refuses to open anything at or
above 250,000 bytes, and why three real `.jr` files in the workspace corpus, the
largest 1.18 MB, cannot be opened in the IDE at all. CodeMirror virtualises by
viewport, so **this app has no size limit**; `src/api/workspace.test.ts` pins that
with a 1.2 MB load.

This is the first stage of the port described in the UI assessment. It shares the
Go apiserver, the `/dataTable` endpoints, the JWT session and the `workspace_ide`
capability with the Flutter app, and runs alongside it rather than replacing it.

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
npm run build                     # the IDE bundle
cd ../jetsclient && flutter build web --release && cd ..   # the Flutter bundle

go build -o /tmp/apiserver ./jets/apiserver

WORKSPACES_HOME=/home/michel/projects/repos/workspaces \
WORKSPACE=jets_ws \
WEB_APP_DEPLOYMENT_DIR=$PWD/jetsclient/build/web \
IDE_APP_DEPLOYMENT_DIR=$PWD/jetsclient_ide/dist \
JETS_DSN='postgresql://...' \
API_SECRET='...' \
/tmp/apiserver -usingSshTunnel
```

Then open <http://localhost:8080/ide/>. `-usingSshTunnel` is what puts the server
on `:8080` without TLS; without it the server listens on `:8443`.

**Set `WEB_APP_DEPLOYMENT_DIR` even if you only care about the IDE.** It defaults
to `/usr/local/lib/web`, which exists in the container image and on no
development machine, and the `/` route is an `http.FileServer` over it. Leave it
unset and every Flutter url 404s — including `/#/login`, because the fragment
never reaches the server and the browser is really asking for `/` — while `/ide/`
carries on working from its own directory. The asymmetry makes it look like the
IDE broke the Flutter app; it did not, and
`TestIdePrefixDoesNotShadowTheFlutterApp` in `jets/apiserver` exists to keep that
answerable without guessing.

The flag is `-IDE_APP_DEPLOYMENT_DIR`, and the environment variable of the same
name overrides it — the same arrangement `WEB_APP_DEPLOYMENT_DIR` uses for the
Flutter bundle. If it points nowhere, `/ide/` answers 404 with a plain message
rather than failing obscurely, so an apiserver built without the bundle still
starts.

### Vite dev server (hot reload)

```bash
npm run dev                                     # apiserver on :8080
JETS_API_ORIGIN=https://localhost:8443 npm run dev   # apiserver on :8443
```

Serves on <http://localhost:5173/ide/> and proxies `/dataTable`, `/login` and the
other endpoints to the apiserver, so there is no CORS involved and the app still
sees a same-origin path. The proxy sets `secure: false` for the `:8443` case,
whose certificate is self-signed.

## In the container image

The bundle is built into `ui_service_ws` alongside the Flutter one, by the same
three-image chain:

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

`jets/apiserver/static_ide.go` serves the bundle under `/ide/` with **one**
`PathPrefix` handler, not the route-per-asset list the Flutter bundle uses. That
is not tidiness: vite emits content-hashed file names, so the set of assets
changes on every build and cannot be enumerated in Go. The hashing is also what
makes the cache policy safe — `/ide/assets/*` is immutable for a year,
`index.html` is `no-cache`.

Anything under `/ide/` that is not a file falls back to `index.html`, so
client-side routes survive a reload. A *missing* asset 404s instead, because
handing back the html shell surfaces in the browser as "unexpected token '<'",
which points nowhere near the real cause.

The Flutter bundle's ~50 explicit asset routes are deliberately untouched. They
work, and consolidating them means moving the static registration after the API
routes so a catch-all cannot shadow `/login` — worth doing when the Flutter app
is retired, not as a rider on this.

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

## Getting here from the Flutter app

The Workspace IDE menu carries a **Code Editor ↗** entry that opens `/ide/` in a
new tab, gated on `workspace_ide` like the other entries. It has no `routePath`,
so it never joins the Flutter app's page stack, and it opens in a tab rather than
an iframe because this app owns its own routing and editor keybindings — both of
which an embedded frame would fight.

**The session is not shared.** This app holds its token in memory, so opening it
means signing in once more. That is a real wrinkle and the obvious thing to fix
next; doing it properly means agreeing where a token may be persisted, which is a
security decision rather than a coding one.

## What this does not do yet

The beachhead is deliberately narrow. Not here: creating, renaming or deleting
files; git operations; the workspace-registry screens; the Query Tool; diffing
against the committed version; the pipeline-config and data-model section forms.
All of those remain in the Flutter app, which is why the two run side by side.

It has also not yet been run against a live apiserver and database. The wire
contract above is verified against the Go source and by unit tests that assert
the exact request shapes, but not by a real session.
