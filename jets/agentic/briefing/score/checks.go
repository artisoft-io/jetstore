package score

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// The checks. Each is a function of the fact set and the briefing, and each
// returns a Result carrying its own Category - so the declaration of how sound a
// check is sits beside the code that implements it and cannot drift from it.
//
// **Reading order matters here.** The two maintenance checks come first because
// they are the phase's headline result and the ones `I-579` says to get exactly
// right; the union at the end reads the others and is a lower bound by
// construction.

// ---------------------------------------------------------------- maintenance

// maintenanceAbsent is the check with no free parameter, and it is the one
// behind `AT.5`'s *5 of 6*.
//
// When the fact set holds drugs and none of them is flagged maintenance, the
// briefing's only subject is those drugs - so any unnegated mention of
// maintenance asserts something the fact set denies. No window, no attribution,
// no lexicon: a containment test over a closed condition.
//
// **Its residue is negation and it is handled rather than ignored.** *"No
// maintenance medications are on record"* is a true sentence about such a
// member, and `negated` looks three words back for the negators a briefing
// writes. That is the one judgement in this check and it is stated in the
// category.
func maintenanceAbsent(f *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatMaintenanceAbsent,
		Question:  "does the briefing describe a member with no maintenance drug as being on maintenance medication?",
		Rule:      "the fact set holds drugs and none is `maintenance Y`, and the prose contains the word `maintenance` unnegated",
		Soundness: Decidable,
		Unit:      "qualifying briefing",
		Residue: "a briefing that says the same thing without the word - `long-term therapy`, `daily medications` - " +
			"is not seen; and a negation more than three words from the word is read as an assertion",
	}
	res := Result{Category: cat}
	if len(f.Drugs) == 0 || f.HasMaintenanceDrug() {
		// Not a qualifying briefing. Denominator 0: it contributes nothing to
		// the rate rather than contributing a pass, which is the difference
		// between `5 of 6` and `5 of 22`.
		return res
	}
	res.Denominator = 1
	echo := p.flagEcho(f)
	for _, i := range p.find("maintenance") {
		if p.negated(i) || echo[i] {
			continue
		}
		res.Flagged = 1
		res.Findings = append(res.Findings, Finding{
			Category: cat.Name,
			Detail: fmt.Sprintf("the briefing says `maintenance` and none of this member's %d drugs is flagged `maintenance Y`",
				len(f.Drugs)),
			Quote: p.quote(i),
		})
		break
	}
	return res
}

// maintenanceDrug is `I-579`'s check: the drug name appears in the prose within
// N words of the word *maintenance*, over the closed list of this member's
// drugs.
//
// **The attribution is clause-bounded and then windowed**, which is the whole of
// why the template arm scores zero - see the package comment for the sentence
// that forced it. An ambiguous mention, one whose surface form two drugs answer
// to, is flagged **only when every owner is non-maintenance**: that is the case
// where the tie-break cannot change the answer, and anywhere else the finding is
// reported with Ambiguous set and left out of the count.
func maintenanceDrug(f *FactSet, p *reading, opt Options) Result {
	cat := Category{
		Name:     CatMaintenanceDrug,
		Question: "which drugs does the briefing call maintenance drugs that the fact set says are not?",
		Rule: fmt.Sprintf("a drug's surface form is claimed by an unnegated `maintenance` anchor in its clause, nearest wins "+
			"and an appositive claims only its own noun, within %d words; and the fact set flags that drug `maintenance N`", opt.Window),
		Soundness: Approximate,
		Direction: BothWays,
		Unit:      "drug",
		Residue: "the containment is decidable and **the attribution is not**, which is why this is approximate where " +
			"CatMaintenanceAbsent is decidable. A maintenance claim naming no drug - `maintenance opioids` - attributes to " +
			"nothing and is missed; a coordination the attribution rules get wrong is counted. Measured against AT.5's hand " +
			"count it agrees exactly on M007, 7 of the 16 drugs named, and scores 0 over 66 template briefings",
	}
	res := Result{Category: cat, Denominator: len(f.Drugs)}
	if len(f.Drugs) == 0 {
		return res
	}
	anchors := p.find("maintenance")
	if len(anchors) == 0 {
		return res
	}
	echo := p.flagEcho(f)
	ms := p.mentions(f)
	ends := mentionEnds(ms)
	flagged := map[int]bool{}
	for _, m := range ms {
		a := p.claimant(m, anchors, opt.Window, ends)
		if a < 0 || echo[a] || p.negated(a) {
			continue
		}
		allWrong := true
		for _, di := range m.drugs {
			if f.Drugs[di].Maintenance {
				allWrong = false
			}
		}
		if !allWrong {
			if len(m.drugs) > 1 {
				res.Findings = append(res.Findings, Finding{
					Category:  cat.Name,
					Subject:   m.form,
					Detail:    "the surface form answers to more than one drug and they do not agree on the indicator",
					Quote:     p.quote(m.at),
					Ambiguous: true,
				})
			}
			continue
		}
		for _, di := range m.drugs {
			if flagged[di] {
				continue
			}
			flagged[di] = true
			res.Flagged++
			res.Findings = append(res.Findings, Finding{
				Category:  cat.Name,
				Subject:   f.Drugs[di].Name,
				Detail:    "called a maintenance drug; the fact set flags it `maintenance N`",
				Quote:     p.quote(m.at),
				Ambiguous: len(m.drugs) > 1,
			})
		}
	}
	return res
}

