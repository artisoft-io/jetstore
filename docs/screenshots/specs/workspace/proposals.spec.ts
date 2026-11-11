// The Proposals screen (user/screens/proposals): the list in its filters, one
// proposal with its decisions and transcript, and a decision recorded on it.
// The decision is recorded on a proposal of 20_workspace.sql that no other
// image shows; only the confirmation is captured, since the decision itself
// carries the time it was made.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/proposals");

test("Proposals", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Proposals");
  const filter = page.getByLabel("Which proposals");
  await expect(page.locator("table.proposal-table")).toBeVisible();
  await shot(page, "proposals-awaiting");

  await filter.selectOption("all");
  await expect(page.getByRole("link", { name: "prop_20003" })).toBeVisible();
  await settle(page);
  await shot(page, "proposals-all");

  // One proposal, opened from the list.
  await page.getByRole("link", { name: "prop_20001" }).click();
  await expect(page.getByRole("heading", { name: "prop_20001" })).toBeVisible();
  await expect(page.getByText("Chain verified")).toBeVisible();
  await settle(page);
  await shot(page, "proposal");
  // The run's transcript, further down.
  await page.locator("ol.transcript > li").last().scrollIntoViewIfNeeded();
  await settle(page);
  await shot(page, "proposal-transcript");
  await page.getByRole("heading", { name: "prop_20001" }).scrollIntoViewIfNeeded();

  // A decision: approve, with a rationale.
  await page.getByLabel("Rationale (optional, recorded with the decision)").fill("Default checked with the vendor.");
  await page.getByRole("button", { name: "Approved", exact: true }).click();
  const banner = page.locator(".banner-ok");
  await expect(banner).toContainText("Recorded Awaiting human approval → Approved");
  await shot(banner, "decision-recorded");
});
