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
check: by hand in this version, over the deliveries registered in a window (see
[Run the feed checks](/docs/install/how-to/run-the-feed-checks/)), and the daily detection task does
not run it yet. A run that cannot read a feed's claim-set table, because a column cannot be read as
the profile says, judges every other feed, writes their findings, and is then recorded as failed,
naming the feed.

## A delivery, its arrival and its predecessor

A **delivery** is one registration of the feed's files: a session of the
[input registry](/docs/user/concepts/files-and-the-input-registry/) whose table name is the feed's.
Its **arrival** is the time it was registered, which is what its session id records. Its
**predecessor** is the feed's previous registration, in the order they were registered.

Each delivery is judged against its predecessor and nothing that came after it. So a check run over
months of history says, for each delivery, what it would have said the day that delivery arrived.

The checks read the claims of two sessions, the delivery's and its predecessor's, by session id,
which every table a pipeline writes carries and indexes. They do not read the whole table. They find
it by the profile's `claim_set.table`: a name with a dot is read as `schema.table`, and a name with
no dot is in the `public` schema, where pipelines write.

## The five checks

| Check | What it reads | It raises an anomaly when | Profile fields it needs |
|---|---|---|---|
| **Stale service dates** (`service_date_stale`) | the newest date of service in the delivery | the newest date of service is more than `max_service_date_lag_days` before the delivery's arrival | `claim_set`, `max_service_date_lag_days` |
| **Wide service-date span** (`service_date_span`) | the oldest and newest dates of service in the delivery | they are more than `max_service_date_span_days` apart. Incremental feeds only | `claim_set`, `max_service_date_span_days` |
| **Missing required field** (`field_completeness`) | each required field, row by row | the share of the delivery's rows carrying a value is below the field's `min_present_share`. A value is present when it is not empty | `claim_set`, `required_fields` |
| **Claims delivered again** (`delivery_overlap`) | the distinct keys of the delivery and of its predecessor | *incremental feed*: the share of the delivery's keys already in its predecessor is above `max_redelivered_share`. *Cumulative feed*: some key of the predecessor is missing from the delivery, so the delivery lost claims | `claim_set`; `max_redelivered_share` for an incremental feed |
| **Delivery did not advance** (`delivery_unchanged`) | the distinct keys and the newest date of service of the delivery and of its predecessor | the delivery holds exactly its predecessor's keys **and** its newest date of service is no later than the predecessor's | `claim_set` |

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
- the claim-set table holds no row of the delivery's session, or does not exist, as when the pipeline
  that reads the feed skips the database load;
- a column the profile names is not in the table;
- the check compares with a predecessor and no earlier delivery survives. This is the feed's first
  delivery, or its earlier deliveries were purged by the retention period, and the two cannot be told
  apart, so the check says the history may be truncated;
- the predecessor has no claims in the table.

## What leaves the database

Claims are about members, and **only aggregates leave the database**. Each delivery is one query,
and it returns counts of rows, distinct keys, distinct members and shared keys, a lag and a span in
days, the number of rows carrying each required field, and yes-or-no answers. No claim, key, member
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
