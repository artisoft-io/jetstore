import { describe, expect, it } from "vitest";
import { ApiClient, ApiError, PermissionDeniedError, SessionExpiredError } from "./client";

interface Call {
  url: string;
  headers: Record<string, string>;
  body: unknown;
}

/** A fetch stand-in that records calls and replays queued responses. */
function stubFetch(queue: Array<{ status: number; body: unknown }>) {
  const calls: Call[] = [];
  const impl = (async (url: string, init?: RequestInit) => {
    calls.push({
      url: String(url),
      headers: (init?.headers ?? {}) as Record<string, string>,
      body: init?.body ? JSON.parse(String(init.body)) : undefined,
    });
    const next = queue.shift() ?? { status: 200, body: {} };
    return {
      ok: next.status >= 200 && next.status < 300,
      status: next.status,
      text: async () => (typeof next.body === "string" ? next.body : JSON.stringify(next.body)),
    } as Response;
  }) as unknown as typeof fetch;
  return { impl, calls };
}

const LOGIN_OK = {
  status: 200,
  body: {
    name: "Ada",
    user_email: "ada@example.com",
    is_admin: false,
    capabilities: ["workspace_ide"],
    token: "token-1",
    jetstore_version: "1.2.3",
  },
};

async function signedIn(queue: Array<{ status: number; body: unknown }>) {
  const { impl, calls } = stubFetch([LOGIN_OK, ...queue]);
  const api = new ApiClient("", impl);
  await api.login("ada@example.com", "pw");
  return { api, calls };
}

