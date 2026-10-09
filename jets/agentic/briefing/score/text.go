package score

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/agentic/briefing/prose"
)

// This file is the reading layer: it turns a briefing into words with positions
// and clauses, and it is where the two decisions that keep the negative control
// clean live.
//
// **A clause is the attribution boundary.** See the package comment: the
// template's semicolons are what stop a maintenance claim about one drug
// reaching the next drug in the list, and no width of window can do that job.
//
// **A number word is a number.** The template writes *"Sixty-two medical
// visits"*, so a count check that read only digits would see no claim where the
// template makes one - and would then be unable to find the template's errors if
// it ever had any, which is exactly the property that makes it a control.

// reading is a briefing read once: words with their offsets, and the clause each
// word belongs to.
type reading struct {
	raw string
	// lower is raw lower-cased, kept for the phrase containment tests that do
	// not need positions.
	lower string
	words []word
	// clauses are spans over raw, in order.
	clauses []span
}

type word struct {
	// text is lower case with the punctuation stripped, which is what a
	// containment test compares.
	text string
	// start and end are byte offsets into raw, so a finding can quote.
	start, end int
	// clause is the index into prose.clauses.
	clause int
}

type span struct {
	start, end int
}

// stripNotice removes the intended-use notice from a rendered briefing.
//
// Two routes, because the two arms deliver it differently. `prose.SplitNotice`
// splits at the renderer's own horizontal rule, which is what the template arm
// carries; and the entity's `Briefing_Disclaimer` value is removed if it is
// still there, which covers a caller that assembled the artefact some other way.
// **The notice must go before anything counts advisory language**: it is the one
// string in the artefact that must carry imperative language, and A section 8.3
// says so in terms.
func stripNotice(f *FactSet, s string) string {
	_, body := prose.SplitNotice(s)
	if f != nil && strings.TrimSpace(f.Disclaimer) != "" {
		body = strings.ReplaceAll(body, strings.TrimSpace(f.Disclaimer), " ")
	}
	return unwrap(body)
}

// unwrap joins a hard-wrapped briefing back into paragraphs, and it is here
// because the negative control found what happens without it.
//
// `prose.Render` wraps at 78 columns, so a sentence of the template arm carries
// line breaks at arbitrary word boundaries. Any check that is **position
// sensitive inside a clause** then reads a mid-sentence line break as the start
// of a new one. `AK.3`'s imperative family is exactly that - a bare verb is
// guidance only when it opens a clause, which its own comment explains at length
// - and the first run of this package over the template arm flagged `M030` on
//
//	Most recent
//	contact: Urgent Care Facility, 1 December.
//
// where `contact` opens a line and opens nothing else. **A false positive on the
// control arm, produced by the line width.**
//
// A blank line is a paragraph break and is kept as one; a single newline becomes
// a space. So the rule is: **unwrap before you count anything that cares where a
// clause begins**, and this package does it once, at the entrance, rather than
// asking each check to remember.
func unwrap(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	paras := strings.Split(s, "\n\n")
	for i, p := range paras {
		paras[i] = strings.Join(strings.Fields(p), " ")
	}
	return strings.Join(paras, "\n\n")
}

// parseProse reads a briefing into words and clauses.
func parseProse(raw string) *reading {
	p := &reading{raw: raw, lower: strings.ToLower(raw)}
	// Clause boundaries. A full stop counts only when it ends a word rather than
	// sitting inside one, so `0.00` and `J45.40` do not split a sentence in two
	// and produce a clause nobody wrote.
	bounds := []int{0}
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case ';', '\n', '\r', '!', '?':
			bounds = append(bounds, i+1)
		case '.':
			if i+1 >= len(raw) || raw[i+1] == ' ' || raw[i+1] == '\n' || raw[i+1] == '\r' || raw[i+1] == '\t' {
				bounds = append(bounds, i+1)
			}
		}
	}
	bounds = append(bounds, len(raw))
	for i := 0; i+1 < len(bounds); i++ {
		if bounds[i+1] > bounds[i] {
			p.clauses = append(p.clauses, span{start: bounds[i], end: bounds[i+1]})
		}
	}
	if len(p.clauses) == 0 {
		p.clauses = append(p.clauses, span{start: 0, end: len(raw)})
	}

	ci := 0
	i := 0
	for i < len(raw) {
		if !isWordByte(raw[i]) {
			i++
			continue
		}
		j := i
		for j < len(raw) && isWordByte(raw[j]) {
			j++
		}
		for ci+1 < len(p.clauses) && i >= p.clauses[ci].end {
			ci++
		}
		p.words = append(p.words, word{text: strings.ToLower(raw[i:j]), start: i, end: j, clause: ci})
		i = j
	}
	return p
}

