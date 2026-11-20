/**
 * @vitest-environment jsdom
 *
 * **A projected flow, in the application, from the bytes a
 * workspace receives.** Added 2026-08-25.
 *
 * `templateApply.test.ts` beside this file already walked the `qc_metrics` projection with
 * the shipped engine, interpreter and `validateForm` — 24 steps, a config out the other
 * end — and said plainly what it could not show: *"There is no flow runner in this
 * application yet"*. That sentence is false since the flow runner landed, and two things
 * stood between the projection and the runner:
 *
 * * `FlowStore` reads `user_flows/` **in the workspace**, and the projections were
 *   committed under `tools/cpipes_contract/`. They are now the third `AssetGroup`, so
 *   `install_workspace_assets` writes them into every workspace.
 * * `productionRegistry.actions` did not carry `cpipesTemplateApply`, so
 *   `FlowStore.load` refused the set at `resolveEscapes` — before any button existed to
 *   press. It is registered now.
 *
 * ## What this file demonstrates, and what it still cannot
 *
 * It reads the twelve documents **off the asset directory** rather than from a fixture, so
 * what is exercised is what the installer copies. Then two levels:
 *
 * 1. **`FlowStore.load` accepts all three projected sets** against `productionRegistry` —
 *    six checks deep, ending at escape resolution. The same load against `emptyRegistry`
 *    fails, which is what makes the registration load-bearing rather than decorative.
 * 2. **The runner drives one end to end in a DOM**: `/flow/qc_report` renders, every
 *    required field is filled through the widgets, `Save configuration` runs the engine's
 *    `stateAction`, and the escape writes `pipes_config/qc_report.pc.json` through the
 *    real `ApiClient`.
 *
 * **What it is not is a browser, and no wording here should be read as one.** Everything
 * below `ApiClient` is real and `fetch` is a stub, exactly as `FlowRunner.test.tsx`'s is;
 * a live apiserver and a database are what would make this file's *end to end*
 * unqualified, and neither was available to the session that wrote it.
 *
 * ## Why `qc_report` for the DOM walk
 *
 * It projects to **one** state — all eleven of its holes are loops, so it asks a filler
 * for nothing and its configuration is its bindings. That makes it the whole of a
 * flow in one render, which is what a seam test wants; `qc_metrics`' 34-step walk is
 * already covered headlessly next door and would test the engine a second time rather
 * than the screen.
 */

import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { emptyRegistry } from "../actions/escapes";
import { productionRegistry } from "../actions/registry";
import { ApiClient } from "../api/client";
import { queryEscape, WorkspaceApi } from "../api/workspace";
import { FlowRunner } from "../screens/FlowRunner";
import { ApiProvider } from "../shell/capabilities";
import { NotificationsProvider, useNotifications } from "../shell/notifications";
import { PromptsProvider } from "../shell/prompts";
import { FlowStore } from "../userflow/store";
import { valueFieldsOf, type Form } from "../userflow/form";
import { configPath } from "./templateApply";

afterEach(cleanup);

/**
 * The asset directory, read as the installer reads it.
 *
 * `jets/workspace_assets/user_flows/` is where `cpipes-contract templates --project`
 * writes and where `//go:embed` takes them from — so a file this test cannot find is a
 * file no workspace would receive.
 */
function assetDir(): string {
  // `import.meta.url` is an http url under jsdom, so the sibling node-environment
  // trick `templateApply.test.ts` uses is not available here. Walk up from the
  // working directory instead, which finds the repository root from wherever
  // vitest was invoked.
  let at = resolve(process.cwd());
  for (;;) {
    const candidate = resolve(at, "jets/workspace_assets/user_flows");
    if (existsSync(candidate)) return `${candidate}/`;
    const up = dirname(at);
    if (up === at) throw new Error("jets/workspace_assets/user_flows is not above the cwd");
    at = up;
  }
}

const ASSETS = assetDir();

const TEMPLATES = ["map_claim_load_stages", "qc_metrics", "qc_report"] as const;
const SUFFIXES = [".uf.json", ".ua.json", ".form.json", ".apply.json"] as const;

/** The workspace a stub apiserver serves, keyed the way `FlowStore` asks for them. */
const files: Record<string, string> = {};
for (const template of TEMPLATES) {
  for (const suffix of SUFFIXES) {
    files[`user_flows/${template}${suffix}`] = readFileSync(`${ASSETS}${template}${suffix}`, "utf8");
  }
}

interface Posted {
  body: Record<string, unknown>;
}

/**
 * The apiserver, as much of it as a projected flow touches: the active workspace, file
 * reads, and the one save the escape performs.
 *
 * A projected flow has no `dataTable` field and no query, which is why this stub is a
 * third the size of `FlowRunner.test.tsx`'s — the surface a template wizard needs is
 * narrow, and that is a fact about the projection rather than about the test.
 */
