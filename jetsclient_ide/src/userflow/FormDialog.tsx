/**
 * A form document, rendered over the current screen. Task C.2b, and the debt
 * **I-68** filed for whoever needed one first.
 *
 * ## What was missing, exactly
 *
 * `showDialog` and `doActionShowDialog` open a `configForm` in a modal, and
 * `actionDispatch.requestFor` has resolved both — with their parameters, and with
 * S.2b's precedence rule (I-24) — since Phase 2. `FormRenderer` has drawn a form
 * document since F.0a. **Neither end was missing; the thing between them was.**
 * I-68 filed it as debt rather than as a task on the stated ground that a
 * speculative host built against no consumer is worse than none, and that the
 * first screen needing one should build it. `/workspaces` has seven, which is
 * more dialogs than the rest of track C's screens have between them.
 *
 * ## Why it lives here
 *
 * `src/userflow/` is the one directory a screen and a flow both already import
 * from. A dialog is a form document plus a scrim: `FormRenderer` is here, `Form`
 * is here, and putting the modal in `src/shell/` would give the session gate and
 * the chrome a dependency on the document schema they otherwise have none of.
 * `src/screens/` would be worse still, because `FlowRunner` lives there and a
 * *flow* is not a screen — the consumer that most needs this is the one that
 * would then be importing sideways.
 *
 * Agreed with track C's other wave-1 sessions before it was built, because
 * C.6, C.9, C.10 and C.13 all need it and a placement decided twice is a
 * placement decided wrongly once.
 *
 * ## It is a `<dialog>`, and the browser owns the hard parts
 *
 * `showModal()` gives focus trapping, the inert background, the top layer and
 * Escape-to-dismiss without a line of code, and gets them right for a screen
 * reader. The one thing it does *not* give is a confirmation before Escape, and
 * the Dart does not either: `showDialog` there is dismissible and the Cancel
 * button and a barrier tap both `Navigator.pop()` with no result. So Escape
 * resolves the promise with `"cancel"`, which is what a barrier tap did.
 *
 * ## Opening one is awaitable, because the Dart's is
 *
 * `showDialog<T>` returns a `Future<T?>` and every caller in `jetsclient` awaits
 * it — the table wants to know whether to refresh, because an insert that
 * succeeded returns `DTActionResult.okDataTableDirty`
 * (`jetsclient/lib/components/data_table.dart`, `actionDispatcher`). Reproducing
 * that as a promise is what lets a screen write `if (await open(...) === "ok")
 * refresh()` instead of threading a callback through the modal.
 *
 * ## What it does not do, stated rather than discovered
 *
 * **It does not seed form state and it does not clear it.** The caller does both,
 * because the caller is the one holding the `navigationParams` the action
 * resolved and the one that knows which group they belong in. Doing it here would
 * mean this component decided a dialog's group, and `FormState` is shared with
 * the screen underneath — a dialog that cleared on close would erase the
 * selection the table published.
 */

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";

import { FormRenderer, type FormHost } from "./FormRenderer";
import type { Form, FormAction } from "./form";
import type { FieldError } from "./validateForm";

/**
 * How a dialog ended.
 *
 * Three outcomes rather than a boolean, because the table's reaction differs:
 * `ok` means the action ran and the rows may have changed, so refresh;
 * `cancel` means nothing happened; `failed` means the action ran and reported,
 * and the banner already carries the message.
 *
 * The Dart collapses the last two — `postInsertRows` pops with a dirty-table
 * result on success and with a plain pop on failure, having written the message
 * into form state — so this is one distinction more than it makes. It is kept
 * because a caller that refreshes on failure re-queries for nothing, and a caller
 * that treats failure as cancellation cannot tell the user why the row did not
 * change.
 */
export type DialogOutcome = "ok" | "cancel" | "failed";

/** The standard Cancel key, which every dialog in the corpus declares. */
export const DIALOG_CANCEL = "dialogCancel";

export interface FormDialogProps {
  /**
   * The dialog's heading. Taken from the *form* rather than passed separately —
   * `FormConfig.title` is what the Dart puts in the `AlertDialog`'s title slot,
   * and 32 of the 32 non-flow forms that are dialogs declare one.
   */
  form: Form;
  host: FormHost;
  errors: FieldError[];
  /** Called with `cancel` when the browser dismisses the dialog. */
  onDismiss(): void;
}

