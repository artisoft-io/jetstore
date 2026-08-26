/**
 * A Workspace IDE section's compiled view, rendered. Tasks C.3 and C.3b.
 *
 * The heading of a section whose files compile into `workspace.db` opens one of
 * these instead of an editor: a strip of tabs, one data table each, querying the
 * compiled artifact rather than listing the sources beneath it. It is the second
 * half of the duality C.1 made explicit on the wire, and the half the React app
 * had none of — `FileTree` treated a `section` node as a plain directory and
 * `WorkspaceNode.compiled_view` was read by nothing.
 *
 * ## Two kinds of tab, and the split is measured rather than assumed
 *
 * Six of the eight tables are **action-less** in the corpus — `actions: []` and
 * `secondRowActions: []` on every one — and none is a field of a form. So there
 * is no selection to publish, no button to gate and no form state to read, which
 * is exactly the case `useDataTable`'s own header says it was written for and the
 * same call `TableScreen.tsx` makes for C.7 and C.8.
 *
 * **The other two are C.3b's**, and they are the last tabs of the Data Model and
 * Jets Rules views: `wsDataModelFilesTable` and `wsJetRulesFilesTable` list the
 * *source files* of their section out of the same `workspace_control` table,
 * separated only by a `LIKE` prefix, and each carries *Add File* (a dialog) and
 * *Delete* (gated on a selected row). Those need a selection published into form
 * state, a dialog host and an action document — the whole of what `TableView`
 * and C.2b's `FormDialog` provide — so they render through a different component.
 *
 * **The branch is on the document, not on a list of keys.** `hasActionBar` asks
 * whether the table document declares any action; a table that grows one is drawn
 * bound without anything here being edited, and a list of two keys would have had
 * to be kept in step with the documents by hand.
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
 * **C.3b puts it in form state as well, and that is not the same thing twice.**
 * The two writes serve different readers: `routeParams` is what the *query* is
 * built from, and `workspace_name` in group 0 is what the *action document* reads
 * for its `workspaceName` envelope extra. Both server actions demand it —
 * `AddWorkspaceFile` and `DeleteWorkspaceFile` fail with *"missing
 * workspace_name"* — and neither can see a route parameter.
 *
 * ## The tree is refreshed by this screen, not by the action document
 *
 * Both `add_workspace_file` and `delete_workspace_files` answer with the
 * recomputed workspace file structure rather than with rows: each ends by
 * reassigning `dataTableAction.Action = "workspace_query_structure"` and
 * returning `WorkspaceQueryStructure`'s body. The Flutter app consumes that
 * directly — `JetsRouterDelegate().workspaceMenuState = mapMenuEntry(l)` in the
 * `deleteWorkspaceFiles` and `addWorkspaceFilesOk` arms of
 * `jetsclient/lib/modules/workspace_ide/screen_delegates.dart`.
 *
 * **This app cannot, and the reason is a property of the grammar rather than an
 * omission.** `ActionHost.post` returns `{ statusCode }` and discards the body
 * (`WorkspaceRegistry.tsx` and this file both implement it that way), so no
 * `post` step can hand a file tree to anything. Widening it would make every
 * action document a potential reader of every response shape, which is a much
 * larger change than this needs.
 *
 * So the host tells the screen that the workspace's files moved and the screen
 * re-reads the tree — `onFilesChanged`, wired to `WorkspaceIde`'s existing
 * `fileTree` call. **One extra round trip, deliberately**: the alternative buys a
 * saved request and costs the grammar a response channel.
 *
 * ## Every tab is mounted, and only one is shown
 *
 * `TabBarView` builds all of its children (`screen_tab_form.dart`,
 * `List<Widget>.generate`), so the Flutter screen issues every tab's query when
 * the view opens. Mounting all of them here reproduces that and buys the thing
 * that matters more: a hook may not be called in a loop whose length changes, and
 * paging and sorting are widget state that would be thrown away by unmounting the
 * tab a user just paged.
 *
 * **The dialog is rendered outside the panels for the same reason.** A panel is
 * hidden with the `hidden` attribute, which hides everything inside it — so a
 * `<dialog>` rendered within the tab that opened it would be invisible the moment
 * the user switched tabs, and `showModal` on a hidden subtree is not a state this
 * app should be able to reach. One dialog per view, at the view's level.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ApiError, type ApiClient } from "../api/client";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { describeUnresolved, resolveEscapes } from "../actions/escapes";
import { productionRegistry } from "../actions/registry";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import type { ActionRequest } from "../datatable/actionDispatch";
import { DataTable } from "../datatable/DataTable";
import { FormState } from "../datatable/formState";
import type { TableConfigDocument } from "../datatable/table";
import { TableConfigDocumentSchema, tableEscapeReferences } from "../datatable/table";
import { TableView } from "../datatable/TableView";
import { fromDocument } from "../datatable/tableTranslate";
import type { ActionConfig, JetsRow } from "../datatable/types";
import { useDataTable } from "../datatable/useDataTable";
import { useNotifications } from "../shell/notifications";
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormDocumentSchema, type Form, type FormAction, type FormDocument } from "../userflow/form";
import { formEscapeReferences } from "../userflow/store";
import { validateAllGroups, type FieldError } from "../userflow/validateForm";
import type { CompiledViewDocument } from "./compiledView";

import actionsJson from "./documents/compiledView.ua.json";
import formsJson from "./documents/compiledView.form.json";

import wsDataModelFilesTable from "../datatable/tables/wsDataModelFilesTable.tc.json";
import wsDataPropertyTable from "../datatable/tables/wsDataPropertyTable.tc.json";
import wsDomainClassTable from "../datatable/tables/wsDomainClassTable.tc.json";
import wsDomainTableTable from "../datatable/tables/wsDomainTableTable.tc.json";
import wsJetRulesFilesTable from "../datatable/tables/wsJetRulesFilesTable.tc.json";
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
  // C.3b's two: the only tables of the eight that carry an action bar.
  wsDataModelFilesTable: wsDataModelFilesTable as TableConfigDocument,
  wsJetRulesFilesTable: wsJetRulesFilesTable as TableConfigDocument,
};

/** The capability both of C.3b's actions need; `admin` bypasses it, as the server does. */
export const WORKSPACE_IDE = "workspace_ide";

