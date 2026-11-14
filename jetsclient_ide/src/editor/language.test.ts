import { describe, expect, it } from "vitest";
import { ensureSyntaxTree } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { isServerValidatedJson, languageExtension, languageNameFor } from "./language";

/** The names of the syntax nodes of a document opened as fileName. */
function nodeNames(fileName: string, doc: string): string[] {
  const state = EditorState.create({ doc, extensions: languageExtension(fileName) });
  const tree = ensureSyntaxTree(state, doc.length, 5000);
  const names: string[] = [];
  tree?.iterate({ enter: (n) => void names.push(n.name) });
  return names;
}

describe("languageNameFor", () => {
  it("recognises the extensions the workspace tree serves", () => {
    expect(languageNameFor("jet_rules/main/rules.jr")).toBe("jetrules");
    expect(languageNameFor("pipes_config/qc_medicalclaim.pc.json")).toBe("json");
    expect(languageNameFor("data/lookups/uszips.sql")).toBe("sql");
    expect(languageNameFor("process_config/base__workspace_init_db.sql")).toBe("sql");
    expect(languageNameFor("lookups/drug_info.csv")).toBe("plain");
    expect(languageNameFor("docs/customisation/observe.md")).toBe("markdown");
    expect(languageNameFor("docs/index.mdx")).toBe("markdown");
    expect(languageNameFor("pipes_config/README.md")).toBe("markdown");
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

describe("a page's front matter", () => {
  const page = "---\ntitle: The observe operator\ndescription: d\nkind: reference\n---\n\n# Heading\n\nText.\n";

  it("is parsed as YAML, not as a setext heading", () => {
    const names = nodeNames("docs/customisation/observe.md", page);
    expect(names).toContain("Frontmatter");
    expect(names).toContain("Pair");
    expect(names).not.toContain("SetextHeading2");
    // The rest of the page is still Markdown.
    expect(names).toContain("ATXHeading1");
  });

  it("is front matter in an .mdx page too", () => {
    expect(nodeNames("docs/index.mdx", page)).toContain("Frontmatter");
  });

  it("is only the block at the top: a later --- is Markdown", () => {
    const names = nodeNames("docs/a.md", "Text\n---\n");
    expect(names).not.toContain("Frontmatter");
    expect(names).toContain("SetextHeading2");
  });
});
