/**
 * Every button the eleven ported flows draw, against the Dart it was transcribed
 * from. Task **C.18**, from **I-155**.
 *
 * **What was unverified was fidelity, not function.** The flows are ported, they run
 * under test, and nothing here suggested they were broken. What I-155 recorded is
 * narrower and exact: **the one artefact that could have checked button fidelity did
 * not contain buttons.** `allFields` walked the three *field* containers a
 * `FormConfig` may use and `FormConfig.actions` is a fourth, so `form_fields.json`
 * reported **zero** of the flows' 143 action-bar buttons until C.0b (jetstore#2022) —
 * and `form_fields.json` is what track F's form work was sized against. Track F
 * transcribed from the Dart rather than from the corpus, which is why it worked, and
 * which is also why nothing checked it.
 *
 * **This file is the check that was infeasible before that commit and is a file
 * comparison after it.** It makes no causal claim and asserts no history; it reads
 * two generated corpora and eleven authored documents and compares them button by
 * button.
 *
 * ## The result, stated as a measurement
 *
 * **147 of 147 buttons match**, across 51 forms in 11 documents: the same key, the
 * same label, the same style, the same capability and the same enablement. The
 * assertions below are that measurement rather than a summary of it — if a number
 * moves, a person reads this file.
 *
 * Three things the comparison had to get right, and each is encoded rather than
 * assumed:
 *
 * 1. **Two corpora, not one.** I-155 names `form_fields.json`, and one of the eleven
 *    flow documents carries a form that is not in it: `homeFiltersUF` holds
 *    `showFailureDetailsDialog`, which lives in `screen_configs.json` because it is a
 *    non-flow screen's dialog. A check reading only the flows' corpus would have
 *    reported it as a document form with no Dart counterpart and been wrong.
 * 2. **Three action names were deliberately renamed**, and `RENAMED_BY_THE_RUNTIME`
 *    below carries the reason for each. They are not transcription drift.
 * 3. **A button can sit among the fields rather than in the action bar**, on both
 *    sides — `FormActionConfig` among a Dart form's fields, `field: "button"` among a
 *    document's rows. Three of the 147 do. The two containers are compared
 *    separately, because a button in the bar and a button in the layout are placed by
 *    different widgets and swapping one for the other would be a real difference.
 *
 * ## What this does not establish
 *
 * It compares **declarations**. A button that is present, correctly named, correctly
 * styled and correctly gated can still run the wrong action — that is failures 2
 * through 5 of `src/actions/README.md`, and none of them is visible from here.
 * `checkActions` (`userflow/documentSet.ts`) already refuses a button naming an
 * action the document does not define, so the two checks are complementary and
 * neither subsumes the other.
 *
 * It also dies with the Flutter app, for `corpusFixtures.test.ts`'s reason: track X
 * deletes `jetsclient/`, and these fixtures stop being a mirror of a running app.
 */

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const here = fileURLToPath(new URL("./", import.meta.url));

/** One button as a Dart corpus reports it (`corpus_support.dart`, `formActionsOf`). */
interface DartButton {
  key: string;
  label: string;
  buttonStyle: string;
  capability?: string;
  enableOnlyWhenFormValid?: boolean;
  enableOnlyWhenFormNotValid?: boolean;
}

interface DartField {
  type: string;
  [k: string]: unknown;
}

interface DartForm {
  actions?: DartButton[];
  fields?: DartField[];
}

/** One button as a `.form.json` declares it (`userflow/form.ts`, `FormActionSchema`). */
interface DocButton {
  action: string;
  label: string;
  style?: string;
  capability?: string;
  enableOnlyWhenFormValid?: boolean;
  enableOnlyWhenFormNotValid?: boolean;
}

interface DocField {
  field: string;
  [k: string]: unknown;
}

interface DocForm {
  rows?: DocField[][];
  actions?: DocButton[];
}

function readJson<T>(relative: string): T {
  return JSON.parse(readFileSync(here + relative, "utf8")) as T;
}

/**
 * The Dart side: both corpora, merged by form key.
 *
 * The flows' corpus wins a collision, and there are none — the 50 keys and the 28 are
 * disjoint, which the test below asserts rather than trusts.
 */
