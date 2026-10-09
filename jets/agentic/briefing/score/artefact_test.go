package score_test

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/agentic/briefing/score"
)

// The validation harness. It reads an `AT.5`-shaped side-by-side artefact and
// scores both arms of every member in it.
//
// # It is gated on an environment variable and it commits no artefact
//
// The artefacts are model output. Committing one would make this a fixture test
// over a captured run, which is the wrong shape twice over: the run is not
// reproducible (`F835`, and `I-575` only fixes it under `pool_size: 1`), and a
// scorer whose test data is one saved run is a scorer that can be tuned to it
// without anybody noticing. So the artefacts stay where they are and the test
// skips when it is not pointed at one:
//
//	JETS_SCORE_ARTEFACT=/path/run_B.md,/path/run_C.md go test -count=1 -v \
//	  -run TestAgainstTheLabelledArtefacts ./jets/agentic/briefing/score/
//
// # What it asserts, and what it only prints
//
// **It asserts the negative control**: the template arm must score zero in every
// category, on every member, in every artefact. That is a property of the arm
// rather than of a run - the template is a total function of the entity map -
// so it is a real assertion and not a snapshot.
//
// **It prints the model arm** and asserts nothing about it. `AT.5`'s hand counts
// are a labelled reference, not a specification, and a test that failed when the
// scorer disagreed with them would be a test that forces the scorer to agree.
// The adjudication is a reader's job and the report is what they read.

const armTemplate, armModel = "template", "model"

type pair struct {
	member   string
	toon     string
	template string
	model    string
}

var (
	reMember   = regexp.MustCompile(`(?m)^## (\S+)\s*$`)
	reInput    = regexp.MustCompile("(?s)### Input[^\n]*\n+```\n(.*?)\n```")
	reTemplate = regexp.MustCompile("(?s)### Arm \\(c\\)[^\n]*\n+```\n(.*?)\n```")
	reModel    = regexp.MustCompile("(?s)### Arm \\(b'\\)[^\n]*\n+```\n(.*?)\n```")
)

// readArtefact splits a side-by-side artefact into member sections. The header
// section before the first `## <id>` and any `##` whose body carries no input
// block are dropped, which is what makes this tolerant of an artefact that grows
// a preamble.
func readArtefact(t *testing.T, path string) []pair {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	doc := string(raw)
	locs := reMember.FindAllStringSubmatchIndex(doc, -1)
	var out []pair
	for i, loc := range locs {
		end := len(doc)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := doc[loc[1]:end]
		p := pair{member: doc[loc[2]:loc[3]]}
		m := reInput.FindStringSubmatch(body)
		if m == nil {
			continue
		}
		p.toon = m[1]
		if m := reTemplate.FindStringSubmatch(body); m != nil {
			p.template = m[1]
		}
		if m := reModel.FindStringSubmatch(body); m != nil {
			p.model = m[1]
		}
		out = append(out, p)
	}
	return out
}

func artefacts(t *testing.T) []string {
	t.Helper()
	v := strings.TrimSpace(os.Getenv("JETS_SCORE_ARTEFACT"))
	if v == "" {
		t.Skip("JETS_SCORE_ARTEFACT is unset; this test reads an AT.5 side-by-side artefact, " +
			"which is model output and is deliberately not committed")
	}
	return strings.Split(v, ",")
}

func TestAgainstTheLabelledArtefacts(t *testing.T) {
	for _, path := range artefacts(t) {
		path := strings.TrimSpace(path)
		t.Run(shortName(path), func(t *testing.T) {
			pairs := readArtefact(t, path)
			if len(pairs) == 0 {
				t.Fatalf("%s: no member sections found", path)
			}
			tmpl, model := &score.Corpus{}, &score.Corpus{}
			for _, p := range pairs {
				entity, err := briefing.DecodeTOONEntity(p.toon)
				if err != nil {
					t.Fatalf("%s %s: decoding the TOON input: %v", path, p.member, err)
				}
				facts := score.ReadFactSet(entity)
				if p.template != "" {
					tmpl.Add(score.Score(p.member, facts, p.template, score.Options{}))
				}
				if p.model != "" {
					model.Add(score.Score(p.member, facts, p.model, score.Options{}))
				}
			}

			t.Logf("\n=== %s: %d members ===", shortName(path), len(pairs))
			t.Logf("\n--- model arm ---\n%s", table(model))
			t.Logf("\n--- template arm (negative control) ---\n%s", table(tmpl))

			// The assertion. Anything the template arm flags is a false positive
			// the scorer can be shown.
			for _, r := range tmpl.Rates() {
				if r.Flagged == 0 {
					continue
				}
				for _, f := range tmpl.FindingsFor(r.Category.Name) {
					t.Errorf("template arm flagged %s: %s — %s\n  %q",
						r.Category.Name, f.Subject, f.Detail, f.Quote)
				}
			}
		})
	}
}

