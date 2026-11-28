package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The commit leaves out the paths it is told to (JM-032): the overrides of
// JetStore-owned assets stay in the tree, unstaged, and every other change is
// staged as `git add -A` stages it. A path with glob characters names itself
// only.
func TestStageChangesLeavesOutThePathsKeptOut(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	wg := newTestWorkspace(t)
	dir := filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("user_flows/flow.uf.json", "installed")
	write("user_flows/[x].uf.json", "installed")
	write("user_flows/x.uf.json", "installed")
	write("jet_rules/a.jr", "rule")
	git("add", "-A")
	git("commit", "-q", "-m", "initial")

	write("user_flows/flow.uf.json", "overridden")
	write("user_flows/[x].uf.json", "overridden")
	write("user_flows/x.uf.json", "edited")
	write("jet_rules/a.jr", "rule edited")
	write("jet_rules/b.jr", "new rule")

	var buf strings.Builder
	if err := wg.stageChanges(&buf, dir, []string{"user_flows/flow.uf.json", "user_flows/[x].uf.json"}); err != nil {
		t.Fatalf("stageChanges: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "Left out of the commit, kept as overrides on this installation: "+
		"user_flows/flow.uf.json, user_flows/[x].uf.json") {
		t.Errorf("the log does not name the files left out:\n%s", buf.String())
	}
	got := git("status", "--porcelain")
	for _, want := range []string{
		"M  jet_rules/a.jr",
		"A  jet_rules/b.jr",
		"M  user_flows/x.uf.json",
		" M user_flows/flow.uf.json",
		" M \"user_flows/[x].uf.json\"",
	} {
		if !strings.Contains(got, want) && !strings.Contains(got, strings.ReplaceAll(want, "\"", "")) {
			t.Errorf("git status has no %q:\n%s", want, got)
		}
	}

	// Nothing kept out: `git add -A` as before.
	buf.Reset()
	if err := wg.stageChanges(&buf, dir, nil); err != nil {
		t.Fatal(err)
	}
	if got := git("status", "--porcelain"); strings.Contains(got, " M ") {
		t.Errorf("a change was left unstaged:\n%s", got)
	}
}
