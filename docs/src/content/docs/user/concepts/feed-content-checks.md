---
title: Feed content checks
description: The five checks that read a delivery's claims against its feed profile, what each one reads, when it raises an anomaly, and why only counts and lags leave the database.
kind: explanation
sidebar:
  order: 9
---

A feed's [profile](/docs/user/how-to/declare-a-feed-profile/) says what its deliveries should look
like. Five of anomaly detection's feed checks open a delivery's **claims**, in the table the
profile's `claim_set` names, and judge them against that declaration. They are the **content
checks**. Two other feed checks, for a delivery that is overdue or of unusual size, read only the
registration of a file and are described with the profile.

The checks run in the `feed_detect` command, beside the two registration checks and the back-dating
check, over the deliveries registered in a window. Where an installation schedules the daily detection
task, the task runs it every day over the last 26 hours, and it can also be run by hand (see
[Run the feed checks](/docs/install/how-to/run-the-feed-checks/)). A run that cannot read a feed's claim-set table, because a column cannot be read as
the profile says, judges every other feed, writes their findings, and is then recorded as failed,
naming the feed.

## A delivery, its arrival and its predecessor

A **delivery** is one file of the feed, at its first registration: a session of the
[input registry](/docs/user/concepts/files-and-the-input-registry/) whose table name is the feed's.
Its **arrival** is the time it was registered, which is what its session id records. Its
**predecessor** is the feed's previous delivery, in the order they were registered. A file
registered again is not a new delivery; see
[What counts as a delivery](#what-counts-as-a-delivery-and-how-re-processing-is-reported).

Each delivery is judged against its predecessor and nothing that came after it. So a check run over
months of history says, for each delivery, what it would have said the day that delivery arrived.

The checks read the claims of two deliveries, the delivery and its predecessor, by session id, which
every table a pipeline writes carries and indexes. They do not read the whole table. They find it by
the profile's `claim_set.table`: a name with a dot is read as `schema.table`, and a name with no dot
is in the `public` schema, where pipelines write.

**A delivery's claims are usually under the session of the pipeline that mapped it**, not under the
registration's: a pipeline started on a file writes its tables under its own session. So for each
delivery the checks read the registration's own session when the table holds rows of it, and
otherwise the earliest completed pipeline run that read the registration and wrote rows to the table
(a failed run's rows only when no completed run has any). The earliest, because a delivery is read as
it first arrived and a re-run maps it again later. The delivery is still named by its registration's
session, and a verdict's detail names the pipeline run whose claims were read.

## The five checks

| Check | What it reads | It raises an anomaly when | Profile fields it needs |
|---|---|---|---|
| **Stale service dates** (`service_date_stale`) | the newest date of service in the delivery | the newest date of service is more than `max_service_date_lag_days` before the delivery's arrival | `claim_set`, `max_service_date_lag_days` |
| **Wide service-date span** (`service_date_span`) | each row's date of service against the delivery's newest | the share of the delivery's rows with a date of service that are dated more than `max_service_date_span_days` before its newest is above `max_out_of_span_share`, which is 0 when absent, so that any such row fires. Incremental feeds only | `claim_set`, `max_service_date_span_days`; `max_out_of_span_share` is optional |
| **Missing required field** (`field_completeness`) | each required field, row by row | the share of the delivery's rows carrying a value is below the field's `min_present_share`. A value is present when it is not empty | `claim_set`, `required_fields` |
| **Claims delivered again** (`delivery_overlap`) | the distinct keys of the delivery and of its predecessor | *incremental feed*: the share of the delivery's keys already in its predecessor is above `max_redelivered_share`. *Cumulative feed*: some key of the predecessor is missing from the delivery, so the delivery lost claims | `claim_set`; `max_redelivered_share` for an incremental feed |
| **Delivery did not advance** (`delivery_unchanged`) | the distinct keys and the newest date of service of the delivery and of its predecessor | the delivery holds exactly its predecessor's keys **and** its newest date of service is no later than the predecessor's | `claim_set` |

**The span is judged row by row, so a few late rows need not fire it.** With no
`max_out_of_span_share`, the check fires when any row is outside the span, which is the same as
judging the newest less the oldest date of service against `max_service_date_span_days`, and its
anomaly records that span in days against the declared one. A single late row, a reversal of a
months-old claim say, then fires the check as surely as a delivery that is back-dated as a whole.
Declaring `max_out_of_span_share` lets that share of rows through; the anomaly then records the share
of rows outside the span against the declared share, and its basis gives the span and the counts as
well.

Whether a feed is **incremental** or **cumulative** is the source configuration's **Is each delivery
cumulative?** setting, not part of the profile. It decides which half of *claims delivered again*
applies, and *wide service-date span* does not judge a cumulative feed, whose deliveries are meant to
reach back.

**Keys are counted, never rows.** A claim is identified by the profile's `key_columns`, and the
overlap and *did not advance* checks compare distinct values of those columns. So a delivery that
repeats last week's claims is found even when its file differs byte for byte, in a load time, a
batch number or the order of its rows. Each of these two checks also reports how many rows the
delivery has against how many distinct keys: when rows outnumber keys, the declared key may not
identify a claim, and that is worth knowing before trusting a judgement made with it.

## Not judged is not clean

A check reports a delivery as **not judged**, with the reason, rather than as passing, when:

- the feed has no profile, or the profile does not declare a field the check needs;
- the check judges incremental feeds only and the feed is cumulative;
- the claim-set table holds no row of the delivery's session, nor of any pipeline run that read it,
  or does not exist, as when the pipeline that reads the feed skips the database load;
- a column the profile names is not in the table;
- the check compares with a predecessor and no earlier delivery survives. This is the feed's first
  delivery, or its earlier deliveries were purged by the retention period, and the two cannot be told
  apart, so the check says the history may be truncated;
- the predecessor has no claims in the table.

## What leaves the database

Claims are about members, and **only aggregates leave the database**. Each delivery is one query,
and it returns counts of rows, distinct keys, distinct members and shared keys, a lag and a span in
days, the number of rows carrying a date of service and of those outside the declared span, the
number of rows carrying each required field, and yes-or-no answers. No claim, key, member
or date of service is returned, written to an anomaly, or shown.

- **A lag, never a date.** *Stale service dates* records how many days the newest date of service is
  behind the arrival. For a delivery with few distinct members (fewer than 11 by default) it records
  the lag in calendar months instead, because the newest date of a small delivery may belong to a
  handful of people. Whether the lag exceeds the declared one is still decided on the exact days,
  inside the database.
- **Equal or not equal, never a fingerprint.** *Delivery did not advance* compares the two key sets
  inside the database, by counting the keys each holds and the keys they share. The anomaly says
  *equal*; nothing that could be traced back to a key is computed or stored.
- **Names are names.** The table and column names in a profile are read as names, however they are
  written, so a profile cannot make a check run anything but its own query.

## What an anomaly says

Each anomaly is on the **feed**, and names the delivery by its session. Its **basis** says what the
delivery was compared with: the profile field and its value, **who declared it and on what date**,
and the counts the judgement rests on. A declaration can be wrong, or out of date, and the basis is
there so that whoever reads the anomaly can weigh it.

Running the checks again over the same deliveries writes nothing twice: an anomaly is identified by
the check, the feed and the delivery.

## What counts as a delivery, and how re-processing is reported

A delivery is a **file**: a distinct file key among the feed's file registrations, at the first time
it was registered. Its arrival, its session and its period are that first registration's.

JetStore registers a file again when it processes it again. A re-map of a client's earlier files
registers every one of them again, and a load can register the same file two or three times within
seconds. The new registration has a session of its own and keeps the file's file key and its
period. It is **re-processing**, not a delivery:

- **No check judges it.** Neither the five content checks nor the overdue, size and back-dated
  checks reach a verdict on it, so a re-map does not look like a run of back-dated files, and a file
  is never compared with its own second load.
- **It is never a predecessor.** The next new file is compared with the previous file as it first
  arrived.
- **Its claims are not read.** A check reads the claims of the delivery's first session, or of the
  pipeline run that first mapped it. A
  re-processing comes later, possibly under another mapping, and a check run over past months must
  say what it would have said on the day the file arrived. If the first session's claims are gone,
  purged or never loaded, the delivery is *not evaluable*, and the detail names the sessions that
  processed the file again.
- **It is counted.** Every check's coverage line names each feed with re-processing in the run's
  window: *N re-registration(s) of M file(s) already delivered, re-processing and not judged*. The
  run record carries the same counts.

A file that is new to the feed but registered for an old period is still a delivery. A backfill of
three missed weeks raises one back-dated finding for each of the three files, however many times
each was registered. A registration with no file key has nothing to identify its file, and is a
delivery of its own.
