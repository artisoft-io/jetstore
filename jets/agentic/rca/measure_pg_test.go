// AD.1's instrument: the denominators §11.3.1's metrics need, computed rather
// than quoted.
//
// # What this file is for
//
// Task AD.1 asks what the source proposal's §11.3.1 capability metrics can be
// computed on, per metric, with denominators and no aggregate. AA.2 counted the
// labelled population at **zero** (plan §10), so six of the seven metrics are
// not computable and the reasons differ; plan §22 is the per-metric table and
// this file is what the numbers in it were read off.
//
// It produces three things and refuses to produce a fourth:
//
//   - **A per-locus census** — for each of AC.1's nine loci, how many runs
//     answered present, absent and not_evaluable. Classify returns all nine
//     verdicts per run rather than the hits (§13.4), which is what makes the
//     denominator exist at all; this asserts that it does, per locus, over a
//     population rather than over one fixture.
//   - **A per-cause-class census** — for each of the imported ten, how many
//     hypotheses the deterministic floor emitted, beside §9.5's evidenceability
//     tier and the number of loci the gate attaches to the class. Two of the ten
//     are attached to no locus at all, so their denominator is **structurally**
//     zero rather than empirically zero, and the assertion below is what makes
//     that a measured claim.
//   - **A model-against-rule comparison**, when a model is named. That is the
//     one comparison this phase can make without inventing labels (plan §10.6,
//     I-275): it scores AC.2's model arm against AC.1's deterministic locus,
//     which is a model against a rule and is not an accuracy.
//
// **What it refuses to produce is Classification Accuracy over the locus
// taxonomy.** AC.1 is the implementation of the nine predicates, so scoring it
// against them is a definition against itself and would print 100% in a row
// whose published target is 90% (I-275). Nothing here computes that number.
//
// # What the population is, and it is not a deployment's
//
// The five sessions below were built by this file. **No pipeline ran** — the
// same sentence criterion 43's fourth clause turns on (I-379) — so every count
// here is a property of a fixture and of the code that read it, and none of them
// is a base rate for how JetStore pipelines fail (R-37). The shapes were chosen
// to exercise the loci rather than sampled from anything, which is the whole of
// why the census is reported with its denominator and never as a rate.
//
// Needs JETS_TEST_DSN; the model arm additionally needs JETS_RCA_MODEL:
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5461/postgres \
//	JETS_RCA_MODEL=granite4.1:3b go test -run TestTheModelAgainstTheDeterministicLocus \
//	  -v -timeout 60m ./jets/agentic/rca/
//
// This machine has no GPU and the models are CPU-bound (R-23), so budget an hour
// and read every model figure with the model's name beside it.

package rca

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A configuration whose one reporting operator instance declares an error
// channel, which is the 9-of-12 jetrules case (F188) and the state in which
// locus per_record_failures_unreportable answers absent rather than present.
const configWithErrorChannel = `{"pipes_config":[{"type":"fan_out","input_channel":{"name":"input"},
  "apply":[{"type":"jetrules","jetrules_config":{"error_channel":{"name":"process_errors.out"}},
            "output_channel":{"name":"ruled"}}]}]}`

