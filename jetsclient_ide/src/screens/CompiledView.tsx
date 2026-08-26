/**
 * A Workspace IDE section's compiled view, rendered. Task C.3.
 *
 * The heading of a section whose files compile into `workspace.db` opens one of
 * these instead of an editor: a strip of tabs, one data table each, querying the
 * compiled artifact rather than listing the sources beneath it. It is the second
 * half of the duality C.1 made explicit on the wire, and the half the React app
 * had none of — `FileTree` treated a `section` node as a plain directory and
 * `WorkspaceNode.compiled_view` was read by nothing.
 *
 * ## `useDataTable` rather than `useTableBinding`, and that is measured
 *
 * All six tables this renders are **action-less** in the corpus — `actions: []`
 * and `secondRowActions: []` on every one — and none is a field of a form. So
 * there is no selection to publish, no button to gate and no form state to read,
 * which is exactly the case `useDataTable`'s own header says it was written for
 * and the same call `TableScreen.tsx` makes for C.7 and C.8.
 *
 * **The two tables that would need the other hook are deliberately not here.**
 * `wsDataModelFilesTable` and `wsJetRulesFilesTable` carry an action bar whose
 * *Delete* is gated on a selected row and whose *Add File* opens a dialog; they
 * are C.3b, named in `TABS_DEFERRED_TO_C3B` and asserted against the corpus so
 * the gap cannot pass for completeness.
 *
 * ## The workspace name is the whole mechanism, and it is invisible in the rows
 *
 * Every one of these tables reads `workspace.db` for **one** workspace. The name
 * does not appear in any `WHERE`: `makeQuery` puts it on the request envelope as
 * `workspaceName` (`datatable/query.ts`, the `KEYS.workspaceName` block at the
 * end), and `DoWorkspaceReadAction`
 * (`jets/datatable/workspace_data_table_action.go`, `DoWorkspaceReadAction`)
 * opens that workspace's SQLite file. **Drop it and the request still succeeds
 * against whatever workspace the server falls back to, and the screen renders
 * perfectly.** That is I-104's shape with a third field, so `CompiledView.test.tsx`
 * asserts the request rather than the rows.
 *
 * In the Flutter app the name arrives through form state —
 * `formState.setValue(0, FSK.wsName, wsName)` in `initializeWorkspaceFileEditor`
 * (`jetsclient/lib/modules/workspace_ide/screen_delegates_helpers.dart`) — and
 * `_makeQuery` reads form state first and the route second. This app has no form
 * here and its workspace comes from the screen's picker rather than from a route
 * parameter, so it is supplied through `QueryContext.routeParams`, which is the
 * branch `makeQuery` already takes when there is no form.
 *
 * ## Every tab is mounted, and only one is shown
 *
 * `TabBarView` builds all of its children (`screen_tab_form.dart`,
 * `List<Widget>.generate`), so the Flutter screen issues every tab's query when
 * the view opens. Mounting all of them here reproduces that and buys the thing
 * that matters more: a hook may not be called in a loop whose length changes, and
 * paging and sorting are widget state that would be thrown away by unmounting the
 * tab a user just paged.
 */

import { useMemo, useState } from "react";

import type { ApiClient } from "../api/client";
import { DataTable } from "../datatable/DataTable";
import type { TableConfigDocument } from "../datatable/table";
import { fromDocument } from "../datatable/tableTranslate";
import { useDataTable } from "../datatable/useDataTable";
import type { CompiledViewDocument } from "./compiledView";

import wsDataPropertyTable from "../datatable/tables/wsDataPropertyTable.tc.json";
import wsDomainClassTable from "../datatable/tables/wsDomainClassTable.tc.json";
import wsDomainTableTable from "../datatable/tables/wsDomainTableTable.tc.json";
import wsJetRulesTable from "../datatable/tables/wsJetRulesTable.tc.json";
import wsLookupColumnTable from "../datatable/tables/wsLookupColumnTable.tc.json";
import wsLookupTableTable from "../datatable/tables/wsLookupTableTable.tc.json";
import wsMainSupportFilesTable from "../datatable/tables/wsMainSupportFilesTable.tc.json";
import wsRuleTermsTable from "../datatable/tables/wsRuleTermsTable.tc.json";

