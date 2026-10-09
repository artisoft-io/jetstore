package eval

import (
	"fmt"
	"sort"
	"strings"
)

// Tool-call conformance: whether a local model can drive the catalogue (J.2,
// criterion 27), under the same reporting rules decision 13 settled for
// compile-pass.
//
// **The rules carry over unchanged, and one is added.** Per tool with
// denominators visible; no aggregate; cases rather than rates below
// RateThreshold. What compile-pass calls Era, this calls Mechanism, and for
// the same reason: a figure that cannot say how the model was asked will be
// compared with one produced the other way. R-14's closing note is explicit
// that the residue to state is which mechanism was measured, not only the
// rates.
//
// **Three numbers per tool, because "drives the catalogue" is three questions.**
// Selecting the right tool, populating its arguments well enough to satisfy the
// schema the client was served, and not reaching for the tool when something
// else was asked. A single pass rate would hide the third entirely — a model
// that calls describe_domain_class for everything scores well on
// describe_domain_class.

// Mechanism records how the model was asked to produce a tool call. Mandatory:
// the two are not interchangeable and their figures are not comparable.
type Mechanism string

const (
	// MechanismNativeTools is Ollama's `tools` request field and the
	// `tool_calls` response field — what an MCP client uses, so it is the
	// faithful measurement of A§10's "drivable by both".
	MechanismNativeTools Mechanism = "native_tools"
	// MechanismStructuredJSON is a `{tool, args}` object generated against a
	// JSON Schema in `format` and validated client-side. Cheaper to build, and
	// a strictly weaker claim: it measures selection and population without the
	// model's own tool-call machinery.
	MechanismStructuredJSON Mechanism = "structured_json"
)

// ToolResult is one tool's outcome.
//
// Trials and OtherTrials are the two denominators, and both are published
// because they answer different questions: Trials asks "when this tool was the
// answer, did it get called", OtherTrials asks "when it was not, did it get
// called anyway".
type ToolResult struct {
	Tool string
	// Arm names a variant of the same tool being compared against another,
	// empty when a tool has only one. Added at T.2 (2026-08-31), where
	// compile_rule_file and validate_cpipes_config each gained a path-valued
	// alternative to a content argument (I-94's remedy) and the question is
	// whether the model names a path as unreliably as it copied a rule.
	//
	// **A field rather than a suffix on Tool, because the two rows are one
	// comparison.** Reporting "compile_rule_file [path]" beside
	// "compile_rule_file [text]" would read as two tools and invite a total
	// across them — which is exactly the aggregate decision 13 refuses, since
	// the denominators are equal by construction here and would make a total
	// look defensible. Arm keeps the pair legible as a pair.
	Arm string
	// Trials is how many prompts had this tool as the expected answer, times
	// the repeat count.
	Trials int
	// Selected is how many of those called exactly this tool.
	Selected int
	// ArgsValid is how many of Selected produced arguments satisfying the wire
	// schema the client was served. Its denominator is Selected, not Trials —
	// arguments cannot be judged on a call that was never made.
	ArgsValid int
	// PayloadTrials is how many of Selected carried a payload the prompt gave
	// the model to pass through — a config, a rule source, a class name.
	// Its own denominator, because tools taking no such argument have none.
	PayloadTrials int
	// PayloadFaithful is how many of PayloadTrials reproduced it intact.
	//
	// **This is the number the schema cannot produce.** rule_text is
	// {"type":"string"} and config relaxes to {"type":"object"} at the wire, so
	// a truncated rule and a config missing half its pipes both satisfy
	// ArgsValid. The two tools whose arguments carry the most meaning are
	// exactly the two whose schemas check the least.
	PayloadFaithful int
	// PayloadDiverted is how many of PayloadTrials did not carry the payload
	// argument at all — the model answered through a *different* argument of the
	// same tool.
	//
	// **Added at T.2 because without it the run said the wrong thing.** Giving
	// validate_cpipes_config a path alternative made the model reach for the path
	// even when the prompt handed it a config inline: 5 of 8 content-arm calls in
	// one dump filled `config_path` and nothing else. Those are schema-valid calls
	// carrying no payload, and folding them into PayloadFaithful's complement reads
	// as *the payload was mangled* — I-68's finding recurring — when what happened
	// is that the payload was never sent. Two different verdicts on the remedy, and
	// the rate alone cannot tell them apart.
	//
	// PayloadFaithful + (mangled) + PayloadDiverted = PayloadTrials.
	PayloadDiverted int
	// OtherTrials is how many trials expected something else, including the
	// no-tool cases.
	OtherTrials int
	// FalsePositives is how many of OtherTrials called this tool anyway.
	FalsePositives int
}

// ReportsRate says whether this tool has enough trials for a percentage to mean
// anything. Same threshold as compile-pass, for the same reason.
func (r ToolResult) ReportsRate() bool { return r.Trials >= RateThreshold }

// Line renders one tool the way it may be published.
func (r ToolResult) Line() string {
	var sel string
	if r.Trials == 0 {
		sel = "not exercised"
	} else if !r.ReportsRate() {
		sel = fmt.Sprintf("selected %d of %d (too few for a rate)", r.Selected, r.Trials)
	} else {
		sel = fmt.Sprintf("selected %d of %d (%.0f%%)", r.Selected, r.Trials,
			100*float64(r.Selected)/float64(r.Trials))
	}
	args := "args n/a"
	if r.Selected > 0 {
		args = fmt.Sprintf("args valid %d of %d", r.ArgsValid, r.Selected)
	}
	if r.PayloadTrials > 0 {
		args += fmt.Sprintf(", payload intact %d of %d", r.PayloadFaithful, r.PayloadTrials)
		if r.PayloadDiverted > 0 {
			args += fmt.Sprintf(" (%d sent no payload)", r.PayloadDiverted)
		}
	}
	fp := fmt.Sprintf("false calls %d of %d", r.FalsePositives, r.OtherTrials)
	name := r.Tool
	if r.Arm != "" {
		name += " · " + r.Arm
	}
	return fmt.Sprintf("%-31s %-34s %-46s %s", name, sel, args, fp)
}

