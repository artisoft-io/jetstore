/**
 * The single way this app talks to the Go apiserver.
 *
 * Three behaviours of that server drive the shape of this file, and all three are
 * load-bearing rather than incidental:
 *
 *  - Auth is `Authorization: Bearer <token>`, parsed in jets/user/token.go.
 *  - **Every authenticated response carries a refreshed token.** `authh` in
 *    jets/apiserver/server.go mints one on each request, so a client that does not
 *    read `token` out of each response body will eventually present a stale one.
 *    Sessions therefore last as long as the user keeps working, and stop lasting
 *    the moment they stop.
 *  - A 401 means the session is over; there is no refresh endpoint to fall back to.
 *
 * The token is deliberately kept in memory only. Putting it in localStorage would
 * outlive the tab and widen the blast radius of any script injection, and the
 * refresh-per-response behaviour above means a reload costs one login rather than a
 * lost day's work.
 */

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** Thrown on 401 so callers can drop to the login screen without sniffing status codes. */
export class SessionExpiredError extends ApiError {
  constructor() {
    super("Session expired, please sign in again.", 401);
    this.name = "SessionExpiredError";
  }
}

export interface User {
  name: string;
  email: string;
  isAdmin: boolean;
  capabilities: string[];
  jetstoreVersion: string;
  devMode: boolean;
}

/** Shape of the /login response (jets/apiserver/api_users.go). */
interface LoginResponse {
  name?: string;
  user_email?: string;
  is_admin?: boolean;
  capabilities?: string[] | null;
  token?: string;
  dev_mode?: string;
  jetstore_version?: string;
}

type Listener = (user: User | null) => void;

export class ApiClient {
  private token: string | null = null;
  private user: User | null = null;
  private listeners = new Set<Listener>();

  /**
   * `fetchImpl` is injectable so the tests can drive this class without a network
   * or a DOM; production callers use the default.
   */
  constructor(
    private readonly baseUrl = "",
    private readonly fetchImpl: typeof fetch = globalThis.fetch.bind(globalThis),
  ) {}

  get currentUser(): User | null {
    return this.user;
  }

  get isAuthenticated(): boolean {
    return this.token !== null;
  }

  /** Notified on sign-in and sign-out so React can re-render the shell. */
  subscribe(fn: Listener): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  private emit(): void {
    for (const fn of this.listeners) fn(this.user);
  }

  async login(email: string, password: string): Promise<User> {
    const res = await this.fetchImpl(`${this.baseUrl}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      // The field is `user_email`, not `email`: /login unmarshals straight into
      // user.User, whose Email carries `json:"user_email"` (jets/user/user.go).
      // Sending `email` unmarshals to an empty string and the server answers
      // "required Email" while the form plainly had one in it.
      body: JSON.stringify({ user_email: email, password }),
    });
    const body = (await this.readJson(res)) as LoginResponse;
    if (!res.ok) {
      throw new ApiError(this.errorText(body, "Sign in failed."), res.status);
    }
    if (!body.token) {
      throw new ApiError("Sign in succeeded but returned no token.", res.status);
    }
    this.token = body.token;
    this.user = {
      name: body.name ?? "",
      email: body.user_email ?? email,
      isAdmin: body.is_admin === true,
      capabilities: body.capabilities ?? [],
      jetstoreVersion: body.jetstore_version ?? "",
      devMode: body.dev_mode === "true",
    };
    this.emit();
    return this.user;
  }

  logout(): void {
    this.token = null;
    this.user = null;
    this.emit();
  }

  /** True when the signed-in user holds a capability. Admin passes everything. */
  can(capability: string): boolean {
    if (!this.user) return false;
    return this.user.isAdmin || this.user.capabilities.includes(capability);
  }

  /**
   * POST to /dataTable, the action multiplexer (see the switch in
   * jets/apiserver/api_tables.go). Returns the decoded body with the refreshed
   * token already consumed and stripped.
   */
  async dataTable<T = Record<string, unknown>>(
    payload: Record<string, unknown>,
  ): Promise<T> {
    return this.post<T>("/dataTable", payload);
  }

  /**
   * POST to /agentic, the agentic supervision endpoint — proposal staging,
   * approval decisions and run transcripts (the switch in
   * jets/apiserver/api_agentic.go). Same envelope shape as /dataTable: an
   * `action` name and its arguments.
   *
   * Added by the agentic_ai project, whose screens live in `src/proposals/`;
   * this method is the whole of its footprint on the api client.
   */
  async agentic<T = Record<string, unknown>>(
    payload: Record<string, unknown>,
  ): Promise<T> {
    return this.post<T>("/agentic", payload);
  }

  /**
   * POST to one of the endpoints an authored action may name. Task F.0a.
   *
   * `dataTable` was the only public poster, and a flow's `post` step names an
   * endpoint: two flows in the corpus use `/registerFileKey` rather than
   * `/dataTable` (`actions/schema.ts`, `EndpointSchema`).
   *
   * **The allowlist stays in the schema and is not repeated here.** S.7 narrowed
   * the endpoint to two values *in the document type*, so a `.ua.json` naming
   * anything else does not parse and never reaches this method — and the Go
   * validator enforces the same enum at save time. A second list here would be a
   * second place to update and the one more likely to be forgotten, which is the
   * failure mode S.8's migrated-flow list is a standing example of.
   *
   * `agentic` above is the same shape for a fixed path; the two do not merge,
   * because that one names its endpoint and this one is handed one.
   */
  async endpoint<T = Record<string, unknown>>(
    path: string,
    payload: Record<string, unknown>,
  ): Promise<T> {
    return this.post<T>(path, payload);
  }

  private async post<T>(path: string, payload: Record<string, unknown>): Promise<T> {
    if (!this.token) throw new SessionExpiredError();
    const res = await this.fetchImpl(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.token}`,
      },
      body: JSON.stringify(payload),
    });

    if (res.status === 401) {
      this.logout();
      throw new SessionExpiredError();
    }

    const body = await this.readJson(res);
    // Consume the refreshed token before anything else looks at the body, and
    // remove it: the rest of the app has no business holding a credential, and
    // this body is rendered in places a user can read and copy.
    const refreshed = this.takeToken(body);
    if (refreshed) this.token = refreshed;

    if (!res.ok) {
      throw new ApiError(this.errorText(body, "The server rejected the request."), res.status);
    }
    return body as T;
  }

  private takeToken(body: unknown): string | null {
    if (!body || typeof body !== "object") return null;
    const record = body as Record<string, unknown>;
    const token = record["token"];
    if (typeof token !== "string" || token === "") return null;
    delete record["token"];
    return token;
  }

  private async readJson(res: Response): Promise<unknown> {
    const text = await res.text();
    if (text === "") return {};
    try {
      return JSON.parse(text);
    } catch {
      // The server answers errors as json, but a proxy or a panic can produce
      // html or a bare string; surface it rather than a parse error.
      return { error: text };
    }
  }

  private errorText(body: unknown, fallback: string): string {
    if (body && typeof body === "object") {
      const record = body as Record<string, unknown>;
      for (const key of ["error", "message"]) {
        const v = record[key];
        if (typeof v === "string" && v !== "") return v;
      }
    }
    return fallback;
  }
}