/**
 * One group, shared by the view and its dialog.
 *
 * C.2b's reasoning unchanged: a dialog's fields are seeded from what the table
 * published, and a second group would mean copying the selection across before
 * every dialog and back after.
 */
const GROUP = 0;

/**
 * The form-state key the action document reads for its `workspaceName` extra.
 *
 * `FSK.wsName` on the Dart side (`jetsclient/lib/utils/constants.dart`, `wsName`),
 * and the same string here so a document ported from a Dart delegate keeps its
 * key names.
 */
const WORKSPACE_NAME_KEY = "workspace_name";

/** Whether a table document declares an action, which is what decides how a tab is drawn. */
export function hasActionBar(document: TableConfigDocument): boolean {
  if (document.source !== "query") return false;
  return (document.actions?.length ?? 0) > 0 || (document.secondRowActions?.length ?? 0) > 0;
}

/**
 * The two documents C.3b's tabs need, parsed once at module load.
 *
 * **Parsed rather than cast**, on `WorkspaceRegistry`'s terms: a document that
 * does not satisfy its schema fails here, in every test that imports this module,
 * rather than when a user presses a button.
 */
const parsed = {
  forms: FormDocumentSchema.safeParse(formsJson),
  actions: ActionDocumentSchema.safeParse(actionsJson),
};

