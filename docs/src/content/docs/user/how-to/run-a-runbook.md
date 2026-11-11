---
title: Run a runbook
description: Start one of the active workspace's runbooks with its parameters, and take it to the end, answering the steps that wait for you.
kind: how-to
sidebar:
  order: 8
---

**Before you start**: your account has `runbook_execute`, and the capability each step needs: a query
needs `jetstore_read`, a pipeline start `run_pipelines`. A runbook is a list of steps in the active
workspace's `runbooks/` folder, which the installation runs for you one at a time; the Developer's
Guide describes [the runbook format](/docs/developer/engine/runbook-format/). The example is
`jets_ws`'s `example_health_check`.

1. Open **Runbooks** from the top bar. Each runbook shows whether it is **valid** and its last run.
   Only a valid runbook can be started, and only one run of a runbook at a time.

   ![The Runbooks screen: jets_ws's example_health_check and patient_profile_template_test, both valid and never run, each with a Start button.](../../../../assets/screenshots/user/how-to/run-a-runbook/runbooks.png)

2. Press **Start** on the runbook. Check its parameters, each with its type and meaning; change any
   that should differ, here `settle_seconds` to `0`. Press **Start run**.

   ![The start form of example_health_check: in_flight_statuses pending and submitted, settle_seconds 0, and since_utc 2026-01-01T00:00:00Z, with Start run and Cancel.](../../../../assets/screenshots/user/how-to/run-a-runbook/start-runbook.png)

3. The run screen opens and the installation works through the steps, each listed with its status and
   outputs. When it stops, the run is **Paused**; read why under **Last error** and in the step
   itself. Here the assert `nothing_in_flight` found a pipeline run still pending.

   ![The run screen of example_health_check, run 100001, paused: Last error says nothing_in_flight, a pipeline run has not finished; the first step, counts, completed with its outputs.](../../../../assets/screenshots/user/how-to/run-a-runbook/paused-on-assert.png)

   Press **Continue** if the condition is expected, **Skip** to pass over the step, **Retry** to run a
   failed step again, or **Abort** to end the run. The bar offers only what the step can take.

4. A **manual** step waits for you to record values. Fill them in and press **Record**.

   ![The record_size step's panel: record how many rows the scratch table should hold, from 1 to 10, with probe_rows filled in as 3 and the Record button.](../../../../assets/screenshots/user/how-to/run-a-runbook/record-value.png)

5. A **ddl** step, which changes the database, always waits for you, and shows exactly what it will
   run: the statement, the values bound into it, its file's hash, and a preview run and rolled back.
   Read it, optionally add a comment, and press **Accept** to run it, or **Deny** to leave the run
   paused.

   ![The confirmation of the create_probe step: the CREATE TABLE and INSERT statement, the bound parameter probe_rows 3, its preview, a comment box, and the Accept and Deny buttons.](../../../../assets/screenshots/user/how-to/run-a-runbook/confirm-ddl.png)

6. A **pause** step stops for you to read what the run has found so far. Its panel shows the
   runbook's message. Read the outputs, then press **Continue** in the bar.

   ![The read_results pause step's panel: the run waits for you, with the runbook's message to read the counts, the clock and the scratch table's row count, then continue to drop the scratch table.](../../../../assets/screenshots/user/how-to/run-a-runbook/pause-step.png)

7. When the run ends, press **Release** to release its result; **Download .rbr.json** then saves it
   as a file.

   ![The run screen of example_health_check, completed: every step completed or skipped, with the Release button.](../../../../assets/screenshots/user/how-to/run-a-runbook/run-completed.png)

**How to tell it worked**: the run's status is **Completed**, and every step's status is completed or
skipped. On the **Runbooks** screen, the runbook's last run shows the same.

**Editing a runbook during a run**: the *Edit future steps* link opens the runbook's file in the
Workspace IDE. Steps already run cannot change; **Continue** reloads the runbook for the rest. The
[Runbooks](/docs/user/screens/runbooks/) page describes the list and the run screen.