function dartForms(): Record<string, DartForm> {
  const flows = readJson<{ forms: Record<string, DartForm> }>(
    "datatable/fixtures/form_fields.json",
  ).forms;
  const screens = readJson<{ forms: Record<string, DartForm> }>(
    "screens/fixtures/screen_configs.json",
  ).forms;
  return { ...screens, ...flows };
}

const FLOW_DOCUMENTS = [
  "clientRegistryUF",
  "fileMappingUF",
  "homeFiltersUF",
  "loadConfigUF",
  "loadFilesUF",
  "mapFileUF",
  "pipelineConfigUF",
  "registerFileKeyUF",
  "sourceConfigUF",
  "startPipelineUF",
  "workspacePullUF",
] as const;

function documentForms(): Array<{ document: string; form: string; config: DocForm }> {
  const out: Array<{ document: string; form: string; config: DocForm }> = [];
  for (const name of FLOW_DOCUMENTS) {
    const doc = readJson<{ forms: Record<string, DocForm> }>(`userflow/forms/${name}.form.json`);
    for (const [form, config] of Object.entries(doc.forms)) {
      out.push({ document: name, form, config });
    }
  }
  return out;
}

/**
 * The three renames, each with why it is not drift.
 *
 * **The schema did not force any of them.** `Identifier` (`userflow/schema.ts`) is
 * `/^[A-Za-z0-9_.-]+$/` and accepts a dot, so `dialog.cancelAction` would have parsed.
 * That is worth saying plainly, because "the grammar made me" would be the easy
 * reading and it is false.
 *
 *  - `dialog.cancelAction` → `dialogCancel`: **the runtime owns this name.**
 *    `DIALOG_CANCEL` (`userflow/FormDialog.tsx`) is a constant of the dialog host, not
 *    a transcription of anything — a document has to spell it the host's way for the
 *    dialog to close.
 *  - `mapper.ok` → `mapperOk`, `mapper.draft` → `mapperDraft`: a naming choice, made
 *    at F.1 when these three moved into `mapFileUF.ua.json`
 *    (`actions/flowActions.test.ts`). Both are defined there and both are referenced
 *    from the form document, and `checkActions` refuses a button naming an action the
 *    document does not define — so the pair cannot drift apart even though the rename
 *    itself is recorded nowhere but here.
 */
const RENAMED_BY_THE_RUNTIME: Record<string, string> = {
  "dialog.cancelAction": "dialogCancel",
  "mapper.ok": "mapperOk",
  "mapper.draft": "mapperDraft",
};

/**
 * Dart button styles against document styles.
 *
 * **This map is a reading, and the test below checks it is the only consistent one
 * rather than asking to be believed.** The Dart has five styles across these forms
 * and the document has three; nothing in either repository states the
 * correspondence, so asserting `style` against a hand-written table would be
 * asserting my own transcription. What the test does instead is derive the relation
 * from the data and require it to be a *function* — no Dart style mapping to two
 * document styles — and then require it to equal this table. A disagreement is then
 * either a real difference or a rule this table has got wrong, and both are worth a
 * person.
 */
const STYLE: Record<string, string> = {
  ufPrimary: "primary",
  ufSecondary: "secondary",
  dialogOk: "primary",
  dialogCancel: "secondary",
  predominentInForm: "primary",
};

function dartBar(f: DartForm): DartButton[] {
  return f.actions ?? [];
}

function dartInField(f: DartForm): DartButton[] {
  return (f.fields ?? [])
    .filter((x) => x.type === "FormActionConfig")
    .map((x) => x as unknown as DartButton);
}

function docBar(f: DocForm): DocButton[] {
  return f.actions ?? [];
}

function docInField(f: DocForm): DocButton[] {
  return (f.rows ?? []).flat().filter((x) => x.field === "button") as unknown as DocButton[];
}

function renamed(key: string): string {
  return RENAMED_BY_THE_RUNTIME[key] ?? key;
}

