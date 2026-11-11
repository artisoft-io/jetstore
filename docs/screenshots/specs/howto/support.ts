// Helpers shared by the tutorial and how-to specs, on top of lib/shots.ts.
//
// Two needs the shared helpers do not meet yet:
//
//  - Some values on a screen are written while the stack runs: the time the
//    configuration was loaded, the time a test or a runbook ran, how many
//    milliseconds it took. They differ at every run, so an image showing them
//    would be rewritten every time. `hideRunValues` and `hideColumn` hide that
//    text just before a shot, at no width, so that a value one digit longer
//    does not move the columns beside it; everything else stays as it is.
//  - A spec that changes data another page's image shows must undo it.
//    `submitDdl` does that through the Query Tool, as a user with the
//    workspace_ide capability would, and takes no image.
import { expect, type Locator, type Page } from "@playwright/test";
import { openScreen, settle, signIn } from "../../lib/shots";

const STYLE_ID = "shots-run-values";
const MARK = "data-shots-run-value";

/** The dates, in UTC, that a value written during this run carries. */
function runDates(): string[] {
  const now = Date.now();
  return [now, now - 86_400_000].map((t) => new Date(t).toISOString().slice(0, 10));
}

async function ensureStyle(page: Page): Promise<void> {
  await page.evaluate(
    ({ id, mark }) => {
      if (document.getElementById(id)) return;
      const style = document.createElement("style");
      style.id = id;
      style.textContent = `[${mark}] { color: transparent !important; font-size: 0 !important; }`;
      document.head.appendChild(style);
    },
    { id: STYLE_ID, mark: MARK },
  );
}

/**
 * Hides every piece of text that carries today's or yesterday's date (UTC),
 * and every element matching `selectors`. The fixtures' dates are all in 2025
 * and early 2026, so only what the stack wrote while it ran is hidden.
 */
export async function hideRunValues(page: Page, selectors: string[] = []): Promise<void> {
  await settle(page);
  await ensureStyle(page);
  await page.evaluate(
    ({ dates, mark, selectors }) => {
      const pattern = new RegExp(dates.join("|"));
      for (const el of Array.from(document.body.querySelectorAll("*"))) {
        if (el.children.length === 0 && pattern.test(el.textContent ?? "")) el.setAttribute(mark, "");
      }
      for (const selector of selectors) {
        for (const el of Array.from(document.querySelectorAll(selector))) el.setAttribute(mark, "");
      }
    },
    { dates: runDates(), mark: MARK, selectors },
  );
}

/**
 * Hides the cells of one column of `table`, found by its header's text: in
 * every row, or only in the rows whose text contains `rowText`.
 */
export async function hideColumn(table: Locator, header: string, rowText?: string): Promise<void> {
  await ensureStyle(table.page());
  await table.evaluate(
    (el, { header, mark, rowText }) => {
      const heads = Array.from(el.querySelectorAll("thead th"));
      const index = heads.findIndex((th) => (th.textContent ?? "").replace(/[▲▼]/g, "").trim() === header);
      if (index < 0) throw new Error(`no column ${header}`);
      for (const row of Array.from(el.querySelectorAll("tbody tr"))) {
        if (rowText && !(row.textContent ?? "").includes(rowText)) continue;
        row.children[index]?.setAttribute(mark, "");
      }
    },
    { header, mark: MARK, rowText },
  );
}

/** The row of `scope` holding `text`, selected by its checkbox. */
export async function selectRow(scope: Page | Locator, text: string): Promise<Locator> {
  const row = scope.getByRole("row").filter({ hasText: text });
  await expect(row).toHaveCount(1);
  await row.getByRole("checkbox").check();
  const page = "page" in scope ? scope.page() : scope;
  await settle(page);
  return row;
}

/**
 * Runs SQL with the Query Tool's Submit DDL, to undo what a spec did. Takes no
 * image; leaves the Query Tool open with the statement's result.
 */
export async function submitDdl(page: Page, sql: string): Promise<void> {
  await openScreen(page, "Query Tool");
  await page.getByRole("textbox", { name: "Query" }).fill(sql);
  await page.getByRole("button", { name: "Submit DDL" }).click();
  const result = page.locator("main table");
  await expect(result.locator("tbody")).not.toContainText("Submit a query above");
  await expect(result.locator("tbody")).not.toContainText(/error/i);
  await settle(page);
}

