/**
 * `fetchStageToClipboard`: read one file out of the S3 stage and put it on the
 * clipboard. jetstore_maintenance_02, defect `D04`, task `AE.3` (2026-10-01).
 *
 * ## What it replaces
 *
 * The Flutter app had this as a *configured* button rather than as code: a
 * deployment's `JETS_CUSTOM_BUTTONS_CONFIG_JSON` was compiled in, and each entry of
 * `type: fetch_stage_to_clipboard` became a button on the Pipeline Status table
 * that substituted the selected row's values into a stage path, fetched it,
 * optionally replaced one string with another, and wrote the clipboard
 * (`jetsclient/lib/button_config.dart`, `buttonConfigActions`, at `c38483b` in
 * `repo_infra_jetstore_platform`). Cedargate's *Analysis Report* is one, and it
 * disappeared with the Flutter app. *Get Run Manifest* is the same mechanism with a
 * fixed path, which is why it is built once here and both are instances of it.
 *
 * ## Why it is an escape and not a `post`
 *
 * **A grammar `post` cannot receive the file.** Home's host answers a `post` with
 * `{statusCode}` and nothing else, and `read` keeps only `rows`, which
 * `fetch_file_from_stage` does not return. So the read goes through the API client,
 * behind `EscapeHost.fetchStageFile`, and this module supplies the one
 * implementation of it (`fetchStageFileThrough`).
 *
 * ## How a document calls it
 *
 * An escape step takes no arguments, so its parameters arrive the way every other
 * escape's inputs do — in form state — written by `set` steps in front of it:
 *
 * ```json
 * { "do": "set", "key": "stage_clipboard.file_path",
 *   "value": { "literal": "process_name={{process_name}}/session_id={{session_id}}/run_manifest.json" } },
 * { "do": "escape", "name": "fetchStageToClipboard" }
 * ```
 *
 * **The escape clears every parameter key before it does anything else.** Home's
 * form state is one group shared by every button on the screen, so a
 * `replace_text` left behind by one button would otherwise be applied by the next
 * one that does not set it — a manifest with its `|` rewritten to `,` because a
 * button that converts a pipe-delimited report was pressed first. Consuming the
 * parameters makes each invocation exactly what its own steps said.
 *
 * `{{name}}` is substituted from form state, unwrapping the one-element list a
 * table's `formStateBinding` publishes for every secondary column. **Double braces,
 * not the grammar's `template` form**, which substitutes `{name}` and turns a
 * missing value into an empty string: the stored paths are Flutter's and use
 * `{{…}}`, and a stage path with an empty segment is a read of the wrong object
 * rather than a failure.
 *
 * ## What it reports, and where
 *
 * Success and the fallback go to the banner through `host.notify("info", …)`;
 * a failure is the returned string, which a screen's `runAction` shows as its
 * error. **A missing object is either**, decided by the document: with
 * `stage_clipboard.missing_message` set it is an expected outcome reported as
 * that text — a run manifest exists for completed cpipes runs only — and without
 * it, an error naming the path.
 */

import type { FormState, FormStateValue } from "../datatable/formState";
import { ApiError, type ApiClient } from "../api/client";
import type { EscapeContext, EscapeHost, StageFetchResult } from "./escapes";

/** The form-state keys a document writes before the escape step. */
export const STAGE_CLIPBOARD_KEYS = {
  /** Required. The path under `JETS_s3_STAGE_PREFIX`, with `{{key}}` placeholders. */
  filePath: "stage_clipboard.file_path",
  /** What the banner calls the file. Defaults to `File`. */
  label: "stage_clipboard.label",
  /** Replaced, every occurrence, by `replace_with`. Ignored unless both are set. */
  replaceText: "stage_clipboard.replace_text",
  replaceWith: "stage_clipboard.replace_with",
  /** When set, a missing object is reported as this text rather than as an error. */
  missingMessage: "stage_clipboard.missing_message",
} as const;

/** One clipboard button, however it was configured. */
export interface StageClipboardSpec {
  /** The path under the stage prefix, with `{{key}}` placeholders. */
  filePath: string;
  label?: string | null;
  replaceText?: string | null;
  replaceWith?: string | null;
  missingMessage?: string | null;
}

