package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
)

// The tests in this file grade criterion 58 -- **no git process is started when
// git is off** -- and the git-on half of criterion 62.
//
// # Why the return value is not the assertion
//
// These operations are no-ops that report success, so a test comparing only the
// returned string would pass unchanged if the implementation still shelled out to
// git and then threw the result away. That is precisely the failure the
// requirement's word *silently* rules out, and it is invisible from the outside:
// the button returns 200 either way, and the only trace is a process that touched
// a repository nobody offered.
//
// So the assertion is made on the process, and it is made twice over. `git` is
// replaced on PATH by a recording stub, and PATH holds nothing else -- so a call
// that reaches exec.Command either leaves a marker file behind (the stub ran) or
// fails to resolve at all (nothing else named git is reachable). The first says
// what happened; the second is what makes the first exhaustive.

// gitOutOfReach puts a recording stub named "git" on an otherwise empty PATH and
// returns a predicate reporting whether anything ran it.
//
// The stub exits 0 rather than failing, on purpose. A stub that failed would make
// every git-off assertion below pass for two different reasons -- the operation
// short-circuited, or it ran git and swallowed the error -- and the second is the
// bug being hunted. Exiting 0 keeps the two apart: if the short-circuit were
// removed, the marker file appears and the test names the process rather than the
// string.
func gitOutOfReach(t *testing.T) func() bool {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "git-was-executed")
	stub := "#!/bin/sh\necho \"$@\" >> " + marker + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(stub), 0o755); err != nil {
		t.Fatalf("while writing the git stub: %v", err)
	}
	// PATH is read by exec.Command at call time, so this covers any git command
	// reached from this goroutine for the duration of the test.
	t.Setenv("PATH", dir)
	return func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}
}

// gitOff turns the switch on for the duration of one test and re-arms the
// sync.Once so the next test reads its own value.
func gitOff(t *testing.T) {
	t.Helper()
	t.Setenv(NoGitAccessEnvVar, "1")
	resetNoGitAccessForTest()
	t.Cleanup(resetNoGitAccessForTest)
	if !NoGitAccess() {
		t.Fatal("setup failed: expected git to be off")
	}
}

// newTestWorkspace creates a workspaces home with one workspace directory in it
// and returns a WorkspaceGit addressing that workspace as the active one.
//
// The directory is real because GetStatus tests for it before it tests anything
// else, and a missing directory answers 'local workspace removed' whatever the
// switch says.
func newTestWorkspace(t *testing.T) *WorkspaceGit {
	t.Helper()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "testws"), 0o755); err != nil {
		t.Fatalf("while creating the workspace directory: %v", err)
	}
	return &WorkspaceGit{
		WorkspaceName:         "testws",
		WorkspaceUri:          "https://github.com/artisoft-io/testws",
		WorkspaceBranch:       "main",
		FeatureBranch:         "feature",
		WorkspacesHome:        home,
		ActiveWorkspace:       "testws",
		ActiveWorkspaceBranch: "main",
	}
}

// TestGitOffStartsNoGitProcess is criterion 58. It calls every operation the
// switch covers and asserts, once at the end, that nothing on PATH named git was
// executed by any of them.
func TestGitOffStartsNoGitProcess(t *testing.T) {
	ranGit := gitOutOfReach(t)
	gitOff(t)
	wg := newTestWorkspace(t)

	if _, err := wg.GetStatus(); err != nil {
		t.Errorf("GetStatus: %v", err)
	}
	if _, err := wg.UpdateLocalWorkspace("u", "u@example.com", "gu", "gt", ""); err != nil {
		t.Errorf("UpdateLocalWorkspace: %v", err)
	}
	profile := &user.GitProfile{Name: "u", Email: "u@example.com", GitHandle: "gu", GitToken: "gt"}
	if _, err := wg.CommitLocalWorkspace(profile, "a message"); err != nil {
		t.Errorf("CommitLocalWorkspace: %v", err)
	}
	if _, err := wg.PushOnlyWorkspace("gu", "gt"); err != nil {
		t.Errorf("PushOnlyWorkspace: %v", err)
	}
	if _, err := wg.GitCommandWorkspace("git status\ngit log -1"); err != nil {
		t.Errorf("GitCommandWorkspace: %v", err)
	}
	if _, err := wg.PullRemoteWorkspace("gu", "gt"); err != nil {
		t.Errorf("PullRemoteWorkspace: %v", err)
	}
	// HeadCommit reports an error by design -- the commit is not known -- so what
	// is graded here is that it did not ask git.
	if _, err := HeadCommit(wg.WorkspacesHome, wg.WorkspaceName); err == nil {
		t.Error("HeadCommit: expected an error saying the commit is not known")
	}

	if ranGit() {
		t.Error("a git process was started with git off; criterion 58 is the whole point of this test")
	}
}

