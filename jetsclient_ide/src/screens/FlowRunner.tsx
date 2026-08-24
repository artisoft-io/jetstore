/**
 * The flow runner. Task F.0a, and the thing I-50 found nobody had built.
 *
 * Phase 2 built a schema, a validator, an interpreter, an engine, a store and
 * two proof flows' documents, and `proofFlows.test.ts` drove them end to end in
 * memory. **No browser ran a flow**, because there was no `/flow/:key` route, no
 * screen that read a `.form.json`, and no `EscapeRegistry` value outside tests.
 * The Flutter app was meanwhile handing users to `/ide/flow/<key>` for two flows,
 * which landed on `App.tsx`'s `path="*"` and redirected to the Workspace IDE.
 *
 * ## This screen is a host, and that is the whole design
 *
 * Everything with a decision in it was already written and tested without a DOM:
 * `engine.step` decides where the flow goes, `runAction` decides what a button
 * does, `validateForm` decides whether the form passes, `FlowStore.load` decides
 * whether the documents are runnable at all. What was missing is the
 * *implementation of the seams* — `ActionHost`'s ten methods and `StepRequest`'s
 * four — against a real API, a real router and a real user.
 *
 * So this file is deliberately close to `proofFlows.test.ts`'s `harness`. Where
 * the test supplies `post: async (r) => { posts.push(r); return {statusCode: 200} }`,
 * this supplies the api client. If the two ever disagree about *ordering* the
 * test is right, because the ordering is `engine.ts`'s and is asserted there.
 *
 * ## Which workspace
 *
 * The deployment's active workspace, from `get_workspace_uri` — not the IDE's
 * picker. A user editing four workspaces still runs the flows of the one this
 * apiserver is configured for; `WorkspaceApi.activeWorkspace` explains why the
 * registry is the wrong question here.
 *
 * ## What this does not do yet, stated rather than hidden
 *
 * A table action of kind `showDialog` or `doActionShowDialog` opens a *dialog*,
 * and this app has no dialog host — five of the 25 configured table actions are
 * one of those two kinds and none is on a proof flow's tables. They report
 * plainly rather than doing nothing, which is the same choice `escapes.ts` made
 * about an unresolved name (I-68).
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { WorkspaceApi } from "../api/workspace";
import { runAction, type ActionHost, type PostResult } from "../actions/interpret";
import { productionRegistry, setFileKeyLabelPattern } from "../actions/registry";
import {
  currentDataRegistryFilters,
  currentHomeFilters,
  setIdFilter,
} from "../actions/homeFilters";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import { FormRenderer } from "../userflow/FormRenderer";
import type { FormAction } from "../userflow/form";
import { useFormQueries } from "../userflow/useFormQueries";
import {
  advance,
  isStandardAction,
  startAt,
  step,
  type FlowPosition,
  type StandardAction,
} from "../userflow/engine";
import { FlowLoadError, FlowStore, tableConfigOf, type LoadedFlow } from "../userflow/store";
import {
  isWholeFormValid,
  validateAllGroups,
  type FieldError,
  type FormValidatorContext,
} from "../userflow/validateForm";

/** The capability the server requires for every workspace read. */
export const FLOW_RUNNER = "workspace_ide";

/**
 * The form-state group a flow runs in.
 *
 * Zero, and every flow in the corpus is zero: the group dimension is for a form
 * with repeating rows, which is `file_mapping`'s mechanism (I-31, I.1) and not a
 * flow-level one. A flow that grows one will size it from a query at load, which
 * is what `resizeFormState` is for.
 */
const GROUP = 0;

interface Loaded {
  workspaceName: string;
  flow: LoadedFlow;
}

