---
title: Develop and test the web app
description: Build the web application, run its test suite and type check, and run it against a local apiserver, either served by the apiserver or from the Vite dev server.
kind: how-to
sidebar:
  order: 0
---

The commands run from `jetsclient_ide/`.

## Build and test

```bash
npm ci
npx vitest run      # the whole suite, once; `npm test` is the same command
npm run build       # tsc --noEmit, vite build -> dist/, then the validator -> dist-validator/
npm run typecheck
```

`dist/` and `dist-validator/` are git-ignored. `npm run build` is the real check: it type-checks first
and fails on a type error the suite does not exercise. `dist-validator/jets_validate_workspace.mjs` is
the workspace validator the compile runs (`vite.validator.config.ts` says why it is a second build).

## Run it against a local apiserver

Two ways. Both talk to the same endpoints on the same origin, so the client code does not know which
one it is in.

### Served by the apiserver, as production does

Point `IDE_APP_DEPLOYMENT_DIR` at the built bundle. From the repository root:

```bash
(cd jetsclient_ide && npm run build)     # the app bundle

go build -o /tmp/apiserver ./jets/apiserver

WORKSPACES_HOME=<the folder holding your workspaces> \
WORKSPACE=jets_ws \
IDE_APP_DEPLOYMENT_DIR=$PWD/jetsclient_ide/dist \
JETS_DSN='postgresql://...' \
API_SECRET='...' \
/tmp/apiserver -usingSshTunnel
```

Then open `http://localhost:8080/`. `-usingSshTunnel` is what puts the server on `:8080` without TLS;
without it the server listens on `:8443`.

The flag is `-IDE_APP_DEPLOYMENT_DIR`, and the environment variable of the same name overrides it. If
it points nowhere, every URL answers 404 with a plain message rather than failing obscurely, so an
apiserver built without the bundle still starts, and `TestRootIs404WithoutABundle` in
`jets/apiserver` keeps that a deployment error rather than a routing mystery. (The Flutter client's
`WEB_APP_DEPLOYMENT_DIR` flag is gone with it.)

### From the Vite dev server, with hot reload

```bash
npm run dev                                          # apiserver on :8080
JETS_API_ORIGIN=https://localhost:8443 npm run dev   # apiserver on :8443
```

It serves on `http://localhost:5173/` and proxies `/dataTable`, `/login` and the other endpoints to
the apiserver, so there is no CORS involved and the app still sees a same-origin path. The proxy sets
`secure: false` for the `:8443` case, whose certificate is self-signed.
