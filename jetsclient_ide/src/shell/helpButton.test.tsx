/**
 * @vitest-environment jsdom
 *
 * The shell's *Help* button: it opens a tab before awaiting anything, asks the
 * server for the documentation cookie, and then points the tab at the page for
 * the current screen. A failed call closes the tab; a blocked tab is reported.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import { ApiClient, ApiError } from "../api/client";
import { AppShell } from "./AppShell";
import { helpPathFor, openHelp, type HelpTab } from "./helpTopics";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  localStorage.clear();
});

function fakeTab() {
  const close = vi.fn<() => void>();
  const tab: HelpTab & { close: typeof close } = {
    opener: "the app",
    location: { href: "about:blank" },
    close,
  };
  return tab;
}

/** A signed-in client whose later posts are answered by `respond`. */
async function signedIn(respond: (path: string, init: RequestInit) => Response): Promise<{
  api: ApiClient;
  fetchImpl: ReturnType<typeof vi.fn>;
}> {
  const fetchImpl = vi.fn(async (url: string, init: RequestInit) => {
    if (url === "/login") {
      return new Response(JSON.stringify({ token: "t0", name: "Ada", user_email: "ada@example.com" }), {
        status: 200,
      });
    }
    return respond(url, init);
  });
  const api = new ApiClient("", fetchImpl as unknown as typeof fetch);
  await api.login("ada@example.com", "pw");
  return { api, fetchImpl };
}

function renderShell(api: ApiClient, initial: string) {
  render(
    <MemoryRouter initialEntries={[initial]}>
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={[]} />}>
          <Route path="workspaces/:workspace_name/home" element={<p>screen</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("ApiClient.openDocsSession", () => {
  it("posts to /docsSession with the bearer token and accepts an empty 204", async () => {
    const { api, fetchImpl } = await signedIn(() => new Response(null, { status: 204 }));
    await api.openDocsSession();

    const [url, init] = fetchImpl.mock.calls[1] as [string, RequestInit];
    expect(url).toBe("/docsSession");
    expect(init.method).toBe("POST");
    expect((init.headers as Record<string, string>)["Authorization"]).toBe("Bearer t0");
  });

  it("throws an ApiError on a refusal", async () => {
    const { api } = await signedIn(
      () => new Response(JSON.stringify({ error: "no documentation" }), { status: 500 }),
    );
    await expect(api.openDocsSession()).rejects.toBeInstanceOf(ApiError);
  });
});

describe("openHelp", () => {
  it("opens the tab before the session call resolves", async () => {
    const tab = fakeTab();
    const order: string[] = [];
    const openTab = vi.fn(() => {
      order.push("open");
      return tab;
    });
    const session = vi.fn(async () => {
      order.push("session");
    });
    expect(await openHelp("/home", session, openTab)).toBeNull();
    expect(order).toEqual(["open", "session"]);
  });

  it("closes the tab and returns the error when the call fails", async () => {
    const tab = fakeTab();
    const message = await openHelp(
      "/home",
      async () => {
        throw new ApiError("The server rejected the request.", 500);
      },
      () => tab,
    );
    expect(message).toBe("The server rejected the request.");
    expect(tab.close).toHaveBeenCalledTimes(1);
    expect(tab.location.href).toBe("about:blank");
  });

  it("reports a blocked tab without calling the server", async () => {
    const session = vi.fn(async () => undefined);
    const message = await openHelp("/home", session, () => null);
    expect(message).toMatch(/blocked/);
    expect(session).not.toHaveBeenCalled();
  });
});

describe("the Help button", () => {
  it("opens a blank tab, sets the cookie once, then shows the mapped page", async () => {
    const { api } = await signedIn(() => new Response(null, { status: 204 }));
    const session = vi.spyOn(api, "openDocsSession");
    const tab = fakeTab();
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);

    renderShell(api, "/workspaces/ws1/home");
    fireEvent.click(screen.getByRole("button", { name: /^Help/ }));

    expect(open).toHaveBeenCalledWith("", "_blank");
    await waitFor(() => expect(tab.location.href).toBe(helpPathFor("/workspaces/ws1/home")));
    expect(tab.location.href).toBe("/docs/user/");
    expect(tab.opener).toBeNull();
    expect(session).toHaveBeenCalledTimes(1);
    expect(tab.close).not.toHaveBeenCalled();
  });

  it("closes the tab and shows the error when the call fails", async () => {
    const { api } = await signedIn(
      () => new Response(JSON.stringify({ error: "documentation unavailable" }), { status: 500 }),
    );
    const tab = fakeTab();
    vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);

    renderShell(api, "/workspaces/ws1/home");
    fireEvent.click(screen.getByRole("button", { name: /^Help/ }));

    expect((await screen.findByRole("alert")).textContent).toContain("documentation unavailable");
    expect(tab.close).toHaveBeenCalledTimes(1);
    expect(tab.location.href).toBe("about:blank");
  });

  it("reports a blocked tab", async () => {
    const { api } = await signedIn(() => new Response(null, { status: 204 }));
    const session = vi.spyOn(api, "openDocsSession");
    vi.spyOn(window, "open").mockReturnValue(null);

    renderShell(api, "/workspaces/ws1/home");
    fireEvent.click(screen.getByRole("button", { name: /^Help/ }));

    expect((await screen.findByRole("alert")).textContent).toMatch(/blocked/);
    expect(session).not.toHaveBeenCalled();
  });
});
