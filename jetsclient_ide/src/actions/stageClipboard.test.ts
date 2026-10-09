/**
 * `fetchStageToClipboard` — jetstore_maintenance_02, `D04`, task `AE.3`.
 *
 * **The fetch is the real `ApiClient` over a stubbed `fetch`**, not a stubbed
 * host method, because the claim worth testing is the one the plan makes about
 * the route: the file comes back through the API client's `dataTable`, in the
 * request shape `fetch_file_from_stage` reads (`jets/apiserver/api_tables.go`,
 * `Data[0]["stage_file_path"]`), and a grammar `post` was never involved. Only
 * the clipboard and the fallback dialog are stubs, because they are the browser.
 */

import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient, SessionExpiredError } from "../api/client";
import { FormState } from "../datatable/formState";
import type { EscapeHost } from "./escapes";
import { productionRegistry } from "./registry";
import {
  STAGE_CLIPBOARD_KEYS,
  applyReplacement,
  browserWriteClipboard,
  fetchStageFileThrough,
  fetchStageToClipboard,
  substituteStagePath,
} from "./stageClipboard";

const GROUP = 0;
const MANIFEST_PATH = "process_name={{process_name}}/session_id={{session_id}}/run_manifest.json";

/**
 * The message the handler sends for a missing object today: its own prefix,
 * `DownloadBufFromS3`'s, and the SDK's `OperationError` around `*types.NoSuchKey`.
 * Composed from the three format strings, not captured from a live bucket.
 */
const NO_SUCH_KEY =
  "error: failed to fetch file from stage: failed to download s3 file " +
  "'s3://bucket/jetstore/stage/process_name=p1/session_id=s1/run_manifest.json': " +
  "operation error S3: GetObject, https response error StatusCode: 404, RequestID: R, " +
  "HostID: H, api error NoSuchKey: The specified key does not exist.";

interface Answer {
  status: number;
  body: unknown;
}

/** A signed-in client whose `/dataTable` answers from a queue, and the bodies it was sent. */
async function signedIn(answers: Answer[]) {
  const sent: Record<string, unknown>[] = [];
  const impl = (async (url: RequestInfo | URL, init?: RequestInit) => {
    if (String(url).endsWith("/login")) {
      return new Response(JSON.stringify({ token: "t0", name: "n", user_email: "a@b.c" }), { status: 200 });
    }
    sent.push(JSON.parse(String(init?.body)) as Record<string, unknown>);
    const next = answers.shift() ?? { status: 500, body: { error: "no answer queued" } };
    return new Response(JSON.stringify(next.body), { status: next.status });
  }) as typeof fetch;
  const api = new ApiClient("", impl);
  await api.login("a@b.c", "pw");
  return { api, sent };
}

/** The selection `pipelineExecStatusTable`'s binding publishes: every secondary key a list. */
function selected(): FormState {
  const formState = new FormState();
  formState.setValue(GROUP, "process_name", ["p1"]);
  formState.setValue(GROUP, "session_id", ["s1"]);
  return formState;
}

function withParameters(formState: FormState, parameters: Partial<Record<keyof typeof STAGE_CLIPBOARD_KEYS, string>>) {
  for (const [name, value] of Object.entries(parameters)) {
    formState.setValue(GROUP, STAGE_CLIPBOARD_KEYS[name as keyof typeof STAGE_CLIPBOARD_KEYS], value);
  }
  return formState;
}

interface Recorded {
  notes: Array<[string, string]>;
  copied: string[];
  shown: Array<[string, string]>;
}

function hostFor(api: ApiClient, clipboard: "accepts" | "refuses" | "absent" = "accepts"): EscapeHost & Recorded {
  const recorded: Recorded = { notes: [], copied: [], shown: [] };
  return {
    ...recorded,
    post: async () => ({ statusCode: 200 }),
    read: async () => [],
    notify: (level, message) => recorded.notes.push([level, message]),
    setBusy: () => {},
    close: () => {},
    userEmail: () => "a@b.c",
    download: () => {},
    fetchStageFile: fetchStageFileThrough(api),
    ...(clipboard === "absent"
      ? {}
      : {
          writeClipboard: async (text: string) => {
            if (clipboard === "refuses") throw new DOMException("Document is not focused.", "NotAllowedError");
            recorded.copied.push(text);
          },
        }),
    showText: (title, text) => recorded.shown.push([title, text]),
  };
}

const run = (formState: FormState, host: EscapeHost) =>
  fetchStageToClipboard({ formState, group: GROUP, flowKey: "homeScreen" }, host);

