package shadow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
)

// The three checks in this file need no database, and that is the point: they
// are the half of criterion 47 that is a property of the code rather than of a
// deployment.

// TestActingStatusesAreTheUnreachableHalf recomputes the ceiling from Appendix
// A.5's own machine.
//
// **ActingStatuses is not allowed to be an opinion held in this package.** The
// claim it makes is that `remediation_proposed` is the articulation point of the
// incident lifecycle — that every status on the far side of it is reachable from
// `detected` only through it, and that none of triaged, diagnosed, reclassified
// or suppressed_as_benign is. That is checkable against audit.IncidentTransitions,
// so it is checked here rather than asserted in a comment, and an edge added to
// A.5 moves the list or fails this test.
func TestActingStatusesAreTheUnreachableHalf(t *testing.T) {
	reachable := map[string]bool{audit.IncidentDetected: true}
	queue := []string{audit.IncidentDetected}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, to := range audit.IncidentTransitions(from) {
			if to == ActingFrontier || reachable[to] {
				continue
			}
			reachable[to] = true
			queue = append(queue, to)
		}
	}
	var got []string
	for _, s := range audit.IncidentStatuses {
		if !reachable[s] {
			got = append(got, s)
		}
	}
	want := append([]string(nil), ActingStatuses...)
	sort.Strings(got)
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("the acting half of A.5's machine is %v; ActingStatuses says %v.\n"+
			"ActingStatuses must be exactly the statuses no path from %q reaches without passing "+
			"through %q — if A.5 gained or lost an edge, this list moves with it",
			got, want, audit.IncidentDetected, ActingFrontier)
	}

	// The other half of the same claim: everything this writer may produce is on
	// the reachable side, so the ceiling and the writer's vocabulary agree.
	for _, s := range ShadowStatuses {
		if !reachable[s] {
			t.Errorf("shadow status %q is on the acting side of the ceiling", s)
		}
		if IsActingStatus(s) {
			t.Errorf("shadow status %q is also declared acting", s)
		}
	}
	// And the three adjudications are refused for the *other* reason, so they
	// must be neither shadow statuses nor acting ones.
	for _, s := range AdjudicationStatuses {
		if IsShadowStatus(s) {
			t.Errorf("adjudication %q is writable by this wiring; a label the measured system wrote is not a label (plan §10.7)", s)
		}
	}
}

// TestNothingWritesTheRemediationTable asserts the absence of an executor over
// the source tree.
//
// **AB.2 tabled Remediation with no executor deliberately, so that Phase 5 has
// something to gate.** That makes the absence a property this phase is entitled
// to assert rather than a fact about what nobody got round to — and criterion
// 47's *no remediation executes* is exactly the claim it supports. The day
// somebody writes the executor, this test names the file.
//
// It looks for a write against the table and not for the table's name: the DDL
// creates it and the attestation counts it, and both are supposed to.
//
// **Test files are counted and reported rather than asserted on**, and the
// distinction is the honest one rather than a convenience. AB.2's
// TestRemediationRefusesAnIrreversibleAction has to insert a remediation to
// establish that §A.2.9's validator rule is enforced by the CHECK — a test that
// could not write the row could not test the refusal. So the claim this makes is
// **no production writer**, which is what criterion 47 needs, and the test-file
// count is logged so that a would-be executor hiding in a `_test.go` is visible
// in the output instead of exempt from it.
func TestNothingWritesTheRemediationTable(t *testing.T) {
	root := repoRoot(t)
	// Assembled rather than written out, so this file does not match itself.
	verbs := strings.Join([]string{"insert\\s+into", "update", "delete\\s+from", "copy"}, "|")
	write := regexp.MustCompile(`(?is)(` + verbs + `)\s+jetsapi\.` + "remediation")

	var offenders, inTests []string
	for _, dir := range []string{"jets", "tools"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".go", ".sql", ".py":
			default:
				return nil
			}
			if path == thisFile(t) {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !write.Match(b) {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_test.py") {
				inTests = append(inTests, rel)
				return nil
			}
			offenders = append(offenders, rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	t.Logf("writes to jetsapi.remediation: %d in production code, %d in test files %v",
		len(offenders), len(inTests), inTests)
	if len(offenders) > 0 {
		t.Errorf("production code writes jetsapi.remediation: %v.\n"+
			"AB.2 tabled the entity with no executor so that Phase 5 has something to gate, and "+
			"criterion 47 says in terms that nothing acts. If this is Phase 5's executor arriving, "+
			"this test is what has to be retired with an argument rather than deleted", offenders)
	}
}

// TestEvidenceSurvivesTheRoundTripByItsWireNames is marshalEvidence's negative
// control, and it asserts the failure as well as the fix.
//
// rca.Evidence carries no struct tags and audit.Evidence carries the column's:
// `statement`, `source`, `source_ref`. Marshalling the ranker's type directly
// produces `SourceRef`, which encoding/json cannot fold onto `source_ref` — so
// the reference into the record is dropped, in silence, by a path where every
// layer reports success. The second half of this test is that failure, asserted
// so that the conversion cannot be removed as redundant.
func TestEvidenceSurvivesTheRoundTripByItsWireNames(t *testing.T) {
	items := []rca.Evidence{{
		Statement: "worker 2 of session s1 reported failed",
		Source:    rca.SourceRunTelemetry,
		SourceRef: "session s1, locus worker_failed",
	}}

	raw, err := marshalEvidence(items)
	if err != nil {
		t.Fatalf("marshalEvidence: %v", err)
	}
	var back []audit.Evidence
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("decoding what the writer would store: %v", err)
	}
	if len(back) != 1 || back[0].SourceRef != items[0].SourceRef ||
		back[0].Statement != items[0].Statement || back[0].Source != items[0].Source {
		t.Fatalf("the evidence did not survive the round trip: wrote %s, read %+v", raw, back)
	}

	naive, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshalling the ranker's type directly: %v", err)
	}
	var lost []audit.Evidence
	if err := json.Unmarshal(naive, &lost); err != nil {
		t.Fatalf("decoding the naive form: %v", err)
	}
	if len(lost) == 1 && lost[0].SourceRef == items[0].SourceRef {
		t.Errorf("marshalling rca.Evidence directly now preserves source_ref (%s).\n"+
			"That would make marshalEvidence's conversion redundant — check whether rca.Evidence "+
			"gained struct tags, and if it did, say so where the conversion is documented rather "+
			"than deleting this test", naive)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "jets", "agentic", "shadow")); err != nil {
		t.Fatalf("%s does not look like the repository root: %v", root, err)
	}
	return root
}

func thisFile(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("shadow_test.go")
	if err != nil {
		t.Fatalf("resolving this file: %v", err)
	}
	return p
}
