/**
 * The workspace registry's documents and the host that runs their actions.
 *
 * **Two screens press these buttons.** `/workspaces` draws them on the registry
 * table, against the row a user selected; the Workspace IDE draws the ones that
 * need a workspace in its toolbar, against the open workspace's registry row
 * (`workspaceToolbar.ts`). Both run the same `workspaceRegistry.ua.json` actions
 * and open the same `workspaceRegistry.form.json` dialogs, so the host lives
 * here rather than in either screen. Moved out of `WorkspaceRegistry.tsx` on
 * 2026-10-11, when the IDE became the second caller; the reasoning recorded in
 * the comments below was written for the registry and holds for both.
 */

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { productionRegistry } from "../actions/registry";
import { ActionDocumentSchema, type ActionDocument } from "../actions/schema";
import { describeUnresolved, resolveEscapes } from "../actions/escapes";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import { fromDocument } from "../datatable/tableTranslate";
import { TableConfigDocumentSchema, tableEscapeReferences } from "../datatable/table";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig, JetsRow, QueryContext, TableConfig } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import { usePrompts } from "../shell/prompts";
import { inAppPath, unservedScreenMessage, withReturnTo } from "./routes";
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormDocumentSchema, type Form, type FormAction, type FormDocument } from "../userflow/form";
import { formEscapeReferences } from "../userflow/store";
import { resolveQuery } from "../userflow/formQueries";
import { useFormQueries } from "../userflow/useFormQueries";
import { validateAllGroups, type FieldError } from "../userflow/validateForm";

import actionsJson from "./documents/workspaceRegistry.ua.json";
import formsJson from "./documents/workspaceRegistry.form.json";
import tableJson from "../datatable/tables/workspaceRegistryTable.tc.json";

/** The table's key, which is also the form-state key its selection is published under. */
export const TABLE_KEY = "workspaceRegistryTable";

/**
 * One group, as every non-repeating form uses.
 *
 * The screen and its dialogs share it deliberately: a dialog's fields are seeded
 * from the row the table published, and a second group would mean copying the
 * selection across before every dialog and back after. The Dart shares one
 * `JetsFormState` between the screen and its dialogs for the same reason
 * (`components/data_table.dart`, `actionDispatcher`).
 */
export const GROUP = 0;

/**
 * The three documents, parsed once at module load.
 *
 * **Parsed rather than cast**, which is the point of bundling them as JSON: a
 * document that does not satisfy its schema fails here, in every test that
 * imports this module, rather than at the moment a user presses a button. A cast
 * would make the bundling a way of skipping the check.
 */
export const parsed = {
  forms: FormDocumentSchema.safeParse(formsJson),
  actions: ActionDocumentSchema.safeParse(actionsJson),
  table: TableConfigDocumentSchema.safeParse(tableJson),
};

/** The registry table's configuration, or null when its document does not parse. */
export const registryTableConfig: TableConfig | null = parsed.table.success
  ? fromDocument(TABLE_KEY, parsed.table.data)
  : null;

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
  if (findings.length > 0) return findings;
  const references = [
    ...formEscapeReferences(parsed.forms.data as FormDocument, "workspaceRegistry"),
    ...tableEscapeReferences(parsed.table.data!).map((r) => ({
      ...r,
      at: `workspaceRegistryTable.tc.json${r.at}`,
    })),
  ];
  for (const step of Object.entries((parsed.actions.data as ActionDocument).actions)) {
    step[1].steps.forEach((s, index) => {
      if (s.do === "escape") {
        references.push({ kind: "actions", name: s.name, at: `/actions/${step[0]}/steps/${index}` });
      }
      if (s.do === "query") {
        references.push({ kind: "queries", name: s.name, at: `/actions/${step[0]}/steps/${index}` });
      }
    });
  }
  // `describeUnresolved` takes the whole list and returns one message or null —
  // the same shape `FlowStore` shows for a flow, so a screen's failure and a
  // flow's read alike.
  const message = describeUnresolved(resolveEscapes(references, productionRegistry));
  return message === null ? [] : [message];
}

/** The query context fields a dialog's tables are built with. */
export type RegistryTableContext = Pick<QueryContext, "routeParams" | "homeFilters" | "dataRegistryFilters">;

export interface RegistryActionHostOptions {
  api: ApiClient;
  /** The query context the registry's dialogs are built with. */
  tableContext: RegistryTableContext;
  /**
   * Called wherever the registry asks its table to refresh — after an action
   * ran and after a dialog closed on OK. The registry's table refreshes through
   * `formState.requestRefresh()`; a caller with no table re-reads what it shows.
   */
  afterAction?: () => void;
}

