/**
 * The Workspace IDE's *Workspace guide* button: which page of the workspace's
 * guide it opens, and whether it is shown at all.
 *
 * An installation's documentation site carries a fourth guide, generated from the
 * compiled workspace at the UI image's build and merged with JetStore's three. The
 * generator writes a manifest beside it, at `/docs/workspace/manifest.json`:
 *
 * ```json
 * {
 *   "workspace": "jets_ws",
 *   "index": "/docs/workspace/",
 *   "pages": {
 *     "data_model/jets_model.jr": "/docs/workspace/reference/domain/jets_model/",
 *     "docs/customisation/observe.md": "/docs/workspace/customisation/observe/"
 *   }
 * }
 * ```
 *
 * `pages` is keyed by the file's path in the workspace, with forward slashes, as
 * the compiler names its source files. The manifest names files and pages only.
 *
 * **The button is shown only when the manifest exists and names the workspace the
 * IDE has open.** An image built without a guide answers 404, and a guide built
 * for one workspace says nothing about another the IDE can open, so in both cases
 * there is no button rather than one that opens the wrong thing.
 *
 * **Opening it follows *Help* exactly** (`openHelp`, `helpTopics.ts`): the tab is
 * opened blank before anything is awaited, so a pop-up blocker allows it; the
 * documentation session sets the cookie the site requires; and the tab is the
 * same named tab *Help* uses, so the two share one documentation tab.
 */

import { useEffect, useState } from "react";
import { z } from "zod";

import type { ApiClient } from "../api/client";
import { decodeLabel } from "../api/workspace";
import { DOCS_PREFIX, DOCS_TAB_NAME, type HelpTab } from "./helpTopics";

/** Where the generator writes the manifest, on the app's own origin. */
export const WORKSPACE_MANIFEST_PATH = "/docs/workspace/manifest.json";

/** The button's label, exactly. */
export const WORKSPACE_GUIDE_LABEL = "Workspace guide";

/**
 * A page of the documentation site: an absolute path under `/docs/`, never
 * another origin (`//host/...`), so a manifest cannot send the tab elsewhere.
 */
function isDocsPage(value: string): boolean {
  return value.startsWith(DOCS_PREFIX) && !value.startsWith("//");
}

/**
 * The manifest as the generator writes it. Unknown fields are allowed, so the
 * generator may add one without breaking the IDE.
 */
const WorkspaceManifestSchema = z.object({
  workspace: z.string().min(1),
  index: z.string().refine(isDocsPage, { message: "index must be a page under /docs/" }),
  pages: z.record(z.string(), z.string()),
});

export type WorkspaceManifest = z.infer<typeof WorkspaceManifestSchema>;

/** The manifest from a parsed body, or `null` when the body is not one. */
export function parseWorkspaceManifest(body: unknown): WorkspaceManifest | null {
  const parsed = WorkspaceManifestSchema.safeParse(body);
  return parsed.success ? parsed.data : null;
}

/**
 * Reads the manifest. `null` means *no guide*: a 404 (an image built without
 * one), any other refusal, a body that is not JSON or not a manifest, or no
 * answer at all. The site is authorised by the cookie the documentation
 * session sets, so call this after `openDocsSession`.
 *
 * `fetchImpl` is injectable for the tests; the default looks `fetch` up at call
 * time, so a test that stubs the global reaches it too.
 */
