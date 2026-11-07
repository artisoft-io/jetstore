package datatable

// Reading a workspace's runbooks from disk. Project runbooks, Phase 1, track AB.
//
// **The files are read from the active workspace's runbooks/ folder**, under
// WORKSPACES_HOME, which is where the IDE's save path writes them. Nothing is
// recorded at compile (plan §1): a run reads and validates the files when it
// starts and whenever a person resumes it, and pins what it read in the run row.
//
// **Every path is checked before it is opened.** A runbook names its SQL files
// itself, and it is read before it is validated, so a path the validator would
// refuse must not reach the file system first: a name is checked against the
// format's pattern, a SQL path must be relative, clean, end in .sql and resolve
// (symbolic links followed) under runbooks/.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// runbookFileCap is the largest runbook or SQL file the executor reads.
const runbookFileCap = 1 << 20

var runbookNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// runbooksDir is the active workspace's runbooks folder.
func runbooksDir(workspaceName string) (string, error) {
	home := strings.TrimSpace(os.Getenv("WORKSPACES_HOME"))
	if home == "" {
		return "", rbErr(http.StatusInternalServerError, "WORKSPACES_HOME is not set")
	}
	return filepath.Join(home, workspaceName, runbook.Dir), nil
}

// sha256Of is a file's hash as a revision records it.
func sha256Of(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// stepDefHash is the hash of a step's definition as written: what an edit at a
// pause may not change for a step that has run. A repeat's covers its nested
// steps.
func stepDefHash(s *runbook.Step) string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return sha256Of(string(b))
}

// readCapped reads a regular file of at most runbookFileCap bytes that
// resolves under root.
func readCapped(root, rel string) (string, error) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(real, realRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("%s resolves outside %s", rel, runbook.Dir)
	}
	f, err := os.Open(real)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", rel)
	}
	b, err := io.ReadAll(io.LimitReader(f, runbookFileCap+1))
	if err != nil {
		return "", err
	}
	if len(b) > runbookFileCap {
		return "", fmt.Errorf("%s is larger than %d bytes", rel, runbookFileCap)
	}
	return string(b), nil
}

// safeSQLPath reports whether a SQL path a runbook names may be opened.
func safeSQLPath(p string) bool {
	return p != "" && !strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") &&
		path.Clean(p) == p && !strings.HasPrefix(p, "../") && p != ".." &&
		strings.HasSuffix(p, ".sql")
}

// loadedRunbook is a runbook read from disk or from a run's pinned revision.
type loadedRunbook struct {
	Name     string
	Runbook  *runbook.Runbook
	Findings []wsvalidate.Finding
	// Files maps each file, relative to runbooks/, to its content: the
	// .rb.json and every SQL file it reads that exists.
	Files map[string]string
}

// Valid reports whether the runbook has no error finding.
func (l *loadedRunbook) Valid() bool {
	if l.Runbook == nil {
		return false
	}
	for _, f := range l.Findings {
		if f.Severity == wsvalidate.Error {
			return false
		}
	}
	return true
}

// Hashes are the files' hashes, as a revision records them.
func (l *loadedRunbook) Hashes() map[string]string {
	out := make(map[string]string, len(l.Files))
	for p, c := range l.Files {
		out[p] = sha256Of(c)
	}
	return out
}

// errorSummary is the findings that refuse a run, in one sentence.
func (l *loadedRunbook) errorSummary() string {
	var msgs []string
	for _, f := range l.Findings {
		if f.Severity == wsvalidate.Error {
			m := f.Message
			if f.Path != "" {
				m = f.Path + ": " + m
			}
			msgs = append(msgs, m)
		}
	}
	if len(msgs) > 5 {
		msgs = append(msgs[:5], fmt.Sprintf("and %d more", len(msgs)-5))
	}
	return strings.Join(msgs, "; ")
}

// refusal is the error a start or a resume answers for an invalid runbook.
func (l *loadedRunbook) refusal() *RunbookError {
	return &RunbookError{
		Status:   http.StatusConflict,
		Message:  fmt.Sprintf("runbook %s does not validate: %s", l.Name, l.errorSummary()),
		Findings: l.Findings,
	}
}

// loadRunbookFromDisk reads and validates runbooks/<name>.rb.json and its SQL
// files from the workspace.
func loadRunbookFromDisk(workspaceName, name string) (*loadedRunbook, error) {
	if !runbookNamePattern.MatchString(name) {
		return nil, rbErr(http.StatusBadRequest, "%q is not a runbook name", name)
	}
	dir, err := runbooksDir(workspaceName)
	if err != nil {
		return nil, err
	}
	fileName := name + runbook.FileSuffix
	content, err := readCapped(dir, fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, rbErr(http.StatusNotFound, "workspace %s has no runbook %s", workspaceName, name)
		}
		return nil, rbErr(http.StatusBadRequest, "while reading runbook %s: %v", name, err)
	}
	files := map[string]string{fileName: content}
	rb, _ := runbook.Decode([]byte(content))
	if rb != nil {
		for _, p := range rb.SQLPaths() {
			if !safeSQLPath(p) {
				continue
			}
			if text, err := readCapped(dir, p); err == nil {
				files[p] = text
			}
		}
	}
	return loadRunbookFiles(name, files), nil
}

// loadRunbookFiles validates a runbook from its files, as Load does at a run's
// start: from disk, or from a run's pinned revision.
func loadRunbookFiles(name string, files map[string]string) *loadedRunbook {
	fileName := name + runbook.FileSuffix
	sqlFiles := map[string]string{}
	for p, c := range files {
		if p != fileName {
			sqlFiles[p] = c
		}
	}
	rb, findings := runbook.Load(fileName, []byte(files[fileName]), sqlFiles)
	if findings == nil {
		findings = []wsvalidate.Finding{}
	}
	return &loadedRunbook{Name: name, Runbook: rb, Findings: findings, Files: files}
}

// listRunbookNames lists the runbooks of a workspace: the names of
// runbooks/*.rb.json, sorted.
func listRunbookNames(workspaceName string) ([]string, error) {
	dir, err := runbooksDir(workspaceName)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("while listing %s: %v", dir, err)
	}
	names := []string{}
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, runbook.FileSuffix) && !e.IsDir() {
			names = append(names, strings.TrimSuffix(n, runbook.FileSuffix))
		}
	}
	sort.Strings(names)
	return names, nil
}
