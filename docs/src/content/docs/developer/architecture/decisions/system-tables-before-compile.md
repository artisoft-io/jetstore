---
title: "System tables are migrated before the workspace compile"
description: "Why the apiserver migrates the jetsapi system tables before it compiles the workspace, and the domain tables after."
kind: explanation
---

*Decided 2026-09-05.*

## Context

At start the apiserver brings the database to the deployed release and compiles the workspace
(`listenAndServe`, `jets/apiserver/server.go`). The migration ran as one step after the compile,
because the domain tables are generated from the workspace's classes, which are known only once the
compile has written `build/tables.json`. But the compile itself writes system tables: it records the
workspace version. When a release added a column the compile writes, the compile ran against the
previous release's schema and the apiserver failed to start.

## Decision

Split the migration in two. `checkSystemTablesVersion` migrates the system tables, described by
`jets/jets_schema.json` in the image, **before** the compile; `checkDomainTablesVersion` migrates the
domain tables after it. The order at start is `checkJetStoreSchema`, `checkSystemTablesVersion`,
`checkWorkspaceVersion` (the compile), `checkDomainTablesVersion`.

## Consequences

- The system-table migration is a function in `jets/migratedb`, not a run of the `update_db` binary,
  because `update_db` always reads `build/tables.json` at the end and cannot run before a compile.
- Both halves are gated on the release, not on the workspace version, which changes on every
  recompile.
- Only `checkDomainTablesVersion` records the release, so a start that migrates and then fails runs
  both halves again next time. Both must therefore be idempotent.

## Where the reason is recorded

- The comment on `checkSystemTablesVersion` in `jets/apiserver/server.go`.
- Commit `15e20018`, *apiserver: migrate the system tables before the workspace compile*.
- [Compiling a workspace](/docs/developer/workspace/compiling/#the-compile-sits-between-two-halves-of-a-schema-migration-and-the-order-is-load-bearing).
