/**
 * The app's own `confirm` and `prompt`. jetstore_maintenance_02 `I-39`,
 * 2026-10-02.
 *
 * **The browser's native ones are not a dialog everywhere this app runs.** The
 * embedded browser pane the app is tested in does not display them:
 * `window.confirm(...)` there returns `false` in about a millisecond and shows
 * nothing, so every confirmed action (*Delete Client*, among others) and every
 * prompt (*Set Session Id*, *Set Request Id*) silently did nothing. Eleven call
 * sites used one or the other; each now asks this provider instead.
 *
 * **The contract is the native one, kept exactly**, so that a call site changes
 * by an `await` and nothing else:
 *
 *  - `confirm(message)` resolves `true` on *OK* and `false` on *Cancel* or
 *    Escape;
 *  - `prompt(message, defaultValue)` resolves the text box's value on *OK* or
 *    Enter — the empty string included, as `window.prompt` returns it — and
 *    `null` on *Cancel* or Escape.
 *
 * **Built on `ModalDialog`, the one `<dialog>` element in the app**
 * (`userflow/FormDialog.tsx`, `AB.2`), so it is `.uf-dialog` and has the width
 * rule `D08` gave every dialog; `dialogStyles.test.ts` would refuse a second
 * `<dialog>`. No backdrop dismissal: neither native prompt has one, and a stray
 * click that cancelled a typed id would lose what was typed.
 *
 * **One at a time**, as `useFormDialog` is: a second request settles the first as
 * cancelled rather than leaving its caller awaiting forever. Nothing in the app
 * asks twice at once — every caller awaits its answer — so this is a guard rather
 * than a queue.
 */

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";

import { ModalDialog } from "../userflow/FormDialog";

export interface Prompts {
  /** `window.confirm`, in the app's own dialog. */
  confirm(message: string): Promise<boolean>;
  /** `window.prompt`, in the app's own dialog. */
  prompt(message: string, defaultValue?: string): Promise<string | null>;
}

/** What is on screen: a confirmation, or a prompt with its starting text. */
type Request =
  | { kind: "confirm"; message: string }
  | { kind: "prompt"; message: string; defaultValue: string };

const PromptsContext = createContext<Prompts | null>(null);

export function PromptsProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<Request | null>(null);
  // Held in a ref, as `useFormDialog` holds its resolver: settling a promise is
  // not a render. Typed as the prompt's answer because a confirmation's is
  // derived from it — any string is OK, null is Cancel.
  const resolver = useRef<((answer: string | null) => void) | null>(null);

  const ask = useCallback(
    (next: Request) =>
      new Promise<string | null>((resolve) => {
        const previous = resolver.current;
        if (previous !== null) previous(null);
        resolver.current = resolve;
        setRequest(next);
      }),
    [],
  );

  // Settles once. `ModalDialog` closes its element on unmount and a browser
  // fires `close` for that, which reaches `onDismiss` after *OK* has already
  // answered — so a second settle is expected and is a no-op here.
  const settle = useCallback((answer: string | null) => {
    setRequest(null);
    const resolve = resolver.current;
    resolver.current = null;
    if (resolve !== null) resolve(answer);
  }, []);

  const value = useMemo<Prompts>(
    () => ({
      confirm: async (message) => (await ask({ kind: "confirm", message })) !== null,
      prompt: (message, defaultValue = "") => ask({ kind: "prompt", message, defaultValue }),
    }),
    [ask],
  );

  return (
    <PromptsContext.Provider value={value}>
      {children}
      {request !== null && <PromptDialog request={request} onAnswer={settle} />}
    </PromptsContext.Provider>
  );
}

export function usePrompts(): Prompts {
  const value = useContext(PromptsContext);
  if (!value) {
    // As `useNotifications`: a screen outside the shell is a wiring mistake, and
    // a confirmation that could not be asked must not read as a *Cancel*.
    throw new Error("usePrompts must be used inside the app shell");
  }
  return value;
}

function PromptDialog({
  request,
  onAnswer,
}: {
  request: Request;
  onAnswer(answer: string | null): void;
}): ReactNode {
  const [text, setText] = useState(request.kind === "prompt" ? request.defaultValue : "");

  // A form, so that Enter in the text box is *OK* as it is in `window.prompt`.
  // *OK* comes first in the markup so that `showModal`, which focuses the first
  // focusable element, lands on it for a confirmation — Enter is then *OK*, as it
  // is natively — and on the text box for a prompt.
  const submit = (event: FormEvent) => {
    event.preventDefault();
    onAnswer(request.kind === "prompt" ? text : "");
  };

  return (
    <ModalDialog label={request.message} onDismiss={() => onAnswer(null)}>
      <form className="uf-form" onSubmit={submit}>
        <p className="prompt-dialog__message">{request.message}</p>
        {request.kind === "prompt" && (
          <div className="field">
            <input
              type="text"
              aria-label={request.message}
              value={text}
              onChange={(event) => setText(event.target.value)}
            />
          </div>
        )}
        <div className="uf-form__actions">
          <button type="submit" className="btn btn-primary">
            OK
          </button>
          <button type="button" className="btn" onClick={() => onAnswer(null)}>
            Cancel
          </button>
        </div>
      </form>
    </ModalDialog>
  );
}
