// Package score measures a prose briefing against the fact set it was made
// from, and declares for every category how sound that measurement is.
//
// It is the instrument `AU.1` needs. `AT.5` measured the model arm against the
// fact set over 22 members and produced the phase's headline counts - 17 of 18
// member-runs misstating a maintenance indicator, 19 of 92 condition values
// omitted, 10 of 22 briefings carrying a statement untrue of the fact set - with
// a 47-line script in a session scratchpad that was never committed. `AU.1` is
// that same measurement run twice with one variable changed, so the scorer has
// to be a package rather than a script, and it has to be one whose numbers can
// be argued with.
//
// # The design rule: every category declares how sound it is
//
// `AT.5`'s own report says of its omission count that it *"is an instrument, not
// a measurement"*. The script asked whether half a condition's content words
// appear anywhere in the prose, and that rule passed a missing *rheumatoid
// arthritis* because *arthritis* is a substring of *osteoarthritis*, and flagged
// an *immunization* the model did name. The three-run range it reported, 17 to
// 21 of 92, is the instrument's spread and not the model's.
//
// **A scorer that returned one undifferentiated number would be worse than the
// hand count it replaces, because it would be believed.** So every category here
// carries a [Soundness] and a residue:
//
//   - [Decidable] - the test is a containment over a closed list taken from the
//     fact set, with no parameter a reader would have to arbitrate. Given the
//     rule, the answer is the answer.
//   - [Approximate] - a heuristic, with a stated [Direction] of error.
//   - [NotAutomatable] - a category a reader has to judge. It reports **zero
//     findings and says so**, rather than being left out, because a category
//     silently absent from a report reads as a category that scored zero.
//
// **[Decidable] is a claim about the rule and not about the reader's question**,
// and every category states both: `Question` is what a reader wants to know and
// `Rule` is what the code tests. `Residue` names the gap. A category can be
// decidable and still answer a narrower question than the one it is a proxy for
// - the maintenance check below is exactly that, and saying so is the whole
// difference between an instrument and a number.
//
// # Why the maintenance category is the one made exact
//
// `I-579` says the model's maintenance error is *"systematic, one-directional
// and unguarded"* and then says it is checkable: *the drug name appears in the
// prose within N words of the word "maintenance"* is a containment test over a
// **closed list**, which is `I-515`'s tractable half one property over. It is
// also the phase's headline result, so it is the number most worth being able to
// defend. Two checks carry it and they have different strengths:
//
//   - [CatMaintenanceAbsent] needs no window at all. When the fact set holds no
//     maintenance drug, *any* unnegated mention of maintenance is a
//     misstatement, because the briefing's only subject is this member's
//     medications. That is a containment test with no free parameter, and it is
//     the check behind `AT.5`'s *5 of 6*.
//   - [CatMaintenanceDrug] attributes a maintenance claim to named drugs and is
//     the drug-level count. It has one parameter, the window, and [Options]
//     records how it was chosen.
//
// # What the window is, and why the clause matters more than the number
//
// The obvious rule - *within N words either side* - has a false positive the
// template arm shows immediately. The template writes
//
//	Naproxen Sodium ER, a maintenance medication, five fills, most recently
//	23 October; Diclofenac Sodium, two fills, most recently 13 December
//
// and `Diclofenac Sodium`, which is not a maintenance drug, is nine words from
// the word *maintenance*. Any window wide enough to catch a list flags it.
//
// So attribution is **clause-bounded first and windowed second**: a drug mention
// is attributed to a maintenance token only when both are in the same clause,
// where a clause ends at a sentence terminator, a semicolon or a newline. The
// template's semicolons then do the work, and a model's list - *"maintenance
// medications like diclofenac, meloxicam, hadlima, ..."* - stays one clause and
// is caught whole. The window is a cap inside the clause rather than the
// mechanism, and [DefaultWindow] was chosen by sweeping it over the labelled
// corpus rather than picked; see the package's artefact test.
//
// # The template arm is a negative control and this package is built to be shown
//
// Every `AT.5` artefact carries both arms and the template scores zero in every
// category in all three runs. **A scorer that flags anything in the template arm
// has a false positive it can be shown**, which is a stronger check than any
// test written from the same head as the code. Three things in this package
// exist because that control found them:
//
//   - the clause bound above;
//   - stripping the intended-use notice, because the notice is the one string in
//     the artefact that must carry imperative language and an advisory check
//     over the whole rendered briefing counts the exemption as a violation
//     (`prose.SplitNotice` is what splits it, so the rule stays in one place);
//   - reading English number words, because the template writes *"Sixty-two
//     medical visits"* and a count check that only reads digits would report a
//     count it could not see as a count it disagrees with.
//
// # What it reuses
//
// The fact set is read with `briefing.Selector`, the same path language the
// prose renderer reads the entity with, so the rule that a single-valued
// multi-valued property arrives as a bare scalar is held in one place. Advisory
// language is `briefing.AdvisoryMarker`, `AK.3`'s own lexicon, rather than a
// second copy - with a supplement this package owns and reports separately,
// because that lexicon was built for record fields and misses two of the three
// sentences `AT.5` hand-counted. Both facts are recorded in [CatAdvisory]'s
// residue rather than hidden in a merged number.
package score

