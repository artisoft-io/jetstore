---
title: Pipelines and sessions
description: Process configurations, pipeline configurations and runs, how a run starts, what its status means, and why the session is what JetStore traces and deletes by.
kind: explanation
sidebar:
  order: 6
---

A **pipeline** reads registered inputs, processes them in steps, and writes data tables and files.
Three things go into making one run, each owned by someone different, and every run writes under a
**session id** of its own.

## Process, pipeline configuration, run

| | What it says | Owned by | `CI` example |
|---|---|---|---|
| **Process configuration** | what the process does: its pipeline definition, the classes it reads and the tables it writes | the workspace | `Patient_Profile_Template` runs `pipes_config/patient_profile_template.pc.json`, reads `hc:Eligibility`, `hc:MedicalClaim` and `hc:PharmacyClaim`, and writes `cintel:Patient_Profile` |
| **Pipeline configuration** | which of a client's data the process reads, its rule settings, and whether it starts by itself | the client's configuration | for `CI`: the `hc:Eligibility` table as its main input, the two claims tables merged in over a six-period lookback, and it starts automatically |
| **Run** (pipeline execution) | one execution of a pipeline configuration, on particular inputs | nobody: it is a record | one row on Home's *Pipelines Status* tab, with its status and session id |

The process is defined once, in the workspace, by the Knowledge Engineer, in
`process_config/base__workspace_init_db.sql`. Each client then has its own pipeline configurations
for it, made in the *Pipeline Configuration* flow, and each pipeline configuration can run many times.

**A word on names.** *Pipeline configuration* means the client's row described above, which is what
the *Pipeline Configuration* flow edits. The `.pc.json` file in the workspace's `pipes_config/` is the
process's **pipeline definition**: the steps a run goes through. Developers call it the compute pipes
configuration, and its every field is in the
[pipeline configuration reference](/docs/developer/reference/generated/pipeline-configuration/).

## Inputs: main, merged and lookback

A pipeline configuration names a **main input**, and may name **merged inputs** read beside it. Each
input is a source of data: a client's file source, or a data table another pipeline wrote.

`CI`'s `Map_Eligibility` reads one input, the eligibility files of `D1`. `Patient_Profile_Template`
reads the mapped `hc:Eligibility` table as its main input, and merges the `hc:MedicalClaim` and
`hc:PharmacyClaim` tables into it. Its claims inputs have a **lookback** of six periods, so a run reads
the claims of earlier months as well as those of the month it runs for: a patient's profile is built
from a history of claims, not from one month's.

## How a run starts

- **By hand**, with *Start Pipeline* on Home's *Pipelines Status* tab, or the *Start Pipeline* flow:
  choose the pipeline configuration, then its main input from the input registry. This needs the
  `run_pipelines` capability.
- **By itself**, when its pipeline configuration is marked automated and all its inputs are
  registered for the same period. A registered file can start a pipeline, and so can the output of
  another run, which is how pipelines chain.

In `CI`, the three `Map_*` pipelines are started by hand. When they finish, their tables are registered
in the input registry, and `Patient_Profile_Template`, which is automated, starts when its inputs for
the period are all there.

An installation limits how many runs go at once. A run started beyond the limit waits as *pending*
and starts when others finish.

## What happens in a run

The pipeline definition says what each step does. Most runs **shard** their input: they split it
across several workers, by domain key, so that one member's records all go to the same worker. The
mapping pipelines read each row, apply the source mapping, and write the mapped table.
`Patient_Profile_Template` sorts and merges each member's eligibility and claims, hands each member's
records to the [rule engine](/docs/user/concepts/rule-engine/) as one unit, writes what the rules
conclude to `cintel:Patient_Profile`, and renders a text briefing on each patient from a template.

`Patient_Profile` is the same pipeline with one difference: a language model, rather than a
template, writes the briefing. It needs the installation's infer server running; the template arm
does not, which is why the examples use it.

A run can also end with **reports**: SQL scripts from the workspace's `reports/` folder, run against
what it wrote.

## What a run's status means

| Status | Meaning | Its output |
|---|---|---|
| `pending` | waiting for a free place under the installation's limit | none yet |
| `submitted` | started, and running | none yet |
| `completed` | finished without errors | registered |
| `errors` | finished, but the rules raised errors for some members; the *Process Errors* screen lists them | registered |
| `recovered` | a worker failed, and the run still went on to the end; check its output before relying on it | registered |
| `interrupted` | stopped part way | not registered: it is partial |
| `failed` | stopped by an error it could not recover from | not registered |
| `timed_out` | stayed *submitted* longer than the installation allows | not registered |

*Registered* means its tables were added to the input registry, so pipelines that read them can use
them, and automated ones start. A run's details show each step and shard, and its errors.

## Sessions

**Every run has a session id, and so does every registered file.** Every row a run writes to a data
table carries its session id, and so does every file it writes, and every record JetStore keeps about
the run. When a run finishes with usable output, its session is entered in the **session registry**,
on Home's *Session Registry* tab. A file's session is registered when the file is.

The session is how a result is traced: a profile row's session leads to its run, the run's inputs to
their sessions, and those to the files that arrived. It is also how a run is repeated: a pipeline
reads an input by its session, so the same session gives the same input.

## Deleting a session

**The session is the unit of deletion.** A session's rows are spread over many tables, the data
tables it wrote, the staging table, and JetStore's own records of the run, and deleting by session
removes all of them together, in one transaction, so nothing is left half deleted.

A deletion also takes the sessions **derived from** the one deleted: every run that read it as its
main input, and every run that read those, in turn. The *Delete* action on the *Session Registry* tab
lists them all before anything is removed. A session that a pending or submitted run is reading waits
until that run ends. Deleted runs leave their status rows on the *Archived Pipelines* tab, so the
record of what ran survives the data.

Deleting a session needs the `delete_session` capability. An installation may also delete sessions
older than its retention period on a schedule, by the same path.

## See also

- [Pipeline Configuration](/docs/user/screens/flows/pipeline-configuration/)
- [Start Pipeline](/docs/user/screens/flows/start-pipeline/)
- [Home](/docs/user/screens/home/)
- [Pipeline Execution Details](/docs/user/screens/pipeline-execution-details/)
- [Process Errors](/docs/user/screens/process-errors/)
- [Run a pipeline](/docs/user/how-to/run-a-pipeline/)
- [Delete a session](/docs/user/how-to/delete-a-session/)