// TestTheMaintenanceWindowIsSwept is the measurement behind DefaultWindow.
//
// It prints the drug-level maintenance count for both arms at every window from
// 4 to 80. What a reader is looking for is the value at which the model count
// stops moving and the value at which the template count starts - and, if the
// template never moves, that the clause bound rather than the window is what is
// holding the control, which is what the package comment claims.
func TestTheMaintenanceWindowIsSwept(t *testing.T) {
	for _, path := range artefacts(t) {
		path := strings.TrimSpace(path)
		pairs := readArtefact(t, path)
		var lines []string
		for w := 4; w <= 80; w += 4 {
			mFlag, tFlag, mMembers := 0, 0, 0
			for _, p := range pairs {
				entity, err := briefing.DecodeTOONEntity(p.toon)
				if err != nil {
					t.Fatalf("%s %s: %v", path, p.member, err)
				}
				facts := score.ReadFactSet(entity)
				if p.model != "" {
					r := score.Score(p.member, facts, p.model, score.Options{Window: w})
					n := r.Flagged(score.CatMaintenanceDrug)
					mFlag += n
					if n > 0 {
						mMembers++
					}
				}
				if p.template != "" {
					tFlag += score.Score(p.member, facts, p.template, score.Options{Window: w}).
						Flagged(score.CatMaintenanceDrug)
				}
			}
			lines = append(lines, fmt.Sprintf("  window %3d   model %3d drugs over %2d members   template %d",
				w, mFlag, mMembers, tFlag))
		}
		t.Logf("\n=== %s: maintenance_drug against the attribution window ===\n%s",
			shortName(path), strings.Join(lines, "\n"))
	}
}

// TestTheOmissionRuleIsCompared runs the three omission rules side by side and
// prints the per-member breakdown, which is where `AT.5`'s hand count is
// concentrated (`M007` 12 of 21, `M008` 6 of 17) and therefore where a
// disagreement is worth opening.
func TestTheOmissionRuleIsCompared(t *testing.T) {
	for _, path := range artefacts(t) {
		path := strings.TrimSpace(path)
		pairs := readArtefact(t, path)
		var lines []string
		var low, point, high, denom int
		for _, p := range pairs {
			if p.model == "" {
				continue
			}
			entity, err := briefing.DecodeTOONEntity(p.toon)
			if err != nil {
				t.Fatalf("%s %s: %v", path, p.member, err)
			}
			facts := score.ReadFactSet(entity)
			r := score.Score(p.member, facts, p.model, score.Options{}).Result(score.CatConditionOmitted)
			low += r.Low
			point += r.Flagged
			high += r.High
			denom += r.Denominator
			if r.Flagged > 0 || r.High > 0 {
				lines = append(lines, fmt.Sprintf("  %-6s lenient %2d   point %2d   strict %2d   of %2d",
					p.member, r.Low, r.Flagged, r.High, r.Denominator))
			}
		}
		lines = append(lines, fmt.Sprintf("  %-6s lenient %2d   point %2d   strict %2d   of %2d",
			"TOTAL", low, point, high, denom))
		t.Logf("\n=== %s: condition_omitted, three rules ===\n%s", shortName(path), strings.Join(lines, "\n"))
	}
}

// TestTheFindingsAreListed prints every finding of the model arm, by category,
// so that a disagreement with the hand count can be adjudicated without running
// anything twice.
func TestTheFindingsAreListed(t *testing.T) {
	paths := artefacts(t)
	path := strings.TrimSpace(paths[0])
	pairs := readArtefact(t, path)
	c := &score.Corpus{}
	for _, p := range pairs {
		if p.model == "" {
			continue
		}
		entity, err := briefing.DecodeTOONEntity(p.toon)
		if err != nil {
			t.Fatalf("%s %s: %v", path, p.member, err)
		}
		c.Add(score.Score(p.member, score.ReadFactSet(entity), p.model, score.Options{}))
	}
	for _, r := range c.Rates() {
		fs := c.FindingsFor(r.Category.Name)
		if len(fs) == 0 {
			continue
		}
		var b strings.Builder
		for _, f := range fs {
			amb := ""
			if f.Ambiguous {
				amb = "  [unattributable]"
			}
			fmt.Fprintf(&b, "  %-28s %s%s\n      %s\n", f.Subject, f.Detail, amb, trunc(f.Quote, 150))
		}
		t.Logf("\n=== %s (%s) ===\n%s", r.Category.Name, r.String(), b.String())
	}
}

func table(c *score.Corpus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %-26s %-22s %-18s %s\n", "category", "count", "soundness", "direction")
	for _, r := range c.Rates() {
		fmt.Fprintf(&b, "  %-26s %-22s %-18s %s\n",
			r.Category.Name, r.String(), r.Category.Soundness, r.Category.Direction)
	}
	fmt.Fprintf(&b, "  (members carrying at least one finding, by category)\n")
	rates := c.Rates()
	sort.SliceStable(rates, func(i, j int) bool { return rates[i].Category.Name < rates[j].Category.Name })
	for _, r := range rates {
		if r.Members == 0 {
			continue
		}
		fmt.Fprintf(&b, "    %-26s %d / %d briefings\n", r.Category.Name, r.Members, len(c.Reports))
	}
	return b.String()
}

func shortName(p string) string {
	i := strings.LastIndex(p, "/")
	return p[i+1:]
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
