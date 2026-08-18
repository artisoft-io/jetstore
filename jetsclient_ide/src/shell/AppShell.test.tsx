/**
 * @vitest-environment jsdom
 *
 * Tests for the application shell (task A.1).
 *
 * The one worth reading is the last: **routes resolve under the mount prefix.**
 * Vite bakes `base` into asset urls at build time and the router's `basename`
 * has to agree with it, so a mismatch is not a redirect — it is a bundle whose
 * assets 404 and a nav link that leaves the app. It is also invisible in
 * development, where the dev server serves from the root.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Navigate, Route, Routes, useLocation } from "react-router-dom";

import { ApiClient } from "../api/client";
import { AppShell, type NavItem } from "./AppShell";
import { useNotifications } from "./notifications";

afterEach(() => {
  cleanup();
  localStorage.clear();
});

/** An ApiClient with a live session, without touching the network. */
async function signedIn(capabilities: string[] = ["workspace_ide"]): Promise<ApiClient> {
  const fetchImpl = vi.fn(async () =>
    new Response(
      JSON.stringify({
        token: "t0",
        name: "Michel",
        user_email: "michel@artisoft.io",
        is_admin: false,
        capabilities,
      }),
      { status: 200 },
    ),
  ) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  return api;
}

function Screen({ name }: { name: string }) {
  const { setError, setStatus } = useNotifications();
  return (
    <div>
      <p>screen: {name}</p>
      <button type="button" onClick={() => setError("something failed")}>
        raise error
      </button>
      <button type="button" onClick={() => setStatus("something worked")}>
        raise status
      </button>
    </div>
  );
}

/** Surfaces the in-app location so a test can assert the basename was stripped. */
function LocationProbe() {
  const location = useLocation();
  return <span data-testid="location">{location.pathname}</span>;
}

function renderShell(
  api: ApiClient,
  nav: NavItem[],
  { basename, initial = "/" }: { basename?: string; initial?: string } = {},
) {
  render(
    <MemoryRouter basename={basename ?? "/"} initialEntries={[initial]}>
      <LocationProbe />
      <Routes>
        <Route path="/" element={<AppShell api={api} nav={nav} />}>
          <Route index element={<Navigate to="/workspace" replace />} />
          <Route path="workspace" element={<Screen name="workspace" />} />
          <Route path="flows" element={<Screen name="flows" />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

const nav: NavItem[] = [
  { to: "/workspace", label: "Workspace IDE", capability: "workspace_ide" },
  { to: "/flows", label: "Flows" },
];

describe("AppShell", () => {
  it("shows the login screen when there is no session", () => {
    renderShell(new ApiClient(), nav);
    expect(screen.getByLabelText(/email/i)).toBeTruthy();
    // And nothing behind it: the gate is a gate, not an overlay.
    expect(screen.queryByRole("navigation")).toBeNull();
  });

  it("renders the chrome and the routed screen once signed in", async () => {
    renderShell(await signedIn(), nav);
    expect(await screen.findByText("screen: workspace")).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "Screens" })).toBeTruthy();
    expect(screen.getByText("Michel")).toBeTruthy();
  });

  it("navigates between screens without a reload", async () => {
    renderShell(await signedIn(), nav);
    await screen.findByText("screen: workspace");

    fireEvent.click(screen.getByRole("link", { name: "Flows" }));
    expect(await screen.findByText("screen: flows")).toBeTruthy();
    expect(screen.queryByText("screen: workspace")).toBeNull();
  });

  it("marks the current screen in the navigation", async () => {
    renderShell(await signedIn(), nav, { initial: "/flows" });
    await screen.findByText("screen: flows");
    expect(screen.getByRole("link", { name: "Flows" }).className).toContain("is-active");
    expect(screen.getByRole("link", { name: "Workspace IDE" }).className).not.toContain("is-active");
  });

  it("hides a nav item the user has no capability for", async () => {
    // Presentation only — the server is the enforcement point. Hiding a screen
    // the user cannot use is courtesy, and A.2 generalises it.
    renderShell(await signedIn([]), nav);
    await screen.findByText("screen: workspace");
    expect(screen.queryByRole("link", { name: "Workspace IDE" })).toBeNull();
    expect(screen.getByRole("link", { name: "Flows" })).toBeTruthy();
  });

  it("raises a screen's error into the shell's banner, and dismisses it", async () => {
    renderShell(await signedIn(), nav);
    await screen.findByText("screen: workspace");

    fireEvent.click(screen.getByRole("button", { name: "raise error" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("something failed");

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  it("lets an error supersede a status", async () => {
    // The success line from the previous action is not what the user needs to
    // read once something has failed.
    renderShell(await signedIn(), nav);
    await screen.findByText("screen: workspace");

    fireEvent.click(screen.getByRole("button", { name: "raise status" }));
    expect(await screen.findByRole("status")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "raise error" }));
    await screen.findByRole("alert");
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("returns to the login screen on sign out", async () => {
    renderShell(await signedIn(), nav);
    await screen.findByText("screen: workspace");

    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    await waitFor(() => expect(screen.getByLabelText(/email/i)).toBeTruthy());
  });

  it("remembers the theme across mounts", async () => {
    const api = await signedIn();
    renderShell(api, nav);
    await screen.findByText("screen: workspace");

    fireEvent.click(screen.getByRole("button", { name: "Toggle colour theme" }));
    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark"),
    );
    expect(localStorage.getItem("jetstore-ide-theme")).toBe("dark");

    cleanup();
    renderShell(api, nav);
    await screen.findByText("screen: workspace");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("resolves routes under the mount prefix, not the root", async () => {
    // The check that matters. The bundle is served at /ide/ and vite bakes that
    // into the asset urls; the router's basename has to agree. A mismatch is
    // invisible in development, where the dev server serves from the root.
    renderShell(await signedIn(), nav, { basename: "/ide", initial: "/ide/flows" });
    expect(await screen.findByText("screen: flows")).toBeTruthy();

    fireEvent.click(screen.getByRole("link", { name: "Flows" }));
    await waitFor(() =>
      expect(screen.getByTestId("location").textContent).toBe("/flows"),
    );

    // The href the browser sees carries the prefix; the location inside the app
    // does not. Losing the prefix here is how a nav link walks out of the app.
    const href = screen.getByRole("link", { name: "Flows" }).getAttribute("href");
    expect(href).toBe("/ide/flows");
  });
});
