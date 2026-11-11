---
title: "lookup.db is rebuilt only when its inputs change"
description: "Why a workspace compile skips rebuilding lookup.db when a fingerprint of its inputs is unchanged, and why modification times are not used."
kind: explanation
---

*Decided 2026-10-11.*

## Context

Packaging the lookup tables into `lookup.db` was most of a workspace compile: about 2.1 s of 2.2 s for
`jets_ws`, measured 2026-10-11. A compile and a rules unit test exclude each other, so that was time a
user waited.

## Decision

Skip the rebuild when the existing `lookup.db` carries a SHA-256 of exactly the inputs a rebuild would
read (`EnsureLookupDb`, `jets/workspace/lookup_db_fingerprint.go`): the deduplicated lookup
definitions, the bytes of every CSV file they name, the SQLite library version and
`lookupDbFormatVersion`. **File modification times are not used**, because a git checkout, a Docker
`COPY` and the database sync all rewrite them.

## Consequences

- Touching a CSV file does not rebuild `lookup.db`; changing one byte of it does.
- Code cannot fingerprint itself, so a change to what the packager writes must bump
  `lookupDbFormatVersion` by hand. That one input is a convention.
- A rebuild writes a new file and renames it into place, so a skipped `lookup.db` is byte for byte
  what a rebuild would write, and a failed build leaves the previous file intact.
- Anything but a readable, complete file with the same fingerprint rebuilds, so a fresh container
  always builds.

## Where the reason is recorded

- The file comment of `jets/workspace/lookup_db_fingerprint.go`.
- Commit `579a7a48`, *Skip repackaging lookup.db when nothing it is built from changed*.
- [Compiling a workspace](/docs/developer/workspace/compiling/#lookupdb-is-rebuilt-only-when-what-it-is-built-from-changed).
