---
title: Document a workspace
description: Give a workspace its own guide in its installation's documentation, with written pages, descriptions on its rules and the choice to show their text, and check that the guide builds before deploying.
kind: how-to
sidebar:
  order: 7
---

An installation's documentation site carries JetStore's three guides and **a fourth, named for its
workspace**, served at `/docs/workspace/`. It is built into the UI image (`Dockerfile.ui_service_ws`)
with the rest of the site, from two things the workspace provides:

- **its compiled `workspace.db`**, from which the domain, the lookups and the rules are generated with
  every image, so they are always those of the deployed workspace;
- **its `docs/` folder**, pages written by the workspace's own people: what the workspace is for, and
  how it customises JetStore.

A workspace with neither a `docs/` folder nor a rule description still gets the generated pages. This
guide covers the three things a workspace adds to them, and the check to run before deploying. The
example throughout is `jets_ws`, whose customisation is two compute-pipes site-operator packages under
`go/cpipes_operators/`.

This was carried out on 2026-11-13 on `jets_ws`, with the generator as it stands on `jets_ai` that
day: its compiled workspace was taken from a `Dockerfile.compile_ws` build, and the site built with a
`docs/` folder laid out as below. It built, with the fourth guide; a broken link in a written page, and
a `reference/` folder in `docs/`, each failed the build, naming the page or folder; and a description
with an escaped quote, compiled into one rule, showed on that rule's page. With `rule_text` on, each
rule showed how the engine reads it beside its text.

## What the guide holds

| Section | Where it comes from | What it holds |
|---|---|---|
| The landing page | the workspace's `docs/index.md`, else generated | what the guide holds, with the workspace's name |
| Written pages | the rest of the workspace's `docs/` | customisation pages, and any explanation the workspace's Knowledge Engineers write |
| `reference/domain/` | generated from `workspace.db` | one page per data-model file: its classes, each with its base classes and its data and object properties; and the domain tables, each with its columns and the class it maps |
| `reference/lookups/` | generated | each lookup table: its key, its columns and their types, its source file and the rules that read it. **Never its rows** |
| `reference/rules/` | generated | one page per main rule file: the rule files it includes, each rule with its description and properties, the unit tests that run it, and the rules that have no description |

**Who reads it: every signed-in user of the installation.** The documentation is opened from the
top bar's *Help* and needs no capability beyond signing in, while the workspace's files are read in the
[Workspace IDE](/docs/user/screens/workspace-ide/), which needs `workspace_ide`. So keep out of the
written pages anything the client would not show every user: lookup or table rows, samples of its
files, data values. The generated pages hold no values unless the workspace asks for the rules' text
(step 3).

## 1. Write the pages

Put them in a `docs/` folder at the root of the workspace repository. **They are written exactly like
JetStore's own pages** ([Writing the documentation](/docs/developer/contributing/documentation/)), with
three differences in where they land:

- **A page's address is its path under `docs/`, below `/docs/workspace/`**:
  `docs/customisation/observe.md` is served at `/docs/workspace/customisation/observe/`.
- **Links between the workspace's pages are absolute and end with a slash**, like JetStore's:
  `[the observe operator](/docs/workspace/customisation/observe/)`. A workspace page may link to
  JetStore's pages the same way, for example to
  [Write a compute-pipes operator](/docs/developer/how-to/write-a-compute-pipes-operator/).
- **The folder must not hold a `reference/` folder.** That is where the generated pages go, and the
  build fails rather than let a written page be replaced by a generated one.

Every page needs the same front matter as JetStore's, with one of the same five kinds; a page without
it fails the build. When `docs/` has no `index.md`, the guide's landing page is generated:

```yaml
---
title: The observe operator
description: demo_observe, a pass-through compute-pipes operator that counts and samples the records of a step.
kind: reference
---
```

A layout that suits most workspaces, as `jets_ws` would lay it out:

```
docs/
  index.md                 overview: what the workspace is for, and what this guide holds
  customisation/
    index.md               overview: the workspace's own code, one line per package
    observe.md             go/cpipes_operators/observe: the demo_observe operator
    siteops.md             go/cpipes_operators/siteops: the registry handed to the compute-pipes node
  workspace_docs.json      optional; see step 3
```

