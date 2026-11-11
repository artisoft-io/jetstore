// Query Tool: user/screens/query-tool.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press } from "./steps";

const { shot } = screenshotsFor("user/screens/query-tool");

const QUERY = `SELECT "hc:Member_ID", "hc:Member_First_Name", "hc:Member_Gender", "cintel:Model_Name", session_id
  FROM "cintel:Patient_Profile"
 ORDER BY "hc:Member_ID"`;

test("Query Tool: a query and its rows, then the same statement as DDL", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Query Tool");
  await expect(page.getByText("Submit a query above to see rows.")).toBeVisible();

  await page.getByLabel("Query").fill(QUERY);
  await press(page, "Submit Query");
  const result = await waitForTable(page);
  await expect(result.locator("tbody tr")).toHaveCount(22);
  await shot(page, "query-result");

  // Submit DDL runs the statement and shows only what the database said.
  await press(page, "Submit DDL");
  await expect(result.getByText("SELECT 22")).toBeVisible();
  await shot(result, "ddl-result");
});
