package feedsummary

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

// The count: statements in a summary that are untrue of the fact set it was
// made from.
//
// It is the briefing scorer's discipline applied to a smaller fact set
// (jets/agentic/briefing/score): **every category says how sound it is**, so
// that a total is never one undifferentiated number. A Decidable category is a
// containment test over a closed list taken from the input; an Approximate one
// is a heuristic with a stated direction of error; a NotAutomatable one reports
// no findings and says so, rather than being left out and read as a zero.
//
// The render is the negative control: the counter is run over every rendered
// summary and must find nothing, which tests the counter as much as the render.

// Soundness grades a category.
type Soundness string

const (
	Decidable      Soundness = "decidable"
	Approximate    Soundness = "approximate"
	NotAutomatable Soundness = "not automatable"
)

// Category is one kind of untrue statement and what the count found of it.
type Category struct {
	Name      string
	Soundness Soundness
	// Rule is what the code tests; Residue is what it does not.
	Rule     string
	Residue  string
	Findings []string
}

// Count is one summary's count.
type Count struct{ Categories []Category }

// Untrue is the number of findings over every category.
func (c *Count) Untrue() int {
	n := 0
	for _, cat := range c.Categories {
		n += len(cat.Findings)
	}
	return n
}

// String lists the findings by category.
func (c *Count) String() string {
	var b strings.Builder
	for _, cat := range c.Categories {
		fmt.Fprintf(&b, "%s (%s): %d", cat.Name, cat.Soundness, len(cat.Findings))
		for _, f := range cat.Findings {
			fmt.Fprintf(&b, "\n  - %s", f)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// The category names.
const (
	CatIdentifier = "identifier not in the fact set"
	CatCause      = "cause not among the readings"
	CatDetector   = "check that did not fire"
	CatDeclarer   = "declarer misnamed"
	CatDate       = "date not in the fact set"
	CatNumber     = "number not in the fact set"
	CatOverdue    = "overdue claimed of a delivery that is not"
	CatOrder      = "readings out of their ranked order"
)

var (
	months      = `(January|February|March|April|May|June|July|August|September|October|November|December)`
	dayMonthYr  = regexp.MustCompile(`\b([0-9]{1,2}) ` + months + `,? ([0-9]{4})\b`)
	monthDayYr  = regexp.MustCompile(`\b` + months + ` ([0-9]{1,2}),? ([0-9]{4})\b`)
	isoDate     = regexp.MustCompile(`\b([0-9]{4})-([0-9]{2})-([0-9]{2})\b`)
	numberToken = regexp.MustCompile(`-?[0-9][0-9,]*(?:\.[0-9]+)?%?`)
	emailToken  = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	incToken    = regexp.MustCompile(`\binc_[A-Za-z0-9_]+`)
	snakeToken  = regexp.MustCompile(`\b[a-z]+(?:_[a-z]+)+\b`)
)

// facts is the fact set, as the counter compares against it.
type facts struct {
	identifiers []string
	causes      []string
	detectors   []string
	declarer    string
	dates       []time.Time
	numbers     []float64
}

func factsOf(in *Input) *facts {
	f := &facts{declarer: strings.ToLower(in.Declaration.DeclaredBy)}
	add := func(s string) {
		if s != "" {
			f.identifiers = append(f.identifiers, s)
		}
	}
	add(in.IncidentID)
	add(in.Feed.Table)
	add(in.Feed.Client)
	add(in.Feed.Org)
	add(in.Feed.ObjectType)
	add(in.Feed.RegisteredUnder)
	add(in.Declaration.DeclaredBy)
	day := func(t time.Time) {
		if !t.IsZero() {
			t = t.UTC()
			f.dates = append(f.dates, time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC))
		}
	}
	num := func(x float64) { f.numbers = append(f.numbers, x) }
	day(in.DetectedAt)
	day(in.Declaration.DeclaredAt)
	num(float64(in.Declaration.AgeDays))
	num(float64(in.Declaration.PeriodsPerDelivery))
	if in.Declaration.OverdueAfterDays != nil {
		num(float64(*in.Declaration.OverdueAfterDays))
	}
	num(float64(in.OtherFeeds))
	for _, x := range in.Findings {
		f.detectors = append(f.detectors, x.Detector)
		add(x.Column)
		num(float64(x.Generation))
		for _, p := range []*float64{x.Observed, x.Bound, x.Upper, x.Magnitude} {
			if p != nil {
				num(*p)
			}
		}
		day(x.ObservedPeriod)
		day(x.BoundPeriod)
		day(x.OverdueFrom)
		if !x.OverdueFrom.IsZero() && in.DetectedAt.After(x.OverdueFrom) {
			num(float64(int(in.DetectedAt.Sub(x.OverdueFrom).Hours() / 24)))
		}
	}
	for _, r := range in.Readings {
		f.causes = append(f.causes, r.Cause)
		num(float64(r.Rank))
		num(r.Confidence)
		num(float64(r.Supporting))
		num(float64(r.Contradicting))
	}
	for _, b := range in.Bursts {
		f.detectors = append(f.detectors, b.Detector)
		day(b.Day)
		num(float64(b.Deliveries))
		num(float64(b.Unraised))
		num(float64(len(b.Others)))
		if len(b.Others) > othersShown {
			num(float64(len(b.Others) - othersShown))
		}
		for _, o := range b.Others {
			add(o)
		}
	}
	for _, d := range f.dates {
		num(float64(d.Year()))
		num(float64(d.Day()))
	}
	// Longest first, so that removing one never leaves a fragment of a longer.
	sort.Slice(f.identifiers, func(i, j int) bool { return len(f.identifiers[i]) > len(f.identifiers[j]) })
	return f
}

func (f *facts) hasDate(t time.Time) bool { return slices.ContainsFunc(f.dates, t.Equal) }

// hasNumber reports whether a number written with the given decimals, possibly
// as a percentage, is a fact rounded to what was written.
func (f *facts) hasNumber(written float64, decimals int, percent bool) bool {
	scale := math.Pow(10, float64(decimals))
	for _, x := range f.numbers {
		cands := []float64{x}
		if percent || decimals > 0 {
			cands = append(cands, x*100)
		}
		for _, c := range cands {
			// Either rounding a writer may have used: half away from zero, or
			// the shortest decimal strconv prints, which rounds 97.25 to 97.2.
			if math.Round(c*scale) == math.Round(written*scale) ||
				strconv.FormatFloat(c, 'f', decimals, 64) == strconv.FormatFloat(written, 'f', decimals, 64) {
				return true
			}
		}
	}
	return false
}

// CountUntrue counts the statements of a summary that are untrue of the input.
func CountUntrue(text string, in *Input) *Count {
	f := factsOf(in)
	c := &Count{}
	cat := func(name string, s Soundness, rule, residue string, found []string) {
		c.Categories = append(c.Categories, Category{Name: name, Soundness: s, Rule: rule, Residue: residue,
			Findings: found})
	}

	var found []string
	for _, id := range incToken.FindAllString(text, -1) {
		if !slices.Contains(f.identifiers, id) {
			found = append(found, id)
		}
	}
	cat(CatIdentifier, Decidable, "every inc_ token is this incident or one a burst names",
		"a feed, client or column named wrongly is not caught unless it is an inc_ token", found)

	lower := strings.ToLower(text)
	found = nil
	allCauses := causes
	for _, tok := range snakeToken.FindAllString(lower, -1) {
		if slices.Contains(allCauses, tok) && !slices.Contains(f.causes, tok) {
			found = append(found, tok)
		}
	}
	cat(CatCause, Decidable, "a cause class written as its name is one of the incident's readings",
		"a cause described in other words is not caught", found)

	found = nil
	for _, tok := range snakeToken.FindAllString(lower, -1) {
		if slices.Contains(feedprofile.Detectors, tok) && !slices.Contains(f.detectors, tok) {
			found = append(found, tok)
		}
	}
	cat(CatDetector, Decidable, "a check written as its name is one that fired on this delivery or its burst",
		"a check described in other words is not caught", found)

	found = nil
	for _, e := range emailToken.FindAllString(text, -1) {
		if strings.ToLower(e) != f.declarer {
			found = append(found, e)
		}
	}
	cat(CatDeclarer, Decidable, "every address written is the declaration's author",
		"a declarer named without an address is not caught", found)

	// Dates, then numbers in what is left once dates and identifiers are gone.
	rest := text
	for _, id := range f.identifiers {
		rest = strings.ReplaceAll(rest, id, " ")
	}
	// What the two categories above judged is not judged again as a number.
	rest = incToken.ReplaceAllString(rest, " ")
	rest = emailToken.ReplaceAllString(rest, " ")
	found = nil
	takeDates := func(re *regexp.Regexp, parse func([]string) (time.Time, bool)) {
		for _, m := range re.FindAllStringSubmatch(rest, -1) {
			t, ok := parse(m)
			if !ok || !f.hasDate(t) {
				found = append(found, m[0])
			}
		}
		rest = re.ReplaceAllString(rest, " ")
	}
	monthOf := func(s string) time.Month {
		for m := time.January; m <= time.December; m++ {
			if m.String() == s {
				return m
			}
		}
		return 0
	}
	ymd := func(y, m, d string, mon time.Month) (time.Time, bool) {
		yy, e1 := strconv.Atoi(y)
		dd, e2 := strconv.Atoi(d)
		if mon == 0 {
			mm, e3 := strconv.Atoi(m)
			if e3 != nil {
				return time.Time{}, false
			}
			mon = time.Month(mm)
		}
		if e1 != nil || e2 != nil {
			return time.Time{}, false
		}
		return time.Date(yy, mon, dd, 0, 0, 0, 0, time.UTC), true
	}
	takeDates(isoDate, func(m []string) (time.Time, bool) { return ymd(m[1], m[2], m[3], 0) })
	takeDates(dayMonthYr, func(m []string) (time.Time, bool) { return ymd(m[3], "", m[1], monthOf(m[2])) })
	takeDates(monthDayYr, func(m []string) (time.Time, bool) { return ymd(m[3], "", m[2], monthOf(m[1])) })
	cat(CatDate, Decidable, "a full date written is a date of the input, to the day",
		"a month or a weekday written without a full date is not checked", found)

	found = nil
	for _, tok := range numberToken.FindAllString(rest, -1) {
		percent := strings.HasSuffix(tok, "%")
		s := strings.TrimSuffix(strings.ReplaceAll(tok, ",", ""), "%")
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}
		decimals := 0
		if i := strings.IndexByte(s, '.'); i >= 0 {
			decimals = len(s) - i - 1
		}
		if !f.hasNumber(x, decimals, percent) {
			found = append(found, tok)
		}
	}
	cat(CatNumber, Approximate,
		"every number written in digits equals a number of the input, rounded to the digits written",
		"undercounts: a number spelled in words is not read, and a wrong number that equals another fact passes",
		found)

	found = nil
	overdue := slices.Contains(f.detectors, feedprofile.FeedOverdue)
	if !overdue && strings.Contains(lower, "overdue") {
		found = append(found, "overdue")
	}
	cat(CatOverdue, Approximate, "the word overdue appears only when the overdue check fired",
		"overcounts a sentence that says the feed is not overdue", found)

	cat(CatOrder, NotAutomatable, "none",
		"whether the readings are given in their ranked order needs a reader; no finding is reported", nil)
	return c
}
