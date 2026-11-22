---
title: Declare a feed profile
description: Say how often a client's feed delivers, how large a delivery is, where its claims are and what thresholds it is judged against, on its source configuration.
kind: how-to
sidebar:
  order: 11
---

A **feed** is one source configuration: the files of one object type, from one vendor, for one
client. Its **feed profile** is what you know about that feed and the files cannot say for
themselves: when a delivery is due, how large one usually is, which table holds its claims and which
columns identify them, and how much drift is acceptable. Anomaly detection's feed checks judge a feed
against its profile, and **a feed with no profile is not judged**: it is listed as not judged, by
check, rather than judged against defaults.

Eight checks read the profile. Three read only what JetStore records when a delivery is registered,
so they judge a feed whose claims are never loaded; they are described in
[The registration checks](#the-registration-checks) below. The other five read the delivery's claims
in the table the profile names.

Whether each delivery holds the whole feed is not part of the profile. It is the source
configuration's **Is each delivery cumulative?** setting, and the checks read it from there.

**Before you start**: your account has `client_config`, and the source configuration exists (see
[Source Configuration](/docs/user/screens/flows/source-configuration/)).

1. Open **Source Configuration** from the *JetStore Workspace* menu, tick the feed's row and press
   **Edit**.
2. Press **Next** through the pages to **Feed Profile (json)**. The pages before it keep what they
   show.
3. Enter the profile as one JSON object. `declared_by` and `declared_at` are required; every other
   field is optional, and a check whose fields are absent does not judge the feed. For example, a feed
   due monthly, ten days' grace, with its claim set and two thresholds:

   ```json
   {
     "period_type": "month_period",
     "overdue_after_days": 10,
     "claim_set": {
       "table": "hc:Claim",
       "key_columns": ["hc:claim_number", "hc:line_number"],
       "service_date_column": "hc:service_date",
       "member_column": "hc:member_id"
     },
     "max_service_date_lag_days": 45,
     "required_fields": [{ "column": "hc:paid_amount", "min_present_share": 0.95 }],
     "declared_by": "you@example.com",
     "declared_at": "2026-11-21"
   }
   ```

4. Press **Next**, then **Save** on the summary. If the profile is refused, the message names each
   field at fault, such as `/overdue_after_days: overdue_after_days is -2 and must be at least 0`, and
   nothing is saved; go **Previous** to the profile, correct it and save again.

**To remove a profile**, empty the field and save. The feed is then not judged.

## The fields

| Field | What it says | Read by the check for |
|---|---|---|
| `period_type` | The unit in which deliveries are due: `month_period`, `week_period` or `day_period`, JetStore's own periods. | a delivery that is overdue; a delivery for a past period |
| `periods_per_delivery` | How many periods one delivery covers: `2` with `week_period` for every two weeks, `3` with `month_period` for quarterly. Absent is 1. | a delivery that is overdue |
| `overdue_after_days` | The grace, in days, after the period in which a delivery was due. | a delivery that is overdue |
| `size_bytes` | `{"min": …, "max": …}`, the expected size of one delivery in bytes; either bound may be left out. Absent, the check uses the feed's recent deliveries (see below). | a delivery of unusual size |
| `claim_set` | Where the feed's claims are once loaded: `table`, the `key_columns` that identify a claim, the `service_date_column` and the `member_column`. All four are required when it is given. A `table` with a dot in it is read as `schema.table`, so `audit.hc:Claim` is the table `hc:Claim` in the schema `audit`; a name with no dot is in the `public` schema, where pipelines write, and the checks do not judge a feed whose table name has more than one dot. **The key is yours to declare**: JetStore does not infer it, and a check reports how many rows share a key before trusting it. | every check that reads the claims |
| `max_service_date_lag_days` | How far the newest date of service may fall behind the delivery. | claims that stop advancing |
| `max_service_date_span_days` | The widest span of service dates one delivery may hold. Not used for a cumulative feed. | a delivery that reaches back in time |
| `max_redelivered_share` | The largest share, from 0 to 1, of a delivery's claims that may have been delivered before. Not used for a cumulative feed. | claims delivered again |
| `required_fields` | Columns that must be present, each with its `min_present_share`, above 0 and at most 1. | a column that goes missing |
| `declared_by`, `declared_at` | Who declared the profile, and the date, written `YYYY-MM-DD`. Required. | every finding, as its evidence |

A field the profile does not have, such as a misspelt one, is refused rather than ignored.

## The registration checks

Each check below needs the fields in its *Needs* column. A feed whose profile lacks one is reported
as **not judged** by that check, naming the field, and the other checks still judge it. A feed with
no profile, or whose saved profile no longer passes validation, is not judged by any check, and the
report says which of the two.

| Check | Fires when | Needs |
|---|---|---|
| `feed_overdue` | No delivery has come for the period one was due in, past the grace. | `period_type`, `overdue_after_days` |
| `delivery_size` | A delivery's size is outside `size_bytes`, bounds included, or, with no `size_bytes`, outside the feed's recent range. | nothing |
| `delivery_backdated` | A delivery is registered for a period earlier than one the feed has already delivered. | `period_type` |

**What a delivery is.** A delivery is one file of the feed at its first registration, matched to the
feed by the source configuration's table name; a later registration of the same file is
re-processing, counted and not judged (see
[Feed content checks](/docs/user/concepts/feed-content-checks/#what-counts-as-a-delivery-and-how-re-processing-is-reported)).
Its arrival is the time it was first registered, its period is the
source period it was registered under, and its size is the file's size as JetStore last listed it. A
delivery registered under another client than its feed's is still counted as the feed's, and the
report lists such deliveries so that the registration can be looked into.

**Overdue** counts from the **latest period the feed has delivered**, not from the period of the last
file to arrive: a late file for an old period does not make an up-to-date feed look overdue. With a
latest period P, the next delivery is due in period P + `periods_per_delivery`, and the feed is
overdue once that period has ended and `overdue_after_days` more have passed. A week period runs
Thursday to Wednesday and a month period is a calendar month. The finding is attached to the feed's
last delivery, the one after which nothing came, so a feed that stays overdue is reported once, not
every day.

**Size, with no `size_bytes`**, is judged against the feed's own recent deliveries: the median of the
last 8 deliveries' sizes, plus or minus the larger of 3 scaled median absolute deviations and half
the median. A delivery with fewer than 4 earlier sizes is not judged. These figures are starting
values, not measured ones; the finding states the range it used. Declaring `size_bytes` is better
when you know what a delivery should weigh, because the recent range follows a feed that drifts
slowly.

**A back-dated delivery** is one registered for a period earlier than the latest period of any
delivery before it. The check cannot tell a mistaken registration from a deliberate backfill of past
periods (registering a file again is not a new delivery, and is not judged); it reports the fact, and the finding names both periods. A delivery registered for the
current period whose claims reach back in time is a different case, which the span check finds.

**Every finding** names the profile it judged against, who declared it and when, and compares each
delivery only with the deliveries registered before it. The first delivery JetStore still holds has
nothing before it, so a check that compares with earlier deliveries does not judge it, and says that
earlier history may have been removed.

## When the checks run

The checks are run by the `feed_detect` command: daily by the detection task, where the installation
schedules it, after anomaly detection and before shadow triage, and by hand by an installation
operator (see [Run the feed checks](/docs/install/how-to/run-the-feed-checks/)). The
[Detection](/docs/user/screens/detection/) screen shows each run. Each run judges the deliveries registered in its window, 26 hours
by default, and judges whether each feed is overdue at the time it runs; every finding becomes an
anomaly on the feed. A run over a window an earlier run covered writes nothing twice. A profile you
save is used from the next run on, and a run never judges a delivery against a profile it did not
have: the findings name the profile's `declared_by` and `declared_at`.

## How to tell it worked

The summary saves and the flow returns to the table. Edit the feed again and the **Feed Profile
(json)** page shows what you saved. The profile is also carried in the client's configuration when
it is saved to the workspace, so it is loaded again with the client.