const PLACEHOLDER = /\{\{([A-Za-z0-9_.-]+)\}\}/g;

/** A form-state value as one string, or why it is not one. */
function single(value: FormStateValue): { ok: true; value: string } | { ok: false; why: "none" | "many" } {
  if (typeof value === "string") return value === "" ? { ok: false, why: "none" } : { ok: true, value };
  if (Array.isArray(value)) {
    if (value.length > 1) return { ok: false, why: "many" };
    const first = value[0];
    return typeof first === "string" && first !== "" ? { ok: true, value: first } : { ok: false, why: "none" };
  }
  return { ok: false, why: "none" };
}

/**
 * Substitutes `{{key}}` from form state, or says why it cannot.
 *
 * **More than one value is refused rather than taking the first.** The Dart's
 * `unpack` takes element 0, which is right on a single-select table and silently
 * wrong on any other: the binding publishes a secondary key as the list of every
 * selected row's value. A placeholder left after substitution — `{{ name }}`, with
 * spaces — is refused too, because sending it would ask the server for an object
 * whose key contains braces.
 */
export function substituteStagePath(
  template: string,
  formState: FormState,
  group: number,
): { ok: true; path: string } | { ok: false; message: string } {
  let failure: string | null = null;
  const path = template.replace(PLACEHOLDER, (whole, key: string) => {
    if (failure !== null) return whole;
    const held = single(formState.getValue(group, key));
    if (held.ok) return held.value;
    failure =
      held.why === "many"
        ? `Select one row: "${key}" has more than one value.`
        : `Select a row first: "${key}" has no value.`;
    return whole;
  });
  if (failure !== null) return { ok: false, message: failure };
  if (path.includes("{{") || path.includes("}}")) {
    return { ok: false, message: `The stage path "${template}" has a placeholder that is not a bare key.` };
  }
  return { ok: true, path };
}

/**
 * `replaceAll`, literally, and only when both halves are given.
 *
 * An empty `replace_text` is treated as absent: the Dart's `replaceAll("", x)`
 * inserts `x` between every character, which no configuration means.
 */
export function applyReplacement(content: string, spec: StageClipboardSpec): string {
  const { replaceText, replaceWith } = spec;
  if (replaceText == null || replaceText === "" || replaceWith == null) return content;
  return content.split(replaceText).join(replaceWith);
}

/**
 * Copies, or shows the text when the browser will not let it be copied (`R-2`).
 *
 * Shared because `D04`'s *Get Schema Event* writes the clipboard too and must
 * fall back the same way. Returns the error to report, or null.
 */
export async function copyWithFallback(
  host: EscapeHost,
  label: string,
  text: string,
): Promise<string | null> {
  if (host.writeClipboard !== undefined) {
    try {
      await host.writeClipboard(text);
      host.notify("info", `${label} copied to the clipboard.`);
      return null;
    } catch {
      // Refused: fall through to the dialog. The reason is not shown, because
      // the user's next step is the same whatever the browser objected to.
    }
  }
  if (host.showText === undefined) {
    return `${label} was fetched, but this screen can neither copy it nor show it.`;
  }
  host.showText(label, text);
  host.notify("info", `The browser did not allow copying ${label}; it is shown so it can be copied by hand.`);
  return null;
}