// maintenanceUnderCall is the other direction, and it is here to be reported as
// zero rather than to be counted.
//
// `F824` measured the error one-directional - `N` becomes `Y` and never the
// reverse - over three runs. **That is a measurement about the model, and an
// instrument that returned 0 because it cannot see the reverse would be
// indistinguishable from one that returned 0 because the reverse did not
// happen.** Detecting *this drug is not a maintenance drug* needs the scope of a
// negation over a noun phrase, which is a reader's judgement rather than a
// containment test, so the category says so and counts nothing.
func maintenanceUnderCall(_ *FactSet, _ *reading, _ Options) Result {
	return Result{Category: Category{
		Name:      CatMaintenanceUnderCall,
		Question:  "which maintenance drugs does the briefing describe as not being maintenance drugs?",
		Rule:      "none; this reports zero and means unmeasured",
		Soundness: NotAutomatable,
		Unit:      "drug",
		Residue: "the whole category. It needs the scope of a negation over a noun phrase. " +
			"F824 measured the model's error one-directional over three runs, so the true count over that corpus is " +
			"believed to be zero - which is a fact about the model and not evidence from this check",
	}}
}

// ------------------------------------------------------------------ adherence

func adherenceSurfaced(f *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatAdherenceSurfaced,
		Question:  "does the briefing mention the adherence ratio at all?",
		Rule:      "the prose contains one of: adherence, adherent, compliance, compliant",
		Soundness: Decidable,
		Unit:      "briefing",
		Residue: "a briefing that reports the number without naming it - `filled 0 of the days supplied` - " +
			"is not seen. Q-98's design keeps the ratio on the entity for both arms, so the denominator is every briefing",
	}
	res := Result{Category: cat, Denominator: 1}
	for _, w := range []string{"adherence", "adherent", "compliance", "compliant"} {
		if idx := p.find(w); len(idx) > 0 {
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{Category: cat.Name, Detail: "mentions " + w, Quote: p.quote(idx[0])})
			break
		}
	}
	return res
}

