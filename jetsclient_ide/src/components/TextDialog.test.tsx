/**
 * @vitest-environment jsdom
 *
 * `TextDialog` — the `R-2` fallback for a refused clipboard write
 * (jetstore_maintenance_02 `AE.3`). What jsdom can observe: the dialog opens
 * with the text in a read-only box, selected, and both ways out close it.
 */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TextDialog } from "./TextDialog";

describe("TextDialog", () => {
  afterEach(cleanup);

  it("opens holding the text, read-only and selected", () => {
    render(<TextDialog title="Run manifest" text={'{"run":1}'} onClose={() => {}} />);
    const dialog = screen.getByRole("dialog", { name: "Run manifest" });
    expect(dialog.hasAttribute("open")).toBe(true);
    expect(dialog.className).toContain("uf-dialog");
    const box = screen.getByRole("textbox", { name: "Run manifest text" }) as HTMLTextAreaElement;
    expect(box.value).toBe('{"run":1}');
    expect(box.readOnly).toBe(true);
    expect(box.selectionStart).toBe(0);
    expect(box.selectionEnd).toBe('{"run":1}'.length);
  });

  it("closes on its button", () => {
    const onClose = vi.fn();
    render(<TextDialog title="Run manifest" text="x" onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on Escape, which the browser delivers as cancel", () => {
    const onClose = vi.fn();
    render(<TextDialog title="Run manifest" text="x" onClose={onClose} />);
    fireEvent(screen.getByRole("dialog"), new Event("cancel", { cancelable: true }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