// TestGitOffOperationsReturnTheNotice pins what the five pure-git operations
// return. The handler writes this string into workspace_registry.last_git_log and
// the *View Last Log* dialog displays it, so the exact text is the interface.
func TestGitOffOperationsReturnTheNotice(t *testing.T) {
	gitOutOfReach(t)
	gitOff(t)
	wg := newTestWorkspace(t)
	profile := &user.GitProfile{Name: "u", Email: "u@example.com", GitHandle: "gu", GitToken: "gt"}

	cases := []struct {
		name string
		call func() (string, error)
	}{
		{"UpdateLocalWorkspace", func() (string, error) {
			return wg.UpdateLocalWorkspace("u", "u@example.com", "gu", "gt", "")
		}},
		{"CommitLocalWorkspace", func() (string, error) {
			return wg.CommitLocalWorkspace(profile, "a message")
		}},
		{"PushOnlyWorkspace", func() (string, error) { return wg.PushOnlyWorkspace("gu", "gt") }},
		{"GitCommandWorkspace", func() (string, error) { return wg.GitCommandWorkspace("git status") }},
		{"PullRemoteWorkspace", func() (string, error) { return wg.PullRemoteWorkspace("gu", "gt") }},
	}
	for _, c := range cases {
		got, err := c.call()
		if err != nil {
			t.Errorf("%s returned an error: %v -- a configured deployment mode is not a failure", c.name, err)
		}
		if got != NoGitAccessNotice {
			t.Errorf("%s = %q, want the notice %q", c.name, got, NoGitAccessNotice)
		}
	}
}

// TestGitOffOperationsSkipTheirOwnValidation covers the reason the switch is
// tested at the top of each function rather than after its guards.
//
// A workspace registered in a deployment with no repository carries no uri and no
// branches, which is exactly what validateGitRef and the uri handling refuse. If
// the check sat below them, the registry screen's buttons would fail for a row
// that is correctly configured for this mode.
func TestGitOffOperationsSkipTheirOwnValidation(t *testing.T) {
	gitOutOfReach(t)
	gitOff(t)
	wg := newTestWorkspace(t)
	wg.WorkspaceUri = ""
	wg.WorkspaceBranch = ""
	wg.FeatureBranch = ""

	if got, err := wg.UpdateLocalWorkspace("u", "u@example.com", "", "", ""); err != nil || got != NoGitAccessNotice {
		t.Errorf("UpdateLocalWorkspace with no branch = (%q, %v), want the notice and no error", got, err)
	}
	if got, err := wg.PullRemoteWorkspace("", ""); err != nil || got != NoGitAccessNotice {
		t.Errorf("PullRemoteWorkspace with no branch = (%q, %v), want the notice and no error", got, err)
	}
}

// TestGitOffStatusVocabulary is the guard the plan asks for at I-517: the status
// strings are coupled to the registry screen by substring
// (jetsclient_ide/src/datatable/tables/workspaceRegistryTable.tc.json), and until
// this test nothing on either side of that seam checked the pairing. This is it,
// stated as the button rules rather than as the strings, so that a future edit to
// the vocabulary fails against the reason rather than against a literal.
func TestGitOffStatusVocabulary(t *testing.T) {
	gitOutOfReach(t)
	gitOff(t)

	active := newTestWorkspace(t)
	other := newTestWorkspace(t)
	other.WorkspaceName = "otherws"
	other.ActiveWorkspace = "testws"
	if err := os.Mkdir(filepath.Join(other.WorkspacesHome, "otherws"), 0o755); err != nil {
		t.Fatalf("while creating the second workspace directory: %v", err)
	}

	activeStatus, err := active.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus on the active workspace: %v", err)
	}
	if activeStatus != "active, no git repository" {
		t.Errorf("active workspace status = %q, want %q", activeStatus, "active, no git repository")
	}
	otherStatus, err := other.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus on a non-active workspace: %v", err)
	}
	if otherStatus != "no git repository" {
		t.Errorf("non-active workspace status = %q, want %q", otherStatus, "no git repository")
	}

	// Delete is disabled when the status contains 'active'. The server refuses to
	// delete the active workspace anyway, so what is protected here is the
	// courtesy of not offering the button.
	if !strings.Contains(activeStatus, "active") {
		t.Errorf("%q must contain 'active' or the screen offers Delete on the running workspace", activeStatus)
	}
	if strings.Contains(otherStatus, "active") {
		t.Errorf("%q must not contain 'active' or Delete is greyed out on every workspace", otherStatus)
	}
	// Commit & Push is enabled only on 'modified', which only git status can
	// establish; the other four gate on 'in progress' and 'removed', neither of
	// which describes a workspace that is simply not under version control.
	for _, status := range []string{activeStatus, otherStatus} {
		for _, forbidden := range []string{"modified", "in progress", "removed"} {
			if strings.Contains(status, forbidden) {
				t.Errorf("status %q contains %q, which the registry screen gates a button on", status, forbidden)
			}
		}
	}
}

