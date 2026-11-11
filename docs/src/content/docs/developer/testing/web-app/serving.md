---
title: How the web app is served and built
description: How the apiserver serves the web application's bundle, how the bundle is built into the UI image, why the JetRules editor mode is a port, and how to read the comments that cite the retired Flutter client.
kind: explanation
sidebar:
  order: 2
---

## Serving

`jets/apiserver/static_ide.go` serves the bundle at `/` with **one** `PathPrefix` handler. That is not
tidiness: vite emits content-hashed file names, so the set of assets changes on every build and cannot
be enumerated in Go. The hashing is also what makes the cache policy safe: `/assets/*` is immutable
for a year, `index.html` is `no-cache`.

Anything that is not a file falls back to `index.html`, so client-side routes survive a reload. A
*missing* asset answers 404 instead, because handing back the HTML shell surfaces in the browser as
"unexpected token '<'", which points nowhere near the real cause.

**The registration is the last thing `Run` does, and that is load-bearing.** gorilla/mux matches in
registration order and this prefix matches everything, so an API route registered *after* it would be
shadowed: `/login` answered with the HTML shell. `TestTheStaticRegistrationIsLastInServerGo` reads
`server.go` and asserts the order rather than trusting it, because the property is the position of one
line in a long function.

**The catch-all became necessary when the Flutter client was retired.** The Flutter client never
called `setPathUrlStrategy`, so Flutter web kept hash routing and every one of its routes lived after
the `#`, never sent to the server; that is why enumerated asset routes and no fallback were enough for
years. React uses real paths, so a reload on any in-app route is a GET the server must answer.

**The bundle is not relocatable.** `/` is baked into every asset URL at build time by vite's `base`,
so serving it from another prefix is a rebuild, not a move. It was `/ide/` until the app moved to the
root; `appAssetPrefix` in `jets/apiserver/static_ide.go` and `BASENAME` in `src/base.ts` (empty,
react-router's root) must agree with it.

## In the container image

The bundle is built into `ui_service_ws` by the three-image chain:

| Dockerfile | What it contributes |
|---|---|
| `Dockerfile.cpipes_base_builder` | Node.js 24.19.0, pinned to a release tarball under `/usr/local/node`, next to Go |
| `Dockerfile.cpipes_builder` | copies `jetsclient_ide/` into the build image |
| `Dockerfile.ui_service_ws` | `npm ci && npm run build`, then copies `dist/` to `/usr/local/lib/ide` |

`/usr/local/lib/ide` is the default of `-IDE_APP_DEPLOYMENT_DIR`, so the image sets no environment
variable for it. The variable still overrides it at runtime.

**`package-lock.json` is committed**, against a repository-wide `.gitignore` rule that excludes lock
files everywhere else; there is an explicit negation for this one. `npm ci` reads the lock and fails
without it, so removing it from git breaks the image build on any machine that builds from a fresh
clone, while continuing to work on a developer's, where the file is present but untracked.

## The JetRules mode

`src/editor/jetrules.ts` is a `StreamLanguage` ported from
`tools/vscode-jetrule/syntaxes/jetrule.tmLanguage.json`, which stays the reference for the language.
That grammar is small, fourteen top-level patterns, so porting was cheaper than pulling in the
TextMate runtime (`vscode-textmate` plus an Oniguruma wasm build) purely to reuse it. **Keep the two in
step:** something the VS Code extension highlights and this does not is a bug in the mode.

A stream tokeniser, not a Lezer grammar, because highlighting is all this needs today. Folding by
rule, structural selection and go-to-definition all want a real tree; that is the point to invest in
Lezer, and this cannot give them one.

There is no `.jr.sql` file type: it was an abandoned experiment, deleted from the Go visitor and from
`language.ts` on 2026-09-12. The reasoning is recorded once, on the `jet_rules` row in
`jets/datatable/wsfile/sections.go`.

## Comments that cite `jetsclient/`

**Many comments in this package cite `jetsclient/lib/...`, and that directory no longer exists.** They
are provenance rather than pointers: this app is a port, and a comment saying which Dart function a
behaviour was taken from is how a reader checks that a reproduction is faithful, including the
reproductions that are deliberately *not* faithful, which are the ones worth arguing about.

They were not rewritten when the directory was deleted, because there is nothing to rewrite them to. To
read one:

```bash
git show b4cda702:jetsclient/lib/utils/constants.dart
```

`b4cda702` is the last commit that had the Flutter client, the parent of its deletion. Any earlier
commit works too; that one is quoted because it is the state the citations were written against.

**One of these citations is load-bearing rather than historical.** `constants.dart` was read at test
time to check that every key an action document names is a declared constant's *value* (the
name-for-value trap). Its 455 values are frozen in `src/actions/fixtures/dart_constant_values.json`,
extracted at that commit, and the fixture explains what the check can no longer see.
