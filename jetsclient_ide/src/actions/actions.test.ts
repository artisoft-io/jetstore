/**
 * Tests for the action grammar and interpreter (task S.2a).
 *
 * The assertion that matters is the last one: **the payload the interpreter
 * produces for `lfLoadFilesUF` is byte-for-byte the payload the Dart produces**,
 * checked against the same audit-log capture A.4a used to close I-4. Everything
 * else here is scaffolding for that claim.
 */

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FormState } from "../datatable/formState";
import loadFilesDoc from "./flows/loadFilesUF.ua.json";
import registerFileKeyDoc from "./flows/registerFileKeyUF.ua.json";
import { describeUnresolved, emptyRegistry, resolveEscapes, type EscapeRegistry } from "./escapes";
import { evaluate, runAction, type ActionHost, type PostRequest } from "./interpret";
import { ActionDocumentSchema, emitJsonSchema, type ActionDocument } from "./schema";

const artifactPath = fileURLToPath(new URL("./action.schema.json", import.meta.url));
const field = { group: 0, key: "t" };

function makeHost(overrides: Partial<ActionHost> = {}) {
  const posts: PostRequest[] = [];
  const notices: [string, string][] = [];
  const events: string[] = [];
  const host: ActionHost = {
    validate: () => true,
    confirm: async () => true,
    post: async (request) => {
      posts.push(request);
      events.push("post");
      return { statusCode: 200 };
    },
    query: async (name) => {
      events.push(`query:${name}`);
      return { process_config_key: "pck-1", entity_rdf_type: "hc:Claim" };
    },
    read: async () => {
      events.push("read");
      return [];
    },
    download: (fileName) => events.push(`download:${fileName}`),
    notify: (level, message) => notices.push([level, message]),
    setBusy: (busy) => events.push(busy ? "busy" : "idle"),
    goToState: (state) => events.push(`goToState:${state}`),
    close: () => events.push("close"),
    userEmail: () => "michel@artisoft.io",
    now: () => 1_700_000_000_000,
    ...overrides,
  };
  return { host, posts, notices, events };
}

const run = (doc: ActionDocument, name: string, formState: FormState, host: ActionHost, registry: EscapeRegistry = emptyRegistry) =>
  runAction({ action: doc.actions[name]!, host, formState, field, registry, flowKey: "x" });

describe("the emitted JSON Schema", () => {
  it("matches the committed artifact", () => {
    const emitted = `${JSON.stringify(emitJsonSchema(), null, 2)}\n`;
    if (process.env.UPDATE_SCHEMA === "1") writeFileSync(artifactPath, emitted);
    expect(readFileSync(artifactPath, "utf8")).toBe(emitted);
  });

  it("closes every object except the five keyed maps", () => {
    const open: string[] = [];
    const walk = (node: unknown, path: string): void => {
      if (node === null || typeof node !== "object") return;
      const obj = node as Record<string, unknown>;
      if (obj["type"] === "object" && obj["additionalProperties"] !== false) open.push(path);
      for (const [key, value] of Object.entries(obj)) walk(value, `${path}.${key}`);
    };
    walk(emitJsonSchema(), "$");
    // Five maps whose *keys* are the data: action names, request column names
    // (twice — `fields` and `fanOut.fields`), the request's top-level extras,
    // and a query's column-to-state mapping. Each constrains its keys with
    // `propertyNames` instead. A sixth appearing later is a hole, not a design.
    expect(open.sort()).toEqual([
      "$.$defs.Rows.anyOf.1.properties.fields",
      "$.$defs.Rows.anyOf.4.properties.fields",
      "$.$defs.Step.anyOf.8.properties.extras",
      "$.$defs.Step.anyOf.9.properties.into",
      "$.properties.actions",
    ]);
  });
});

describe("the two proof flows' documents", () => {
  it.each([
    ["registerFileKeyUF", registerFileKeyDoc, 2],
    ["loadFilesUF", loadFilesDoc, 3],
  ])("%s validates and has %i actions", (_name, doc, count) => {
    const result = ActionDocumentSchema.safeParse(doc);
    expect(result.success ? [] : result.error.issues).toEqual([]);
    expect(Object.keys((doc as ActionDocument).actions)).toHaveLength(count);
  });

  it("covers the five arms the two flows define in Dart", () => {
    // `register_file_key` has 2 `case ActionKeys.` labels and `load_files` 3.
    const names = [
      ...Object.keys(registerFileKeyDoc.actions),
      ...Object.keys(loadFilesDoc.actions),
    ];
    expect(names.sort()).toEqual([
      "dialogCancel",
      "lfDropTable",
      "lfLoadFilesUF",
      "lfSyncFileKey",
      "rfkSubmitSchemaEventUF",
    ]);
  });
});

