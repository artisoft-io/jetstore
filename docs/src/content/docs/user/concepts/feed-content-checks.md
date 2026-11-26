---
title: Feed content checks
description: The five checks that judge a delivery's claims against its feed profile, from a summary of each delivery that outlives its claims, what each one reads, when it raises an anomaly, and why no claim leaves the database.
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

The checks read the claims of a delivery by session id, which every table a pipeline writes carries
and indexes. They do not read the whole table. They find it by
the profile's `claim_set.table`: a name with a dot is read as `schema.table`, and a name with no dot
is in the `public` schema, where pipelines write.

**A delivery's claims are usually under the session of the pipeline that mapped it**, not under the
registration's: a pipeline started on a file writes its tables under its own session. So for each
delivery the checks read the registration's own session when the table holds rows of it, and
otherwise the earliest completed pipeline run that read the registration and wrote rows to the table
(a failed run's rows only when no completed run has any). The earliest, because a delivery is read as
it first arrived and a re-run maps it again later. The delivery is still named by its registration's
session, and a verdict's detail names the pipeline run whose claims were read.

## A summary of each delivery

**The checks judge from a summary of each delivery, not from its claims**, so they still judge a
delivery whose claims the retention period has since deleted. Before it judges, each run of
`feed_detect` summarises every delivery of a profiled feed that has no summary yet and whose claims
are still in the table, its predecessors included, so the first run summarises every delivery still
held. The summary is a row of `jetsapi.claim_set_summary`, and it is deleted with the session whose
claims it read, on the retention period, not with the claims themselves. Where the claims have a
shorter retention of their own, the purge keeps a delivery's claims until a run of `feed_detect`
has summarised them, or has run since without summarising them, but never past the session's
retention period. A run with `-dry-run`
judges from the summaries it computes and writes none.

A summary holds counts (rows, distinct keys, distinct members, rows with a date of service), the
newest and oldest date of service, the span between them, a histogram of the rows by days behind the
newest, the number of rows carrying a value in **every** column of the table, and a fingerprint and a
sketch of the delivery's keys. Whether the lag exceeds `max_service_date_lag_days`, and how many rows
fall outside `max_service_date_span_days`, are decided on the exact days when the summary is written,
against the declaration then. So a field added to `required_fields` later is judged on older
deliveries too, and so is a span declared later on one of the histogram's bounds (0, 1, 7, 14, 21, 28,
30, 31, 45, 60, 90, 120, 150, 180, 270, 365, 545, 730 or 1095 days). A change the summary cannot answer,
on a delivery whose claims are gone, is reported as not judged rather than guessed.

## The five checks

| Check | What it reads | It raises an anomaly when | Profile fields it needs |
|---|---|---|---|
| **Stale service dates** (`service_date_stale`) | the newest date of service in the delivery | the newest date of service is more than `max_service_date_lag_days` before the delivery's arrival | `claim_set`, `max_service_date_lag_days` |
| **Wide service-date span** (`service_date_span`) | each row's date of service against the delivery's newest | the share of the delivery's rows with a date of service that are dated more than `max_service_date_span_days` before its newest is above `max_out_of_span_share`, which is 0 when absent, so that any such row fires. Incremental feeds only | `claim_set`, `max_service_date_span_days`; `max_out_of_span_share` is optional |
| **Missing required field** (`field_completeness`) | each required field, row by row | the share of the delivery's rows carrying a value is below the field's `min_present_share`. A value is present when it is not empty | `claim_set`, `required_fields` |
| **Claims delivered again** (`delivery_overlap`) | the sketches of the delivery's keys and of its predecessor's | *incremental feed*: the estimated share of the delivery's keys already in its predecessor is above `max_redelivered_share` by more than the estimate's error. *Cumulative feed*: more of the predecessor's keys are estimated missing from the delivery than the estimate's error, so the delivery lost claims | `claim_set`; `max_redelivered_share` for an incremental feed |
| **Delivery did not advance** (`delivery_unchanged`) | the distinct keys and the newest date of service of the delivery and of its predecessor | the delivery holds exactly its predecessor's keys **and** its newest date of service is no later than the predecessor's | `claim_set` |