describe("substituteStagePath", () => {
  it("unwraps the one-element lists a table binding publishes", () => {
    expect(substituteStagePath(MANIFEST_PATH, selected(), GROUP)).toEqual({
      ok: true,
      path: "process_name=p1/session_id=s1/run_manifest.json",
    });
  });

  it("takes a scalar as it is", () => {
    const formState = new FormState();
    formState.setValue(GROUP, "process_name", "p2");
    formState.setValue(GROUP, "session_id", "s2");
    expect(substituteStagePath(MANIFEST_PATH, formState, GROUP)).toEqual({
      ok: true,
      path: "process_name=p2/session_id=s2/run_manifest.json",
    });
  });

  it("refuses a placeholder with no value rather than sending an empty segment", () => {
    const formState = new FormState();
    formState.setValue(GROUP, "process_name", ["p1"]);
    expect(substituteStagePath(MANIFEST_PATH, formState, GROUP)).toEqual({
      ok: false,
      message: 'Select a row first: "session_id" has no value.',
    });
  });

  it("refuses two selected rows rather than taking the first, as the Dart's unpack does", () => {
    const formState = selected();
    formState.setValue(GROUP, "session_id", ["s1", "s2"]);
    expect(substituteStagePath(MANIFEST_PATH, formState, GROUP)).toEqual({
      ok: false,
      message: 'Select one row: "session_id" has more than one value.',
    });
  });

  it("refuses a placeholder that is not a bare key", () => {
    const result = substituteStagePath("a/{{ session_id }}/b", selected(), GROUP);
    expect(result.ok).toBe(false);
  });
});

describe("applyReplacement", () => {
  it("replaces every occurrence, literally", () => {
    expect(applyReplacement("a|b|c", { filePath: "x", replaceText: "|", replaceWith: "," })).toBe("a,b,c");
    expect(applyReplacement("a.b", { filePath: "x", replaceText: ".", replaceWith: "-" })).toBe("a-b");
  });

  it("does nothing unless both halves are given, and ignores an empty replace_text", () => {
    expect(applyReplacement("a|b", { filePath: "x", replaceText: "|" })).toBe("a|b");
    expect(applyReplacement("ab", { filePath: "x", replaceText: "", replaceWith: "," })).toBe("ab");
  });

  it("accepts an empty replace_with, which deletes", () => {
    expect(applyReplacement("a|b", { filePath: "x", replaceText: "|", replaceWith: "" })).toBe("ab");
  });
});