describe("values", () => {
  let formState: FormState;
  beforeEach(() => {
    formState = new FormState();
    formState.setValue(0, "scalar", "one");
    formState.setValue(0, "list", ["a", "b"]);
    formState.setValue(0, "pg", "{x,y}");
  });

  const evalWith = (value: never, index?: number) =>
    evaluate(value, { formState, field, host: makeHost().host, ...(index !== undefined ? { index } : {}) });

  it("unpacks a scalar and the first of a list", () => {
    expect(evalWith({ fromKey: "scalar" } as never)).toBe("one");
    expect(evalWith({ fromKey: "list" } as never)).toBe("a");
    expect(evalWith({ fromKey: "missing" } as never)).toBeNull();
  });

  it("decodes a Postgres array literal, and encodes one back", () => {
    expect(evalWith({ fromKeyList: "pg" } as never)).toEqual(["x", "y"]);
    expect(evalWith({ pgArrayFromKey: "list" } as never)).toBe("{a,b}");
    // `'{}'` is the empty list, not a one-element list containing "{}".
    formState.setValue(0, "empty", "{}");
    expect(evalWith({ fromKeyList: "empty" } as never)).toEqual([]);
  });

  it("interpolates a template, which is how four keys become one string", () => {
    expect(evalWith({ template: "{scalar}-{list}" } as never)).toBe("one-a");
  });

  it("refuses fromKeyAtIndex outside a fanOut", () => {
    // The first of the three rules the schema cannot state. Reading element zero
    // instead would be a plausible-looking wrong answer.
    expect(() => evalWith({ fromKeyAtIndex: "list" } as never)).toThrow(/outside a fanOut/);
    expect(evalWith({ fromKeyAtIndex: "list" } as never, 1)).toBe("b");
  });
});

describe("control flow", () => {
  it("stops without failing when the form is invalid", () => {
    // The Dart returns null from an invalid form: the user changed their mind,
    // and an error banner would be wrong.
    const { host, posts } = makeHost({ validate: () => false });
    return expect(
      run(registerFileKeyDoc as ActionDocument, "rfkSubmitSchemaEventUF", new FormState(), host),
    ).resolves.toBeNull().then(() => expect(posts).toHaveLength(0));
  });

  it("stops without failing when a confirmation is declined", async () => {
    const doc = {
      schemaVersion: 1,
      actions: { a: { description: "d", steps: [{ do: "confirm", message: "sure?" }, { do: "close" }] } },
    } as unknown as ActionDocument;
    const { host, events } = makeHost({ confirm: async () => false });
    expect(await run(doc, "a", new FormState(), host)).toBeNull();
    expect(events).not.toContain("close");
  });

  it("reports the server's own message on failure, and stops", async () => {
    const { host, posts, notices } = makeHost({
      post: async (r) => {
        posts.push(r);
        return { statusCode: 500, error: "database is on fire" };
      },
    });
    const outcome = await run(loadFilesDoc as ActionDocument, "lfSyncFileKey", new FormState(), host);
    expect(outcome).toBe("database is on fire");
    expect(notices).toEqual([["error", "database is on fire"]]);
  });

  it("lowers the spinner even when the post throws", async () => {
    const { host, events } = makeHost({
      post: async () => {
        throw new Error("network");
      },
    });
    await expect(run(loadFilesDoc as ActionDocument, "lfSyncFileKey", new FormState(), host)).rejects.toThrow();
    expect(events).toEqual(["busy", "idle"]);
  });

  it("asks tables to re-read after a successful write, and not after a failed one", async () => {
    // `invokeCallbacks()` — the second notification channel. Without it a write
    // leaves every table on screen showing pre-write rows.
    const formState = new FormState();
    const refreshed = vi.fn();
    formState.onRefreshRequested(refreshed);
    await run(loadFilesDoc as ActionDocument, "lfSyncFileKey", formState, makeHost().host);
    expect(refreshed).toHaveBeenCalledTimes(1);

    const { host } = makeHost({ post: async () => ({ statusCode: 500 }) });
    await run(loadFilesDoc as ActionDocument, "lfSyncFileKey", formState, host);
    expect(refreshed).toHaveBeenCalledTimes(1);
  });
});

describe("the escape registry", () => {
  it("names every unresolved escape at once rather than the first", () => {
    const unresolved = resolveEscapes(
      [
        { kind: "actions", name: "gone", at: "x.a" },
        { kind: "actions", name: "alsoGone", at: "x.b" },
        { kind: "initializers", name: "seedFromHomeFilters", at: "x" },
      ],
      { ...emptyRegistry, initializers: { seedFromHomeFilters: () => {} } },
    );
    expect(unresolved.map((u) => u.name)).toEqual(["gone", "alsoGone"]);
    expect(describeUnresolved(unresolved)).toContain('x.b: no actions escape named "alsoGone"');
    expect(describeUnresolved([])).toBeNull();
  });

  it("throws rather than skipping when an unresolved escape is run", async () => {
    // The third rule the schema cannot state. A no-op here would let a flow lose
    // a step and still look like it worked.
    const doc = {
      schemaVersion: 1,
      actions: { a: { description: "d", steps: [{ do: "escape", name: "missing" }] } },
    } as unknown as ActionDocument;
    await expect(run(doc, "a", new FormState(), makeHost().host)).rejects.toThrow(
      /resolveEscapes was not run/,
    );
  });

  it("runs a resolved escape and takes its failure as the outcome", async () => {
    const doc = {
      schemaVersion: 1,
      actions: {
        a: { description: "d", steps: [{ do: "escape", name: "there" }, { do: "close" }] },
      },
    } as unknown as ActionDocument;
    const { host, events } = makeHost();
    const registry: EscapeRegistry = { ...emptyRegistry, actions: { there: async () => "no" } };
    expect(await run(doc, "a", new FormState(), host, registry)).toBe("no");
    expect(events).not.toContain("close");
  });
});

