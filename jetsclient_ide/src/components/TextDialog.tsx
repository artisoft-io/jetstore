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
 * **A native `<dialog>`, on `FormDialog`'s terms** (`userflow/FormDialog.tsx`):
 * `showModal()` gives the top layer, the inert background, the focus trap and
 * Escape-to-dismiss, and in jsdom — which has no `showModal` — the `open`
 * attribute is set instead, which is only reachable where the missing behaviour
 * is unobservable. It carries `uf-dialog` so that whatever width rule the app
 * gives its dialogs (`D08`) reaches this one too.
 *
 * Not in `src/userflow/` beside `FormDialog`, because it renders no form
 * document: what put that one there was `FormRenderer`, and this has nothing of
 * the document schema in it.
 */

import { useEffect, useRef, type ReactNode } from "react";

export interface TextDialogProps {
  title: string;
  text: string;
  onClose(): void;
}

export function TextDialog({ title, text, onClose }: TextDialogProps): ReactNode {
  const dialog = useRef<HTMLDialogElement>(null);
  const area = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const element = dialog.current;
    if (element === null) return;
    if (!element.open) {
      if (typeof element.showModal === "function") element.showModal();
      else element.setAttribute("open", "");
    }
    // Selected on open, so Ctrl+C is the whole of what is left to do.
    area.current?.focus();
    area.current?.select();
    return () => {
      if (element.open) {
        if (typeof element.close === "function") element.close();
        else element.removeAttribute("open");
      }
    };
  }, []);

  return (
    <dialog
      ref={dialog}
      className="uf-dialog text-dialog"
      aria-label={title}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClose={onClose}
    >
      <h2 className="uf-form__title">{title}</h2>
      <p className="uf-form__label">The browser did not allow copying. Select the text and copy it.</p>
      <textarea
        ref={area}
        className="text-dialog__text"
        aria-label={`${title} text`}
        readOnly
        rows={16}
        value={text}
        // Inline rather than in `styles.css`, deliberately and for now: two other
        // tracks of the same phase (`D08`'s dialog width, `D05`'s footer) were
        // editing that file when this was written. Moving it is one rule.
        style={{ width: "100%", boxSizing: "border-box", fontFamily: "var(--mono)" }}
      />
      <div className="uf-form__actions">
        <button type="button" className="btn btn-primary" onClick={onClose}>
          Close
        </button>
      </div>
    </dialog>
  );
}