/**
 * Opens a file of the workspace in the Workspace IDE's editor, through the
 * file tree: `folders` are the folders to expand, under the tree's own groups,
 * and `file` is the file's name. Leaves the caret, and the view, at the start
 * of the file or at its end.
 */
export async function openWorkspaceFile(
  page: Page,
  folders: string[],
  file: string,
  at: "start" | "end" = "start",
): Promise<void> {
  await openScreen(page, "Workspace IDE");
  for (const folder of folders) {
    const expand = page.getByRole("button", { name: `Expand ${folder}`, exact: true });
    if (await expand.count()) await expand.click();
  }
  await page.getByRole("button", { name: new RegExp(`^${file.replace(/\./g, "\\.")} `) }).click();
  await expect(page.getByRole("tab", { name: new RegExp(file.replace(/\./g, "\\.")) })).toBeVisible();
  await page.locator(".cm-content").click();
  await page.keyboard.press(at === "end" ? "Control+End" : "Control+Home");
  await settle(page);
}

/**
 * In the open editor, changes the line that ends with `from` so that it ends
 * with `to`, by typing as a user would: the caret goes to the end of the line,
 * back over what the two have in common, and the differing part is retyped.
 */
export async function changeLine(page: Page, from: string, to: string): Promise<void> {
  const line = page.locator(".cm-line", { hasText: from });
  await expect(line).toHaveCount(1);
  let suffix = 0;
  while (suffix < from.length && suffix < to.length && from.at(-1 - suffix) === to.at(-1 - suffix)) suffix++;
  let prefix = 0;
  while (prefix < from.length - suffix && prefix < to.length - suffix && from[prefix] === to[prefix]) prefix++;
  await line.click();
  await page.keyboard.press("End");
  for (let i = 0; i < suffix; i++) await page.keyboard.press("ArrowLeft");
  for (let i = 0; i < from.length - suffix - prefix; i++) await page.keyboard.press("Backspace");
  await page.keyboard.type(to.slice(prefix, to.length - suffix));
  await expect(page.locator(".cm-line", { hasText: to })).toHaveCount(1);
}

/** Saves the open file and waits for the editor to say so. */
export async function saveFile(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByText("Saved", { exact: true })).toBeVisible();
  await dismissNotice(page);
}

/** Closes the notice under the top bar, if one is showing. */
export async function dismissNotice(page: Page): Promise<void> {
  const dismiss = page.locator(".banner").getByRole("button", { name: "Dismiss" });
  if (await dismiss.count()) await dismiss.click();
}

/**
 * Undoes what a spec changed, whether its steps passed or not: signs in again,
 * which also leaves whatever dialog a failed step left open, and runs `sql`
 * with Submit DDL, one statement after another. For a spec's `test.afterEach`.
 */
export async function undo(page: Page, statements: string[]): Promise<void> {
  await signIn(page);
  await submitDdl(page, statements.join(";\n"));
}

/**
 * The key a row the app creates during a run starts from: the fixtures set
 * every sequence of the tables these specs write to 100000, and keep their own
 * rows below it.
 */
export const FIRST_RUN_KEY = 100001;

/** SQL that sets a table's key sequence back to where the fixtures left it. */
export function resetKeySql(table: string): string {
  return `SELECT setval(pg_get_serial_sequence('jetsapi.${table}', 'key'), ${FIRST_RUN_KEY - 1})`;
}

/**
 * SQL that deletes the rules unit test runs this run of the specs started
 * (keys from FIRST_RUN_KEY), and puts the keys back. The fixtures' own runs
 * are kept.
 */
export function deleteTestRunsSql(): string[] {
  return [
    `DELETE FROM jetsapi.rules_unit_test_check WHERE execution_key >= ${FIRST_RUN_KEY} OR key >= ${FIRST_RUN_KEY}`,
    `DELETE FROM jetsapi.rules_unit_test_execution WHERE key >= ${FIRST_RUN_KEY}`,
    resetKeySql("rules_unit_test_check"),
    resetKeySql("rules_unit_test_execution"),
  ];
}

/**
 * SQL that drops a workspace file's saved change. Used after a spec has put
 * the file's content back itself; it cannot restore the file on disk.
 */
export function dropWorkspaceChangeSql(file: string): string {
  return `DELETE FROM jetsapi.workspace_changes WHERE workspace_name = 'jets_ws' AND file_name = '${file}'`;
}
