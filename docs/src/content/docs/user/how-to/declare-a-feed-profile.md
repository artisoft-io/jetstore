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

In this version JetStore stores and checks the profile; the feed checks that read it come in a later
release.

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
| `period_type` | The unit in which deliveries are due: `month_period`, `week_period` or `day_period`, JetStore's own periods. | a delivery that is overdue |
| `periods_per_delivery` | How many periods one delivery covers: `2` with `week_period` for every two weeks, `3` with `month_period` for quarterly. Absent is 1. | a delivery that is overdue |
| `overdue_after_days` | The grace, in days, after the period in which a delivery was due. | a delivery that is overdue |
| `size_bytes` | `{"min": …, "max": …}`, the expected size of one delivery in bytes; either bound may be left out. Absent, the check uses the feed's recent deliveries. | a delivery of unusual size |
| `claim_set` | Where the feed's claims are once loaded: `table`, the `key_columns` that identify a claim, the `service_date_column` and the `member_column`. All four are required when it is given. **The key is yours to declare**: JetStore does not infer it, and a check reports how many rows share a key before trusting it. | every check that reads the claims |
| `max_service_date_lag_days` | How far the newest date of service may fall behind the delivery. | claims that stop advancing |
| `max_service_date_span_days` | The widest span of service dates one delivery may hold. Not used for a cumulative feed. | a delivery that reaches back in time |
| `max_redelivered_share` | The largest share, from 0 to 1, of a delivery's claims that may have been delivered before. Not used for a cumulative feed. | claims delivered again |
| `required_fields` | Columns that must be present, each with its `min_present_share`, above 0 and at most 1. | a column that goes missing |
| `declared_by`, `declared_at` | Who declared the profile, and the date, written `YYYY-MM-DD`. Required. | every finding, as its evidence |

A field the profile does not have, such as a misspelt one, is refused rather than ignored.

## How to tell it worked

The summary saves and the flow returns to the table. Edit the feed again and the **Feed Profile
(json)** page shows what you saved. The profile is also carried in the client's configuration when
it is saved to the workspace, so it is loaded again with the client.
