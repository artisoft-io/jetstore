---
title: "Decision records"
description: "The design decisions JetStore has made, one short record each: the context, the decision, its consequences, and where its reason is recorded."
kind: overview
sidebar:
  label: Overview
  order: 0
---

A decision record says why JetStore is built one way and not another, so that the next person to
change it knows what the change gives up. Each record is short and has four parts:

- **Context**: what was true, and what forced a choice.
- **Decision**: what was chosen, and when.
- **Consequences**: what follows from it, including what it costs.
- **Where the reason is recorded**: the commit, code comment or page where the reason was written
  when the decision was made.

**A record is written only when its reason was recorded at the time.** A record written later from
memory tends to rationalise the outcome, so a decision whose reason cannot be found in a commit, a
comment or a page has no record here. A new decision gets its record in the pull request that makes
it.

| Record | Decided |
|---|---|
| [One rule engine, the C++ one](/docs/developer/architecture/decisions/one-rule-engine/) | 2026-10-22 |
| [The compiled workspace is versioned and carried in the images](/docs/developer/architecture/decisions/compiled-workspace-in-the-image/) | 2026-01-20, completed 2026-08-22 |
| [A React web app in place of the Flutter client](/docs/developer/architecture/decisions/react-web-app/) | 2026-08-11, Flutter retired 2026-08-26 |
| [The pipeline configuration contract is a matrix](/docs/developer/architecture/decisions/contract-matrix/) | 2026-08-11 |
| [System tables are migrated before the workspace compile](/docs/developer/architecture/decisions/system-tables-before-compile/) | 2026-09-05 |
| [Workspace validation fails when it cannot run](/docs/developer/architecture/decisions/validator-fails-closed/) | 2026-10-01 |
| [lookup.db is rebuilt only when its inputs change](/docs/developer/architecture/decisions/lookup-db-fingerprint/) | 2026-10-11 |
| [Rules written as JSON are rendered to JetRules source](/docs/developer/architecture/decisions/symbolic-rules-rendered/) | 2026-10-12 |
| [User SQL runs on restricted database logins](/docs/developer/architecture/decisions/restricted-sql-logins/) | 2026-11-11 |
