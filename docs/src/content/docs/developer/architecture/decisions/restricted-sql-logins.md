---
title: "User SQL runs on restricted database logins"
description: "Why the SQL a user sends runs on two restricted PostgreSQL logins rather than on the apiserver's connection after SET ROLE."
kind: explanation
---

*Decided 2026-11-11.*

## Context

JetStore lets a user send SQL: the Query Tool, *Submit DDL*, a runbook's query, wait and ddl steps, and
the SQL fragments of a data table. That SQL used to run on the apiserver's own database connection.
The writing paths ran with the apiserver's full rights, so a user allowed to use the IDE could change
`jetsapi.users` or `role_capability` and give themselves any role. Reads ran after
`SET LOCAL ROLE jets_reader`, but a PostgreSQL session may switch back to any role its *login* is a
member of, and the statement itself can do it, with `set_config('role', ...)` in a `DO` block or a
function call. So a role switch on the apiserver's login restricts nothing the restricted SQL cannot
undo.

## Decision

Run user SQL on logins that are themselves restricted. Package `jets/dblogin` creates two logins per
database, each a member of one group role and of nothing else, so there is no role to switch back
to:

- **`jets_reader`**, for every read whose SQL comes from the client: every table, less the secret
  columns and the login table.
- **`jets_ide`**, for the free-SQL paths that write and for the workspace's own init scripts in
  `update_db`: every table, but the security tables read-only and without their secret columns, and
  no `TRIGGER` privilege, since a trigger on a table the apiserver writes would run as the apiserver.

## Consequences

- The apiserver and `update_db` create and update the logins at every start, so the apiserver's
  database user needs `CREATEROLE`.
- Each login's password is generated once per database and kept in `jetsapi.restricted_login`, which
  only the apiserver's user may read. It is sent to PostgreSQL as a SCRAM verifier, so it never
  appears in a statement a server log could record, and no deployment setting is involved.
- The login is per database, a member of a cluster-wide group role, because roles are cluster-wide:
  two databases on one cluster would otherwise reset each other's passwords.
- The apiserver's user is a member of both logins, so it can cancel their statements (a runbook's
  *Abort*) and owns, by membership, the tables the IDE login creates.

## Where the reason is recorded

- The package documentation of `jets/dblogin` (`dblogin.go`), *Why logins, and not SET ROLE*.
- Commit `40ecb1da`, *User SQL runs on restricted database logins, not the apiserver's*.
- [Design of the system tables](/docs/developer/workspace/system-tables/#the-security-tables-and-the-restricted-logins).
