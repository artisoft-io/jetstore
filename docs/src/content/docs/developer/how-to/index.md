---
title: How-to guides
description: Tasks that extend or build JetStore itself, each written from a run on a workstation checkout.
kind: overview
sidebar:
  label: Overview
  order: 0
---

Each guide here was carried out on a checkout of the `jetstore` repository before it was written, and
says what was run and what it gave. They assume a Linux workstation with the toolchain that
[Build JetStore from source](/docs/developer/how-to/build-from-source/) lists.

- [Build JetStore from source](/docs/developer/how-to/build-from-source/): the C++ rule engine
  (`libjets.so`), the Go modules, the web app and this documentation site.
- [Write a Lambda for a workspace](/docs/developer/how-to/write-a-workspace-lambda/): a workspace's own
  handler in place of one of JetStore's stock Lambdas, where its code lives, what it builds against and
  how the stack picks it up.
- [Write a compute-pipes operator](/docs/developer/how-to/write-a-compute-pipes-operator/): a new
  `type` of transformation step, from its configuration struct to the contract matrix and the tests.
- [Add a system table](/docs/developer/how-to/add-a-system-table/): a table of the `jetsapi` schema,
  the migration that creates it, the grants the restricted logins get on it and its generated
  reference.
- [Add a screen or a user flow](/docs/developer/how-to/add-a-screen-or-user-flow/): a screen of the
  web app with its route, menu entry and help page, and a user flow's three documents and their
  validation.
- [Add a workspace file type](/docs/developer/how-to/add-a-workspace-file-type/): a row of the save
  path's validator table, its test, and the IDE section that shows the file.
- [Document a workspace](/docs/developer/how-to/document-a-workspace/): a workspace's own guide in its
  installation's documentation, from its written pages, its rules' descriptions and its compiled
  `workspace.db`, and the build that checks it before a deployment.

Tasks specific to the compute-pipes engine are in its own
[how-to guides](/docs/developer/compute-pipes/how-to/).
