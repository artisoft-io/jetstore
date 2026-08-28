/**
 * The Infer Server Admin screen. Task C.5, `/inferServerAdmin`.
 *
 * The first of track C's form-only screens, and the first screen in this app to
 * render a `.form.json` that is not a flow's.
 *
 * ## The form is a document, and it lives beside the screen
 *
 * `documents/inferServerAdminScreen.form.json`, imported as a JSON module and
 * parsed through `FormDocumentSchema` — the decision C.2 took for the whole of
 * track C. **A screen's documents are bundled and a flow's are not**, and the
 * reason is a circularity rather than filing: `routing.ts` makes a flow React's
 * *iff* `user_flows/<key>.uf.json` exists **in the workspace**, so a flow's
 * documents are a per-workspace fact by design. There is one `/inferServerAdmin`
 * in a deployment, not one per workspace, so the flows' mechanism is one this
 * screen has no use for rather than a default it is departing from.
 *
 * ## Four things the Dart form does that `FormDocumentSchema` cannot say
 *
 * One is built, three are reported and not worked around. They are C.5's finding
 * and they bind C.10, C.13 and C.14 — the schema was measured against the flows,
 * and this screen's button vocabulary is richer than any flow needed.
 *
 *  1. **`isEnabledEval` — built.** `FormActionSchema.enabledWhen` names a
 *     predicate in the escape registry, which is the cut every other closure in
 *     this port already took. Start and Stop are its only users.
 *  2. **Row flex — not built.** `FormSchema.rows` is an array of arrays and a row
 *     carries no weight, while the Dart's `inputFieldsV2` exists *precisely* so a
 *     row can (`FormFieldRowConfig.flex`; the request and response rows are 2 and
 *     3). Sized in `infer_server.css` here instead, which is a divergence in
 *     where the fact lives rather than in what the user sees.
 *  3. **`onLoadActionKey` — not built, and deliberately not.** A screen fetching
 *     on mount is ordinary React and the effect below is it. Putting it in the
 *     document would add a form-level member for one form and buy nothing the
 *     flows want: a flow's first state runs its action through the engine.
 *  4. **`showCopyToClipboard` — not built.** `TextInput.tsx` records it as one of
 *     five options no flow sets; this screen is the first thing to set one, and a
 *     read-only textarea is still selectable. Lost convenience, stated.
 *
 * ## And no `.ua.json`
 *
 * A screen may have an action document; this one cannot. `EndpointSchema`
 * (`actions/schema.ts`) allows two endpoints and this screen talks to a third,
 * `/inferServer` — and the allowlist is right to be short, since it is mirrored
 * by the Go validator at save time. So the actions are TypeScript below, reached
 * through `onFormAction`, which is the same seam a flow's escape uses.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { ApiClient } from "../api/client";
import { FormState } from "../datatable/formState";
import { productionRegistry } from "../actions/registry";
import { FormRenderer, type FormHost } from "../userflow/FormRenderer";
import { FormDocumentSchema, type Form, type FormAction } from "../userflow/form";
import { validateForm, type FieldError } from "../userflow/validateForm";
import type { TableConfig } from "../datatable/types";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import document from "./documents/inferServerAdminScreen.form.json";
import "./infer_server.css";
import {
  ACTIONS,
  INFER_ENDPOINT,
  INFER_REQUEST,
  INFER_RESPONSE,
  INFER_SERVER_ADMIN,
  INFER_SERVER_STATE,
  INFER_STATUS_LABEL,
  MACROS,
  carriesStatus,
  lifecycleMessage,
  pretty,
  statusOf,
} from "./inferServer";

/**
 * Parsed once at module load rather than per render.
 *
 * **A bundled document is checked at build time in the sense that matters and at
 * no time in the sense that does**: it is not a workspace file, so
 * `SaveWorkspaceFileContent` never validates it, and nothing else would notice a
 * malformed one. Parsing here throws at import — loudly, in every test that
 * touches the screen — rather than rendering a form with fields missing.
 */
const FORM: Form = FormDocumentSchema.parse(document).forms["inferServerAdminForm"]!;

/** One form, one validation group; the form does not repeat. */
const GROUP = 0;

