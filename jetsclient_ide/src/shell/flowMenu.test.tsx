/**
 * @vitest-environment jsdom
 *
 * *User Administration* in the *JetStore Workspace* menu (2026-11-07).
 *
 * The menu is `FLOW_MENU` in `App.tsx`, rendered by the shell's `FlowMenu`. Its
 * last entry declares `user_admin`, the capability the screen's two writes
 * require server-side; the entry is a link for a user who holds it and for the
 * admin, whom `can` passes for every capability, and an inert item naming the
 * capability for everyone else — disabled, not hidden, as every other gated
 * control in the shell is (`capabilities.tsx`).
 */

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";

import { ApiClient } from "../api/client";
import { FLOW_MENU } from "../App";
import { USER_ADMIN } from "../screens/UserAdmin";
import { AppShell } from "./AppShell";
import { resetSelectedClient } from "./selectedClient";

afterEach(() => {
  cleanup();
  resetSelectedClient();
});

async function mount(options: { isAdmin: boolean; capabilities: string[] }) {
  const fetchImpl = vi.fn(async (url: string | URL) => {
    if (String(url) === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Someone",
          user_email: "someone@example.com",
          is_admin: options.isAdmin,
          capabilities: options.capabilities,
        }),
        { status: 200 },
      );
    }
    return new Response(JSON.stringify({ rows: [] }), { status: 200 });
  }) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("someone@example.com", "pw");
  render(
    <MemoryRouter initialEntries={["/home"]}>
      <AppShell api={api} nav={[]} flowMenu={FLOW_MENU} />
    </MemoryRouter>,
  );
  await screen.findByLabelText("Open a configuration flow");
}

const entry = () => screen.getByText("User Administration");

describe("the JetStore Workspace menu", () => {
  it("ends with User Administration, on /userAdmin, gated on user_admin", () => {
    const last = FLOW_MENU[FLOW_MENU.length - 1]!;
    expect(last).toEqual({ to: "/userAdmin", label: "User Administration", capability: USER_ADMIN });
    expect(USER_ADMIN).toBe("user_admin");
    // The one entry that declares a capability: the others are open to every
    // signed-in user, and gated where they act.
    expect(FLOW_MENU.filter((i) => i.capability !== undefined)).toEqual([last]);
  });

  it("renders it last in the menu", async () => {
    await mount({ isAdmin: false, capabilities: [USER_ADMIN] });
    const items = [...screen.getByLabelText("Configuration flows").children];
    expect(items.map((el) => el.textContent)).toEqual(FLOW_MENU.map((i) => i.label));
    expect(items[items.length - 1]!.textContent).toBe("User Administration");
  });

  it("is a link for a user holding user_admin", async () => {
    await mount({ isAdmin: false, capabilities: [USER_ADMIN] });
    expect(entry().tagName).toBe("A");
    expect(entry().getAttribute("href")).toContain("/userAdmin");
    expect(entry().getAttribute("aria-disabled")).toBeNull();
  });

  it("is a link for the admin, who holds no capability and passes every check", async () => {
    await mount({ isAdmin: true, capabilities: [] });
    expect(entry().tagName).toBe("A");
    expect(entry().getAttribute("href")).toContain("/userAdmin");
  });

  it("is disabled, not hidden, and names the capability, for a user without it", async () => {
    // `user_profile` is what every seeded role holds; it does not open this entry.
    await mount({ isAdmin: false, capabilities: ["jetstore_read", "user_profile"] });
    expect(entry().tagName).not.toBe("A");
    expect(entry().getAttribute("href")).toBeNull();
    expect(entry().getAttribute("aria-disabled")).toBe("true");
    expect(entry().getAttribute("title")).toContain(USER_ADMIN);
    // The entries that declare nothing stay links for the same user.
    expect(screen.getByText("Clients & Vendors").tagName).toBe("A");
  });
});