**One customisation page per custom Lambda or site operator**, written from its code:

- cite the code by its path in the workspace and its identifier, never a line number;
- say what it changes from JetStore's stock behaviour: for a Lambda, which of JetStore's Lambdas it
  stands beside or replaces ([Write a Lambda for a workspace](/docs/developer/how-to/write-a-workspace-lambda/)
  lists them); for a site operator, its configuration fields and an example pipeline step;
- name no secret, account number, bucket name or host, even where the code holds one.

The workspace's pages live in its own repository and reach only its own installation, so unlike
JetStore's they may name the client and its vocabulary.

## 2. Describe the rules

A rule's description is a **rule property named `description`**, a string in double quotes, written in
the rule's header beside the others:

```
# Copy the primary and secondary diagnosis codes to the list of diagnosis codes
[HC_PrimaryDiag01, description="Adds the claim's primary diagnosis code to its list of diagnosis codes"]:
  (?dc rdf:type hc:DiagnosisCodes).
  (?dc hc:Primary_Diagnosis_Code ?v)
  ->
  (?dc hc:Diagnosis_Codes ?v)
;
```

The rule's page shows it under the rule's name. **It changes nothing the rule does**: the compiler acts
on the properties it knows (`s`, the salience; `o`; `aggregate_check`) and keeps any other as data only,
in the rule's label and in `workspace.db` (`PostProcessJetruleProperties` and `makeRuleLabel`,
`jets/compilerv2/compiler/jet_rule_listener_validation.go`). Write it on one line; a double quote inside
the text is written `\"`. A comment above the rule, as in the example, stays in the rule file and is
not shown.

**The rules' owners write the descriptions**, the Knowledge Engineer with the rule's business owner, in
the workspace's pull requests, and keep them in step with the rules. Say what the rule is meant to do,
in the business's words, rather than restating its antecedents: the generated reading (step 3) already
says what the rule does. Each rules page lists the rules that have no description, and the reference
overview counts them, so a workspace can see what is left to write.

## 3. Choose whether the rules' text is shown

By default the rule pages show each rule's name, description, source file and properties, and **not
its text**. A workspace that wants its rules read in the guide, for example by business owners
reviewing them, turns the text on in `docs/workspace_docs.json`:

```json
{ "rule_text": true }
```

Each rule then shows its text, and beside it a **reading in English, generated from the compiled
rule**: when each antecedent holds, then each consequent. The reading never guesses: a construct it
does not render is shown as not rendered, with the operator's name, and the rule's reading is marked
partial. The configuration resources of the aggregate operators, of `substring_of` (its `jets:from`
and `jets:length`) and of `replace_char_of` (its `jets:replace_chars` and `jets:replace_with`) are
read from the `triple(...)` declarations compiled with the rule; where those of `substring_of` or
`replace_char_of` are missing, or a rule sets them at run time, the reading names the configuration
resource rather than guess its values (*the substring of the Drug_Name configured by drugNameConfig*).
The description says what the rule is meant to do, the reading what it does; a reader who finds them
disagreeing has found something to raise with the rule's owner.

A `rule_text` that is not `true` or `false`, or a file that is not a JSON object, fails the build.

**It is off by default because of who reads the guide.** A rule's text holds its literals (the codes,
the thresholds, the names the rules test for), and every signed-in user of the installation can read
the guide, while the rule files themselves need `workspace_ide`. Turning it on shows those literals to
every user. The file is the workspace's, reviewed in its pull requests, so the choice travels with the
rules and no build script changes. Without the file, or with `"rule_text": false`, neither the text nor
the reading is shown. Descriptions are shown either way: their authors chose their words for this.

## 4. Check the guide before you deploy

**A page with a broken link, or without its front matter, fails the workspace's UI image build**, as a
JetStore page fails JetStore's. Run the same build on a workstation first. It needs a checkout of
`jetstore` at the release you deploy, with Node 24, and the workspace compiled.

### Get the compiled workspace

