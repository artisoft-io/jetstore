import { describe, expect, it } from "vitest";
import { isServerValidatedJson, languageNameFor } from "./language";

describe("languageNameFor", () => {
  it("recognises the extensions the workspace tree serves", () => {
    expect(languageNameFor("jet_rules/main/rules.jr")).toBe("jetrules");
    expect(languageNameFor("pipes_config/qc_medicalclaim.pc.json")).toBe("json");
    expect(languageNameFor("data/lookups/uszips.sql")).toBe("sql");
    expect(languageNameFor("process_config/base__workspace_init_db.sql")).toBe("sql");
    expect(languageNameFor("lookups/drug_info.csv")).toBe("plain");
  });

  it("ignores case", () => {
    expect(languageNameFor("A.JR")).toBe("jetrules");
    expect(languageNameFor("B.JSON")).toBe("json");
  });

  it("falls back to plain for anything unknown", () => {
    expect(languageNameFor("README")).toBe("plain");
    expect(languageNameFor("")).toBe("plain");
  });
});

describe("isServerValidatedJson", () => {
  it("matches what SaveWorkspaceFileContent parses before writing", () => {
    expect(isServerValidatedJson("a.json")).toBe(true);
    expect(isServerValidatedJson("pipes_config/x.pc.json")).toBe(true);
    expect(isServerValidatedJson("A.JSON")).toBe(true);
    expect(isServerValidatedJson("a.jr")).toBe(false);
  });
});
