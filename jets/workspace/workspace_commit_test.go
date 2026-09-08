package workspace

import (
	"os"
	"testing"
)

// The subject is the validation between an image build argument and
// workspace_version.workspace_commit, not UpdateWorkspaceVersionDb itself -- that
// one needs a pool and a database, and the part of it worth pinning is the part
// that decides whether prose reaches the column.
//
// **Why this is a test rather than a reading of the regular expression it
// replaces.** The rule is stated twice in the tree, here and as `shaPattern` in
// jets/datatable/git, and the two are only useful while they agree. A test that
// names the cases -- short, long, uppercase, spaced, empty, prose -- is what makes
// a later edit to either one visible, and the cases below are the values an
// environment variable actually turns up carrying: unset, a tag, a short sha, and
// the output of a command that appended a newline.

func TestIsWorkspaceCommitSha(t *testing.T) {
	full := "9f2c1a4b7d8e0f1234567890abcdef0123456789"
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"full lowercase sha", full, true},
		{"all digits", "0123456789012345678901234567890123456789", true},
		{"all hex letters", "abcdefabcdefabcdefabcdefabcdefabcdefabcd", true},
		{"empty", "", false},
		{"short sha", full[:12], false},
		{"one character short", full[:39], false},
		{"one character long", full + "0", false},
		{"uppercase", "9F2C1A4B7D8E0F1234567890ABCDEF0123456789", false},
		{"non hex letter", "9g2c1a4b7d8e0f1234567890abcdef0123456789", false},
		{"leading space", " " + full[1:], false},
		{"prose", "unknown", false},
		{"branch name", "refs/heads/jets_ai", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isWorkspaceCommitSha(c.in); got != c.want {
				t.Errorf("isWorkspaceCommitSha(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestWorkspaceCommitFromBuildEnv(t *testing.T) {
	full := "9f2c1a4b7d8e0f1234567890abcdef0123456789"
	cases := []struct {
		name        string
		set         bool
		value       string
		wantSha     string
		wantIgnored string
	}{
		// Unset is the ordinary case in a deployment nobody has configured for
		// this, and it is reported as neither a sha nor a complaint.
		{"unset", false, "", "", ""},
		{"empty", true, "", "", ""},
		{"whitespace only", true, "  \n", "", ""},
		{"sha", true, full, full, ""},
		// A build script writing the sha to a file and reading it back is where the
		// newline comes from; the value is the same commit either way.
		{"sha with trailing newline", true, full + "\n", full, ""},
		{"sha with surrounding spaces", true, "  " + full + "  ", full, ""},
		// The raw value comes back so the caller can say which mistake was made
		// rather than only that the column is null.
		{"prose", true, "unknown", "", "unknown"},
		{"short sha", true, full[:12], "", full[:12]},
		{"uppercase sha", true, "9F2C1A4B7D8E0F1234567890ABCDEF0123456789", "", "9F2C1A4B7D8E0F1234567890ABCDEF0123456789"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv(workspaceGitShaEnvVar, c.value)
			} else {
				// t.Setenv is what restores the variable afterwards, and there is
				// no t.Unsetenv, so the variable is registered for restoration and
				// then removed -- "absent" and "present and empty" are different
				// inputs and both are exercised here.
				t.Setenv(workspaceGitShaEnvVar, "")
				if err := os.Unsetenv(workspaceGitShaEnvVar); err != nil {
					t.Fatal(err)
				}
			}
			sha, ignored := workspaceCommitFromBuildEnv()
			if sha != c.wantSha || ignored != c.wantIgnored {
				t.Errorf("workspaceCommitFromBuildEnv() = (%q, %q), want (%q, %q)",
					sha, ignored, c.wantSha, c.wantIgnored)
			}
		})
	}
}
