/**
 * A CodeMirror 6 mode for the JetRules DSL.
 *
 * Ported from `tools/vscode-jetrule/syntaxes/jetrule.tmLanguage.json`, which stays
 * the reference for what the language looks like. That grammar is small — fourteen
 * top-level patterns — which is why porting was cheaper than dragging in the
 * TextMate runtime (vscode-textmate plus an Oniguruma wasm build) purely to reuse
 * it. Keep the two in step: a construct highlighted in the VS Code extension and
 * not here is a bug in this file.
 *
 * A `StreamLanguage` rather than a Lezer grammar, deliberately. Highlighting is all
 * this needs today, and a stream tokeniser is a few hundred lines against a real
 * parser's several thousand. If the IDE later wants folding by rule, structural
 * selection, or go-to-definition, that is the point to invest in Lezer — those want
 * a tree, and this cannot give them one.
 */

import { StreamLanguage, type StreamParser, type StringStream } from "@codemirror/language";
import { LanguageSupport } from "@codemirror/language";
import { tags as t } from "@lezer/highlight";

/** `keyword.control.jetrule` */
const STATEMENT_KEYWORDS = new Set([
  "class",
  "lookup_table",
  "triple",
  "rule_sequence",
  "main",
  "jetstore_config",
]);

/** `storage.modifier.jetrule` */
const STORAGE_DECLARATORS = new Set(["resource", "volatile_resource"]);

/** `storage.type.jetrule` */
const TYPES = new Set([
  "int",
  "uint",
  "long",
  "ulong",
  "double",
  "text",
  "date",
  "datetime",
  "bool",
]);

/** `constant.language.jetrule` */
const CONSTANTS = new Set(["true", "false", "null"]);

/** `support.function.jetrule` */
const FUNCTIONS = new Set(["create_uuid_resource", "create_entity", "toText"]);

/** `keyword.operator.logical.jetrule` */
const LOGICAL_OPERATORS = new Set(["and", "or", "not"]);

/**
 * `support.type.property-name.jetrule` — the `$`-prefixed configuration keys. The
 * grammar enumerates them rather than matching `$\w+`, so an unknown `$foo` stays
 * unhighlighted and reads as the typo it probably is. That is worth preserving.
 */
const CONFIG_PROPERTIES = new Set([
  "base_classes",
  "data_properties",
  "object_properties",
  "grouping_properties",
  "as_table",
  "max_looping",
  "max_rule_exec",
  "input_types",
  "main_rule_sets",
  "table_name",
  "csv_file",
  "key",
  "columns",
]);

const IDENT = /^[A-Za-z_][A-Za-z0-9_]*/;

export interface JetRulesState {
  /** Inside a double-quoted string that has not closed yet. */
  inString: boolean;
  /** Saw `[`; the next identifier names a rule. */
  expectRuleName: boolean;
  /** Previous token was a namespace prefix, so the name after `:` is a type. */
  afterNamespace: boolean;
}