function stubServer(saveAnswer: { status: number; body: unknown } = { status: 200, body: {} }) {
  const posts: Posted[] = [];
  const saved: { fileName: string; content: string }[] = [];
  const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
    const path = String(url);
    const body = JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>;
    posts.push({ body });

    if (path === "/login") {
      return new Response(
        JSON.stringify({
          token: "t0",
          name: "Test User",
          user_email: "user@example.com",
          is_admin: false,
          capabilities: ["workspace_ide"],
        }),
        { status: 200 },
      );
    }

    switch (body["action"]) {
      case "get_workspace_uri":
        return new Response(
          JSON.stringify({
            workspace_name: "jets_ws",
            workspace_uri: "git@example",
            workspace_branch: "jets_ai",
            workspace_file_key_label_re: "",
          }),
          { status: 200 },
        );

      case "get_workspace_document": {
        const data = (body["data"] as { file_name: string }[])[0]!;
        const name = decodeURIComponent(data.file_name.replace(/\+/g, " "));
        if (files[name] === undefined) {
          return new Response(JSON.stringify({ error: `no such file: ${name}` }), { status: 404 });
        }
        return new Response(JSON.stringify({ file_content: files[name] }), { status: 200 });
      }

      case "save_workspace_file_content": {
        const data = (body["data"] as { file_name: string; file_content: string }[])[0]!;
        saved.push({ fileName: data.file_name, content: data.file_content });
        return new Response(JSON.stringify(saveAnswer.body), { status: saveAnswer.status });
      }

      default:
        return new Response(
          JSON.stringify({ error: `unexpected action ${String(body["action"])}` }),
          { status: 422 },
        );
    }
  }) as unknown as typeof fetch;

  return { fetchImpl, posts, saved };
}

async function client(saveAnswer?: { status: number; body: unknown }) {
  const stub = stubServer(saveAnswer);
  const api = new ApiClient("", stub.fetchImpl);
  await api.login("user@example.com", "pw");
  return { ...stub, api };
}

describe("a projected document set, where the installer puts it", () => {
  it("loads through FlowStore for all three shipped templates", async () => {
    const { api } = await client();
    const store = new FlowStore(new WorkspaceApi(api), {
      workspaceName: "jets_ws",
      registry: productionRegistry,
    });
    for (const template of TEMPLATES) {
      const loaded = await store.load(template);
      expect(loaded.key).toBe(template);
      // The action document names one action and it is the escape, so a set that
      // loads is a set whose escape resolved.
      expect(Object.keys(loaded.actions.actions)).toEqual(["cpipesTemplateApply"]);
      expect(Object.keys(loaded.forms.forms).length).toBeGreaterThan(0);
      // No projected flow carries a `dataTable` field, so nothing is fetched from
      // `table_configs/`. Asserted rather than assumed: it is the reason the stub
      // above serves twelve files and not more.
      expect(loaded.tables).toEqual({});
    }
  });

  /**
   * **The registration is what makes the difference, and this is the check that says so.**
   *
   * Without `cpipesTemplateApply` in the registry the set fails at `resolveEscapes` —
   * the sixth of `FlowStore.load`'s six checks, and the last one before a flow would
   * render. That is exactly the state this application was in until it was registered, and stating it
   * as a failing load rather than as prose is what stops the entry above being read as
   * "the documents happen to be fine".
   */
  it("does not load in a build whose registry lacks the escape", async () => {
    const { api } = await client();
    const store = new FlowStore(new WorkspaceApi(api), {
      workspaceName: "jets_ws",
      registry: emptyRegistry,
    });
    await expect(store.load("qc_report")).rejects.toThrow(/cpipesTemplateApply/);
  });
});

/**
 * Fills every value field of a form, the way a careful author would. With `keepDefaults`
 * a field that carries a `defaultValue` is left as the form seeded it.
 */
function fillEveryField(form: Form, keepDefaults = false): void {
  for (const field of valueFieldsOf(form)) {
    if (field.field !== "text") continue;
    if (keepDefaults && field.defaultValue !== undefined) continue;
    const json = (field.rules ?? []).some((r) => r.rule === "json");
    // **The value is the field's key**, as `templateApply.test.ts` does and for the
    // same reason: a config whose every value names the step that produced it reads
    // as a map from wizard to configuration, which is what makes it evidence.
    const value = json ? `["${field.key}"]` : field.key;
    fireEvent.change(screen.getByLabelText(field.label), { target: { value } });
  }
}

