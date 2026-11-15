---
title: Screens
description: One page for each of JetStore's screens and user flows, and what every screen has in common, from the top bar to the way a table sorts, pages and copies.
kind: overview
sidebar:
  label: Overview
  order: 0
---

These pages describe JetStore's screens one at a time: what each one is for, every control, column
and tab on it, the capability it needs, how you reach it and where it leads. They are reference
pages, to look things up in; the how-to guides say how to get a task done across several screens.
*Help*, in the top bar, opens the page of the screen you are on.

What every screen has in common is on this page, so the screen pages do not repeat it.

## The top bar

Every screen shows the same bar across the top.

| Item | What it does |
|---|---|
| **JetStore Workspace ▾** | Opens the menu of configuration flows: *Clients & Vendors*, *Source Configuration*, *Source Mapping*, *Rules Configuration*, *Pipeline Configuration* and *User Administration*. |
| **Home** | The [Home](/docs/user/screens/home/) screen: runs, registered data, sessions and archived runs. |
| **Filter Client** | Narrows every table that has a *Client* column to one client, on every screen, until you choose *Filter Client* again. It lists the clients of the installation. |
| **Workspaces**, **Workspace IDE**, **Proposals**, **Incidents**, **Detection**, **Runbooks**, **Query Tool**, **Infer Server Admin** | The other screens. An entry you lack the capability for is shown disabled. |
| your name | Your git profile. |
| **Dark** or **Light** | Switches the colour theme. |
| **Help** | Opens this guide at the page of the screen you are on. |
| **Sign out** | Ends your session. |

Your session lives in the browser tab. **Reach screens through the app's own links and buttons**:
reloading the page, or typing an address, signs you out. Your browser's *Back* button works.

## Tables

Most screens are built around one or more tables, and every table works the same way.

- **Selecting rows.** A table you can act on has a check box on each row. Some take one row at a
  time, and ticking another clears the first; others take several. A selected row is highlighted.
  A table that takes several has a check box in its heading as well, which selects every row of
  the page, or clears them all; it is half ticked when only some are selected. A selection is kept
  when you change page, so selecting all on each page in turn selects the whole list.
- **Buttons.** A table's buttons sit above it, in up to three rows. A button that needs a selected
  row is disabled until you select one. A button that applies only to some rows, *Get Run
  Manifest* to a completed run for example, stays disabled for the others. **Hover over a disabled
  button to see why**: no row selected, the wrong kind of row, or a capability your account lacks.
- **Sorting.** Each table opens sorted by one column, marked ▲ or ▼. Press a column heading to sort
  by it, and press it again to reverse the order.
- **Copying a value.** Click a cell to copy its whole value to the clipboard, even when the cell
  shows it shortened: a file key shown as `.../eligibility.csv`, for example.
- **Pages.** A table with more rows than fit on one page has a footer: *Rows per page*, the range
  shown, such as *1–10 of 22*, and buttons for the first, previous, next and last pages. A table
  that fits on one page has no footer.
- **Refresh** reads the table again. Tables do not refresh by themselves, so press it to see a run
  move on.
- **An empty value is shown as an empty cell.**
- **A wide table scrolls sideways** inside the window.

Hover over a column heading to see what the column holds.

## Dialogs and messages

A dialog opens over the screen and keeps it until you close it with its own button: *Close*,
*Cancel* or *Save*. A field that fails its check says why beneath it, and the dialog stays open
until you correct it. Messages from JetStore, such as *Pending pipelines checked*, or an error from
the server, appear in a banner at the top of the screen.

## User flows

A user flow is a screen that takes you through a task in steps, one form or table per step. See
[User flows](/docs/user/screens/flows/) for how they work and a page for each.

## The screens in this section

| Screen | What it shows | Reached from |
|---|---|---|
| [Home](/docs/user/screens/home/) | runs of pipelines, the data registry, the session registry and archived runs | the top bar; it is the screen you land on |
| [Pipeline Execution Details](/docs/user/screens/pipeline-execution-details/) | each step and shard of one run | Home, *View Execution Details* |
| [CPIPES Execution Details](/docs/user/screens/cpipes-execution-details/) | each step of one run, totalled over its nodes | Home, *View Execution Stats* |
| [Process Errors](/docs/user/screens/process-errors/) | the errors a run's rules raised, with the records behind each one | Home, *View Process Errors* |
| [File Loader Status](/docs/user/screens/file-loader-status/) | every file loaded into a staging table | the *Source Mapping* flow, *Loader Status* |
| [Staging Table or Domain Table View](/docs/user/screens/table-view/) | the rows one session wrote to one table | Home's *Data Registry*, and *File Loader Status* |
| [Input File Preview](/docs/user/screens/input-file-preview/) | the first lines of a file, as it arrived | the *Load Files* flow |
| [Query Tool](/docs/user/screens/query-tool/) | the result of an SQL statement you write | the top bar |
| [Sign in and register](/docs/user/screens/sign-in/) | signing in, and registering an account | the address of the installation |
| [Workspaces](/docs/user/screens/workspaces/) | the workspaces of the installation, and compiling, committing, pulling and loading configuration | the top bar |
| [Workspace IDE](/docs/user/screens/workspace-ide/) | a workspace's files, edited and compiled, and its overrides | the top bar, or *Workspaces* |
| [Rules Configuration](/docs/user/screens/rules-configuration/) | the objects the rules read at the start of each session | the *JetStore Workspace* menu |
| [Git Profile](/docs/user/screens/git-profile/) | the name, email and token JetStore commits with | your name in the top bar |
| [User Administration](/docs/user/screens/user-administration/) | accounts, their roles, and activating them | the *JetStore Workspace* menu |
| [Runbooks](/docs/user/screens/runbooks/) | a workspace's runbooks, their runs and one run's steps | the top bar, or *Workspaces* |
| [Proposals](/docs/user/screens/proposals/) | the changes AI agents propose, and their decisions | the top bar |
| [Incidents](/docs/user/screens/incidents/) | incidents raised over runs, and their hypotheses and verdicts | the top bar |
| [Detection](/docs/user/screens/detection/) | whether the detection and triage tasks ran | the top bar |
| [Infer Server Admin](/docs/user/screens/infer-server-admin/) | the GPU inference server and its models | the top bar |
