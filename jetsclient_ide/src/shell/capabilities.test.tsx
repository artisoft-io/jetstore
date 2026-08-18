/**
 * @vitest-environment jsdom
 *
 * Tests for the capability model (task A.2).
 *
 * Two things are being pinned. The **policy** — what an absent, held, missing or
 * empty capability means — and the **treatment**, which is that a control the
 * user cannot use is disabled and still visible. The treatment is the part A.1
 * got wrong, so it is asserted rather than left to inspection.
 */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ActionButton, ApiProvider, permissionFor, useCan } from "./capabilities";

afterEach(cleanup);

async function signedIn(capabilities: string[], isAdmin = false): Promise<ApiClient> {
  const fetchImpl = vi.fn(async () =>
    new Response(
      JSON.stringify({
        token: "t0",
        name: "Michel",
        user_email: "michel@artisoft.io",
        is_admin: isAdmin,
        capabilities,
      }),
      { status: 200 },
    ),
  ) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("michel@artisoft.io", "pw");
  return api;
}

describe("permissionFor", () => {
  it("allows a control that names no capability", async () => {
    // 33 of the 42 gated sites in the Flutter app simply omit it.
    const api = await signedIn([]);
    expect(permissionFor(api, undefined)).toEqual({ allowed: true });
  });

  it("allows a capability the user holds", async () => {
    const api = await signedIn(["workspace_ide"]);
    expect(permissionFor(api, "workspace_ide").allowed).toBe(true);
  });

  it("denies one the user lacks, and names it", async () => {
    const api = await signedIn(["run_pipelines"]);
    const permission = permissionFor(api, "workspace_ide");
    expect(permission.allowed).toBe(false);
    // The Flutter app disables silently; naming the capability is what makes the
    // disabled control actionable rather than mysterious.
    expect(permission.reason).toContain("workspace_ide");
  });

  it("allows an admin anything named", async () => {
    const api = await signedIn([], true);
    expect(permissionFor(api, "client_config").allowed).toBe(true);
  });

  it("denies an empty capability, even for an admin", async () => {
    // A configuration error rather than an absent requirement, matching the
    // server: "configuration error: missing capability on sql statement".
    // The Flutter app would let an admin through, because its callers test
    // isAdmin before consulting the capability; no configuration in the corpus
    // exercises the difference.
    const api = await signedIn([], true);
    const permission = permissionFor(api, "");
    expect(permission.allowed).toBe(false);
    expect(permission.reason).toContain("Misconfigured");
  });

  it("denies everything when there is no session", () => {
    expect(permissionFor(new ApiClient(), "workspace_ide").allowed).toBe(false);
  });
});

function Probe({ capability }: { capability?: string }) {
  const permission = useCan(capability);
  return <span>{permission.allowed ? "yes" : "no"}</span>;
}

describe("useCan", () => {
  it("reads through the shell's api", async () => {
    const api = await signedIn(["workspace_ide"]);
    render(
      <ApiProvider api={api}>
        <Probe capability="workspace_ide" />
      </ApiProvider>,
    );
    expect(screen.getByText("yes")).toBeTruthy();
  });

  it("throws outside the shell rather than answering wrongly", () => {
    // Answering "denied" would look like a permissions problem and send someone
    // to the database; answering "allowed" would be worse.
    const quiet = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Probe capability="workspace_ide" />)).toThrow(/inside the app shell/);
    quiet.mockRestore();
  });
});

describe("ActionButton", () => {
  it("is usable when the capability is held", async () => {
    const api = await signedIn(["workspace_ide"]);
    const onClick = vi.fn();
    render(
      <ApiProvider api={api}>
        <ActionButton capability="workspace_ide" onClick={onClick}>
          Save
        </ActionButton>
      </ApiProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onClick).toHaveBeenCalled();
  });

  it("is visible but inert without it, and says why", async () => {
    const api = await signedIn([]);
    const onClick = vi.fn();
    render(
      <ApiProvider api={api}>
        <ActionButton capability="workspace_ide" onClick={onClick}>
          Save
        </ActionButton>
      </ApiProvider>,
    );
    const button = screen.getByRole("button", { name: "Save" });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.getAttribute("title")).toContain("workspace_ide");
    fireEvent.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("composes with the caller's own disabled reason", async () => {
    // A pristine form and a missing capability both suppress the action; only
    // the capability explains itself, because it is the one the user can act on.
    const api = await signedIn(["workspace_ide"]);
    render(
      <ApiProvider api={api}>
        <ActionButton capability="workspace_ide" disabled>
          Save
        </ActionButton>
      </ApiProvider>,
    );
    const button = screen.getByRole("button", { name: "Save" });
    expect(button.hasAttribute("disabled")).toBe(true);
    expect(button.getAttribute("title")).toBeNull();
  });
});