describe("lfLoadFilesUF against the Dart's own payload", () => {
  /**
   * The capture is `../datatable/fixtures/load_files_flutter_audit.log` — the
   * apiserver's stdout while the Flutter app was driven through `load_files`,
   * which is what closed I-4 for the read side. It contains the write too, and
   * that is what this checks.
   */
  const captured = (() => {
    const log = readFileSync(
      fileURLToPath(new URL("../datatable/fixtures/load_files_flutter_audit.log", import.meta.url)),
      "utf8",
    );
    for (const line of log.split("\n")) {
      if (!line.startsWith("{")) continue;
      const entry = JSON.parse(line) as { message?: string };
      if (typeof entry.message !== "string") continue;
      const body = JSON.parse(entry.message) as Record<string, unknown>;
      if (body["action"] === "insert_rows") return body;
    }
    throw new Error("no insert_rows payload in the capture");
  })();

  /** The form state the two tables had published when the capture was taken. */
  function capturedFormState(): FormState {
    const row = (captured["data"] as Record<string, string>[])[0]!;
    const formState = new FormState();
    // Both tables publish through `otherColumns`, so every value is an array
    // even when the table is single-select — which is why the Dart writes
    // `state[FSK.client][0]`.
    for (const key of ["client", "org", "object_type", "table_name"]) {
      formState.setValue(0, key, [row[key]!]);
    }
    for (const key of ["file_key", "input_registry.session_id", "source_period_key"]) {
      formState.setValue(0, key, [row[key]!]);
    }
    return formState;
  }

  it("produces the captured request, field for field", async () => {
    const row = (captured["data"] as Record<string, string>[])[0]!;
    const { host, posts } = makeHost({ now: () => Number(row["session_id"]) });
    const outcome = await run(loadFilesDoc as ActionDocument, "lfLoadFilesUF", capturedFormState(), host);

    expect(outcome).toBeNull();
    expect(posts).toHaveLength(1);
    expect(posts[0]!.endpoint).toBe("/dataTable");
    expect(posts[0]!.body).toEqual(captured);
  });

  it("serialises to the same bytes, which is what makes captures diffable", async () => {
    // Not a correctness requirement — the server unmarshals into a map — but it
    // is achievable for free because the document's field order is authored, and
    // it turns "compare two payloads" into `diff`.
    const row = (captured["data"] as Record<string, string>[])[0]!;
    const { host, posts } = makeHost({ now: () => Number(row["session_id"]) });
    await run(loadFilesDoc as ActionDocument, "lfLoadFilesUF", capturedFormState(), host);
    expect(JSON.stringify(posts[0]!.body)).toBe(JSON.stringify(captured));
  });

  it("sends one row per selected file key, with a distinct session id each", async () => {
    // The capture has one file selected. The fan-out is the point of the arm, so
    // the multi-row case is asserted separately rather than left to the capture.
    const formState = capturedFormState();
    formState.setValue(0, "file_key", ["a.csv", "b.csv", "c.csv"]);
    formState.setValue(0, "input_registry.session_id", ["1", "2", "3"]);
    formState.setValue(0, "source_period_key", ["10", "20", "30"]);

    const { host, posts } = makeHost({ now: () => 1_000 });
    await run(loadFilesDoc as ActionDocument, "lfLoadFilesUF", formState, host);

    const rows = (posts[0]!.body as { data: Record<string, string>[] }).data;
    expect(rows.map((r) => r["file_key"])).toEqual(["a.csv", "b.csv", "c.csv"]);
    expect(rows.map((r) => r["source_period_key"])).toEqual(["10", "20", "30"]);
    // `"${DateTime.now().millisecondsSinceEpoch + i}"` — the Dart adds the index
    // so that three rows inserted in the same millisecond are three sessions.
    expect(rows.map((r) => r["session_id"])).toEqual(["1000", "1001", "1002"]);
    // The single-select columns repeat rather than being indexed.
    expect(new Set(rows.map((r) => r["client"]))).toEqual(new Set(["CI"]));
  });

  it("refuses to fan out over a key that is not a list", async () => {
    // The second rule the schema cannot state. A scalar here would send one row
    // built from a string's characters.
    const formState = capturedFormState();
    formState.setValue(0, "file_key", "just-one.csv");
    await expect(
      run(loadFilesDoc as ActionDocument, "lfLoadFilesUF", formState, makeHost().host),
    ).rejects.toThrow(/does not hold a list/);
  });
});