export function FlowRunner({ api }: { api: ApiClient }) {
  const { key } = useParams<{ key: string }>();
  const [search] = useSearchParams();
  const navigate = useNavigate();
  const { setError, setStatus } = useNotifications();

  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [loadFindings, setLoadFindings] = useState<string[] | null>(null);
  const [position, setPosition] = useState<FlowPosition | null>(null);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [busy, setBusy] = useState(false);
  /**
   * Bumped whenever the form state changes, so this screen re-renders with it.
   *
   * **Two things below are computed from the store rather than from React state,
   * and both were silently stale without this.** `formValid` decides whether a
   * button is enabled, and the Dart gets that from `setValueAndNotify` rebuilding
   * the form (`components/form_button.dart` reads `formState.isFormValid()` on
   * every build); and `groupCount` decides how many rows a repeating form draws,
   * which changes when the query returns and `resizeFormState` runs.
   *
   * Added by F.1 because it is the first form where both matter: `mapperOk` and
   * `mapperDraft` swap as the user types, and until F.1 no form on this screen
   * gated a button on validity at all.
   */
  const [stateVersion, setStateVersion] = useState(0);

  // One form state for the whole flow, deliberately: a flow's states share their
  // values — `loadFilesUF`'s second table filters on the first's selection — and
  // a per-state store would lose them at every transition.
  const formState = useMemo(() => new FormState(), [key]); // eslint-disable-line react-hooks/exhaustive-deps
  const workspaceApi = useMemo(() => new WorkspaceApi(api), [api]);

  useEffect(() => formState.subscribe(() => setStateVersion((n) => n + 1)), [formState]);

  /**
   * The flow's parameters, seeded into group 0 before anything reads them.
   *
   * **A flow can need arguments and until F.1 this route could not carry any.**
   * `mapFileUF` is served in Flutter by
   * `/fileMappingUF/mapping/:table_name/:object_type`
   * (`jetsclient/lib/routes/jets_routes_app.dart`, `ufMappingPath`), and both are
   * substituted into its form's queries — with neither, the worksheet has no rows
   * to draw. **Five** of the eleven flow routes carry parameters this way —
   * `clientRegistryUF` and `sourceConfigUF` a `startAtKey`, `mapFileUF` those
   * two, `workspacePullUF` five and `loadConfigUF` a `workspace_name`. This
   * comment said four until F.10 counted them off `jetsRoutesMap`; the Flutter
   * half that fills the query string is `userFlowRoutes`
   * (`jetsclient/lib/routes/migrated_user_flows.dart`).
   *
   * A query string rather than path segments, because the names differ per flow
   * and a positional path would have to be declared somewhere the router can see
   * before the document is read. `?table_name=…&object_type=…`.
   *
   * Seeded once per flow, before the load, so `planQueries` sees them on its
   * first pass and no query runs twice.
   */
  useEffect(() => {
    for (const [name, value] of search.entries()) formState.setValue(0, name, value);
    // `search` is a new object per render; its serialisation is the dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formState, search.toString()]);

  useEffect(() => {
    if (key === undefined) return;
    let cancelled = false;
    setLoaded(null);
    setLoadFindings(null);
    setBusy(true);
    void (async () => {
      try {
        const active = await workspaceApi.activeWorkspace();
        setFileKeyLabelPattern(active.fileKeyLabelRe === "" ? null : active.fileKeyLabelRe);
        const store = new FlowStore(workspaceApi, {
          workspaceName: active.name,
          registry: productionRegistry,
        });
        const flow = await store.load(key);
        if (cancelled) return;
        setLoaded({ workspaceName: active.name, flow });
        setPosition(startAt(flow.flow));
      } catch (error) {
        if (cancelled) return;
        // **A load failure renders here rather than in the banner.** The findings
        // are a list with a JSON Pointer each, and they are the answer to "why
        // will this flow not run" — a one-line banner would show the first and
        // lose the rest, which is what `resolveEscapes` returning a list exists
        // to prevent.
        setLoadFindings(
          error instanceof FlowLoadError
            ? error.findings.map((f) => (f.path === "" ? f.message : `${f.path}: ${f.message}`))
            : [error instanceof Error ? error.message : String(error)],
        );
      } finally {
        if (!cancelled) setBusy(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [key, workspaceApi]);

  const fetcher: DataTableFetcher = useCallback(
    (payload) => api.dataTable(payload),
    [api],
  );

  /** Leaves the flow: the document's `exitScreenPath`, or the IDE. */
  const exit = useCallback(() => {
    const path = loaded?.flow.flow.exitScreenPath;
    // A Flutter route rather than one of ours — two flows set it, both to
    // `/workspaces`, and that screen is track C's. Sending the browser there is
    // correct while both apps exist and is the same full page load the handoff
    // in the other direction is (`userflow/routing.ts`).
    if (path !== undefined && path !== "") window.location.href = `/#${path}`;
    else void navigate("/workspace");
  }, [loaded, navigate]);

  const queryPost = useCallback(
    (payload: Record<string, unknown>) => api.dataTable<{ result_map?: Record<string, unknown> }>(payload),
    [api],
  );

  const currentForm = useMemo(() => {
    if (loaded === null || position === null) return null;
    const state = loaded.flow.flow.states[position.stateKey];
    if (state === undefined) return null;
    return loaded.flow.forms.forms[state.formConfig] ?? null;
  }, [loaded, position]);

  // The form's named queries — the dropdown and typeahead item sources of I.2b.
  // Declared after `currentForm` because they are the *current* form's, and a
  // transition changes the set: `FlowStore` loads every form of the flow, and
  // only the one on screen has a reason to be querying.
  const queries = useFormQueries(currentForm, formState, queryPost);

  /** The form's named validator, resolved. Undefined for a form naming none. */
  const validator = useMemo((): FormValidatorContext | undefined => {
    if (currentForm?.validator === undefined || loaded === null) return undefined;
    // `FlowStore.load` refuses the set if the name does not resolve, so reaching
    // here with an unregistered name is not possible — the lookup is total.
    return {
      validate: productionRegistry.validators[currentForm.validator]!,
      flowKey: loaded.flow.key,
    };
  }, [currentForm, loaded]);

  /**
   * Sizing and seeding a repeating form. Task F.1.
   *
   * The count comes from the query, not from the document — `resizeFormState`
   * grows and never shrinks (I.1), which is the Dart's behaviour and is why the
   * form state is per flow and the groups accumulate rather than reset. Then the
   * named `rowInitializers` escape writes each row into its group.
   *
   * **Runs when the rows change, and writes are idempotent**, so a re-query
   * caused by a changed parameter re-seeds rather than appends: the escape sets
   * the same keys of the same groups.
   */
  const repeatRows = currentForm?.repeat === undefined ? undefined : queries.rows(currentForm.repeat.from);
  useEffect(() => {
    const repeat = currentForm?.repeat;
    if (repeat === undefined || repeatRows === undefined || loaded === null) return;
    const seed = productionRegistry.rowInitializers[repeat.seed]!;
    formState.resizeFormState(repeatRows.length);
    repeatRows.forEach((row, index) => {
      seed({ formState, group: GROUP, flowKey: loaded.flow.key }, row, index);
    });
    formState.notifyListeners();
  }, [currentForm, repeatRows, formState, loaded]);

  // **A failed item query is a banner, not a load failure.** The form renders:
  // the dropdown shows its prompt entry and no choices, the typeahead offers no
  // suggestions, and everything else on the form still works. Refusing the whole
  // screen would be the wrong trade — `loadFilesUF` has no queries at all and
  // `fmMappingFormUF` has three, only one of which any given field needs.
  useEffect(() => {
    if (queries.error !== null) setError(`Loading the form's choices failed: ${queries.error}`);
  }, [queries.error, setError]);

  // Recomputed on every store change — see `stateVersion`. A memo rather than an
  // inline call so the dependency on the version is written down rather than
  // implied by where the expression happens to sit.
  const formValid = useMemo(
    () => (currentForm === null ? true : isWholeFormValid(currentForm, formState, GROUP, validator)),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [currentForm, formState, validator, stateVersion],
  );

  /**
   * Live validation messages, for a form that asks for them.
   *
   * The header of `FormRenderer` says a renderer that validated on every
   * keystroke "would show a required-field error before the user had typed
   * anything, which is not what the Flutter app does" — and on **this** form it
   * is exactly what the Flutter app does (`autovalidate`, `form.ts`). Both are
   * right: it is a property of the form, so it is in the document, and the
   * decision stays with the caller rather than moving into the renderer.
   */
  useEffect(() => {
    if (currentForm?.autovalidate !== true) return;
    setErrors(validateAllGroups(currentForm, formState, GROUP, validator));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentForm, formState, validator, stateVersion]);

  // `goToState` writes through a ref because the interpreter may call it in the
  // middle of an action whose result also moves the flow; the last write wins,
  // and `step` applies it after the action returns.
  const jumpTo = useRef<string | null>(null);

  const host: ActionHost = useMemo(
    () => ({
      query: async (name: string) => {
        // The one Dart site is `getProcessInputRdfTypes`, in a flow track F has
        // not reached. Refusing loudly beats returning null, which the
        // interpreter reads as "no rows" and reports as data rather than as a
        // build gap.
        throw new Error(`named query "${name}" is not registered in this build`);
      },
      validate: () => {
        if (currentForm === null) return true;
        const found = validateAllGroups(currentForm, formState, GROUP, validator);
        setErrors(found);
        return found.length === 0;
      },
      confirm: async (message: string) => window.confirm(message),
      post: async (request): Promise<PostResult> => {
        try {
          // The endpoint is the document's, from the two `EndpointSchema` allows;
          // the body is the interpreter's, already built from the step.
          await api.endpoint(request.endpoint, request.body);
          return { statusCode: 200 };
        } catch (error) {
          if (error instanceof ApiError) {
            return { statusCode: error.status, error: error.message };
          }
          return { statusCode: 500, error: error instanceof Error ? error.message : String(error) };
        }
      },
      notify: (level, message) => (level === "error" ? setError(message) : setStatus(message)),
      setBusy,
      goToState: (state: string) => {
        jumpTo.current = state;
      },
      close: exit,
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    [api, currentForm, exit, formState, setError, setStatus, validator],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<string | null> => {
      const action = loaded?.flow.actions.actions[name];
      if (action === undefined) {
        // `validateDocumentSet` refuses this at load, so reaching it means a
        // document was run without being checked — say that rather than
        // reporting a missing button to the user.
        throw new Error(`action "${name}" is not in this flow's action document`);
      }
      return runAction({
        action,
        host,
        formState,
        field: { group: GROUP, key: name },
        registry: productionRegistry,
        flowKey: loaded!.flow.key,
      });
    },
    [loaded, host, formState],
  );

  const press = useCallback(
    async (name: string) => {
      if (loaded === null || position === null) return;
      setBusy(true);
      setError(null);
      try {
        if (!isStandardAction(name)) {
          // A form button naming an entry in the action document. It does not go
          // through `step`, which is the Dart's shape too: the six standard keys
          // are the flow's and everything else is the action grammar's.
          const outcome = await runNamedAction(name);
          if (outcome !== null) setError(outcome);
        } else {
          const result = await step(name as StandardAction, {
            flow: loaded.flow.flow,
            position,
            formState,
            group: GROUP,
            runStateAction: runNamedAction,
            validate: () => host.validate(),
            exit,
          });
          if (result.outcome !== null) setError(result.outcome);
          setPosition(result.position);
        }
        // An action's `goToState` is applied last, after the engine has had its
        // say — the Dart's `setCurrentUserFlowState` does the same `visitedPages`
        // bookkeeping `ufNext` would have (`schema.ts`, `goToStates`).
        const jump = jumpTo.current;
        jumpTo.current = null;
        if (jump !== null) setPosition((current) => (current === null ? current : advance(current, jump)));
      } catch (error) {
        setError(error instanceof Error ? error.message : String(error));
      } finally {
        setBusy(false);
      }
    },
    [loaded, position, formState, host, exit, runNamedAction, setError],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      switch (request.kind) {
        case "runAction":
          void press(request.name);
          return;
        case "navigate":
          // A `configScreenPath` is a Flutter route; see `exit`.
          window.location.href = `/#${request.path}`;
          return;
        case "escape": {
          const escape = productionRegistry.actions[request.name];
          if (escape === undefined) {
            setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
            return;
          }
          void escape({ formState, group: GROUP, flowKey: loaded?.flow.key ?? "" }).then(() =>
            formState.requestRefresh(),
          );
          return;
        }
        case "promptFilter": {
          // **`window.prompt` rather than a dialog, and that is faithful rather
          // than a shortcut.** The Dart's `showGetInputDialog` is one `TextField`
          // with CANCEL and OK (`jetsclient/lib/components/dialogs.dart`,
          // `showGetInputDialog`), which is what a prompt is; `host.confirm`
          // already maps the same way. Cancel yields null and the Dart's
          // `if (sessionIds != null)` guard is the same check.
          const answer = window.prompt(request.prompt);
          if (answer === null) return;
          setIdFilter(request.column, answer);
          formState.notifyListeners();
          formState.requestRefresh();
          return;
        }
        case "openDialog":
        case "runActionThenDialog":
          setError(
            `"${action.label}" opens a dialog, and this app has no dialog host yet — see I-68`,
          );
          return;
      }
    },
    [press, formState, loaded, setError],
  );

  const onFormAction = useCallback((action: FormAction) => void press(action.action), [press]);

  /**
   * The filters every table on the form is queried with. Task F.5.
   *
   * **Rebuilt whenever the form state notifies**, which is what makes the escape
   * visible: `updateHomeFilters` writes a module-level store and calls
   * `notifyListeners`, `stateVersion` bumps, this memo recomputes, and
   * `useTableBinding` sees a new context object and therefore a new query
   * payload. Keying a table refresh off `requestRefresh` alone would refetch with
   * the *old* payload, which is the failure this is shaped to avoid.
   */
  const tableContext = useMemo(
    () => ({ homeFilters: currentHomeFilters(), dataRegistryFilters: currentDataRegistryFilters() }),
    [stateVersion],
  );

  if (key === undefined) return null;

  if (loadFindings !== null) {
    return (
      <main className="screen uf-runner">
        <h1>{key}</h1>
        <div className="banner banner-error" role="alert">
          <div>
            <p>This user flow cannot be loaded.</p>
            <ul>
              {loadFindings.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        </div>
      </main>
    );
  }

  if (loaded === null || position === null || currentForm === null) {
    return (
      <main className="screen uf-runner">
        <p role="status">{busy ? `Loading ${key}…` : `${key} has no form to show.`}</p>
      </main>
    );
  }

  const state = loaded.flow.flow.states[position.stateKey]!;

  return (
    <main className="screen uf-runner">
      <header className="uf-runner__header">
        {/* A flow document carries no title — S.1 dropped it, because the key is
            the name and two names for one thing is one too many (I-14). The form
            may carry one, and `FormRenderer` shows it. */}
        <h1>{key}</h1>
        <p className="uf-runner__state">{state.description}</p>
      </header>

      <FormRenderer
        // **Remount on every transition, deliberately.** Two states' forms are
        // structurally alike — `loadFilesUF`'s are both one `dataTable` field in
        // one row — so React reconciles them as the same element and the widgets
        // keep their internal state: the table went on showing the *previous*
        // table's label and rows after Next. The values that must survive live in
        // `FormState`, outside React, and `useTableBinding` restores a selection
        // from it on mount — so a fresh subtree per state is both correct and
        // what the Flutter app does, which pushes a new page per state.
        key={position.stateKey}
        form={currentForm}
        errors={errors}
        host={{
          formState,
          group: GROUP,
          queryRows: queries.rows,
          queriesLoading: queries.loading,
          // **The smaller of the two, and both halves are load-bearing.** The
          // query's row count is what the Dart draws, and the store grows and
          // never shrinks (I.1) — so taking the store's alone keeps drawing rows
          // a re-query no longer has, and taking the query's alone draws a group
          // the store does not have yet, because the resize happens in an effect
          // *after* the render that learns the rows. `min` is the state both are
          // true in.
          groupCount:
            currentForm.repeat === undefined
              ? 1
              : Math.min(repeatRows?.length ?? 0, formState.groupCount),
          tableConfig: (tableKey) => tableConfigOf(loaded.flow, tableKey),
          fetcher,
          tableContext,
          predicates: productionRegistry.predicates,
          cellFilters: (tableKey) => cellFiltersFor(loaded.flow, tableKey),
          onTableAction,
          onFormAction,
          formValid: formValid,
          busy,
        }}
      />
    </main>
  );
}

/**
 * The per-column display filters a table document names, resolved.
 *
 * The document carries a *name* per column and `DataTable` takes a map from
 * column name to function — so this is where the registry is consulted. A name
 * that does not resolve cannot reach here: `FlowStore.load` refuses the whole set
 * first, which is the point of resolving at load rather than at render.
 */
function cellFiltersFor(
  loaded: LoadedFlow,
  tableKey: string,
): Record<string, (value: string | null) => string | null> {
  const document = loaded.tables[tableKey];
  if (document === undefined) return {};
  const filters: Record<string, (value: string | null) => string | null> = {};
  for (const column of document.columns) {
    const filter = column.cellFilter ? productionRegistry.cellFilters[column.cellFilter] : undefined;
    if (filter !== undefined) filters[column.name] = filter;
  }
  return filters;
}
