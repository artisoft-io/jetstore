package template

import (
	"strings"
)

// Markup is the text of one paragraph: literal words with holes in them
// (§10.2). Four forms, and one of them is not a construct:
//
//	{{ chain }}                                  a substitution
//	{{if pred}} ... {{end}}                      a span emitted when the predicate holds
//	{{with chain}} ... {{end}}                   a span scoped to the node the chain yields
//	{{each chain sep: 'a' last: 'b'}} ... {{end}} the span once per node, the results joined
//	{{ chain require 'message' }}                a substitution that asserts
//
// **`if`, `with` and `each` are block forms and not constructs**, on §10.4's
// working definition: a construct is a total function from a value to a value,
// and a block form decides whether and how often a span is emitted. `require`
// is a modifier on the same definition - it changes no value and produces none.
// R-115 is the note that R-107's construct count watches one of the two places
// this notation can grow, and the block forms are the other.
//
// **Whitespace in markup is not whitespace in the output.** Every paragraph is
// wrapped, and `wrap` collapses runs of whitespace through `strings.Fields`, so
// the only decision a boundary carries is whether there is a space there at
// all. That is what lets `{{if Care_Setting != ''}}{{Care_Setting}}, {{end}}
// {{Service_Date|d_MMMM}}.` render correctly with the setting and without it.

// span is one piece of compiled markup.
type span interface {
	render(ctx any, r *renderState) string
}

type literalSpan struct{ text string }

type substSpan struct {
	ch      *chain
	require string // "" when the substitution asserts nothing
	pos     position
}

type ifSpan struct {
	pred predicate
	body []span
}

type withSpan struct {
	ch   *chain
	body []span
}

type eachSpan struct {
	ch        *chain
	sep, last string
	body      []span
}

// rawAction is one `{{...}}` before it is understood, with the offset a compile
// failure is reported at.
type rawAction struct {
	body string
	off  int
}

// rawItem is either literal text or an action.
type rawItem struct {
	literal string
	action  *rawAction
}

// scanMarkup splits markup into literals and actions (C4).
//
// It looks for the closing `}}` outside single-quoted literals, so a separator
// may contain one: `{{each xs sep: '}} ' last: '}} '}}` is odd and is legal.
func scanMarkup(src string, p position) ([]rawItem, error) {
	var out []rawItem
	i := 0
	for i < len(src) {
		start := strings.Index(src[i:], "{{")
		if start < 0 {
			if bad := strings.Index(src[i:], "}}"); bad >= 0 {
				return nil, compileErr(p.at(i+bad), "'}}' closes nothing; a substitution opens with '{{'")
			}
			out = append(out, rawItem{literal: src[i:]})
			break
		}
		if bad := strings.Index(src[i:i+start], "}}"); bad >= 0 {
			return nil, compileErr(p.at(i+bad), "'}}' closes nothing; a substitution opens with '{{'")
		}
		if start > 0 {
			out = append(out, rawItem{literal: src[i : i+start]})
		}
		open := i + start
		end := closingBraces(src[open+2:])
		if end < 0 {
			return nil, compileErr(p.at(open), "'{{' is never closed; a substitution ends with '}}'")
		}
		out = append(out, rawItem{action: &rawAction{
			body: strings.TrimSpace(src[open+2 : open+2+end]),
			off:  open,
		}})
		i = open + 2 + end + 2
	}
	return out, nil
}

// closingBraces returns the offset of the `}}` that closes an action, skipping
// any inside a single-quoted literal.
func closingBraces(s string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && strings.HasPrefix(s[i:], "}}") {
			return i
		}
	}
	return -1
}

