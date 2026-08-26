/**
 * The two escapes `/processErrors/:session_id` needs. Task C.9.
 *
 * ## One of the Dart's three arms is here and two are not, which is the useful count
 *
 * `processErrorsActions` (`jetsclient/lib/modules/actions/process_errors_delegates.dart`,
 * `processErrorsActions`) has three live cases besides `dialogCancel`, and only
 * one of them needs a body:
 *
 * | Arm | Where it went |
 * |---|---|
 * | `setupShowReteTriplesV2` | `loadReteSession` below — an escape |
 * | `reteSession.VisitEntity` | the action document, as a guarded `set` |
 * | `setupShowReteTriples` (v1) | nowhere; C.0 deleted the v1 dialog |
 *
 * **The second is the one worth stating.** It reads two form-state values, checks
 * that the property is a `named_resource`, and copies one into another key — which
 * is `{ "do": "set", "when": { "op": "equals", … } }` exactly, using F.2's guard
 * and no new vocabulary. So this screen adds **one** escape rather than two, and
 * I-74's rule holds again: the escape count is an upper bound on what the grammar
 * cannot say, not a count of what looked like a function in the Dart.
 *
 * ## Why the remaining one cannot be a step
 *
 * `loadReteSession` fetches one column of one row and then **decodes it twice**:
 * the cell is a JSON object, and its `rdf_types` member is itself a JSON string
 * (`process_errors_delegates.dart`, the `setupShowReteTriplesV2` case). Neither
 * `query` nor `post` reaches inside a value — `query` answers a row by column name
 * and `set` copies a key — and a grammar that could walk into a JSON document
 * would be a second language beside the one for form state. This is the shape
 * `escapes.ts` reserves the mechanism for.
 */

import type { EscapeContext, EscapeHost } from "./escapes";
import type { JetsRow } from "../datatable/types";

/** The three keys the explorer's three tables read. See `table.ts`, `ModelSourceSchema`. */
export const RETE_RDF_TYPES = "rete_session.rdf_types";
export const RETE_ENTITY_KEY_BY_TYPE = "rete_session.entity_key_by_type";
export const RETE_ENTITY_DETAILS_BY_KEY = "rete_session.entity_details_by_key";

/**
 * The three models, as form state can hold them.
 *
 * **`FormStateValue` is `string | string[] | null | undefined` and the Dart's
 * `JetsFormState` is `dynamic`**, so the two apps cannot store the same thing.
 * The Dart puts decoded maps and lists straight into form state; this stores
 * **JSON strings**, and the table document's `model` lookup parses on the way out
 * (`datatable/useTableBinding.ts`, `modelRowsOf`).
 *
 * That is not a workaround — it is closer to the Dart than the alternative. The
 * saved session is already nested JSON at three depths: the cell is a JSON
 * object, its `rdf_types` member is a JSON *string*, and the members of
 * `entity_key_by_type` and `entity_details_by_key` are JSON strings too, which is
 * why the Dart's two handlers each end in `json.decode(...)`
 * (`jetsclient/lib/modules/rete_session/model_handlers.dart`). Keeping the values
 * encoded until the table asks for them reproduces exactly that, and the
 * alternative — widening `FormStateValue` to hold arbitrary JSON — would change
 * the type every form field, every where clause and every escape in this app
 * reads, to serve one screen.
 */
function asJsonString(raw: unknown): string | null {
  if (raw == null) return null;
  return typeof raw === "string" ? raw : JSON.stringify(raw);
}

/**
 * Loads the saved rete session for the selected error row.
 *
 * **The statement is composed here rather than registered**, which is a departure
 * from F.6's `queries` namespace and is argued rather than convenient: a
 * registered query is substituted from form state as text, and the value here is
 * `process_errors.key`, an integer primary key that arrives on the selection. The
 * Dart writes it into the statement the same way
 * (`process_errors_delegates.dart`, the `rawQuery['query']` assignment). It is a
 * **read** on one column of one row, dispatched as `raw_query` and therefore gated
 * by `CapabilityReadData` (`jets/apiserver/api_tables.go`, the `raw_query` case).
 *
 * The key is coerced through `Number` before it is spliced, so a form-state value
 * that is not a number cannot reach the statement. The Dart does not do this — it
 * interpolates `keyList[0].toString()` — and the difference costs nothing here,
 * because the only writer of that key is the table's own `formStateBinding`.
 */
