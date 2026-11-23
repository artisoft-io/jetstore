---
title: Run the feed checks
description: Run feed_detect over the deliveries of the last day or over a history, read what it recorded, and run a retrospective through the Query Tool on a database the command cannot reach.
kind: how-to
sidebar:
  order: 14
---

The **feed checks** judge each client feed against the
[feed profile](/docs/user/how-to/declare-a-feed-profile/) declared for it: three read what JetStore
records when a delivery is registered, and five read the delivery's claims, aggregated inside the
database (see [Feed content checks](/docs/user/concepts/feed-content-checks/)). The `feed_detect`
command runs all eight. It needs no inference server and reads no member-level value out of the
database. Where the installation sets `JETS_AGENTIC_DETECT_HOUR_UTC`, the daily detection task runs
it as `feed_detect -since 26h`, after `anomaly_detect` and before `shadow_triage`, over every client;
it is run by hand for anything else this page describes.

`feed_detect` is in the same image as `anomaly_detect`. It reads the database from
`JETS_DSN_URI_VALUE`, or from `JETS_DSN_JSON_VALUE` as the other detection commands do, or from
`-dsn`.

## The modes

| Mode | Command | What it does |
|---|---|---|
| window, the default | `feed_detect -since 26h` | Judges the deliveries registered in the last 26 hours, and whether each feed is overdue now. This is the daily run. |
| backfill | `feed_detect -mode backfill -from 2026-01-01` | Judges every delivery registered since the date, each against the deliveries before it and nothing after, and dates each finding at the first daily run that would have seen it. *Overdue* is judged at every such run. `-step` sets the interval, a day by default, counted back from `-as-of`. |
| export | `feed_detect -mode export -from … -as-of … -results results.txt` | Prints the statements the checks run, to be run elsewhere. Opens no connection. |
| judge | `feed_detect -mode judge -from … -as-of … -results results.txt` | Runs the checks over what those statements returned, and prints their verdicts. Opens no connection. |

The flags every mode takes:

- `-client a,b` limits the run to those clients; without it, every client with a source
  configuration is judged.
- `-profiles profiles.json` reads the profiles from a file, one JSON object keyed by the feed's table
  name, instead of from the source configurations. A table name in the file that no feed has is
  reported, so a misspelt one does not leave its feed silently unjudged.
- `-as-of` is the detection time, RFC 3339; it defaults to now.
- `-dry-run`, for a window or a backfill, judges and writes nothing.

A run over deliveries an earlier run judged writes nothing twice: a finding is identified by its
check, the feed and the delivery.

## What a run records

A window or a backfill writes one row to `jetsapi.agentic_detection_run`, under the command
`feed_detect`, and logs the same. The row's summary has a **coverage** entry per check:

- how many feeds it judged, and every feed it did not, with the reason: no profile, a stored profile
  that no longer passes validation, a field the check needs and the profile lacks, a cumulative feed
  for a check that judges incremental ones only;
- how many of the window's deliveries fired, were silent, and were **not judged**, by reason. Not
  judged is never clean;
- per feed, how many verdicts carry `history_truncated`: a delivery with nothing before it is the
  feed's first, or its history was purged, and the two cannot be told apart.

**A feed whose claim-set statement fails** does not stop the others. Its content checks judge
nothing, the other feeds are judged and their findings written, and the run is then recorded
**failed**, naming the feed. The record gives the database's error code and not its message, because
a message about a value that could not be read quotes the value, and it may be a member's. The usual
cause is a profile that names a column of the wrong kind, such as a member column as the date of
service.

## Raise a backfill's findings as incidents

A backfill writes its findings dated at the first daily run that would have seen each one, so the
daily `shadow_triage -since 24h` never reads them. To have shadow triage raise them as incidents, run
it once with a window reaching back to the backfill's start, and with `-feeds-only`:

```bash
feed_detect -mode backfill -from 2026-03-19
shadow_triage -feeds-only -since 6000h -limit 2000
```