export function FormDialog({ form, host, errors, onDismiss }: FormDialogProps): ReactNode {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const element = ref.current;
    if (element === null) return;
    // `showModal` rather than the `open` attribute: only the former puts the
    // element in the top layer, makes the rest of the document inert and traps
    // focus.
    //
    // **jsdom does not implement it**, and the fallback is a real decision rather
    // than a test shim. Setting `open` renders the dialog and gives it its ARIA
    // role, which is everything a test can observe; what is lost — the top layer,
    // the inert background, the focus trap — is exactly what jsdom has no
    // concept of. So the fallback is *only* reachable where the missing
    // behaviour is unobservable, and a browser never takes it.
    if (!element.open) {
      if (typeof element.showModal === "function") element.showModal();
      else element.setAttribute("open", "");
    }
    return () => {
      if (element.open) {
        if (typeof element.close === "function") element.close();
        else element.removeAttribute("open");
      }
    };
  }, []);

  return (
    <dialog
      ref={ref}
      className="uf-dialog"
      aria-label={form.title ?? "Dialog"}
      // Escape and a backdrop click both land here. The Dart's barrier is
      // dismissible and pops with no result, so both are `cancel`.
      onCancel={(event) => {
        event.preventDefault();
        onDismiss();
      }}
      onClose={onDismiss}
    >
      <FormRenderer form={form} host={host} errors={errors} />
    </dialog>
  );
}

/** What a caller asks for when it opens one. */
export interface DialogRequest {
  /** The form document's key, for the caller's own resolution. */
  form: string;
  /** `navigationParams` and `stateFormNavigationParams`, already resolved. */
  params: Record<string, string>;
}

export interface OpenDialog {
  /** The request currently on screen, or null. */
  readonly request: DialogRequest | null;
  /** Opens one and resolves when it closes. */
  open(request: DialogRequest): Promise<DialogOutcome>;
  /** Closes the open one with an outcome. A form action calls this. */
  close(outcome: DialogOutcome): void;
}

/**
 * Open/close state for one dialog at a time, and the promise that resolves.
 *
 * **One at a time is the corpus, not a simplification.** No `configForm` in
 * either corpus opens a second dialog: the seven of `workspaceRegistryTable` are
 * all terminal, and `pipelineExecStatusTable`'s one is a viewer. A stack would be
 * machinery with no consumer, which is the argument I-68 made about this whole
 * component.
 *
 * The resolver is held in a ref rather than in state because settling a promise
 * is not a render: putting it in state would re-render on open *and* on the
 * resolver being stored, and the second render has nothing to show.
 */
export function useFormDialog(): OpenDialog {
  const [request, setRequest] = useState<DialogRequest | null>(null);
  const resolver = useRef<((outcome: DialogOutcome) => void) | null>(null);

  const close = useCallback((outcome: DialogOutcome) => {
    setRequest(null);
    const resolve = resolver.current;
    resolver.current = null;
    // A close with nothing waiting is not an error — a screen may close a dialog
    // in a cleanup — so this is a guard rather than a throw.
    if (resolve !== null) resolve(outcome);
  }, []);

  const open = useCallback(
    (next: DialogRequest) =>
      new Promise<DialogOutcome>((resolve) => {
        // Settling any previous promise first: leaving one pending would leak an
        // awaiting caller for the lifetime of the screen.
        const previous = resolver.current;
        if (previous !== null) previous("cancel");
        resolver.current = resolve;
        setRequest(next);
      }),
    [],
  );

  return { request, open, close };
}

/**
 * Whether a form action is the standard Cancel.
 *
 * A dialog's buttons are ordinary `FormAction`s and reach the caller through
 * `onFormAction`; the Cancel one is the only key whose behaviour belongs to the
 * dialog rather than to the action document. Every other key names an entry the
 * caller runs.
 *
 * **`viewGitLogWorkspaceDialog` is the case that makes this a function rather
 * than a comparison at one call site**: its single button is labelled *Close*,
 * styled `dialogOk`, and its key is `dialogCancel`
 * (`jetsclient/lib/modules/workspace_ide/form_config.dart`,
 * `FormKeys.viewGitLogWorkspace`). A viewer's only exit is a cancel, and reading
 * the label or the style instead of the key would get it wrong.
 */
export const isDialogCancel = (action: FormAction): boolean => action.action === DIALOG_CANCEL;
