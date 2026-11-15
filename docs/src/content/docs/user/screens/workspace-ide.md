---
title: Workspace IDE
description: The Workspace IDE, where a workspace's files are read and edited, its overrides listed and reverted, and its compiled data model, rules and lookups browsed.
kind: reference
---

The *Workspace IDE* is the editor of a workspace: its files in a tree, each opened in a tab, the
changes saved on this installation listed and revertible, and what the compiler made of the data
model, the rules and the lookups. How a saved file becomes an override, and why it takes a compile
and a commit to make it last, is in [Workspaces](/docs/user/concepts/workspaces/).

**Reach it** from *Workspace IDE* in the top bar, which opens the active workspace, or with **Open**
on the [Workspaces](/docs/user/screens/workspaces/) screen, which opens the selected one at
`/workspaces/<name>/home`. A runbook run's *Edit future steps* link opens it too. It needs the
`workspace_ide` capability: the top bar entry is disabled without it, and a user who reaches the
screen anyway sees *Read only* and cannot save.

![The Workspace IDE on jets_ws: the workspace picker, and Workspace guide and Save, at the top, Refresh below, the file tree on the left with Data Model, Jets Rules, Lookups and Pipes Config, and the empty Workspace Changes table on the right.](../../../../assets/screenshots/user/screens/workspace-ide/ide-changes.png)

## The bars at the top

| Control | What it does |
|---|---|
| **Workspace picker** | The workspace being edited, with its branch. Choosing another opens it. |
| **Save** | Saves the file in the current tab. Enabled when the tab has unsaved changes and the file is not read-only. `Ctrl+S` does the same in the editor. |
| **Refresh** | Reads the file tree and the changes table again. |
| **Workspace guide** | Beside *Save*: opens this installation's guide to the workspace at the page for the open file (below). Shown only when the installation has a guide to the workspace open in the IDE, and to every user, since the guide needs no capability. |

## The workspace guide

An installation's documentation has a fourth guide, named for its workspace: its domain, lookups and
rules, generated from the compiled workspace, and the pages the workspace's own people wrote.
**Workspace guide** opens it in the documentation's tab, the one *Help* uses, at the page for the file
open in the editor:

| File open in the editor | Page opened |
|---|---|
| A data-model file | its page of the domain: its classes, their properties and the rules written in it, such as the compiler's generated rules |
| A lookup's source file | the lookups page |
| A main rule file | its rules page: the files it includes, the rules written in it with their descriptions, and its unit tests |
| A rule file that main rule files include | its own rules page: the main rule files that include it, and the rules written in it with their descriptions |
| A page of the workspace's `docs/` folder | that page |
| Any other file, or none | the guide's landing page |

The button is not shown when the installation's documentation has no guide to the workspace open in
the IDE: an installation built without one, or another workspace opened here. The guide describes the
workspace as it was compiled into the deployed image, so an edit saved here, a page of `docs/`
included, is not in it until it is committed to the workspace's repository and the images are
rebuilt. The top bar's *Help* still opens this page. How a workspace writes its guide is in
[Document a workspace](/docs/developer/how-to/document-a-workspace/).

![The page of jets_ws's guide that Workspace guide opens for data_model/hc_model.jr: the file's domain classes, each with its base classes and a table of its properties, their kind and type, and the rules written in the file listed on the right.](../../../../assets/screenshots/user/screens/workspace-ide/workspace-guide-page.png)

## The file tree

The left panel lists the workspace's files by section: *Data Model*, *Jets Rules*, *Lookups*,
*Pipes Config*, *User Flows*, *Table Configurations*, *Process Configuration*, *Reports*, *Runbooks*,
*Documentation* and the others the workspace has, each file
with its size. **Filter files** narrows the tree to the names that match.

- **A file** opens in a tab of the editor.
- **The caret of a section** expands or collapses it.
- **The name of *Data Model*, *Jets Rules* or *Lookups*** opens that section's compiled view (below),
  rather than expanding it.

