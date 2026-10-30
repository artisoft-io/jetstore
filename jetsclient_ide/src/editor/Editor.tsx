/**
 * The CodeMirror 6 editor, wrapped for React.
 *
 * The wrapper exists because CodeMirror owns its own DOM and its own state, and
 * React must not fight it for either. So the view is created once for the lifetime
 * of the mount, and afterwards React communicates through transactions, with one
 * exception:
 *
 *  - a new `docKey` (a different file) gets a fresh `EditorState`, not a
 *    transaction. **Corrected 2026-10-27** (`jetstore_maintenance_02` `I-65`): it
 *    was a whole-document replace dispatched into the same state, which `history()`
 *    recorded, so one undo after opening a file put the previous file's text in its
 *    tab, ready to be saved over it. A fresh state starts with an empty history and
 *    fires no `onChange`
 *  - a language change is applied through a compartment rather than by rebuilding
 *  - the *content* prop is deliberately not synced back into the view, because the
 *    user is the source of truth for it while the file is open
 *
 * That last point is the mirror image of the bug in the Flutter client, where
 * every rebuild constructed a fresh field and the buffer had to be re-seeded from
 * form state each time.
 */

import { useEffect, useRef } from "react";
import { EditorState, Compartment, type Extension } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine,
  highlightActiveLineGutter, drawSelection, rectangularSelection,
  highlightSpecialChars, crosshairCursor } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { searchKeymap, highlightSelectionMatches, search } from "@codemirror/search";
import { bracketMatching, foldGutter, foldKeymap, indentOnInput,
  indentUnit } from "@codemirror/language";
import { closeBrackets, closeBracketsKeymap } from "@codemirror/autocomplete";
import { languageExtension } from "./language";
import { jetStoreEditorTheme } from "./theme";

export interface EditorProps {
  /** Changes when a different file is shown; drives a fresh editor state. */
  docKey: string;
  /** File name, used to pick the language. */
  fileName: string;
  /** The text to show for `docKey` when it is shown. Later changes are ignored by design. */
  content: string;
  readOnly?: boolean;
  onChange?: (value: string) => void;
  onSave?: () => void;
}

export function Editor({ docKey, fileName, content, readOnly, onChange, onSave }: EditorProps) {
  const host = useRef<HTMLDivElement | null>(null);
  const view = useRef<EditorView | null>(null);
  const language = useRef(new Compartment());
  const editable = useRef(new Compartment());

  // Kept in refs so changing a handler never tears down the view.
  const changeHandler = useRef(onChange);
  const saveHandler = useRef(onSave);
  changeHandler.current = onChange;
  saveHandler.current = onSave;

  // The language and the read-only flag a fresh state starts with. The effects
  // below reconfigure a live state; a fresh one reads these.
  const fileNameRef = useRef(fileName);
  const readOnlyRef = useRef(readOnly);
  fileNameRef.current = fileName;
  readOnlyRef.current = readOnly;
  // The file the view's state was made for, so mounting does not make it twice.
  const shownDocKey = useRef(docKey);

  /** A new state for one file: its text, the current language, an empty history. */
  const makeState = (doc: string): EditorState => {
    const saveKeymap = keymap.of([
      {
        key: "Mod-s",
        preventDefault: true,
        run: () => {
          saveHandler.current?.();
          return true;
        },
      },
    ]);

    const extensions: Extension[] = [
      lineNumbers(),
      foldGutter(),
      highlightSpecialChars(),
      history(),
      drawSelection(),
      EditorState.allowMultipleSelections.of(true),
      indentOnInput(),
      indentUnit.of("  "),
      bracketMatching(),
      closeBrackets(),
      rectangularSelection(),
      crosshairCursor(),
      highlightActiveLine(),
      highlightActiveLineGutter(),
      highlightSelectionMatches(),
      search({ top: true }),
      // Save must come before the default keymap so Mod-s is not swallowed.
      saveKeymap,
      keymap.of([...closeBracketsKeymap, ...defaultKeymap, ...searchKeymap,
        ...historyKeymap, ...foldKeymap, indentWithTab]),
      EditorView.lineWrapping,
      jetStoreEditorTheme,
      language.current.of(languageExtension(fileNameRef.current)),
      editable.current.of(EditorView.editable.of(!readOnlyRef.current)),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) changeHandler.current?.(u.state.doc.toString());
      }),
    ];

    return EditorState.create({ doc, extensions });
  };

  useEffect(() => {
    if (!host.current) return;

    const v = new EditorView({
      state: makeState(content),
      parent: host.current,
    });
    view.current = v;
    shownDocKey.current = docKey;
    return () => {
      v.destroy();
      view.current = null;
    };
    // Created once per mount. Language and read-only updates are handled by the
    // effects below, as transactions; a new file by the one after this.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A different file: a fresh state, so undo cannot walk backwards into the
  // previous file's text. A replace transaction here was `I-65`: `history()`
  // recorded it. `setState` dispatches nothing, so no `onChange` fires either,
  // and the document is the caller's `content` for that file.
  useEffect(() => {
    const v = view.current;
    if (!v || shownDocKey.current === docKey) return;
    v.setState(makeState(content));
    shownDocKey.current = docKey;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [docKey]);

  useEffect(() => {
    view.current?.dispatch({
      effects: language.current.reconfigure(languageExtension(fileName)),
    });
  }, [fileName]);

  useEffect(() => {
    view.current?.dispatch({
      effects: editable.current.reconfigure(EditorView.editable.of(!readOnly)),
    });
  }, [readOnly]);

  return <div className="editor-host" ref={host} />;
}