/** Findings from parsing and from resolving escape names, for the banner. */
export function documentFindings(): string[] {
  const findings: string[] = [];
  for (const [name, result] of Object.entries(parsed)) {
    if (!result.success) {
      for (const issue of result.error.issues) {
        findings.push(`${name}: ${issue.path.join("/")} — ${issue.message}`);
      }
    }
  }
  // Every bundled table document is parsed too, which the C.3 half of this file
  // did not do: those six were only ever cast. A table whose document stopped
  // fitting its schema would have rendered an empty strip.
  for (const [key, document] of Object.entries(COMPILED_VIEW_TABLES)) {
    const result = TableConfigDocumentSchema.safeParse(document);
    if (!result.success) {
      for (const issue of result.error.issues) {
        findings.push(`${key}: ${issue.path.join("/")} — ${issue.message}`);
      }
    }
  }
  if (findings.length > 0) return findings;
  const references = [
    ...formEscapeReferences(parsed.forms.data as FormDocument, "compiledView"),
    ...Object.entries(COMPILED_VIEW_TABLES).flatMap(([key, document]) =>
      tableEscapeReferences(document).map((r) => ({ ...r, at: `${key}.tc.json${r.at}` })),
    ),
  ];
  for (const [name, action] of Object.entries((parsed.actions.data as ActionDocument).actions)) {
    action.steps.forEach((s, index) => {
      if (s.do === "escape") {
        references.push({ kind: "actions", name: s.name, at: `/actions/${name}/steps/${index}` });
      }
      if (s.do === "query") {
        references.push({ kind: "queries", name: s.name, at: `/actions/${name}/steps/${index}` });
      }
    });
  }
  const message = describeUnresolved(resolveEscapes(references, productionRegistry));
  return message === null ? [] : [message];
}

export interface CompiledViewProps {
  api: ApiClient;
  document: CompiledViewDocument;
  /** The workspace whose `workspace.db` every tab reads. */
  workspaceName: string;
  /**
   * Called after an action that adds or removes a workspace file. Task C.3b.
   *
   * Optional because the six action-less views never fire it, and a caller that
   * renders only those owes nothing. See the header for why the screen re-reads
   * the tree rather than the action document consuming the response.
   */
  onFilesChanged?: () => void;
}