// adherenceEvaluated counts `F823`: the model giving an integer-division
// artefact an evaluative reading.
//
// **This is a lexicon and it is wrong in both directions**, which is why it is
// approximate. It misses a phrasing nobody thought of, and it flags an
// evaluative word that happens to sit near the ratio without qualifying it. The
// line it draws is the one `AT.5`'s hand count drew: reading the number out is
// not evaluation - *"an adherence value of 0"* - and re-expressing it as a grade
// is - *"no reported adherence"*, *"one hundred percent adherence"*, *"full
// adherence"*, *"0.00%"*, the unit `M040` invented that the pipeline cannot
// produce.
func adherenceEvaluated(f *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatAdherenceEvaluated,
		Question:  "does the briefing attach an evaluative reading to the adherence ratio?",
		Rule:      "an evaluative term from a closed lexicon, or a percent sign, occurs in the same clause as an adherence word",
		Soundness: Approximate,
		Direction: BothWays,
		Unit:      "briefing",
		Residue: "the boundary between reading the number out and grading it is a reader's line, drawn here where " +
			"AT.5's hand count drew it: `an adherence value of 0` is not evaluation and `no reported adherence` is",
	}
	res := Result{Category: cat, Denominator: 1}
	anchors := anchorsFor(p, "adherence", "adherent", "compliance", "compliant")
	if len(anchors) == 0 {
		return res
	}
	for _, a := range anchors {
		// A percent sign anywhere in the clause. `M040` writes the ratio as
		// `0.00%`, which is a unit the pipeline cannot produce - F822 measured
		// the ratio taking two values by integer division - and the character is
		// dropped by the tokeniser, so it is looked for in the clause as
		// written rather than among the words.
		if strings.Contains(p.clauseText(p.words[a].clause), "%") {
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: "%",
				Detail: "the ratio rendered as a percentage; the pipeline produces no such unit (F822)",
				Quote:  p.quote(a),
			})
			return res
		}
		for _, phrase := range evaluativeTerms {
			for _, i := range p.find(phrase) {
				if p.words[i].clause != p.words[a].clause {
					continue
				}
				res.Flagged = 1
				res.Findings = append(res.Findings, Finding{
					Category: cat.Name,
					Subject:  phrase,
					Detail:   "an evaluative reading of a ratio that takes two values on this pipeline (F822)",
					Quote:    p.quote(a),
				})
				return res
			}
		}
	}
	return res
}

// evaluativeTerms is the closed lexicon. `percent` and `%` are in it because the
// ratio is not a percentage and rendering it as one is `M040`'s invented unit.
var evaluativeTerms = []string{
	"full", "complete", "perfect", "total", "high", "low", "poor", "good",
	"excellent", "strong", "weak", "varying", "variable", "inconsistent",
	"no reported", "not reported", "none reported", "one hundred percent",
	"hundred percent", "percent", "100", "partial", "adequate", "inadequate",
	"consistent", "suboptimal", "optimal",
}

// adherenceAttributed counts `F826`: an adherence value attributed to a drug
// that carries none. It reuses maintenanceDrug's attribution machinery exactly,
// with the anchor changed and the fact-set predicate changed.
func adherenceAttributed(f *FactSet, p *reading, opt Options) Result {
	cat := Category{
		Name:     CatAdherenceAttributed,
		Question: "which drugs does the briefing give an adherence value that the fact set gives none?",
		Rule: fmt.Sprintf("a drug's surface form is claimed by an unnegated adherence anchor in its clause, within %d words, "+
			"and the fact set gives that drug no ratio", opt.Window),
		Soundness: Approximate,
		Direction: BothWays,
		Unit:      "drug",
		Residue: "wrong in both directions, and the corpus shows one of each. It **misses** a quantifier rather than a " +
			"name - M031's `both medications have an adherence value of 0` names neither. It **over-flags** a drug the " +
			"briefing correctly excludes in the same clause - M030's `and SUMAtriptan Succinate, which is not a " +
			"maintenance drug`, where the model gives it no value and the nearest anchor claims it anyway",
	}
	res := Result{Category: cat, Denominator: len(f.Drugs)}
	if len(f.Drugs) == 0 {
		return res
	}
	anchors := anchorsFor(p, "adherence", "adherent")
	if len(anchors) == 0 {
		return res
	}
	ms := p.mentions(f)
	ends := mentionEnds(ms)
	flagged := map[int]bool{}
	for _, m := range ms {
		a := p.claimant(m, anchors, opt.Window, ends)
		if a < 0 || p.negated(a) {
			continue
		}
		allWrong := true
		for _, di := range m.drugs {
			if f.Drugs[di].HasAdherence {
				allWrong = false
			}
		}
		if !allWrong {
			continue
		}
		for _, di := range m.drugs {
			if flagged[di] {
				continue
			}
			flagged[di] = true
			res.Flagged++
			res.Findings = append(res.Findings, Finding{
				Category:  cat.Name,
				Subject:   f.Drugs[di].Name,
				Detail:    "given an adherence value; the fact set carries none for it",
				Quote:     p.quote(m.at),
				Ambiguous: len(m.drugs) > 1,
			})
		}
	}
	return res
}

