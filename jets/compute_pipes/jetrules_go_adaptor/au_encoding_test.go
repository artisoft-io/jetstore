package jetrules_go_adaptor

// `AU.1`: does TOON cost accuracy against JSON? (agentic_ai Phase 7 track AU)
//
// # The design is paired per member, and that is what makes it one variable
//
// For each member the rules run **once**. The briefing entity is extracted
// **once**, and that one `map[string]any` is: encoded as TOON, encoded as JSON,
// and read into the `score.FactSet` both arms are scored against. So the two
// arms differ in the bytes the model was sent and in nothing else - not in the
// fact set, not in the rule session, not in the member, and not in the scorer's
// idea of the truth.
//
// **That is a stronger control than `AT.5` needed.** There the arms differed in
// the renderer; here they differ in the serialisation of one identical map, and
// the fact set the scorer uses is the same object for both.
//
// # Why it stratifies, and why stratifying does not de-confound
//
// **TOON is not one encoding.** `detectTabular` emits a table when every element
// of a list has the same key set and every field is primitive, and the expanded
// form otherwise (agentic_ai §1.25.2, reproduced on 40 of 40 lists). **JSON has
// no such split**, so a pooled TOON-against-JSON comparison sets one encoding
// against a blend of two, in a proportion fixed by this population's regimen mix
// (**I-577**).
//
// So every count below is reported **per stratum**. And the stratum is not
// independent of the hazard: a list is expanded *because* it carries a
// multi-valued field, and a multi-valued field is what a model mis-zips. **So
// the stratified result says "records of this shape" rather than "this encoding
// form is safer"**, and the report must not upgrade one into the other.
//
// # Gate
//
// Needs a compiled workspace and JETS_AU_MODEL_URL. Skips otherwise, so an
// ordinary run makes no network call.

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/score"
)

// auTabularHeader matches a TOON list header carrying a field list, which is
// what the tabular form looks like: `has_Briefing_Medical_Events[5]{A,B,C}:`.
var auTabularHeader = regexp.MustCompile(`\[\d+\]\{[^}]*\}:`)

type auArm struct {
	encoding string
	report   *score.Report
	prompt   int
	eval     int
	chars    int
}

type auMember struct {
	id      string
	tabular bool // any list in the TOON came out tabular
	arms    map[string]*auArm
}

func TestAU1EncodingAgainstAccuracy(t *testing.T) {
	requireCompiledWorkspace(t)
	if os.Getenv("JETS_AU_MODEL_URL") == "" {
		t.Skip("JETS_AU_MODEL_URL is not set; see the header of this file")
	}
	// The model arm reads JETS_AT5_MODEL_URL; AU uses its own name so a stray
	// AT.5 environment cannot start a 44-call run by accident.
	t.Setenv("JETS_AT5_MODEL_URL", os.Getenv("JETS_AU_MODEL_URL"))
	if v := os.Getenv("JETS_AU_MODEL"); v != "" {
		t.Setenv("JETS_AT5_MODEL", v)
	}
	arm := at5Model(t)
	if arm == nil {
		t.Fatal("the model arm did not build")
	}
	p := at5LoadPopulation(t)

	var doc strings.Builder
	fmt.Fprintf(&doc, "# `AU.1` — TOON against JSON over one fact set\n\n")
	fmt.Fprintf(&doc, "Run %s. Backend `%s`, model `%s`.\n\n",
		time.Now().UTC().Format(time.RFC3339), arm.url, arm.model)
	fmt.Fprintf(&doc, "**Every figure is an upper bound**: the population's NDCs were curated to "+
		"resolve, so it excludes the unresolved-NDC case that occurs in production (criterion 80), "+
		"and 27 of its 301 medical claims carry a primary diagnosis with no description behind it "+
		"(I-566).\n\n")

	members := make([]*auMember, 0, len(p.memberIDs))
	for _, id := range p.memberIDs {
		s, rm := ruleSessionOver(t, at5Fixture(p, id))
		pr := readProjection(t, s, rm)

		// One extraction, one fact set, two encodings.
		entity := at5ExtractMap(t, s, pr.briefing, at5PromptExclusions)
		facts := score.ReadFactSet(entity)
		toon := at5EncodeString(t, s, pr.briefing, "toon", at5PromptExclusions)
		jsonEnc := at5EncodeString(t, s, pr.briefing, "json", at5PromptExclusions)

		m := &auMember{id: id, tabular: auTabularHeader.MatchString(toon), arms: map[string]*auArm{}}
		for _, e := range []struct {
			name string
			body string
		}{{"toon", toon}, {"json", jsonEnc}} {
			_, answer, finish, promptTok, evalTok, err := arm.call(e.body)
			if err != nil {
				t.Fatalf("%s %s: %v", id, e.name, err)
			}
			if finish != "stop" {
				t.Logf("WARNING %s %s finished with %q", id, e.name, finish)
			}
			m.arms[e.name] = &auArm{
				encoding: e.name,
				report:   score.Score(id, facts, answer, score.Options{}),
				prompt:   promptTok, eval: evalTok, chars: len(e.body),
			}
			fmt.Fprintf(&doc, "## %s — %s (%d chars, %d prompt tokens)\n\n```\n%s\n```\n\n",
				id, e.name, len(e.body), promptTok, strings.TrimSpace(answer))
		}
		members = append(members, m)
	}

	auReport(t, &doc, members)

	if out := os.Getenv("JETS_AU_ARTEFACT"); out != "" {
		if err := os.WriteFile(out, []byte(doc.String()), 0o644); err != nil {
			t.Fatalf("writing %s: %v", out, err)
		}
		t.Logf("artefact written to %s", out)
	}
}

// auReport prints the counts per encoding, then per encoding per stratum.
func auReport(t *testing.T, doc *strings.Builder, members []*auMember) {
	t.Helper()
	strata := []struct {
		name string
		keep func(*auMember) bool
	}{
		{"all", func(*auMember) bool { return true }},
		{"TOON tabular", func(m *auMember) bool { return m.tabular }},
		{"TOON expanded", func(m *auMember) bool { return !m.tabular }},
	}
	for _, st := range strata {
		var n int
		cost := map[string][2]int{} // encoding -> {prompt tokens, chars}
		totals := map[string]map[string][2]int{}
		for _, m := range members {
			if !st.keep(m) {
				continue
			}
			n++
			for enc, a := range m.arms {
				c := cost[enc]
				cost[enc] = [2]int{c[0] + a.prompt, c[1] + a.chars}
				if totals[enc] == nil {
					totals[enc] = map[string][2]int{}
				}
				for _, r := range a.report.Results {
					v := totals[enc][r.Category.Name]
					totals[enc][r.Category.Name] = [2]int{v[0] + r.Flagged, v[1] + r.Denominator}
				}
			}
		}
		if n == 0 {
			continue
		}
		line := fmt.Sprintf("\n### %s — %d members\n\n| Category | TOON | JSON |\n|---|---|---|\n", st.name, n)
		names := make([]string, 0, len(totals["toon"]))
		for k := range totals["toon"] {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			a, b := totals["toon"][k], totals["json"][k]
			line += fmt.Sprintf("| %s | %d / %d | %d / %d |\n", k, a[0], a[1], b[0], b[1])
		}
		line += fmt.Sprintf("| **prompt tokens** | **%d** | **%d** |\n", cost["toon"][0], cost["json"][0])
		line += fmt.Sprintf("| **prompt characters** | **%d** | **%d** |\n", cost["toon"][1], cost["json"][1])
		doc.WriteString(line)
		t.Log(line)
	}
}