/** A tab whose table has no action bar: C.3's six, unchanged. */
function ReadOnlyTabPanel({
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

/** A tab whose table has an action bar: C.3b's two. */
function BoundTabPanel({
  api,
  tableKey,
  document,
  workspaceName,
  formState,
  onAction,
  hidden,
  panelId,
  tabId,
}: {
  api: ApiClient;
  tableKey: string;
  document: TableConfigDocument;
  workspaceName: string;
  formState: FormState;
  onAction(request: ActionRequest, action: ActionConfig): void;
  hidden: boolean;
  panelId: string;
  tabId: string;
}) {
  const config = useMemo(() => fromDocument(tableKey, document), [tableKey, document]);
  // **Keyed on the workspace name and nothing else — I-184.** `useTableBinding`
  // takes `context` as a dependency of the query it builds, so a new object
  // identity is a refetch; rebuilding it on every form-state change would refetch
  // on every *selection* and then discard the selection, because the restore is
  // guarded on the row set having changed and it has not.
  const context = useMemo(
    () => ({ routeParams: { workspace_name: workspaceName } }),
    [workspaceName],
  );
  const fetcher = useMemo(() => (payload: Record<string, unknown>) => api.dataTable(payload), [api]);

  return (
    <div id={panelId} role="tabpanel" aria-labelledby={tabId} hidden={hidden}>
      <TableView
        config={config}
        field={{ group: GROUP, key: tableKey }}
        formState={formState}
        fetcher={fetcher}
        context={context}
        predicates={productionRegistry.predicates}
        cellFilters={{}}
        onAction={onAction}
      />
    </div>
  );
}

export function CompiledView({ api, document, workspaceName, onFilesChanged }: CompiledViewProps) {
  const [active, setActive] = useState(0);
  const { setError, setStatus } = useNotifications();

  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  /** Bumped on every form-state change so the view re-renders with it. */
  const [, setStateVersion] = useState(0);

  const formState = useMemo(() => new FormState(), []);
  /** Whether the last action stopped because the user stopped it — **I-186**. */
  const haltedByUser = useRef(false);
  const dialog = useFormDialog();

  const findings = useMemo(documentFindings, []);
  const forms = parsed.forms.success ? parsed.forms.data.forms : {};
  const actions = parsed.actions.success ? parsed.actions.data.actions : {};

  useEffect(() => formState.subscribe(() => setStateVersion((n) => n + 1)), [formState]);

  /**
   * The workspace the action document posts against — see the header.
   *
   * Written on mount and whenever the picker moves. `notifyListeners` is not
   * called: this runs before any action can, and a render is already scheduled by
   * the state change that brought the new name in.
   */
  useEffect(() => {
    formState.setValue(GROUP, WORKSPACE_NAME_KEY, workspaceName);
  }, [formState, workspaceName]);

  const fetcher = useCallback((payload: Record<string, unknown>) => api.dataTable(payload), [api]);
  const currentForm: Form | null =
    dialog.request === null ? null : (forms[dialog.request.form] ?? null);

  const host: ActionHost = useMemo(
    () => ({
      query: async () => {
        // No step of this view's action document is a `query`, and
        // `documentFindings` resolves every `query` name at load — so reaching
        // this is a document that changed without its host.
        throw new Error("compiledView's actions declare no query step");
      },
      validate: () => {
        if (currentForm === null) return true;
        const found = validateAllGroups(currentForm, formState, GROUP);
        setErrors(found);
        if (found.length > 0) haltedByUser.current = true;
        return found.length === 0;
      },
      confirm: async (message: string) => {
        const agreed = window.confirm(message);
        if (!agreed) haltedByUser.current = true;
        return agreed;
      },
      post: async (request): Promise<PostResult> => {
        try {
          await api.endpoint(request.endpoint, request.body);
          return { statusCode: 200 };
        } catch (error) {
          if (error instanceof ApiError) return { statusCode: error.status, error: error.message };
          return { statusCode: 500, error: error instanceof Error ? error.message : String(error) };
        }
      },
      read: async (request) => {
        try {
          const body = await api.endpoint<{ rows?: unknown }>(request.endpoint, request.body);
          return Array.isArray(body.rows) ? (body.rows as JetsRow[]) : [];
        } catch {
          return null;
        }
      },
      download: () => {
        throw new Error("compiledView's actions declare no download");
      },
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      // A view has no states and no screen to leave; `close` dismisses the open
      // dialog, which is the nearest thing. C.2b's reasoning, unchanged.
      goToState: () => {},
      close: () => dialog.close("ok"),
      navigate: () => {
        throw new Error("compiledView's actions declare no navigation");
      },
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [api, currentForm, formState, setError, setStatus, dialog.close],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
      haltedByUser.current = false;
      const action = actions[name];
      if (action === undefined) {
        throw new Error(`action "${name}" is not in this view's action document`);
      }
      return runAction({
        action,
        host,
        formState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: "compiledView",
      });
    },
    [actions, host, formState],
  );

  /**
   * Both of this view's actions move workspace files, so both refresh the tree.
   *
   * The selection is cleared with them: the rows it names are gone after a delete
   * and the table is about to refetch, and a selection that survives its rows is
   * what gates the Delete button on a file that no longer exists.
   */
  const afterFileAction = useCallback(() => {
    formState.setValue(GROUP, "source_file_name", []);
    formState.requestRefresh();
    onFilesChanged?.();
  }, [formState, onFilesChanged]);

  /** Opens a dialog and waits for it **without holding `busy`** — a modal is not work (C.2b). */
  const openAndRefresh = useCallback(
    async (form: string, _params: Record<string, string>) => {
      setBusy(false);
      const outcome = await dialog.open({ form, params: _params });
      if (outcome === "ok") afterFileAction();
    },
    [dialog, afterFileAction],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      void (async () => {
        setError(null);
        setBusy(true);
        try {
          switch (request.kind) {
            case "runAction": {
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) {
                setError(outcome);
                return;
              }
              // A user who cancelled the confirmation changed nothing, so the
              // tree is not re-read and the selection stands.
              if (haltedByUser.current) return;
              afterFileAction();
              return;
            }
            case "openDialog": {
              for (const [key, value] of Object.entries(request.params)) {
                formState.setValue(GROUP, key, value);
              }
              formState.notifyListeners();
              setErrors([]);
              await openAndRefresh(request.form, request.params);
              return;
            }
            case "navigate":
            case "runActionThenDialog":
            case "escape":
            case "promptFilter":
              // None of the eight table documents configures any of these, and
              // `documentFindings` cannot see an action type — so this reports
              // rather than being silently absent, which is C.2b's rule.
              setError(`"${action.label}" asks for ${request.kind}, which a compiled view does not serve`);
              return;
          }
        } catch (error) {
          setError(error instanceof Error ? error.message : String(error));
        } finally {
          setBusy(false);
        }
      })();
    },
    [afterFileAction, formState, openAndRefresh, runNamedAction, setError],
  );

  /** A dialog's button. Cancel is the dialog's; everything else is the document's. */
  const onFormAction = useCallback(
    (action: FormAction) => {
      if (isDialogCancel(action)) {
        dialog.close("cancel");
        return;
      }
      void (async () => {
        setBusy(true);
        try {
          const outcome = await runNamedAction(action.action);
          if (outcome !== null) {
            setError(outcome);
            dialog.close("failed");
            return;
          }
          // The dialog stays open when the user stopped it — the field errors
          // `validate` produced are on screen and the form is still there.
          if (haltedByUser.current) return;
          dialog.close("ok");
        } catch (error) {
          setError(error instanceof Error ? error.message : String(error));
          dialog.close("failed");
        } finally {
          setBusy(false);
        }
      })();
    },
    [dialog, runNamedAction, setError],
  );

  if (findings.length > 0) {
    return (
      <div className="compiled-view">
        <div className="banner banner-error" role="alert">
          <div>
            <p>This view&apos;s documents cannot be loaded.</p>
            <ul>
              {findings.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    );
  }

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
      {document.tabs.map((tab, i) => {
        const table = COMPILED_VIEW_TABLES[tab.table];
        // A tab naming a table this app does not bundle is an authoring error in
        // the view document, and it says so rather than rendering an empty table:
        // the schema can check that `table` is an identifier and not that it is
        // *this* identifier, so the check has to be here.
        if (table === undefined) {
          throw new Error(`compiled view names table "${tab.table}", which is not bundled`);
        }
        const shared = {
          key: tab.table,
          api,
          tableKey: tab.table,
          document: table,
          workspaceName,
          hidden: i !== active,
          panelId: `cv-panel-${tab.table}`,
          tabId: `cv-tab-${tab.table}`,
        };
        return hasActionBar(table) ? (
          <BoundTabPanel {...shared} formState={formState} onAction={onTableAction} />
        ) : (
          <ReadOnlyTabPanel {...shared} />
        );
      })}
      {currentForm !== null && (
        <FormDialog
          form={currentForm}
          errors={errors}
          onDismiss={() => dialog.close("cancel")}
          host={{
            formState,
            group: GROUP,
            // No field of `addWorkspaceFile` names a `dropdownItemsQuery`, so
            // nothing asks and the answer is always "no rows for that name".
            queryRows: () => undefined,
            queriesLoading: false,
            tableConfig: (key: string) => fromDocument(key, COMPILED_VIEW_TABLES[key]!),
            fetcher,
            tableContext: { routeParams: { workspace_name: workspaceName } },
            predicates: productionRegistry.predicates,
            cellFilters: () => ({}),
            onTableAction,
            onFormAction,
            formValid: true,
            groupCount: 1,
            busy,
          }}
        />
      )}
    </div>
  );
}
