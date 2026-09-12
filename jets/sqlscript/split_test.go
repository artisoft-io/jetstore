package sqlscript

import (
	"strings"
	"testing"
)

func bodies(t *testing.T, script string) []string {
	t.Helper()
	stmts, err := Split(script)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	out := make([]string, len(stmts))
	for i := range stmts {
		out[i] = stmts[i].Body
	}
	return out
}

func TestSplitPlain(t *testing.T) {
	got := bodies(t, "SELECT 1;\nSELECT 2;\n")
	want := []string{"SELECT 1", "SELECT 2"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The failure that produced this package: a `;` in a `--` comment split the
// statement in two, and the second fragment began mid-sentence with bare prose,
// so PostgreSQL reported a syntax error nowhere near the semicolon.
func TestSemicolonInLineComment(t *testing.T) {
	script := `-- Registering it does not schedule it; a pipeline runs when somebody
-- starts it.
INSERT INTO t VALUES (1);
`
	got := bodies(t, script)
	if len(got) != 1 {
		t.Fatalf("got %d statements %q, want 1", len(got), got)
	}
	if got[0] != "INSERT INTO t VALUES (1)" {
		t.Fatalf("body = %q", got[0])
	}
}

// The live instance in workspaces/walrus_ws: a multi-line JSON literal holding
// `Drug on Formulary; Non-preferred`. Two client init scripts carry twelve of
// these and could not be loaded at all before this package.
func TestSemicolonInStringLiteral(t *testing.T) {
	script := `INSERT INTO t VALUES ('{
  "I": "Drug on Formulary; Non-preferred",
  "J": "Non Formulary; Non-preferred"
}');
SELECT 2;
`
	got := bodies(t, script)
	if len(got) != 2 {
		t.Fatalf("got %d statements %q, want 2", len(got), got)
	}
	if !strings.Contains(got[0], "Non-preferred") || got[1] != "SELECT 2" {
		t.Fatalf("bodies = %q", got)
	}
}

func TestSplitConstructs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script string
		want   int
	}{
		{"escaped quote", `SELECT 'a;''b;c'; SELECT 2;`, 2},
		{"quoted identifier", `SELECT "col;name" FROM t; SELECT 2;`, 2},
		{"escaped quoted identifier", `SELECT "a;""b;c" FROM t; SELECT 2;`, 2},
		{"E string backslash", `SELECT E'a\';b'; SELECT 2;`, 2},
		{"E string backslash backslash", `SELECT E'a\\';SELECT 2;`, 2},
		{"block comment", "/* a; b */ SELECT 1; SELECT 2;", 2},
		{"nested block comment", "/* a; /* b; */ c; */ SELECT 1;", 1},
		{"dollar quote empty tag", `DO $$ BEGIN x; y; END $$; SELECT 2;`, 2},
		{"dollar quote named tag", `DO $fn$ a; $fn$; SELECT 2;`, 2},
		{"dollar quote nested other tag", `DO $a$ x; $b$ y; $b$ z; $a$;`, 1},
		{"parameter placeholder", `SELECT $1; SELECT $2;`, 2},
		{"dollar in identifier", `SELECT foo$bar; SELECT 2;`, 2},
		{"substitution token", `SELECT * FROM t WHERE session_id='$SESSIONID';`, 1},
		{"empty statements skipped", `;;SELECT 1;;;`, 1},
		{"comment to eof", "SELECT 1;\n-- trailing; note", 2},
	} {
		got := bodies(t, tc.script)
		if len(got) != tc.want {
			t.Errorf("%s: got %d statements %q, want %d", tc.name, len(got), got, tc.want)
		}
	}
}

// PostgreSQL does not require the last statement of a script to be terminated.
// The reader this package replaces dropped it silently, which turned a missing
// semicolon into a statement that never ran and never complained.
func TestUnterminatedFinalStatementIsReturned(t *testing.T) {
	got := bodies(t, "SELECT 1;\nSELECT 2\n")
	if len(got) != 2 || got[1] != "SELECT 2" {
		t.Fatalf("got %q, want the final statement returned", got)
	}
}

// A trailing comment is a chunk with no body. It is returned rather than
// dropped so that a caller which gives a comment meaning — run_reports names
// its report in one — can tell a dangling name from a finished file.
func TestTrailingCommentIsBodylessStatement(t *testing.T) {
	stmts, err := Split("SELECT 1;\n-- end of file\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2", len(stmts))
	}
	if stmts[1].Body != "" || len(stmts[1].Comments) != 1 || stmts[1].Comments[0].Text != "end of file" {
		t.Fatalf("trailing chunk = %+v", stmts[1])
	}
	if stmts[1].BodyLine != 0 {
		t.Fatalf("BodyLine = %d, want 0 for a bodyless chunk", stmts[1].BodyLine)
	}
}

