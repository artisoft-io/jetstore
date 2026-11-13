---
title: "Reading a stage folder: the latest session, and the columns it was written with"
description: "latest_session_only keeps the latest completed session of each period of a file_key stage read, and every such csv read checks each file's header line against its channel's columns."
kind: explanation
sidebar:
  order: 15
---

**Recorded 2026-11-12.**

**The symptom.** A pipeline reads another pipeline's output from the stage area through `file_key`,
for example `$CLIENT/<ObjectType>/${WEEK_PERIOD}W`. The folder holds one session folder per run that
wrote to it, and the read takes all of them. After a change to the class behind the channel, the files
are mapped again, and the new session folder lands beside the old one. The read then gets duplicate
records, and the old files, written with fewer columns, are read by position with the new ones: every
column after the inserted properties is shifted. A text value in a number column fails a cast (`strconv.ParseFloat: parsing "…": invalid syntax`). A text value in a text column goes through
unnoticed.

Two changes answer it, and they are independent.

## `latest_session_only`: one session per period

A stage input channel, or a merge channel, that reads through `file_key` may set
`"latest_session_only": true`. Each period folder of the read then keeps one session folder, and the
others are skipped and logged. Nothing is deleted. Kept means:

- **The latest by session id**, compared as numbers. A session id is when the file was *mapped*, not
  when it was delivered, so the option is for a feed with **one delivery per period**. In a period with
  two real deliveries, the read would keep one of them. That is why the option is set per channel and
  is off by default.
- **And a completed run**: a `session_registry` row and a `pipeline_execution_status` row whose status
  leaves its outputs usable (`completed`, `recovered`, `errors`). A failed run is registered too, and a
  running one has written part of its folder, so neither is kept. A period holding data with no
  completed session **fails the read**, naming the period's folder and its sessions, rather than reading
  nothing.
- Sessions a user deleted are dropped first, as for every stage read, so deleting the latest session
  makes the one before it the latest.
- With a cumulative object type (`source_object_type`, see
  [the cumulative note](/docs/developer/compute-pipes/notes/cumulative-files/)), each period keeps its
  latest session first, then the latest period's is kept: one session in all.

The registry is read when the folder is listed, by the sharding starter or by the reducing starter
(`AssignStageFileKeys`, which hands each node its files), so the nodes all read what the starter chose.
Every listing of a `file_key` channel goes through `ListStageChannel` (`latest_session.go`).

**What it does not do.** A period that was never mapped again still holds only its old session, so a
model change still needs the remap. And the skipped sessions stay in the Session Registry and on S3
until they are deleted or purged.

`ValidatePipeSpecConfig` refuses the field on a channel that is not of type `stage` with a `file_key`,
since no other read lists session folders.

## The column check: a file in another layout fails

A stage csv file's first line is the columns it was written with. The reader used to skip that line
unread. Now, for a `csv` channel of type `stage` with a `file_key` (main input or merge channel), each
file's header line is compared with its channel spec's columns, position by position
(`stage_columns_check.go`):

- **At the first column whose name differs, the file is refused**, and the error names the file, the
  column's position and both names.
- **A file that only lacks trailing columns** (a property appended to the class), **or carries extra
  trailing ones**, reads every shared column at its place. It is logged and read, as before.
- Not checked: a `headerless_csv` channel, a read of this session's own steps (`read_step_id`, written
  by the same run with the same specs), and a channel spec with `same_columns_as_input` or
  `has_dynamic_columns`.

**What to expect when it ships.** A pipeline whose `file_key` read still reaches files written before
a class change used to read them shifted, and now fails, naming the file. That failure is the defect
surfacing, not a new one: delete the old session or map it again. A property **renamed** in place, at
the same position, also fails the check, although its values were read at the right place.
