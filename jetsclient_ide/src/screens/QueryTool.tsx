/**
 * The Query Tool. Task C.4.
 *
 * The one screen in the corpus where a user types SQL and the client sends it.
 * Flutter serves it at `/queryTool` with `ScreenWithMultiForms` over
 * `queryToolInputForm` and `queryToolResultViewForm`
 * (`jetsclient/lib/routes/jets_routes_app.dart`, the `queryToolPath` entry).
 *
 * ## What the server does with the text, which is the substance of this screen
 *
 * Both buttons post to `/dataTable` and both are gated, on the same capability:
 *
 * | Button | `action` | Handler | Requires |
 * |---|---|---|---|
 * | Submit Query | `raw_query_tool` | `ExecRawQuery` | `datatable.CapabilityQueryTool` |
 * | Submit DDL | `exec_ddl` | `ExecDataManagementStatement` | the literal `"workspace_ide"` |
 *
 * `CapabilityQueryTool` **is** `"workspace_ide"` (`jets/datatable/data_table_action.go`,
 * the `CapabilityQueryTool` const), per the seed file's own description of that
 * capability as covering "the query tool" (`jets/jets_init_db.sql`). So the two
 * buttons differ in what comes back — rows against a one-cell command tag — and
 * not in what is permitted, and **neither is a privilege boundary over the
 * other**: `ExecRawQuery` passes the string to `dbpool.Query`, which executes a
 * `DROP TABLE` as willingly as a `SELECT`. The split is a client convenience.
 * Recorded as **F84**, because it is the kind of thing a later reader will assume
 * the opposite of.
 *
 * `raw_query` and `raw_query_tool` were one case requiring nothing until I-2 split
 * them, and `TestRawQueryToolIsGatedMoreTightlyThanRawQuery`
 * (`jets/apiserver/read_dispatch_test.go`) refuses to let them share one again.
 *
 * **One divergence from the Flutter client, and it is in the safe direction.**
 * There, `queryToolOk` declares `capability: "workspace_ide"` and `queryToolDdlOk`
 * declares none (`jetsclient/lib/modules/form_config_impl.dart`,
 * `FormKeys.queryToolInputForm`) — so the SELECT button is disabled for a user the
 * DDL button is offered to. The server refuses both either way, so it is
 * presentation and not a hole; both buttons here name the capability. **I-166.**
 *
 * ## Two forms, one form state
 *
 * `ScreenWithMultiForms` gives each form its own `JetsFormState`, so the input
 * form reaches the result form's through `formState.peersFormState?[1]` and writes
 * the statement into it (`jetsclient/lib/modules/actions/query_tool_screen_delegates.dart`,
 * `queryToolFormActions`). **The peer write is the mechanism, and one form state
 * is the same mechanism without the indirection**: the statement is set under
 * `raw_query.ready` or `raw_query.ddl.ready`, then `query.ready` is set and
 * notifies, which unblocks the result table's one where clause and refreshes it.
 *
 * ## No form document
 *
 * `ui-c2-registry` settled that a screen's form documents are bundled at
 * `src/screens/documents/<screenKey>.form.json`. **This screen is the first to
 * decline to need one**, and the reason is not that it is small: a *peer-form
 * write* is a screen mechanism the form document has no construct for, so
 * authoring these two forms would put a real behaviour outside the document and
 * leave the document describing a textarea. The input side is a textarea and two
 * buttons; the result side is one `dataTable` field.
 *
 * **The table configuration is a document either way** — task I-102's decision 1
 * — translated out of the Flutter corpus rather than written, and imported here.
 */

import { useCallback, useMemo, useState } from "react";

import type { ApiClient } from "../api/client";
import { DataTable } from "../datatable/DataTable";
import { FormState } from "../datatable/formState";
import { RAW_QUERY_KEYS } from "../datatable/query";
import { TableConfigDocumentSchema } from "../datatable/table";
import { fromDocument } from "../datatable/tableTranslate";
import type { DataTableFetcher } from "../datatable/useDataTable";
import { useTableBinding } from "../datatable/useTableBinding";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import { TextInput } from "../widgets/TextInput";
import resultSetDocument from "../datatable/tables/queryToolResultSetTable.tc.json";

/** The capability both the screen and the two statements require. */
export const QUERY_TOOL = "workspace_ide";

/** `FSK.rawQuery` — the key the input field writes. */
const RAW_QUERY = "raw_query";