describe("fetchStageToClipboard", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("is registered, so a document naming it resolves", () => {
    expect(productionRegistry.actions["fetchStageToClipboard"]).toBe(fetchStageToClipboard);
  });

  it("fetches through the API client, copies and reports to the banner", async () => {
    const { api, sent } = await signedIn([{ status: 200, body: { file_content: '{"run":1}', token: "t1" } }]);
    const host = hostFor(api);
    const formState = withParameters(selected(), { filePath: MANIFEST_PATH, label: "Run manifest" });

    expect(await run(formState, host)).toBeNull();

    expect(sent).toEqual([
      {
        action: "fetch_file_from_stage",
        data: [{ stage_file_path: "process_name=p1/session_id=s1/run_manifest.json" }],
      },
    ]);
    expect(host.copied).toEqual(['{"run":1}']);
    expect(host.notes).toEqual([["info", "Run manifest copied to the clipboard."]]);
    expect(host.shown).toEqual([]);
  });

  it("applies replace_text and replace_with before copying", async () => {
    const { api } = await signedIn([{ status: 200, body: { file_content: "a|b\nc|d" } }]);
    const host = hostFor(api);
    const formState = withParameters(selected(), { filePath: "x/{{session_id}}.csv", replaceText: "|", replaceWith: "," });
    expect(await run(formState, host)).toBeNull();
    expect(host.copied).toEqual(["a,b\nc,d"]);
  });

  it("consumes its parameters, so one button's replace_text never reaches the next", async () => {
    const { api } = await signedIn([
      { status: 200, body: { file_content: "a|b" } },
      { status: 200, body: { file_content: "a|b" } },
    ]);
    const host = hostFor(api);
    const formState = withParameters(selected(), { filePath: "x.csv", replaceText: "|", replaceWith: "," });
    await run(formState, host);
    for (const key of Object.values(STAGE_CLIPBOARD_KEYS)) expect(formState.getValue(GROUP, key)).toBeUndefined();

    withParameters(formState, { filePath: "y.json" });
    await run(formState, host);
    expect(host.copied).toEqual(["a,b", "a|b"]);
  });

  it("shows the text in a dialog when the browser refuses the clipboard write (R-2)", async () => {
    const { api } = await signedIn([{ status: 200, body: { file_content: "the text" } }]);
    const host = hostFor(api, "refuses");
    const formState = withParameters(selected(), { filePath: "x.json", label: "Run manifest" });

    expect(await run(formState, host)).toBeNull();

    expect(host.copied).toEqual([]);
    expect(host.shown).toEqual([["Run manifest", "the text"]]);
    expect(host.notes).toEqual([
      ["info", "The browser did not allow copying Run manifest; it is shown so it can be copied by hand."],
    ]);
  });

  it("goes straight to the dialog on a host with no clipboard", async () => {
    const { api } = await signedIn([{ status: 200, body: { file_content: "the text" } }]);
    const host = hostFor(api, "absent");
    expect(await run(withParameters(selected(), { filePath: "x.json" }), host)).toBeNull();
    expect(host.shown).toEqual([["File", "the text"]]);
  });

  it("returns an error naming the server's reason when the fetch fails", async () => {
    const { api } = await signedIn([{ status: 400, body: { error: "error: failed to fetch file from stage: timeout" } }]);
    const host = hostFor(api);
    const message = await run(withParameters(selected(), { filePath: "x.json", label: "Run manifest" }), host);
    expect(message).toBe("Run manifest could not be fetched: error: failed to fetch file from stage: timeout");
    expect(host.copied).toEqual([]);
    expect(host.shown).toEqual([]);
  });

  it("reports a missing object as an error naming the path, when the document says nothing else", async () => {
    const { api } = await signedIn([{ status: 400, body: { error: NO_SUCH_KEY } }]);
    const host = hostFor(api);
    const message = await run(withParameters(selected(), { filePath: MANIFEST_PATH, label: "Analysis Report" }), host);
    expect(message).toBe(
      'Analysis Report: there is no file at "process_name=p1/session_id=s1/run_manifest.json" in the stage.',
    );
  });

  it("reports a missing object as a plain fact when the document supplies one", async () => {
    const { api } = await signedIn([{ status: 400, body: { error: NO_SUCH_KEY } }]);
    const host = hostFor(api);
    const formState = withParameters(selected(), {
      filePath: MANIFEST_PATH,
      label: "Run manifest",
      missingMessage: "There is no manifest for this run.",
    });
    expect(await run(formState, host)).toBeNull();
    expect(host.notes).toEqual([["info", "There is no manifest for this run."]]);
    expect(host.copied).toEqual([]);
  });

  it("reads a 404 as missing too, so the handler can be taught one without this changing", async () => {
    const { api } = await signedIn([{ status: 404, body: { error: "no such object" } }]);
    expect(await fetchStageFileThrough(api)("x.json")).toEqual({ kind: "missing", path: "x.json" });
  });

  it("does not read a 400 without NoSuchKey as missing", async () => {
    const { api } = await signedIn([{ status: 400, body: { error: "stage_file_path must be string" } }]);
    expect(await fetchStageFileThrough(api)("x.json")).toEqual({
      kind: "error",
      message: "stage_file_path must be string",
    });
  });

  it("lets a 401 through to the screen, which signs the user out", async () => {
    const { api } = await signedIn([{ status: 401, body: { error: "expired" } }]);
    await expect(fetchStageFileThrough(api)("x.json")).rejects.toBeInstanceOf(SessionExpiredError);
  });

  it("refuses before fetching when no row is selected", async () => {
    const { api, sent } = await signedIn([]);
    const host = hostFor(api);
    const formState = withParameters(new FormState(), { filePath: MANIFEST_PATH });
    expect(await run(formState, host)).toBe('Select a row first: "process_name" has no value.');
    expect(sent).toEqual([]);
  });

  it("says so on a host that cannot fetch from the stage", async () => {
    const { api } = await signedIn([]);
    const { fetchStageFile: _dropped, ...host } = hostFor(api);
    expect(await run(withParameters(selected(), { filePath: "x.json" }), host)).toBe(
      "This screen cannot fetch files from the stage.",
    );
  });

  it("says so when the document names no file", async () => {
    const { api } = await signedIn([]);
    expect(await run(selected(), hostFor(api))).toBe("File: the button names no stage file.");
  });
});

describe("browserWriteClipboard", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("writes through navigator.clipboard", async () => {
    const writeText = vi.fn(async () => {});
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    await browserWriteClipboard("hello");
    expect(writeText).toHaveBeenCalledWith("hello");
  });

  it("rejects where there is no clipboard API, which is any insecure context", async () => {
    vi.stubGlobal("navigator", {});
    await expect(browserWriteClipboard("hello")).rejects.toThrow(/not available/);
  });

  it("rejects when the browser refuses", async () => {
    vi.stubGlobal("navigator", {
      clipboard: { writeText: async () => Promise.reject(new DOMException("denied", "NotAllowedError")) },
    });
    await expect(browserWriteClipboard("hello")).rejects.toThrow("denied");
  });
});
