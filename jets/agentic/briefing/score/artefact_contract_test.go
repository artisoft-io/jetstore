package score

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/briefingtmpl"
	"github.com/artisoft-io/jetstore/jets/agentic/briefing/prose"
)

// **R-111 made visible.** This package reads a rendered briefing and hard-codes
// properties of it; `AY.3` turned that briefing into a JSON document, so both
// properties are now values in a configuration file that can be changed without
// this directory being opened. The file this test watches is
// `jets/agentic/briefing/briefingtmpl/patient_profile_briefing.json`: the
// separator is `sep: '; '` and `last: '; and '` on the medications `each`, and
// the width is its root `"width": 78`.
//
// # The two properties are not equally load bearing, and saying so is the point
//
// **The separator is.** `parseProse` ends a clause at `;` (`parseProse`,
// `jets/agentic/briefing/score/text.go:107`) and attribution is clause bounded
// before it is windowed (`claimant`, `:298`). A medication clause of this
// briefing reads *"lisinopril, a maintenance medication, three fills, most
// recently 24 August"* - commas throughout - so a document that separated
// medications with a comma would put every drug of the list in one clause and a
// maintenance claim about one would reach the next. The scorer would keep
// returning numbers, and they would be wrong in the direction that matters: a
// false finding against the arm whose whole job is to score zero.
//
// **The width is not, and `R-111` prices it too high.** `unwrap` collapses a
// single newline to a space at any width and is a no-op on text that carries
// none (`unwrap`, `:89`), so a document that wrapped at 60, or stopped wrapping,
// would not move a single finding. What the width change would break is the
// sentence in `unwrap`'s own doc block, which says the briefing wraps at 78 and
// is the reason anybody believes this package needs it. So the width is
// asserted here as **documentation that can fail** rather than as a guard, and
// the failure message says which of the two the reader is looking at.
//
// # What is still uncovered
//
// This test renders `patient_profile`'s document. **A second template is a
// second artefact and this package has no contract with it at all** - which is
// `R-111`'s residual and is not closeable from here: a scorer written for one
// briefing cannot assert a property of a briefing that does not exist yet. The
// honest form of the remainder is that `score` is `patient_profile`'s scorer,
// and the entry that says so is `I-737`.

// contractEntity is three medications and one visit: three because `sep` and
// `last` are different strings in the document and two drugs exercise only
// `last`, and one visit because the medications element is what this test is
// about and the rest is there to keep the briefing shaped like one.
func contractEntity() map[string]any {
	dt := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	return map[string]any{
		"Briefing_Disclaimer":   "Informational only. This is not medical advice.",
		"Medical_Event_Count":   1,
		"Pharmacy_Event_Count":  3,
		"Earliest_Service_Date": dt(2025, 6, 10),
		"Latest_Service_Date":   dt(2025, 8, 14),
		"has_Briefing_Medical_Events": []any{
			map[string]any{
				"Care_Setting": "Independent Laboratory",
				"Service_Date": dt(2025, 8, 14),
			},
		},
		"has_Briefing_Pharmacy_Events": []any{
			map[string]any{
				"Drug_Name": "lisinopril", "Maintenance": "Y",
				"Fill_Count": 3, "Fill_Date": []any{dt(2025, 6, 15), dt(2025, 7, 20), dt(2025, 8, 24)},
			},
			map[string]any{
				"Drug_Name": "metformin", "Maintenance": "Y",
				"Fill_Count": 2, "Fill_Date": []any{dt(2025, 6, 2), dt(2025, 7, 30)},
			},
			map[string]any{
				"Drug_Name": "sertraline", "Maintenance": "N",
				"Fill_Count": 1, "Fill_Date": dt(2025, 7, 2),
			},
		},
	}
}

const (
	drug1, drug2, drug3 = "lisinopril", "metformin", "sertraline"
)

// renderBothArms renders the contract entity through the configured document
// and through `prose.Render`, so that the assertions below hold for whichever
// of the two produced the artefact a caller handed this package. **Q-100 is
// open**, so both are live and neither can be assumed away.
func renderBothArms(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for name, render := range map[string]func(map[string]any) (string, error){
		"briefingtmpl": briefingtmpl.Render,
		"prose":        prose.Render,
	} {
		s, err := render(contractEntity())
		if err != nil {
			t.Fatalf("%s.Render: %v", name, err)
		}
		out[name] = s
	}
	return out
}

