/**
 * @vitest-environment jsdom
 *
 * The git profile screen. Task C.14.
 *
 * **Driven through a stubbed `fetch` with everything below `ApiClient` real**,
 * which is `FlowRunner.test.tsx`'s shape and I-104's rule: the document-level
 * checks a screen port passes are not the ones that catch a missing binding, and
 * only a test that renders can see what the screen actually sends.
 *
 * The three things worth asserting are the three decisions:
 *
 *  - **the form is seeded from the session**, not from route parameters, which is
 *    what makes the parameterless route correct rather than merely tidier;
 *  - **`user_email` in the request is the signed-in account's**, which is the
 *    half of I-187 this app controls — the server takes whoever the request
 *    names;
 *  - **the confirmation field is not sent**, because the statement has no column
 *    for it and it is a second copy of a credential.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient } from "../api/client";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { GitProfileScreen, validateDraft } from "./GitProfile";

afterEach(cleanup);

interface Options {
  capabilities?: string[];
  /** The `gitProfile` the sign-in response carries; omitted means none at all. */
  gitProfile?: Record<string, string>;
  status?: number;
}

async function clientWith(options: Options = {}) {
  const sent: Record<string, unknown>[] = [];
  const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
    if (String(url).endsWith("/login")) {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Ada",
          user_email: "ada@example.com",
          is_admin: false,
          capabilities: options.capabilities ?? ["user_profile"],
          ...(options.gitProfile ? { gitProfile: options.gitProfile } : {}),
        }),
        { status: 200 },
      );
    }
    sent.push(JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>);
    return new Response(JSON.stringify({ token: "t1" }), { status: options.status ?? 200 });
  }) as unknown as typeof fetch;
  const api = new ApiClient("", fetchImpl);
  await api.login("ada@example.com", "pw");
  return { api, sent };
}

/**
 * The shell's banner, in miniature.
 *
 * `AppShell` renders the notification banners and the screen only raises them,
 * so a harness that omits this asserts on field errors and calls them server
 * errors — which the first version of this file did, and passed.
 */
function Banner() {
  const { error, status } = useNotifications();
  return (
    <>
      {error != null && <p role="alert">{error}</p>}
      {status != null && <p role="status">{status}</p>}
    </>
  );
}

function renderScreen(api: ApiClient) {
  return render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <Banner />
        <MemoryRouter initialEntries={["/git-profile"]}>
          <Routes>
            <Route path="/git-profile" element={<GitProfileScreen api={api} />} />
            <Route path="/" element={<p>the workspace</p>} />
          </Routes>
        </MemoryRouter>
      </NotificationsProvider>
    </ApiProvider>,
  );
}

function field(label: string): HTMLInputElement {
  return screen.getByLabelText(label) as HTMLInputElement;
}

function type(label: string, value: string): void {
  fireEvent.change(field(label), { target: { value } });
}

/** Fills the form with a valid profile, so a test can change one thing. */
function fillValid(): void {
  type("Name", "Ada Lovelace");
  // Four characters, not three: `gitProfileFormValidator` wants `> 3`, and a
  // three-character handle here is what made the first version of this file
  // silently assert nothing.
  type("Git Handle", "adal");
  type("Email", "ada@example.com");
  type("Github Token", "ghp_secret");
  type("Github Token Confirmation", "ghp_secret");
}

const PROFILE = { git_name: "Ada L", git_email: "ada@git.example", git_handle: "adal" };