func anchorsFor(p *reading, words ...string) []int {
	var out []int
	for _, w := range words {
		out = append(out, p.find(w)...)
	}
	return out
}

// ------------------------------------------------------------------- omission

// conditionOmitted is the category `AT.5`'s report calls *an instrument, not a
// measurement*, and it is the one this package changes most.
//
// # Three rules over one question, reported as a bracket
//
// *Is this condition named?* is a reader's judgement. A single heuristic that
// answers it produces one number that looks like a measurement, which is what
// the previous instrument did and what its own author warned about. So this
// reports three:
//
//   - **lenient** - any distinctive token appears. Under-flags: a briefing that
//     writes *arthritis* passes for *rheumatoid arthritis*. This is the **lower
//     bound** on the omission count.
//   - **point** - at least half the distinctive tokens appear. The previous
//     script's rule, kept so that the two are comparable.
//   - **strict** - every distinctive token appears. Over-flags: a legitimate
//     paraphrase fails. This is the **upper bound**.
//
// `Flagged` is the point rule and `Low`/`High` bracket it. **A hand count that
// falls outside the bracket is a finding about the instrument**; one inside it
// is the most this rule can say.
//
// # One change to the rule itself, and it moves a number
//
// Tokens are matched **as words**, not as substrings. `AT.5`'s script used
// `strings.Contains` and therefore passed `M007`'s missing *rheumatoid
// arthritis*, because *arthritis* is inside *osteoarthritis* - which its own
// report names as the instrument's clearest error. A word-boundary test does not
// make that mistake, and the corpus is where the difference is measured rather
// than asserted.
//
// A condition whose parenthesised code appears in the prose is named outright,
// which is decidable and needs no tokens at all.
func conditionOmitted(f *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatConditionOmitted,
		Question:  "which Condition_Summary values does the prose not name?",
		Rule:      "the condition's code is absent and fewer than half its distinctive words occur as words in the prose",
		Soundness: Approximate,
		Direction: BothWays,
		Unit:      "condition value",
		Residue: "a paraphrase a reader would accept fails the strict rule and a shared word passes the lenient one; " +
			"the bracket is where the answer is, and the point estimate inside it is a rule rather than a reading",
	}
	res := Result{Category: cat, Denominator: len(f.Conditions), Brackets: true}
	for _, c := range f.Conditions {
		if c.Code != "" && p.has(c.Code) {
			continue
		}
		hit := 0
		var missing []string
		for _, t := range c.tokens {
			if p.hasWord(t) {
				hit++
			} else {
				missing = append(missing, t)
			}
		}
		n := len(c.tokens)
		if hit == 0 {
			res.Low++
		}
		if hit < n {
			res.High++
		}
		if hit < (n+1)/2 {
			res.Flagged++
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name,
				Subject:  c.Raw,
				Detail:   fmt.Sprintf("%d of %d distinctive words present; missing %s", hit, n, strings.Join(missing, ", ")),
			})
		}
	}
	return res
}

// ------------------------------------------------------------ prompt and tone