export interface RegistryActionHost {
  /** The form state the selection is published into and the dialogs read. */
  formState: FormState;
  /** Whether an action is in flight. */
  busy: boolean;
  /** The bar's `onAction`. */
  onAction(request: ActionRequest, action: ActionConfig): void;
  /** The dialog an action opened, or null. Render it once, anywhere in the screen. */
  dialog: ReactNode;
}

export function useRegistryActionHost({
  api,
  tableContext,
  afterAction,
}: RegistryActionHostOptions): RegistryActionHost {
  const navigate = useNavigate();
  /** Where a flow opened from this screen should return to (D.8). */
  const location = useLocation();
  const here = `${location.pathname}${location.search}`;
  const { setError, setStatus } = useNotifications();
  const prompts = usePrompts();

  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  /**
   * Bumped on every form-state change so the screen re-renders with it.
   *
   * The same device `FlowRunner` uses and for the same reason: `FormState` is
   * mutable and keeps its identity, so nothing else tells React that a selection
   * was published or a field typed into.
   */
  const [, setStateVersion] = useState(0);

  const formState = useMemo(() => new FormState(), []);
  /**
   * Whether the last action stopped because the *user* stopped it. **I-186.**
   *
   * `runAction` returns `string | null`, and `null` means two different things:
   * the action ran to the end, or a `validate` step failed / a `confirm` was
   * refused — both of which return `{ done: true, outcome: null }`
   * (`actions/interpret.ts`, the `validate` and `confirm` arms). A flow runner
   * does not need to tell them apart, because the *engine* validates separately
   * and simply does not advance. **A dialog host does**: treating a failed
   * validation as success closes the dialog over the messages it just produced,
   * which is what the first version of this screen did — the required-field
   * error rendered for one frame into a dialog that was being unmounted.
   *
   * Rather than widen `runAction`'s return type — an exported contract another
   * project reads — the host records what it was asked. It is the host's own
   * answer being remembered, which is why a ref is honest here and not a
   * workaround.
   */
  const haltedByUser = useRef(false);
  const dialog = useFormDialog();

  const forms = parsed.forms.success ? parsed.forms.data.forms : {};
  const actions = parsed.actions.success ? parsed.actions.data.actions : {};
  const config = registryTableConfig;

  useEffect(() => formState.subscribe(() => setStateVersion((n) => n + 1)), [formState]);

  const refresh = useCallback(() => {
    formState.requestRefresh();
    afterAction?.();
  }, [formState, afterAction]);

  const fetcher: DataTableFetcher = useCallback((payload) => api.dataTable(payload), [api]);
  const queryPost = useCallback(
    (payload: Record<string, unknown>) =>
      api.dataTable<{ result_map?: Record<string, unknown> }>(payload),
    [api],
  );

  const currentForm: Form | null =
    dialog.request === null ? null : (forms[dialog.request.form] ?? null);
  const queries = useFormQueries(currentForm, formState, queryPost);

  const host: ActionHost = useMemo(
    () => ({
      query: async (name: string) => {
        const registered = productionRegistry.queries[name];
        if (registered === undefined) throw new Error(`named query "${name}" is not registered`);
        const sql = resolveQuery(
          { sql: registered.sql, ...(registered.params ? { params: [...registered.params] } : {}) },
          formState,
        );
        if (sql === null) return null;
        const body = await queryPost({ action: "raw_query_map", query_map: { [name]: sql } });
        const rows = body.result_map?.[name];
        if (!Array.isArray(rows) || rows.length === 0) return null;
        const first = rows[0] as (string | null)[];
        return Object.fromEntries(
          registered.columns.map((column, index) => [column, first[index] ?? null]),
        );
      },
      validate: () => {
        if (currentForm === null) return true;
        const found = validateAllGroups(currentForm, formState, GROUP);
        setErrors(found);
        if (found.length > 0) haltedByUser.current = true;
        return found.length === 0;
      },
      confirm: async (message: string) => {
        const agreed = await prompts.confirm(message);
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
      download: (fileName, content) => {
        const url = URL.createObjectURL(new Blob([content], { type: "text/csv" }));
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = fileName;
        anchor.click();
        URL.revokeObjectURL(url);
      },
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      // **A screen has no states, so these two are the shape of the difference.**
      // `goToState` is a flow's transition and there is nothing here to go to;
      // `close` on a flow leaves it, and on a screen the nearest thing is
      // dismissing the dialog that is open. A dialog's action document says
      // nothing about either, so both are inert rather than wrong.
      goToState: () => {},
      close: () => dialog.close("ok"),
      // C.3's route, reached without a page reload. The cross-*app* link a
      // `showScreen` action makes is still `window.location.href` below; this is
      // for a destination inside this bundle, and `openWorkspace` is the one
      // escape that has one.
      navigate: (path: string) => navigate(path),
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [api, currentForm, formState, navigate, queryPost, setError, setStatus, dialog.close, prompts],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
      haltedByUser.current = false;
      const action = actions[name];
      if (action === undefined) {
        throw new Error(`action "${name}" is not in this screen's action document`);
      }
      return (await runAction({
        action,
        host,
        formState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: "workspaceRegistry",
      })).message;
    },
    [actions, host, formState],
  );

  /**
   * Opens a dialog and waits for it, **without holding `busy`**.
   *
   * `busy` means *an action is in flight*, and every button in the app reads it —
   * including the dialog's own. Holding it across the dialog's lifetime, which is
   * what awaiting inside the `try` did, disabled every button in the form the
   * dialog had just opened: the OK button could not be pressed, so the dialog
   * could not be closed, so `busy` was never released. **A modal is not work.**
   *
   * Found by the first rendering test of the dialog host and worth the paragraph
   * rather than the one-line fix, because the shape recurs: `setBusy(true)` in a
   * `try` and `setBusy(false)` in its `finally` is right for a request and wrong
   * for anything that waits on the user.
   */
  const openAndRefresh = useCallback(
    async (form: string, params: Record<string, string>) => {
      setBusy(false);
      const outcome = await dialog.open({ form, params });
      if (outcome === "ok") refresh();
    },
    [dialog, refresh],
  );

  /**
   * A table action's button.
   *
   * The request kinds a screen sees. `promptFilter` cannot arrive — this table
   * has no filter buttons — and reaching it would be a configuration this screen
   * does not have, so it reports rather than being silently absent.
   */
  const onAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      void (async () => {
        setError(null);
        setBusy(true);
        try {
          switch (request.kind) {
            case "runAction": {
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) setError(outcome);
              refresh();
              return;
            }
            case "navigate": {
              // **Both of this table's `configScreenPath`s are flows** — *Load
              // Client Config* and *Pull Workspace* — and until X.1 this handed
              // the browser to the Flutter app to serve them. There is no other
              // app now, so the same two templates resolve in-app through the map
              // ported out of the Dart before it was deleted.
              const internal = inAppPath(action.configScreenPath, request.params);
              if (internal !== null) {
                // D.8: both of these are flows, so both come back here.
                void navigate(withReturnTo(internal, here));
                return;
              }
              setError(unservedScreenMessage(action.label, request.path));
              return;
            }
            case "openDialog": {
              // **The dialog host, I-68's consumer.** The params the action
              // resolved are seeded into the shared group before the form opens,
              // because the dialog's fields read them by key — that is what makes
              // `navigationParams` mean anything.
              for (const [key, value] of Object.entries(request.params)) {
                formState.setValue(GROUP, key, value);
              }
              formState.notifyListeners();
              setErrors([]);
              await openAndRefresh(request.form, request.params);
              return;
            }
            case "runActionThenDialog": {
              const outcome = await runNamedAction(request.name);
              if (outcome !== null) {
                setError(outcome);
                return;
              }
              for (const [key, value] of Object.entries(request.params)) {
                formState.setValue(GROUP, key, value);
              }
              formState.notifyListeners();
              await openAndRefresh(request.form, request.params);
              return;
            }
            case "escape": {
              const escape = productionRegistry.actions[request.name];
              if (escape === undefined) {
                setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
                return;
              }
              await escape({ formState, group: GROUP, flowKey: "workspaceRegistry" }, host);
              refresh();
              return;
            }
            case "promptFilter":
              setError(`"${action.label}" asks for a filter, which this screen does not configure`);
              return;
          }
        } catch (error) {
          setError(error instanceof Error ? error.message : String(error));
        } finally {
          setBusy(false);
        }
      })();
    },
    [formState, here, host, navigate, openAndRefresh, refresh, runNamedAction, setError],
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
          // The dialog stays open when the user stopped it — see `haltedByUser`.
          // The field errors `validate` produced are on screen and the form is
          // still there to correct.
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

  const dialogElement =
    currentForm !== null && config !== null ? (
      <FormDialog
        form={currentForm}
        errors={errors}
        onDismiss={() => dialog.close("cancel")}
        host={{
          formState,
          group: GROUP,
          queryRows: queries.rows,
          queriesLoading: queries.loading,
          tableConfig: () => config,
          fetcher,
          tableContext,
          predicates: productionRegistry.predicates,
          cellFilters: () => ({}),
          onTableAction: onAction,
          onFormAction,
          formValid: true,
          groupCount: 1,
          busy,
        }}
      />
    ) : null;

  return { formState, busy, onAction, dialog: dialogElement };
}
