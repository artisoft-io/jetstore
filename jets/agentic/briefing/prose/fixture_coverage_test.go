package prose_test

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// **I-679's residual, closed by a tripwire rather than by sharing the
// fixtures.**
//
// Criterion 82 is checked in `jets/agentic/briefing/briefingtmpl`, against a
// **transcription** of this package's render cases: `section1106Entity` and
// `TestRenderCases`' table are unexported helpers of this test binary, so no
// other package can call them. `I-679` accepted that and named what it cannot
// catch - *a fixture `prose_test.go` gains later*, which would be a case the
// equivalence test has never seen and nothing would say so.
//
// **Exporting the fixtures was the repair `I-679` proposed and it is not the
// one taken.** It would put test data in a non-test file of a shipped package,
// and it would only pay once the equivalence test called it - which is an edit
// to `briefingtmpl`, a package `AY.5` was told not to touch. A tripwire needs
// neither: it fails here, in the package that grew the case, at the moment the
// case is added.
//
// # What it checks and what it does not
//
// It compares the `name:` literals of the two files as sets. A case added to
// `TestRenderCases` and not transcribed fails; a case added to `briefingtmpl`
// alone does not, deliberately, because that direction is a document the two
// arms are both rendered through and is that file's business.
//
// **A whole new `func Test...` here is not covered**, and no mechanism would
// be: the two files name their tests differently on purpose -
// `TestAPharmacyEventWithoutDrugNameIsRefused` against
// `TestTheTwoRefusalsThatAreTemplateAssertions` - because one asserts a refusal
// and the other asserts that the refusal became two `require`s. That is a
// reader's judgement and the residual stays a residual.

var reCaseName = regexp.MustCompile(`(?m)^\s*name:\s*"([^"]*)"`)

func caseNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v - if the file moved, the transcription it guards moved with it "+
			"and this test is the thing that should be repaired rather than removed: %v", path, err, err)
	}
	out := map[string]bool{}
	for _, m := range reCaseName.FindAllStringSubmatch(string(src), -1) {
		out[m[1]] = true
	}
	return out
}

func TestEveryRenderCaseIsAlsoAnEquivalenceCase(t *testing.T) {
	mine := caseNames(t, "prose_test.go")
	theirs := caseNames(t, "../briefingtmpl/equivalence_test.go")

	if len(mine) == 0 {
		t.Fatal("no render cases found here, so this test is watching nothing; the table's `name:` " +
			"field is what it reads")
	}

	var missing []string
	for name := range mine {
		if !theirs[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("criterion 82 does not see %d of this package's %d render cases, because the equivalence "+
			"test carries a transcription of them (I-679): %s\n\nAdd each one to TestRenderCases in "+
			"jets/agentic/briefing/briefingtmpl/equivalence_test.go, where it is rendered through both "+
			"arms and compared.", len(missing), len(mine), strings.Join(missing, ", "))
	}
}