/**
 * Write a value and tell the widgets.
 *
 * `FormState.setValue` is silent — the Dart's is too, and its
 * `setValueAndNotify` is the pair (`jets_form_state.dart`). Every write on this
 * screen comes from a delegate rather than from typing, so every one of them has
 * to notify: without it the state changes and the visible text box does not,
 * which is exactly what `syncWithFormState` exists to prevent on the Dart side.
 */
function setAndNotify(formState: FormState, key: string, value: string): void {
  formState.setValue(GROUP, key, value);
  formState.notifyListeners();
}

/** A screen with no tables. `tableConfig` is unreachable and says so. */
function noTables(key: string): TableConfig {
  throw new Error(`the Infer Server Admin form declares no tables, and asked for "${key}"`);
}

export function InferServerAdmin({ api }: { api: ApiClient }) {
  const formState = useMemo(() => new FormState(1), []);
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [confirmStop, setConfirmStop] = useState(false);
  const { setError, setStatus } = useNotifications();

  /**
   * One subscription, two jobs, and neither is the widgets'.
   *
   * `FormState` lives outside React and the widgets subscribe to it themselves,
   * so a write reaches the boxes without this. Two things above them do not:
   *
   *  - **the gates.** `enabledWhen` is read during *this* component's render, so
   *    writing `infer.server.state` has to bring the render back.
   *  - **the errors.** `FORM.autovalidate` is the document's
   *    `autovalidateMode: always`, and `FlowRunner` obeys it the same way — the
   *    renderer's header says validating on every keystroke is not what the
   *    Flutter app does, and on *this* form it is exactly what it does. Both are
   *    right: it is a property of the form, so it is in the document, and the
   *    decision stays with the caller.
   *
   * Without the second, Submit is disabled by the mount-time required error and
   * stays disabled however much the user types, because nothing recomputes it.
   */
  const [, bump] = useState(0);
  useEffect(
    () =>
      formState.subscribe(() => {
        bump((n) => n + 1);
        if (FORM.autovalidate === true) setErrors(validateForm(FORM, formState, GROUP));
      }),
    [formState],
  );

  // **Validated once before anything is typed, which the Dart gets for free and
  // this does not.** `autovalidateMode: always` means the Flutter form has an
  // answer from the first frame, and Submit reads `enableOnlyWhenFormValid`
  // against it. Here `errors` starts empty, so without this the button is live on
  // a form whose required field is untouched — the one state in which pressing it
  // is guaranteed to be wrong.
  useEffect(() => {
    setErrors(validateForm(FORM, formState, GROUP));
  }, [formState]);

  /**
   * POST an envelope, and put the failure where the user is looking.
   *
   * **The response goes in the response box and not in the shell banner**, which
   * is what the Dart does: on this screen *"the server is stopped"* and *"you
   * lack the capability"* are both routine answers to a request, not application
   * errors, and the box is where the user is reading. A transport failure — no
   * json at all — is the shell's, because it is not about this request.
   */
  const post = useCallback(
    async (envelope: unknown): Promise<Record<string, unknown> | null> => {
      try {
        return await api.endpoint(INFER_ENDPOINT, envelope as Record<string, unknown>);
      } catch (cause) {
        const message = cause instanceof Error ? cause.message : String(cause);
        setAndNotify(formState, INFER_RESPONSE, message);
        return null;
      }
    },
    [api, formState],
  );

  const applyStatus = useCallback(
    (body: unknown) => {
      const { state, label } = statusOf(body);
      formState.setValue(GROUP, INFER_SERVER_STATE, state);
      setAndNotify(formState, INFER_STATUS_LABEL, label);
    },
    [formState],
  );

  const refresh = useCallback(async () => {
    applyStatus(await post({ action: "server_status" }));
  }, [applyStatus, post]);

  // `onLoadActionKey`, as an effect. See the header: this is item 3.
  const loaded = useRef(false);
  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void refresh();
  }, [refresh]);

  const lifecycle = useCallback(
    async (starting: boolean) => {
      setAndNotify(
        formState,
        INFER_STATUS_LABEL,
        starting ? "Status: starting..." : "Status: stopping...",
      );
      const body = await post({ action: starting ? "start_server" : "stop_server" });
      applyStatus(body);
      if (body !== null) setStatus(lifecycleMessage(starting, body));
    },
    [applyStatus, formState, post, setStatus],
  );

  const submit = useCallback(async () => {
    const found = validateForm(FORM, formState, GROUP);
    setErrors(found);
    if (found.length > 0) return;
    const raw = formState.getValue(GROUP, INFER_REQUEST);
    if (typeof raw !== "string") return;
    let envelope: unknown;
    try {
      envelope = JSON.parse(raw);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setAndNotify(formState, INFER_RESPONSE, `The request is not valid json: ${message}`);
      return;
    }
    setAndNotify(formState, INFER_RESPONSE, "Working...");
    const body = await post(envelope);
    if (body === null) return;
    // A lifecycle action is legal to type by hand, so keep the status in step
    // when one comes back through the request box rather than through a button.
    if (carriesStatus(body)) applyStatus(body);
    setAndNotify(formState, INFER_RESPONSE, pretty(body));
  }, [applyStatus, formState, post]);

  const onFormAction = useCallback(
    (action: FormAction) => {
      setError(null);
      setStatus(null);
      const macro = MACROS[action.action];
      if (macro !== undefined) {
        setAndNotify(formState, INFER_REQUEST, pretty(macro));
        return;
      }
      // The stop confirmation is a render, not an await: the Dart opens a
      // danger-zone dialog and this app has no dialog host on `jets_ai` yet
      // (I-68). A panel below the toolbar is the same decision point without
      // building the second one.
      if (action.action === ACTIONS.stop) {
        setConfirmStop(true);
        return;
      }
      const run =
        action.action === ACTIONS.start
          ? () => lifecycle(true)
          : action.action === ACTIONS.refresh
            ? refresh
            : action.action === ACTIONS.submit
              ? submit
              : null;
      if (run === null) return;
      setBusy(true);
      void run().finally(() => setBusy(false));
    },
    [formState, lifecycle, refresh, setError, setStatus, submit],
  );

  const host: FormHost = {
    formState,
    group: GROUP,
    groupCount: 1,
    queryRows: () => undefined,
    queriesLoading: false,
    tableConfig: noTables,
    fetcher: () => {
      throw new Error("the Infer Server Admin form fetches no tables");
    },
    predicates: productionRegistry.predicates,
    cellFilters: () => ({}),
    onTableAction: () => undefined,
    onFormAction,
    formValid: errors.length === 0,
    busy,
  };

  /**
   * **Two lists, because a rule can gate a button without decorating a box.**
   * D.11, from the report: the request box sat in the error state from the
   * moment the screen opened, on a screen the user may have come to only to
   * press *Start* or *Models*.
   *
   * `errors` still gates Submit — `host.formValid` reads it below and an empty
   * request is still not submittable. What changes is only what the *field* is
   * told: an empty box is a box nobody has filled in yet, and saying so in red
   * before the user has typed is a reprimand for arriving.
   *
   * **Emptiness is re-tested here rather than the error being matched on its
   * rule**, because `FieldError` carries a key and a message and not the rule
   * that raised it. Today `required` is the only rule on the field, so the two
   * are the same test; if a second rule is added, this filter keeps hiding only
   * the case it was written for and the new one shows.
   */
  const requestValue = formState.getValue(GROUP, INFER_REQUEST);
  const requestEmpty = typeof requestValue !== "string" || requestValue.trim() === "";
  const shownErrors = requestEmpty ? errors.filter((e) => e.key !== INFER_REQUEST) : errors;

  return (
    <main className="screen infer-admin">
      <FormRenderer form={FORM} host={host} errors={shownErrors} />

      {confirmStop && (
        <div className="banner infer-admin__confirm" role="alertdialog">
          <p>
            Stop the Infer Server? The GPU instance is terminated and any loaded model is
            unloaded.
          </p>
          <ActionButton
            className="btn btn-danger"
            capability={INFER_SERVER_ADMIN}
            disabled={busy}
            onClick={() => {
              setConfirmStop(false);
              setBusy(true);
              void lifecycle(false).finally(() => setBusy(false));
            }}
          >
            Stop it
          </ActionButton>
          <ActionButton className="btn btn-secondary" onClick={() => setConfirmStop(false)}>
            Keep it running
          </ActionButton>
        </div>
      )}
    </main>
  );
}
