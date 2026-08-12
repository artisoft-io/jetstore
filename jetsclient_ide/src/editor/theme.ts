/**
 * Editor theming.
 *
 * The tokens come from the same CSS custom properties the rest of the shell uses
 * (see styles.css), so the editor follows the app's light/dark state without a
 * second source of truth and without a JS theme switch — CodeMirror reads the
 * cascade like anything else.
 */

import { EditorView } from "@codemirror/view";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";
import type { Extension } from "@codemirror/state";

const editorTheme = EditorView.theme({
  "&": {
    color: "var(--ink)",
    backgroundColor: "var(--editor-bg)",
    height: "100%",
    fontSize: "13px",
  },
  ".cm-content": {
    fontFamily: "var(--mono)",
    padding: "10px 0",
    caretColor: "var(--accent)",
  },
  ".cm-scroller": { fontFamily: "var(--mono)", lineHeight: "1.6" },
  "&.cm-focused": { outline: "none" },
  ".cm-gutters": {
    backgroundColor: "var(--editor-gutter)",
    color: "var(--ink-3)",
    border: "none",
    borderRight: "1px solid var(--rule)",
  },
  ".cm-activeLine": { backgroundColor: "var(--editor-active)" },
  ".cm-activeLineGutter": {
    backgroundColor: "var(--editor-active)",
    color: "var(--ink-2)",
  },
  ".cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection": {
    backgroundColor: "var(--editor-selection)",
  },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--accent)" },
  ".cm-searchMatch": {
    backgroundColor: "var(--editor-match)",
    outline: "1px solid var(--accent)",
  },
  ".cm-searchMatch.cm-searchMatch-selected": { backgroundColor: "var(--editor-match-active)" },
  ".cm-panels": {
    backgroundColor: "var(--surface-2)",
    color: "var(--ink)",
    border: "none",
    borderTop: "1px solid var(--rule)",
  },
  ".cm-panels input, .cm-panels button": {
    fontFamily: "var(--sans)",
    fontSize: "12px",
    background: "var(--surface)",
    color: "var(--ink)",
    border: "1px solid var(--rule-2)",
    borderRadius: "4px",
    padding: "3px 6px",
  },
  ".cm-foldPlaceholder": {
    backgroundColor: "var(--surface-2)",
    border: "1px solid var(--rule-2)",
    color: "var(--ink-3)",
  },
  ".cm-tooltip": {
    background: "var(--surface)",
    border: "1px solid var(--rule-2)",
    borderRadius: "6px",
    color: "var(--ink)",
  },
});

/**
 * One highlight style for both themes: every colour is a variable, so the palette
 * swaps with the page rather than being duplicated per mode.
 */
const highlight = HighlightStyle.define([
  { tag: t.lineComment, color: "var(--syn-comment)", fontStyle: "italic" },
  { tag: t.blockComment, color: "var(--syn-comment)", fontStyle: "italic" },
  { tag: t.string, color: "var(--syn-string)" },
  { tag: t.number, color: "var(--syn-number)" },
  { tag: t.keyword, color: "var(--syn-keyword)", fontWeight: "600" },
  { tag: t.modifier, color: "var(--syn-keyword)", fontWeight: "600" },
  { tag: t.atom, color: "var(--syn-constant)" },
  { tag: t.bool, color: "var(--syn-constant)" },
  { tag: t.null, color: "var(--syn-constant)" },
  { tag: t.typeName, color: "var(--syn-type)" },
  { tag: t.namespace, color: "var(--syn-namespace)" },
  { tag: t.labelName, color: "var(--syn-rule)", fontWeight: "700" },
  { tag: t.variableName, color: "var(--syn-variable)" },
  { tag: t.propertyName, color: "var(--syn-property)" },
  { tag: t.function(t.variableName), color: "var(--syn-function)" },
  { tag: t.operator, color: "var(--syn-operator)" },
  { tag: t.punctuation, color: "var(--ink-3)" },
  { tag: t.bracket, color: "var(--ink-2)" },
  { tag: t.meta, color: "var(--syn-meta)" },
  { tag: t.invalid, color: "var(--crit)" },
]);

export const jetStoreEditorTheme: Extension = [editorTheme, syntaxHighlighting(highlight)];