// TestGitOffStatusClearsFeatureBranch covers the field rather than the return
// value. DoWorkspaceReadAction writes FeatureBranch back into the result row, so
// a name left here is a branch name displayed for a deployment that has none.
func TestGitOffStatusClearsFeatureBranch(t *testing.T) {
	gitOutOfReach(t)
	gitOff(t)
	wg := newTestWorkspace(t)
	wg.FeatureBranch = "some-feature"

	if _, err := wg.GetStatus(); err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if wg.FeatureBranch != "" {
		t.Errorf("FeatureBranch = %q after GetStatus with git off, want empty", wg.FeatureBranch)
	}
}

// TestGitOffStatusReportsARemovedDirectoryFirst is the ordering assertion. A
// workspace whose directory has been deleted is removed whether or not this
// deployment has a repository, and four of the screen's buttons gate on that
// word; reporting it as 'no git repository' would re-enable them for a workspace
// with nothing behind it.
func TestGitOffStatusReportsARemovedDirectoryFirst(t *testing.T) {
	gitOutOfReach(t)
	gitOff(t)
	wg := newTestWorkspace(t)
	if err := os.Remove(filepath.Join(wg.WorkspacesHome, wg.WorkspaceName)); err != nil {
		t.Fatalf("while removing the workspace directory: %v", err)
	}

	status, err := wg.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status != "local workspace removed" {
		t.Errorf("status = %q, want %q: the directory test runs before the switch", status, "local workspace removed")
	}
}

// TestGitOffHeadCommitIsNotAlarming grades the wording rather than the
// short-circuit. UpdateWorkspaceVersionDb already treats an error here as
// "record a null commit" and prints it after a Notice prefix, so what a
// deployment configured without a repository sees on every compile is this
// sentence. It has to read as the expected state it is.
func TestGitOffHeadCommitIsNotAlarming(t *testing.T) {
	ranGit := gitOutOfReach(t)
	gitOff(t)
	wg := newTestWorkspace(t)

	sha, err := HeadCommit(wg.WorkspacesHome, wg.WorkspaceName)
	if err == nil {
		t.Fatal("expected an error: the commit is not known when git is off")
	}
	if sha != "" {
		t.Errorf("HeadCommit returned %q; nothing here can produce a sha", sha)
	}
	msg := err.Error()
	if !strings.Contains(msg, NoGitAccessEnvVar) {
		t.Errorf("the error must name %s so a reader knows which setting produced it; got %q",
			NoGitAccessEnvVar, msg)
	}
	for _, alarming := range []string{"error", "fail", "fatal"} {
		if strings.Contains(strings.ToLower(msg), alarming) {
			t.Errorf("the error reads as a fault (%q in %q); this is an expected state", alarming, msg)
		}
	}
	if ranGit() {
		t.Error("HeadCommit started a git process with git off")
	}
}

// TestGitOnStillErrorsOnANonRepository is the git-on half of criterion 62: with
// the variable absent and git on PATH, a workspace directory that is not a
// repository fails GetStatus exactly as it does today. Nothing about the switch
// is allowed to turn that into a status string.
func TestGitOnStillErrorsOnANonRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; this case grades today's behaviour with git on")
	}
	// testing has no t.Unsetenv, and *absent* is the state every deployment that
	// exists today is in -- so it is restored by hand rather than approximated
	// with the empty string, which no_git_access_test.go grades separately.
	previous, wasSet := os.LookupEnv(NoGitAccessEnvVar)
	os.Unsetenv(NoGitAccessEnvVar)
	resetNoGitAccessForTest()
	t.Cleanup(func() {
		if wasSet {
			os.Setenv(NoGitAccessEnvVar, previous)
		}
		resetNoGitAccessForTest()
	})
	if NoGitAccess() {
		t.Fatal("setup failed: expected git to be on with the variable unset")
	}

	wg := newTestWorkspace(t)
	// Stop git's upward search at the workspaces home, so the result does not
	// depend on whether TMPDIR happens to sit inside somebody's repository.
	t.Setenv("GIT_CEILING_DIRECTORIES", wg.WorkspacesHome)

	if status, err := wg.GetStatus(); err == nil {
		t.Errorf("GetStatus on a directory that is not a repository returned %q, want an error", status)
	}
}
