// user/how-to/run-a-runbook: start jets_ws's example_health_check runbook and
// take it through an assert that pauses, a manual step, a ddl step and a pause
// step, to the end of the run.
//
// What the run leaves is removed after it, through the Query Tool: the
// scratch table its create_probe step made, and the run's own rows, so the
// Runbooks page shows no run.
import { expect, test, type Locator, type Page } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn } from "../../lib/shots";
import { FIRST_RUN_KEY, dismissNotice, hideRunValues, resetKeySql, undo } from "./support";

const { shot } = screenshotsFor("user/how-to/run-a-runbook");
const RUNBOOK = "example_health_check";

/** Hides what a run writes at every run: its times and durations. */
async function hideRunTimes(page: Page): Promise<void> {
  await hideRunValues(page, [".runbook-timing"]);
}

/** Waits until the run's step `stepId` shows `panel` (a Record or Confirm panel). */
async function waitForPanel(page: Page, label: string): Promise<Locator> {
  const panel = page.locator(`section[aria-label="${label}"]`);
  await expect(async () => {
    if (!(await panel.count())) await page.getByRole("button", { name: "Refresh" }).click();
    await expect(panel).toBeVisible({ timeout: 2_000 });
  }).toPass({ timeout: 60_000 });
  await settle(page);
  return panel;
}

test("Run a runbook", async ({ page }) => {
  await signIn(page);

  // Start it, with its parameters.
  await openScreen(page, "Runbooks");
  await shot(page, "runbooks");
  await page.getByRole("button", { name: `Start ${RUNBOOK}` }).click();
  const start = page.getByRole("region", { name: `Start ${RUNBOOK}` });
  await start.getByRole("textbox", { name: /^settle_seconds/ }).fill("0");
  await shot(page, "start-runbook");
  await start.getByRole("button", { name: "Start run" }).click();

  // An assert that fails and pauses the run: a run of 00_base.sql is pending.
  await expect(page.getByText("Last error")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(/nothing_in_flight: A pipeline run has not finished/)).toBeVisible();
  await settle(page);
  await hideRunTimes(page);
  await shot(page, "paused-on-assert");
  await page.getByRole("button", { name: "Continue" }).click();

  // A manual step: record a value.
  const record = await waitForPanel(page, "Record record_size");
  await record.getByRole("textbox", { name: /^probe_rows/ }).fill("3");
  await shot(record, "record-value");
  await record.getByRole("button", { name: "Record" }).click();

  // A ddl step: read the statement, then accept it.
  const confirm = await waitForPanel(page, "Confirm create_probe");
  await shot(confirm, "confirm-ddl");
  await confirm.getByRole("button", { name: "Accept" }).click();

  // The pause step read_results: read, then Continue in the bar.
  const pause = await waitForPanel(page, "Pause read_results");
  await shot(pause, "pause-step");
  await dismissNotice(page);
  await page.getByRole("button", { name: "Continue", exact: true }).click();

  // The ddl step drop_probe, then the run ends.
  const drop = await waitForPanel(page, "Confirm drop_probe");
  await drop.getByRole("button", { name: "Accept" }).click();
  await expect(page.locator(".pill").filter({ hasText: /Completed/ }).first()).toBeVisible({ timeout: 60_000 });
  await settle(page);
  await hideRunTimes(page);
  await shot(page, "run-completed");
});

// Not part of the page, and run whether the steps passed or not: remove what
// the run left, the scratch table its create_probe step made and the run's
// rows, and put the keys back, so the Runbooks page shows only the fixtures'.
test.afterEach(async ({ page }) => {
  await undo(page, [
    "DROP TABLE IF EXISTS public.runbook_example_probe",
    `DELETE FROM jetsapi.runbook_step_run WHERE key >= ${FIRST_RUN_KEY} OR run_key >= ${FIRST_RUN_KEY}`,
    `DELETE FROM jetsapi.runbook_run WHERE key >= ${FIRST_RUN_KEY}`,
    resetKeySql("runbook_step_run"),
    resetKeySql("runbook_run"),
  ]);
});
