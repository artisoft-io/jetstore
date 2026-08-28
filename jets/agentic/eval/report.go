// Package eval measures whether the copilot works, under the reporting rules
// decision 13 settled.
//
// **The rules are constraints on what may be said, not formatting.** They exist
// because the corpus is skewed — map_record and partition_writer are 83% of 458
// transformation instances, and nine operators have four instances or fewer —
// so the obvious summary of a run would report the health of two operators and
// conceal thirteen. Decision 13, accepted 2026-08-18:
//
//   - **Per operator, with denominators visible.** A rate without its
//     denominator is unreadable when the denominators differ by two orders of
//     magnitude.
//   - **No aggregate figure at all**, not even beside the breakdown. It is the
//     number that would be quoted.
//   - **Cases, not rates, below five instances.** "Three of the four analyze
//     instances compile" is honest; "75% on analyze" invites a comparison the
//     sample cannot support.
//   - **An operator with no live instances reports untested**, not zero. Zero
//     is a measurement; untested is the absence of one, and clustering is the
//     case (I-15 deleted its only instance).
//   - **Every figure records which side of the gap-19 transition it came
//     from**, because templates absorb the fat head and leave the model the
//     thin tail: a rate from before and a rate from after are two different
//     populations (Phase 0 plan §5.3.7).
//
// These are enforced here rather than left to whoever writes the summary. A
// reporting discipline that depends on discipline is not one.
//
// **One rule was added at P.1, 2026-08-27, and it was already being enforced
// next door.** A report must name the model that answered and the population
// its cases came from. Decision 13's own argument is that two populations
// reported as one is the failure to prevent, and plan §4 raised provenance as
// the design constraint for the golden-case library — but this type could be
// published saying neither, while ToolReport in toolcall.go has required both
// since J.2 built it under the same decision. Nothing had called this one
// (F32), so the asymmetry cost nothing until it did.
package eval

import (
	"fmt"
	"sort"
	"strings"
)

// RateThreshold is the number of cases below which an operator reports cases
// rather than a rate. Five, so "four of four" is still cases: decision 13 says
// four or fewer, and the boundary is inclusive.
const RateThreshold = 5

// Era records which side of the gap-19 template transition a figure came from.
// It is mandatory: a report that cannot say is a report that will be compared
// against one from the other side.
type Era string

const (
	// EraPreTemplates is before gap 19's template mechanism exists. The model
	// is asked for everything, including the repetitive head.
	EraPreTemplates Era = "pre_templates"
	// EraPostTemplates is after it. Templates handle the head deterministically,
	// so what reaches the model is the thin tail — the product improves while
	// measured task difficulty rises.
	EraPostTemplates Era = "post_templates"
)

// OperatorResult is one operator's outcome. Attempted and Passed are raw
// counts; everything a reader should see is derived from them here rather than
// by the reader.
type OperatorResult struct {
	Operator  string
	Attempted int
	Passed    int
	// LiveInstances is how many instances the corpus holds for this operator.
	// Zero means untested rather than failed, and the distinction is the whole
	// of clustering's entry.
	LiveInstances int
	// NotRun says why an operator with live instances was never attempted,
	// when the reason is the harness rather than the split.
	//
	// **Added at P.1, 2026-08-27, because the first run needed it for every
	// operator.** "not run — 241 live instances available" reads as *the split
	// held none out*, which is a fact about the split and nobody's fault. It is
	// the wrong sentence when the split held out plenty and the harness refused
	// them — on this contract, because the operator's schema does not fit the
	// context window (F112). Two different situations rendering in one line is
	// how a reader concludes the harness is fine and the split was unlucky.
	NotRun string
}

// Untested reports whether the corpus has nothing to measure this operator
// with. An operator with instances but no attempts is *not* untested — that is
// a harness that did not run, and saying "untested" would hide it.
func (r OperatorResult) Untested() bool { return r.LiveInstances == 0 }

// ReportsRate says whether this operator has enough cases for a percentage to
// mean anything.
func (r OperatorResult) ReportsRate() bool {
	return !r.Untested() && r.Attempted >= RateThreshold
}

// Line renders one operator the way it may be published.
func (r OperatorResult) Line() string {
	switch {
	case r.Untested():
		return fmt.Sprintf("%-18s untested — no live instances in the corpus", r.Operator)
	case r.Attempted == 0 && r.NotRun != "":
		return fmt.Sprintf("%-18s not run — %s (%d live instances)", r.Operator, r.NotRun, r.LiveInstances)
	case r.Attempted == 0:
		return fmt.Sprintf("%-18s not run — %d live instances available", r.Operator, r.LiveInstances)
	case !r.ReportsRate():
		return fmt.Sprintf("%-18s %d of %d cases compiled (too few for a rate)",
			r.Operator, r.Passed, r.Attempted)
	default:
		return fmt.Sprintf("%-18s %d of %d cases compiled (%.0f%%)",
			r.Operator, r.Passed, r.Attempted, 100*float64(r.Passed)/float64(r.Attempted))
	}
}