/** The whole mechanism, given a spec. What a runtime-configured button calls. */
export async function runStageClipboard(
  spec: StageClipboardSpec,
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> {
  const label = spec.label != null && spec.label !== "" ? spec.label : "File";
  if (spec.filePath === "") return `${label}: the button names no stage file.`;
  if (host.fetchStageFile === undefined) return "This screen cannot fetch files from the stage.";

  const substituted = substituteStagePath(spec.filePath, context.formState, context.group);
  if (!substituted.ok) return substituted.message;

  const fetched: StageFetchResult = await host.fetchStageFile(substituted.path);
  switch (fetched.kind) {
    case "missing":
      if (spec.missingMessage != null && spec.missingMessage !== "") {
        host.notify("info", spec.missingMessage);
        return null;
      }
      return `${label}: there is no file at "${fetched.path}" in the stage.`;
    case "error":
      return `${label} could not be fetched: ${fetched.message}`;
    case "content":
      return copyWithFallback(host, label, applyReplacement(fetched.content, spec));
  }
}

/** A string parameter out of form state; anything else is absent. */
function parameter(formState: FormState, group: number, key: string): string | null {
  const value = formState.getValue(group, key);
  return typeof value === "string" ? value : null;
}

/**
 * The registered escape: reads its parameters, clears them, and runs.
 *
 * The parameters are cleared **before** the fetch, so an invocation that fails
 * part-way still leaves nothing behind for the next button.
 */
export async function fetchStageToClipboard(
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> {
  const { formState, group } = context;
  const spec: StageClipboardSpec = {
    filePath: parameter(formState, group, STAGE_CLIPBOARD_KEYS.filePath) ?? "",
    label: parameter(formState, group, STAGE_CLIPBOARD_KEYS.label),
    replaceText: parameter(formState, group, STAGE_CLIPBOARD_KEYS.replaceText),
    replaceWith: parameter(formState, group, STAGE_CLIPBOARD_KEYS.replaceWith),
    missingMessage: parameter(formState, group, STAGE_CLIPBOARD_KEYS.missingMessage),
  };
  for (const key of Object.values(STAGE_CLIPBOARD_KEYS)) formState.setValue(group, key, null);
  return runStageClipboard(spec, context, host);
}

/**
 * Whether a failed `fetch_file_from_stage` means *there is no such object*.
 *
 * **Two forms, and only the second exists today.** The handler answers every
 * download failure with **400** and the error text
 * (`jets/apiserver/api_tables.go`, the `fetch_file_from_stage` case), having
 * flattened the SDK's typed error to a string twice — once in `DownloadBufFromS3`
 * (`jets/awsi/awsi.go`) and once in the handler — so no status distinguishes a
 * missing object from a failed read. What survives is S3's error code inside the
 * text: the SDK formats `*types.NoSuchKey` as `NoSuchKey: …`, and the UI task role
 * holds `s3:List*` on the bucket (`SourceBucket.GrantReadWrite`), so a missing key
 * is reported as `NoSuchKey` rather than masked as `AccessDenied`. Measured by
 * reading the code on 2026-10-01, not against a live bucket.
 *
 * A **404** is accepted as well, so that the handler can be taught to answer one
 * without this function changing; that is the better fix and is recorded rather
 * than made here, because the server is not this task's.
 */
export function isMissingStageObject(error: ApiError): boolean {
  if (error.status === 404) return true;
  return error.status === 400 && /\bNoSuchKey\b/.test(error.message);
}

/** `EscapeHost.fetchStageFile`, implemented on the API client. */
export function fetchStageFileThrough(api: Pick<ApiClient, "dataTable">) {
  return async (path: string): Promise<StageFetchResult> => {
    try {
      const body = await api.dataTable<{ file_content?: unknown }>({
        action: "fetch_file_from_stage",
        data: [{ stage_file_path: path }],
      });
      if (typeof body.file_content !== "string") {
        return { kind: "error", message: "the server answered without the file's content." };
      }
      return { kind: "content", content: body.file_content };
    } catch (error) {
      // A 401 is not caught here on purpose: `ApiClient` has already signed the
      // user out, and the screen's own handler is what reacts to that.
      if (error instanceof ApiError && error.status !== 401) {
        return isMissingStageObject(error) ? { kind: "missing", path } : { kind: "error", message: error.message };
      }
      throw error;
    }
  };
}

/**
 * `EscapeHost.writeClipboard` for a browser. Rejects when there is no clipboard
 * API — outside a secure context `navigator.clipboard` is undefined — and when
 * the browser refuses the write.
 */
export async function browserWriteClipboard(text: string): Promise<void> {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (clipboard === undefined || typeof clipboard.writeText !== "function") {
    throw new Error("the clipboard is not available here");
  }
  await clipboard.writeText(text);
}
