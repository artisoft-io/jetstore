/**
 * @vitest-environment jsdom
 *
 * Tests for the per-route favicon (task D.9, from **I-267**).
 *
 * **The one worth reading is the last: the static document names the JetStore
 * icon.** Everything else here tests the swap, and the swap is the half that
 * cannot regress silently — a wrong icon is visible. The default in
 * `index.html` is the half that *can*: it is outside the bundle, no test
 * imports it, and reverting it would reinstate exactly the reported defect on
 * `/register`, on any route reached before hydration, and on every route if this
 * module were ever unmounted. So it is asserted from the file.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MemoryRouter, Route, Routes, useNavigate } from "react-router-dom";

// `?raw` rather than `readFileSync`: this file runs under jsdom, where
// `import.meta.url` is an http url and `node:fs` cannot resolve it. Vite
// inlines the file at transform time, so the assertion below reads the same
// bytes the build entry does.
import indexHtml from "../../index.html?raw";

import { faviconFor, RouteFavicon } from "./favicon";

afterEach(() => {
  cleanup();
  document.querySelectorAll('link[rel="icon"]').forEach((l) => l.remove());
});

/** Mounts the component at a url, the way the shell does: under the router. */
function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="*" element={<RouteFavicon />} />
      </Routes>
    </MemoryRouter>,
  );
}

const href = () => document.querySelector('link[rel="icon"]')?.getAttribute("href");

describe("which icon a route gets", () => {
  it("gives the editor its own icon, bare and addressed at a workspace", () => {
    expect(faviconFor("/workspace").file).toBe("favicon.svg");
    expect(faviconFor("/workspaces/acme/home").file).toBe("favicon.svg");
  });

  it("gives every other route the JetStore icon", () => {
    for (const path of [
      "/",
      "/home",
      "/register",
      "/query-tool",
      "/proposals",
      "/flow/clientRegistryUF",
      "/ruleConfig",
      "/nothing-here",
    ]) {
      expect(faviconFor(path).file, path).toBe("favicon.ico");
    }
  });

  /*
    The registry is not the editor, and the two paths differ by one character of
    prefix — `/workspaces` against `/workspace` — which is what a `startsWith`
    would get wrong. `matchPath` is why this passes; the assertion is here so
    that swapping it for a cheaper comparison fails.
  */
  it("does not treat the workspace registry as the editor", () => {
    expect(faviconFor("/workspaces").file).toBe("favicon.ico");
  });
});

describe("the mounted component", () => {
  it("writes the editor's icon on an editor route", async () => {
    renderAt("/workspace");
    await waitFor(() => expect(href()).toBe(`${import.meta.env.BASE_URL}favicon.svg`));
    expect(document.querySelector('link[rel="icon"]')?.getAttribute("type")).toBe("image/svg+xml");
  });

  it("writes the JetStore icon elsewhere, carrying the asset prefix", async () => {
    renderAt("/home");
    await waitFor(() => expect(href()).toBe(`${import.meta.env.BASE_URL}favicon.ico`));
    expect(document.querySelector('link[rel="icon"]')?.getAttribute("type")).toBe("image/x-icon");
  });

  /*
    The direction that matters: the swap has to be reversible. A user reaching
    the editor and then leaving it is the ordinary case, and an effect that only
    ever upgraded would leave the editor's icon on the rest of the session —
    which is the reported defect again, arrived at by navigation instead of by
    the document.
  */
  it("swaps back when the user leaves the editor", async () => {
    function Leave() {
      const navigate = useNavigate();
      return <button onClick={() => navigate("/home")}>leave</button>;
    }
    render(
      <MemoryRouter initialEntries={["/workspace"]}>
        <RouteFavicon />
        <Routes>
          <Route path="*" element={<Leave />} />
        </Routes>
      </MemoryRouter>,
    );
    await waitFor(() => expect(href()).toBe(`${import.meta.env.BASE_URL}favicon.svg`));
    fireEvent.click(screen.getByText("leave"));
    await waitFor(() => expect(href()).toBe(`${import.meta.env.BASE_URL}favicon.ico`));
  });

  it("reuses the document's link element rather than adding a second", async () => {
    const existing = document.createElement("link");
    existing.setAttribute("rel", "icon");
    existing.setAttribute("href", "/stale.png");
    document.head.appendChild(existing);

    renderAt("/home");
    await waitFor(() => expect(href()).toBe(`${import.meta.env.BASE_URL}favicon.ico`));
    expect(document.querySelectorAll('link[rel="icon"]')).toHaveLength(1);
  });
});

describe("the static document", () => {
  /*
    Asserted from the file because nothing else can. `index.html` is not
    imported by the bundle — vite reads it as the build entry — so it is the one
    part of this fix with no compiler and no other test behind it, and it is the
    part that decides what a cold load and the shell-less /register route show.
  */
  it("declares the JetStore icon, which is what every unswapped route gets", () => {
    const links = indexHtml.match(/<link\b[^>]*rel="icon"[^>]*>/g) ?? [];
    expect(links).toEqual(['<link rel="icon" type="image/x-icon" href="/favicon.ico" />']);
  });
});
