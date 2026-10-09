/**
 * Test helpers for the app's own `confirm` and `prompt` (`prompts.tsx`,
 * jetstore_maintenance_02 `I-39`, 2026-10-02).
 *
 * Before that date a screen test stubbed `window.confirm` or `window.prompt` and
 * read back what it was called with. These drive the dialog instead, which is
 * the stronger test: a stub answers whether or not anything was shown, and these
 * fail unless the question is on screen. Imported only by tests, so it never
 * reaches the bundle.
 */

import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { expect } from "vitest";

/** Answers the open confirmation and returns its message. */
export async function answerConfirm(choice: "OK" | "Cancel"): Promise<string> {
  const dialog = await screen.findByRole("dialog");
  const message = dialog.getAttribute("aria-label") ?? "";
  fireEvent.click(within(dialog).getByRole("button", { name: choice }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  return message;
}

/**
 * Answers the open prompt — types `value` and presses Enter, or presses Cancel
 * when `value` is null — and returns its message.
 */
export async function answerPrompt(value: string | null): Promise<string> {
  const dialog = await screen.findByRole("dialog");
  const message = dialog.getAttribute("aria-label") ?? "";
  if (value === null) {
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
  } else {
    const input = within(dialog).getByRole("textbox");
    fireEvent.change(input, { target: { value } });
    fireEvent.submit(input.closest("form")!);
  }
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  return message;
}
