package template

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// A chain alternates paths and constructs, separated by a vertical bar (§10.4):
// `has_Medical_Events[]|latest_by: Service_Date|Care_Setting` is a chain of
// three. It carries a *list* of values, and every link either addresses into
// that list, maps over it, folds it or takes the first of it.
//
// # The cardinality and the kind are compile-time facts, and C9 is both of them
//
// Walking §10.5's `Shape` and `Takes` columns over the links gives two
// judgements at every point: how many values the chain carries (`one` or
// `many`) and what kind they are. **Neither needs a record**, which is what
// makes C9 a compile failure rather than a run-time surprise.
//
// The three refusals C9 names fall out of that walk:
//
//   - a fold on a single value - `Fill_Date|max` where `Fill_Date[]|max` was
//     meant, which would silently take one date and call it the latest;
//   - a date-consuming construct after a text-producing one - `join` then
//     `d_MMMM`, where the date is already gone;
//   - `with` or `each` over a chain that does not yield nodes.
//
// **A map is refused on a single value for the same reason a fold is**, and
// §10.5 says so in terms: `Condition_Summary[]` then `strip_code` then `join`
// type-checks and `Condition_Summary[]` then `join` then `strip_code` does not.
// That is the rule most likely to refuse a template a reader thinks is fine
// (I-641), and the escape is `[]`, which yields a one-element list over a scalar
// (F1144) and costs two characters.
//
// **What is never refused is an absence of information.** A path says nothing
// about what it addressed, so it yields `kindUnknown` and every construct
// accepts it. C9 refuses a contradiction and nothing else, which is the
// narrowest form of the check that still catches the three cases above.
type chain struct {
	raw   string
	links []chainLink
	// kind and card are the chain's judgement at its end, which is what a
	// block form checks before it scopes a span to it.
	kind valueKind
	card cardinality
}

type cardinality int

const (
	cardOne cardinality = iota
	cardMany
)

func (c cardinality) String() string {
	if c == cardOne {
		return "a single value"
	}
	return "a list"
}

// chainLink is one link: a path, or a construct with its arguments.
type chainLink struct {
	path      briefing.Selector
	isPath    bool
	construct *construct
	args      []chainArg
}

// chainArg is one construct argument, in whichever of the two forms C8 admits.
type chainArg struct {
	literal string
	path    briefing.Selector
	isPath  bool
}

// parseChain compiles a chain and gives it its static judgement.
//
// `startKind` is what the chain is applied to: a node at the head of a
// substitution, and a node again inside a `with` or an `each`, because those
// are the only two things that rebind the context. There is no other context
// to be in.
func parseChain(src string, p position) (*chain, error) {
	raw := strings.TrimSpace(src)
	if raw == "" {
		return nil, compileErr(p, "an empty chain: a substitution needs a path")
	}
	parts := splitOutsideQuotes(raw, '|')
	ch := &chain{raw: raw, kind: kindNode, card: cardOne}
	prevWasPath := false
	for i, part := range parts {
		link := strings.TrimSpace(part)
		if link == "" {
			return nil, compileErr(p, "chain %q has an empty link; '|' separates a path or a construct from the next", raw)
		}
		// **Which links may be paths is what makes C6 possible at all**, and
		// §10.4 leaves it to be worked out: if any name can be a path then no
		// name is unknown, and "an unknown construct name" has nothing to
		// refuse. Two rules settle it, and neither costs the notation anything
		// §10.9 uses.
		//
		// The first link is a path, because there is no value before it but
		// the context node and no construct takes a node from nowhere.
		//
		// **A link directly after a path is a construct**, because a path
		// after a path is a path: `a.b` is one link and writing it `a|b` buys
		// nothing. So an unrecognised name there is C6 rather than a property
		// nobody will ever find. After a *construct* the rule relaxes - a
		// known name is the construct and anything else is a path - which is
		// what lets a chain address into the node `latest_by` yields (§10.4's
		// three-link example).
		//
		// The cost is I-642: a property spelled like a construct is reached by
		// writing it as a path no construct could be - with `[]`, or dotted -
		// and F1144 makes `[]` over a scalar free.
		name := constructNameOf(link)
		c, isConstruct := constructs[name]
		switch {
		case i == 0:
			if err := ch.appendPath(link, p, raw); err != nil {
				return nil, err
			}
			prevWasPath = true
			continue
		case isConstruct:
		case prevWasPath:
			return nil, compileErr(p, "chain %q: %q is not a construct; the constructs are %s",
				raw, name, constructNames())
		default:
			if err := ch.appendPath(link, p, raw); err != nil {
				return nil, err
			}
			prevWasPath = true
			continue
		}
		prevWasPath = false
		args, err := parseArgs(link, c, p)
		if err != nil {
			return nil, err
		}
		if err := ch.appendConstruct(c, args, p, raw); err != nil {
			return nil, err
		}
	}
	return ch, nil
}

