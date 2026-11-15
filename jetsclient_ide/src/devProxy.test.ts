import { describe, expect, it } from "vitest";
import config, { isClientRoutePageLoad } from "../vite.config";

// The dev server proxies the api paths to an apiserver; `/register` and `/login`
// are also client routes, so a page load of either must stay on the dev server.
describe("dev proxy", () => {
  it("keeps a page load of a client route on the dev server", () => {
    expect(isClientRoutePageLoad("GET", "/register")).toBe(true);
    expect(isClientRoutePageLoad("GET", "/register?x=1")).toBe(true);
    expect(isClientRoutePageLoad("HEAD", "/login")).toBe(true);
    expect(isClientRoutePageLoad("GET", "/login/")).toBe(true);
  });

  it("proxies the api calls on the same paths", () => {
    expect(isClientRoutePageLoad("POST", "/register")).toBe(false);
    expect(isClientRoutePageLoad("OPTIONS", "/login")).toBe(false);
    // A prefix match is the proxy's; the bypass is for the exact route only.
    expect(isClientRoutePageLoad("GET", "/registerFileKey")).toBe(false);
    expect(isClientRoutePageLoad("GET", "/docs/user/")).toBe(false);
  });

  it("wires the bypass on the proxied paths", () => {
    type Bypass = (req: { method: string; url: string }) => string | undefined;
    const proxy = (config as { server?: { proxy?: Record<string, { bypass?: Bypass }> } }).server?.proxy ?? {};
    // A string result is the url vite serves itself (its html fallback); undefined proxies.
    expect(proxy["/register"]?.bypass?.({ method: "GET", url: "/register" })).toBe("/register");
    expect(proxy["/register"]?.bypass?.({ method: "POST", url: "/register" })).toBeUndefined();
    expect(proxy["/login"]?.bypass?.({ method: "GET", url: "/login" })).toBe("/login");
    expect(proxy["/dataTable"]?.bypass?.({ method: "POST", url: "/dataTable" })).toBeUndefined();
  });
});
