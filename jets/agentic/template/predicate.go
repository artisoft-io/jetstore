package template

import (
	"strings"
)

// A predicate decides whether an element or a span is emitted (§10.4):
//
//	pred := term ( ("and"|"or") term )*   with parentheses, `and` binding tighter
//	term := chain op literal
//
// **Parentheses are in the grammar rather than left out**, because without them
// `a > 0 or b > 0 and c > 0` groups as `a > 0 or (b > 0 and c > 0)` and a
// template that means the other thing has no way to say so. An unbalanced
// parenthesis is a compile failure; a precedence trap is not catchable at all.
//
// **The two families of operator read the value differently, and that is the
// whole of what a template author has to remember.** `==` and `!=` compare the
// chain's first value as trimmed *text* and take a quoted literal; `>`, `>=`,
// `<` and `<=` compare it as a *number*, take a bare numeric literal, and are
// false when the value does not read as one. So a term comparing a property
// with the empty literal is *the property is there and is not blank*, and
// `Event_Count > 0` is *it is there and reads as a number greater than zero*,
// which is what the accessors of the package this generalises do (`asInt`,
// `jets/agentic/briefing/prose/entity.go:183`).
//
// **The literal's form is what C11 checks, and the two rules differ by
// operator.** A quoted literal after `>` is refused and a bare word after `==`
// is refused, rather than one of them being coerced: an unquoted word on the
// right of an equality is a *path* compared against a path if bare words are
// allowed, and that is a silently different question.
type predicate interface {
	eval(ctx any) bool
}

type predAnd struct{ left, right predicate }
type predOr struct{ left, right predicate }

func (p predAnd) eval(ctx any) bool { return p.left.eval(ctx) && p.right.eval(ctx) }
func (p predOr) eval(ctx any) bool  { return p.left.eval(ctx) || p.right.eval(ctx) }

type predTerm struct {
	ch  *chain
	op  string
	lit string
	num float64
}

func (p predTerm) eval(ctx any) bool {
	switch p.op {
	case "==":
		return strings.TrimSpace(p.ch.text(ctx)) == p.lit
	case "!=":
		return strings.TrimSpace(p.ch.text(ctx)) != p.lit
	}
	v, ok := p.ch.first(ctx)
	if !ok {
		return false
	}
	f, ok := asFloat(v)
	if !ok {
		return false
	}
	switch p.op {
	case ">":
		return f > p.num
	case ">=":
		return f >= p.num
	case "<":
		return f < p.num
	default:
		return f <= p.num
	}
}

// --- the parser -------------------------------------------------------------

type predToken struct {
	kind predTokenKind
	text string
}

type predTokenKind int

const (
	tokChain predTokenKind = iota // a run of chain text
	tokQuoted
	tokOp
	tokAnd
	tokOr
	tokLParen
	tokRParen
)

// lexPredicate splits a predicate into tokens. Parentheses and operators are
// recognised outside quoted literals only, so a literal may contain either.
func lexPredicate(src string) []predToken {
	var out []predToken
	var run strings.Builder
	flush := func() {
		if s := strings.TrimSpace(run.String()); s != "" {
			switch s {
			case "and":
				out = append(out, predToken{kind: tokAnd})
			case "or":
				out = append(out, predToken{kind: tokOr})
			default:
				out = append(out, predToken{kind: tokChain, text: s})
			}
		}
		run.Reset()
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\'':
			flush()
			end := strings.IndexByte(src[i+1:], '\'')
			if end < 0 {
				// An unterminated literal is reported by the parser, which has
				// the position; the lexer carries the rest as the literal.
				out = append(out, predToken{kind: tokQuoted, text: src[i+1:]})
				return out
			}
			out = append(out, predToken{kind: tokQuoted, text: src[i+1 : i+1+end]})
			i += end + 1
		case c == '(':
			flush()
			out = append(out, predToken{kind: tokLParen})
		case c == ')':
			flush()
			out = append(out, predToken{kind: tokRParen})
		case c == '=' || c == '!' || c == '<' || c == '>':
			flush()
			j := i
			for j < len(src) && (src[j] == '=' || src[j] == '!' || src[j] == '<' || src[j] == '>') {
				j++
			}
			out = append(out, predToken{kind: tokOp, text: src[i:j]})
			i = j - 1
		case c == ' ' || c == '\t' || c == '\n':
			// `and` and `or` are keywords wherever they stand alone, so a run
			// break matters; everything else inside a chain carries no spaces
			// and the chain parser refuses one that does.
			flush()
		default:
			run.WriteByte(c)
		}
	}
	flush()
	return out
}

type predParser struct {
	toks []predToken
	i    int
	pos  position
	src  string
}

