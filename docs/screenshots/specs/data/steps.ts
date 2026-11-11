// Steps the data and pipeline specs share: select a row of a table, press a
// button, and reach the screens that open from Home's Pipelines Status tab.
// Not a spec (the specs match *.spec.ts), so Playwright does not run it.
import { expect, type Locator, type Page } from "@playwright/test";
import { openScreen, settle, waitForTable } from "../../lib/shots";

/** Ticks the first row of `table` whose text contains every one of `texts`. */
export async function selectRow(table: Locator, ...texts: string[]): Promise<void> {
  let rows = table.locator("tbody tr");
  for (const text of texts) rows = rows.filter({ hasText: text });
  const row = rows.first();
  await row.locator("input[type=checkbox]").first().check();
  await expect(row).toHaveAttribute("aria-selected", "true");
  await settle(table.page());
}

/**
 * Scrolls a wide table to its last columns, which the window would otherwise
 * clip: what a user does to read them.
 */
export async function scrollToLastColumns(table: Locator): Promise<void> {
  await table.locator(".jets-datatable__scroll").evaluate((el) => {
    el.scrollLeft = el.scrollWidth;
  });
}

/** Presses a button by its label, exactly. */
export async function press(page: Page | Locator, label: string): Promise<void> {
  const button = page.getByRole("button", { name: label, exact: true });
  await expect(button).toBeEnabled();
  await button.click();
  await settle("page" in page ? page.page() : page);
}

/** The open dialog. */
export function dialog(page: Page): Locator {
  return page.locator("dialog[open]");
}

/**
 * Home's Pipelines Status tab, the run of Patient_Profile_Template in `status`
 * selected, then `button` pressed: how every run's detail screen is reached.
 */
export async function openRunScreen(page: Page, status: string, button: string): Promise<void> {
  await openScreen(page, "Home");
  const runs = await waitForTable(page);
  await selectRow(runs, "Patient_Profile_Template", status);
  await press(page, button);
}