describe("ApiClient", () => {
  it("signs in and exposes the user", async () => {
    const { api, calls } = await signedIn([]);
    expect(api.isAuthenticated).toBe(true);
    expect(api.currentUser?.email).toBe("ada@example.com");
    expect(api.currentUser?.capabilities).toEqual(["workspace_ide"]);
    expect(calls[0]?.url).toBe("/login");
  });

  it("posts the email as user_email, the field the server unmarshals", () => {
    // Regression: this was `email`, which unmarshals into user.User as an empty
    // Email — the server then answers "required Email" for a form that plainly
    // had one, and the audit log records a login for user "". The tag is
    // `json:"user_email"` in jets/user/user.go, and the Flutter client posts the
    // same key. Asserting the wire field rather than the argument name is the
    // whole point of this test.
    return signedIn([]).then(({ calls }) => {
      expect(calls[0]?.body).toEqual({ user_email: "ada@example.com", password: "pw" });
      expect(calls[0]?.body).not.toHaveProperty("email");
    });
  });

  it("rejects a login that returns no token", async () => {
    const { impl } = stubFetch([{ status: 200, body: { user_email: "a@b.c" } }]);
    const api = new ApiClient("", impl);
    await expect(api.login("a@b.c", "pw")).rejects.toThrow(/no token/i);
    expect(api.isAuthenticated).toBe(false);
  });

  it("surfaces the server's error text on a failed login", async () => {
    const { impl } = stubFetch([{ status: 422, body: { error: "Invalid User or Password" } }]);
    const api = new ApiClient("", impl);
    await expect(api.login("a@b.c", "nope")).rejects.toThrow("Invalid User or Password");
  });

  it("sends the bearer token on dataTable calls", async () => {
    const { api, calls } = await signedIn([{ status: 200, body: { ok: true } }]);
    await api.dataTable({ action: "read" });
    expect(calls[1]?.url).toBe("/dataTable");
    expect(calls[1]?.headers["Authorization"]).toBe("Bearer token-1");
  });

  it("adopts the refreshed token from each response", async () => {
    // The server mints a new token per request; a client that ignores it will
    // eventually present a stale one, so this is the behaviour worth pinning.
    const { api, calls } = await signedIn([
      { status: 200, body: { token: "token-2" } },
      { status: 200, body: { token: "token-3" } },
    ]);
    await api.dataTable({ action: "read" });
    expect(calls[1]?.headers["Authorization"]).toBe("Bearer token-1");
    await api.dataTable({ action: "read" });
    expect(calls[2]?.headers["Authorization"]).toBe("Bearer token-2");
  });

  it("strips the token out of the body it returns", async () => {
    const { api } = await signedIn([
      { status: 200, body: { token: "token-2", file_content: "hello" } },
    ]);
    const body = await api.dataTable<Record<string, unknown>>({ action: "x" });
    expect(body["file_content"]).toBe("hello");
    expect(body["token"]).toBeUndefined();
  });

  it("treats 401 as the end of the session", async () => {
    const { api } = await signedIn([{ status: 401, body: { error: "Unauthorized" } }]);
    await expect(api.dataTable({ action: "read" })).rejects.toBeInstanceOf(SessionExpiredError);
    expect(api.isAuthenticated).toBe(false);
    expect(api.currentUser).toBeNull();
  });

  // C.17 / I-189: a capability refusal used to arrive as a 401 and sign the user
  // out. These four pin the contract on this side of it.
  //
  // **They were run against the pre-C.17 client first, and two of them passed** —
  // which is C.4/C.14's sixth failure mode in `src/actions/README.md` (jetstore#2028,
  // unmerged at the time of writing) caught in the act,
  // and is worth recording rather than quietly fixing. Deleting the 403 branch from
  // `post` failed only *treats 403 as a refusal* and *falls back to a readable
  // message*; the other two went green against a plain `ApiError`, because
  // `errorText` already surfaced the body's message and `post` already left the
  // session alone on any status but 401. Both were rewritten to assert
  // `PermissionDeniedError` rather than its effects, and both then failed.
  //
  // **And the assertion that reads as the heart of the defect is the weakest one
  // here.** `api.isAuthenticated` staying true was true before C.17 too: the client
  // never signed anyone out on a 403, because the server never sent one. The defect
  // was that a refusal arrived as a 401, so **nothing on this side can reproduce
  // it** — the guards that would have failed on the day are in `jets/datatable` and
  // `jets/apiserver`, not here.
  it("treats 403 as a refusal and keeps the session", async () => {
    const { api } = await signedIn([
      {
        status: 403,
        body: { error: "error: unauthorized, cannot get user info or does not have permission" },
      },
    ]);
    await expect(api.dataTable({ action: "insert_rows" })).rejects.toBeInstanceOf(
      PermissionDeniedError,
    );
    expect(api.isAuthenticated).toBe(true);
    expect(api.currentUser).not.toBeNull();
  });

  it("carries the server's refusal message, which is the part that names the capability", async () => {
    const { api } = await signedIn([
      { status: 403, body: { error: "error: unauthorized, user do not have required capability" } },
    ]);
    const err = await api.dataTable({ action: "drop_table" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PermissionDeniedError);
    expect((err as Error).message).toContain("user do not have required capability");
  });

  // A refusal is an ApiError, so a caller that catches the base class is unaffected;
  // and it is *not* a SessionExpiredError, which is what every existing consumer
  // discriminates on — `WorkspaceIde`'s guard, and agentic_ai's two proposal screens.
  // Neither needed an edit, and this test is why that claim is checkable.
  it("makes a refusal an ApiError and not a SessionExpiredError", async () => {
    const { api } = await signedIn([{ status: 403, body: { error: "no" } }]);
    const err = await api.dataTable({ action: "x" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PermissionDeniedError);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).not.toBeInstanceOf(SessionExpiredError);
    expect((err as ApiError).status).toBe(403);
  });

  it("falls back to a readable message when a 403 carries no body", async () => {
    const { api } = await signedIn([{ status: 403, body: "" }]);
    await expect(api.dataTable({ action: "x" })).rejects.toThrow(/do not have permission/);
  });

  it("refuses to call dataTable without a session", async () => {
    const { impl } = stubFetch([]);
    const api = new ApiClient("", impl);
    await expect(api.dataTable({ action: "read" })).rejects.toBeInstanceOf(SessionExpiredError);
  });

  it("reports a non-401 failure with the server's message", async () => {
    const { api } = await signedIn([{ status: 400, body: { error: "not a valid json file" } }]);
    await expect(api.dataTable({ action: "save" })).rejects.toThrow("not a valid json file");
    // A 400 is about the request, not the session; stay signed in.
    expect(api.isAuthenticated).toBe(true);
  });

  it("survives a non-json error body", async () => {
    const { api } = await signedIn([{ status: 502, body: "<html>bad gateway</html>" }]);
    await expect(api.dataTable({ action: "read" })).rejects.toThrow(/bad gateway/);
  });

  it("resolves capabilities, with admin passing everything", async () => {
    const { api } = await signedIn([]);
    expect(api.can("workspace_ide")).toBe(true);
    expect(api.can("run_pipelines")).toBe(false);

    const { impl } = stubFetch([
      { status: 200, body: { user_email: "root@x", is_admin: true, capabilities: [], token: "t" } },
    ]);
    const admin = new ApiClient("", impl);
    await admin.login("root@x", "pw");
    expect(admin.can("anything_at_all")).toBe(true);
  });

  it("notifies subscribers on sign-in and sign-out", async () => {
    const seen: Array<string | null> = [];
    const { impl } = stubFetch([LOGIN_OK]);
    const api = new ApiClient("", impl);
    api.subscribe((u) => seen.push(u?.email ?? null));
    await api.login("ada@example.com", "pw");
    api.logout();
    expect(seen).toEqual(["ada@example.com", null]);
  });
});