describe("the runner drives one, in a DOM, against the real client", () => {
  it("renders qc_report, collects its bindings and saves the config", async () => {
    const { api, saved } = await client();

    render(
      <ApiProvider api={api}>
        <NotificationsProvider>
          <PromptsProvider>
            <MemoryRouter initialEntries={["/flow/qc_report"]}>
              <Routes>
                <Route path="/flow/:key" element={<FlowRunner api={api} />} />
                {/* Where a finished flow lands. **`/home` now, and it was
                    `/workspace` until then** — a flow declaring no `exitScreenPath`,
                    which a projected one does not, used to exit to the Workspace IDE
                    wherever it began. It now returns to the `returnTo` its
                    url carries, and to the app's index when it carries none, which
                    is this case. Stubbed so that "the flow completed" is observable
                    rather than a router warning. */}
                <Route path="/home" element={<p>Home</p>} />
              </Routes>
            </MemoryRouter>
          </PromptsProvider>
        </NotificationsProvider>
      </ApiProvider>,
    );

    // The one state's description, from the `.uf.json` the generator wrote.
    await waitFor(() =>
      expect(screen.getByText("The values this template is parameterised by")).toBeTruthy(),
    );

    const form = (JSON.parse(files["user_flows/qc_report.form.json"]!) as {
      forms: Record<string, Form>;
    }).forms["bindings"]!;
    fillEveryField(form);

    // Re-queried after the fill rather than held across it: React replaces the
    // node when form state changes, and a click on a detached element is a click
    // that does nothing and reports nothing.
    await waitFor(() =>
      expect(
        (screen.getByRole("button", { name: "Save configuration" }) as HTMLButtonElement).disabled,
      ).toBe(false),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save configuration" }));

    await waitFor(() => {
      const alerts = screen.queryAllByRole("alert").map((n) => n.textContent);
      expect({ saved: saved.length, alerts }).toEqual({ saved: 1, alerts: [] });
    });
    // Escaped, because that is what `saveFile` takes and what the server
    // unescapes on the way in.
    expect(saved[0]!.fileName).toBe(queryEscape(configPath("qc_report")));

    // **A config, not a form dump.** The escape substitutes into the generator's
    // skeleton, so what comes back is the expanded template with the collected values
    // in it — which is the property the apply plan was built for.
    const config = JSON.parse(saved[0]!.content) as Record<string, unknown>;
    expect(Array.isArray(config["channels"]) || Array.isArray(config["pipes"])).toBe(true);
    expect(saved[0]!.content).toContain("bindings._item.report");

    // And the flow finished: `ufCompleted` exits after the state action, so the
    // runner is gone and the app is where a completed flow leaves a user — the
    // index, since this url carries no `returnTo`.
    await waitFor(() => expect(screen.getByText("Home")).toBeTruthy());
  });

  /**
   * **The notification context keys start at their neutral defaults and are saved as
   * such.** They are the keys a deployment's notification template reads, so the flow
   * offers a default the author can see and change rather than a name fixed in the
   * shipped documents; left alone, the default is what the config carries.
   */
  it("saves the notification keys the form seeded when the author leaves them", async () => {
    const { api, saved } = await client();

    render(
      <ApiProvider api={api}>
        <NotificationsProvider>
          <PromptsProvider>
            <MemoryRouter initialEntries={["/flow/qc_report"]}>
              <Routes>
                <Route path="/flow/:key" element={<FlowRunner api={api} />} />
                <Route path="/home" element={<p>Home</p>} />
              </Routes>
            </MemoryRouter>
          </PromptsProvider>
        </NotificationsProvider>
      </ApiProvider>,
    );

    await waitFor(() =>
      expect(screen.getByText("The values this template is parameterised by")).toBeTruthy(),
    );
    expect(
      (screen.getByLabelText("Notification key for the file type") as HTMLInputElement).value,
    ).toBe("$NOTIFICATION_FILE_TYPE");

    const form = (JSON.parse(files["user_flows/qc_report.form.json"]!) as {
      forms: Record<string, Form>;
    }).forms["bindings"]!;
    fillEveryField(form, true);
    await waitFor(() =>
      expect(
        (screen.getByRole("button", { name: "Save configuration" }) as HTMLButtonElement).disabled,
      ).toBe(false),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save configuration" }));
    await waitFor(() => expect(saved.length).toBe(1));

    const config = JSON.parse(saved[0]!.content) as { context: { key: string }[] };
    expect(config.context.map((c) => c.key)).toEqual([
      "$PROCESSING_TICKET",
      "$REPORT",
      "$NOTIFICATION_FILE_TYPE",
      "$NOTIFICATION_TYPE",
      "layout_name",
    ]);
  });

  /**
   * The failure path, because it is the one an author meets and the one no schema layer
   * can report: a required binding left empty stops the save with the escape's own
   * message, and the collected values survive for the author to fix.
   */
  it("refuses to save an incomplete configuration and says which binding is missing", async () => {
    const { api, saved } = await client();

    render(
      <ApiProvider api={api}>
        <NotificationsProvider>
          <PromptsProvider>
            <MemoryRouter initialEntries={["/flow/qc_report"]}>
              <Routes>
                <Route path="/flow/:key" element={<FlowRunner api={api} />} />
              </Routes>
            </MemoryRouter>
          </PromptsProvider>
        </NotificationsProvider>
      </ApiProvider>,
    );

    await waitFor(() =>
      expect(screen.getByText("The values this template is parameterised by")).toBeTruthy(),
    );

    const save = screen.getByRole("button", { name: "Save configuration" });
    // `enableOnlyWhenFormValid` is what the projection puts on this button, so an empty
    // form cannot reach the escape at all — the guard is the form's rather than the
    // escape's, and that is the layering the generator intended.
    expect((save as HTMLButtonElement).disabled).toBe(true);
    expect(saved.length).toBe(0);
  });
});

/**
 * The shell's two banners, with the status in its tone, as `AppShell` draws them
 * (`banner-ok` or `banner-warn`). Rendered outside the router, as the shell is,
 * so what a flow said survives the flow's exit.
 */
function Banners() {
  const { error, status, statusTone } = useNotifications();
  return (
    <>
      {error != null && <div role="alert">{error}</div>}
      {status != null && error == null && (
        <div role="status" className={`banner banner-${statusTone}`}>
          {status}
        </div>
      )}
    </>
  );
}

/** Mounts `/flow/qc_report` against a server answering the save as given, fills it and saves. */
async function fillAndSave(saveAnswer: { status: number; body: unknown }) {
  const { api, saved } = await client(saveAnswer);
  render(
    <ApiProvider api={api}>
      <NotificationsProvider>
        <PromptsProvider>
          <Banners />
          <MemoryRouter initialEntries={["/flow/qc_report"]}>
            <Routes>
              <Route path="/flow/:key" element={<FlowRunner api={api} />} />
              <Route path="/home" element={<p>Home</p>} />
            </Routes>
          </MemoryRouter>
        </PromptsProvider>
      </NotificationsProvider>
    </ApiProvider>,
  );
  await waitFor(() =>
    expect(screen.getByText("The values this template is parameterised by")).toBeTruthy(),
  );
  const form = (JSON.parse(files["user_flows/qc_report.form.json"]!) as {
    forms: Record<string, Form>;
  }).forms["bindings"]!;
  fillEveryField(form);
  await waitFor(() =>
    expect(
      (screen.getByRole("button", { name: "Save configuration" }) as HTMLButtonElement).disabled,
    ).toBe(false),
  );
  fireEvent.click(screen.getByRole("button", { name: "Save configuration" }));
  await waitFor(() => expect(saved.length).toBe(1));
  return { saved };
}

/**
 * **A configuration the server saves with warnings says so.** The save check
 * answers its validator's warnings in a 200 (a key the pipeline ignores when it
 * runs, for one), and the Workspace IDE shows them; the escape that writes a
 * projected flow's configuration dropped them, so the flow ended as if the check
 * had found nothing.
 */
describe("the runner shows what the save check found", () => {
  it("finishes the flow and shows each warning, in the warning tone", async () => {
    await fillAndSave({
      status: 200,
      body: {
        file_name: queryEscape(configPath("qc_report")),
        warnings: [
          {
            severity: "warning",
            code: "cpipes_config_unknown_field",
            message: 'ignored when the pipeline runs: unknown field "output_tabel"',
            path: "/output_tabel",
          },
          { severity: "warning", code: "x", message: "a finding on a line", line: 7 },
        ],
      },
    });

    // Still a success: the flow completes and leaves, and the warning outlives it.
    await waitFor(() => expect(screen.getByText("Home")).toBeTruthy());
    const banner = await screen.findByRole("status");
    expect(banner.className).toContain("banner-warn");
    expect(banner.textContent).toContain(`Saved ${configPath("qc_report")}, with 2 warnings:`);
    expect(banner.textContent).toContain(
      '/output_tabel: ignored when the pipeline runs: unknown field "output_tabel"',
    );
    expect(banner.textContent).toContain("line 7: a finding on a line");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("says nothing extra when the check found nothing, nor from a server that sends no warnings", async () => {
    for (const body of [{ warnings: [] }, {}]) {
      await fillAndSave({ status: 200, body });
      await waitFor(() => expect(screen.getByText("Home")).toBeTruthy());
      expect(screen.queryByRole("status")).toBeNull();
      expect(screen.queryByRole("alert")).toBeNull();
      cleanup();
    }
  });

  it("keeps a refused save an error, and the flow open", async () => {
    await fillAndSave({
      status: 400,
      body: {
        error: `${configPath("qc_report")} cannot be saved:\n  /cluster_config: cluster_config is required`,
      },
    });
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(`Cannot save ${configPath("qc_report")}`);
    expect(alert.textContent).toContain("cluster_config is required");
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByText("Home")).toBeNull();
  });
});
