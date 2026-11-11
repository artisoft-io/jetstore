// Input File Preview: user/screens/input-file-preview. No image: the screen
// reads the file from the installation's bucket, and the stack has none.
//
// The spec follows the way to it, the Load Files flow's Preview File button.
// In this version that button leads to "Page not found": it puts the file key,
// folders and all, into the address unencoded, and the route takes one segment.
// When that is fixed this spec fails at the last line; then assert the heading
// "Input File Preview" instead, and update the page's "How you reach it".
import { expect, test } from "@playwright/test";
import { openFlowMenu, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

test("Input File Preview, from the Load Files flow", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Mapping");
  const sources = await waitForTable(page);
  await selectRow(sources, "CI_D1_hc:Eligibility");
  await press(page, "Load Data");
  const files = await waitForTable(page);
  await selectRow(files, "M021-M040_M007_M008_eligibility.csv");
  await press(page, "Preview File");
  await expect(page).toHaveURL(/\/filePreviewPath\/jetstore\/input\/client=CI\//);
  await expect(page.getByText("Page not found")).toBeVisible();
});