/**
 * The committed table documents a compiled view may name.
 *
 * Bundled rather than read from the workspace, per **I-170**: there is one Data
 * Model view per deployment, not one per workspace, and the Flutter equivalent is
 * `getTableConfig(DTKeys.wsDomainClassTable)` compiled into the app.
 */
export const COMPILED_VIEW_TABLES: Record<string, TableConfigDocument> = {
  wsDomainClassTable: wsDomainClassTable as TableConfigDocument,
  wsDataPropertyTable: wsDataPropertyTable as TableConfigDocument,
  wsDomainTableTable: wsDomainTableTable as TableConfigDocument,
  wsJetRulesTable: wsJetRulesTable as TableConfigDocument,
  wsRuleTermsTable: wsRuleTermsTable as TableConfigDocument,
  wsMainSupportFilesTable: wsMainSupportFilesTable as TableConfigDocument,
  // C.3a's two, authored rather than translated — the `lookups` view the Flutter
  // app never had. See `table.test.ts`'s authored-documents block for what checks
  // them in place of the round trip.
  wsLookupTableTable: wsLookupTableTable as TableConfigDocument,
  wsLookupColumnTable: wsLookupColumnTable as TableConfigDocument,
};

export interface CompiledViewProps {
  api: ApiClient;
  document: CompiledViewDocument;
  /** The workspace whose `workspace.db` every tab reads. */
  workspaceName: string;
}

function ViewTab({
  api,
  tableKey,
  workspaceName,
  hidden,
  panelId,
  tabId,
}: {
  api: ApiClient;
  tableKey: string;
  workspaceName: string;
  hidden: boolean;
  panelId: string;
  tabId: string;
}) {
  const document = COMPILED_VIEW_TABLES[tableKey];
  // A tab naming a table this app does not bundle is an authoring error in the
  // view document, and it says so rather than rendering an empty table: the
  // schema can check that `table` is an identifier and not that it is *this*
  // identifier, so the check has to be here.
  if (document === undefined) {
    throw new Error(`compiled view names table "${tableKey}", which is not bundled`);
  }
  return (
    <TabPanel
      api={api}
      tableKey={tableKey}
      document={document}
      workspaceName={workspaceName}
      hidden={hidden}
      panelId={panelId}
      tabId={tabId}
    />
  );
}

function TabPanel({
  api,
  tableKey,
  document,
  workspaceName,
  hidden,
  panelId,
  tabId,
}: {
  api: ApiClient;
  tableKey: string;
  document: TableConfigDocument;
  workspaceName: string;
  hidden: boolean;
  panelId: string;
  tabId: string;
}) {
  // `fromDocument` restores the constants the schema dropped — `apiPath`,
  // `apiAction`, `sortColumnTableName` — so what the hook consumes is the same
  // `TableConfig` the query builder has always taken.
  const config = useMemo(() => fromDocument(tableKey, document), [tableKey, document]);
  const context = useMemo(
    () => ({ routeParams: { workspace_name: workspaceName } }),
    [workspaceName],
  );
  const fetcher = useMemo(() => (payload: Record<string, unknown>) => api.dataTable(payload), [api]);
  const state = useDataTable({ config, context, fetcher });

  return (
    <div id={panelId} role="tabpanel" aria-labelledby={tabId} hidden={hidden}>
      <DataTable config={config} state={state} />
    </div>
  );
}

export function CompiledView({ api, document, workspaceName }: CompiledViewProps) {
  const [active, setActive] = useState(0);

  return (
    <div className="compiled-view">
      <div className="tabbar" role="tablist" aria-label={`${document.label} views`}>
        {document.tabs.map((tab, i) => (
          <div key={tab.table} className={`tab${i === active ? " is-active" : ""}`}>
            <button
              type="button"
              id={`cv-tab-${tab.table}`}
              className="tab-label"
              role="tab"
              aria-selected={i === active}
              aria-controls={`cv-panel-${tab.table}`}
              onClick={() => setActive(i)}
            >
              {tab.label}
            </button>
          </div>
        ))}
      </div>
      {document.tabs.map((tab, i) => (
        <ViewTab
          key={tab.table}
          api={api}
          tableKey={tab.table}
          workspaceName={workspaceName}
          hidden={i !== active}
          panelId={`cv-panel-${tab.table}`}
          tabId={`cv-tab-${tab.table}`}
        />
      ))}
    </div>
  );
}