// advisoryLanguage reuses `AK.3`'s lexicon and says what reusing it costs.
//
// # The base is briefing.AdvisoryMarker and the supplement is this package's
//
// `AK.3` was built to police the fields of a briefing *record*, and its own doc
// comment says it is blunt on purpose and safe because its blast radius is a
// declared free-text field. Applied to whole prose it is neither blunt enough
// nor broad enough in the same places, and the corpus shows exactly where: it
// misses `M025`'s *"suggesting focus on medication adherence"* because
// `suggesting` is not `suggest` under a word-boundary test, and it misses
// `"Remember, the adherence ratio ..."` because `remember to` is a phrase and
// `remember` is not in the imperative list.
//
// **So the supplement is separate and every finding says which fired.** Widening
// the shared lexicon would change the guardrail's behaviour for a caller that
// did not ask, and this package has no standing to do that; a supplement it owns
// can be argued with on its own terms and can be shown to the guardrail's owner
// as evidence rather than as a diff.
//
// The notice is stripped before this runs. See stripNotice: the intended-use
// notice is the one string in the artefact that must carry imperative language,
// and counting it is not a false positive so much as a category error.
func advisoryLanguage(_ *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatAdvisory,
		Question:  "does the briefing carry an imperative or a recommendation?",
		Rule:      "AK.3's lexicon (briefing.AdvisoryMarker) over each clause, plus this package's prose supplement",
		Soundness: Approximate,
		Direction: BothWays,
		Unit:      "briefing",
		Residue: "a lexicon cannot see advice phrased as an observation. AK.3's own comment says it is blunt by design " +
			"because its blast radius is a declared free-text field; over whole prose that argument does not hold, " +
			"and this check is reported as approximate for that reason rather than inheriting the guardrail's confidence",
	}
	res := Result{Category: cat, Denominator: 1}
	for i := range p.clauses {
		text := p.clauseText(i)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if marker, ok := briefing.AdvisoryMarker(text); ok {
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: marker,
				Detail: "AK.3 lexicon", Quote: strings.Join(strings.Fields(text), " "),
			})
			continue
		}
		if marker, ok := proseSupplement(text); ok {
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: marker,
				Detail: "prose supplement; AK.3's lexicon does not see this form", Quote: strings.Join(strings.Fields(text), " "),
			})
		}
	}
	return res
}

// proseSupplement is what `AK.3`'s lexicon does not see in whole prose. Every
// entry is here because the labelled corpus carries it, and none is speculative:
// a lexicon grown by imagination is a lexicon whose false-positive rate nobody
// measured.
func proseSupplement(s string) (string, bool) {
	low := " " + strings.ToLower(strings.Join(strings.Fields(s), " ")) + " "
	for _, m := range supplementMarkers {
		if strings.Contains(low, " "+m) {
			return m, true
		}
	}
	return "", false
}

var supplementMarkers = []string{
	// participles of the recommendation verbs. AK.3 matches on word boundaries,
	// so `suggest` does not match inside `suggesting`.
	"suggesting", "recommending", "advising", "urging", "encouraging",
	"indicating a need", "warranting", "warrants",
	// clause-initial `remember` without the `to`. AK.3 carries `remember to`.
	"remember,", "remember that", "note that", "keep in mind",
	// the passive form of an instruction, which reads as guidance to the reader.
	"should be", "may want to", "might consider", "it is advisable",
}

// aboutTheSummary counts `F831`, the finding `AT.5`'s report calls the one that
// is about the product rather than the guardrail: the model writes a document
// about a document.
func aboutTheSummary(_ *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatAboutTheSummary,
		Question:  "does the briefing write about the claim summary rather than about the member?",
		Rule:      "the prose contains `claim summary` or `the summary`",
		Soundness: Decidable,
		Unit:      "briefing",
		Residue: "a sentence about the document that names it some other way - `the record indicates`, `as listed` - " +
			"is not seen, so this is the count for these two phrases and not for the register",
	}
	res := Result{Category: cat, Denominator: 1}
	for _, phrase := range []string{"claim summary", "the summary"} {
		if idx := p.find(phrase); len(idx) > 0 {
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{Category: cat.Name, Subject: phrase, Quote: p.quote(idx[0])})
			break
		}
	}
	return res
}