// ad1Census writes five runs of different shapes into one database and returns
// their session ids in a stable order.
//
// **The shapes are chosen and that is the population's whole provenance.** Worker
// rows are written by the cpipes node's own two statements, for the reason
// triage's suite gives; headers, process_errors rows and stored configurations
// are composed here, because their production writers need S3, a compiled
// workspace and a state machine.
func ad1Census(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	ctx := context.Background()

	// 1. The five-locus run: a failed worker, a stalled sibling, a collapsed
	//    worker, per-record errors, and an operator with nowhere to report.
	k := header(t, pool, "cgt", "loader", "claims", "s-ad1-multi", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-multi", "reducing01", 0, "failed", 5000, 12, 0,
		"download failed,rules pool exhausted", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-multi", "reducing01", 1, "", 0, 0, 0, "",
		now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-multi", "reducing01", 2, "completed", 8000, 3, 40, "",
		now.Add(-time.Hour))
	processErrors(t, pool, k, "s-ad1-multi", 20, 6)
	storeConfig(t, pool, k, "s-ad1-multi", configNoErrorChannel)
	if err := observe.InsertAnomaly(ctx, pool, &observe.Anomaly{
		AnomalyId: "an-ad1", DetectedAt: now, SessionId: "s-ad1-multi",
		SubjectType: observe.SubjectWorker, SubjectRef: "reducing01/2",
		SignalType: observe.SignalVolume, ObservedValue: "0.005",
		ExpectedBasis: "within-run input against output on the worker row",
		Confounders:   []string{observe.ConfounderOnErrorDrop},
		DetectorRef:   "volume_collapse@1",
	}); err != nil {
		t.Fatalf("InsertAnomaly: %v", err)
	}

	// 2. A terminal header with no worker row at all.
	header(t, pool, "cgt", "loader", "claims", "s-ad1-notstarted", "failed", now.Add(-2*time.Hour))

	// 3. A worker that never terminated under a header that did.
	k = header(t, pool, "cgt", "loader", "claims", "s-ad1-stall", "failed", now.Add(-3*time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-stall", "reducing01", 0, "", 0, 0, 0, "",
		now.Add(-3*time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-stall", "reducing01", 1, "completed", 4000, 0, 3900, "",
		now.Add(-3*time.Hour))
	storeConfig(t, pool, k, "s-ad1-stall", configWithErrorChannel)

	// 4. Per-record failures that were reported, by an operator configured to
	//    report them — so locus 7 is present and locus 8 is absent.
	k = header(t, pool, "cgt", "loader", "claims", "s-ad1-perrecord", "failed", now.Add(-4*time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-perrecord", "reducing01", 0, "completed", 6000, 44, 5900,
		"", now.Add(-4*time.Hour))
	processErrors(t, pool, k, "s-ad1-perrecord", 44, 44)
	storeConfig(t, pool, k, "s-ad1-perrecord", configWithErrorChannel)

	// 5. A run that completed with nothing wrong with it, which is the case a
	//    census without it cannot report an absent verdict from.
	k = header(t, pool, "cgt", "loader", "claims", "s-ad1-clean", "completed", now.Add(-5*time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-clean", "reducing01", 0, "completed", 7000, 0, 6980, "",
		now.Add(-5*time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-ad1-clean", "reducing01", 1, "completed", 7100, 0, 7050, "",
		now.Add(-5*time.Hour))
	storeConfig(t, pool, k, "s-ad1-clean", configWithErrorChannel)

	return []string{"s-ad1-multi", "s-ad1-notstarted", "s-ad1-stall", "s-ad1-perrecord", "s-ad1-clean"}
}

// The two censuses criterion 44 asks for, with the denominators that make them
// readable and no rate anywhere.
func TestTheLocusAndClassCensusesCarryTheirDenominators(t *testing.T) {
	pool := freshDB(t, "rca_ad1")
	sessions := ad1Census(t, pool)

	locusCensus := map[string]map[triage.Verdict]int{}
	for _, l := range triage.Loci {
		locusCensus[l] = map[triage.Verdict]int{}
	}
	classCensus := map[string]int{}
	evaluable, findings, hypotheses := 0, 0, 0

	for _, s := range sessions {
		rep, ranking := rank(t, pool, s)
		if len(rep.Findings) != len(triage.Loci) {
			t.Fatalf("%s produced %d findings, want %d: a classifier that returns its hits supplies "+
				"numerators and no denominators (§13.4)", s, len(rep.Findings), len(triage.Loci))
		}
		findings += len(rep.Findings)
		evaluable += rep.Evaluable
		for i := range rep.Findings {
			f := &rep.Findings[i]
			locusCensus[f.Locus][f.Verdict]++
		}
		hypotheses += len(ranking.Hypotheses)
		for i := range ranking.Hypotheses {
			classCensus[ranking.Hypotheses[i].CauseCategory]++
		}
	}

	// The denominator is the population, per locus, and it is the same number
	// for every locus by construction. That is the property criterion 44 needs
	// and it is asserted rather than assumed.
	t.Logf("locus census over %d run(s) this file created; no pipeline ran (I-379)", len(sessions))
	t.Logf("%-38s %8s %8s %14s %8s", "locus", "present", "absent", "not_evaluable", "denom")
	for _, l := range triage.Loci {
		c := locusCensus[l]
		sum := c[triage.Present] + c[triage.Absent] + c[triage.NotEvaluable]
		t.Logf("%-38s %8d %8d %14d %8d", l, c[triage.Present], c[triage.Absent], c[triage.NotEvaluable], sum)
		if sum != len(sessions) {
			t.Errorf("locus %s has a denominator of %d over %d runs", l, sum, len(sessions))
		}
	}
	if findings != len(sessions)*len(triage.Loci) {
		t.Errorf("%d findings over %d runs, want %d", findings, len(sessions),
			len(sessions)*len(triage.Loci))
	}
	t.Logf("evaluable findings: %d of %d (Report.Evaluable summed over the population)",
		evaluable, findings)

	// F271: locus written_not_arrived is not answerable from jetsapi at all, so
	// its denominator is a population and its evaluable count is zero, wherever
	// this runs.
	if n := locusCensus[triage.LocusWrittenNotArrived][triage.NotEvaluable]; n != len(sessions) {
		t.Errorf("locus %s was evaluable on %d of %d runs; F271 says it is answerable on none",
			triage.LocusWrittenNotArrived, len(sessions)-n, len(sessions))
	}

	t.Logf("cause-class census over the same %d run(s): floor hypotheses per class", len(sessions))
	t.Logf("%-30s %14s %6s %12s", "cause class", "evidenceable", "loci", "hypotheses")
	structurallyZero, counted := 0, 0
	for _, c := range Classes() {
		t.Logf("%-30s %14s %6d %12d", c.Name, c.Evidenceability, len(c.Loci), classCensus[c.Name])
		counted += classCensus[c.Name]
		if len(c.Loci) != 0 {
			continue
		}
		// A class the gate attaches to no locus has no evidence position in any
		// run, so its denominator is structurally zero: the floor cannot emit
		// one however many runs it reads (F395).
		structurallyZero++
		if classCensus[c.Name] != 0 {
			t.Errorf("class %s is attached to no locus and the floor emitted %d hypotheses for it",
				c.Name, classCensus[c.Name])
		}
	}
	if structurallyZero != 2 {
		t.Errorf("%d classes are attached to no locus, want 2 (F395); I-262 counts three classes the "+
			"record cannot evidence and one of those three does have a locus", structurallyZero)
	}
	// **The rows above have to account for every hypothesis, not merely be
	// non-empty, and `counted` sums the rows rather than the map.** Two versions
	// of this guard were written and a control defeated both. The first asked
	// whether the floor emitted anything at all; the second summed the census
	// map. A control keying the census on the hypothesis's free-text `Cause`
	// instead of its `CauseCategory` passed them both — every class row read
	// zero, the two structurally-zero classes read zero along with them, and the
	// map's total was unchanged because the same hypotheses were still being
	// counted under different keys. Summing what is **printed** is what makes a
	// zero in a class row a measurement rather than an accident of the key.
	if counted != hypotheses || hypotheses == 0 {
		t.Fatalf("the class census accounts for %d of the %d hypotheses the floor emitted, so a zero "+
			"in any class row says nothing", counted, hypotheses)
	}

	// The other half of the class denominator: two loci are in no row of §9.5,
	// so a run exhibiting either produces no hypothesis for it (F394). A census
	// that did not say so would report a zero that looks like a measurement.
	unmapped := 0
	for _, l := range triage.Loci {
		if len(ClassesFor(l)) == 0 {
			unmapped++
			t.Logf("locus %s maps to no cause class, so it contributes to no class denominator", l)
		}
	}
	if unmapped != 2 {
		t.Errorf("%d loci map to no cause class, want 2 (F394)", unmapped)
	}
}

// The model against the rule — plan §10.6's independence route, and the only
// labelled comparison this phase can make without inventing labels.
//
// **It asserts nothing about the model.** Every count is a property of the
// answer; the label it is scored against is AC.1's deterministic verdict, which
// is a rule rather than a truth. R-49 is the standing warning that a table of
// counts with a model name on it reads as an evaluation, and the reason this
// prints its denominators beside every figure.
func TestTheModelAgainstTheDeterministicLocus(t *testing.T) {
	model := os.Getenv("JETS_RCA_MODEL")
	if model == "" {
		t.Skip("JETS_RCA_MODEL not set; needs an inference server and a named model")
	}
	pool := freshDB(t, "rca_ad1_model")
	sessions := ad1Census(t, pool)

	host := os.Getenv("JETS_INFER_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	client := &infer.Client{Host: host, Model: model, RequestTimeout: 20 * time.Minute, MaxRetry: -1}
	ctx := context.Background()

	var tot ConsultReport
	tot.Model = model
	calls, answered, produced, admitted := 0, 0, 0, 0
	// byLocus counts the model's hypotheses at each locus, split by what AC.1
	// said about that locus in the same run.
	byLocus := map[string]map[triage.Verdict]int{}
	for _, l := range triage.Loci {
		byLocus[l] = map[triage.Verdict]int{}
	}

	for _, s := range sessions {
		ev, err := triage.Gather(ctx, pool, s, 0)
		if err != nil {
			t.Fatalf("triage.Gather(%s): %v", s, err)
		}
		rep := triage.Default().Classify(ev)
		anomalies, err := observe.ReadAnomalies(ctx, pool, s)
		if err != nil {
			t.Fatalf("observe.ReadAnomalies(%s): %v", s, err)
		}
		in := &Input{Report: rep, Evidence: ev, Anomalies: anomalies}

		verdict := map[string]triage.Verdict{}
		for i := range rep.Findings {
			verdict[rep.Findings[i].Locus] = rep.Findings[i].Verdict
		}

		start := time.Now()
		got, cr, err := Default().Consult(ctx, client, model, in)
		if err != nil {
			t.Fatalf("Consult(%s): %v", s, err)
		}
		calls++
		t.Logf("%s (%.0fs): %s", s, time.Since(start).Seconds(), cr.Describe())
		if !cr.Answered {
			continue
		}
		answered++
		produced += cr.Hypotheses
		admitted += len(got.Hypotheses)
		for i := range got.Hypotheses {
			h := &got.Hypotheses[i]
			if v, ok := verdict[h.Locus]; ok {
				byLocus[h.Locus][v]++
			}
			t.Logf("  %d. %-26s at %-34s [triage said %s] conf %.2f (+%d/-%d)", h.Rank,
				h.CauseCategory, h.Locus, verdict[h.Locus], h.Confidence,
				len(h.SupportingEvidence), len(h.ContradictingEvidence))
		}
		tot.Sessions++
		tot.Answered = true
		tot.Hypotheses += cr.Hypotheses
		tot.WithContradicting += cr.WithContradicting
		tot.SupportingItems += cr.SupportingItems
		tot.ContradictingItems += cr.ContradictingItems
		tot.UnsubstantiatedSources += cr.UnsubstantiatedSources
		tot.ClassesWithNoLocus += cr.ClassesWithNoLocus
		tot.PairsOutsideTheTable += cr.PairsOutsideTheTable
		tot.LociNotPresent += cr.LociNotPresent
		tot.LociAbsent += cr.LociAbsent
		tot.LociNotEvaluable += cr.LociNotEvaluable
		tot.PromptTokens += cr.PromptTokens
		tot.EvalTokens += cr.EvalTokens
		tot.FloorHypotheses += cr.FloorHypotheses
		tot.FloorWithContradicting += cr.FloorWithContradicting
		tot.FloorUnsubstantiatedSource += cr.FloorUnsubstantiatedSource
	}

	// Tier A, at the two grains it has. §11.3.1 defines the Schema Rejection
	// Rate as Tier A failures over total outputs and §11.1.3 defines Tier A as
	// schema conformance, referential integrity and lifecycle legality — so an
	// answer and a hypothesis are both outputs, and the two denominators are
	// different numbers rather than one reported twice.
	t.Logf("Tier A over model %s: %d call(s), %d admitted by the schema, %d refused; "+
		"%d hypotheses produced, %d admitted by Hypothesis.Validate, %d refused",
		model, calls, answered, calls-answered, produced, admitted, produced-admitted)

	t.Logf("model hypotheses per locus, against what AC.1 said about that locus in the same run "+
		"(denominator %d admitted hypotheses over %d run(s)); this is a model against a rule and is "+
		"not an accuracy", admitted, len(sessions))
	t.Logf("%-38s %8s %8s %14s", "locus", "present", "absent", "not_evaluable")
	names := make([]string, 0, len(byLocus))
	names = append(names, triage.Loci...)
	sort.SliceStable(names, func(i, j int) bool { return locusOrder(names[i]) < locusOrder(names[j]) })
	for _, l := range names {
		c := byLocus[l]
		if c[triage.Present]+c[triage.Absent]+c[triage.NotEvaluable] == 0 {
			continue
		}
		t.Logf("%-38s %8d %8d %14d", l, c[triage.Present], c[triage.Absent], c[triage.NotEvaluable])
	}

	tot.SessionId = "the AD.1 census"
	t.Logf("TOTAL over %d answered session(s): %s", tot.Sessions, tot.Describe())

	// The instrument's own invariant, asserted even here so that a figure quoted
	// from this run is a decomposition rather than two independent counts.
	if tot.LociAbsent+tot.LociNotEvaluable != tot.LociNotPresent {
		t.Errorf("LociAbsent %d + LociNotEvaluable %d is not LociNotPresent %d",
			tot.LociAbsent, tot.LociNotEvaluable, tot.LociNotPresent)
	}
}
