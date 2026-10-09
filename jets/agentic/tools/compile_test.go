package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The happy path, through the registry rather than by calling the handler —
// the registry is the interface the Phase-1 loop uses, so that is what should
// be exercised.
func TestCompileRuleFile_ValidRuleCompiles(t *testing.T) {
	ws := fixture(t)
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	out, err := reg.Call(context.Background(), ws, "compile_rule_file", compileReq(t, `
# A rule file that declares its own vocabulary and uses it.
resource is_valid = "is_valid";
text hello = "hello";
`, ""))
	if err != nil {
		t.Fatalf("unexpected tool error: %v", err)
	}
	report, ok := out.(*CompileReport)
	if !ok {
		t.Fatalf("expected a *CompileReport, got %T", out)
	}
	if !report.Valid {
		t.Fatalf("expected a clean compile, got diagnostics: %v", report.Diagnostics)
	}
}

// The path the repair loop actually depends on: a broken rule comes back with
// a diagnostic that says what and where, not merely that something failed.
func TestCompileRuleFile_InvalidRuleReportsWhereItBroke(t *testing.T) {
	ws := fixture(t)
	out, err := CompileRuleFile(context.Background(), ws, compileReq(t,
		"# line 1 is a comment\nthis is not valid jetrule syntax @@@\n", "broken_rule.jr"))
	if err != nil {
		t.Fatalf("a rule that does not compile is a verdict, not a tool failure: %v", err)
	}
	report := out.(*CompileReport)
	if report.Valid {
		t.Fatal("expected the compile to fail")
	}
	if len(report.Diagnostics) == 0 {
		t.Fatal("a failed compile with no diagnostics is the state this tool exists to end")
	}
	var located int
	for _, d := range report.Diagnostics {
		if d.File == "" {
			continue
		}
		located++
		if d.File != "broken_rule.jr" {
			t.Errorf("diagnostic names %q; the error is in the submitted file, named broken_rule.jr", d.File)
		}
		if d.Line != 2 {
			t.Errorf("diagnostic reports line %d; the offending line is 2", d.Line)
		}
	}
	if located == 0 {
		t.Errorf("no diagnostic carries a file, so a repair prompt could not say where: %v", report.Diagnostics)
	}
}

// The tool must be inert. A validator that mutates the workspace it validates
// is worse than no validator, and the specific hazard is Phase 0's F4: a full
// compile deletes workspace.db before writing it. Running with saveJson false
// is what avoids that, and this asserts the outcome rather than the setting.
func TestCompileRuleFile_LeavesTheWorkspaceUntouched(t *testing.T) {
	ws := fixture(t)
	dir, err := ws.LocalDir()
	if err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, dir)
	for _, text := range []string{
		"text greeting = \"hello\";\n",
		"this does not compile @@@\n",
	} {
		if _, err := CompileRuleFile(context.Background(), ws, compileReq(t, text, "")); err != nil {
			t.Fatalf("tool error: %v", err)
		}
	}
	after := snapshot(t, dir)
	for path, sum := range before {
		if after[path] != sum {
			t.Errorf("%s changed during a compile; the live workspace must be untouched", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("%s was created in the live workspace during a compile", path)
		}
	}
}

// A bad file_name is rejected with something a caller can act on, rather than
// reaching the compiler's path confinement and failing from three layers down.
func TestCompileRuleFile_RejectsUnusableFileNames(t *testing.T) {
	ws := fixture(t)
	for _, tc := range []struct{ name, fileName, wants string }{
		{"path separator", "sub/dir.jr", "bare file name"},
		{"parent escape", "../escape.jr", "bare file name"},
		{"wrong suffix", "rules.txt", ".jr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileRuleFile(context.Background(), ws, compileReq(t, "text x = \"y\";\n", tc.fileName))
			if err == nil {
				t.Fatalf("expected %q to be rejected", tc.fileName)
			}
			if !strings.Contains(err.Error(), tc.wants) {
				t.Errorf("error %q does not explain the problem (wanted mention of %q)", err, tc.wants)
			}
		})
	}
}

// Blank input is a caller error, not an invalid rule: reporting "your rule
// does not compile" for an empty submission would be misleading.
func TestCompileRuleFile_BlankTextIsAToolError(t *testing.T) {
	ws := fixture(t)
	if _, err := CompileRuleFile(context.Background(), ws, compileReq(t, "   \n\t\n", "")); err == nil {
		t.Fatal("expected blank rule_text to be rejected")
	}
}

// A workspace with no local materialisation says so plainly rather than
// failing somewhere inside the compiler with a path that was never there.
func TestCompileRuleFile_NoLocalDirIsExplained(t *testing.T) {
	_, err := CompileRuleFile(context.Background(), &Workspace{}, compileReq(t, "text x = \"y\";\n", ""))
	if err == nil {
		t.Fatal("expected an error for a workspace with no local directory")
	}
	if !strings.Contains(err.Error(), "local directory") {
		t.Errorf("error %q does not name the reason", err)
	}
}

func compileReq(t *testing.T, ruleText, fileName string) json.RawMessage {
	t.Helper()
	args := map[string]string{"rule_text": ruleText}
	if fileName != "" {
		args["file_name"] = fileName
	}
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// snapshot records every file under dir with its size and modification time —
// enough to catch a rewrite, a deletion or a creation without hashing a whole
// workspace.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = info.ModTime().String() + ":" + strconv.FormatInt(info.Size(), 10)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