export const jetRulesParser: StreamParser<JetRulesState> = {
  name: "jetrule",

  startState(): JetRulesState {
    return { inString: false, expectRuleName: false, afterNamespace: false };
  },

  token(stream: StringStream, state: JetRulesState): string | null {
    if (state.inString) return consumeString(stream, state);

    if (stream.eatSpace()) return null;

    // Comments run to end of line.
    if (stream.eat("#")) {
      stream.skipToEnd();
      return "comment";
    }

    if (stream.eat('"')) {
      state.inString = true;
      state.afterNamespace = false;
      return consumeString(stream, state);
    }

    if (stream.match("@JetCompilerDirective")) {
      state.afterNamespace = false;
      return "directive";
    }

    // Rule variables: ?claim, ?state
    if (stream.eat("?")) {
      stream.match(IDENT);
      state.afterNamespace = false;
      return "ruleVariable";
    }

    // Configuration properties: $csv_file, $key
    if (stream.eat("$")) {
      const name = stream.match(IDENT);
      state.afterNamespace = false;
      return Array.isArray(name) && CONFIG_PROPERTIES.has(name[0]) ? "configProperty" : null;
    }

    // A rule header opens with `[`; the identifier after it names the rule.
    if (stream.eat("[")) {
      state.expectRuleName = true;
      state.afterNamespace = false;
      return "bracket";
    }
    if (stream.eat("]")) {
      state.expectRuleName = false;
      state.afterNamespace = false;
      return "bracket";
    }

    // Multi-character operators first, so `->` does not tokenise as `-` then `>`.
    if (stream.match("->") || stream.match("==") || stream.match("!=") ||
        stream.match("<=") || stream.match(">=") || stream.match("r?")) {
      state.afterNamespace = false;
      return "operator";
    }

    const ch = stream.peek();
    if (ch != null && "<>+-*/=".includes(ch)) {
      stream.next();
      state.afterNamespace = false;
      return "operator";
    }

    if (ch === ":") {
      stream.next();
      // Leave `afterNamespace` alone: it was set by the identifier before this
      // colon and is consumed by the identifier after it.
      return "punctuation";
    }

    if (ch != null && "(){},;.".includes(ch)) {
      stream.next();
      state.afterNamespace = false;
      return "punctuation";
    }

    if (stream.match(/^[0-9]+(\.[0-9]+)?/)) {
      state.afterNamespace = false;
      return "number";
    }

    const word = stream.match(IDENT);
    if (Array.isArray(word)) return classifyWord(word[0], stream, state);

    // Nothing matched; consume a character so the tokeniser always advances.
    stream.next();
    state.afterNamespace = false;
    return null;
  },

  languageData: {
    commentTokens: { line: "#" },
    closeBrackets: { brackets: ["(", "[", "{", '"'] },
    indentOnInput: /^\s*[}\])]$/,
  },

  tokenTable: {
    directive: t.meta,
    ruleName: t.labelName,
    ruleVariable: t.variableName,
    configProperty: t.propertyName,
    storage: t.modifier,
    keyword: t.keyword,
    type: t.typeName,
    constant: t.atom,
    function: t.function(t.variableName),
    namespace: t.namespace,
    qualifiedName: t.typeName,
    operator: t.operator,
    punctuation: t.punctuation,
    bracket: t.bracket,
    comment: t.lineComment,
    string: t.string,
    number: t.number,
  },
};

function consumeString(stream: StringStream, state: JetRulesState): string {
  while (!stream.eol()) {
    const c = stream.next();
    if (c === "\\") {
      // The grammar escapes only `\"` and `\\`, but consuming whatever follows a
      // backslash is the safe reading and keeps an escaped quote from closing.
      stream.next();
      continue;
    }
    if (c === '"') {
      state.inString = false;
      return "string";
    }
  }
  // Unterminated at end of line. JetRules strings do not span lines, so drop out
  // rather than swallowing the rest of the file into one string token.
  state.inString = false;
  return "string";
}

function classifyWord(word: string, stream: StringStream, state: JetRulesState): string | null {
  const wasAfterNamespace = state.afterNamespace;
  state.afterNamespace = false;

  if (state.expectRuleName) {
    state.expectRuleName = false;
    return "ruleName";
  }

  // `array of` is a single construct in the grammar.
  if (word === "array" && stream.match(/^\s+of\b/)) return "keyword";

  if (STORAGE_DECLARATORS.has(word)) return "storage";
  if (STATEMENT_KEYWORDS.has(word)) return "keyword";
  if (LOGICAL_OPERATORS.has(word)) return "operator";
  if (CONSTANTS.has(word)) return "constant";
  if (FUNCTIONS.has(word)) return "function";
  if (TYPES.has(word)) return "type";

  // `rdf:type`, `jets:State` — the prefix is a namespace, the name after the
  // colon is the type it qualifies. The flag has to be set here and survive the
  // colon token in between, which is why `token` leaves it alone for ":".
  if (stream.match(/^\s*:\s*[A-Za-z_]/, false)) {
    state.afterNamespace = true;
    return "namespace";
  }
  if (wasAfterNamespace) return "qualifiedName";

  return null;
}

export const jetRulesLanguage = StreamLanguage.define(jetRulesParser);

export function jetRules(): LanguageSupport {
  return new LanguageSupport(jetRulesLanguage);
}
