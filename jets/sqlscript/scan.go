package sqlscript

import "strings"

// TokenKind is what a Token is.
type TokenKind int

const (
	// Other is one byte that begins none of the constructs below: whitespace,
	// a newline, punctuation, an operator character, a digit, or a `$` that
	// does not open a dollar-quoted string (a placeholder such as `$1`, or a
	// stray one). Split looks for the `;` among them.
	Other TokenKind = iota
	// LineComment is `-- ...` up to the newline, which is not part of it.
	LineComment
	// BlockComment is `/* ... */`, nesting as PostgreSQL's does.
	BlockComment
	// String is `'...'`, a doubled quote being an embedded one.
	String
	// EscapeString is `E'...'` or `e'...'`, in which a backslash escapes the
	// next character.
	EscapeString
	// QuotedIdent is `"..."`, a doubled quote being an embedded one.
	QuotedIdent
	// DollarString is `$tag$...$tag$`, the empty tag `$$...$$` included.
	DollarString
	// Ident is an identifier or a keyword: a letter, `_` or a non-ASCII byte,
	// then any of those, digits and `$`. Read whole, so that a `$` inside one
	// (`foo$bar`) is not the start of a dollar-quoted string.
	Ident
)

// Token is one lexical token of a script.
type Token struct {
	Kind TokenKind
	// Text is the token exactly as written, its delimiters included.
	Text string
	// Offset is the byte offset of Text in the script.
	Offset int
	// Line is the 1-based line of the script the token starts on.
	Line int
}

// Inner is a quoted token's text without its delimiters: the body of a
// String, EscapeString, QuotedIdent or DollarString, as written (a doubled
// quote stays doubled, an escape stays escaped). For any other kind it is
// Text.
func (t Token) Inner() string {
	switch t.Kind {
	case String, QuotedIdent:
		return t.Text[1 : len(t.Text)-1]
	case EscapeString:
		return t.Text[2 : len(t.Text)-1]
	case DollarString:
		tag, _ := dollarTag(t.Text, 0)
		return t.Text[len(tag) : len(t.Text)-len(tag)]
	}
	return t.Text
}

// Scanner reads a script one token at a time, recognising the constructs
// listed in the package comment and nothing more: it is the lexing Split
// finds statement boundaries with, exported so that a caller looking for
// something else in a statement (jets/runbook looks for `:name` bind
// parameters and text substitution) reads the script the same way rather than
// keeping a copy of the rules.
type Scanner struct {
	src  string
	pos  int
	line int
}

// NewScanner returns a Scanner positioned at the start of script.
func NewScanner(script string) *Scanner {
	return &Scanner{src: script, line: 1}
}

// Next returns the next token, and false when the script is exhausted. A
// construct the script opens and never closes is an *UnterminatedError naming
// the line it opened on, after which the Scanner returns nothing more.
func (s *Scanner) Next() (Token, bool, error) {
	src, i, n := s.src, s.pos, len(s.src)
	if i >= n {
		return Token{}, false, nil
	}
	kind, end := Other, i+1
	var err error
	switch c := src[i]; {
	case c == '-' && i+1 < n && src[i+1] == '-':
		kind, end = LineComment, n
		if j := strings.IndexByte(src[i:], '\n'); j >= 0 {
			end = i + j
		}
	case c == '/' && i+1 < n && src[i+1] == '*':
		kind = BlockComment
		end, _, err = skipBlockComment(src, i, s.line)
	case c == '\'':
		kind = String
		end, _, err = skipQuoted(src, i+1, s.line, '\'', false)
	case c == '"':
		kind = QuotedIdent
		end, _, err = skipQuoted(src, i+1, s.line, '"', false)
	case (c == 'e' || c == 'E') && i+1 < n && src[i+1] == '\'' && !isIdentCont(prevByte(src, i)):
		kind = EscapeString
		end, _, err = skipQuoted(src, i+2, s.line, '\'', true)
	case c == '$':
		if tag, ok := dollarTag(src, i); ok {
			kind = DollarString
			end, _, err = skipDollarQuoted(src, i+len(tag), s.line, tag)
		}
	case isIdentStart(c):
		kind = Ident
		for end < n && isIdentCont(src[end]) {
			end++
		}
	}
	if err != nil {
		s.pos = n
		return Token{}, false, err
	}
	tok := Token{Kind: kind, Text: src[i:end], Offset: i, Line: s.line}
	s.pos = end
	s.line += strings.Count(tok.Text, "\n")
	return tok, true, nil
}

// Skip moves the Scanner n bytes forward from where the next token would
// start, counting the newlines it passes. It is for a caller that reads a
// construct of its own on top of the tokens, such as a `:name` or a
// `${...}`, and goes on after it.
func (s *Scanner) Skip(n int) {
	end := min(len(s.src), s.pos+max(n, 0))
	s.line += strings.Count(s.src[s.pos:end], "\n")
	s.pos = end
}

// IsIdentStart reports whether c may begin an identifier, as Scanner reads
// one: a letter, `_`, or any byte of a non-ASCII character.
func IsIdentStart(c byte) bool { return isIdentStart(c) }
