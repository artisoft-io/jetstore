package datatable

// Every SQL file of the rule workspaces, and their report directives, saves:
// the runbook and reports `.sql` rows (2026-11-14) refuse none of them. The
// workspaces are separate repositories, so the test skips unless
// JETS_SQL_CORPUS_DIR names a directory holding one or more workspaces, each
// a directory with a workspace_control.json:
//
//	JETS_SQL_CORPUS_DIR=<...>/workspaces go test -count=1 -run WorkspaceSql ./jets/datatable/
//
// Run it with -count=1: the corpus is outside this Go module, so nothing in
// the test cache key changes when the workspaces move. The counts are logged,
// not asserted; the property is that nothing is refused.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/run_reports/directives"
	"github.com/artisoft-io/jetstore/jets/runbook"
)

func TestWorkspaceSqlFilesPassTheSaveCheck(t *testing.T) {
	dir := os.Getenv("JETS_SQL_CORPUS_DIR")
	if dir == "" {
		t.Skip("JETS_SQL_CORPUS_DIR is not set; see the header of this file")
	}
	var roots []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "workspace_control.json" {
			roots = append(roots, filepath.Dir(p))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	if len(roots) == 0 {
		t.Fatalf("no workspace (a workspace_control.json) under %s", dir)
	}
	for _, root := range roots {
		ws := os.DirFS(root)
		counts := map[string]int{}
		err := fs.WalkDir(ws, ".", func(rel string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			kind := ""
			switch {
			case rel == directives.ConfigFile:
				kind = "report directives"
			case strings.HasSuffix(strings.ToLower(rel), ".sql") && isRunbookSQL(rel):
				kind = runbook.Dir + " sql"
			case strings.HasSuffix(strings.ToLower(rel), ".sql") && isReportsSQL(rel):
				kind = "reports sql"
			default:
				return nil
			}
			content, err := fs.ReadFile(ws, rel)
			if err != nil {
				return err
			}
			counts[kind]++
			if _, err := checkWorkspaceFileIn(rel, string(content), ws); err != nil {
				t.Errorf("%s: %s is refused by the save check: %v", filepath.Base(root), rel, err)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
		t.Logf("%s: %v", filepath.Base(root), counts)
	}
}