// Label is the row's identity — the tool, and the arm where there is one.
func (r ToolResult) Label() string {
	if r.Arm == "" {
		return r.Tool
	}
	return r.Tool + " · " + r.Arm
}

// AbstentionResult is the no-tool population: prompts no catalogue tool
// answers. It is reported separately rather than folded into any tool, because
// its denominator belongs to none of them.
type AbstentionResult struct {
	// Trials is how many prompts expected no tool call at all.
	Trials int
	// Abstained is how many of those made no tool call.
	Abstained int
}

// ToolReport is a whole conformance measurement.
//
// **No aggregate, and here the reason is sharper than decision 13's.** A total
// across tools would average a tool that takes no arguments against one whose
// argument is a whole cpipes config, and the two are not the same task. The
// denominators are equal by construction in this harness, which makes a total
// look more defensible than it is — equal denominators say the cases were
// balanced, not that the tasks were.
type ToolReport struct {
	Mechanism Mechanism
	// Model is what the server said answered, which need not be what was asked
	// for: a tag can resolve to a different build.
	Model string
	// CaseSource says where the prompts came from. Mandatory, because these are
	// hand-written probes rather than a corpus, and a reader who assumes
	// otherwise will over-read the figures.
	CaseSource string
	// Repeats is how many times each case was run. Published because Trials
	// multiplies it in, and a reader cannot otherwise tell six prompts run
	// three times from eighteen prompts.
	Repeats    int
	Tools      []ToolResult
	Abstention AbstentionResult
}

// Validate refuses a report that cannot be published honestly. String calls it,
// so an invalid report cannot be rendered by accident.
func (r *ToolReport) Validate() error {
	switch r.Mechanism {
	case MechanismNativeTools, MechanismStructuredJSON:
	case "":
		return fmt.Errorf("eval: the report does not say which mechanism produced the tool calls; " +
			"a figure without that will be compared with one produced the other way")
	default:
		return fmt.Errorf("eval: %q is not a mechanism", r.Mechanism)
	}
	if r.Model == "" {
		return fmt.Errorf("eval: the report does not name the model that answered")
	}
	if r.CaseSource == "" {
		return fmt.Errorf("eval: the report does not say where its prompts came from")
	}
	if r.Repeats < 1 {
		return fmt.Errorf("eval: the report says each case ran %d times", r.Repeats)
	}
	if len(r.Tools) == 0 {
		return fmt.Errorf("eval: the report covers no tools")
	}
	for _, t := range r.Tools {
		if t.Selected > t.Trials {
			return fmt.Errorf("eval: %s selected %d of %d trials", t.Label(), t.Selected, t.Trials)
		}
		if t.ArgsValid > t.Selected {
			return fmt.Errorf("eval: %s had %d valid argument sets from %d calls",
				t.Label(), t.ArgsValid, t.Selected)
		}
		if t.PayloadFaithful > t.PayloadTrials {
			return fmt.Errorf("eval: %s reproduced %d payloads intact from %d that carried one",
				t.Label(), t.PayloadFaithful, t.PayloadTrials)
		}
		if t.PayloadTrials > t.Selected {
			return fmt.Errorf("eval: %s had %d payload trials from %d calls",
				t.Label(), t.PayloadTrials, t.Selected)
		}
		if t.PayloadFaithful+t.PayloadDiverted > t.PayloadTrials {
			return fmt.Errorf("eval: %s reports %d intact and %d with no payload from %d trials",
				t.Label(), t.PayloadFaithful, t.PayloadDiverted, t.PayloadTrials)
		}
		if t.FalsePositives > t.OtherTrials {
			return fmt.Errorf("eval: %s was called falsely %d times in %d trials that expected "+
				"something else", t.Label(), t.FalsePositives, t.OtherTrials)
		}
	}
	if r.Abstention.Abstained > r.Abstention.Trials {
		return fmt.Errorf("eval: abstained on %d of %d trials",
			r.Abstention.Abstained, r.Abstention.Trials)
	}
	return nil
}

// String renders the report, returning the validation error as text rather than
// an empty string so a broken report is loud.
func (r *ToolReport) String() string {
	if err := r.Validate(); err != nil {
		return "INVALID REPORT: " + err.Error()
	}
	ts := make([]ToolResult, len(r.Tools))
	copy(ts, r.Tools)
	sort.Slice(ts, func(i, j int) bool { return ts[i].Label() < ts[j].Label() })

	var b strings.Builder
	fmt.Fprintf(&b, "tool-call conformance — mechanism %s, model %s\n", r.Mechanism, r.Model)
	fmt.Fprintf(&b, "cases: %s, each run %d time(s)\n\n", r.CaseSource, r.Repeats)
	for _, t := range ts {
		b.WriteString("  " + t.Line() + "\n")
	}
	if r.Abstention.Trials > 0 {
		fmt.Fprintf(&b, "\n  %-24s abstained %d of %d prompts no tool answers\n",
			"(no tool expected)", r.Abstention.Abstained, r.Abstention.Trials)
	}
	b.WriteString("\nNo aggregate figure is published: a total would average a tool taking no " +
		"arguments against one whose argument is a whole cpipes config (decision 13).\n")
	return b.String()
}