// parsePredicate compiles a predicate, or says why it cannot (C11).
func parsePredicate(src string, p position) (predicate, error) {
	if strings.TrimSpace(src) == "" {
		return nil, compileErr(p, "an empty predicate")
	}
	pp := &predParser{toks: lexPredicate(src), pos: p, src: src}
	node, err := pp.parseOr()
	if err != nil {
		return nil, err
	}
	if pp.i < len(pp.toks) {
		if pp.toks[pp.i].kind == tokRParen {
			return nil, compileErr(p, "predicate %q has an unbalanced ')'", src)
		}
		return nil, compileErr(p, "predicate %q carries trailing text; "+
			"terms are joined with 'and' or 'or'", src)
	}
	return node, nil
}

func (pp *predParser) parseOr() (predicate, error) {
	left, err := pp.parseAnd()
	if err != nil {
		return nil, err
	}
	for pp.i < len(pp.toks) && pp.toks[pp.i].kind == tokOr {
		pp.i++
		right, err := pp.parseAnd()
		if err != nil {
			return nil, err
		}
		left = predOr{left: left, right: right}
	}
	return left, nil
}

func (pp *predParser) parseAnd() (predicate, error) {
	left, err := pp.parseUnary()
	if err != nil {
		return nil, err
	}
	for pp.i < len(pp.toks) && pp.toks[pp.i].kind == tokAnd {
		pp.i++
		right, err := pp.parseUnary()
		if err != nil {
			return nil, err
		}
		left = predAnd{left: left, right: right}
	}
	return left, nil
}

func (pp *predParser) parseUnary() (predicate, error) {
	if pp.i >= len(pp.toks) {
		return nil, compileErr(pp.pos, "predicate %q ends after 'and' or 'or'; "+
			"a connective joins two terms", pp.src)
	}
	if pp.toks[pp.i].kind == tokLParen {
		pp.i++
		inner, err := pp.parseOr()
		if err != nil {
			return nil, err
		}
		if pp.i >= len(pp.toks) || pp.toks[pp.i].kind != tokRParen {
			return nil, compileErr(pp.pos, "predicate %q has an unbalanced '('", pp.src)
		}
		pp.i++
		return inner, nil
	}
	return pp.parseTerm()
}

func (pp *predParser) parseTerm() (predicate, error) {
	// The chain runs until the operator, and a quoted token before the operator
	// is a construct's argument rather than the comparison's right-hand side -
	// `X[]|join: ', ', ' and ' == 'a and b'` is one term with three literals in
	// it. The quotes are put back, because the chain parser reads them.
	var chainText strings.Builder
	for pp.i < len(pp.toks) &&
		(pp.toks[pp.i].kind == tokChain || pp.toks[pp.i].kind == tokQuoted) {
		if chainText.Len() > 0 {
			chainText.WriteByte(' ')
		}
		if pp.toks[pp.i].kind == tokQuoted {
			chainText.WriteString("'" + pp.toks[pp.i].text + "'")
		} else {
			chainText.WriteString(pp.toks[pp.i].text)
		}
		pp.i++
	}
	if chainText.Len() == 0 {
		return nil, compileErr(pp.pos, "predicate %q: a term begins with a chain", pp.src)
	}
	if pp.i >= len(pp.toks) || pp.toks[pp.i].kind != tokOp {
		return nil, compileErr(pp.pos,
			"predicate %q: the term %q carries no operator; the operators are ==, !=, >, >=, < and <=",
			pp.src, chainText.String())
	}
	op := pp.toks[pp.i].text
	pp.i++
	switch op {
	case "==", "!=", ">", ">=", "<", "<=":
	default:
		return nil, compileErr(pp.pos,
			"predicate %q: %q is not an operator; the operators are ==, !=, >, >=, < and <=", pp.src, op)
	}
	ch, err := parseChain(chainText.String(), pp.pos)
	if err != nil {
		return nil, err
	}
	if pp.i >= len(pp.toks) {
		return nil, compileErr(pp.pos, "predicate %q: %q has nothing on its right", pp.src, op)
	}
	tok := pp.toks[pp.i]
	pp.i++
	term := predTerm{ch: ch, op: op}
	if op == "==" || op == "!=" {
		if tok.kind != tokQuoted {
			return nil, compileErr(pp.pos,
				"predicate %q: %q compares text and needs a quoted literal on its right, not %q",
				pp.src, op, tok.text)
		}
		term.lit = tok.text
		return term, nil
	}
	if tok.kind != tokChain {
		return nil, compileErr(pp.pos,
			"predicate %q: %q compares numbers and needs a bare number on its right, not %q",
			pp.src, op, tok.text)
	}
	f, ok := asFloat(tok.text)
	if !ok {
		return nil, compileErr(pp.pos,
			"predicate %q: %q compares numbers and %q does not read as one", pp.src, op, tok.text)
	}
	term.num = f
	return term, nil
}
