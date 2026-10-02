/**
 * @vitest-environment jsdom
 *
 * The app's own `confirm` and `prompt`, and the rule that nothing else asks.
 * jetstore_maintenance_02 `I-39`, 2026-10-02.
 *
 * The first block pins the native contract the call sites rely on: a call site
 * changed by an `await` and nothing else, so `true`/`false` and value/`null`
 * have to mean exactly what they meant. The rule that nothing else asks is
 * `nativePrompts.test.ts`, which needs the node environment to read sources.
 */

import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { PromptsProvider, usePrompts, type Prompts } from "./prompts";

afterEach(cleanup);

/** Renders the provider and hands back what a screen would get from the hook. */
function mount(): Prompts {
  let prompts: Prompts | null = null;
  function Probe() {
    prompts = usePrompts();
    return null;
  }
  render(
    <PromptsProvider>
      <Probe />
    </PromptsProvider>,
  );
  return prompts!;
}

/** Whether a promise has settled yet, without waiting for it to. */
async function settled<T>(promise: Promise<T>): Promise<{ done: boolean; value?: T }> {
  const pending = Symbol("pending");
  const value = await Promise.race([promise, Promise.resolve(pending)]);
  return value === pending ? { done: false } : { done: true, value: value as T };
}

describe("confirm", () => {
  it("shows the message and resolves true on OK", async () => {
    const prompts = mount();
    let answer!: Promise<boolean>;
    act(() => {
      answer = prompts.confirm("Delete the client?");
    });
    const dialog = screen.getByRole("dialog", { name: "Delete the client?" });
    expect(within(dialog).getByText("Delete the client?")).toBeTruthy();
    expect(await settled(answer)).toEqual({ done: false });

    fireEvent.click(within(dialog).getByRole("button", { name: "OK" }));
    expect(await answer).toBe(true);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("resolves false on Cancel", async () => {
    const prompts = mount();
    let answer!: Promise<boolean>;
    act(() => {
      answer = prompts.confirm("Delete the client?");
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(await answer).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("resolves false on Escape, which the browser delivers as cancel", async () => {
    const prompts = mount();
    let answer!: Promise<boolean>;
    act(() => {
      answer = prompts.confirm("Delete the client?");
    });
    fireEvent(screen.getByRole("dialog"), new Event("cancel", { cancelable: true }));
    expect(await answer).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("has no text box", () => {
    const prompts = mount();
    act(() => {
      void prompts.confirm("Sure?");
    });
    expect(within(screen.getByRole("dialog")).queryByRole("textbox")).toBeNull();
  });
});

describe("prompt", () => {
  it("resolves the typed value on OK", async () => {
    const prompts = mount();
    let answer!: Promise<string | null>;
    act(() => {
      answer = prompts.prompt("Enter session IDs");
    });
    const dialog = screen.getByRole("dialog", { name: "Enter session IDs" });
    fireEvent.change(within(dialog).getByRole("textbox"), { target: { value: "s1, s2" } });
    expect(await settled(answer)).toEqual({ done: false });
    fireEvent.click(within(dialog).getByRole("button", { name: "OK" }));
    expect(await answer).toBe("s1, s2");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("submits on Enter, which is the form's submit", async () => {
    const prompts = mount();
    let answer!: Promise<string | null>;
    act(() => {
      answer = prompts.prompt("Enter request IDs");
    });
    const box = screen.getByRole("textbox");
    fireEvent.change(box, { target: { value: "r1" } });
    fireEvent.submit(box.closest("form")!);
    expect(await answer).toBe("r1");
  });

  it("starts from the default value, and an untouched OK returns it", async () => {
    const prompts = mount();
    let answer!: Promise<string | null>;
    act(() => {
      answer = prompts.prompt("Name?", "acme");
    });
    expect((screen.getByRole("textbox") as HTMLInputElement).value).toBe("acme");
    fireEvent.click(screen.getByRole("button", { name: "OK" }));
    expect(await answer).toBe("acme");
  });

  it("returns the empty string, not null, for OK on an empty box — as window.prompt does", async () => {
    const prompts = mount();
    let answer!: Promise<string | null>;
    act(() => {
      answer = prompts.prompt("Anything?");
    });
    fireEvent.click(screen.getByRole("button", { name: "OK" }));
    expect(await answer).toBe("");
  });

  it("resolves null on Cancel and on Escape", async () => {
    const prompts = mount();
    let first!: Promise<string | null>;
    act(() => {
      first = prompts.prompt("Enter session IDs");
    });
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "typed" } });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(await first).toBeNull();

    let second!: Promise<string | null>;
    act(() => {
      second = prompts.prompt("Enter session IDs");
    });
    fireEvent(screen.getByRole("dialog"), new Event("cancel", { cancelable: true }));
    expect(await second).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("one at a time", () => {
  it("settles the first request as cancelled when a second arrives", async () => {
    const prompts = mount();
    let first!: Promise<boolean>;
    let second!: Promise<string | null>;
    act(() => {
      first = prompts.confirm("First?");
    });
    act(() => {
      second = prompts.prompt("Second?");
    });
    expect(await first).toBe(false);
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(screen.getByRole("dialog", { name: "Second?" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(await second).toBeNull();
  });
});

describe("outside the shell", () => {
  it("throws rather than answering Cancel for a question nobody saw", () => {
    function Orphan() {
      usePrompts();
      return null;
    }
    // React logs the error it rethrows; the assertion is the throw.
    const quiet = console.error;
    console.error = () => {};
    try {
      expect(() => render(<Orphan />)).toThrow(/inside the app shell/);
    } finally {
      console.error = quiet;
    }
  });
});
