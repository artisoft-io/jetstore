package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withEnv points the package's workspace globals at a temp layout and restores
// them, since they are set from the environment in init() and several tests
// share the package.
func withEnv(t *testing.T, home, prefix, repo string) {
	t.Helper()
	oldHome, oldPrefix, oldVersion := workspaceHome, wprefix, workspaceVersion
	workspaceHome, wprefix, workspaceVersion = home, prefix, "1787999999"
	t.Setenv("WORKSPACES_REPO", repo)
	t.Cleanup(func() {
		workspaceHome, wprefix, workspaceVersion = oldHome, oldPrefix, oldVersion
	})
}

func seedRepo(t *testing.T, repo, prefix string) {
	t.Helper()
	ws := filepath.Join(repo, prefix)
	if err := os.MkdirAll(filepath.Join(ws, "build"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "build", "classes.json"), []byte(`{"classes":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "lookup.db"), []byte("sqlite"), 0644); err != nil {
		t.Fatal(err)
	}
}

// The case this exists for: an image carries its workspace at WORKSPACES_REPO,
// nothing has copied it to WORKSPACES_HOME, and the version check has just
// decided not to fetch. Without the seed the next read finds an empty directory.
func TestSeedsFromTheImageWhenHomeIsEmpty(t *testing.T) {
	dir := t.TempDir()
	repo, home := filepath.Join(dir, "workspaces"), filepath.Join(dir, "tmp", "workspaces")
	seedRepo(t, repo, "jets_ws")
	withEnv(t, home, "jets_ws", repo)

	if err := ensureLocalRepoSeeded(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(home, "jets_ws", "build", "classes.json"))
	if err != nil {
		t.Fatalf("the file jetrules_utils.go reads was not seeded: %v", err)
	}
	if string(got) != `{"classes":[]}` {
		t.Errorf("seeded content is %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, "jets_ws", "lookup.db")); err != nil {
		t.Errorf("lookup.db was not seeded: %v", err)
	}
}

// Every container already has cbooter's copy, so the seed must not run a second
// one over it - and must not fail on the files being there.
func TestDoesNothingWhenTheWorkspaceIsAlreadyThere(t *testing.T) {
	dir := t.TempDir()
	repo, home := filepath.Join(dir, "workspaces"), filepath.Join(dir, "jetsdata", "workspaces")
	seedRepo(t, repo, "jets_ws")
	seedRepo(t, home, "jets_ws")
	mine := filepath.Join(home, "jets_ws", "build", "classes.json")
	if err := os.WriteFile(mine, []byte(`{"classes":["local"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	withEnv(t, home, "jets_ws", repo)

	if err := ensureLocalRepoSeeded(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(mine)
	if string(got) != `{"classes":["local"]}` {
		t.Errorf("an existing workspace was overwritten: %q", got)
	}
}

// The zip lambdas carry no image and so no WORKSPACES_REPO. They must reach the
// same code and do nothing at all.
func TestDoesNothingWithoutAnImageWorkspace(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "tmp", "workspaces")
	withEnv(t, home, "jets_ws", "")

	if err := ensureLocalRepoSeeded(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Error("something was created for a runtime that carries no workspace")
	}
}

// A container whose image puts the workspace where the code already reads it.
func TestDoesNothingWhenRepoAndHomeAreTheSame(t *testing.T) {
	dir := t.TempDir()
	seedRepo(t, dir, "jets_ws")
	withEnv(t, dir, "jets_ws", dir)

	if err := ensureLocalRepoSeeded(); err != nil {
		t.Fatal(err)
	}
}

// The caller has just declined to fetch on the strength of the image's version.
// If the image then has no workspace, that is worth an error naming both facts,
// not a silent empty directory that fails somewhere else.
func TestFailsLoudlyWhenTheImageClaimsAVersionAndCarriesNoWorkspace(t *testing.T) {
	dir := t.TempDir()
	repo, home := filepath.Join(dir, "workspaces"), filepath.Join(dir, "tmp", "workspaces")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	withEnv(t, home, "jets_ws", repo)

	err := ensureLocalRepoSeeded()
	if err == nil {
		t.Fatal("a missing image workspace was not reported")
	}
	for _, want := range []string{"1787999999", "jets_ws"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
}