// echoesPrompt counts a TOON field name or an instruction reproduced into the
// answer.
//
// **The closed list comes from the entity, which is what makes the first half
// decidable.** The names in the prompt are exactly the names in the map the
// prompt was encoded from, so `PropertyNames` is not a guess - it is the same
// list, read off the same document.
//
// The second half is the `Briefing_Data_Flags` value read back. That is
// `I-578`'s subject: a sentence of prompt instruction that lives on the entity,
// which the model treats as content. It is approximate because a paraphrase of
// the flag is still an echo and half a flag is a judgement.
//
// The third half runs only when the caller supplies the prompt, and is a
// verbatim run of eight words or more. Eight is long enough that ordinary
// English does not collide - the corpus has no false positive at eight and two
// at five.
func echoesPrompt(f *FactSet, p *reading, opt Options) Result {
	cat := Category{
		Name:     CatEchoesPrompt,
		Question: "does the briefing reproduce a field name or an instruction from the prompt?",
		Rule: "an underscored property name occurs verbatim, or a bare one in the TOON form `Name[n]`; or an identifier " +
			"from a flag value occurs; or five consecutive words of a flag value occur; or, with Options.Prompt, an eight-word run of it",
		Soundness: Decidable,
		Unit:      "briefing",
		Residue: "a paraphrased instruction is still an echo to a reader and is not seen here - M021's `Adherence ratios " +
			"are not applicable as no maintenance drugs are present` shares five of the flag's six content words in a " +
			"different order and is the model reasoning rather than quoting. With no Options.Prompt the last clause does not run",
	}
	res := Result{Category: cat, Denominator: 1}
	// (a) a property name reproduced verbatim. An underscored name is a token
	// no briefing writes by accident; a bare one is an ordinary English word,
	// so it counts only in the TOON array form `Diagnosis[3]`, which is the
	// shape `M038` reproduced.
	for _, name := range f.PropertyNames {
		hit := ""
		if strings.Contains(name, "_") && p.has(name) {
			hit = name
		} else if p.has(name + "[") {
			hit = name + "[n]"
		}
		if hit == "" {
			continue
		}
		res.Flagged = 1
		res.Findings = append(res.Findings, Finding{
			Category: cat.Name, Subject: hit,
			Detail: "an entity property name, verbatim", Quote: quoteAround(p, name),
		})
	}
	// (b) an underscored identifier that occurs inside a flag's *value*. The
	// ratio reaches this entity as `Adherence` (F839), so `Adherence_Ratio` is
	// a name the prompt carries only inside the flag sentence - and three
	// briefings write it out.
	for _, id := range identifiersIn(f.Flags) {
		if !p.has(id) {
			continue
		}
		res.Flagged = 1
		res.Findings = append(res.Findings, Finding{
			Category: cat.Name, Subject: id,
			Detail: "a property name from a Briefing_Data_Flags value, verbatim (I-578)", Quote: quoteAround(p, id),
		})
	}
	// (c) a contiguous run of a flag's own words. Contiguity is what separates
	// reproduction from agreement: `Adherence ratios are not applicable as no
	// maintenance drugs are present` shares five of the flag's six content
	// words and is the model reasoning rather than quoting, and it shares no run.
	if echo := p.flagEcho(f); len(echo) > 0 {
		for i, on := range echo {
			if !on {
				continue
			}
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: "Briefing_Data_Flags",
				Detail: "a five-word run of a flag value, verbatim (I-578)", Quote: p.quote(i),
			})
			break
		}
	}
	if strings.TrimSpace(opt.Prompt) != "" {
		if run, ok := sharedRun(opt.Prompt, p, 8); ok {
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: run, Detail: "an eight-word run of the system prompt",
			})
		}
	}
	return res
}

func quoteAround(p *reading, needle string) string {
	i := strings.Index(p.lower, strings.ToLower(needle))
	if i < 0 {
		return ""
	}
	lo, hi := i-80, i+len(needle)+80
	if lo < 0 {
		lo = 0
	}
	if hi > len(p.raw) {
		hi = len(p.raw)
	}
	return strings.Join(strings.Fields(p.raw[lo:hi]), " ")
}

func sharedRun(prompt string, p *reading, n int) (string, bool) {
	pt := normaliseTokens(prompt)
	if len(pt) < n || len(p.words) < n {
		return "", false
	}
	grams := make(map[string]bool, len(pt))
	for i := 0; i+n <= len(pt); i++ {
		grams[strings.Join(pt[i:i+n], " ")] = true
	}
	for i := 0; i+n <= len(p.words); i++ {
		parts := make([]string, n)
		for k := 0; k < n; k++ {
			parts[k] = p.words[i+k].text
		}
		g := strings.Join(parts, " ")
		if grams[g] {
			return g, true
		}
	}
	return "", false
}

