// Package sqlscript splits a PostgreSQL script into its individual statements.
//
// It exists because JetStore reads SQL from workspace files and has to decide
// where one statement ends and the next begins: the workspace init db scripts
// under `process_config/` and the report scripts under `reports/`. Reading up to
// the next `;` byte is wrong, because a `;` is an ordinary character inside a
// comment, a string literal, a quoted identifier or a dollar-quoted body — and
// the failure is reported by PostgreSQL as a syntax error in the *following*
// fragment, a long way from the semicolon that caused it.
//
// The scanner recognises the lexical constructs PostgreSQL's own lexer
// recognises, to the extent needed to find statement boundaries:
//
//   - `-- ...` to end of line
//   - `/* ... */`, nesting, as PostgreSQL does (unlike the SQL standard)
//   - `'...'` with `”` as an embedded quote
//   - `E'...'` and `e'...'` with backslash escapes
//   - `"..."` quoted identifiers with `""` as an embedded quote
//   - `$tag$...$tag$` dollar quoting, including the empty tag `$$...$$`
//   - identifiers, so that a `$` inside one (`foo$bar`) is not mistaken for the
//     start of a dollar-quoted string, and `$1` stays a parameter placeholder
//
// It is a boundary scanner and not a parser: it does not know keywords and does
// not validate anything beyond the constructs above being closed.
//
// Prefer *not* splitting at all where the whole script can be handed to
// PostgreSQL in one `Exec` — a multi-statement simple query, which is what
// pgx sends when Exec is called with no arguments, is executed by the server as
// one implicit transaction and parsed by the only lexer that cannot disagree
// with PostgreSQL. Split is for callers that need the statements separately
// because each one has its own meaning to JetStore, which is the case for the
// report scripts: each statement produces its own output file.
package sqlscript

import (
	"fmt"
	"strings"
)

// Statement is one statement found in a script.
type Statement struct {
	// Text is the statement exactly as written, including any comments that
	// precede it, with the terminating `;` and surrounding whitespace removed.
	Text string

	// Body is Text with the leading comment block removed: the statement from
	// its first non-comment token. It is empty for a chunk that holds nothing
	// but comments, which is what a trailing comment at the end of a file
	// produces. Callers that execute a statement should execute Body.
	Body string

	// Comments holds the comment block that precedes Body, one entry per
	// comment, in source order. A `--` comment contributes one entry per line.
	// Comments appearing *inside* the statement are part of Body and are not
	// listed here.
	Comments []Comment

	// Line is the 1-based line of the source where Text starts, and BodyLine
	// the 1-based line where Body starts. They differ when the statement has a
	// leading comment block. BodyLine is 0 when Body is empty.
	Line     int
	BodyLine int
}

// Comment is one comment of a statement's leading comment block: its text with
// the `--` or `/* */` markers and surrounding whitespace removed, and the
// 1-based line of the source it starts on.
type Comment struct {
	Text string
	Line int
}

// Split returns the statements of script in source order.
//
// A statement is terminated by a top-level `;`. Trailing text after the last
// `;` is returned as a final statement when it holds anything other than
// whitespace, rather than being dropped: PostgreSQL does not require the last
// statement of a script to be terminated, and silently discarding it hides an
// authoring slip instead of reporting one. Empty chunks — `;;`, or whitespace
// between two terminators — are not returned.
//
// An unterminated block comment, string literal, quoted identifier or
// dollar-quoted string is an error naming the line on which it opened. That is
// the diagnostic a caller most wants and the one PostgreSQL cannot give,
// because by the time the server sees the text the damage has already been done
// by whatever split it.
func Split(script string) ([]Statement, error) {
	var out []Statement
	start := 0     // byte offset where the current chunk starts
	startLine := 1 // line on which the current chunk starts
	line := 1

	flush := func(end int) {
		if s, ok := statementOf(script[start:end], startLine); ok {
			out = append(out, s)
		}
	}

	i, n := 0, len(script)
	for i < n {
		switch c := script[i]; {
		case c == '\n':
			line++
			i++

		case c == ';':
			flush(i)
			i++
			start, startLine = i, line

		case c == '-' && i+1 < n && script[i+1] == '-':
			// Line comment: runs to the newline, which the next loop pass counts.
			if j := strings.IndexByte(script[i:], '\n'); j >= 0 {
				i += j
			} else {
				i = n
			}

		case c == '/' && i+1 < n && script[i+1] == '*':
			end, endLine, err := skipBlockComment(script, i, line)
			if err != nil {
				return nil, err
			}
			i, line = end, endLine

		case c == '\'':
			end, endLine, err := skipQuoted(script, i+1, line, '\'', false)
			if err != nil {
				return nil, err
			}
			i, line = end, endLine

		case c == '"':
			end, endLine, err := skipQuoted(script, i+1, line, '"', false)
			if err != nil {
				return nil, err
			}
			i, line = end, endLine

		case (c == 'e' || c == 'E') && i+1 < n && script[i+1] == '\'' && !isIdentCont(prevByte(script, i)):
			// E'...' — backslash escapes are honoured inside it.
			end, endLine, err := skipQuoted(script, i+2, line, '\'', true)
			if err != nil {
				return nil, err
			}
			i, line = end, endLine

		case c == '$':
			if tag, ok := dollarTag(script, i); ok {
				end, endLine, err := skipDollarQuoted(script, i+len(tag), line, tag)
				if err != nil {
					return nil, err
				}
				i, line = end, endLine
			} else {
				// A parameter placeholder ($1) or a stray dollar sign.
				i++
			}

		case isIdentStart(c):
			// Consume the whole identifier so that a `$` within it — which
			// PostgreSQL allows after the first character — cannot be read as
			// the start of a dollar-quoted string.
			i++
			for i < n && isIdentCont(script[i]) {
				i++
			}

		default:
			i++
		}
	}
	flush(n)
	return out, nil
}