export async function loadReteSession(
  context: EscapeContext,
  host: EscapeHost,
): Promise<string | null> {
  const held = context.formState.getValue(0, "key");
  const raw = Array.isArray(held) ? held[0] : held;
  const key = Number(raw);
  if (raw == null || raw === "" || !Number.isFinite(key)) {
    return "No process error row is selected.";
  }
  const rows: JetsRow[] | null = await host.read({
    endpoint: "/dataTable",
    body: {
      action: "raw_query",
      query: `SELECT rete_session_triples FROM jetsapi.process_errors WHERE key = ${key}`,
    },
  });
  if (rows === null) return "Could not read the rule session for this row.";
  const cell = rows[0]?.[0] ?? null;
  if (cell === null) {
    // **Not an error.** `process_errors.rete_session_triples` is null unless the
    // pipeline was configured to save sessions — `max_rete_sessions_saved` on the
    // pipeline configuration — and the errors table carries a `rete_session_saved`
    // column precisely so a user can see which rows have one. An empty explorer is
    // the honest answer; a banner would report a configuration choice as a failure.
    context.formState.setValue(0, RETE_RDF_TYPES, null);
    context.formState.setValue(0, RETE_ENTITY_KEY_BY_TYPE, null);
    context.formState.setValue(0, RETE_ENTITY_DETAILS_BY_KEY, null);
    context.formState.notifyListeners();
    return null;
  }
  let session: unknown;
  try {
    session = JSON.parse(cell);
  } catch {
    return "The stored rule session could not be read.";
  }
  if (session === null || typeof session !== "object" || Array.isArray(session)) {
    return "The stored rule session could not be read.";
  }
  const model = session as Record<string, unknown>;
  context.formState.setValue(0, RETE_RDF_TYPES, asJsonString(model["rdf_types"]));
  context.formState.setValue(0, RETE_ENTITY_KEY_BY_TYPE, asJsonString(model["entity_key_by_type"]));
  context.formState.setValue(
    0,
    RETE_ENTITY_DETAILS_BY_KEY,
    asJsonString(model["entity_details_by_key"]),
  );
  context.formState.notifyListeners();
  return null;
}

/**
 * Seeds one repeated row of the input-records dialog. Task C.9.
 *
 * `viewInputRecords`'s `inputFieldRowBuilder`
 * (`jetsclient/lib/modules/form_config_impl.dart`, `FormKeys.viewInputRecords`),
 * term for term. The query returns one row per input source of the pipeline —
 * main, merged and injected — and each row becomes one validation group holding
 * one table.
 *
 * **The eighth value is computed and the other seven are copied**, and it is the
 * reason this is an escape rather than a column map: `domainKeyColumn` is
 * `'<object_type>:domain_key'`, the name of the key column in a *domain* table,
 * which differs per row and which the table's where clause then resolves through
 * `lookupColumnInFormState` (`table.ts`, `WhereClauseSchema`). A seed that only
 * copied columns could not produce it.
 */
export function seedInputRecordsRow(context: EscapeContext, row: JetsRow, index: number): void {
  const at = (position: number): string => row[position] ?? "";
  const set = (key: string, value: string): void => context.formState.setValue(index, key, value);
  set("label", at(1));
  set("table_name", at(2));
  set("lookback_periods", at(3));
  set("session_id", at(4));
  set("pipeline_execution_status_key", at(5));
  set("object_type", at(6));
  set("domain_key", at(7));
  set("domainKeyColumn", `${at(6)}:domain_key`);
}