// --------------------------------------------------------------------- counts

// countMismatch reads the numeric claims a briefing makes about its own volume
// and compares them with the projected counts.
//
// **It reads number words**, because the template writes them - see text.go.
// Without that this check would be silent on the control arm, and a check that
// cannot fire on the control is not controlled.
func countMismatch(f *FactSet, p *reading, _ Options) Result {
	cat := Category{
		Name:      CatCountMismatch,
		Question:  "does the briefing state a count the fact set contradicts?",
		Rule:      "a number, in digits or words, immediately followed by a counted noun this fact set carries a count for",
		Soundness: Approximate,
		Direction: UnderFlags,
		Unit:      "briefing",
		Residue: "only the immediate `<number> <noun>` form is read. A count stated at a distance - `there are 14 of these` - " +
			"is not seen, and neither is a count of something the projection does not count",
	}
	res := Result{Category: cat, Denominator: 1}
	type target struct {
		nouns []string
		want  int
		what  string
	}
	targets := []target{
		{[]string{"medical"}, f.MedicalEvents, "medical events"},
		{[]string{"pharmacy"}, f.PharmacyEvents, "pharmacy events"},
		{[]string{"conditions", "condition", "diagnoses", "diagnosis"}, len(f.Conditions), "conditions"},
	}
	countedNoun := map[string]bool{
		"events": true, "event": true, "visits": true, "visit": true,
		"encounters": true, "encounter": true, "claims": true, "claim": true,
		"entries": true, "entry": true, "records": true, "record": true,
	}
	for i := 0; i < len(p.words); i++ {
		n, used, ok := numberAt(p, i)
		if !ok || used == 0 {
			continue
		}
		j := i + used
		if j >= len(p.words) {
			break
		}
		for _, t := range targets {
			if t.want == 0 {
				continue
			}
			matched := false
			for _, noun := range t.nouns {
				if p.words[j].text != noun {
					continue
				}
				// `three medical encounters` - the qualifier then a counted
				// noun; or `three conditions` where the noun counts itself.
				if countedNoun[noun] || (j+1 < len(p.words) && countedNoun[p.words[j+1].text]) {
					matched = true
				}
			}
			if !matched || n == t.want {
				continue
			}
			res.Flagged = 1
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: t.what,
				Detail: fmt.Sprintf("the briefing says %d; the fact set says %d", n, t.want),
				Quote:  p.quote(i),
			})
		}
		i = j - 1
	}
	return res
}

// fillCountMismatch is the same machinery over the one element section 1.10.3
// calls the only one a representative can act on with no judgement at all: the
// fill count.
func fillCountMismatch(f *FactSet, p *reading, opt Options) Result {
	cat := Category{
		Name:     CatFillCountMismatch,
		Question: "does the briefing give a drug a fill count the fact set contradicts?",
		Rule: fmt.Sprintf("a `<number> fill(s)` phrase and a drug's surface form in the same clause within %d words, "+
			"where the fact set gives that drug a different count", opt.Window),
		Soundness: Approximate,
		Direction: BothWays,
		Unit:      "drug",
		Residue: "a clause naming two drugs and one count attributes the count to both, which is right for " +
			"`with two fills each` and wrong for a list where only one carries it",
	}
	res := Result{Category: cat, Denominator: len(f.Drugs)}
	if len(f.Drugs) == 0 {
		return res
	}
	ms := p.mentions(f)
	flagged := map[int]bool{}
	for i := 0; i+1 < len(p.words); i++ {
		n, used, ok := numberAt(p, i)
		if !ok || used == 0 {
			continue
		}
		j := i + used
		if j >= len(p.words) || (p.words[j].text != "fills" && p.words[j].text != "fill") {
			continue
		}
		// The nearest mention in the clause and no other. `M029` writes
		// `sertraline HCl, ... with three fills recorded, and ... buspirone
		// HCl, ... with five fills` in one clause and is correct about both;
		// a clause-wide rule attributes each count to each drug and reports
		// two errors in a sentence that has none.
		m, ok := nearestMention(p, ms, i, opt.Window)
		if !ok {
			continue
		}
		for _, di := range m.drugs {
			if flagged[di] || f.Drugs[di].Fills == 0 || f.Drugs[di].Fills == n {
				continue
			}
			flagged[di] = true
			res.Flagged++
			res.Findings = append(res.Findings, Finding{
				Category: cat.Name, Subject: f.Drugs[di].Name,
				Detail:    fmt.Sprintf("the briefing says %d fills; the fact set says %d", n, f.Drugs[di].Fills),
				Quote:     p.quote(i),
				Ambiguous: len(m.drugs) > 1,
			})
		}
	}
	return res
}