describe("the git profile screen", () => {
  it("seeds the three fields from the session rather than from the url", async () => {
    const { api } = await clientWith({ gitProfile: PROFILE });
    renderScreen(api);

    expect(field("Name").value).toBe("Ada L");
    expect(field("Email").value).toBe("ada@git.example");
    expect(field("Git Handle").value).toBe("adal");
    // The token is never seeded: the sign-in response carries one and this app
    // deliberately does not keep it (`api/client.ts`, `LoginResponse.gitProfile`).
    expect(field("Github Token").value).toBe("");
  });

  it("renders empty fields for a user whose sign-in carried no profile", async () => {
    const { api } = await clientWith();
    renderScreen(api);
    expect(field("Name").value).toBe("");
    expect(field("Git Handle").value).toBe("");
  });

  it("sends the statement's five columns, with user_email from the session", async () => {
    const { api, sent } = await clientWith({ gitProfile: PROFILE });
    renderScreen(api);
    fillValid();
    fireEvent.click(screen.getByRole("button", { name: "Submit" }));

    await waitFor(() => expect(sent).toHaveLength(1));
    const request = sent[0]!;
    expect(request["action"]).toBe("insert_rows");
    expect(request["fromClauses"]).toEqual([{ table: "update/user_git_profile" }]);
    expect(request["data"]).toEqual([
      {
        git_name: "Ada Lovelace",
        git_email: "ada@example.com",
        git_handle: "adal",
        git_token: "ghp_secret",
        // Not the four route parameters' `user_email`, which this route does not
        // have — the signed-in account's, which is what the Flutter delegate
        // overwrites its own form state with before posting.
        user_email: "ada@example.com",
      },
    ]);
    // The confirmation is a form-local rule and has no column. The Dart posts the
    // whole of form-state group 0 and therefore sends it; this does not.
    const data = (request["data"] as Record<string, unknown>[])[0]!;
    expect(Object.keys(data)).not.toContain("git_token.confirm");
    expect(Object.keys(data)).not.toContain("gitTokenConfirm");
  });

  it("leaves the screen and updates the held profile on success", async () => {
    const { api } = await clientWith({ gitProfile: PROFILE });
    renderScreen(api);
    fillValid();
    fireEvent.click(screen.getByRole("button", { name: "Submit" }));

    await screen.findByText("the workspace");
    expect(api.currentUser?.gitProfile).toEqual({
      gitName: "Ada Lovelace",
      gitEmail: "ada@example.com",
      gitHandle: "adal",
    });
  });

  it("refuses to send an invalid form and says why", async () => {
    const { api, sent } = await clientWith();
    renderScreen(api);
    // Everything valid but the confirmation, which is the one rule that is not a
    // length: the two token fields must agree.
    fillValid();
    type("Github Token Confirmation", "ghp_other");
    fireEvent.click(screen.getByRole("button", { name: "Submit" }));

    await screen.findByText("Git tokens does not match.");
    expect(sent).toHaveLength(0);
  });

  it("shows no errors before the first submit", async () => {
    const { api } = await clientWith();
    renderScreen(api);
    expect(screen.queryByText("Name must be provided.")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Submit" }));
    await screen.findByText("Name must be provided.");
  });

  it("lower-cases the handle and the address as they are typed", async () => {
    const { api } = await clientWith();
    renderScreen(api);
    type("Git Handle", "AdaL");
    type("Email", "Ada@Example.COM");
    type("Name", "Ada Lovelace");
    expect(field("Git Handle").value).toBe("adal");
    expect(field("Email").value).toBe("ada@example.com");
    // `none` on the name, so a capital survives there.
    expect(field("Name").value).toBe("Ada Lovelace");
  });

  it("reports a server refusal without leaving the screen", async () => {
    // 422, not 401: the api client turns *every* 401 into a session expiry and
    // signs the user out, so a capability refusal from this endpoint would end
    // the session rather than report anything. That is `client.ts`'s policy and
    // it is recorded as **I-189** rather than worked around here.
    const { api } = await clientWith({ status: 422 });
    renderScreen(api);
    fillValid();
    fireEvent.click(screen.getByRole("button", { name: "Submit" }));

    // The banner, not a field error — both carry `role="alert"`, so naming the
    // text is what makes this test about the server's refusal.
    await screen.findByText(/Saving the git profile failed/);
    expect(screen.queryByText("the workspace")).toBeNull();
  });

  it("disables Submit for a user without the user_profile capability", async () => {
    const { api } = await clientWith({ capabilities: ["jetstore_read"] });
    renderScreen(api);
    const submit = screen.getByRole("button", { name: "Submit" }) as HTMLButtonElement;
    expect(submit.disabled).toBe(true);
    // Visible and inert rather than absent, and it says which capability —
    // `shell/capabilities.tsx`.
    expect(submit.getAttribute("title")).toContain("user_profile");
  });
});

describe("validateDraft", () => {
  const valid = {
    gitName: "Ada",
    gitEmail: "ada@x.example",
    gitHandle: "adal",
    gitToken: "ghp_secret",
    gitTokenConfirm: "ghp_secret",
  };

  it("accepts a filled profile", () => {
    expect(validateDraft(valid)).toEqual({});
  });

  it("distinguishes a name that is missing from one that is too short", () => {
    expect(validateDraft({ ...valid, gitName: "" }).gitName).toBe("Name must be provided.");
    expect(validateDraft({ ...valid, gitName: "A" }).gitName).toBe("Name is too short.");
  });

  it("keeps the Dart's thresholds rather than rounding them", () => {
    // Four characters pass, three do not — `> 3` in `gitProfileFormValidator`.
    expect(validateDraft({ ...valid, gitHandle: "abcd" }).gitHandle).toBeUndefined();
    expect(validateDraft({ ...valid, gitHandle: "abc" }).gitHandle).toBeDefined();
    // Six pass, five do not — `> 5` on the token.
    expect(validateDraft({ ...valid, gitToken: "abcdef", gitTokenConfirm: "abcdef" }).gitToken).toBeUndefined();
    expect(validateDraft({ ...valid, gitToken: "abcde", gitTokenConfirm: "abcde" }).gitToken).toBeDefined();
  });

  it("requires the token even when only the name is changing", () => {
    // The update writes `git_token` unconditionally, so a blank one wipes the
    // stored token rather than leaving it alone.
    expect(validateDraft({ ...valid, gitToken: "", gitTokenConfirm: "" }).gitToken).toBeDefined();
  });
});
