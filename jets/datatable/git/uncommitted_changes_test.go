package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestUncommittedChanges is the check Push Only reports with: after the
// database's overrides are written over the tree, a file that differs from the
// last commit is a change the push does not send.
func TestUncommittedChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// Git on, whatever an earlier test left the switch's sync.Once holding.
	t.Setenv(NoGitAccessEnvVar, "")
	resetNoGitAccessForTest()
	t.Cleanup(resetNoGitAccessForTest)
	wg := newTestWorkspace(t)
	dir := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	lookup := filepath.Join(dir, "lookups", "drug_class.csv")
	if err := os.MkdirAll(filepath.Dir(lookup), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lookup, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	run("add", "-A")
	run("commit", "-q", "-m", "initial")

	if changes, err := wg.UncommittedChanges(); err != nil || changes != "" {
		t.Fatalf("clean tree: got %q, %v; want no change", changes, err)
	}
	// An override written over the tree, as a lookup refresh's is.
	if err := os.WriteFile(lookup, []byte("a,b\n1,2\n3,4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := wg.UncommittedChanges()
	if err != nil {
		t.Fatalf("UncommittedChanges: %v", err)
	}
	if !strings.Contains(changes, "lookups/drug_class.csv") {
		t.Errorf("got %q; want the modified lookup listed", changes)
	}
}
