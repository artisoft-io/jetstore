---
title: Concepts
description: The ideas JetStore is built on, one page each, in the order to read them.
kind: overview
sidebar:
  label: Overview
  order: 0
---

These pages explain what each idea in JetStore is and why it is that way. They are not procedures:
the how-to guides say how to do things, and the screen pages describe each screen. Read them in this
order, because each one uses the ideas of the pages before it.

| Page | What it explains |
|---|---|
| [JetStore in one page](/docs/user/concepts/jetstore/) | what JetStore does with a client's files, who configures what, and where everything lives |
| [Clients, organisations and object types](/docs/user/concepts/clients-and-object-types/) | who the data belongs to, who sends it, and what kind of data it is |
| [Source configurations and domain keys](/docs/user/concepts/source-configurations/) | how JetStore knows how to read a file and map its columns, and how it keeps one patient's records together |
| [Files and the input registry](/docs/user/concepts/files-and-the-input-registry/) | how a file is named, how JetStore registers it, and the ledger every pipeline reads from |
| [Workspaces](/docs/user/concepts/workspaces/) | the repository of rules, data model, pipeline definitions, reports and user flows that makes JetStore do a particular job |
| [Pipelines and sessions](/docs/user/concepts/pipelines-and-sessions/) | process and pipeline configurations, a run and its statuses, and the session that tags everything a run writes |
| [The rule engine](/docs/user/concepts/rule-engine/) | what a rule is, how rules see a patient's records, and what they write |
| [Users, roles and capabilities](/docs/user/concepts/users-and-capabilities/) | how an account gets its rights, and what each capability opens in the screens |

The examples on every page use the `jets_ws` workspace and its client `CI` (*Clinical Intel*).
