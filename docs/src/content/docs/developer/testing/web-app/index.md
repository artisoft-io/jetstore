---
title: The web application
description: What jetsclient_ide is, JetStore's React and TypeScript web application, why its editor is CodeMirror, and where to find how it is developed, tested and served.
kind: overview
sidebar:
  label: Overview
---

`jetsclient_ide` is the JetStore web application: a React + TypeScript single-page app built around
[CodeMirror 6](https://codemirror.net/), served by the Go apiserver at the root.

**The directory is named for what this was and not for what it is.** It began as the Workspace IDE,
one screen mounted at `/ide/`, running beside the Flutter client. The Flutter client was retired on
2026-08-26 and this app moved to `/`, so it is now the whole product: the screens, the user flows and
the editor. Renaming the directory would touch the Dockerfiles, the Go handler, the module paths and
every citation in its comments, so it has not been done.

**The editor is why it exists.** The Flutter client's was a single `TextFormField`, and Flutter lays a
text field's whole document out on every frame rather than virtualising by line, so cost grew with the
size of the file instead of with the part of it on screen. That is why it refused to open anything at
or above 250,000 bytes, and why some real `.jr` files, the largest 1.18 MB, could not be opened at
all. CodeMirror virtualises by viewport, so **this app has no size limit**; `src/api/workspace.test.ts`
pins that with a 1.2 MB load.

**What has not been tested is narrower than it was.** The user flows run end to end under jsdom
against a stubbed `fetch`, with everything below `ApiClient` real, and they load from an installed
workspace; a browser, a database and a person driving them against a live apiserver is a different
test.

## In this section

- [Develop and test the web app](/docs/developer/testing/web-app/develop-and-test/): build it, run
  the tests, and run it against a local apiserver.
- [Web app reference](/docs/developer/testing/web-app/reference/): the source layout, and the server
  contract the client depends on.
- [How the web app is served and built](/docs/developer/testing/web-app/serving/): the static
  handler, the container image, the JetRules editor mode, and the citations into the retired Flutter
  client.
- [Validating action documents](/docs/developer/testing/web-app/action-documents/): what the checks on
  a user flow's `.ua.json` guarantee, and what they do not.
