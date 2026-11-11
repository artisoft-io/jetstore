// Helpers for the screenshot specs: sign in, reach a screen through the app's
// own links, wait for a table to settle, and write a screenshot beside the
// documentation page that shows it.
//
// An image is written only when it differs from the one already committed by
// more than a small threshold, so a run with no change to the UI writes no file.
import { existsSync, mkdirSync, readFileSync, writeFileSync, appendFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, type Locator, type Page } from "@playwright/test";
import pixelmatch from "pixelmatch";
import { PNG } from "pngjs";

const here = dirname(fileURLToPath(import.meta.url));
/** docs/ in this checkout. */
export const DOCS_DIR = resolve(here, "../..");
const CONTENT_DIR = join(DOCS_DIR, "src/content/docs");
const ASSETS_DIR = join(DOCS_DIR, "src/assets/screenshots");
/** One line per image, read by run.mjs for its summary. */
export const RESULTS_FILE = resolve(here, "../test-results/shots.jsonl");

/** The stack's admin. Its password is generated for each run and is only ever in the environment. */
export const ADMIN_EMAIL = "admin@localhost.test";

/**
 * The share of pixels that may differ before an image is rewritten. Small enough
 * to catch a changed label, large enough to ignore anti-aliasing noise.
 */
const MAX_DIFF_RATIO = Number(process.env.SHOTS_MAX_DIFF_RATIO ?? "0.0005");
/** Per-pixel colour tolerance, pixelmatch's `threshold` (0 to 1). */
const PIXEL_THRESHOLD = 0.1;

/** Signs in as the stack's admin and waits for the top bar. */
export async function signIn(page: Page): Promise<void> {
  const password = process.env.JETS_ADMIN_PWD;
  if (!password) throw new Error("JETS_ADMIN_PWD is not set: run the specs with `npm run shots`");
  // The light theme, whatever a previous session stored.
  await page.addInitScript(() => {
    try {
      localStorage.setItem("jetstore-ide-theme", "light");
    } catch {
      /* storage unavailable: the default theme is light */
    }
  });
  await page.goto("/");
  await page.locator("#email").fill(ADMIN_EMAIL);
  await page.locator("#password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("navigation", { name: "Screens" })).toBeVisible();
}

/**
 * Opens a screen from the top bar, by its label (Home, Workspaces, Query Tool…).
 *
 * Always reach a screen through the app's links: the session lives in memory,
 * so loading an address directly signs the user out.
 */
export async function openScreen(page: Page, label: string): Promise<void> {
  await page.getByRole("navigation", { name: "Screens" }).getByRole("link", { name: label, exact: true }).click();
  await settle(page);
}

/** Opens an entry of the brand menu (the configuration flows, Rules Configuration, User Administration). */
export async function openFlowMenu(page: Page, label: string): Promise<void> {
  await page.getByLabel("Open a configuration flow").click();
  await page.getByRole("navigation", { name: "Configuration flows" }).getByRole("link", { name: label, exact: true }).click();
  await settle(page);
}

/** Selects a tab by its label and waits for its content. */
export async function openTab(page: Page, label: string): Promise<void> {
  await page.getByRole("tab", { name: label, exact: true }).click();
  await expect(page.getByRole("tab", { name: label, exact: true })).toHaveAttribute("aria-selected", "true");
  await settle(page);
}

/**
 * Waits until the page has stopped changing: no request in flight, no table
 * still saying "Loading…", and the same content twice in a row.
 */
export async function settle(page: Page, scope?: Locator): Promise<void> {
  const root = scope ?? page.locator("body");
  await page.waitForLoadState("networkidle");
  await expect(root.getByText("Loading…", { exact: true })).toHaveCount(0);
  let previous = "";
  for (let i = 0; i < 20; i++) {
    const current = await root.innerHTML();
    if (current === previous) return;
    previous = current;
    await page.waitForTimeout(250);
  }
}

/** Waits for a data table to have its rows, or to say it has none. */
export async function waitForTable(page: Page, table?: Locator): Promise<Locator> {
  const t = table ?? page.locator(".jets-datatable").first();
  await expect(t).toBeVisible();
  await expect(t.locator("tbody tr").first()).toBeVisible();
  await settle(page, t);
  return t;
}

export interface ShotOptions {
  /** Regions to paint over, for content that changes at every run. */
  mask?: Locator[];
}

export type ShotOutcome = "written" | "unchanged" | "new";

/**
 * The screenshots of one documentation page.
 *
 * `pagePath` is the page's path under docs/src/content/docs, without the
 * extension: `user/screens/home`. Its images go to
 * docs/src/assets/screenshots/<pagePath>/<name>.png. The page shows one with a
 * path relative to its own file: from user/screens/home.md,
 * `![What it shows](../../../../assets/screenshots/user/screens/home/tabs.png)`.
 */
export function screenshotsFor(pagePath: string) {
  if (!/^[a-z0-9-]+(\/[a-z0-9-]+)*$/.test(pagePath)) {
    throw new Error(`page path ${JSON.stringify(pagePath)}: use the page's path under docs/src/content/docs, in lower case`);
  }
  const pageFile = [join(CONTENT_DIR, `${pagePath}.md`), join(CONTENT_DIR, pagePath, "index.md"), join(CONTENT_DIR, `${pagePath}.mdx`)];
  if (!pageFile.some(existsSync)) {
    console.warn(`[shots] no page at docs/src/content/docs/${pagePath}: its images are written anyway`);
  }
  const dir = join(ASSETS_DIR, pagePath);

  /**
   * Takes a screenshot of the page, or of one element when `target` is a
   * locator, and writes it as <name>.png unless it matches the committed image.
   */
  async function shot(target: Page | Locator, name: string, options: ShotOptions = {}): Promise<ShotOutcome> {
    if (!/^[a-z0-9][a-z0-9-]*$/.test(name)) {
      throw new Error(`image name ${JSON.stringify(name)}: use lower case letters, digits and hyphens`);
    }
    const page = "page" in target ? target.page() : target;
    await settle(page);
    const png = await target.screenshot({
      animations: "disabled",
      caret: "hide",
      mask: options.mask,
      scale: "css",
    });
    const file = join(dir, `${name}.png`);
    const outcome = compareAndWrite(file, png);
    mkdirSync(dirname(RESULTS_FILE), { recursive: true });
    appendFileSync(RESULTS_FILE, JSON.stringify({ file: relative(DOCS_DIR, file), outcome }) + "\n");
    return outcome;
  }

  return { shot, dir };
}

function compareAndWrite(file: string, png: Buffer): ShotOutcome {
  if (existsSync(file)) {
    const before = PNG.sync.read(readFileSync(file));
    const after = PNG.sync.read(png);
    if (before.width === after.width && before.height === after.height) {
      const differing = pixelmatch(before.data, after.data, null, after.width, after.height, {
        threshold: PIXEL_THRESHOLD,
      });
      if (differing / (after.width * after.height) <= MAX_DIFF_RATIO) return "unchanged";
    }
    writeFileSync(file, png);
    return "written";
  }
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, png);
  return "new";
}
