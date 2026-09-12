package sqlscript

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

func TestValidateScriptAcceptsTheCorpusShapes(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{"empty", ""},
		{"semicolon in a comment", "-- it does not schedule it; a pipeline runs when somebody\nSELECT 1;"},
		{"semicolon in a literal", "INSERT INTO t VALUES ('Drug on Formulary; Non-preferred');"},
		{"semicolon in a quoted identifier", `SELECT "odd;name" FROM t;`},
		{"dollar quoted body", "DO $fn$ BEGIN x; y; END $fn$;"},
		{"substitution token", "SELECT * FROM t WHERE session_id='$SESSIONID';"},
		// Both usi error reports are written this way and both work.
		{"unterminated final statement", "--out.csv;\nSELECT 1 FROM t WHERE x IS NULL\n"},
		// Six of the 26 reports/*.sql carry no name comment, legitimately.
		{"no name comment", "BEGIN;\nSELECT 1;\nCOMMIT;\n"},
		{"trailing comment", "SELECT 1;\n-- End of Export Client Script\n"},
	} {
		if got := ValidateScript(tc.script); len(got) != 0 {
			t.Errorf("%s: want no findings, got %+v", tc.name, got)
		}
	}
}

func TestValidateScriptReportsTheOpeningLine(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		wantLine     int
		wantIn       string
	}{
		{"string literal", "SELECT 1;\n\nINSERT INTO t VALUES ('oops;\n", 3, "unterminated string literal"},
		{"quoted identifier", "SELECT 1;\nSELECT \"oops;\n", 2, "unterminated quoted identifier"},
		{"block comment", "SELECT 1;\n\n\n/* oops\n", 4, "unterminated block comment"},
		{"dollar quoted", "SELECT 1;\nDO $fn$ oops;\n", 2, "unterminated dollar-quoted string $fn$"},
	} {
		got := ValidateScript(tc.script)
		if len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %+v", tc.name, got)
			continue
		}
		f := got[0]
		if f.Severity != wsvalidate.Error {
			t.Errorf("%s: severity = %q, want error", tc.name, f.Severity)
		}
		if f.Code != UnterminatedCode {
			t.Errorf("%s: code = %q, want %q", tc.name, f.Code, UnterminatedCode)
		}
		// The line the construct *opened* on, not where the file ran out.
		if f.Line != tc.wantLine {
			t.Errorf("%s: line = %d, want %d (message: %s)", tc.name, f.Line, tc.wantLine, f.Message)
		}
		// The message carries no line: it travels as Line, so that a renderer
		// printing both does not say it twice.
		if f.Message != tc.wantIn {
			t.Errorf("%s: message = %q, want %q", tc.name, f.Message, tc.wantIn)
		}
		if strings.Contains(f.Message, "line") {
			t.Errorf("%s: message = %q, want the line only in Line", tc.name, f.Message)
		}
		// A finding must carry no JSON Pointer: a SQL file has no document path.
		if f.Path != "" {
			t.Errorf("%s: path = %q, want empty", tc.name, f.Path)
		}
	}
}

// The severity is Error, which blocks a save, and that is licensed by every
// .sql file that exists lexing clean rather than by confidence. This is the
// measurement, made re-runnable: 89 files, 0 failures, 2026-09-12 — 82 under
// `workspaces/` and 7 in this repository. Pointed at the parent checkout's root
// it reports 90, the extra being a gitignored query in its `out/`; the figure
// is documentation of a scope rather than an assertion, which is why the test
// asserts only that nothing fails.
//
// Skips unless JETS_SQL_CORPUS_DIR names a directory to walk, because the
// workspaces are separate repositories and are not present in a plain checkout
// of this one — the pattern JETS_PC_CORPUS_DIR established. Run it with
// -count=1; the corpus is outside this Go module, so nothing in the test cache
// key changes when the workspaces move.
//
//	JETS_SQL_CORPUS_DIR=<...> go test -count=1 -run TestCorpusLexesClean ./jets/sqlscript/
func TestCorpusLexesClean(t *testing.T) {
	dir := os.Getenv("JETS_SQL_CORPUS_DIR")
	if dir == "" {
		t.Skip("JETS_SQL_CORPUS_DIR is not set; see the header of this test")
	}
	n := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return err
		}
		if strings.Contains(path, "/build/") || strings.Contains(path, "/.git/") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n++
		if findings := ValidateScript(string(content)); len(findings) > 0 {
			t.Errorf("%s: %s", path, findings[0].Message)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	if n == 0 {
		t.Fatalf("no .sql files under %s", dir)
	}
	t.Logf("%d .sql files lexed clean", n)
}