// Report is a whole measurement.
//
// **There is deliberately no total, and its absence is the design.** Decision 13
// bans an aggregate compile-pass figure outright — not "discouraged", and not
// "acceptable beside the breakdown". Two operators are 83% of the corpus, so
// any total is those two wearing a coat, and it is the number that would
// travel. R-9 makes the related point that a compile-pass percentage arrives
// somewhere as a quality claim; the defence is not having one to quote.
//
// A caller that genuinely needs one number for a specific purpose should
// compute it from Operators at the call site, where the choice is visible in
// the diff and someone can object to it.
type Report struct {
	Era Era
	// Model is what the server said answered, which need not be what was asked
	// for: a tag can resolve to a different build. Mandatory.
	//
	// **This field and CaseSource are P.1's provenance decision, 2026-08-27,
	// and where they are is the decision.** Plan §4 posed the question as
	// *"`Case` gains a provenance field, or the two libraries stay apart"*. The
	// libraries stay apart — I-115 and I-136 settled that a repair case and a
	// synthetic runtime case differ in *what a case contains* and not only in
	// where it came from, so a label on `Case` would advertise a comparison
	// that does not exist. What was actually missing is one layer up: a
	// compile-pass report could be published without naming the model that
	// produced it or the population it measured, while ToolReport — the sibling
	// type in this package, written for J.2 under the same decision — has
	// required both since it was built. The asymmetry was invisible because
	// nothing called this one (F32).
	Model string
	// CaseSource says where the cases came from. Mandatory, for the reason the
	// field exists on ToolReport: a reader who assumes a corpus will over-read
	// figures drawn from anything else, and two populations reported as one is
	// decision 13's own failure mode arriving through the door it left open.
	CaseSource string
	Operators  []OperatorResult
	// HeldOutFiles names the files the cases came from, so a reader can tell
	// whether a figure was measured against material the few-shot pool also
	// saw. File-level is decision 13's split; instance-level would leak,
	// because instances within one file share channel names and house idiom.
	HeldOutFiles []string
}

// Validate refuses a report that cannot be published honestly. It is called by
// String, so an invalid report cannot be rendered by accident.
func (r *Report) Validate() error {
	if r.Era == "" {
		return fmt.Errorf("eval: the report does not say which side of the gap-19 transition it " +
			"came from; a figure without that cannot be compared with any other figure")
	}
	if r.Era != EraPreTemplates && r.Era != EraPostTemplates {
		return fmt.Errorf("eval: %q is not an era", r.Era)
	}
	if r.Model == "" {
		return fmt.Errorf("eval: the report does not name the model that answered")
	}
	if r.CaseSource == "" {
		return fmt.Errorf("eval: the report does not say where its cases came from; a compile-pass " +
			"figure whose population is unstated will be read as one drawn from the live corpus")
	}
	if len(r.Operators) == 0 {
		return fmt.Errorf("eval: the report covers no operators")
	}
	for _, op := range r.Operators {
		if op.Passed > op.Attempted {
			return fmt.Errorf("eval: %s passed %d of %d attempted", op.Operator, op.Passed, op.Attempted)
		}
		if op.Untested() && op.Attempted > 0 {
			return fmt.Errorf("eval: %s has no live instances but %d attempts; one of the two is wrong",
				op.Operator, op.Attempted)
		}
	}
	if len(r.HeldOutFiles) == 0 {
		return fmt.Errorf("eval: the report names no held-out files, so nobody can tell what it was " +
			"measured against")
	}
	return nil
}

// String renders the report. It returns the validation error as text rather
// than an empty string, so a broken report is loud.
func (r *Report) String() string {
	if err := r.Validate(); err != nil {
		return "INVALID REPORT: " + err.Error()
	}
	ops := make([]OperatorResult, len(r.Operators))
	copy(ops, r.Operators)
	sort.Slice(ops, func(i, j int) bool { return ops[i].Operator < ops[j].Operator })

	var b strings.Builder
	fmt.Fprintf(&b, "compile-pass by operator (%s), model %s\n", r.Era, r.Model)
	fmt.Fprintf(&b, "cases: %s\n", r.CaseSource)
	fmt.Fprintf(&b, "held out: %s\n\n", strings.Join(r.HeldOutFiles, ", "))
	for _, op := range ops {
		b.WriteString("  " + op.Line() + "\n")
	}
	b.WriteString("\nNo aggregate figure is published: two operators are 83% of the corpus, " +
		"so a total reports their health and conceals the rest (decision 13).\n")
	return b.String()
}