// -------------------------------------------------- what a reader has to judge

func invention(_ *FactSet, _ *reading, _ Options) Result {
	return Result{Category: Category{
		Name:      CatInvention,
		Question:  "does the briefing name a thing that is not in the fact set at all?",
		Rule:      "none; this reports zero and means unmeasured",
		Soundness: NotAutomatable,
		Unit:      "briefing",
		Residue: "the whole category. F828's `oxycontin` for `oxyCODONE-Acetaminophen` is a drug name that is not " +
			"this member's drug, and no containment test separates it from a legitimate short form - which is the same " +
			"reason the drug surface forms above accept `metformin` for `metFORMIN HCl`",
	}}
}

func mispairing(_ *FactSet, _ *reading, _ Options) Result {
	return Result{Category: Category{
		Name:      CatMispairing,
		Question:  "does the briefing attach a qualifier of one value to another?",
		Rule:      "none; this reports zero and means unmeasured",
		Soundness: NotAutomatable,
		Unit:      "briefing",
		Residue: "the whole category. F829's `(J4540) ... and (J309) ..., both of which are uncomplicated` needs the " +
			"scope of a modifier over a coordination, which is a reader's judgement",
	}}
}

func implication(_ *FactSet, _ *reading, _ Options) Result {
	return Result{Category: Category{
		Name:      CatImplication,
		Question:  "does the briefing imply a relation the fact set does not carry?",
		Rule:      "none; this reports zero and means unmeasured",
		Soundness: NotAutomatable,
		Unit:      "briefing",
		Residue: "the whole category, and section 1.7.4 says so in terms: implication is *the residue*, no check catches " +
			"it and none is proposed. F834's `for diabetes management` is true of the world and absent from the fact set",
	}}
}

// --------------------------------------------------------------------- union

// untrueUnion is `AT.5`'s *at least one statement not true of the fact set*, and
// it is a **lower bound** by construction.
//
// It is the union of the categories above that assert something false about the
// fact set. The three not-automatable categories are exactly the ways a briefing
// can be untrue that nothing here sees, so the gap between this number and a
// hand count is the size of the reader's residue rather than a defect - and a
// report that quoted this figure without saying so would be claiming the residue
// is empty.
func untrueUnion(rep *Report) Result {
	cat := Category{
		Name:      CatUntrue,
		Question:  "does the briefing carry at least one statement not true of the fact set?",
		Rule:      "the union of the maintenance, adherence-attribution, count and fill-count categories",
		Soundness: Approximate,
		Direction: UnderFlags,
		Unit:      "briefing",
		Residue: "invention, mispairing and implication are not automatable and are not in the union, so this is a " +
			"lower bound and the gap to a hand count is the reader's residue rather than an error",
	}
	res := Result{Category: cat, Denominator: 1}
	var parts []string
	for _, name := range []string{
		CatMaintenanceAbsent, CatMaintenanceDrug, CatAdherenceAttributed,
		CatCountMismatch, CatFillCountMismatch,
	} {
		if rep.Flagged(name) > 0 {
			parts = append(parts, name)
		}
	}
	if len(parts) > 0 {
		res.Flagged = 1
		res.Findings = append(res.Findings, Finding{
			Category: cat.Name,
			Detail:   "flagged by " + strings.Join(parts, ", "),
		})
	}
	return res
}