describe("the eleven flows' buttons against the Dart", () => {
  it("has a Dart form for every form the eleven documents declare", () => {
    const dart = dartForms();
    const missing = documentForms()
      .filter(({ form }) => !(form in dart))
      .map(({ document, form }) => `${document}/${form}`);
    expect(missing).toEqual([]);
  });

  it("draws the two corpora from disjoint form keys, so merging them loses nothing", () => {
    const flows = Object.keys(
      readJson<{ forms: Record<string, DartForm> }>("datatable/fixtures/form_fields.json").forms,
    );
    const screens = Object.keys(
      readJson<{ forms: Record<string, DartForm> }>("screens/fixtures/screen_configs.json").forms,
    );
    expect(flows.filter((k) => screens.includes(k))).toEqual([]);
  });

  // The measurement. It is an equality rather than a lower bound so that adding a
  // form or a button brings somebody back to this file — which is the whole value of
  // a fidelity check that has already passed.
  it("compares 147 buttons across 51 forms in 11 documents", () => {
    const dart = dartForms();
    let forms = 0;
    let buttons = 0;
    for (const { form, config } of documentForms()) {
      forms += 1;
      buttons += docBar(config).length + docInField(config).length;
      const d = dart[form] ?? {};
      expect(dartBar(d).length + dartInField(d).length).toBe(
        docBar(config).length + docInField(config).length,
      );
    }
    expect(forms).toBe(51);
    expect(buttons).toBe(147);
  });

  it("declares the same buttons, in the same container, in the same order", () => {
    const dart = dartForms();
    const differences: string[] = [];
    for (const { document, form, config } of documentForms()) {
      const d = dart[form] ?? {};
      for (const [where, a, b] of [
        ["action bar", dartBar(d), docBar(config)],
        ["in-field", dartInField(d), docInField(config)],
      ] as Array<[string, DartButton[], DocButton[]]>) {
        const want = a.map((x) => renamed(x.key));
        const got = b.map((x) => x.action);
        if (want.join(",") !== got.join(",")) {
          differences.push(`${document}/${form} ${where}: dart ${want} vs document ${got}`);
        }
      }
    }
    expect(differences).toEqual([]);
  });

  it("gives every button the Dart's label, capability and enablement", () => {
    const dart = dartForms();
    const differences: string[] = [];
    for (const { document, form, config } of documentForms()) {
      const d = dart[form] ?? {};
      for (const [a, b] of [
        ...zip(dartBar(d), docBar(config)),
        ...zip(dartInField(d), docInField(config)),
      ]) {
        const at = `${document}/${form}/${a.key}`;
        if (a.label !== b.label) differences.push(`${at} label: ${a.label} vs ${b.label}`);
        // **The capability is the field to check hardest.** 20 capability claims on a
        // button were invisible to every corpus until C.0b (F88), 7 of them the flows'.
        // A button that lost its capability in transcription is a control offered to
        // someone the Dart hides it from — and the server refuses either way, so the
        // failure is a user pressing a button that cannot work, **not** a privilege
        // escalation. All 7 are present; this line is what keeps that true.
        if ((a.capability ?? null) !== (b.capability ?? null)) {
          differences.push(`${at} capability: ${a.capability} vs ${b.capability}`);
        }
        for (const flag of ["enableOnlyWhenFormValid", "enableOnlyWhenFormNotValid"] as const) {
          if (Boolean(a[flag]) !== Boolean(b[flag])) {
            differences.push(`${at} ${flag}: ${Boolean(a[flag])} vs ${Boolean(b[flag])}`);
          }
        }
      }
    }
    expect(differences).toEqual([]);
  });

  // Two assertions rather than one, because the second is what makes the first mean
  // anything: the observed relation is a function, *and* it is this table.
  it("maps each Dart button style onto exactly one document style, as STYLE says", () => {
    const dart = dartForms();
    const observed = new Map<string, Set<string>>();
    for (const { form, config } of documentForms()) {
      const d = dart[form] ?? {};
      for (const [a, b] of [
        ...zip(dartBar(d), docBar(config)),
        ...zip(dartInField(d), docInField(config)),
      ]) {
        const seen = observed.get(a.buttonStyle) ?? new Set<string>();
        seen.add(b.style ?? "(none)");
        observed.set(a.buttonStyle, seen);
      }
    }
    const ambiguous = [...observed].filter(([, v]) => v.size > 1);
    expect(ambiguous.map(([k, v]) => `${k} -> ${[...v]}`)).toEqual([]);
    const asTable = Object.fromEntries([...observed].map(([k, v]) => [k, [...v][0]]));
    expect(asTable).toEqual(STYLE);
  });
});

function zip<A, B>(a: A[], b: B[]): Array<[A, B]> {
  return a.slice(0, Math.min(a.length, b.length)).map((x, i) => [x, b[i]!]);
}