**The span is judged row by row, so a few late rows need not fire it.** With no
`max_out_of_span_share`, the check fires when any row is outside the span, which is the same as
judging the newest less the oldest date of service against `max_service_date_span_days`, and its
anomaly records that span in days against the declared one. A single late row, a reversal of a
months-old claim say, then fires the check as surely as a delivery that is back-dated as a whole.
Declaring `max_out_of_span_share` lets that share of rows through; the anomaly then records the share
of rows outside the span against the declared share, and its basis gives the span and the counts as
well.

**Records a retain window dropped still count.** A mapping step with a `retain_window` keeps only the
records dated within a window around the file's date and counts the rest, by reason, on the run that
read the delivery (see [the operators](/docs/developer/compute-pipes/operators/)). Those are the
records these two checks exist to see, so both read the counts:

- *Stale service dates* counts the records dropped as before the window as stale when
  `max_service_date_lag_days` is at most the window's days before the file date. It changes the
  verdict only when every dated record of the delivery was dropped as older than the window: the
  check then fires where it would otherwise have had nothing to judge.
- *Wide service-date span* counts the records dropped before or after the window as outside the span
  when `max_service_date_span_days` is at most the window's width, with the rows outside the span and
  with the rows carrying a date of service in its share.

Both take the file date to be close to the arrival, and the anomaly's detail names the window and
the drops so a reader can check. The span check's detail and basis also quote the run's *Input Span*
and *Core Span* (see [Home](/docs/user/screens/home/)), which are measured before the window drops
anything. A pipeline with no retain window drops nothing, and both checks judge as before.

Whether a feed is **incremental** or **cumulative** is the source configuration's **Is each delivery
cumulative?** setting, not part of the profile. It decides which half of *claims delivered again*
applies, and *wide service-date span* does not judge a cumulative feed, whose deliveries are meant to
reach back.

**Keys are counted, never rows.** A claim is identified by the profile's `key_columns`, and the
overlap and *did not advance* checks compare distinct values of those columns.

**Claims delivered again is an estimate, and says so.** Each summary keeps a *sketch* of its keys (a
HyperLogLog of 16,384 registers, 12 KB), and two sketches estimate how many keys two deliveries
share. The estimate's standard error is about 0.8% of the keys of the two together for large
deliveries, and a key or two for small ones; the error stated with it is three standard errors and
three keys. The check's detail and its anomaly's basis give the estimate and its error, and the
check fires only when the estimate passes the declared share, or zero missing keys, by more than that
error. So a few claims missing from a cumulative feed of many thousands can go unseen, and a share
just above `max_redelivered_share` may not fire. *Delivery did not advance* stays exact: each summary
also keeps a *fingerprint* of its keys, the sum of a 64-bit hash of each, and two key sets with the
same fingerprint and the same number of keys are equal but for a chance of about one in 2^64. Its
anomalies are written under generation 2 (`delivery_overlap@2`), so the findings of the estimate are
not mistaken for the counted ones written before it. So a delivery that
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
- the predecessor has no claims in the table, and no summary;
- the check compares with a predecessor and the delivery or its predecessor has fewer distinct keys
  than the small-delivery minimum (11 by default): its summary keeps no fingerprint or sketch, so the
  two are not compared (`small_delivery`);
- the delivery's claims are gone and its summary was written under a declaration the check cannot
  read it against now, such as another `claim_set`, or a span off the histogram's bounds
  (`summary_declaration_changed`).

## What leaves the database

Claims are about members, and **only aggregates leave the database**. Each delivery's summary is
one query, which computes everything above inside the database, the fingerprint and the sketch
included, so no claim, key or member is returned, written to the summary or an anomaly, or shown.
The newest and oldest dates of service of a delivery are kept in its summary, and are never written
to an anomaly.

- **A lag, never a date.** *Stale service dates* records how many days the newest date of service is
  behind the arrival. For a delivery with few distinct members (fewer than 11 by default) the summary
  keeps its dates as the first of their month and the check records the lag in calendar months,
  because the newest date of a small delivery may belong to a handful of people. Whether the lag
  exceeds the declared one is still decided on the exact days, when the summary is written.
- **A fingerprint and a sketch, only of many keys.** The fingerprint and the sketch are computed from
  a hash of each key and are kept only for a delivery of at least 11 distinct keys (by default):
  the fingerprint of a single claim would be that claim's hash, and a short claim number can be found
  from its hash by trying every number. An anomaly says *equal*, or gives an estimated count.
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
