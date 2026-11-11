---
title: Read a run's process errors
description: Find the errors the rules raised in a run that ended with errors, read what each says, and look at the records behind one.
kind: how-to
sidebar:
  order: 4
---

**Before you start**: your account has `jetstore_read`. A run that finished but whose rules raised
errors for some members ends with the status `errors`; its output is registered all the same.

1. On **Home**'s **Pipelines Status** tab, tick the run with status `errors`.

   ![Pipelines Status with the Patient_Profile_Template run of 7 January 2026, status errors, ticked.](../../../../assets/screenshots/user/how-to/read-process-errors/run-with-errors.png)

2. Press **View Process Errors**. Each row is one error: the member it concerns (the domain key),
   the input column, and the message.

   ![Pipeline Execution Errors for the run: rows for member M007 and others, each on hc:Seconday_Diagnosis_Code1, saying DiagnosisDescriptionLookup has no row for key M5450 or M5451, with the claim.](../../../../assets/screenshots/user/how-to/read-process-errors/process-errors.png)

3. Read the messages for a pattern. Here every error is the same: two diagnosis codes, `M5450` and
   `M5451`, are not in the workspace's diagnosis lookup, so the rules could not describe them. The fix
   is in the workspace, not in the run.
4. To see the data behind one error, tick it and press **View Input Records**. The dialog lists the
   member's records in each table the run read.

**What to do next**: fix the cause, here the lookup, in the workspace (see
[Work on a workspace](/docs/user/how-to/work-on-a-workspace/)), compile, and run the pipeline again
(see [Run a pipeline](/docs/user/how-to/run-a-pipeline/)). A run that `failed` has no process errors
but a failure; **View Failure Details** shows it. The [Process Errors](/docs/user/screens/process-errors/)
page describes every column.
