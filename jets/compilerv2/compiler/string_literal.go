package compiler

import "strings"

// String literals
//
// The grammar's STRING token is a double-quoted string in which \" and \\ are
// escapes (JetRule.g4, STRING and ESC). Any other character may follow a
// backslash, so "\d" and "\t" lex too. UnquoteString gives the value an author
// means by such a token, and QuoteString writes a value back as one:
//
//	\"  a double quote
//	\\  a backslash
//	\t  a tab
//	\n  a line feed
//	\r  a carriage return
//
// Any other backslash is kept as written, with the character after it, so a
// regular expression such as "^(\d\d\d)" reaches the engine as ^(\d\d\d), as
// it always has. To store a backslash before one of " \ t n r, double it.
//
// Until 2026-11-13 the compiler only removed the quotes (StripQuotes), so
// "\"" was stored as a backslash and a quote, "a\\b" kept both backslashes and
// "\t" was a backslash and a t.

// UnquoteString returns the value of a STRING token: s without its double
// quotes, with its escapes replaced by the characters they stand for. Text
// that is not double-quoted is returned unchanged, so a caller may pass a
// token that can also be a keyword or an identifier.
func UnquoteString(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	body := s[1 : len(s)-1]
	if !strings.Contains(body, `\`) {
		return body
	}
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\\' && i+1 < len(body) {
			if r, ok := unescaped[body[i+1]]; ok {
				b.WriteByte(r)
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// unescaped maps the character after a backslash to the one the escape stands for.
var unescaped = map[byte]byte{'"': '"', '\\': '\\', 't': '\t', 'n': '\n', 'r': '\r'}

// QuoteString writes v as a STRING token that UnquoteString reads back as v.
//
// A double quote, a tab, a line feed and a carriage return are escaped. A
// backslash is doubled only where it would otherwise be read as the start of
// an escape — before one of " \ t n r, before a character that is itself
// escaped, or at the end — so a regular expression keeps its single
// backslashes: ^(\d\d\d) is written "^(\d\d\d)".
func QuoteString(v string) string {
	var b strings.Builder
	b.Grow(len(v) + 2)
	b.WriteByte('"')
	for i := 0; i < len(v); i++ {
		switch c := v[i]; c {
		case '"':
			b.WriteString(`\"`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			if i+1 == len(v) || strings.IndexByte("\"\\tnr\t\n\r", v[i+1]) >= 0 {
				b.WriteString(`\\`)
			} else {
				b.WriteByte('\\')
			}
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
