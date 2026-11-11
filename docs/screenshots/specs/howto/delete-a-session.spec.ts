// user/how-to/delete-a-session: delete a session from the Session Registry.
//
// The session is the spec's own: CI's eligibility file for December 2025,
// registered and never mapped, which the spec registers through the Query
// Tool before its steps, so that no other page's list ever shows it. The stack
// carries the deletion out at once. Afterwards the spec removes the deletion's
// own record, and anything a failed step left, so the stack ends as it began.
import { expect, test } from "@playwright/test";
import { openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { hideRunValues, selectRow, submitDdl, undo } from "./support";

const { shot } = screenshotsFor("user/how-to/delete-a-session");
const SESSION = "1764579600301";
const FILE_KEY =
  "jetstore/input/client=CI/org=D1/object_type=hc:Eligibility/year=2025/month=12/day=1/M021-M040_M007_M008_eligibility_december.csv";

test("Delete a session", async ({ page }) => {
  await signIn(page);

  // Not part of the page: register the file, as it was on 1 December 2025
  // (source period 102 of 00_base.sql).
  await submitDdl(
    page,
    [
      "INSERT INTO jetsapi.input_registry (key, client, org, object_type, file_key, source_period_key, table_name,",
      "  source_type, session_id, user_email, last_update)",
      `VALUES (30001, 'CI', 'D1', 'hc:Eligibility', '${FILE_KEY}',`,
      `  102, 'CI_D1_hc:Eligibility', 'file', '${SESSION}', 'system', '2025-12-01 09:00:30');`,
      "INSERT INTO jetsapi.session_registry (session_id, source_type, client, source_period_key, month_period,",
      "  week_period, day_period, last_update)",
      `SELECT '${SESSION}', 'file', 'CI', 102, month_period, week_period, day_period, '2025-12-01 09:00:30'`,
      "  FROM jetsapi.source_period WHERE key = 102",
    ].join("\n"),
  );

  await openScreen(page, "Home");
  await openTab(page, "Session Registry");
  const registry = await waitForTable(page);
  await selectRow(page, SESSION);
  await shot(page, "select-session");

  await page.getByRole("button", { name: "Delete", exact: true }).click();
  const confirm = page.getByRole("button", { name: "OK", exact: true });
  await expect(page.getByText(`${SESSION}: hc:Eligibility, client CI`)).toBeVisible();
  await shot(page, "confirm-deletion");
  await confirm.click();

  const row = registry.getByRole("row").filter({ hasText: SESSION });
  await expect(async () => {
    await page.getByRole("button", { name: "Refresh" }).click();
    await expect(row).toContainText("done", { timeout: 2_000 });
  }).toPass({ timeout: 30_000 });
  // Scrolled to the deletion columns from Process Name, not to the table's
  // end: the last column holds the time of this run's request, whose width
  // varies, and would move every column left of it.
  await registry.locator(".jets-datatable__scroll").evaluate((el) => {
    const th = Array.from(el.querySelectorAll("thead th")).find((t) => t.textContent?.includes("Process Name"));
    if (!th) throw new Error("no Process Name column");
    el.scrollLeft = (th as HTMLElement).offsetLeft;
  });
  await hideRunValues(page);
  await shot(page, "deletion-done");
});

// Not part of the page, and run whether the steps passed or not: the
// deletion's record, and the session's rows if it was not deleted.
test.afterEach(async ({ page }) => {
  await undo(page, [
    `DELETE FROM jetsapi.session_deletion WHERE session_id = '${SESSION}'`,
    `DELETE FROM jetsapi.input_registry WHERE session_id = '${SESSION}'`,
    `DELETE FROM jetsapi.session_registry WHERE session_id = '${SESSION}'`,
  ]);
});