import (
	"fmt"
	"sort"
	"strings"
)

// Soundness is how much a category's count is worth. See the package comment.
type Soundness string

const (
	// Decidable: a containment test over a closed list drawn from the fact set,
	// with no parameter a reader arbitrates. It is a claim about the rule; the
	// category's Residue says what the rule does not reach.
	Decidable Soundness = "decidable"
	// Approximate: a heuristic. Direction says which way it is wrong.
	Approximate Soundness = "approximate"
	// NotAutomatable: a reader's judgement. Reports zero and says so.
	NotAutomatable Soundness = "not-automatable"
)

// Direction is the known error direction of an approximate check.
type Direction string

const (
	// DirectionNone is for a decidable or not-automatable category.
	DirectionNone Direction = ""
	// OverFlags reports things a reader would not: an upper bound.
	OverFlags Direction = "over-flags (upper bound)"
	// UnderFlags misses things a reader would flag: a lower bound.
	UnderFlags Direction = "under-flags (lower bound)"
	// BothWays is wrong in both directions and brackets nothing on its own.
	BothWays Direction = "wrong in both directions"
)

// Category names of this package. They are `AT.5`'s categories, because those
// are the ones validated against a hand count.
const (
	CatMaintenanceAbsent    = "maintenance_absent"
	CatMaintenanceDrug      = "maintenance_drug"
	CatMaintenanceUnderCall = "maintenance_under_call"
	CatAdherenceSurfaced    = "adherence_surfaced"
	CatAdherenceEvaluated   = "adherence_evaluated"
	CatAdherenceAttributed  = "adherence_attributed"
	CatConditionOmitted     = "condition_omitted"
	CatAdvisory             = "advisory_language"
	CatAboutTheSummary      = "writes_about_the_summary"
	CatEchoesPrompt         = "echoes_the_prompt"
	CatCountMismatch        = "count_mismatch"
	CatFillCountMismatch    = "fill_count_mismatch"
	CatUntrue               = "untrue_of_the_fact_set"
	CatInvention            = "invention"
	CatMispairing           = "mispairing"
	CatImplication          = "implication"
)

// A Category is one thing the scorer counts, together with everything a reader
// needs in order to decide how much the count is worth.
//
// **Question and Rule are separate fields on purpose.** A category whose rule
// answers its question exactly is rare; the useful ones are proxies, and a
// report that printed only the count would hide which is which.
type Category struct {
	// Name is the stable identifier, one of the Cat constants.
	Name string
	// Question is what a reader wants to know.
	Question string
	// Rule is exactly what the code tests.
	Rule string
	// Soundness is how far Rule can be trusted to answer Question.
	Soundness Soundness
	// Direction is the error direction when Soundness is Approximate.
	Direction Direction
	// Unit names what the denominator counts: "briefing", "drug",
	// "condition value", "qualifying briefing".
	Unit string
	// Residue is what the rule does not reach, in a reader's words. It is
	// required: a category with an empty residue is one nobody has thought
	// about, and the constructor below refuses to build one.
	Residue string
}