/** The form-state group. One, as everywhere outside `file_mapping`. */
const GROUP = 0;

const RESULT_TABLE_KEY = "queryToolResultSetTable";

/**
 * The result table, from its document.
 *
 * **Parsed rather than cast**, which is what makes the bundled document a
 * document rather than a typed literal: a `.tc.json` that stopped satisfying the
 * schema would fail here, at module load, with the same finding text the Go
 * validator produces at save time.
 */
const resultTableConfig = fromDocument(
  RESULT_TABLE_KEY,
  TableConfigDocumentSchema.parse(resultSetDocument),
);

/** `queryToolFormValidator`'s one rule (`query_tool_screen_delegates.dart`). */
export function validateQuery(query: string): string | null {
  if (query.length > 1) return null;
  if (query.length === 1) return "Query too short.";
  return "Query must be provided.";
}

export function QueryTool({ api }: { api: ApiClient }) {
  const { setError } = useNotifications();
  const formState = useMemo(() => new FormState(), []);
  const [error, setFieldError] = useState<string | null>(null);

  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);

  const table = useTableBinding({
    config: resultTableConfig,
    field: { group: GROUP, key: RESULT_TABLE_KEY },
    formState,
    fetcher,
  });

  /**
   * Submits the statement in the box.
   *
   * `ddl` picks which key it lands under, and `makeRawQuery` reads the plain key
   * first — so **the other key is cleared rather than left**. The Dart can leave
   * both set because it nulls all three once the rows arrive; this does not clear
   * them (see `makeRawQuery`), so a DDL press after a query press would otherwise
   * re-run the query.
   */
  const submit = useCallback(
    (ddl: boolean) => {
      const raw = formState.getValue(GROUP, RAW_QUERY);
      const query = typeof raw === "string" ? raw : "";
      const message = validateQuery(query);
      setFieldError(message);
      if (message !== null) return;

      setError(null);
      formState.setValue(GROUP, RAW_QUERY_KEYS.query, ddl ? null : query);
      formState.setValue(GROUP, RAW_QUERY_KEYS.ddl, ddl ? query : null);
      // Last, and the only one followed by a notification: it is the key the
      // result table's where clause blocks on, so setting it is what both
      // unblocks the table and asks it to re-query. The Dart spells the pair
      // `setValueAndNotify`; this store separates them, and the order is the
      // same — write, then tell.
      formState.setValue(GROUP, RAW_QUERY_KEYS.gate, query);
      formState.notifyListeners();
      // Re-running the *same* statement changes no key and therefore no payload,
      // so the watched-key refresh above would not fire. This is the channel a
      // table action already uses for "something changed on the server".
      formState.requestRefresh();
    },
    [formState, setError],
  );

  return (
    <main className="screen">
      <h1>Query Tool</h1>
      <p className="screen-sub">
        Statements run against the JetStore database with your own authority. Both buttons
        require the <code>{QUERY_TOOL}</code> capability, and the server enforces it.
      </p>

      <div className="stack qt-input">
        <TextInput
          formState={formState}
          group={GROUP}
          fieldKey={RAW_QUERY}
          label="Query"
          hint="Paste query"
          maxLines={10}
          // `FormInputFieldConfig.maxLength` on the Dart field, kept rather than
          // rounded: it is a paste target for generated SQL.
          maxLength={4000000}
          {...(error !== null ? { error } : {})}
        />
        <div className="screen-actions">
          <ActionButton
            capability={QUERY_TOOL}
            className="btn btn-primary"
            disabled={table.loading}
            onClick={() => submit(false)}
          >
            Submit Query
          </ActionButton>
          <ActionButton
            capability={QUERY_TOOL}
            className="btn"
            disabled={table.loading}
            onClick={() => submit(true)}
          >
            Submit DDL
          </ActionButton>
        </div>
      </div>

      <DataTable
        // **Without a footer, and that is a correction rather than a preference.**
        // The raw-statement body carries no `offset` or `limit`, so the whole
        // result set arrives at once and a page control would re-post an identical
        // body and re-render the same rows under a different range label. The
        // Dart's footer paginates an in-memory model; this app's paginates by
        // re-querying, and only one of the two survives a request with no paging
        // in it. The document says `noFooter: false` because the Dart's does.
        config={{ ...resultTableConfig, noFooter: true }}
        state={table}
        modes={table.modes}
      />
    </main>
  );
}
