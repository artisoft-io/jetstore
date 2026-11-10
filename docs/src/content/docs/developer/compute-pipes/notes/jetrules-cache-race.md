---
title: "The jetrules caches and the cold start of 2026-08-31"
description: "A double-checked lock that published an empty cache, and the change elsewhere that stopped hiding it."
kind: explanation
sidebar:
  order: 3
---

**Symptom.** A pipeline runs, the workspace is intact, and the rule engine reports
a class it can plainly see:

```
Error getting multi-value data properties for class cintel:Patient_Profile :
error: domain table for class cintel:Patient_Profile is not found in the local
workspace, cache contains 0 entries
```

**Cause: a double-checked lock with the check on the wrong side of the work.**
`GetWorkspaceDomainTables` and its two siblings assigned the shared map *before*
filling it:

```go
if domainTablesMap == nil {          // unsynchronised read
    mx.Lock(); defer mx.Unlock()
    if domainTablesMap == nil {
        domainTablesMap = make(...)  // published empty, then filled
```

A goroutine reaching the outer test inside that window sees a non-nil map, never
takes the lock, and gets an **empty cache with a nil error**. `GetWorkspaceControl`
in the same file has always done it correctly — build into a local, assign on
success — which is the shape all four have now.

**Read the counts.** The log says `Worker Pool of size 6` and carries exactly
**5** copies of the error. One winner, five losers. That arithmetic is what turns
"probably a race" into a diagnosis.

## Why it appeared when it did, which is the part worth carrying

The race is as old as the caches. What changed is that something stopped hiding
it.

`actions_coordinate_cp.go` pre-loads all three caches through
`LoadJetrulesCaches()` — single-threaded, before any worker goroutine exists —
but only `if didSync`. And `didSync` is true only when the workspace was
**fetched from the database**: the image-workspace skip returns
`false, ensureLocalRepoSeeded()`. So enabling that skip for the native lambda
(`5f9b1bc8`, 2026-08-22) silently removed the pre-load, and the first load of
these caches moved from an initialisation step into a six-way race.

**A commit that changes which branch is taken can arm a defect on the branch it
did not touch.** Nothing in that change went near these caches; it changed a
boolean two call frames away, and the boolean was the only thing serialising
them. The pre-load now runs on both paths.

## What a test can do here

Everything, unlike the two workspace-permission defects that preceded it — those
needed a second uid to observe, and this needs two goroutines.
`TestDomainCachesAreNotPublishedBeforeTheyAreFull` releases 16 of them together
and fails if any sees an empty map. Against the original code it reports
`saw 0 entries`, which is the production failure verbatim; two of the three
caches lost the race on the first run and the third did not, which is the usual
reminder that a green race test is weaker evidence than a red one.