export async function readWorkspaceManifest(
  fetchImpl: typeof fetch = (input, init) => globalThis.fetch(input, init),
): Promise<WorkspaceManifest | null> {
  try {
    const res = await fetchImpl(WORKSPACE_MANIFEST_PATH, {
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    if (!res.ok) return null;
    return parseWorkspaceManifest(await res.json());
  } catch {
    return null;
  }
}

/** Whether `manifest` is a guide to `workspace`, the workspace the IDE has open. */
export function describesWorkspace(manifest: WorkspaceManifest | null, workspace: string): boolean {
  return manifest !== null && workspace !== "" && manifest.workspace === workspace;
}

/**
 * The workspace-relative path of a file the IDE has open, in the manifest's form.
 *
 * The IDE keys a file tab by the server's `file_name`, which is the path in the
 * workspace (`jet_rules/process/main.jr`) escaped by Go's `url.QueryEscape`
 * (`wsfile/visitor.go`), so `/` arrives as `%2F` and a space as `+`. Decoding
 * it gives the compiler's name for the file. Backslashes and a leading `./` or
 * `/` are normalised away, in case a path ever arrives in another form.
 */
export function workspacePathOf(fileName: string): string {
  return decodeLabel(fileName)
    .replace(/\\/g, "/")
    .replace(/^(\.\/|\/)+/, "");
}

/**
 * The page to open for the file open in the editor (its escaped `file_name`, or
 * `null` when no file is open): the file's own page when the manifest has one,
 * else the guide's landing page.
 */
export function guidePageFor(manifest: WorkspaceManifest, fileName: string | null): string {
  if (fileName === null) return manifest.index;
  const page = manifest.pages[workspacePathOf(fileName)];
  return page !== undefined && isDocsPage(page) ? page : manifest.index;
}

/** What `openWorkspaceGuide` needs. */
export interface OpenWorkspaceGuideOptions {
  /** The workspace the IDE has open. */
  workspace: string;
  /** The open file's escaped `file_name`, or `null` when no file tab is active. */
  fileName: string | null;
  openDocsSession: () => Promise<void>;
  readManifest?: () => Promise<WorkspaceManifest | null>;
  openTab?: () => HelpTab | null;
}

/**
 * Opens the workspace guide's page for the open file in the documentation tab.
 * Resolves to an error message for the banner, or `null` when the tab was
 * pointed at the page.
 *
 * The tab handling is `openHelp`'s, for the same reasons (`helpTopics.ts`): the
 * tab is opened before anything is awaited, `opener` is cleared before it leaves
 * this origin's blank page, and a failure closes the tab only when this click
 * opened it.
 */
export async function openWorkspaceGuide({
  workspace,
  fileName,
  openDocsSession,
  readManifest = () => readWorkspaceManifest(),
  openTab = () => window.open("", DOCS_TAB_NAME),
}: OpenWorkspaceGuideOptions): Promise<string | null> {
  const tab = openTab();
  if (tab === null) {
    return `The browser blocked the documentation tab. Allow pop-ups for this site and press ${WORKSPACE_GUIDE_LABEL} again.`;
  }
  let opened = false;
  try {
    opened = tab.location.href === "about:blank";
  } catch {
    // A tab whose page this origin cannot read is not one this click opened.
  }
  const fail = (message: string): string => {
    if (opened) tab.close();
    return message;
  };
  try {
    await openDocsSession();
  } catch (err) {
    return fail(
      err instanceof Error && err.message !== "" ? err.message : "The documentation could not be opened.",
    );
  }
  const manifest = await readManifest();
  if (!describesWorkspace(manifest, workspace)) {
    return fail(`The documentation has no guide for ${workspace}.`);
  }
  tab.opener = null;
  tab.location.href = guidePageFor(manifest!, fileName);
  tab.focus?.();
  return null;
}

/**
 * The manifest of the installation's workspace guide, read once the IDE knows
 * its workspace, or `null` while it is being read and when there is none.
 *
 * **Read once per mount, not per workspace.** The manifest belongs to the image,
 * not to the workspace picked: changing workspace only changes whether it
 * describes the one open (`describesWorkspace`), which the caller decides on
 * each render. It needs the documentation cookie, so the session is opened
 * first. Any failure means no button and no banner: a deployment without a
 * guide is not an error.
 */
export function useWorkspaceManifest(
  api: ApiClient,
  workspace: string,
  readManifest: () => Promise<WorkspaceManifest | null> = () => readWorkspaceManifest(),
): WorkspaceManifest | null {
  const [manifest, setManifest] = useState<WorkspaceManifest | null>(null);
  const known = workspace !== "";
  useEffect(() => {
    if (!known) return;
    let cancelled = false;
    void (async () => {
      try {
        await api.openDocsSession();
      } catch {
        return;
      }
      const found = await readManifest();
      if (!cancelled) setManifest(found);
    })();
    return () => {
      cancelled = true;
    };
    // Once the workspace is known, and not again on a change of workspace.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api, known]);
  return manifest;
}
