package rca

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The deterministic floor.
//
// §9.8's instruction is that the contradicting side "should be populated from
// Anomaly.Confounders before any model is asked for one, so that a model's
// contribution is measurable against a floor rather than against nothing". This
// file is that floor. It holds no threshold and asks no model; everything it
// emits is either transcribed from the gate's §9.5 table, read off an AC.1
// finding, or read off an anomaly a detector wrote.
//
// # Why confidence is counted rather than weighted
//
// A weighted score would need weights, and this repository already carries
// three unsourced numbers between observe and triage (R-21, R-36); a fourth set
// invented to order hypotheses would be a judgement wearing arithmetic. So the
// floor counts: confidence is the fraction of a hypothesis's evidence items
// that are for it, which is §B.3's escalation trigger — contradictory evidence
// exceeding supporting — expressed as a number rather than as a rule. The
// gate's own reading enters the count as evidence rather than as a coefficient:
// a class §9.5 says the record cannot evidence gets a contradicting item saying
// so, and therefore ranks below one it can, without anything here deciding by
// how much.
//
// **What that buys and what it costs.** It buys a ranking whose basis is a
// sentence an operator can check by counting the two lists, and it costs
// resolution: three hypotheses with one supporting item and one contradicting
// item each are tied at 0.5, and the tie is broken by §9.4's locus order rather
// than by anything about the run. The ties are reported as ties in the basis
// rather than hidden by the sort.

// Ranker is the deterministic floor. It has one field and no thresholds, which
// is the same asymmetry triage.Classifier reports for the opposite reason:
// there the one number was a detector's, here there is no number at all.
type Ranker struct {
	// Generation is the trailing number of the ranker_ref.
	Generation int
}

// Default is the ranker.
func Default() Ranker { return Ranker{Generation: 1} }

// Ref is the ranker_ref written on every hypothesis.
func (r Ranker) Ref() string { return fmt.Sprintf("rca@%d", r.Generation) }

// Input is what the floor reads for one session.
type Input struct {
	// Report is AC.1's nine verdicts. Required.
	Report *triage.Report
	// Evidence is what AC.1 gathered, reused rather than re-read so that a
	// hypothesis's corroborators are the same rows the verdict was reached on.
	// Optional: a caller holding only a Report gets a ranking with the findings
	// as its only supporting evidence.
	Evidence *triage.Evidence
	// Anomalies are what N.4's detectors said about this session. Optional, and
	// empty is the ordinary case — jetsapi.anomaly is deployed nowhere that has
	// not run `update_db -migrateDb` (P3 F101, I-169), and no detector runs on a
	// schedule yet.
	Anomalies []observe.Anomaly
}

// Rank returns the session's ranked hypotheses.
//
// It emits one hypothesis per (present locus, cause class §9.5 maps that locus
// to) and nothing else: it does not invent a class for a locus the gate's table
// does not reach, and it does not emit a hypothesis for a locus that did not
// fire. A session with no present locus yields no hypotheses and a Basis saying
// which loci were asked and which could not be.
func (r Ranker) Rank(in *Input) *Ranking {
	out := &Ranking{RankerRef: r.Ref()}
	if in == nil || in.Report == nil {
		out.Basis = "no triage report was supplied, so nothing was ranked"
		return out
	}
	out.SessionId = in.Report.SessionId

	byLocus := map[string]*triage.Finding{}
	for i := range in.Report.Findings {
		byLocus[in.Report.Findings[i].Locus] = &in.Report.Findings[i]
	}
	var present []string
	for _, l := range triage.Loci {
		f := byLocus[l]
		if f == nil {
			continue
		}
		switch f.Verdict {
		case triage.Present:
			present = append(present, l)
		case triage.NotEvaluable:
			out.UnaskedLoci = append(out.UnaskedLoci, l)
		}
	}

	anomaliesAt := map[string][]observe.Anomaly{}
	var unplacedSignals []string
	for i := range in.Anomalies {
		a := in.Anomalies[i]
		if l, ok := signalLocus[a.SignalType]; ok {
			anomaliesAt[l] = append(anomaliesAt[l], a)
			continue
		}
		unplacedSignals = append(unplacedSignals, fmt.Sprintf("%s (%s)", a.AnomalyId, a.SignalType))
	}

	for _, locus := range present {
		classes := ClassesFor(locus)
		if len(classes) == 0 {
			out.UnmappedLoci = append(out.UnmappedLoci, locus)
			continue
		}
		for _, c := range classes {
			out.Hypotheses = append(out.Hypotheses,
				r.hypothesis(in, byLocus, anomaliesAt, locus, c))
		}
	}

	sortHypotheses(out.Hypotheses)
	for i := range out.Hypotheses {
		out.Hypotheses[i].Rank = i + 1
		out.Hypotheses[i].Basis = rankBasis(&out.Hypotheses[i], len(out.Hypotheses))
	}
	out.Basis = r.rankingBasis(out, present, unplacedSignals, len(in.Anomalies))
	return out
}

