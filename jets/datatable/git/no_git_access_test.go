package git

import (
	"strings"
	"testing"
)

// TestNoGitAccessVocabulary pins the values that turn git off and, more
// importantly, the ones that must not.
//
// **The empty string is the case this test exists for.** build_ui_service.go
// sets every task-definition variable with jsii.String(os.Getenv(...)) and
// filters nothing, so on the day JETS_NO_GIT_ACCESS joins that map every
// existing deployment starts receiving it set to "". If this switch were read
// with os.LookupEnv, or with a plain != "" test, git would turn off in every one
// of them at once and nothing would say so. That failure is site-wide, silent,
// and would be found in production by an operator who never asked for any of
// this -- so the empty string is asserted here rather than left to follow from
// the vocabulary.
func TestNoGitAccessVocabulary(t *testing.T) {
	cases := []struct {
		value string
		off   bool
	}{
		// Off: the whole truthy vocabulary, and the case and space that a
		// hand-edited environment file produces.
		{"1", true}, {"true", true}, {"yes", true}, {"on", true},
		{"TRUE", true}, {"True", true}, {"  1  ", true}, {" on\n", true},

		// On: the CDK's unset-variable value first, then the values a reader
		// might expect to work and which deliberately do not.
		{"", false},
		{"0", false}, {"false", false}, {"no", false}, {"off", false},
		{"enabled", false}, {"disabled", false}, {"ture", false}, {"y", false},
		{"2", false}, {" ", false},
	}
	for _, c := range cases {
		t.Setenv(NoGitAccessEnvVar, c.value)
		resetNoGitAccessForTest()
		if got := NoGitAccess(); got != c.off {
			t.Errorf("%s=%q: NoGitAccess() = %v, want %v", NoGitAccessEnvVar, c.value, got, c.off)
		}
	}
}

// TestNoGitAccessUnset covers the state every deployment is in today: the
// variable absent entirely, which must leave git on.
func TestNoGitAccessUnset(t *testing.T) {
	resetNoGitAccessForTest()
	if NoGitAccess() {
		t.Fatal("git must be on when the variable is unset")
	}
}

// TestNoGitAccessReadOnce asserts the value is not re-read per call, so two
// callers a second apart cannot disagree.
func TestNoGitAccessReadOnce(t *testing.T) {
	t.Setenv(NoGitAccessEnvVar, "1")
	resetNoGitAccessForTest()
	if !NoGitAccess() {
		t.Fatal("expected git off")
	}
	t.Setenv(NoGitAccessEnvVar, "")
	if !NoGitAccess() {
		t.Error("the value must be read once at startup, not per call")
	}
}

// TestNoGitAccessNoticeNamesTheVariable guards the one property the notice has
// beyond being a string: a reader of a last_git_log in a deployment they did not
// configure has to be able to tell which setting produced the silence.
func TestNoGitAccessNoticeNamesTheVariable(t *testing.T) {
	if !strings.Contains(NoGitAccessNotice, NoGitAccessEnvVar) {
		t.Errorf("the notice must name %s; got %q", NoGitAccessEnvVar, NoGitAccessNotice)
	}
}
