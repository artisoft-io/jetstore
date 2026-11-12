---
title: "A React web app in place of the Flutter client"
description: "Why JetStore's web application is a React and TypeScript app, how it replaced the Flutter client, and what the move changed in the apiserver."
kind: explanation
---

*Decided 2026-08-11; the Flutter client retired 2026-08-26.*

## Context

JetStore's web client was a Flutter web application, `jetsclient/`. Its workspace file editor was a
single text field, and Flutter lays out a text field's whole document on every frame rather than
only the visible lines, so the cost grew with the file's size. The client therefore refused to open
any file above a size limit (`workspaceFileEditorSizeLimit`), and the largest rule files, over a
megabyte, could not be opened in the editor at all.

## Decision

Build a React and TypeScript application, `jetsclient_ide`, on CodeMirror 6, which lays out only the
lines in view. It was first served beside the Flutter client at `/ide/`, sharing its endpoints, its
token and its capabilities, and it grew screen by screen. On 2026-08-26 the Flutter client was
deleted and the React app moved to the root, `/`.

## Consequences

- The editor has no size limit; a test loads a 1.2 MB file to keep it that way.
- The apiserver serves the bundle with one catch-all handler, and needs a fallback to `index.html`
  for client-side routes: Flutter kept its routes after the `#`, React uses real paths. The
  catch-all is registered last so that it cannot shadow an API route, and
  `TestTheStaticRegistrationIsLastInServerGo` checks the order.
- The Flutter toolchain, about a gigabyte, left the images.
- Comments in `jetsclient_ide` that cite `jetsclient/lib/...` are provenance for what was ported, read
  with `git show b4cda702:<path>`.
- The JetRules highlighting is a port of the VS Code extension's grammar, which stays the reference.

## Where the reason is recorded

- Commit `1154bc6d`, *Add the CodeMirror Workspace IDE, served at /ide/*, which gives the editor's
  reason. The retirement commit, `e4672bfa`, *retire jetsclient, and serve the app at the root*,
  records what serving the root required; it records no further reason for the retirement itself.
- [How the web app is served and built](/docs/developer/testing/web-app/serving/), for the serving
  consequences.
