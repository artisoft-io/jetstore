---
title: Run rules unit tests
description: Start some of a workspace's rules unit tests from the Workspaces screen, and read each run's status, checks and log.
kind: how-to
sidebar:
  order: 7
---

**Before you start**: your account has `workspace_ide`, and the workspace has been compiled since its
test files last changed: the tests run the compiled rules, and the list of tests is the one the last
compile recorded. A test is a `.tst.json` file beside the rules it tests; the Developer's Guide
explains [how to write one](/docs/developer/testing/rules-unit-tests/writing-tests/).

1. Open **Workspaces**, tick the workspace, and press **Rules Unit Tests**. The flow opens on the
   workspace's test runs, newest first.
2. Press **+ Add**, and tick the tests to run. A test with findings, in the **Findings** column,
   cannot be started: fix its file and compile again.

   ![Select Rules Unit Tests: jets_ws's five test cases, with retraction_defect1.tst.json and salience_shared_consequent.tst.json ticked.](../../../../assets/screenshots/user/how-to/run-rules-unit-tests/select-tests.png)

3. Press **Start Test(s)**. Each test becomes a run in the list. Press **Refresh** until each has a
   status: `pass`, `fail` or `error`. Small tests take seconds.

   ![Rules Unit Test Runs: the two new runs, 100001 retraction defect 1 and 100002 salience, shared consequent, both pass, above three earlier runs, one of which failed.](../../../../assets/screenshots/user/how-to/run-rules-unit-tests/test-runs.png)

4. Tick a run and press **View Result**. Each row is one check of the test file, by session: a triple
   it expected or excluded, the outcome, and what was found. **Back** returns to the runs.

   ![The checks of the retraction defect 1 run: for the sessions control and withdrawn, each expected and excluded triple, all passing.](../../../../assets/screenshots/user/how-to/run-rules-unit-tests/test-result.png)

5. To see how the engine ran, tick a run and press **View Log**. The log ends with the test's
   verdict.

**How to tell it worked**: every run says `pass`. For a `fail`, **View Result** names the check that
failed; for an `error`, the **Reason** column and the log say why the test could not run. The
[Rules Unit Tests](/docs/user/screens/flows/rules-unit-tests/) page describes each step of the flow.
