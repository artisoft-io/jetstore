package workspace

// Recording a workspace's rules unit tests in workspace.db at compile.
// Project rules_unit_test_harness, Phase 1, task AD.1.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// UnitTestsDir is where a workspace keeps its rules unit tests, relative to
// the workspace root.
const UnitTestsDir = "jet_rules/unit_tests"

// UnitTestSuffix names a rules unit test definition.
const UnitTestSuffix = ".tst.json"

// UnitTestRecord is one row of workspace.db's unit_tests table.
//
// Findings holds the error findings only, and is empty for a test that may be
// started. That is the contract the rules_unit_test insert target reads
// (track AC, I-17): a row whose findings are NULL, '', '[]' or 'null' is
// runnable, so a warning written here would refuse a test the validator
// accepts.
type UnitTestRecord struct {
	Path        string
	TestName    string
	ProcessName string
	LegacyMode  string
	Comment     string
	Findings    []wsvalidate.Finding
}

// ReadUnitTests validates every jet_rules/unit_tests/**/*.tst.json under
// workspacePath. A test whose content is wrong is returned with its findings
// rather than as an error; the error is for a tree that cannot be walked.
// The records are sorted by path.
func ReadUnitTests(workspacePath string) ([]UnitTestRecord, error) {
	root := filepath.Join(workspacePath, filepath.FromSlash(UnitTestsDir))
	records := make([]UnitTestRecord, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), UnitTestSuffix) {
			return nil
		}
		rel, err := filepath.Rel(workspacePath, path)
		if err != nil {
			return err
		}
		records = append(records, readUnitTest(path, filepath.ToSlash(rel)))
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipDir) {
		return nil, fmt.Errorf("while reading %s: %w", UnitTestsDir, err)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	return records, nil
}

func readUnitTest(path, rel string) UnitTestRecord {
	record := UnitTestRecord{Path: rel}
	td, findings, err := rulestest.LoadTestDefinition(path)
	if err != nil {
		record.Findings = []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "unreadable",
			Message:  fmt.Sprintf("the test file cannot be read: %v", err),
		}}
		return record
	}
	if td != nil {
		record.TestName = td.TestName
		record.ProcessName = td.JetrulesConfig.ProcessName
		record.LegacyMode = td.LegacyMode()
		record.Comment = td.Comment
	} else {
		// The definition did not decode (an unknown key, or not JSON of the right
		// shape), so its name is read loosely, for the table to show something
		// better than a path. Nothing else is taken from a refused file.
		record.TestName, record.ProcessName = looseNames(path)
	}
	if rulestest.HasErrors(findings) {
		record.Findings = wsvalidate.ErrorsOnly(findings)
	}
	return record
}

func looseNames(path string) (testName, processName string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var loose struct {
		TestName       any `json:"test_name"`
		JetrulesConfig struct {
			ProcessName any `json:"process_name"`
		} `json:"jetrules_config"`
	}
	if json.Unmarshal(content, &loose) != nil {
		return "", ""
	}
	testName, _ = loose.TestName.(string)
	processName, _ = loose.JetrulesConfig.ProcessName.(string)
	return testName, processName
}

// SaveUnitTests records the workspace's rules unit tests in workspace.db's
// unit_tests table, replacing what a previous compile wrote. It returns how
// many were recorded and how many of those carry findings.
func SaveUnitTests(ctx context.Context, db *sql.DB, workspacePath string) (recorded, invalid int, err error) {
	records, err := ReadUnitTests(workspacePath)
	if err != nil {
		return 0, 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM unit_tests"); err != nil {
		return 0, 0, fmt.Errorf("while clearing unit_tests: %w", err)
	}
	for _, r := range records {
		var findings any // NULL when the test may be started
		if len(r.Findings) > 0 {
			b, err := json.Marshal(r.Findings)
			if err != nil {
				return 0, 0, err
			}
			findings = string(b)
			invalid++
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO unit_tests
			(path, test_name, process_name, legacy_mode, comment, findings) VALUES (?, ?, ?, ?, ?, ?)`,
			r.Path, r.TestName, r.ProcessName, nullIfEmpty(r.LegacyMode), r.Comment, findings)
		if err != nil {
			return 0, 0, fmt.Errorf("while recording %s: %w", r.Path, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	return len(records), invalid, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
