// The Runbooks screen (user/screens/runbooks): jets_ws's runbooks, the start
// form of example_health_check, and one run of it, performed here up to the
// confirmation of its first ddl step, then aborted. Its times change at every
// run, so the images of the run are of the parts that do not show them.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/runbooks");

test("Runbooks", async ({ page }) => {
  test.setTimeout(240_000);
  await signIn(page);
  await openScreen(page, "Runbooks");
  await page.getByLabel("Workspace").selectOption("jets_ws");
  await expect(page.getByRole("button", { name: "Start example_health_check" })).toBeVisible();
  await settle(page);
  await shot(page, "runbooks");

  await page.getByRole("button", { name: "Start example_health_check" }).click();
  await expect(page.getByRole("heading", { name: "Start example_health_check" })).toBeVisible();
  await settle(page);
  await shot(page, "start-form");

  await page.getByRole("button", { name: "Start run" }).click();
  await expect(page.getByRole("heading", { name: /example_health_check · run/ })).toBeVisible();
  // The assert after the counts pauses the run: the seeded pending run is in flight.
  await expect(page.getByRole("button", { name: "Continue", exact: true })).toBeVisible({ timeout: 60_000 });
  await settle(page);
  await shot(page.locator(".screenbar"), "run-paused-controls");
  await expect(page.getByRole("listitem", { name: "Step nothing_in_flight" })).toContainText("A pipeline run has not finished");

  // Continue past the assert: the run goes on through the clock wait and the
  // settle loop, then stops at the manual step for a value.
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  const record = page.getByRole("region", { name: "Record record_size" });
  await expect(record).toBeVisible({ timeout: 120_000 });
  await settle(page);
  await shot(record, "step-manual");
  await record.getByRole("textbox").fill("3");
  await record.getByRole("button", { name: "Record" }).click();

  // The first ddl step waits for a person, showing what Accept would run.
  const confirm = page.getByRole("region", { name: "Confirm create_probe" });
  await expect(confirm).toBeVisible({ timeout: 60_000 });
  await settle(page);
  await shot(confirm, "step-ddl-confirmation");

  // Abort instead: nothing is created. A finished run can be released.
  await page.getByRole("button", { name: "Abort", exact: true }).click();
  await expect(page.getByRole("button", { name: "Release", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Release", exact: true }).click();
  const release = page.locator("dialog[open]");
  await expect(release).toBeVisible();
  await release.getByRole("button", { name: "OK" }).click();
  await expect(page.getByRole("button", { name: "Download .rbr.json" })).toBeVisible();
  await settle(page);
  await shot(page.locator(".screenbar"), "run-released-controls");
});
