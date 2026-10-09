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

// The exclusions, and the three properties that make them safe.
//
// The seed stands in for a fetch that would have delivered `workspace.tgz` and
// `sqlite` — neither of which carries `.git` or `lookups/`. Trimming those is
// bringing the seed back to the contract, not guessing at what is read; see
// copyWorkspaceExcept.
func TestSeedLeavesOutWhatTheCallerNames(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	withEnv(t, home, "jets_ws", repo)
	ws := filepath.Join(repo, "jets_ws")
	seedRepo(t, repo, "jets_ws")
	// The two large entries the compute-pipes fetch never delivers...
	mk := func(rel string, perm os.FileMode) {
		t.Helper()
		p := filepath.Join(ws, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), perm); err != nil {
			t.Fatal(err)
		}
	}
	mk(".git/HEAD", 0644)
	mk("lookups/codes.csv", 0644)
	// ...an executable, because 16 of the workspace's files are...
	mk("scripts/run.sh", 0755)
	// ...and a nested `lookups/`, which must NOT be skipped: the caller named a
	// top-level entry, and silently matching it at any depth would be a surprise.
	mk("data_model/lookups/keep.csv", 0644)

	if err := ensureLocalRepoSeeded(".git", "lookups"); err != nil {
		t.Fatalf("ensureLocalRepoSeeded: %v", err)
	}

	dest := filepath.Join(home, "jets_ws")
	for _, gone := range []string{".git", "lookups"} {
		if _, err := os.Stat(filepath.Join(dest, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should not have been copied, got err=%v", gone, err)
		}
	}
	for _, kept := range []string{
		"build/classes.json", "lookup.db", "scripts/run.sh", "data_model/lookups/keep.csv",
	} {
		if _, err := os.Stat(filepath.Join(dest, kept)); err != nil {
			t.Errorf("%s should have been copied: %v", kept, err)
		}
	}
	// The execute bit survives, which os.CopyFS also preserved and a naive
	// rewrite would drop.
	fi, err := os.Stat(filepath.Join(dest, "scripts/run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("scripts/run.sh lost its execute bit: %#o", fi.Mode().Perm())
	}
	// And every directory is traversable by any identity, which is the property
	// the whole seed exists to preserve — see jets/workspace/README.md.
	if err := filepath.WalkDir(dest, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return iErr
		}
		if info.Mode().Perm()&0o005 != 0o005 {
			t.Errorf("%s is not world-traversable: %#o", p, info.Mode().Perm())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// With no exclusions it copies everything, which is what run_reports gets: it
// reads lookups/ and would break if this function had the list baked in.
func TestSeedCopiesEverythingWhenNothingIsNamed(t *testing.T) {
	home, repo := t.TempDir(), t.TempDir()
	withEnv(t, home, "jets_ws", repo)
	seedRepo(t, repo, "jets_ws")
	if err := os.MkdirAll(filepath.Join(repo, "jets_ws", "lookups"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "jets_ws", "lookups", "codes.csv"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureLocalRepoSeeded(); err != nil {
		t.Fatalf("ensureLocalRepoSeeded: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "jets_ws", "lookups", "codes.csv")); err != nil {
		t.Errorf("lookups/ must be copied when the caller names no exclusions: %v", err)
	}
}
