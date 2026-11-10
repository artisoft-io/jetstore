---
title: Writing the documentation
description: Where documentation pages go, the kinds of page, their front matter, links, and how to preview the site.
kind: how-to
---

JetStore's documentation is plain Markdown in the `docs/` folder of the `jetstore` repository, built
into a static site with [Astro Starlight](https://starlight.astro.build/). It is changed by pull
request like the code, and **a change to what a user sees, how JetStore is extended, or how it is
deployed updates `docs/` in the same pull request**. The pull-request template asks which pages a
change updated, or why none needed to.

## Where a page goes

Pages live under `docs/src/content/docs/`, in one folder per guide:

| Folder | Guide | For |
|---|---|---|
| `user/` | User Guide | the people who configure and run JetStore |
| `developer/` | Developer's Guide | engineers who extend or maintain JetStore |
| `install/` | Installation Guide | whoever deploys JetStore into an AWS account |

The sidebar is generated from these folders. Adding a page, or a folder of pages, needs no change to
the site's configuration: put the file where it belongs and it appears. A folder's group in the
sidebar takes the title of the folder's `index.md`, of kind `overview`; a folder without one is labelled
with its own name. Within a folder, pages with a `sidebar.order` in their front matter come first,
lowest first; the rest follow by file name, with the folder's `index.md` ahead of them.

The address of a page is its path under `docs/src/content/docs/`, without the extension:
`developer/contributing/documentation.md` is served at `/docs/developer/contributing/documentation/`.

## The five kinds of page

Each page is one kind, and says so in its front matter. A page that mixes kinds is the usual reason
documentation is hard to use, so split it instead: the reader looking for a field list should not
have to read the rationale first.

| `kind` | Answers | Example |
|---|---|---|
| `tutorial` | *teach me*: learning by doing, start to finish | your first pipeline, from a new file to its output |
| `how-to` | *help me reach a goal*: one task, as steps | load a file and map its columns |
| `reference` | *tell me exactly*: complete and dry description | every field of a pipeline configuration step |
| `explanation` | *tell me why*: background, design and reasons | why a session is the unit of deletion |
| `overview` | *where am I*: a section's landing page | the `index.md` of each guide and folder |

## Front matter

Every page starts with this block. `title`, `description` and `kind` are required; **a page without
them fails the build**.

```yaml
---
title: Load a file and map its columns
description: Register a new file, map its columns to the domain model, and check the result.
kind: how-to
---
```

The `title` is the page's heading; do not repeat it as a `#` heading in the body. Starlight's other
front matter fields work as documented, for example `sidebar: { label: Overview, order: 0 }` on a
folder's `index.md`.

## Links

**Link between pages with absolute paths that start with the site's base, `/docs/`, and end with a
slash**: `[the Installation Guide](/docs/install/)`, or with an anchor,
`[front matter](/docs/developer/contributing/documentation/#front-matter)`. An anchor is the heading
in lower case, with spaces as hyphens and punctuation removed.

The build checks every internal link and anchor, and **fails on a broken one**, and on a relative
link. Renaming or moving a page therefore fails the build until every link to it is updated.

**JetStore's *Help* button links into this site too.** The map from a screen to its page is
`jetsclient_ide/src/shell/helpTopics.ts`, and its test fails when the map names a page that does not
exist. When you rename or move a page that the map names, update the map in the same pull request.

## Generated pages

The pages under `developer/reference/generated/` are written by `npm run gen` from JetStore's own
source files, and are not in git. Do not edit them: change the source, or the generator in
`docs/scripts/`. The [reference overview](/docs/developer/reference/) lists each page and its source.

## Preview and build

From the `docs/` folder, with Node 24:

```sh
npm ci            # once, and after package-lock.json changes
npm run dev       # live preview at http://localhost:4321/docs/
npm run check     # validate every page's front matter, without a full build
npm run build     # the full site in docs/dist/, with the links checked and the search index built
```

`npm run dev` reloads a page as you save it. The search box works only in a full build: run
`npm run build`, then `npm run preview` to browse `docs/dist/` with search.

## What a page must not contain

`jetstore` is a public repository. **No page names a client**: no client's workspace name, its rule
vocabulary, its data values, or anything else that identifies an installation. Examples use the
`jets_ws` workspace, or invented names. Material specific to one installation belongs in that
installation's workspace repository, not here.

**Cite code by file path and identifier, never by line number**:
`jets/apiserver/static_ide.go`, function `appHandler`. A line number goes stale at the next change to
the file, and nothing would notice.