**Documentation** lists the workspace's `docs/` folder: the pages its people write for the
[workspace guide](#the-workspace-guide), Markdown files (`.md` or `.mdx`) in `docs/` and its
subfolders, and the guide's settings file, `docs/workspace_docs.json`. A page opens, is edited and is
saved like any other file, and **Workspace guide** opens that page in the guide. A saved page needs no
compile: nothing compiles it, and the guide shows it once it is committed and the images are rebuilt.
The section has no **Add File**: a new page is added in the workspace's repository. How to write a page is in
[Document a workspace](/docs/developer/how-to/document-a-workspace/).

![The Documentation section of jets_ws's file tree, with its customisation folder, index.md and workspace_docs.json, and index.md open in the editor, its front matter highlighted as YAML.](../../../../assets/screenshots/user/screens/workspace-ide/ide-documentation.png)

## Editing a file

![hc_model.jr open in a tab of the editor, highlighted as JetRules, with the status bar at the bottom.](../../../../assets/screenshots/user/screens/workspace-ide/ide-file.png)

Every file opens in its own tab, whatever its size. The editor highlights JetRules, JSON, SQL and
Markdown; other files, such as a lookup's CSV, show as plain text. A tab with unsaved changes shows a dot, and its status bar says
*Modified*:

![The status bar of an edited file: its path, its language, its size in characters, and Modified.](../../../../assets/screenshots/user/screens/workspace-ide/ide-status-modified.png)

The status bar shows the file's path, its language, *Read only* when it cannot be edited, its size and
whether it is saved. Files JetStore installs into every workspace are read-only: a workspace changes
what one of them does by extending it, as
[Workspace assets JetStore installs](/docs/developer/workspace/assets/) explains. A Markdown file,
such as a `README.md` beside the pipelines, is read-only too, except a page under `docs/` and the
workspace's own `README.md`, at the top of the tree. Saving that README makes no start of JetStore
compile the workspace.

**Saving checks the file first.** A JSON file that does not parse is refused before it is sent. The
server then checks the file types it knows, such as user flows, table documents, SQL scripts,
pipeline definitions and the guide's `workspace_docs.json`, and refuses a save that would break them,
saying why. A pipeline definition (`.pc.json`) is refused only for what would stop the pipeline when
it starts, whatever the run: a value the definition's format cannot hold, no `cluster_config`, an
output table column whose `rdf_type` is not in the
[closed list of column types](/docs/developer/compute-pipes/output-tables/#column-types), or a check
the start makes of the whole definition before any step runs (an empty conditional step, a
`column_encodings` entry with no column, a text template that does not compile or whose key is used
twice, a writer setting on an input channel, `latest_session_only` on a channel that does not read a
staged `file_key`, or `use_jet_rules_go: true`). A page of `docs/`
is refused when the documentation build would refuse it for its front matter: none, YAML that does not
parse, or no `title`, `description` or `kind` (one of `tutorial`, `how-to`, `reference`,
`explanation` or `overview`). The editor shows a page's front matter, its opening `---` block, as
YAML. A file in the place of the workspace guide's generated pages, under `docs/reference/` or named
`docs/reference.md` or `docs/reference.mdx`, in any case, is refused whatever it holds, with a message
saying that place is generated. A page's links are checked when the images are built. A
`reports/config.json` is refused when a reports file it runs could not run: a statement with no name
comment above it, a date pattern `{D:` that is not closed, or a report whose name gives no output
format and whose `statementProperties` declare none. A reports file under `reports/` is refused for the
same reasons when `reports/config.json` runs it. A SQL file under `runbooks/` is refused when it holds
a statement no runbook step may run, such as `BEGIN`, `COMMIT`, `SET ROLE` or `COPY`.

**A save can succeed with warnings.** A check may find something that does not stop the file working
but that you should know about: in a pipeline definition, every key the pipeline does not know and so
ignores (a misspelt setting is one that silently does not apply), and an output table column with no
`rdf_type`, which creating the table needs; in a user flow's form, a table it draws that the
workspace's `table_configs/` does not hold. The file is saved, and the banner says so in the warning
colours, listing each warning at its place in the file (a JSON Pointer such as
`/pipes_config/0/apply/1/device_writer_type`, or a line) until you dismiss it.

A saved file becomes an **override**: it is held in
the database, takes precedence over the file in the image, and appears in *Workspace Changes*.

Closing a tab with unsaved changes asks first; **OK** discards them.

![The confirmation asked when an edited tab is closed: hc_model.jr has unsaved changes, close it anyway?](../../../../assets/screenshots/user/screens/workspace-ide/ide-close-unsaved.png)

## Workspace Changes

With no tab open, the right panel shows the workspace's overrides: every file saved on this
installation and not yet committed.

| Column | What it shows |
|---|---|
| **Workspace Name** | The workspace. |
| **Change OID** | The identifier of the stored content. |
| **File Name** | The file's path in the workspace. |
| **Content Type** | The kind of file. |
| **Change Status** | The state of the change. |
| **User Email** | Who saved it. |
| **Last Update** | When it was saved. |

**Delete/Revert Changes** reverts the selected overrides, and **Delete/Revert ALL Changes** reverts
every one, each after a confirmation: the file in the image applies again. A file added since the
installation was deployed is deleted, and the confirmation says so. **Delete/Revert ALL Changes keeps
the compiled files** (`workspace.db`, `lookup.db`, `workspace.tgz`, `reports.tgz`) from the last compile,
so compile again after reverting. A change that has been
committed and pushed from the [Workspaces](/docs/user/screens/workspaces/) screen leaves this table.

## Compiled views

![The Data Model compiled view: tabs for Domain Classes, Data Properties, Domain Tables and Data Model Files, the first listing jets_ws's classes, whether each is persisted as a table, and its source file.](../../../../assets/screenshots/user/screens/workspace-ide/ide-compiled-data-model.png)

A compiled view shows what the last compile made of a section, read from `workspace.db`, in tabs:

| View | Tabs |
|---|---|
| **Data Model** | *Domain Classes*, *Data Properties*, *Domain Tables*, *Data Model Files* |
| **Jets Rules** | *Jet Rules*, *Rule Terms*, *Files Relationship*, *Jet Rules Files* |
| **Lookups** | *Lookup Tables*, *Lookup Columns* |

*Jet Rules* lists each rule with its salience and its text as written, the *Main Rule File* it is
compiled under, and its *File*, the file it is written in: the main rule file itself or one it
includes. A rule in a file that two main rule files include is listed once under each.
*Files Relationship* lists, for each main rule file, every file it includes, directly or through
another included file. *Jet Rules Files* lists the included rule files as well as the main ones, with
*Main Rule File?* telling them apart.

![The Jet Rules tab of the Jets Rules compiled view: jets_ws's rules by name, each with its salience, its text as written, the main rule file it is compiled under, jet_rules/clinical_intel/cintel_main.jr, and the file it is written in, an included rule file.](../../../../assets/screenshots/user/screens/workspace-ide/ide-compiled-jet-rules.png)

![The Files Relationship tab of the Jets Rules compiled view: each of jets_ws's main rule files with every file it includes, cintel_main.jr with its data model, rule and lookup files, and each unit test's rule file with jets_model.jr.](../../../../assets/screenshots/user/screens/workspace-ide/ide-compiled-files-relationship.png)

The *Data Model Files* and *Jet Rules Files* tabs also have **Add File**, which creates an empty file
in the section, and **Delete**, which deletes the selected files. A compiled view reflects the last
compile, so it does not show an edit until the workspace is compiled again.

## See also

- [Work on a workspace](/docs/user/how-to/work-on-a-workspace/)
- [Your first rule change](/docs/user/tutorials/first-rule-change/)
