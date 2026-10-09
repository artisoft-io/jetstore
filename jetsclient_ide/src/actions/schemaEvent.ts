/**
 * `copySchemaEvent`: the selected run's schema event, onto the clipboard.
 * jetstore_maintenance_02, defect `D04`, task `AE.5` (2026-10-01).
 *
 * ## Keyed on `main_input_registry_key`, not on `input_session_id`
 *
 * The report asks for the schema event *of the input session*. Looked up that way
 * it can return several rows: `input_registry` is unique on
 * `(client, org, object_type, table_name, session_id)` (`jets/jets_schema.json`),
 * and `RegisterFileKeys` writes one row per domain key under one session — so a
 * session is not a registration. `pipeline_execution_status.main_input_registry_key`
 * names exactly one row, sits on the same table row, and is what the run was
 * started from; the Pipeline Status table's binding publishes it as of this task.
 * The assessment's `R8` and fact `F17`, and a correction to the report.
 *
 * ## Why it is an escape
 *
 * It reads one column of one row and writes the clipboard, and neither half is a
 * step: `query` resolves a *registered* statement into form state and nothing
 * writes a clipboard. It is `loadReteSession`'s shape (`processErrors.ts`) — the
 * key is an integer primary key arriving on the selection, coerced through
 * `Number` before it is spliced, and the read is a `raw_query` gated by
 * `jetstore_read` — ending in `copyWithFallback`, so a refused clipboard write
 * shows the text instead (`R-2`).
 */

import type { EscapeContext, EscapeHost } from "./escapes";
import { copyWithFallback } from "./stageClipboard";

/** The form-state key the table's binding publishes the registry key under. */
export const MAIN_INPUT_REGISTRY_KEY = "main_input_registry_key";

export async function copySchemaEvent(context: EscapeContext, host: EscapeHost): Promise<string | null> {
  const held = context.formState.getValue(context.group, MAIN_INPUT_REGISTRY_KEY);
  if (Array.isArray(held) && held.length > 1) return "Select one run: more than one is selected.";
  const raw = Array.isArray(held) ? held[0] : held;
  const key = Number(raw);
  if (raw == null || raw === "" || !Number.isInteger(key)) {
    return "Select a run first: it has no input registration to read.";
  }
  const rows = await host.read({
    endpoint: "/dataTable",
    body: {
      action: "raw_query",
      query: `SELECT schema_provider_json FROM jetsapi.input_registry WHERE key = ${key}`,
    },
  });
  if (rows === null) return "Could not read the schema event for this run.";
  if (rows.length === 0) return `There is no input registration ${key} for this run.`;
  const event = rows[0]?.[0] ?? null;
  if (event === null || event === "") {
    // Not an error: a registration made without a schema provider has none, and
    // that is the answer rather than a failure.
    host.notify("info", "This run's input registration has no schema event.");
    return null;
  }
  return copyWithFallback(host, "Schema event", event);
}