// hypothesis builds one (locus, class) pair's case for and against.
func (r Ranker) hypothesis(in *Input, byLocus map[string]*triage.Finding,
	anomaliesAt map[string][]observe.Anomaly, locus string, c CauseClass) Hypothesis {

	f := byLocus[locus]
	h := Hypothesis{
		HypothesisId:  fmt.Sprintf("%s/%s/%s", in.Report.SessionId, locus, c.Name),
		Cause:         causeSentence(locus, c),
		CauseCategory: c.Name,
		Locus:         locus,
		RankerRef:     r.Ref(),
		// Not nil. An empty list is the claim that nothing was found against
		// this hypothesis; the code below is what decides whether it stays
		// empty, and for eight of the ten classes §9.5 alone guarantees it does
		// not.
		ContradictingEvidence: []Evidence{},
	}

	// --- the case for --------------------------------------------------------

	h.SupportingEvidence = append(h.SupportingEvidence, Evidence{
		Statement: fmt.Sprintf("triage found locus %s present for session %s: %s",
			locus, in.Report.SessionId, f.Basis),
		Source:    SourceRunTelemetry,
		SourceRef: fmt.Sprintf("%s/%s (%s)", in.Report.SessionId, locus, f.ClassifierRef),
	})

	// Other positions §9.5 says this class shows at, that also fired. A class
	// evidenced at two of its four positions is better supported than one
	// evidenced at one, and this is the only place the floor compares classes
	// on anything but their §9.5 row.
	for _, other := range c.Loci {
		if other == locus {
			continue
		}
		if g := byLocus[other]; g != nil && g.Verdict == triage.Present {
			h.SupportingEvidence = append(h.SupportingEvidence, Evidence{
				Statement: fmt.Sprintf("§9.5 says this class can also be carried by locus %s, and that "+
					"locus is present for this run as well: %s", other, g.Basis),
				Source:    SourceRunTelemetry,
				SourceRef: fmt.Sprintf("%s/%s", in.Report.SessionId, other),
			})
		}
	}

	for _, a := range anomaliesAt[locus] {
		h.SupportingEvidence = append(h.SupportingEvidence, Evidence{
			Statement: fmt.Sprintf("detector %s raised a %s anomaly on %s %s: observed %s against %s",
				a.DetectorRef, a.SignalType, a.SubjectType, a.SubjectRef, a.ObservedValue, a.ExpectedBasis),
			Source:    SourceRunTelemetry,
			SourceRef: fmt.Sprintf("jetsapi.anomaly %s", a.AnomalyId),
		})
	}

	h.SupportingEvidence = append(h.SupportingEvidence, corroborators(in, locus, c)...)

	// --- the case against ----------------------------------------------------

	// §9.5's own verdict on whether the record can evidence this class. Every
	// class but parse_failure carries one, which is what makes the floor
	// advisory by construction rather than by intention.
	if c.Evidenceability != Evidenced {
		h.ContradictingEvidence = append(h.ContradictingEvidence, Evidence{
			Statement: fmt.Sprintf("the execution record's ability to evidence %s is %q: %s",
				c.Name, c.Evidenceability, c.Note),
			Source:    SourceCodeInspection,
			SourceRef: "phase-4 plan §9.5",
		})
	}

	// The confounders the classifier and the detectors declared. This is the
	// substrate §9.8 named, and it is the reason the contradicting side is
	// populated before a model is asked for one.
	for_, against := confounderEvidence(in.Report.SessionId, locus, f.Confounders, c,
		"triage finding "+f.ClassifierRef)
	h.SupportingEvidence = append(h.SupportingEvidence, for_...)
	h.ContradictingEvidence = append(h.ContradictingEvidence, against...)
	for _, a := range anomaliesAt[locus] {
		for_, against := confounderEvidence(in.Report.SessionId, locus, a.Confounders, c,
			"jetsapi.anomaly "+a.AnomalyId)
		h.SupportingEvidence = append(h.SupportingEvidence, for_...)
		h.ContradictingEvidence = append(h.ContradictingEvidence, against...)
	}

	// The class's other positions, which were asked and said no, or could not
	// be asked at all. The two are different claims and are worded differently:
	// the first is evidence against, the second is a bound on the case.
	for _, other := range c.Loci {
		if other == locus {
			continue
		}
		g := byLocus[other]
		if g == nil {
			continue
		}
		switch g.Verdict {
		case triage.Absent:
			h.ContradictingEvidence = append(h.ContradictingEvidence, Evidence{
				Statement: fmt.Sprintf("§9.5 says this class can also be carried by locus %s, and that "+
					"locus was evaluated and does not hold: %s", other, g.Basis),
				Source:    SourceRunTelemetry,
				SourceRef: fmt.Sprintf("%s/%s", in.Report.SessionId, other),
			})
		case triage.NotEvaluable:
			h.ContradictingEvidence = append(h.ContradictingEvidence, Evidence{
				Statement: fmt.Sprintf("§9.5 says this class can also be carried by locus %s, and that "+
					"locus could not be evaluated at all, so the case for this class is bounded rather "+
					"than complete: %s", other, g.Basis),
				Source:    SourceRunTelemetry,
				SourceRef: fmt.Sprintf("%s/%s", in.Report.SessionId, other),
			})
		}
	}

	// The one cross-locus rule, and it is the use for a locus §9.5 maps to
	// nothing. Locus per_record_failures_unreportable's stated *cannot see* is
	// that the record cannot distinguish it from a clean run (§9.4 row 8), so a
	// run in that state cannot be called benign on the record's silence.
	if c.Name == CauseBenignVariation {
		if g := byLocus[triage.LocusPerRecordFailuresUnreportable]; g != nil && g.Verdict == triage.Present {
			h.ContradictingEvidence = append(h.ContradictingEvidence, Evidence{
				Statement: "locus per_record_failures_unreportable is present for this run: an operator " +
					"that can report per-record errors is configured without an error channel, so its " +
					"failures have nowhere to go and the absence of process_errors rows is not evidence " +
					"of their absence (§9.4 row 8). " + g.Basis,
				Source:    SourceRunTelemetry,
				SourceRef: fmt.Sprintf("%s/%s", in.Report.SessionId, triage.LocusPerRecordFailuresUnreportable),
			})
		}
	}

	h.Confidence = confidence(len(h.SupportingEvidence), len(h.ContradictingEvidence))
	return h
}