func TestLeadingComments(t *testing.T) {
	script := "SELECT 0;\n\n-- first line\n-- process=out.csv;\n/* block */\nSELECT 1 -- inline; kept\n  FROM t;\n"
	stmts, err := Split(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2", len(stmts))
	}
	s := stmts[1]
	want := []Comment{{"first line", 3}, {"process=out.csv;", 4}, {"block", 5}}
	if len(s.Comments) != len(want) {
		t.Fatalf("comments = %+v, want %+v", s.Comments, want)
	}
	for i := range want {
		if s.Comments[i] != want[i] {
			t.Fatalf("comments = %+v, want %+v", s.Comments, want)
		}
	}
	// A comment inside the statement belongs to the body, not to the block.
	if !strings.Contains(s.Body, "-- inline; kept") {
		t.Fatalf("body = %q, want the inline comment retained", s.Body)
	}
	if !strings.HasPrefix(s.Body, "SELECT 1") {
		t.Fatalf("body = %q, want it to start at the first non-comment token", s.Body)
	}
	if s.Line != 3 {
		t.Errorf("Line = %d, want 3 (the first comment line)", s.Line)
	}
	if s.BodyLine != 6 {
		t.Errorf("BodyLine = %d, want 6 (the SELECT)", s.BodyLine)
	}
}

func TestLineNumbers(t *testing.T) {
	script := "SELECT 1;\n\nSELECT '\n\n';\nSELECT 3;\n"
	stmts, err := Split(script)
	if err != nil {
		t.Fatal(err)
	}
	wantLines := []int{1, 3, 6}
	if len(stmts) != len(wantLines) {
		t.Fatalf("got %d statements, want %d", len(stmts), len(wantLines))
	}
	for i, want := range wantLines {
		if stmts[i].Line != want {
			t.Errorf("statement %d Line = %d, want %d", i, stmts[i].Line, want)
		}
	}
}

func TestUnterminatedConstructsAreErrors(t *testing.T) {
	for _, tc := range []struct {
		name, script, wantSubstr string
	}{
		{"string", "SELECT 1;\nSELECT 'oops;\n", "unterminated string literal opened on line 2"},
		{"identifier", "SELECT 1;\nSELECT \"oops;\n", "unterminated quoted identifier opened on line 2"},
		{"block comment", "SELECT 1;\n/* oops\n", "unterminated block comment opened on line 2"},
		{"dollar quote", "SELECT 1;\nDO $fn$ oops;\n", "unterminated dollar-quoted string $fn$ opened on line 2"},
	} {
		_, err := Split(tc.script)
		if err == nil {
			t.Errorf("%s: want an error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSubstr) {
			t.Errorf("%s: error = %q, want it to contain %q", tc.name, err, tc.wantSubstr)
		}
	}
}

func TestLocate(t *testing.T) {
	script := "SELECT 1;\nSELECT oops;\n"
	// 1-based character position of the 'o' of "oops": 10 chars on line 1
	// including the newline, then "SELECT " is 7.
	line, col := Locate(script, 18)
	if line != 2 || col != 8 {
		t.Errorf("Locate = (%d,%d), want (2,8)", line, col)
	}
	// Multi-byte characters count as one, as the protocol specifies.
	if line, col := Locate("é\nx", 3); line != 2 || col != 1 {
		t.Errorf("Locate over multi-byte = (%d,%d), want (2,1)", line, col)
	}
	if line, col := Locate(script, 0); line != 0 || col != 0 {
		t.Errorf("Locate(0) = (%d,%d), want (0,0)", line, col)
	}
	if line, col := Locate(script, 999); line != 0 || col != 0 {
		t.Errorf("Locate(999) = (%d,%d), want (0,0)", line, col)
	}
}

func TestSplitEmpty(t *testing.T) {
	for _, s := range []string{"", "\n\n", "   \t\n"} {
		stmts, err := Split(s)
		if err != nil {
			t.Fatalf("Split(%q): %v", s, err)
		}
		if len(stmts) != 0 {
			t.Errorf("Split(%q) = %d statements, want 0", s, len(stmts))
		}
	}
}