// Locate converts a 1-based character position, as PostgreSQL reports it in the
// Position field of an error, to a 1-based line and column in script. The
// protocol measures Position in characters rather than bytes, so the count is
// over runes. A position outside the script yields (0, 0).
func Locate(script string, position int) (line, col int) {
	if position < 1 {
		return 0, 0
	}
	line, col, at := 1, 1, 1
	for _, r := range script {
		if at == position {
			return line, col
		}
		at++
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	if at == position {
		// A position one past the last character, which PostgreSQL reports for
		// an error at the very end of a statement.
		return line, col
	}
	return 0, 0
}

// statementOf builds a Statement from one raw chunk, reporting false when the
// chunk holds nothing at all.
func statementOf(chunk string, chunkLine int) (Statement, bool) {
	// Advance chunkLine past the leading whitespace that the previous
	// terminator left behind, so Line names the first line of real text.
	for i := 0; i < len(chunk); i++ {
		switch chunk[i] {
		case '\n':
			chunkLine++
		case ' ', '\t', '\r', '\v', '\f':
		default:
			chunk = chunk[i:]
			goto trimmed
		}
	}
	return Statement{}, false
trimmed:
	text := strings.TrimRight(chunk, " \t\r\n\v\f")
	comments, body, bodyOffset := leadingComments(text, chunkLine)
	bodyLine := 0
	if body != "" {
		bodyLine = chunkLine + strings.Count(text[:bodyOffset], "\n")
	}
	return Statement{
		Text:     text,
		Body:     body,
		Comments: comments,
		Line:     chunkLine,
		BodyLine: bodyLine,
	}, true
}

// leadingComments peels the comment block off the front of a statement,
// returning the comment texts, the remaining body and the body's byte offset
// within text.
func leadingComments(text string, firstLine int) (comments []Comment, body string, offset int) {
	i, n := 0, len(text)
	line := firstLine
	for i < n {
		switch {
		case text[i] == '\n':
			line++
			i++
		case text[i] == ' ' || text[i] == '\t' || text[i] == '\r' || text[i] == '\v' || text[i] == '\f':
			i++
		case text[i] == '-' && i+1 < n && text[i+1] == '-':
			end := n
			if j := strings.IndexByte(text[i:], '\n'); j >= 0 {
				end = i + j
			}
			comments = append(comments, Comment{strings.TrimSpace(text[i+2 : end]), line})
			i = end
		case text[i] == '/' && i+1 < n && text[i+1] == '*':
			// Split has already established that the comment closes.
			end, endLine, err := skipBlockComment(text, i, line)
			if err != nil {
				return comments, strings.TrimSpace(text[i:]), i
			}
			comments = append(comments, Comment{strings.TrimSpace(text[i+2 : end-2]), line})
			i, line = end, endLine
		default:
			return comments, strings.TrimSpace(text[i:]), i
		}
	}
	return comments, "", 0
}

func skipBlockComment(s string, i, line int) (int, int, error) {
	openLine := line
	depth := 0
	n := len(s)
	for i < n {
		switch {
		case i+1 < n && s[i] == '/' && s[i+1] == '*':
			depth++
			i += 2
		case i+1 < n && s[i] == '*' && s[i+1] == '/':
			depth--
			i += 2
			if depth == 0 {
				return i, line, nil
			}
		default:
			if s[i] == '\n' {
				line++
			}
			i++
		}
	}
	return 0, 0, fmt.Errorf("unterminated block comment opened on line %d", openLine)
}

// skipQuoted scans from i, which is just past the opening quote, to just past
// the matching close. A doubled quote character is an embedded one. When
// backslash is true a backslash escapes the next character, which is how
// PostgreSQL reads an E” string.
func skipQuoted(s string, i, line int, quote byte, backslash bool) (int, int, error) {
	openLine := line
	n := len(s)
	for i < n {
		switch c := s[i]; {
		case backslash && c == '\\' && i+1 < n:
			if s[i+1] == '\n' {
				line++
			}
			i += 2
		case c == quote:
			if i+1 < n && s[i+1] == quote {
				i += 2
				continue
			}
			return i + 1, line, nil
		case c == '\n':
			line++
			i++
		default:
			i++
		}
	}
	kind := "string literal"
	if quote == '"' {
		kind = "quoted identifier"
	}
	return 0, 0, fmt.Errorf("unterminated %s opened on line %d", kind, openLine)
}

// dollarTag reports whether the `$` at i opens a dollar-quoted string, and if
// so returns the opening delimiter including both dollar signs.
func dollarTag(s string, i int) (string, bool) {
	j := i + 1
	n := len(s)
	if j < n && isIdentStart(s[j]) {
		j++
		for j < n && isIdentCont(s[j]) && s[j] != '$' {
			j++
		}
	}
	if j < n && s[j] == '$' {
		return s[i : j+1], true
	}
	return "", false
}

func skipDollarQuoted(s string, i, line int, tag string) (int, int, error) {
	openLine := line
	rest := s[i:]
	j := strings.Index(rest, tag)
	if j < 0 {
		return 0, 0, fmt.Errorf("unterminated dollar-quoted string %s opened on line %d", tag, openLine)
	}
	line += strings.Count(rest[:j+len(tag)], "\n")
	return i + j + len(tag), line, nil
}

func prevByte(s string, i int) byte {
	if i == 0 {
		return 0
	}
	return s[i-1]
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '$'
}