The shortest way is the image the deployment builds anyway. On a machine with JetStore's
`cpipes_builder:latest` image (step 1 of [Build and deploy a workspace](/docs/install/how-to/build-and-deploy-a-workspace/#build)),
build `Dockerfile.compile_ws` from the workspace's checkout under a tag of your own, and copy the
compiled workspace out of it:

```bash
docker build -f <jetstore>/dockerfiles/Dockerfile.compile_ws \
  --build-arg WORKSPACE=jets_ws --build-arg JETS_VERSION=$(date +%s) \
  -t jets_ws_docs_check <jets_ws checkout>

id=$(docker create jets_ws_docs_check)
docker cp "$id":/workspaces/jets_ws ./jets_ws_compiled
docker rm "$id"
```

The image compiles the workspace as a deployment does: it installs JetStore's assets into a copy of the
tree and compiles it, writing `workspace.db` at the tree's root. On `jets_ws` the build took four
seconds on top of an existing `cpipes_builder`. Use a tag of your own rather than `compile_ws:latest`,
which the next UI image build would read.

**Copy the whole folder, not `workspace.db` alone.** Beside each main rule file the compiler writes its
compiled model (`cintel_main.jr.json` beside `jet_rules/clinical_intel/cintel_main.jr`), and the
generator reads from it which file each rule is written in and the order of the files a main rule file
includes. With `workspace.db` alone the guide still builds, and says *not recorded* for both, as it did
on the 2026-11-13 run. The UI image's build reads the whole compiled workspace, so this is what an
installation shows.

### Build the site with it

From the `jetstore` checkout:

```bash
cd docs
npm ci
JETS_WORKSPACE_DB=<path>/jets_ws_compiled/workspace.db \
JETS_WORKSPACE_NAME=jets_ws \
JETS_WORKSPACE_DOCS=<jets_ws checkout>/docs \
npm run build
```

| Variable | What it names |
|---|---|
| `JETS_WORKSPACE_DB` | the compiled `workspace.db`, in the compiled workspace's folder. Without it the site is JetStore's three guides alone, which is the public build |
| `JETS_WORKSPACE_NAME` | the workspace, which names the fourth guide in the sidebar and on its landing page. Without it, the name of the folder holding `workspace.db` |
| `JETS_WORKSPACE_DOCS` | the workspace's `docs/` folder. Optional: without it the guide is the generated pages |
| `JETS_WORKSPACE_VERSION`, `JETS_VERSION` | optional: the workspace's version and JetStore's, shown in the footer. In the UI image, `JETS_VERSION` is the one `cpipes_builder` was built with, and `JETS_WORKSPACE_VERSION` the build argument |

The generator's line in the output says what it wrote, for example `the jets_ws guide, 24 pages (32
classes, 6 lookups, 131 rules, 1 described; rule text not shown)`. The build fails, naming the page,
on a broken link or anchor and on a page whose front matter is missing or invalid. When it passes,
`npm run preview` serves the result at `http://localhost:4321/docs/`, with search, and the
workspace's guide at `http://localhost:4321/docs/workspace/`. The generated pages are written into the `jetstore`
checkout's git-ignored folders and are never committed.

## How the guide is opened

The guide is in the sidebar of every page of the installation's documentation, beside JetStore's three,
and the documentation's [landing page](/docs/) links it beside theirs, as *jets_ws guide* for `jets_ws`.
In the [Workspace IDE](/docs/user/screens/workspace-ide/#the-workspace-guide), the **Workspace guide**
button opens the guide's page for the file open in the editor: a data-model file's domain page, a main
rule file's rules page, a lookup's source file's lookups page, or a file under `docs/` its own page. It
reads the map from file to page from `/docs/workspace/manifest.json`, which the generator writes with
the guide and which names files and pages only. The top bar's *Help* still opens JetStore's page for
the screen.

## See also

- [Customising an installation](/docs/install/planning/customise-an-installation/#the-workspaces-documentation):
  where the workspace's documentation sits among what makes an installation its own.
- [Images](/docs/install/reference/images/#dockerfileui_service_ws): what the UI image builds.
- [Writing the documentation](/docs/developer/contributing/documentation/): the conventions the
  workspace's pages share with JetStore's.
