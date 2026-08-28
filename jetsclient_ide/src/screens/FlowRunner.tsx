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
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";

import { ApiError, type ApiClient } from "../api/client";
import { WorkspaceApi } from "../api/workspace";
import { runAction, type ActionHost, type ActionResult, type PostResult } from "../actions/interpret";
import { productionRegistry, setFileKeyLabelPattern } from "../actions/registry";
import { setCpipesWorkspace } from "../cpipes/templateApply";
import {
  currentDataRegistryFilters,
  currentHomeFilters,
  setIdFilter,
} from "../actions/homeFilters";
import type { ActionRequest } from "../datatable/actionDispatch";
import { FormState } from "../datatable/formState";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig, JetsRow } from "../datatable/types";
import { useNotifications } from "../shell/notifications";
import {
  FLOW_EXIT_FALLBACK,
  RETURN_TO,
  inAppPath,
  returnToPath,
  unservedScreenMessage,
  withReturnTo,
} from "./routes";
import { FormDialog, isDialogCancel, useFormDialog } from "../userflow/FormDialog";
import { FormRenderer } from "../userflow/FormRenderer";
import type { FormAction } from "../userflow/form";
import { resolveQuery } from "../userflow/formQueries";
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
  /** Where this flow was opened, so a flow it opens can come back here (D.8). */
  const location = useLocation();
  const here = `${location.pathname}${location.search}`;
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
  /**
   * The dialog host. **I-68, paid by C.2b's screen and wired here in the same
   * change** — leaving the report in place while the host existed would have had
   * the app telling a user something that had stopped being true.
   *
   * The five table actions of kind `showDialog` or `doActionShowDialog` are still
   * on no proof flow's tables, which is what let F.0a meet them and not need them.
   * So this path has no shipping consumer today and is wired anyway: the cost of
   * an unexercised branch is small, and the cost of a button that reports a
   * missing feature after the feature lands is a user who stops trusting the
   * message.
   */
  const dialog = useFormDialog();
  /** See `WorkspaceRegistry.tsx` — `runAction` cannot say why it stopped (I-186). */
  const haltedByUser = useRef(false);

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
    for (const [name, value] of search.entries()) {
      // **`returnTo` is the runner's own parameter, not the flow's** (D.8). Every
      // other name in the query string is a flow argument and is seeded by name;
      // this one names where to go when the flow ends, no form declares it, and
      // seeding it would put a route into form state where a value belongs.
      if (name === RETURN_TO) continue;
      formState.setValue(0, name, value);
    }
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
        // agentic_ai's `cpipesTemplateApply` writes a `.pc.json` into the workspace
        // it was configured against, and `EscapeContext` deliberately does not carry
        // one. Set beside the line above because it is the same fact arriving from
        // the same response, and before the load because `FlowStore` resolves escape
        // names against the registry (their U.3).
        setCpipesWorkspace({ workspaceName: active.name, api: workspaceApi });
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

  /**
   * Leaves the flow: the document's `exitScreenPath`, then where it came from,
   * then the front door. Task D.8, from **I-265**.
   *
   * **The order is authored, inferred, default, and the first two agree wherever
   * both exist today.** `exitScreenPath` is a decision somebody wrote into the
   * document and it wins: the two flows that set one both set `/workspaces`, and
   * both are launched from `/workspaces` by `workspaceRegistryTable`'s buttons
   * (`screens/fixtures/screen_reachability.json`, `reachedFrom`), so the
   * precedence is untested by the corpus rather than contentious in it.
   *
   * **`returnTo` replaces a pop the port dropped.** A Flutter flow with no
   * `exitScreenPath` popped the navigator back to whatever pushed it
   * (`user_flow_actions.dart`); this fell back to a constant instead, and since
   * X.1 moved the index off the editor that constant has been the wrong screen
   * for everyone. See `screens/routes.ts` for why the origin travels in the url
   * rather than as `navigate(-1)` or a store.
   *
   * **In-app since X.1.** `exitScreenPath` is a *Flutter* route template, so it
   * goes through `inAppPath`; `returnTo` is already this app's own path and does
   * not.
   */
  const exit = useCallback(() => {
    const internal = inAppPath(loaded?.flow.flow.exitScreenPath, {});
    void navigate(internal ?? returnToPath(search) ?? FLOW_EXIT_FALLBACK);
    // `search` is a new object per render; its serialisation is the dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loaded, navigate, search.toString()]);

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
  /** The form a dialog is showing, or null. */
  const dialogForm =
    dialog.request === null || loaded === null
      ? null
      : (loaded.flow.forms.forms[dialog.request.form] ?? null);

  /**
   * The named queries of whichever form is *on top*. Task C.2b.
   *
   * A dialog's form has its own `queries`, and running the state's instead would
   * leave a query-backed dropdown in a dialog silently empty — the failure this
   * app refuses everywhere else. `dialogForm` is declared below and is null
   * whenever no dialog is open, which is every frame of the eleven shipping flows:
   * none of their tables carries a `showDialog` action, which is why F.0a met the
   * two kinds and did not need them.
   *
   * **What is *not* switched is `formValid`.** It stays the state's form's, so a
   * dialog button with `enableOnlyWhenFormValid` would read the wrong form. No
   * configuration in either corpus does that — the five dialog-opening table
   * actions name forms whose buttons gate on `capability` alone — and inventing
   * the plumbing for it here would be building against nothing. Named rather than
   * left to be found.
   */
  const queries = useFormQueries(dialogForm ?? currentForm, formState, queryPost);

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
      /**
       * Runs a registered statement and returns its first row by column name.
       * Task F.6.
       *
       * **This threw until now, and the comment it replaces named the flow that
       * would fix it.** The one Dart site is `getProcessInputRdfTypes`
       * (`modules/actions/utils/get_process_info.dart`,
       * `getProcessInputRdfTypes`), reached by `pcAddPipelineConfigUF`.
       *
       * **`raw_query_map` rather than `raw_query`, so this shares I.2b's
       * transport** — the same one request a form's item sources go through,
       * with the same `{key}` substitution and the same literal quoting
       * (`userflow/formQueries.ts`). The Dart uses `raw_query` here and
       * `raw_query_map` for a form's queries; collapsing to one is the port's
       * choice and it is the one that already had a tested substitution path.
       *
       * **Positional rows become a map here rather than in the interpreter**,
       * because naming the columns is the registered query's job and the step's
       * `into` is written against those names. A statement whose row is shorter
       * than its declared columns yields nulls for the rest, which is what the
       * interpreter already writes for a column the row does not carry.
       */
      query: async (name: string) => {
        const registered = productionRegistry.queries[name];
        if (registered === undefined) {
          // `resolveEscapes` refuses the set at load, so this is unreachable from
          // a loaded flow and says so rather than reporting an empty result.
          throw new Error(`named query "${name}" is not registered in this build`);
        }
        const sql = resolveQuery(
          { sql: registered.sql, ...(registered.params ? { params: [...registered.params] } : {}) },
          formState,
        );
        // A missing parameter is "no rows", which is the branch the Dart takes
        // when its own statement finds nothing: `getProcessInputRdfTypes` returns
        // null and the arm stops with "No rows returned".
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
        const found = validateAllGroups(currentForm, formState, GROUP, validator);
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
      /**
       * Reads rows from an endpoint, for an action escape. Task F.8.
       *
       * **`rows` rather than `result_map`, which is the other read shape this
       * screen already speaks.** `query` above posts `raw_query_map` and reads
       * `result_map[name]`; a `read` action answers `{rows: [[…]]}`
       * (`jets/datatable/data_table_action.go`, `ReadDataTableAction`), and
       * `downloadMapping` is the only caller of either that wants more than one
       * row.
       *
       * Null on any failure, including the 401 the api client turns into a
       * sign-out — the caller decides what to say, because the Dart's two
       * failure branches say different things.
       */
      read: async (request) => {
        try {
          const body = await api.endpoint<{ rows?: unknown }>(request.endpoint, request.body);
          return Array.isArray(body.rows) ? (body.rows as JetsRow[]) : [];
        } catch {
          return null;
        }
      },
      /**
       * Hands the browser a file to save. Task F.8.
       *
       * The Dart's `download()` (`jetsclient/lib/utils/download.dart`) builds a
       * blob url, clicks an anchor and revokes it. **The revoke is not optional
       * housekeeping**: the object url pins the blob for the document's lifetime,
       * and a mapping export is a megabyte of held memory per press.
       */
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
      goToState: (state: string) => {
        jumpTo.current = state;
      },
      close: exit,
      userEmail: () => api.currentUser?.email ?? "",
      now: () => Date.now(),
    }),
    [api, currentForm, exit, formState, queryPost, setError, setStatus, validator],
  );

  const runNamedAction = useCallback(
    async (name: string): Promise<ActionResult> => {
      haltedByUser.current = false;
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
          const { message } = await runNamedAction(name);
          if (message !== null) setError(message);
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

  /**
   * Opens a `configForm` a table action named, and waits for it. Task C.2b.
   *
   * The document is `loaded.flow.forms.forms[key]` — `FlowStore` loads every form
   * of the flow, not only the ones its states name, so a dialog's form is already
   * in hand and this needs no fetch. A `configForm` that is *not* there is a
   * document-set error `validateDocumentSet` should have caught, so it reports
   * rather than opening an empty modal.
   */
  const openFlowDialog = useCallback(
    async (form: string, params: Record<string, string>) => {
      if (loaded === null) return;
      if (loaded.flow.forms.forms[form] === undefined) {
        setError(`this flow has no form named "${form}" — its document set is inconsistent`);
        return;
      }
      for (const [name, value] of Object.entries(params)) formState.setValue(GROUP, name, value);
      formState.notifyListeners();
      setErrors([]);
      // Not busy while a modal waits on the user; see `WorkspaceRegistry.tsx`.
      setBusy(false);
      const outcome = await dialog.open({ form, params });
      if (outcome === "ok") formState.requestRefresh();
    },
    [dialog, formState, loaded, setError],
  );

  const onTableAction = useCallback(
    (request: ActionRequest, action: ActionConfig) => {
      switch (request.kind) {
        case "runAction":
          void press(request.name);
          return;
        case "navigate": {
          // See `exit`: this app serves the destinations now, and says so when it
          // does not. A flow reached from a flow carries this one back (D.8).
          const to = inAppPath(action.configScreenPath, request.params);
          if (to !== null) void navigate(withReturnTo(to, here));
          else setError(unservedScreenMessage(action.label, request.path));
          return;
        }
        case "escape": {
          const escape = productionRegistry.actions[request.name];
          if (escape === undefined) {
            setError(`"${action.label}" needs the ${request.name} escape, which is not in this build`);
            return;
          }
          void escape({ formState, group: GROUP, flowKey: loaded?.flow.key ?? "" }, host).then(() =>
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
          // **I-68's debt, paid by C.2b.** The host is
          // `userflow/FormDialog.tsx`; what a flow supplies that a screen does
          // not is the form *document*, which is already loaded — `FlowStore`
          // reads every form of the flow, so a `configForm` a table names is in
          // `loaded.flow.forms` beside the ones its states name.
          void openFlowDialog(request.form, request.params);
          return;
        case "runActionThenDialog":
          void (async () => {
            const { message } = await runNamedAction(request.name);
            if (message !== null) {
              setError(message);
              return;
            }
            await openFlowDialog(request.form, request.params);
          })();
          return;
      }
    },
    [press, formState, loaded, setError, host, openFlowDialog],
  );

  /**
   * A form button, from the flow's own form or from a dialog over it.
   *
   * The two are told apart by which form is on screen rather than by the button:
   * a dialog's Cancel is the dialog's, and everything else runs through `press`,
   * which is what the flow's own buttons do. `dialogForm` is non-null only while
   * a dialog is open.
   */
  const onFormAction = useCallback(
    (action: FormAction) => {
      if (dialog.request === null) {
        void press(action.action);
        return;
      }
      if (isDialogCancel(action)) {
        dialog.close("cancel");
        return;
      }
      void (async () => {
        const { message } = await runNamedAction(action.action);
        if (message !== null) {
          setError(message);
          dialog.close("failed");
          return;
        }
        if (haltedByUser.current) return;
        dialog.close("ok");
      })();
    },
    [dialog, press, runNamedAction, setError],
  );


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
        {/* **The key, and here that is right rather than the defect D.7 fixed.**
            The document did not load, so its title does not exist to be shown —
            and this screen's subject is the failure, whose audience is whoever
            authors the documents. `loadFilesUF` is the string they need to find
            the file; *Load Files* is not. */}
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
        {/* **Task D.7, and the comment this replaces was wrong about its own
            reason.** It read *"a flow document carries no title — S.1 dropped
            it, because the key is the name and two names for one thing is one
            too many (I-14)"*. I-14 refuses a document naming *itself*; a title
            names nothing else and duplicates nothing, and the Flutter app drew
            one above every flow form from the route's `ScreenConfig`
            (`jetsclient/lib/screens/user_flow_screen.dart:37`). So the heading
            was showing `loadFilesUF` on a claim that did not hold — I-263. The
            title is in the document now (`UserFlowSchema`); a flow without one
            falls back to the key, which is what this line used to do
            unconditionally. The form may carry one too, and `FormRenderer`
            shows it below this. */}
        <h1>{loaded.flow.flow.title ?? key}</h1>
        <p className="uf-runner__state">{state.description}</p>
      </header>

      {dialogForm !== null && (
        <FormDialog
          form={dialogForm}
          errors={errors}
          onDismiss={() => dialog.close("cancel")}
          host={{
            formState,
            group: GROUP,
            queryRows: queries.rows,
            queriesLoading: queries.loading,
            groupCount: 1,
            tableConfig: (tableKey) => tableConfigOf(loaded.flow, tableKey),
            fetcher,
            tableContext,
            predicates: productionRegistry.predicates,
            cellFilters: (tableKey) => cellFiltersFor(loaded.flow, tableKey),
            onTableAction,
            onFormAction,
            formValid,
            busy,
          }}
        />
      )}

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