// constructNameOf is the leading identifier of a link, up to its argument
// colon. A link carrying `[]`, a dot or a space is never a construct name, so
// the lookup that follows cannot claim a path.
func constructNameOf(link string) string {
	name := link
	if i := strings.IndexByte(link, ':'); i >= 0 {
		name = link[:i]
	}
	return strings.TrimSpace(name)
}

func (ch *chain) appendPath(link string, p position, raw string) error {
	if strings.ContainsAny(link, " \t\n") {
		// `ParseSelector` would accept a segment with a space in it, because a
		// briefing field is whatever a schema names. Inside markup a space is
		// how two things are separated, so a path carrying one is a missing
		// keyword rather than a property: `{{if X}}` mistyped as `{{iff X}}`
		// reaches here as the path "iff X".
		return compileErr(p, "chain %q: %q is not a path; a path carries no spaces, "+
			"and a block form is written {{if ...}}, {{with ...}} or {{each ...}}", raw, link)
	}
	if ch.kind != kindNode && ch.kind != kindUnknown {
		return compileErr(p, "chain %q: %q addresses into %s, which is not a node",
			raw, link, ch.kind)
	}
	sel, err := briefing.ParseSelector(link)
	if err != nil {
		return compileErr(p, "chain %q: %v", raw, err)
	}
	ch.links = append(ch.links, chainLink{path: sel, isPath: true})
	ch.kind = kindUnknown
	if ch.card == cardMany || strings.Contains(link, "[]") {
		ch.card = cardMany
	}
	return nil
}

func (ch *chain) appendConstruct(c *construct, args []chainArg, p position, raw string) error {
	if !accepts(c.takes, ch.kind) {
		return compileErr(p, "chain %q: %q takes %s and is applied to %s",
			raw, c.name, c.takes, ch.kind)
	}
	if c.shape != shapeFirst && ch.card == cardOne {
		return compileErr(p,
			"chain %q: %q is a %s and is applied to %s; a %s needs a list, which a path writes with []",
			raw, c.name, c.shape, ch.card, c.shape)
	}
	ch.links = append(ch.links, chainLink{construct: c, args: args})
	ch.kind = c.gives
	if c.shape != shapeMap {
		ch.card = cardOne
	}
	return nil
}

// parseArgs reads a construct's arguments and checks their number and their
// kind, which are C7 and C8.
func parseArgs(link string, c *construct, p position) ([]chainArg, error) {
	var raw string
	if i := strings.IndexByte(link, ':'); i >= 0 {
		raw = strings.TrimSpace(link[i+1:])
	}
	var written []string
	if raw != "" {
		for _, a := range splitOutsideQuotes(raw, ',') {
			written = append(written, strings.TrimSpace(a))
		}
	}
	if len(written) != len(c.args) {
		return nil, compileErr(p, "%q takes %d argument(s) and is given %d, as %q",
			c.name, len(c.args), len(written), link)
	}
	out := make([]chainArg, 0, len(written))
	for i, w := range written {
		switch c.args[i] {
		case argLiteral:
			lit, ok := unquote(w)
			if !ok {
				return nil, compileErr(p,
					"%q takes %s as argument %d and is given %q; a literal is written between single quotes",
					c.name, argLiteral, i+1, w)
			}
			out = append(out, chainArg{literal: lit})
		default:
			if _, ok := unquote(w); ok {
				return nil, compileErr(p,
					"%q takes %s as argument %d and is given the quoted literal %s",
					c.name, argPath, i+1, w)
			}
			sel, err := briefing.ParseSelector(w)
			if err != nil {
				return nil, compileErr(p, "%q: argument %d: %v", c.name, i+1, err)
			}
			out = append(out, chainArg{path: sel, isPath: true})
		}
	}
	return out, nil
}

