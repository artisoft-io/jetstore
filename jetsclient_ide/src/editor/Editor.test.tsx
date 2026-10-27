/**
 * @vitest-environment jsdom
 *
 * **Undo never crosses from one file into another.** `jetstore_maintenance_02`
 * `I-65`, found in the browser on 2026-10-27: the editor showed a new file by
 * dispatching a whole-document replace into the same state, `history()` recorded
 * it, and one Ctrl+Z after opening a file put the previous file's text in its tab,
 * marked modified, ready to be saved over it. A new file now gets a fresh state.
 *
 * This renders the real CodeMirror editor, unlike the screen tests, which replace
 * it with a text box: the history is what is under test.
 */

import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EditorView } from "@codemirror/view";
import { redo, undo } from "@codemirror/commands";

import { Editor } from "./Editor";

afterEach(cleanup);

const A = { docKey: "pipes_config/a.pc.json", fileName: "a.pc.json", content: '{\n  "a": 1\n}\n' };
const B = { docKey: "pipes_config/b.pc.json", fileName: "b.pc.json", content: '{\n  "b": 2\n}\n' };

/** The live view under the rendered editor. */
function viewOf(container: HTMLElement): EditorView {
  const dom = container.querySelector(".cm-editor");
  if (!(dom instanceof HTMLElement)) throw new Error("no editor rendered");
  const view = EditorView.findFromDOM(dom);
  if (view === null) throw new Error("no view for the editor");
  return view;
}

/** Types `text` at the end of the document, as one user edit. */
function typeAtEnd(view: EditorView, text: string) {
  const end = view.state.doc.length;
  view.dispatch({ changes: { from: end, insert: text }, userEvent: "input.type" });
}

describe("the editor's undo history", () => {
  it("does not undo into the previous file after a switch", () => {
    const onChange = vi.fn();
    const { container, rerender } = render(<Editor {...A} onChange={onChange} />);
    rerender(<Editor {...B} onChange={onChange} />);
    const view = viewOf(container);

    expect(view.state.doc.toString()).toBe(B.content);
    expect(undo(view)).toBe(false);
    expect(view.state.doc.toString()).toBe(B.content);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not undo into the previous file's edits either", () => {
    const onChange = vi.fn();
    const { container, rerender } = render(<Editor {...A} onChange={onChange} />);
    typeAtEnd(viewOf(container), "// edited in a\n");
    onChange.mockClear();

    rerender(<Editor {...B} onChange={onChange} />);
    const view = viewOf(container);
    undo(view);
    undo(view);

    expect(view.state.doc.toString()).toBe(B.content);
    expect(redo(view)).toBe(false);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("still undoes an edit within one file", () => {
    const onChange = vi.fn();
    const { container, rerender } = render(<Editor {...A} onChange={onChange} />);
    rerender(<Editor {...B} onChange={onChange} />);
    const view = viewOf(container);

    typeAtEnd(view, "// edited in b\n");
    expect(onChange).toHaveBeenLastCalledWith(`${B.content}// edited in b\n`);

    expect(undo(view)).toBe(true);
    expect(view.state.doc.toString()).toBe(B.content);
    expect(onChange).toHaveBeenLastCalledWith(B.content);
    expect(undo(view)).toBe(false);
  });

  it("shows the content it is given when a file is shown again", () => {
    const { container, rerender } = render(<Editor {...A} />);
    rerender(<Editor {...B} />);
    rerender(<Editor {...A} content={`${A.content}// unsaved\n`} />);

    expect(viewOf(container).state.doc.toString()).toBe(`${A.content}// unsaved\n`);
  });

  it("keeps the file's language and read-only flag in the fresh state", () => {
    const { container, rerender } = render(<Editor {...A} />);
    rerender(<Editor {...B} readOnly />);
    const view = viewOf(container);

    expect(view.state.facet(EditorView.editable)).toBe(false);
    expect(view.state.doc.toString()).toBe(B.content);
  });
});