// isWordByte is a word character for the purposes of splitting a briefing into
// words. Digits are in, so that `0.00%` reads as `0` `00` and a count check can
// see a number; the apostrophe is out, so `member's` reads as `member`.
func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// has reports a phrase anywhere in the briefing, case-insensitively. It is the
// blunt test and is used only where the category says so.
func (p *reading) has(phrase string) bool {
	return strings.Contains(p.lower, strings.ToLower(phrase))
}

// hasWord reports a word - not a substring. This is the distinction that moves
// `AT.5`'s omission count: `arthritis` is inside `osteoarthritis` and is not a
// word of it.
func (p *reading) hasWord(w string) bool {
	w = strings.ToLower(w)
	for i := range p.words {
		if p.words[i].text == w || singular(p.words[i].text) == singular(w) {
			return true
		}
	}
	return false
}

// find returns the word indices where a phrase's words occur as a run.
func (p *reading) find(phrase string) []int {
	want := normaliseTokens(phrase)
	if len(want) == 0 {
		return nil
	}
	var out []int
	for i := 0; i+len(want) <= len(p.words); i++ {
		ok := true
		for k, t := range want {
			if p.words[i+k].text != t {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// quote returns the clause a word sits in, trimmed, which is what a finding
// shows a reader.
func (p *reading) quote(wordIndex int) string {
	if wordIndex < 0 || wordIndex >= len(p.words) {
		return ""
	}
	c := p.clauses[p.words[wordIndex].clause]
	return strings.TrimSpace(strings.Join(strings.Fields(p.raw[c.start:c.end]), " "))
}

// clauseText is one clause as written.
func (p *reading) clauseText(i int) string {
	if i < 0 || i >= len(p.clauses) {
		return ""
	}
	return strings.TrimSpace(p.raw[p.clauses[i].start:p.clauses[i].end])
}

// A mention is one occurrence of a drug's surface form in the briefing.
type mention struct {
	// drugs are the indices into FactSet.Drugs that this form answers to. More
	// than one is an ambiguous mention.
	drugs []int
	// form is the surface form that matched.
	form string
	// at and through are word indices, inclusive.
	at, through int
}

// mentions finds every drug the briefing names, longest form first so that a
// specific name is not consumed by a fragment of it.
func (p *reading) mentions(f *FactSet) []mention {
	var out []mention
	taken := make([]bool, len(p.words))
	for _, form := range f.forms {
		want := strings.Fields(form)
		for i := 0; i+len(want) <= len(p.words); i++ {
			if taken[i] {
				continue
			}
			ok := true
			for k, t := range want {
				if taken[i+k] || p.words[i+k].text != t {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			for k := range want {
				taken[i+k] = true
			}
			out = append(out, mention{drugs: f.surfaces[form], form: form, at: i, through: i + len(want) - 1})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at < out[j].at })
	return out
}

// distance is how many words separate a mention from an anchor, zero when the
// anchor is inside the mention.
func (p *reading) distance(m mention, anchor int) int {
	if anchor < m.at {
		return m.at - anchor
	}
	if anchor > m.through {
		return anchor - m.through
	}
	return 0
}

// claimant returns the anchor a mention is attributed to, or -1.
//
// # Nearest wins, and an appositive claims only its own noun
//
// The clause bound is what keeps the template arm clean, and the labelled corpus
// showed it is not sufficient on its own. `M029` writes a **correct** sentence:
//
//	adherence is noted as zero for sertraline HCl, a maintenance medication,
//	with three fills recorded, and no adherence noted for buspirone HCl, a
//	non-maintenance medication, with five fills
//
// It is one clause - the model coordinates with commas and `and` rather than
// with a semicolon - so a clause-wide rule attributes *sertraline's* maintenance
// label to buspirone and reports a misstatement in a briefing that has none.
//
// Two rules together fix it and neither is about this sentence:
//
//   - **Nearest wins.** A mention is claimed by the closest anchor in its
//     clause, not by every anchor in it. Buspirone's closest `maintenance` is
//     the one inside `non-maintenance`, which `negated` then cancels.
//   - **An appositive claims only its own noun.** `sertraline HCl, a maintenance
//     medication` is a noun phrase in apposition: the anchor sits under an
//     article, and immediately before the article is the drug it describes. Such
//     an anchor claims that drug and nothing else. This is also the template's
//     own construction - `Naproxen Sodium ER, a maintenance medication, five
//     fills` - so the control arm is safe under it by structure rather than by
//     the semicolon that happens to follow.
//
// The list constructions are untouched, and that is the test of the rule: `for
// maintenance medications: X and Y`, `on maintenance medications including X, Y,
// Z` and `maintenance medications like <sixteen drugs>` all put the anchor after
// a preposition or a participle rather than under an article, so none is an
// appositive and each claims its whole list.
func (p *reading) claimant(m mention, anchors []int, window int, ends map[int]bool) int {
	best, bestD := -1, window+1
	for _, a := range anchors {
		if p.words[a].clause != p.words[m.at].clause {
			continue
		}
		if owner, ok := p.appositiveOwner(a, ends); ok {
			// This anchor describes one noun. It claims that noun and no other.
			if owner < m.at || owner > m.through {
				continue
			}
		}
		if d := p.distance(m, a); d < bestD {
			best, bestD = a, d
		}
	}
	return best
}

// appositiveOwner reports the word index inside the mention an anchor is in
// apposition to, when it is. `ends` marks the last word of every drug mention.
func (p *reading) appositiveOwner(anchor int, ends map[int]bool) (int, bool) {
	if anchor < 2 {
		return 0, false
	}
	art := p.words[anchor-1].text
	if art != "a" && art != "an" && art != "the" {
		return 0, false
	}
	c := p.words[anchor].clause
	for i := anchor - 2; i >= 0 && i >= anchor-3; i-- {
		if p.words[i].clause != c {
			return 0, false
		}
		if ends[i] {
			return i, true
		}
	}
	return 0, false
}

// mentionEnds marks the last word index of every mention, which is what
// appositiveOwner looks back for.
func mentionEnds(ms []mention) map[int]bool {
	out := make(map[int]bool, len(ms))
	for _, m := range ms {
		out[m.through] = true
	}
	return out
}

// negated reports whether a word is negated by something within three words
// before it in the same clause.
//
// **Three is the reach of the negations a briefing actually writes** - *no
// maintenance medications*, *not a maintenance drug*, *none of the maintenance
// drugs*, *without any maintenance therapy*. It is deliberately short: a wider
// reach starts cancelling a negation that belongs to a different noun, and the
// error that produces is silent, where a missed negation shows up as a finding a
// reader can throw out.
//
// **`non` is in the list because the labelled corpus made it necessary**, and
// the case is worth stating because it is the one place a negation is a prefix
// rather than a word. `M021` writes *"a medication, Amoxicillin, for a
// non-maintenance indication"* - correct about a member with no maintenance
// drug - and the tokeniser splits the hyphen, so without `non` the sentence
// reads as an assertion that the drug is a maintenance drug. That member is
// `AT.5`'s *5 of 6* rather than *6 of 6*, and this is why.
func (p *reading) negated(at int) bool {
	c := p.words[at].clause
	for i := at - 1; i >= 0 && i >= at-3; i-- {
		if p.words[i].clause != c {
			break
		}
		switch p.words[i].text {
		case "no", "not", "none", "non", "without", "neither", "nor", "never", "any":
			return true
		}
	}
	return false
}

// flagEcho marks the words of the briefing that reproduce a Briefing_Data_Flags
// value, so that a check can decline to read a quoted instruction as a claim.
//
// **This is the one place the fact set makes an exclusion decidable.** The flag
// this pipeline asserts is *"Adherence_Ratio is applicable only for maintenance
// drugs"*, and it carries the word `maintenance`. A briefing that reads it back
// - which five of the twenty-two do - has said the word without saying anything
// about this member's drugs, and a containment test that could not tell the
// difference would count the model's compliance as its error. The flag values
// are in the entity, so the run of words to exclude is read rather than guessed.
//
// A run of five or more consecutive words shared with a flag is an echo. Five is
// long enough that no ordinary sentence in the corpus collides with the flag and
// short enough to catch a paraphrase that keeps the tail - *"applies only to
// maintenance drugs"*.
func (p *reading) flagEcho(f *FactSet) []bool {
	echo := make([]bool, len(p.words))
	if f == nil {
		return echo
	}
	const run = 5
	for _, flag := range f.Flags {
		ft := normaliseTokens(flag)
		if len(ft) < run {
			continue
		}
		grams := map[string]int{}
		for i := 0; i+run <= len(ft); i++ {
			grams[strings.Join(ft[i:i+run], " ")] = i
		}
		for i := 0; i+run <= len(p.words); i++ {
			parts := make([]string, run)
			for k := 0; k < run; k++ {
				parts[k] = p.words[i+k].text
			}
			if _, ok := grams[strings.Join(parts, " ")]; ok {
				for k := 0; k < run; k++ {
					echo[i+k] = true
				}
			}
		}
	}
	return echo
}

// numberAt reads a number at a word index, in digits or in English words, and
// returns how many words it consumed.
//
// It reads what a briefing writes and no more: units, teens, tens, hyphenated
// compounds and `hundred`. There is no attempt at a general numeral parser -
// `Sixty-two` is the shape the template emits and `62` is the shape the model
// emits, and a briefing that says *several* is making no numeric claim to check.
func numberAt(p *reading, i int) (int, int, bool) {
	if i >= len(p.words) {
		return 0, 0, false
	}
	if n, err := strconv.Atoi(p.words[i].text); err == nil {
		return n, 1, true
	}
	total, used, any := 0, 0, false
	for i+used < len(p.words) && used < 4 {
		t := p.words[i+used].text
		switch {
		case numberWords[t] > 0 || t == "zero":
			total += numberWords[t]
			used++
			any = true
		case t == "hundred" && any:
			if total == 0 {
				total = 1
			}
			total *= 100
			used++
		case t == "and" && any && used > 0:
			// `one hundred and two`. Consume it only if a number follows.
			if i+used+1 < len(p.words) && numberWords[p.words[i+used+1].text] > 0 {
				used++
				continue
			}
			return total, used, any
		default:
			return total, used, any
		}
	}
	return total, used, any
}

var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7,
	"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18,
	"nineteen": 19, "twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

// normaliseTokens lower-cases a string and splits it on everything that is not a
// word character. `oxyCODONE-Acetaminophen` becomes `oxycodone acetaminophen`.
func normaliseTokens(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		if !isWordByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isWordByte(s[j]) {
			j++
		}
		out = append(out, strings.ToLower(s[i:j]))
		i = j
	}
	return out
}

// distinctiveTokens are the words of a description that a briefing naming it
// would have to use: longer than three characters and not a stop word.
//
// The stop list is the clinical-description vocabulary rather than an English
// one. `unspecified`, `initial encounter`, `not elsewhere classified` and `other`
// appear in a large fraction of ICD descriptions and carry no information about
// which condition is meant, so counting them as content words makes two
// unrelated conditions look alike.
//
// A description with no distinctive token at all falls back to every token it
// has, so that `Fibromyalgia` and `Obesity, unspecified` are both checkable.
func distinctiveTokens(s string) []string {
	toks := normaliseTokens(s)
	var out []string
	for _, t := range toks {
		if len(t) <= 3 || conditionStopWords[t] {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		out = toks
	}
	return out
}

var conditionStopWords = map[string]bool{
	"with": true, "without": true, "unspecified": true, "other": true,
	"specified": true, "elsewhere": true, "classified": true, "initial": true,
	"encounter": true, "adult": true, "current": true, "term": true, "long": true,
	"episode": true, "single": true, "states": true, "type": true, "due": true,
	"from": true, "than": true, "such": true, "and": true, "the": true,
}

// singular strips a trailing `s` from a word longer than four characters, which
// is the only inflection a condition description and a briefing disagree on in
// this corpus - `knees` for `knee`, `disorders` for `disorder`. It is
// deliberately not a stemmer: a stemmer conflates words a clinical vocabulary
// keeps apart, and the failure would be a condition passing as named.
func singular(w string) string {
	if len(w) > 4 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}

func sortByLengthDesc(s []string) {
	sort.SliceStable(s, func(i, j int) bool {
		if len(s[i]) != len(s[j]) {
			return len(s[i]) > len(s[j])
		}
		return s[i] < s[j]
	})
}

// values, nodes, firstText, firstInt, scalar and asInt are the small entity
// readers this package needs. They are the prose renderer's, restated rather
// than exported from it: those are unexported helpers of a *renderer*, and
// exporting them would make a rendering package's internals part of a scoring
// package's contract. The selector language, which is the part with a rule worth
// holding in one place, *is* shared - see facts.go, which resolves every path
// through `briefing.Selector`.
func values(node any, sel briefing.Selector) []any {
	located := sel.Resolve(node)
	out := make([]any, 0, len(located))
	for _, l := range located {
		if l.Value != nil {
			out = append(out, l.Value)
		}
	}
	return out
}

// nodes returns the object-valued results of a selector: the event maps. A
// scalar among them is dropped rather than rendered, because `extractAsEntity`
// writes a map for every object property and a scalar there is not an event.
func nodes(node any, sel briefing.Selector) []map[string]any {
	var out []map[string]any
	for _, v := range values(node, sel) {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func firstText(node any, sel briefing.Selector) string {
	for _, v := range values(node, sel) {
		return scalar(v)
	}
	return ""
}

func firstInt(node any, sel briefing.Selector) (int, bool) {
	for _, v := range values(node, sel) {
		return asInt(v)
	}
	return 0, false
}

// scalar and asInt are this package's own, and they are **not**
// [briefing.ValueText] and [briefing.AsInt].
//
// `AY.5` unified the three copies `I-643` recorded and left these two, which
// makes this the place to say why rather than leave the next reader to find a
// fourth copy and assume it was missed. **The default arm is the difference**:
// a value that is neither a scalar nor a time reads as `""` here and as
// `fmt.Sprintf("%v", x)` there, so a map reaching a word count contributes
// nothing rather than contributing `map[...]`. This package counts words in an
// artefact and attributes them to drugs; a change that put a Go value's default
// formatting into that stream would move findings, and a scorer's findings
// should move on a measurement rather than on a refactor whose subject was
// duplication elsewhere.
//
// The narrower type switches are the same story with a smaller consequence: a
// fact set comes from a decoded document, where every number is a float64.
func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		return x.Format("2006-01-02")
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case bool:
		return strconv.FormatBool(x)
	default:
		return ""
	}
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case uint:
		return int(x), true
	case uint64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	}
	return 0, false
}

// nearestMention is the drug mention closest to a word in the same clause, or
// false when none is within window. It is the attribution a numeric phrase
// needs: a count belongs to one drug, and picking every drug in the clause turns
// a correct coordination into two findings - see fillCountMismatch.
func nearestMention(p *reading, ms []mention, at, window int) (mention, bool) {
	best, bestD, ok := mention{}, window+1, false
	for _, m := range ms {
		if p.words[m.at].clause != p.words[at].clause {
			continue
		}
		if d := p.distance(m, at); d < bestD {
			best, bestD, ok = m, d, true
		}
	}
	return best, ok
}

// identifiersIn returns the underscored identifiers occurring inside a set of
// strings - the property names a flag sentence names in passing. They are the
// closed list CatEchoesPrompt's second clause runs over.
func identifiersIn(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		for _, field := range strings.Fields(s) {
			field = strings.Trim(field, `.,;:()"'`)
			if !strings.Contains(field, "_") || len(field) < 4 {
				continue
			}
			if !seen[field] {
				seen[field] = true
				out = append(out, field)
			}
		}
	}
	return out
}