// eval walks the chain over a context value and returns the list it carries at
// the end. **It cannot fail**: every construct is total and a path that is not
// there resolves to nothing, which is a shorter list rather than an error.
func (ch *chain) eval(ctx any) []any {
	cur := []any{ctx}
	for i := range ch.links {
		link := &ch.links[i]
		switch {
		case link.isPath:
			next := make([]any, 0, len(cur))
			for _, v := range cur {
				for _, l := range link.path.Resolve(v) {
					if l.Value != nil {
						next = append(next, l.Value)
					}
				}
			}
			cur = next
		case link.construct.shape == shapeFold:
			v, ok := link.construct.fold(cur, link.args)
			if !ok {
				cur = nil
				continue
			}
			cur = []any{v}
		case link.construct.shape == shapeFirst:
			if len(cur) == 0 {
				continue
			}
			v, ok := link.construct.one(cur[0], link.args)
			if !ok {
				cur = nil
				continue
			}
			cur = []any{v}
		default: // shapeMap
			next := make([]any, 0, len(cur))
			for _, v := range cur {
				if out, ok := link.construct.one(v, link.args); ok {
					next = append(next, out)
				}
			}
			cur = next
		}
	}
	return cur
}

// first is the value a substitution renders: the first the chain yields, and
// the rest are discarded (F1145).
func (ch *chain) first(ctx any) (any, bool) {
	vals := ch.eval(ctx)
	if len(vals) == 0 {
		return nil, false
	}
	return vals[0], true
}

// text is the first value as text, which is what a substitution writes.
func (ch *chain) text(ctx any) string {
	v, ok := ch.first(ctx)
	if !ok {
		return ""
	}
	return scalar(v)
}

// requireNodes reports whether a chain may be scoped to by `with` or `each`.
// A chain of paths alone yields `kindUnknown` and is allowed, because a path
// says nothing about what it addressed; a chain ending in a construct that
// produces text is refused, because it cannot be a node.
func (ch *chain) requireNodes(form string, p position) error {
	if ch.kind == kindNode || ch.kind == kindUnknown {
		return nil
	}
	return compileErr(p, "{{%s %s}} is scoped to %s, which is not a node", form, ch.raw, ch.kind)
}

// unquote reads a single-quoted literal.
//
// **Literals are single-quoted and there is no escape for a quote inside one**
// (R-119). The body of a template lives in a JSON string, so double quotes
// would need escaping on every `join`, every `plural` and every equality;
// R-110 is taken deliberately and there is no case for paying it twice. A
// literal that must contain an apostrophe is the case this cannot express.
func unquote(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1], true
	}
	return "", false
}

// splitOutsideQuotes splits on a separator that is not inside a single-quoted
// literal, which is what lets `join: ', ', ' and '` carry a comma.
func splitOutsideQuotes(s string, sep byte) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'':
			inQuote = !inQuote
			b.WriteByte(c)
		case c == sep && !inQuote:
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	out = append(out, b.String())
	return out
}

// indexOutsideQuotes finds a substring that is not inside a single-quoted
// literal.
func indexOutsideQuotes(s, sub string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && strings.HasPrefix(s[i:], sub) {
			return i
		}
	}
	return -1
}

// position is where in a template document a compile failure is, and it is
// `compileInferPromptTemplate`'s precedent rather than a convention of this
// package: that function names the placeholder and the position it failed at,
// because the author of the template is the reader of the error.
type position struct {
	// path is the element, as a document path: `elements[2].elements[0].text`.
	path string
	// off is the byte offset into that markup, or -1 when the failure is the
	// element itself rather than something inside its text.
	off int
}

func (p position) String() string {
	if p.off < 0 {
		return p.path
	}
	return fmt.Sprintf("%s at offset %d", p.path, p.off)
}

func (p position) at(off int) position { return position{path: p.path, off: off} }

func compileErr(p position, format string, args ...any) error {
	return fmt.Errorf("%s: %s", p, fmt.Sprintf(format, args...))
}
