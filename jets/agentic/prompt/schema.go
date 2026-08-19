// Package prompt builds what a run sends: a schema narrow enough to constrain
// generation, and the instruction around it.
//
// **The measurement this package exists because of.** The Phase 1 plan's §3.3
// says a request for one TransformationSpec "constrains generation to
// #/$defs/TransformationSpec rather than to the whole document — which is what
// makes the narrow task narrow at the wire level and not merely in the
// instruction text". Addressing one $defs entry does narrow the *root*. It does
// not, on this contract, narrow the *size*, because every operator reaches the
// same shared substrate of channel specs, column evaluators and expression
// nodes. Measured over the emitted cpipes schema on 2026-08-18:
//
//	$defs entry                    closure    bytes    ~tokens
//	ChannelSpec                      4/144    5,326      1,331
//	TransformationColumnSpec        22/144   38,002      9,500
//	TransformationSpecMapRecord     61/144  121,138     30,284
//	TransformationSpec              80/144  172,722     43,180
//
// The infer server's context is 32,768 tokens (OLLAMA_CONTEXT_LENGTH). So a
// TransformationSpec schema **does not fit at all**, and a single operator's
// spec fills 92% of the window before a word of instruction is written. There
// is no cheap prune either: the 61 definitions of a map_record closure are a
// few kilobytes each, with no dominant member to remove.
//
// Two consequences, and neither is a detail:
//
//   - **A task must declare a schema that fits, and be refused if it does not.**
//     Silently sending an over-budget schema gets it truncated by the server or
//     rejected late, after the tokens are spent. Fits() is the check and Task
//     validation is where it belongs.
//   - **This is direct evidence for decision 9's granularity.** The Phase 0
//     plan's §5.3.7 argued that templates convert "generate a 177KB config" into
//     "generate 453 independently validatable TransformationColumnSpec
//     fragments", and called that "precisely the regime in which a small
//     self-hosted model is credible". The numbers above say that is not merely
//     preferable — at TransformationSpec granularity the schema does not fit in
//     the window, so the fragment is the only unit that works today.
package prompt

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DefaultContextTokens is the infer server's configured context length
// (OLLAMA_CONTEXT_LENGTH, cdk/jetstore_one/stack/build_infer_service.go). It is
// a default rather than a constant because gap 12 may move it, and a caller
// that knows better should say so.
const DefaultContextTokens = 32768

// DefaultReserveTokens is what Fits leaves for the instruction, any few-shot
// examples and the answer itself. A schema that consumes the whole window
// leaves nothing to ask a question with.
const DefaultReserveTokens = 8192

// Subschema returns a self-contained JSON Schema document rooted at one $defs
// entry of doc, carrying that entry's transitive $ref closure and nothing else.
//
// Self-contained matters: Ollama's `format` takes a schema document, not a
// reference into one, so "constrain to #/$defs/X" has to become a document
// whose root is X. The closure is what keeps that document from being the whole
// contract, and — per the package comment — how much it keeps out depends
// entirely on which entry is chosen.
func Subschema(doc []byte, defName string) ([]byte, error) {
	var parsed map[string]any
	if err := json.Unmarshal(doc, &parsed); err != nil {
		return nil, fmt.Errorf("prompt: the schema document is not valid JSON: %w", err)
	}
	defs, ok := parsed["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("prompt: the schema document has no $defs to address")
	}
	if _, ok := defs[defName]; !ok {
		return nil, fmt.Errorf("prompt: no $defs entry named %q (the document has %d; %s)",
			defName, len(defs), nearest(defs, defName))
	}

	keep := map[string]bool{defName: true}
	stack := []string{defName}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, ref := range collectRefs(defs[cur]) {
			if !keep[ref] {
				if _, ok := defs[ref]; !ok {
					// A dangling $ref would produce a document that cannot
					// compile. Better to say so than to emit it.
					return nil, fmt.Errorf("prompt: %q references #/$defs/%s, which the document does not define",
						cur, ref)
				}
				keep[ref] = true
				stack = append(stack, ref)
			}
		}
	}

	kept := make(map[string]any, len(keep))
	for name := range keep {
		kept[name] = defs[name]
	}
	out := map[string]any{
		"$schema": parsed["$schema"],
		"$ref":    "#/$defs/" + defName,
		"$defs":   kept,
	}
	if out["$schema"] == nil {
		delete(out, "$schema")
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("prompt: while encoding the sub-schema: %w", err)
	}
	return encoded, nil
}

// collectRefs walks a decoded schema node and returns the local $defs names it
// references. Only local references are followed: a remote $ref is a different
// problem and this contract has none.
func collectRefs(node any) []string {
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch t := n.(type) {
		case map[string]any:
			for k, v := range t {
				if k == "$ref" {
					if s, ok := v.(string); ok && strings.HasPrefix(s, "#/$defs/") {
						out = append(out, strings.TrimPrefix(s, "#/$defs/"))
						continue
					}
				}
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(node)
	return out
}

// EstimateTokens is a deliberately rough estimate: four bytes to the token.
//
// It is an estimate and the name says so. A real tokeniser would be exact and
// would mean carrying the model's vocabulary into this package to answer a
// question whose useful form is "is this obviously too big". JSON schemas are
// punctuation-dense and tokenise a little worse than four bytes, so this errs
// toward optimism — which is the wrong direction, and the reason Fits reserves
// a large margin rather than trusting the number.
func EstimateTokens(b []byte) int { return len(b) / 4 }

// Fits reports whether a schema leaves room to ask a question and receive an
// answer. contextTokens of zero means DefaultContextTokens; reserve of zero
// means DefaultReserveTokens.
//
// This is checked before a run starts rather than discovered at the server,
// because an over-budget schema is either truncated — silently changing what
// constrains generation — or rejected after the request has been paid for.
func Fits(schema []byte, contextTokens, reserve int) error {
	if contextTokens <= 0 {
		contextTokens = DefaultContextTokens
	}
	if reserve <= 0 {
		reserve = DefaultReserveTokens
	}
	used := EstimateTokens(schema)
	if used+reserve > contextTokens {
		return fmt.Errorf(
			"prompt: the schema is about %d tokens and the context is %d with %d reserved for the "+
				"instruction and the answer; address a narrower $defs entry — a whole TransformationSpec "+
				"does not fit, a TransformationColumnSpec does (see the package comment)",
			used, contextTokens, reserve)
	}
	return nil
}

// nearest offers a few candidate names when one is not found, so a typo does
// not require reading a 144-entry document to correct.
//
// Substring matching is not enough here and was the first attempt: the typos
// that actually happen are single-character — Chanel for Channel — and neither
// string contains the other. So this is edit distance, with a substring pass
// kept because it catches the other common case, naming a fragment of a longer
// type.
func nearest(defs map[string]any, want string) string {
	lower := strings.ToLower(want)
	var close, contains []string
	for name := range defs {
		l := strings.ToLower(name)
		switch {
		case editDistance(l, lower) <= 2:
			close = append(close, name)
		case strings.Contains(l, lower) || strings.Contains(lower, l):
			contains = append(contains, name)
		}
	}
	hits := append(close, contains...)
	if len(hits) == 0 {
		return "no similar name"
	}
	sort.Strings(hits)
	if len(hits) > 5 {
		hits = hits[:5]
	}
	return "did you mean " + strings.Join(hits, ", ") + "?"
}

// editDistance is Levenshtein over two rows. Small and exact beats a
// dependency for a hint in an error message.
func editDistance(a, b string) int {
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