// compileMarkup turns markup into a span tree.
//
// The block stack is C5 - an `{{if}}` with no `{{end}}` and an `{{end}}`
// closing nothing are both failures of this loop, and both name the offset of
// the construct that is wrong rather than of the end of the string.
func compileMarkup(src string, p position) ([]span, error) {
	items, err := scanMarkup(src, p)
	if err != nil {
		return nil, err
	}
	type frame struct {
		open  *rawAction
		kind  string
		build func(body []span) span
		body  []span
	}
	var stack []frame
	var out []span
	emit := func(s span) {
		if n := len(stack); n > 0 {
			stack[n-1].body = append(stack[n-1].body, s)
			return
		}
		out = append(out, s)
	}
	for _, it := range items {
		if it.action == nil {
			if it.literal != "" {
				emit(literalSpan{text: it.literal})
			}
			continue
		}
		act := it.action
		at := p.at(act.off)
		keyword, rest := splitKeyword(act.body)
		switch keyword {
		case "end":
			if rest != "" {
				return nil, compileErr(at, "{{end}} takes nothing, and carries %q", rest)
			}
			if len(stack) == 0 {
				return nil, compileErr(at, "{{end}} closes nothing")
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			emit(top.build(top.body))
		case "if", "with", "each":
			if err := refuseRequire(act.body, keyword, at); err != nil {
				return nil, err
			}
			if strings.TrimSpace(rest) == "" {
				return nil, compileErr(at, "{{%s}} carries no %s", keyword, argumentOf(keyword))
			}
			build, err := blockOf(keyword, rest, at)
			if err != nil {
				return nil, err
			}
			stack = append(stack, frame{open: act, kind: keyword, build: build})
		default:
			s, err := compileSubst(act.body, at)
			if err != nil {
				return nil, err
			}
			emit(s)
		}
	}
	if len(stack) > 0 {
		top := stack[len(stack)-1]
		return nil, compileErr(p.at(top.open.off), "{{%s}} is never closed; a block ends with {{end}}", top.kind)
	}
	return out, nil
}

func argumentOf(keyword string) string {
	if keyword == "if" {
		return "predicate"
	}
	return "chain"
}

// splitKeyword takes the first word of an action, which is what decides whether
// it is a block form. A substitution whose first path happens to be spelled
// `if` is not expressible, and that is stated rather than worked around: the
// three keywords are reserved.
func splitKeyword(body string) (keyword, rest string) {
	i := strings.IndexAny(body, " \t\n")
	if i < 0 {
		return body, ""
	}
	return body[:i], strings.TrimSpace(body[i+1:])
}

// refuseRequire is C12's second half: `require` is a modifier on a
// substitution and nowhere else. A block form asserting something would have
// no value to assert it about - `{{if X == 'y' require 'm'}}` reads as *this
// must hold*, and what it would mean is not decidable from the text.
func refuseRequire(body, keyword string, p position) error {
	if i := requireIndex(body); i >= 0 {
		return compileErr(p, "{{%s}} carries a 'require'; require is a modifier on a substitution, "+
			"which is the only form that reads a value", keyword)
	}
	return nil
}

// compileSubst reads a substitution and its optional `require` (C12).
func compileSubst(body string, p position) (span, error) {
	chainText := body
	message := ""
	if i := requireIndex(body); i >= 0 {
		chainText = strings.TrimSpace(body[:i])
		arg := strings.TrimSpace(body[i+len("require"):])
		lit, ok := unquote(arg)
		if !ok || strings.TrimSpace(lit) == "" {
			return nil, compileErr(p, "require carries no message; it is written "+
				"{{ chain require 'what was not there' }} and the message is what a reader of the "+
				"violation has to act on")
		}
		message = lit
	}
	ch, err := parseChain(chainText, p)
	if err != nil {
		return nil, err
	}
	return substSpan{ch: ch, require: message, pos: p}, nil
}

// requireIndex finds the `require` modifier: a whole word, outside quotes, so
// that a property named `required_by` and a literal carrying the word are both
// left alone.
func requireIndex(body string) int {
	inQuote := false
	for i := 0; i < len(body); i++ {
		if body[i] == '\'' {
			inQuote = !inQuote
			continue
		}
		if inQuote || !strings.HasPrefix(body[i:], "require") {
			continue
		}
		end := i + len("require")
		if (i == 0 || isMarkupSpace(body[i-1])) && (end == len(body) || isMarkupSpace(body[end])) {
			return i
		}
	}
	return -1
}

func isMarkupSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// blockOf compiles a block form's header and returns what builds it once its
// body is known.
func blockOf(keyword, rest string, p position) (func([]span) span, error) {
	switch keyword {
	case "if":
		pred, err := parsePredicate(rest, p)
		if err != nil {
			return nil, err
		}
		return func(body []span) span { return ifSpan{pred: pred, body: body} }, nil
	case "with":
		ch, err := parseChain(rest, p)
		if err != nil {
			return nil, err
		}
		if err := ch.requireNodes("with", p); err != nil {
			return nil, err
		}
		return func(body []span) span { return withSpan{ch: ch, body: body} }, nil
	default:
		ch, sep, last, err := parseEachHeader(rest, p)
		if err != nil {
			return nil, err
		}
		return func(body []span) span {
			return eachSpan{ch: ch, sep: sep, last: last, body: body}
		}, nil
	}
}

// parseEachHeader reads `chain sep: 'a' last: 'b'`.
//
// **`sep` and `last` are both required and are not defaulted**, for `width`'s
// reason (§10.7): the shape of the output depends on them and a silent default
// is a byte difference nobody is looking for. A template that wants one
// separator throughout writes the same literal twice, which is visible.
//
// They are found from the right, so a construct argument earlier in the chain
// cannot be mistaken for them.
func parseEachHeader(rest string, p position) (*chain, string, string, error) {
	lastAt := lastIndexOutsideQuotes(rest, "last:")
	sepAt := -1
	if lastAt >= 0 {
		sepAt = lastIndexOutsideQuotes(rest[:lastAt], "sep:")
	}
	if lastAt < 0 || sepAt < 0 {
		return nil, "", "", compileErr(p,
			"{{each}} is written {{each chain sep: 'a' last: 'b'}} and carries %q; "+
				"both separators are required", rest)
	}
	sep, ok := unquote(strings.TrimSpace(rest[sepAt+len("sep:") : lastAt]))
	if !ok {
		return nil, "", "", compileErr(p, "{{each}}: sep takes %s", argLiteral)
	}
	last, ok := unquote(strings.TrimSpace(rest[lastAt+len("last:"):]))
	if !ok {
		return nil, "", "", compileErr(p, "{{each}}: last takes %s", argLiteral)
	}
	ch, err := parseChain(strings.TrimSpace(rest[:sepAt]), p)
	if err != nil {
		return nil, "", "", err
	}
	if err := ch.requireNodes("each", p); err != nil {
		return nil, "", "", err
	}
	return ch, sep, last, nil
}

func lastIndexOutsideQuotes(s, sub string) int {
	best := -1
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && strings.HasPrefix(s[i:], sub) {
			best = i
		}
	}
	return best
}