// confounderEvidence turns a confounder list into evidence items, on the side
// the confounder actually falls.
//
// A configured-behaviour confounder — on_error_drop, sampling_cap and the three
// beside them — is an explanation rather than a doubt, so it is evidence *for*
// benign_variation and against every other class. A record-limit confounder is
// against all ten, benign included, because §9.5's last row is that benign can
// be proposed and not established.
func confounderEvidence(session, locus string, confounders []string, c CauseClass,
	ref string) (supporting, against []Evidence) {

	for _, name := range confounders {
		if benignExplanation(name) && c.Name == CauseBenignVariation {
			supporting = append(supporting, Evidence{
				Statement: fmt.Sprintf("%s is a configured behaviour that would produce this observation "+
					"deliberately, which is the case for benign variation rather than a doubt about it",
					name),
				Source:    SourceDetectorConfounder,
				SourceRef: fmt.Sprintf("%s: %s at %s/%s", ref, name, session, locus),
			})
			continue
		}
		against = append(against, Evidence{
			Statement: confounderStatement(name, c),
			Source:    SourceDetectorConfounder,
			SourceRef: fmt.Sprintf("%s: %s at %s/%s", ref, name, session, locus),
		})
	}
	return supporting, against
}

func confounderStatement(name string, c CauseClass) string {
	if benignExplanation(name) {
		return fmt.Sprintf("%s is configured on this run: a behaviour the pipeline was asked for would "+
			"produce the same observation, which is an alternative to %s that the record does not exclude",
			name, c.Name)
	}
	return fmt.Sprintf("%s: the comparison behind this locus is bounded, so it does not exclude an "+
		"explanation other than %s", name, c.Name)
}