// A Finding is one thing the scorer found, with enough to adjudicate it by hand.
type Finding struct {
	// Category is the Cat constant it belongs to.
	Category string
	// Subject is the fact-set entity the finding is about - a drug name, a
	// condition value, a property name - or "" when it is about the briefing.
	Subject string
	// Detail says what is wrong, in one line.
	Detail string
	// Quote is the span of prose the finding was read off, so that a reader can
	// adjudicate without opening the artefact.
	Quote string
	// Ambiguous marks a finding whose subject could not be attributed to one
	// fact-set entity - a surface form two drugs share. **It is reported and not
	// counted**: an instrument that guesses here would be measuring its own
	// tie-break.
	Ambiguous bool
}

// A Result is one category's count over one briefing.
type Result struct {
	Category Category
	// Denominator is what this briefing contributes to the category's
	// denominator: 1 for a per-briefing category, the number of drugs or
	// condition values otherwise, and 0 for a briefing the category does not
	// apply to (a member with no drugs contributes nothing to a drug rate).
	Denominator int
	// Flagged is the count under the category's own rule.
	Flagged int
	// Low and High bracket Flagged when the category can state a range - a
	// lenient rule and a strict one over the same question. Both are zero when
	// the category does not bracket.
	Low, High int
	// Brackets reports whether Low and High are meaningful.
	Brackets bool
	// Findings are the individual flags, including the ambiguous ones, which are
	// present here and excluded from Flagged.
	Findings []Finding
}

// A Report is every category's Result over one briefing, in a fixed order so
// that two runs can be diffed.
type Report struct {
	// Subject is whatever the caller called this briefing - a member id.
	Subject string
	Results []Result
}

// Result returns the result for a category name, or nil.
func (r *Report) Result(name string) *Result {
	for i := range r.Results {
		if r.Results[i].Category.Name == name {
			return &r.Results[i]
		}
	}
	return nil
}

// Flagged is a category's count over this briefing, or 0 if it did not run.
func (r *Report) Flagged(name string) int {
	if res := r.Result(name); res != nil {
		return res.Flagged
	}
	return 0
}

// Options are the scorer's parameters. There are deliberately few, and each one
// is a number a reader could disagree with, which is why they are here rather
// than buried as constants.
type Options struct {
	// Window is how many words may separate a drug mention from the word it is
	// being attributed to, inside one clause. See [DefaultWindow].
	Window int
	// Prompt is the system prompt the briefing was produced under, when the
	// caller has it. It turns [CatEchoesPrompt] from a check over property names
	// into one that can also see an instruction reproduced verbatim. Empty is a
	// normal value and narrows that category rather than failing it.
	Prompt string
}

// DefaultWindow is the attribution window in words, inside a clause.
//
// **It was swept rather than chosen**, and the sweep says something about the
// design as well as about the number. Over `at5_run_B` the drug-level
// maintenance count runs 9, 14, 15, 17, 17, 18 at windows 4, 8, 12, 16, 20, 24
// and is **flat at 18 from 24 to 80**; the count of members flagged does not
// move at all after 4. The **template arm is 0 at every value tested**.
//
// So the window is a guard rather than a tuning knob. Two things follow and the
// second is the one worth carrying:
//
//   - 40 sits well inside the flat region, so the number is not load-bearing:
//     anything from 24 upward gives the same answer on this corpus.
//   - **the clause bound is what keeps the control clean, not the window.** If
//     the window were doing that work the template count would rise somewhere in
//     the sweep, and it does not. A reader tempted to tune this number should
//     know that tuning it changes nothing here, and that the thing it looks like
//     it protects is protected by something else.
//
// A caller pointing this package at another corpus should sweep it again rather
// than inherit the number: it is a property of how long the sentences are, and a
// briefing at 300 words is not this one.
const DefaultWindow = 40

