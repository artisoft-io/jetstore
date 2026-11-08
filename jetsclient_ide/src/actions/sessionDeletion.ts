/**
 * The Session Registry tab's *Delete*: the escape that lists what a deletion
 * reaches, and the predicate that gates the button. `session_registry` Phase 2,
 * JM-081, task G.3, 2026-11-08.
 *
 * ## Why an escape, and only for the first step
 *
 * *Delete* is the action `deleteSessions` in `homeScreen.ua.json`: this escape,
 * then `confirm`, then `post delete_sessions`, then `notify`. Three of the four
 * are grammar. This one is not because **a `post` step keeps the status and
 * drops the body** (`interpret.ts`, the `post` arm), and the preview's body is the
 * whole of it — the sessions the deletion reaches, which the user must see before
 * answering. `EscapeHost.read` returns rows, which is what this needs.
 *
 * **And it does not confirm, on purpose.** `escapes.ts` keeps `confirm` off the
 * escape host so that what the user is asked is in the document. So this writes
 * the list into form state and the document's own `confirm` step shows it, by
 * its new `detailFromKey`; the question stays authored and the list stays
 * computed, and neither is the other's.
 *
 * ## What the confirmation is checked against
 *
 * The preview's ids are written to `session_deletion_expected`, and the `post`
 * sends them back as `expected`. The server recomputes the closure and answers
 * 409 when it differs (a session registered or a run started since the
 * preview), so what is queued is what was confirmed or nothing.
 *
 * ## The server's shapes, in one place
 *
 * `previewRequest` and `previewRowOf` below are the only code that knows the
 * request and response of `preview_session_deletion` (`jets/apiserver`,
 * `DoDataTableAction`), and `homeScreen.ua.json`'s `deleteSessions` the only
 * document that knows `delete_sessions`'. Aligned with the server's shapes as
 * its pull request states them (jetstore#2410, 2026-11-08):
 *
 *  - the preview takes one `data` row per selected session, `{session_id}`, and
 *    answers `{rows: [{session_id, client, source_type, month_period (int or
 *    null), reads_session ("" for a selected one), process_name, object_type,
 *    active (bool)}]}`; 404 when a selected session is no longer registered, 403
 *    without `delete_session`;
 *  - `delete_sessions` takes one `data` row, `{session_id: [ids], expected:
 *    [ids]}` — `expected` is read from the rows, not from the envelope — and
 *    answers `{queued, waiting}`, or 409 when the closure changed. **`queued`
 *    is not the closure's size** (a session already queued by a user is not
 *    counted), and a `post` step drops the body anyway, so the notification
 *    after it is static.
 */

import type { FormState } from "../datatable/formState";
import type { EndpointRequest, EscapeContext, EscapeHost } from "./escapes";

/** The table key whose selection is the sessions to delete. */
export const SESSION_REGISTRY_TABLE = "sessionRegistryTable";
/**
 * The form-state key the table binds every selected row's `deletion_status`
 * to (`sessionRegistryTable.tc.json`, `formStateBinding.otherColumns`).
 */
export const SELECTED_DELETION_STATUS = "sr_deletion_status";
/** The ids the preview listed, which `delete_sessions` sends back as `expected`. */
export const EXPECTED_KEY = "session_deletion_expected";
/** The preview as text, one session per line, which `confirm` shows. */
export const PREVIEW_KEY = "session_deletion_preview";

/** One session the deletion reaches, as the preview lists it. */
export interface PreviewRow {
  sessionId: string;
  client: string;
  sourceType: string;
  monthPeriod: string;
  /** The session it reads through `input_session_id`; empty for a selected one. */
  readsSession: string;
  processName: string;
  objectType: string;
  /** A pending or submitted run on it or reading it: its deletion waits for that. */
  active: boolean;
}

/** The selected sessions' ids, in selection order. */
function selectedSessions(formState: FormState, group: number): string[] {
  const value = formState.getValue(group, SESSION_REGISTRY_TABLE);
  if (Array.isArray(value)) return value.filter((v): v is string => typeof v === "string" && v !== "");
  return typeof value === "string" && value !== "" ? [value] : [];
}

/** The preview request: one data row per selected session. */
export function previewRequest(sessionIds: string[]): EndpointRequest {
  return {
    endpoint: "/dataTable",
    body: {
      action: "preview_session_deletion",
      data: sessionIds.map((sessionId) => ({ session_id: sessionId })),
    },
  };
}