// corroborators are the class-specific readings §9.5 names as the record's
// instruments, taken off the evidence AC.1 already gathered. They are
// deliberately few: §9.5 names an instrument for four of the ten classes and
// inventing one for the others is the step this floor exists not to take.
func corroborators(in *Input, locus string, c CauseClass) []Evidence {
	ev := in.Evidence
	if ev == nil {
		return nil
	}
	var out []Evidence
	switch c.Name {
	case CauseParseFailure:
		// §9.5: "input_bad_records_count is exactly this and it counts without
		// sampling."
		var bad, rows int64
		if ev.Workers != nil {
			for _, w := range ev.Workers.Rows {
				rows++
				if w.InputBadRecords != nil {
					bad += *w.InputBadRecords
				}
			}
		}
		if bad > 0 {
			out = append(out, Evidence{
				Statement: fmt.Sprintf("input_bad_records_count sums to %d over %d worker rows, which is "+
					"read-time parse failure counted without sampling", bad, rows),
				Source:    SourceRunTelemetry,
				SourceRef: fmt.Sprintf("%s: pipeline_execution_details.input_bad_records_count", ev.SessionId),
			})
		}
	case CauseValidationBreach:
		// §9.5: a jets:exception reaches process_errors, and the jetrules
		// family is the one that saves a rete session with it (F185).
		if ev.Errors != nil && ev.Errors.WithReteSession > 0 {
			out = append(out, Evidence{
				Statement: fmt.Sprintf("%d of %d process_errors rows carry a saved rete session, which is "+
					"the jetrules family's own column and is the closest the table comes to saying a rule "+
					"raised the error (F185)", ev.Errors.WithReteSession, ev.Errors.Rows),
				Source:    SourceDqResult,
				SourceRef: fmt.Sprintf("%s: jetsapi.process_errors", ev.SessionId),
			})
		}
	case CauseSourceContentChange:
		if ev.Errors != nil && ev.Errors.WithInputColumn > 0 {
			out = append(out, Evidence{
				Statement: fmt.Sprintf("%d of %d process_errors rows name an input_column, which is written "+
					"by the inference operator family and is the only column-level signal the table carries "+
					"(F185)", ev.Errors.WithInputColumn, ev.Errors.Rows),
				Source:    SourceDqResult,
				SourceRef: fmt.Sprintf("%s: jetsapi.process_errors.input_column", ev.SessionId),
			})
		}
	case CauseInfrastructureFailure:
		if ev.Header != nil && ev.Header.FailureDetails != "" {
			out = append(out, Evidence{
				Statement: fmt.Sprintf("the run header carries %d characters of failure_details, one of "+
					"whose four undeclared shapes is the decoded ECS StoppedReason; which of the decoder's "+
					"arms produced it is not recorded (F197, F198)", len(ev.Header.FailureDetails)),
				Source:    SourceRunTelemetry,
				SourceRef: fmt.Sprintf("%s: pipeline_execution_status.failure_details", ev.SessionId),
			})
		}
	case CauseBenignVariation:
		// The five configured-behaviour confounders the run's stored config
		// mentions. observe reads them for a detector; here they are the case
		// for benign rather than a qualifier on a signal.
		if ev.Config != nil && ev.Config.Read {
			for _, name := range ev.Config.Found {
				if !benignExplanation(name) {
					continue
				}
				out = append(out, Evidence{
					Statement: fmt.Sprintf("the run's stored configuration mentions %s, a behaviour that "+
						"would produce this observation on purpose", name),
					Source:    SourceCodeInspection,
					SourceRef: fmt.Sprintf("%s: cpipes_execution_status.cpipes_config_json", ev.SessionId),
				})
			}
		}
	}
	return out
}

// confidence is the fraction of a hypothesis's evidence that is for it.
func confidence(sup, con int) float64 {
	if sup+con == 0 {
		return 0
	}
	return float64(sup) / float64(sup+con)
}

// causeSentence is the hypothesis's cause in words. §A.2.8 caps it at 500
// characters and this stays well inside; it names the class and the position it
// was inferred from, because a cause read without its locus is R-27.
func causeSentence(locus string, c CauseClass) string {
	return fmt.Sprintf("%s, inferred from locus %s — §9.5 lists %s among the loci that can carry this "+
		"class", c.Name, locus, locus)
}

