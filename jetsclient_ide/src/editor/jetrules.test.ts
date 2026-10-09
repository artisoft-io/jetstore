import { describe, expect, it } from "vitest";
import { StringStream } from "@codemirror/language";
import { jetRulesParser, type JetRulesState } from "./jetrules";

/**
 * Drives the stream parser the way CodeMirror does — a line at a time, carrying
 * state across — and returns [text, token] pairs with whitespace dropped.
 */
function tokenize(code: string): Array<[string, string | null]> {
  const state: JetRulesState = jetRulesParser.startState!(2);
  const out: Array<[string, string | null]> = [];
  for (const line of code.split("\n")) {
    const stream = new StringStream(line, 2, 2);
    while (!stream.eol()) {
      const start = stream.pos;
      const token = jetRulesParser.token(stream, state) ?? null;
      const text = line.slice(start, stream.pos);
      if (stream.pos === start) throw new Error(`tokeniser stalled at ${start}: ${line}`);
      if (text.trim() !== "") out.push([text, token]);
    }
  }
  return out;
}

/** The token assigned to the first occurrence of `text`. */
function tokenFor(code: string, text: string): string | null | undefined {
  return tokenize(code).find(([t]) => t === text)?.[1];
}

describe("jetrules stream parser", () => {
  it("advances on every input it is given", () => {
    // The guard in tokenize() throws if the parser ever fails to consume; this is
    // the case that matters because a stalled tokeniser hangs the editor.
    const odd = '~ ` | & % ^ \\ é \t []{}();:.,\n@Unknown $notAProperty ?';
    expect(() => tokenize(odd)).not.toThrow();
  });

  it("highlights comments to end of line", () => {
    const toks = tokenize("# a comment with \"quotes\" and ?vars\nclass Foo");
    expect(toks[0]?.[1]).toBe("comment");
    // The comment must not leak into the next line.
    expect(tokenFor("# c\nclass Foo", "class")).toBe("keyword");
  });

  it("recognises declarators, keywords and types", () => {
    expect(tokenFor('volatile_resource tag = "tag";', "volatile_resource")).toBe("storage");
    expect(tokenFor("resource x", "resource")).toBe("storage");
    expect(tokenFor("lookup_table PlaceOfServiceLookup {", "lookup_table")).toBe("keyword");
    expect(tokenFor('"NAME" as text', "text")).toBe("type");
  });

  it("recognises rule variables and rule headers", () => {
    expect(tokenFor("(?state rdf:type jets:State)", "?state")).toBe("ruleVariable");
    expect(tokenFor("[AM_State01]:", "AM_State01")).toBe("ruleName");
    // `[` used as a list opener must not make the next identifier a rule name.
    expect(tokenFor('$key = ["CODE"]', '"CODE"')).toBe("string");
  });

  it("splits namespaced identifiers into prefix and qualified name", () => {
    const toks = tokenize("(?s rdf:type jets:State)");
    expect(toks.find(([t]) => t === "rdf")?.[1]).toBe("namespace");
    expect(toks.find(([t]) => t === "type")?.[1]).toBe("qualifiedName");
    expect(toks.find(([t]) => t === "jets")?.[1]).toBe("namespace");
    expect(toks.find(([t]) => t === "State")?.[1]).toBe("qualifiedName");
  });

  it("does not treat the colon after a rule header as a namespace", () => {
    const toks = tokenize("[R1]:\n  (?a b:c)");
    expect(toks.find(([t]) => t === "R1")?.[1]).toBe("ruleName");
    expect(toks.find(([t]) => t === "b")?.[1]).toBe("namespace");
  });

  it("handles strings, escapes and config properties", () => {
    expect(tokenFor('$csv_file = "lookups/a.csv",', "$csv_file")).toBe("configProperty");
    // Enumerated in the grammar, so an unknown $name stays unhighlighted.
    expect(tokenFor("$not_a_property = 1", "$not_a_property")).toBe(null);
    const escaped = tokenize('"a \\" still string" class');
    expect(escaped[0]?.[1]).toBe("string");
    expect(escaped.find(([t]) => t === "class")?.[1]).toBe("keyword");
  });

  it("does not let an unterminated string swallow the next line", () => {
    expect(tokenFor('"unterminated\nclass Foo', "class")).toBe("keyword");
  });

  it("recognises operators, functions and constants", () => {
    expect(tokenFor("a -> b", "->")).toBe("operator");
    expect(tokenFor("a and not b", "and")).toBe("operator");
    expect(tokenFor("x >= 3", ">=")).toBe("operator");
    expect(tokenFor("(?root p create_entity 0)", "create_entity")).toBe("function");
    expect(tokenFor("x = true", "true")).toBe("constant");
    expect(tokenFor("x = 42.5", "42.5")).toBe("number");
  });

  it("treats `array of` as one keyword", () => {
    expect(tokenFor("array of text", "array of")).toBe("keyword");
  });

  it("tokenises a realistic rule without stalling or misreading structure", () => {
    const src = [
      "# Analysis rules",
      'volatile_resource hasAnalysisRoot = "hasAnalysisRoot";',
      "",
      "[AM_State02]:",
      "  (?state rdf:type jets:State).",
      "  (?state hasAnalysisRoot ?root)",
      "  ->",
      "  (?root am:hasPatientProfile create_entity 0)",
      ";",
    ].join("\n");
    const toks = tokenize(src);
    expect(toks.find(([t]) => t === "AM_State02")?.[1]).toBe("ruleName");
    expect(toks.find(([t]) => t === "->")?.[1]).toBe("operator");
    expect(toks.filter(([, k]) => k === "ruleVariable").length).toBe(4);
  });
});