/**
 * One response row, read by name.
 *
 * The response is `{rows: [<Dependent as JSON>...]}`. A row is read by its
 * snake_case names, the plan's; Go's untagged field names are accepted too, so
 * that a `Dependent` serialised without tags does not show the user a list of
 * blanks. A row with no session id is dropped — it names nothing to delete — and
 * a positional row (an array) is not a shape this action answers with.
 */
export function previewRowOf(raw: unknown): PreviewRow | null {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) return null;
  const row = raw as Record<string, unknown>;
  const text = (snake: string, go: string): string => {
    const value = row[snake] ?? row[go];
    if (value === null || value === undefined) return "";
    return typeof value === "string" ? value : String(value);
  };
  const sessionId = text("session_id", "SessionID");
  if (sessionId === "") return null;
  const active = row["active"] ?? row["Active"];
  return {
    sessionId,
    client: text("client", "Client"),
    sourceType: text("source_type", "SourceType"),
    monthPeriod: text("month_period", "MonthPeriod"),
    readsSession: text("reads_session", "ReadsSession"),
    processName: text("process_name", "ProcessName"),
    objectType: text("object_type", "ObjectType"),
    active: active === true || active === "true",
  };
}

/**
 * One line of the confirmation: the id, what the session is, and why it is
 * there.
 *
 * *What it is* is its process for a pipeline's session and its object type for a
 * file's, the two things a user recognises a session by; the source type when it
 * has neither. **The client is shown as written**, `(unknown)` included: that is
 * the purge's name for a session whose client could not be read (`I-18`), and a
 * user deleting it should see that rather than a blank.
 */
export function previewLine(row: PreviewRow): string {
  const what = [row.processName || row.objectType || row.sourceType || "session"];
  if (row.client !== "") what.push(`client ${row.client}`);
  if (row.monthPeriod !== "") what.push(`month period ${row.monthPeriod}`);
  if (row.readsSession !== "") what.push(`reads ${row.readsSession}`);
  if (row.active) what.push("waits for its run to end");
  return `${row.sessionId}: ${what.join(", ")}`;
}

/** The confirmation's detail: a count, then one line per session. */
export function previewText(rows: PreviewRow[]): string {
  const selected = rows.filter((r) => r.readsSession === "").length;
  const dependents = rows.length - selected;
  const head =
    dependents === 0
      ? `${rows.length} session${rows.length === 1 ? "" : "s"}:`
      : `${rows.length} sessions, ${dependents} of them reading a selected one:`;
  return [head, ...rows.map(previewLine)].join("\n");
}

/**
 * Lists what deleting the selected sessions reaches. The action escape.
 *
 * Writes `session_deletion_expected` and `session_deletion_preview`, or stops the
 * action with a message: nothing selected, a preview the server refused, or a
 * preview that lists nothing. **Both keys are cleared first**, so a refused
 * preview can never leave the previous one for `confirm` to show.
 */
export async function previewSessionDeletion(
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> {
  const { formState, group } = context;
  formState.setValue(group, EXPECTED_KEY, null);
  formState.setValue(group, PREVIEW_KEY, null);
  const sessionIds = selectedSessions(formState, group);
  if (sessionIds.length === 0) return "Select the sessions to delete first";

  host.setBusy(true);
  let raw: unknown[] | null;
  try {
    raw = await host.read(previewRequest(sessionIds));
  } finally {
    host.setBusy(false);
  }
  // `read` answers null for any failed request and keeps no message, so the
  // likely causes are named here: the capability (403), and a session gone
  // since the table was read (404, "not registered in session_registry").
  if (raw === null) {
    return "Could not list the sessions to delete: the server refused the preview. You need delete_session, and every selected session must still be registered; refresh and select again.";
  }
  const rows = raw.map(previewRowOf).filter((r): r is PreviewRow => r !== null);
  if (rows.length === 0) return "The server listed no session to delete";

  formState.setValue(group, EXPECTED_KEY, rows.map((r) => r.sessionId));
  formState.setValue(group, PREVIEW_KEY, previewText(rows));
  return null;
}

/**
 * Whether *Delete* may run: no selected row already has a deletion, queued,
 * failed or done. The predicate `isEnabledEscapeFor` names for the button.
 *
 * Read off the key the table binds every selected row's `deletion_status` to,
 * which holds the non-null values only (`binding.ts`, `resetSecondaryKeys`) — so
 * it is null exactly when no selected row has one, on a multi-select table, where
 * the button's `enableWhen` can only test the first row.
 */
export const noSessionDeletionRequested = (formState: FormState, group: number): boolean => {
  const value = formState.getValue(group, SELECTED_DELETION_STATUS);
  return value === null || value === undefined || (Array.isArray(value) && value.length === 0);
};