// Score measures one prose briefing against one fact set.
//
// The prose is the rendered briefing, notice and all. **The notice is stripped
// here rather than by the caller** - see the package comment; a caller that has
// to remember is a caller that will not.
func Score(subject string, facts *FactSet, briefing string, opt Options) *Report {
	if opt.Window <= 0 {
		opt.Window = DefaultWindow
	}
	d := parseProse(stripNotice(facts, briefing))
	rep := &Report{Subject: subject}
	for _, check := range checks {
		rep.Results = append(rep.Results, check(facts, d, opt))
	}
	// The union category reads the ones above it, so it runs last and out of the
	// table rather than inside it.
	rep.Results = append(rep.Results, untrueUnion(rep))
	return rep
}

// checks is the fixed order every report is produced in.
var checks = []func(*FactSet, *reading, Options) Result{
	maintenanceAbsent,
	maintenanceDrug,
	maintenanceUnderCall,
	adherenceSurfaced,
	adherenceEvaluated,
	adherenceAttributed,
	conditionOmitted,
	advisoryLanguage,
	aboutTheSummary,
	echoesPrompt,
	countMismatch,
	fillCountMismatch,
	invention,
	mispairing,
	implication,
}

// A Corpus accumulates reports so that a rate can be stated with its
// denominator, which decision 13 requires of every rate this phase reports.
type Corpus struct {
	Reports []*Report
}

// Add records one briefing's report.
func (c *Corpus) Add(r *Report) { c.Reports = append(c.Reports, r) }

// A Rate is one category summed over a corpus.
type Rate struct {
	Category Category
	// Flagged and Denominator are the rate, in the category's own Unit.
	Flagged, Denominator int
	// Members is how many briefings carry at least one finding, which is the
	// denominator `AT.5` reports several categories in.
	Members int
	// Briefings is how many briefings the category applied to.
	Briefings int
	// Low and High are the summed bracket, when the category brackets.
	Low, High int
	Brackets  bool
	// Ambiguous is how many findings could not be attributed and are therefore
	// excluded from Flagged.
	Ambiguous int
}

// String renders a rate the way this repository writes one: a count over the
// denominator it was measured against, never a bare percentage.
func (r Rate) String() string {
	s := fmt.Sprintf("%d / %d %s", r.Flagged, r.Denominator, plural(r.Category.Unit, r.Denominator))
	if r.Brackets {
		s += fmt.Sprintf(" [%d-%d]", r.Low, r.High)
	}
	if r.Ambiguous > 0 {
		s += fmt.Sprintf(" (+%d unattributable)", r.Ambiguous)
	}
	return s
}

// Rates sums the corpus by category, in report order.
func (c *Corpus) Rates() []Rate {
	if len(c.Reports) == 0 {
		return nil
	}
	order := make([]string, 0, len(c.Reports[0].Results))
	byName := map[string]*Rate{}
	for _, rep := range c.Reports {
		for i := range rep.Results {
			res := &rep.Results[i]
			r, ok := byName[res.Category.Name]
			if !ok {
				r = &Rate{Category: res.Category}
				byName[res.Category.Name] = r
				order = append(order, res.Category.Name)
			}
			r.Flagged += res.Flagged
			r.Denominator += res.Denominator
			r.Low += res.Low
			r.High += res.High
			r.Brackets = r.Brackets || res.Brackets
			if res.Denominator > 0 {
				r.Briefings++
			}
			if res.Flagged > 0 {
				r.Members++
			}
			for _, f := range res.Findings {
				if f.Ambiguous {
					r.Ambiguous++
				}
			}
		}
	}
	out := make([]Rate, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}

// FindingsFor returns every finding of one category across the corpus, sorted by
// subject so that two runs list them the same way. It is what a reader opens
// when they want to adjudicate a count rather than read it.
func (c *Corpus) FindingsFor(category string) []Finding {
	var out []Finding
	for _, rep := range c.Reports {
		res := rep.Result(category)
		if res == nil {
			continue
		}
		for _, f := range res.Findings {
			f.Subject = rep.Subject + ": " + f.Subject
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Subject < out[j].Subject })
	return out
}

func plural(unit string, n int) string {
	if n == 1 || strings.HasSuffix(unit, "s") {
		return unit
	}
	return unit + "s"
}
