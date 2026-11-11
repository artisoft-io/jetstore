// user/how-to/query-the-data: Submit Query and Submit DDL in the Query Tool.
// The table the page creates, it drops at the end.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { undo } from "./support";

const { shot } = screenshotsFor("user/how-to/query-the-data");

test("Query the data", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Query Tool");
  const query = page.getByRole("textbox", { name: "Query" });
  const result = page.locator("main .jets-datatable").first();

  // A query: claim lines per member.
  await query.fill(
    [
      'SELECT "hc:Member_ID", count(*) AS claim_lines,',
      '       min("hc:Service_Date_From") AS first_service, max("hc:Service_Date_From") AS last_service',
      'FROM "hc:MedicalClaim"',
      "WHERE session_id = '1767604560202'",
      'GROUP BY "hc:Member_ID"',
      'ORDER BY "hc:Member_ID"',
    ].join("\n"),
  );
  await page.getByRole("button", { name: "Submit Query" }).click();
  await waitForTable(page, result);
  await expect(result.locator("tbody")).toContainText("M007");
  await shot(page, "submit-query");

  // A statement: keep that result as a table of its own.
  await query.fill(
    [
      "CREATE TABLE public.ci_claim_lines_per_member AS",
      'SELECT "hc:Member_ID" AS member_id, count(*) AS claim_lines',
      'FROM "hc:MedicalClaim"',
      "WHERE session_id = '1767604560202'",
      'GROUP BY "hc:Member_ID"',
    ].join("\n"),
  );
  await page.getByRole("button", { name: "Submit DDL" }).click();
  await expect(result.locator("tbody")).toContainText("SELECT 22");
  await shot(page, "submit-ddl");

  // Drop it when done.
  await query.fill("DROP TABLE public.ci_claim_lines_per_member");
  await page.getByRole("button", { name: "Submit DDL" }).click();
  await expect(result.locator("tbody")).toContainText("DROP TABLE");
});

// Not part of the page: the table, if a failed step left it.
test.afterEach(async ({ page }) => {
  await undo(page, ["DROP TABLE IF EXISTS public.ci_claim_lines_per_member"]);
});
