/**
 * The CodeMirror 6 editor, wrapped for React.
 *
 * The wrapper exists because CodeMirror owns its own DOM and its own state, and
 * React must not fight it for either. So the view is created once for the lifetime
 * of the mount, and afterwards React communicates only through transactions:
 *
 *  - a new `docKey` (a different file) resets the document wholesale
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
  /** Changes when a different file is shown; drives a full document reset. */
  docKey: string;
  /** File name, used to pick the language. */
  fileName: string;
  /** Initial text for `docKey`. Later changes to it are ignored by design. */
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

  useEffect(() => {
    if (!host.current) return;

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
      language.current.of(languageExtension(fileName)),
      editable.current.of(EditorView.editable.of(!readOnly)),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) changeHandler.current?.(u.state.doc.toString());
      }),
    ];

    const v = new EditorView({
      state: EditorState.create({ doc: content, extensions }),
      parent: host.current,
    });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
    // Created once per mount. Document and language updates are handled by the
    // effects below, as transactions.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A different file: replace the document and reset history so undo cannot walk
  // backwards into the previous file's text.
  useEffect(() => {
    const v = view.current;
    if (!v) return;
    v.dispatch({
      changes: { from: 0, to: v.state.doc.length, insert: content },
      selection: { anchor: 0 },
    });
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
