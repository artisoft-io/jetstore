---
title: "A cumulative file, and why its flag is read at start"
description: "How a cumulative source's lookback reads only the latest delivery, and why the flag is read at startup rather than from the run's source."
kind: explanation
sidebar:
  order: 13
---

**Recorded 2026-10-15.**

**The symptom.** A client whose file is *cumulative* (each delivery holds the whole feed) sends a new
delivery every month, and a pipeline reading that object type with `lookback_periods: "6"` reads seven
period folders and every session in each: each record once per delivery, so the read grows with
every month and its output repeats every record.

**The flag is `source_config.is_cumulative`, and the run's own source_config row cannot carry it.** A
clinical review's main input is eligibility; the cumulative feeds are pharmacy and medical claims,
which it reads as stage channels with a lookback. So two halves meet (`cumulative_source.go`):

- **At start**, `loadCumulativeObjectTypes` (called from `shardingInitializeCpipes` after
  `prepareCpipesEnv`) reads every source_config row of the run's `$CLIENT` and writes
  `${CUMULATIVE_OBJECT_TYPES}` into the env. The env is saved with the startup and replayed by every
  reducing step, so all steps see what the sharding step read.
- **The channel names what it reads**: `source_object_type` on a stage input channel (or a merge
  channel). When it names a cumulative object type, `GetS3Objects4LookbackPeriod` and
  `GetPartitionSize4LookbackPeriod` keep only `latestDelivery`.

**Latest means the most recent period folder holding data, then its latest session** — not the
largest session id across the lookback. The period folder is the delivery's date
(`$CLIENT/<ObjectType>/${MONTH_PERIOD}M/$SESSIONID`); the session id is when it was mapped. Re-mapping
an old delivery makes a newer session in an older period, and must not displace the latest delivery.
Session ids compare as numbers (`sessionAfter`).

**Three things it refuses or does not do.**
- **The stage folder holds no org.** If some of a client's orgs declare an object type cumulative and
  others do not, it can be read neither way; it is listed in `${MIXED_CUMULATIVE_OBJECT_TYPES}` and a
  cumulative read of it fails the run with the reason.
- **An object directly in a period folder** (no session folder) fails a cumulative read, rather than
  being kept or dropped by guess.
- **A channel with no `lookback_periods` is not affected**: the sharding path lists its prefix
  directly. The flag means nothing until a pipeline names the object type, so setting it changes no
  run of a pipeline that does not.
