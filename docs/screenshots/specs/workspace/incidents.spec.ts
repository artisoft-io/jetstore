// The Incidents screen (user/screens/incidents): the open incidents, the
// adjudicated ones, and one incident with its hypotheses, transitions and the
// verdict controls. No verdict is recorded.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/incidents");

test("Incidents", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Incidents");
  await expect(page.getByRole("link", { name: "inc_20001" })).toBeVisible();
  await settle(page);
  await shot(page, "incidents-open");

  await page.getByLabel("Which incidents").selectOption("adjudicated");
  await expect(page.getByRole("link", { name: "inc_20003" })).toBeVisible();
  await settle(page);
  await shot(page, "incidents-adjudicated");

  await page.getByLabel("Which incidents").selectOption("open");
  await page.getByRole("link", { name: "inc_20001" }).click();
  await expect(page.getByRole("heading", { name: "inc_20001" })).toBeVisible();
  await settle(page);
  await shot(page, "incident");

  // Further down: the transitions and the verdict controls.
  await page.getByRole("heading", { name: "Your verdict" }).scrollIntoViewIfNeeded();
  await settle(page);
  await shot(page, "incident-verdict");
});
