package prompt

// The schema in the prompt as well as in `format` — the Go half of I-180's
// remedy, ported at AG.2 (2026-09-05).
//
// **Why this exists at all.** Phase 1's I-41 measured four prompts against
// `granite4.1:3b` with no model change between them: the schema in `format`
// alone scored 0 of 9, naming the discriminator tokens scored 0 of 9, the
// schema rendered into the prompt as TypeScript scored 0 of 9 on one hole and
// 1 of 1 on the other, and adding one worked example per variant scored 9 of 9.
// The remedy Phase 1 wrote down — *the schema has to be in the prompt* — was
// implemented in the Python toolchain (`as_typescript`,
// `tools/cpipes_contract/cpipes_contract/authoring.py:466`) and never reached
// Go. I-180 is that gap, and Phase 3's P.1 tried the cheap half of it — the
// JSON Schema pasted into the instruction — and the figure moved by nothing.
//
// **TypeScript rather than JSON Schema, and the reason is size rather than
// taste.** I-41 measured `ColumnMapping` at 5,252 tokens as JSON Schema and
// 1,086 as TypeScript, `PartitionWriterPipe` at 11,747 against 2,173 — 18 to
// 20%. A discriminated union is one line per variant instead of a `oneOf` and a
// mapping. What goes in `format` costs 2 to 3 prompt tokens whatever its size
// (Q-15, F113); what goes in the prompt costs its own rendering, so the
// difference between the two encodings is the difference between an arm that
// fits and one that does not.
//
// **This is a port and not a redesign**, so it renders what the Python renders
// and diverges in exactly one respect, which is deliberate: property and
// definition names are emitted in sorted order. Python preserves the JSON
// document's key order; a Go map has none, and F118's lesson — a harness whose
// output reshuffles between runs cannot be compared with itself — says to sort
// rather than to reach for an ordered decoder.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// maxTypeScriptDoc is how much of a `description` is carried into a rendered
// comment. The Python renderer's figure, kept so the two produce the same
// lines.
const maxTypeScriptDoc = 110

// AsTypeScript renders a self-contained sub-schema — the output of Subschema —
// as TypeScript type declarations, one per `$defs` entry, followed by a line
// naming the type the answer must have.
//
// It is the prompt-side encoding of a schema. The wire-side encoding is the
// JSON Schema itself, in `format` or in `guided_json`, and the two must be the
// same document or the guarantee is not what it appears: this takes the same
// bytes the caller is about to send.
func AsTypeScript(document []byte, root string) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal(document, &parsed); err != nil {
		return "", fmt.Errorf("prompt: the schema document is not valid JSON: %w", err)
	}
	defs, _ := parsed["$defs"].(map[string]any)
	if len(defs) == 0 {
		return "", fmt.Errorf("prompt: the schema document has no $defs to render")
	}
	names := make([]string, 0, len(defs))
	for name := range defs {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		node, _ := defs[name].(map[string]any)
		if doc := firstLine(node["description"]); doc != "" {
			fmt.Fprintf(&b, "// %s\n", doc)
		}
		fmt.Fprintf(&b, "type %s = %s;\n", name, typeOf(node))
	}
	fmt.Fprintf(&b, "\n// Produce one value of type %s.\n", root)
	return b.String(), nil
}

// typeOf renders one schema node as a TypeScript type expression.
func typeOf(node map[string]any) string {
	if node == nil {
		return "unknown"
	}
	if ref, ok := node["$ref"].(string); ok {
		return ref[strings.LastIndex(ref, "/")+1:]
	}
	if branches := branchesOf(node); branches != nil {
		var parts []string
		seen := map[string]bool{}
		nullable := false
		for _, raw := range branches {
			b, _ := raw.(map[string]any)
			if b != nil && b["type"] == "null" {
				nullable = true
				continue
			}
			p := typeOf(b)
			if !seen[p] {
				seen[p] = true
				parts = append(parts, p)
			}
		}
		out := strings.Join(parts, " | ")
		if out == "" {
			out = "unknown"
		}
		if nullable {
			out += " | null"
		}
		return out
	}
	if enum, ok := node["enum"].([]any); ok {
		parts := make([]string, 0, len(enum))
		for _, v := range enum {
			parts = append(parts, literal(v))
		}
		return strings.Join(parts, " | ")
	}
	if v, ok := node["const"]; ok {
		return literal(v)
	}
	switch kind := node["type"].(type) {
	case string:
		return typeOfKind(kind, node)
	case []any:
		// A type union written as a list rather than as a `oneOf`. The contract
		// does not use this today; rendering it as `unknown` would be a silent
		// hole in a prompt, so it is spelled out.
		var parts []string
		nullable := false
		for _, k := range kind {
			s, _ := k.(string)
			if s == "null" {
				nullable = true
				continue
			}
			parts = append(parts, typeOfKind(s, node))
		}
		out := strings.Join(parts, " | ")
		if out == "" {
			out = "unknown"
		}
		if nullable {
			out += " | null"
		}
		return out
	}
	return "unknown"
}

func typeOfKind(kind string, node map[string]any) string {
	switch kind {
	case "array":
		items, _ := node["items"].(map[string]any)
		return typeOf(items) + "[]"
	case "object":
		props, _ := node["properties"].(map[string]any)
		if len(props) == 0 {
			return "Record<string, unknown>"
		}
		required := map[string]bool{}
		if list, ok := node["required"].([]any); ok {
			for _, r := range list {
				if s, ok := r.(string); ok {
					required[s] = true
				}
			}
		}
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			child, _ := props[k].(map[string]any)
			opt := "?"
			if required[k] {
				opt = ""
			}
			parts = append(parts, fmt.Sprintf("%s%s: %s", k, opt, typeOf(child)))
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	case "string":
		return "string"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	}
	return "unknown"
}

// branchesOf returns a node's `oneOf` or `anyOf` members, or nil when it has
// neither.
func branchesOf(node map[string]any) []any {
	if b, ok := node["oneOf"].([]any); ok {
		return b
	}
	if b, ok := node["anyOf"].([]any); ok {
		return b
	}
	return nil
}

// literal renders an enum or const value the way TypeScript spells it, which is
// the way JSON does.
func literal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "unknown"
	}
	return string(b)
}

func firstLine(v any) string {
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > maxTypeScriptDoc {
		s = s[:maxTypeScriptDoc]
	}
	return s
}
