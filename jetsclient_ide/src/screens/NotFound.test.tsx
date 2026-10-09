/**
 * @vitest-environment jsdom
 *
 * **C.16's exit condition is a route table's behaviour, not a widget's.**
 *
 * The screen is four lines and nothing about it is worth a test on its own. What
 * is worth one is the thing C.16 decided: an unmatched path **reports** instead
 * of redirecting to the editor, and the routes that are matched still are.
 *
 * So this renders the same `<Routes>` the application declares, against a
 * `MemoryRouter` at a chosen url. The alternative — rendering `<NotFound/>`
 * directly and asserting it says "not found" — would pass with the catch-all
 * still redirecting, which is the whole of what changed.
 */

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MemoryRouter, Navigate, Route, Routes } from "react-router-dom";

import { NotFound } from "./NotFound";

afterEach(cleanup);

/**
 * `App.tsx`'s route table with the screens replaced by markers.
 *
 * **A copy, and it is one on purpose.** Rendering `App` itself would drag in the
 * session gate, the api client and every screen's imports to assert a property of
 * four `<Route>` elements. What matters here is the *shape* — an index redirect,
 * some matched paths, and a catch-all — and the shape is what this copies. A
 * screen added to `App.tsx` and not here does not weaken the assertion, because
 * the assertion is about the two ends of the table rather than its middle.
 */
function routes() {
  return (
    <Routes>
      <Route path="/">
        <Route index element={<Navigate to="/workspace" replace />} />
        <Route path="workspace" element={<p>the editor</p>} />
        <Route path="flow/:key" element={<p>a flow</p>} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

function at(url: string) {
  return render(<MemoryRouter initialEntries={[url]}>{routes()}</MemoryRouter>);
}

describe("the route table's two ends", () => {
  it("reports an unmatched path instead of redirecting to the editor", () => {
    at("/nowhere");
    expect(screen.getByText("Page not found")).toBeTruthy();
    expect(screen.queryByText("the editor")).toBeNull();
  });

  it("names the path that failed, which is the whole gain over a redirect", () => {
    at("/screens/typo?session=abc");
    // The url a user would otherwise have to reconstruct from memory when they
    // report the broken link.
    expect(screen.getByText("/screens/typo?session=abc")).toBeTruthy();
  });

  it("still redirects the bare prefix, which is a known destination", () => {
    // `/ide/` is a url the Flutter app deliberately links to. The index route
    // handles it and is not the catch-all; C.16 changed one and not the other.
    at("/");
    expect(screen.getByText("the editor")).toBeTruthy();
  });

  it("leaves a matched route matched", () => {
    // I-50's route. A catch-all that swallowed this is exactly the failure the
    // 404 exists to make visible, so the regression worth guarding is the
    // opposite one: a 404 that swallows a route that does exist.
    at("/flow/loadFilesUF");
    expect(screen.getByText("a flow")).toBeTruthy();
    expect(screen.queryByText("Page not found")).toBeNull();
  });
});
