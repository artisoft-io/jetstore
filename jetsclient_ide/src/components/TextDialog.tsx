/**
 * A modal holding text the user can select and copy by hand.
 * jetstore_maintenance_02, defect `D04`, task `AE.3` (2026-10-01).
 *
 * **It exists for one failure, and it is the mitigation for `R-2`.** A clipboard
 * button writes the clipboard *after* an awaited fetch, and a browser may refuse
 * that write: the user gesture that permitted it can have expired by the time the
 * file arrives, and outside a secure context there is no `navigator.clipboard` at
 * all. Failing there would throw away a file the server has just sent, so the
 * escape hands the text to the host's `showText` and the screen renders this.
 *
 * **Built on `ModalDialog`, which is the one `<dialog>` element in the app**
 * (`userflow/FormDialog.tsx`, `AB.2`). The first version of this component
 * rendered its own `<dialog>`; that was a second dialog system the same day `AB.2`
 * removed the need for one, and `dialogStyles.test.ts` refuses it by name. What
 * `ModalDialog` carries — `showModal`, the jsdom fallback, Escape as dismissal,
 * `.uf-dialog` and so the width rule `D08` gave every dialog — this gets for free.
 *
 * **It opts into `dismissOnBackdrop`**, which a form dialog does not: nothing is
 * typed here, so a click outside loses nothing, and a viewer you cannot click
 * away from is friction for no protection. `ModalDialog` already ignores a click
 * whose press began inside, so dragging a selection past the edge does not close it.
 *
 * Not in `src/userflow/` beside `FormDialog`, because it renders no form document.
 */

import { useEffect, useRef, type ReactNode } from "react";

import { ModalDialog } from "../userflow/FormDialog";

export interface TextDialogProps {
  title: string;
  text: string;
  onClose(): void;
}

export function TextDialog({ title, text, onClose }: TextDialogProps): ReactNode {
  const area = useRef<HTMLTextAreaElement>(null);

  // Selected on open, so Ctrl+C is all that is left to do. Two routes because
  // the order matters: this child effect runs *before* `ModalDialog`'s
  // `showModal`, while the dialog is still closed and a browser will not focus
  // into it — so in a browser the selection comes from `onFocus` when
  // `showModal` focuses the first focusable element, which is the text box; in
  // jsdom, which has no `showModal`, it comes from here.
  useEffect(() => {
    area.current?.focus();
    area.current?.select();
  }, []);

  return (
    <ModalDialog label={title} onDismiss={onClose} dismissOnBackdrop>
      <h2 className="uf-form__title">{title}</h2>
      <p className="uf-form__label">The browser did not allow copying. Select the text and copy it.</p>
      <textarea
        ref={area}
        className="text-dialog__text"
        aria-label={`${title} text`}
        readOnly
        rows={16}
        value={text}
        onFocus={(event) => event.currentTarget.select()}
      />
      <div className="uf-form__actions">
        <button type="button" className="btn btn-primary" onClick={onClose}>
          Close
        </button>
      </div>
    </ModalDialog>
  );
}