`-feeds-only` classifies only the sessions carrying a feed finding detected in the window, and **no
pipeline run**. Without it, the same window would classify every run that started in it, and raise run
incidents for runs that no daily triage ever read. `-since` is a duration counted back from now
(`6000h` is 250 days), and `-limit` caps the sessions read, newest first, so give it more than the
backfill's number of findings. A second run raises nothing twice. `-feeds-only` cannot be combined
with `-session` or `-process`, and `-dry-run` reports what it would write and writes nothing.

**On a deployment that schedules detection**, run both from the daily detection task's definition
(the stack output `AgenticDetectTaskDefinitionArn`), overriding two containers' commands. The entry
point already names the binary, so the command is the flags alone: `feedDetectContainer` takes
`-mode backfill -from 2026-03-19`, and `shadowTriageContainer` takes `-feeds-only -since 6000h -limit
2000`. The task runs `anomalyDetectContainer` first with its usual command, which writes nothing it
has already written, and each container waits for the one before it to finish. The incidents are written in shadow mode, at
`diagnosed`, and the people with `agent_supervision` see them on the Incidents screen, where they are
given verdicts.

## Run a retrospective through the Query Tool

A retrospective asks what the checks would have said over a past period, on a database `feed_detect`
cannot reach. It runs in rounds: the export prints statements, you run them in the
[Query Tool](/docs/user/how-to/query-the-data/) and keep what they return, and the export, given
those results, prints the statements that depend on them. The statements are the checks' own, with
the profiles and the window written in, so the verdicts at the end are the checks' verdicts on that
database's own aggregates.

**Before you start**: the profiles in a file, declared **before** you read any result, and access to
the Query Tool on the client's installation (`workspace_ide`).

1. Print the first round, choosing the period and fixing the detection time:

   ```bash
   feed_detect -mode export -from 2026-01-01 -as-of 2026-06-01T05:00:00Z \
     -profiles profiles.json -results results.txt > round.sql
   ```

   The file opens with the command to run next; keep its flags as they are in every later round and
   in the judge.
2. `round.sql` holds one or more **parts**. Each is one statement on one line, after comment lines
   saying what it reads. Paste a part into the Query Tool and press **Submit Query**. It returns one
   row with one text column.
3. Copy that text, as it is, onto the end of `results.txt`. Do the same for every part of the round.
4. Run the command from step 1 again. It prints the next round. Repeat steps 2 to 4 until it prints
   *The export is complete*. With `-client` there are three rounds: the feeds, then their
   registrations and claim-set tables, then one statement per delivery. Without `-client` the first
   round lists the clients, and there are four.
5. Judge the results:

   ```bash
   feed_detect -mode judge -from 2026-01-01 -as-of 2026-06-01T05:00:00Z \
     -profiles profiles.json -results results.txt -json judged.json
   ```

   It prints each check's coverage, every finding with the date it would have been raised, and one
   tab-separated line per verdict: check, feed, delivery, session, verdict, reason. `-json` writes the
   same, with the findings as anomalies.

**The statements carry no semicolon**, because the Query Tool returns nothing for a statement that
contains one. A profile whose claim-set table or column name contains a semicolon is refused by the
export, naming the feed and the field; rename it or leave that feed out of the file.

**Keep each part under the Query Tool's limit.** A statement is cancelled after 110 seconds. A part
combines statements up to 100,000 characters by default, about sixty deliveries in the last round, and
a cumulative feed's statement reads two whole deliveries, so on large tables give
`-max-statement-bytes 20000` or less to make the parts smaller. Results already in the file are not
asked for again, so a round can be resumed.

**What leaves the client's database** is what the checks themselves read: the source configurations
of the feeds, each delivery's registration (session, client and org, file key, period and file size), the column names
of the claim-set tables, and per delivery a dozen counts, a lag and a span. No claim, key, member or
date of service is in a result.