// sortHypotheses orders by §9.5's evidenceability tier, then by confidence,
// then by §9.4's locus order, then by §9.5's class order. The two tie-breaks
// are positional rather than evaluative so that a ranking is reproducible;
// rankBasis says when one was used.
//
// **The tier is first and that is a correction rather than a design choice** —
// see CauseClass.Evidenceable, which carries the measurement that produced it.
func sortHypotheses(hs []Hypothesis) {
	sort.SliceStable(hs, func(i, j int) bool {
		a, b := &hs[i], &hs[j]
		ea := evidenceabilityOf(a.CauseCategory) != None
		eb := evidenceabilityOf(b.CauseCategory) != None
		if ea != eb {
			return ea
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		if la, lb := locusOrder(a.Locus), locusOrder(b.Locus); la != lb {
			return la < lb
		}
		return classOrder(a.CauseCategory) < classOrder(b.CauseCategory)
	})
}

// rankBasis is criterion 45's last clause for one hypothesis: why it ranks
// where it does, in numbers a reader can check by counting the two lists.
func rankBasis(h *Hypothesis, total int) string {
	sup, con := len(h.SupportingEvidence), len(h.ContradictingEvidence)
	b := fmt.Sprintf("ranked %d of %d. %d evidence item(s) for and %d against, so confidence is "+
		"%d/%d = %.2f — the floor counts evidence rather than weighting it, so this number is a "+
		"ratio of the two lists below and not a probability.",
		h.Rank, total, sup, con, sup, sup+con, h.Confidence)
	b += fmt.Sprintf(" Derived from locus %s, which §9.5 lists as carrying %s.", h.Locus, h.CauseCategory)
	if e := evidenceabilityOf(h.CauseCategory); e == None {
		b += " §9.5 answers \"no\" to whether the record can evidence this class, so it is ranked below" +
			" every class the record can speak to whatever its count; a class the substrate cannot" +
			" evidence is not a weak candidate but one there is no instrument for."
	} else {
		b += fmt.Sprintf(" §9.5's evidenceability for this class is %q, which puts it above every class"+
			" the record cannot evidence at all.", e)
	}
	if con == 0 {
		b += " Nothing was found against it, which for this class means §9.5 records the record as able" +
			" to evidence it and no confounder was declared."
	}
	b += " Ties at equal confidence are broken by §9.4's locus order and then by §9.5's class order," +
		" which are positional and say nothing about this run."
	return b
}

// rankingBasis is the account of the ranking as a whole: what was considered,
// what was dropped before any hypothesis existed, and what could not be asked.
// The per-hypothesis basis says why one outranks another and cannot say any of
// this.
func (r Ranker) rankingBasis(out *Ranking, present, unplacedSignals []string, anomalies int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s ranked %d hypotheses for session %s. ", r.Ref(), len(out.Hypotheses), out.SessionId)
	if len(present) == 0 {
		b.WriteString("No locus is present, so no hypothesis was emitted: this ranking asserts nothing " +
			"about the run rather than asserting that it was healthy. ")
	} else {
		fmt.Fprintf(&b, "%d locus/loci are present (%s), and each was mapped to the cause classes "+
			"phase-4 plan §9.5 lists as able to carry it. ", len(present), strings.Join(present, ", "))
	}
	if len(out.UnmappedLoci) > 0 {
		fmt.Fprintf(&b, "%d present locus/loci map to no cause class in §9.5 and produced no hypothesis "+
			"(%s); that is a property of the gate's table rather than of this run. ",
			len(out.UnmappedLoci), strings.Join(out.UnmappedLoci, ", "))
	}
	if len(out.UnaskedLoci) > 0 {
		fmt.Fprintf(&b, "%d of the nine loci could not be evaluated (%s), so every class whose only "+
			"evidence position is among them has not been ruled out — it has not been looked at. ",
			len(out.UnaskedLoci), strings.Join(out.UnaskedLoci, ", "))
	}
	var noLocus, noEvidence []string
	for _, c := range causeClasses {
		if len(c.Loci) == 0 {
			noLocus = append(noLocus, c.Name)
		}
		if c.Evidenceability == None {
			noEvidence = append(noEvidence, c.Name)
		}
	}
	fmt.Fprintf(&b, "%d of the ten cause classes are attached to no locus at all in §9.5 (%s) and can "+
		"therefore never be emitted by this floor, whatever the run did. A further %d are attached to a "+
		"locus and answered \"no\" in §9.5's evidenceability column (%s): those are emitted where their "+
		"locus fires and carry §9.5's own reason as evidence against themselves, which is how a class the "+
		"record cannot support is put in front of a reader rather than hidden from one. ",
		len(noLocus), strings.Join(noLocus, ", "),
		len(noEvidence)-len(noLocus), strings.Join(without(noEvidence, noLocus), ", "))
	if anomalies == 0 {
		b.WriteString("No anomaly rows were supplied for this session, so no detector's confounders " +
			"entered the case against; jetsapi.anomaly is deployed only where `update_db -migrateDb` " +
			"has run. ")
	} else {
		fmt.Fprintf(&b, "%d anomaly row(s) were read. ", anomalies)
	}
	if len(unplacedSignals) > 0 {
		fmt.Fprintf(&b, "%d anomaly/anomalies carry a signal type §9.4 has no locus for and contributed "+
			"to no hypothesis (%s): step_regression is a comparison across runs rather than a position "+
			"in one run's record. ", len(unplacedSignals), strings.Join(unplacedSignals, ", "))
	}
	b.WriteString("No model was consulted.")
	return b.String()
}

// without returns a minus b, preserving a's order.
func without(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}