// TestTheMedicationSeparatorIsAClauseBoundary is the load-bearing half of
// R-111. It asserts the property this package needs - one drug per clause -
// rather than the character that currently delivers it, and then asserts that
// the character is what delivers it, so a reader who breaks the test can see
// which of the two moved.
func TestTheMedicationSeparatorIsAClauseBoundary(t *testing.T) {
	for arm, rendered := range renderBothArms(t) {
		t.Run(arm, func(t *testing.T) {
			body := stripNotice(nil, rendered)

			clauses := clausesOfDrugs(t, body)
			if len(clauses) != 3 {
				t.Fatalf("expected the three drug names in the briefing, found %d: %s", len(clauses), body)
			}
			if clauses[drug1] == clauses[drug2] || clauses[drug2] == clauses[drug3] ||
				clauses[drug1] == clauses[drug3] {
				t.Errorf("two medications share a clause, so an attribution about one can reach the other; "+
					"the document's medications `each` no longer separates with `;`.\n%s", body)
			}

			// The same text with the semicolons replaced: what a document that
			// separated with a comma would produce. If this does NOT collapse
			// the list into one clause then `parseProse` has changed and the
			// paragraph at the head of this file is describing something else.
			commas := clausesOfDrugs(t, strings.ReplaceAll(body, "; ", ", "))
			if len(commas) != 3 || commas[drug1] != commas[drug2] || commas[drug2] != commas[drug3] {
				t.Errorf("replacing the semicolons no longer merges the medication list into one clause "+
					"(%v), so `;` is not what this package's attribution depends on and R-111 needs "+
					"rewriting rather than this test relaxing", commas)
			}
		})
	}
}

// clausesOfDrugs reports the clause index `parseProse` gives each drug name.
func clausesOfDrugs(t *testing.T, body string) map[string]int {
	t.Helper()
	p := parseProse(body)
	out := map[string]int{}
	for _, w := range p.words {
		switch w.text {
		case drug1, drug2, drug3:
			if _, seen := out[w.text]; !seen {
				out[w.text] = w.clause
			}
		}
	}
	return out
}

// TestTheArtefactIsStillWrappedAtTheDocumentedWidth is the other half of R-111
// and it is documentation rather than a guard - see the head of this file. It
// fails when `unwrap`'s doc block stops describing the artefact, which is worth
// a failing test and is not worth a reader believing the scorer broke.
func TestTheArtefactIsStillWrappedAtTheDocumentedWidth(t *testing.T) {
	for arm, rendered := range renderBothArms(t) {
		t.Run(arm, func(t *testing.T) {
			wrappedSomewhere := false
			for _, line := range strings.Split(rendered, "\n") {
				if len([]rune(line)) > prose.Width {
					t.Errorf("a line of %d runes exceeds the documented width of %d: %q",
						len([]rune(line)), prose.Width, line)
				}
			}
			// A line break inside a sentence is what `unwrap` exists for. Its
			// absence costs no finding; it means `unwrap` has become a no-op and
			// its doc block an account of an older artefact.
			for _, para := range strings.Split(rendered, "\n\n") {
				if strings.Contains(strings.TrimSpace(para), "\n") {
					wrappedSomewhere = true
				}
			}
			if !wrappedSomewhere {
				t.Errorf("no paragraph carries a line break, so the briefing is not wrapped and `unwrap` " +
					"no longer does anything; `unwrap`'s doc block (text.go:69) describes an artefact this is not")
			}
		})
	}
}

// TestTheNoticeIsSeparableByTheRule is the coupling R-111 does not name, and it
// is the one with the sharpest consequence.
//
// `stripNotice` removes the intended-use notice by splitting at the renderer's
// horizontal rule (`SplitNotice`, `jets/agentic/briefing/prose/prose.go:106`),
// and the rule is a **literal element of the document** - sixty-eight hyphens,
// configuration like everything else. A document that dropped it leaves
// `SplitNotice` returning the whole artefact as body, and the notice is then
// counted as prose. It is the one string in the artefact that must carry
// imperative language, so the advisory categories would fire on the exemption
// itself: findings against the arm that exists to score zero, from a template
// change nothing else would report.
func TestTheNoticeIsSeparableByTheRule(t *testing.T) {
	for arm, rendered := range renderBothArms(t) {
		t.Run(arm, func(t *testing.T) {
			notice, _ := prose.SplitNotice(rendered)
			if strings.TrimSpace(notice) == "" {
				t.Fatalf("SplitNotice found no rule in the artefact, so the notice reaches every check " +
					"in this package as prose")
			}
			if body := stripNotice(nil, rendered); strings.Contains(body, "not medical advice") {
				t.Errorf("the notice survives stripNotice and will be scored as prose:\n%s", body)
			}
		})
	}
}
