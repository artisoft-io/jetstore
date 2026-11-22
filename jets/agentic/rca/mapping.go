package rca

import (
	"slices"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The gate's table, transcribed.
//
// The gate's analysis put the imported ten cause classes against the nine loci
// and answered, per class, *which loci can carry it* and *can the record
// evidence the class*. That table is the whole of what the deterministic floor
// knows about causes, and it is transcribed here rather than re-derived: the
// floor's value is that what it scores is the gate's own reading, so that a gap
// in the floor is a finding about the table rather than about this file.
//
// **Two consequences of transcribing rather than improving.** Loci
// step_never_started and per_record_failures_unreportable appear in no row of
// the table, so the floor emits no hypothesis when either fires and says so in
// Ranking.UnmappedLoci. And benign_variation's cell reads *"6 above all"*,
// which is not a closed list; it is transcribed as the one locus it names, with
// the wording kept in the note so a reader can see that the table hedged.

// Evidenceability is the gate table's evidence column, reduced to five values. The exact
// wording of each cell is kept in Note, because the reduction is this file's
// and the wording is the gate's.
type Evidenceability string

const (
	// Evidenced — the record does this class well. One class only.
	Evidenced Evidenceability = "evidenced"
	// Conditional — evidenced only when a configuration switch is set, or,
	// for the two source classes, when a feed's profile declares what the
	// class is judged against.
	Conditional Evidenceability = "conditional"
	// Coarse — evidenced at a grain coarser than the class name implies.
	Coarse Evidenceability = "coarse"
	// Asymmetric — the class can be proposed and not established.
	Asymmetric Evidenceability = "asymmetric"
	// None — the record cannot evidence the class at all, or can
	// evidence the locus and not the cause.
	None Evidenceability = "none"
)

// Evidenceabilities is the vocabulary in its sort order, better-evidenced
// first. It mirrors the model's `Evidenceability`, which was added at the user's
// decision so that the tier a rank is computed from is persisted with the row rather than
// recovered by re-running a ranker that is not deterministic on its model arm.
var Evidenceabilities = []Evidenceability{Evidenced, Conditional, Coarse, Asymmetric, None}

// EvidenceabilityOf returns the gate table's answer for a cause class, or None for a
// hypothesis that names no class.
//
// **None is the honest value for an unnamed class rather than a fallback.** A
// hypothesis with no `cause_category` makes no claim in the imported vocabulary
// at all, so there is no table row to read — and the tier that says *the record
// cannot evidence this* is the one that ranks it last, which is where a claim
// nobody can check belongs.
func EvidenceabilityOf(causeCategory string) Evidenceability {
	for i := range causeClasses {
		if causeClasses[i].Name == causeCategory {
			return causeClasses[i].Evidenceability
		}
	}
	return None
}

// CauseClass is one row of the gate's table.
type CauseClass struct {
	// Name is the IncidentClassification member.
	Name string
	// Loci are the table's "loci that can carry it", in locus order.
	Loci []string
	// Evidenceability is the reduction of the table's evidence column.
	Evidenceability Evidenceability
	// Note is that column's own words, which are what a contradicting evidence
	// item quotes. It is written to be read after "the record cannot fully
	// evidence this class: ".
	Note string

	// DeclaredAt, for a class graded Conditional on a feed's declaration, are
	// the loci at which that declaration is read: the delivery loci. At every
	// other locus the class has no declaration to be judged against and takes
	// the grade Elsewhere. Empty for every other class, which grades the same
	// at every locus.
	DeclaredAt []string
	// Elsewhere is the grade at a locus not in DeclaredAt.
	Elsewhere Evidenceability
}

// At is the class's grade at one locus: Evidenceability, or Elsewhere where the
// class's condition is a declaration this locus does not read.
//
// **The grade is per locus for exactly one reason, and it is the one that kept
// source_delivery_failure ranked last on a run that never started.** A declared
// cadence evidences the class at delivery_overdue, where feed_overdue compared
// the feed's registrations with it. Nothing links a run's header to a feed's
// cadence, so at run_not_started the record still cannot say what was due, and
// a class-wide re-grade would have lifted the class into the evidenceable tier
// there on the strength of a declaration nobody read.
func (c CauseClass) At(locus string) Evidenceability {
	if len(c.DeclaredAt) == 0 || slices.Contains(c.DeclaredAt, locus) {
		return c.Evidenceability
	}
	return c.Elsewhere
}

// EvidenceabilityAt returns the table's grade for a cause class at a locus, or
// None for a hypothesis that names no class. It is what a written hypothesis's
// basis records, because the grade a hypothesis was ranked by is the grade at
// its own locus.
func EvidenceabilityAt(causeCategory, locus string) Evidenceability {
	for i := range causeClasses {
		if causeClasses[i].Name == causeCategory {
			return causeClasses[i].At(locus)
		}
	}
	return None
}

// causeClasses is the gate's table, in its own order.
var causeClasses = []CauseClass{
	{
		Name: CauseSourceDeliveryFailure,
		Loci: []string{triage.LocusRunNotStarted, triage.LocusDeliveryOverdue},
		// **Conditional, evidenced when a cadence is declared.** The table graded
		// this class none, for want of anything recording what was due. A feed
		// profile's period type and grace are that record, and feed_overdue is
		// the comparison; so at delivery_overdue the class is evidenced exactly
		// as far as a cadence was declared. At run_not_started it is still none.
		Evidenceability: Conditional,
		DeclaredAt:      []string{triage.LocusDeliveryOverdue},
		Elsewhere:       None,
		Note: "it is evidenced only where a feed's profile declares a cadence: the period type and grace say " +
			"what was due, and the overdue check compares the feed's registrations with them, so at locus " +
			"delivery_overdue the declaration is what was owed. Anywhere else input_registry and " +
			"file_key_staging record what arrived and nothing records what was due; there is no schedule, " +
			"no SLA and no expected-delivery table, so \"did not arrive\" is the absence of a row with nothing " +
			"asserting a row was owed",
	},
	{
		Name: CauseSourceContentChange,
		Loci: []string{triage.LocusRowsLostSilently, triage.LocusPerRecordFailuresReported,
			triage.LocusDeliveryDepartsProfile},
		// **Conditional, at column grain when a profile names the column.** The
		// table graded this class coarse and its note ended "column-level change
		// needs a profile". At delivery_departs_profile there is one: a check
		// that compares a column the profile names — a required field, the
		// service date, the business key — evidences the change at that
		// column. At the run loci it stays coarse.
		Evidenceability: Conditional,
		DeclaredAt:      []string{triage.LocusDeliveryDepartsProfile},
		Elsewhere:       Coarse,
		Note: "it is evidenced at column grain only where a feed's profile names the column a delivery check " +
			"compared. Elsewhere volume and file size move on the worker row, but input_bad_records_count is " +
			"read-time parse failure only and is 0 for parquet by construction; column-level change needs a " +
			"profile, and the analyze operator runs in 4 of 47 configurations into a destination the " +
			"config chooses",
	},
	{
		Name: CauseTransportFailure,
		Loci: []string{triage.LocusRunNotStarted, triage.LocusWorkerFailed,
			triage.LocusSinkFailedUnderCompletedWorker, triage.LocusWrittenNotArrived},
		Evidenceability: Coarse,
		Note: "the text reaches failure_details or an error_message and the class does not, and the " +
			"one machine-readable class the platform computes is discarded before the database; " +
			"locus written_not_arrived is the only structural instrument and it is a check against S3 or " +
			"Postgres rather than a query",
	},
	{
		Name:            CauseParseFailure,
		Loci:            []string{triage.LocusRowsLostSilently, triage.LocusPerRecordFailuresReported},
		Evidenceability: Evidenced,
		Note: "input_bad_records_count is exactly this and it counts without sampling; the one exclusion is " +
			"that an unparseable date inside map_record is discarded with no row, no log line " +
			"and no count",
	},
	{
		Name:            CauseValidationBreach,
		Loci:            []string{triage.LocusPerRecordFailuresReported},
		Evidenceability: Conditional,
		Note: "a jets:exception reaches process_errors for the 9 of 12 jetrules instances that wire a " +
			"channel, capped at 20 per operator instance, and without saying which rule " +
			"(the message is the only populated column)",
	},
	{
		Name: CauseTransformationDefect,
		// delivery_departs_profile is here because a departure can be the
		// mapping's rather than the supplier's: a configuration that leaves a
		// column unmapped makes a required field look absent from every
		// delivery, and the floor has to offer that reading beside the source's.
		Loci: []string{triage.LocusWorkerNotTerminated, triage.LocusWorkerFailed,
			triage.LocusRowsLostSilently, triage.LocusPerRecordFailuresReported,
			triage.LocusDeliveryDepartsProfile},
		// **The table answers "no" here and a later finding supersedes it, so this row is
		// Coarse rather than None.** The table's reason was that establishing what
		// changed requires binding a run to a workspace version and nothing
		// does. pipeline_execution_status gained a workspace_name and a
		// workspace_version the same day, and jetsapi.workspace_version gained
		// a workspace_commit; the later finding records the consequence in
		// terms: *a run names a workspace and a compiled version, two runs of
		// the same step under different versions are now distinguishable, and
		// what changed inside those versions is still not answerable*. That is
		// the definition of Coarse.
		//
		// **The classification is taken from that finding rather than decided here.**
		// Reclassifying a row of the gate's table on this task's authority would
		// be the unreviewed extraction the gate exists to prevent; what this file does is
		// apply a correction already made elsewhere and
		// record that the table's own cell has not been edited.
		Evidenceability: Coarse,
		Note: "the locus yes, the cause coarsely: the StepRegression detector establishes that this step used to " +
			"work, and a run now names a workspace and a compiled version, which that " +
			"version now pairs with a commit — so two runs of the same step under different " +
			"versions are distinguishable and what changed inside a version is not. **The join " +
			"is through a read that is known wrong**: four call sites resolve the workspace version as " +
			"MAX(version) with no workspace predicate, so a deployment with two workspaces can stamp a " +
			"run with the other one's version",
	},
	{
		Name: CauseInfrastructureFailure,
		Loci: []string{triage.LocusRunNotStarted, triage.LocusWorkerNotTerminated,
			triage.LocusWorkerFailed},
		Evidenceability: Coarse,
		Note: "the ECS StoppedReason arm of the decoder is real and which of its four arms fired is not " +
			"recorded, and locus worker_not_terminated collapses a killed task, an out-of-memory " +
			"kill, a lost node and a hung read into one observation",
	},
	{
		Name:            CauseDependencyFailure,
		Loci:            nil,
		Evidenceability: None,
		Note: "there is no dependency graph of any kind; the cross-step DAG join is available for at most " +
			"one hop and only for stage-mediated hops, and external dependencies are modelled " +
			"nowhere",
	},
	{
		Name:            CauseCapacityOrCostDeviation,
		Loci:            nil,
		Evidenceability: None,
		Note: "no cost, no memory, no CPU, and the only duration is one wall clock per worker that " +
			"includes queueing on the header",
	},
	{
		Name: CauseBenignVariation,
		// The two delivery loci are here because the declaration may be wrong:
		// a departure from a stale declaration is not a fault in the feed.
		Loci: []string{triage.LocusRowsLostSilently, triage.LocusDeliveryOverdue,
			triage.LocusDeliveryDepartsProfile},
		Evidenceability: Asymmetric,
		Note: "the class can be proposed and not established: confirming benign needs the absence of a " +
			"cause, which no record supplies, and ConfigConfounders.Read == false is the ordinary state " +
			"for the runs that failed earliest. the triage design's cell reads \"6 above all\", which is not a " +
			"closed list of loci",
	},
}

// ClassesFor returns the cause classes the table says a locus can carry, in the
// table's order. It returns nothing for step_never_started and
// per_record_failures_unreportable, which appear in no row.
//
// The two delivery loci map to three classes each, not one: delivery_overdue
// to source_delivery_failure and benign_variation, delivery_departs_profile to
// source_content_change, transformation_defect and benign_variation.
func ClassesFor(locus string) []CauseClass {
	var out []CauseClass
	for i := range causeClasses {
		if slices.Contains(causeClasses[i].Loci, locus) {
			out = append(out, causeClasses[i])
		}
	}
	return out
}

// Classes returns the gate's table.
func Classes() []CauseClass { return slices.Clone(causeClasses) }

// classOrder is a class's position in the table, used as the last tie-break so that
// two rankings over identical evidence agree.
func classOrder(name string) int {
	for i := range causeClasses {
		if causeClasses[i].Name == name {
			return i
		}
	}
	return len(causeClasses)
}

// locusOrder is a locus's position in onset order — how early the failure
// occurs — used as the first tie-break.
//
// **It is not the vocabulary's order, and the difference is the two delivery
// loci.** The vocabulary is appended to and never renumbered, so they sit last
// there, although a delivery precedes the run that loads it. Onset order puts
// them first: a delivery that never came retains the least, as run_not_started
// does among the run loci, and a delivery that departs from its profile comes
// before anything a run does with it. The nine run loci keep their relative
// order, so a ranking over a run alone is unchanged. The tie can arise only on
// a session that is both a delivery and a run, which is where a delivery
// finding should be read first.
func locusOrder(locus string) int {
	if i := slices.Index(onsetOrder, locus); i >= 0 {
		return i
	}
	return len(onsetOrder)
}

var onsetOrder = append(slices.Clone(triage.DeliveryLoci), runLoci()...)

func runLoci() []string {
	var out []string
	for _, l := range triage.Loci {
		if !slices.Contains(triage.DeliveryLoci, l) {
			out = append(out, l)
		}
	}
	return out
}

// sameRecord reports whether two loci are positions in the same record: both
// in a run's execution record, or both in a feed's deliveries. A class's other
// positions are compared only within one record, because a run's session is
// never asked about a delivery locus and a registration session rarely has a
// run, so the other record's verdicts would charge every hypothesis an item
// for a question nobody put to that session.
func sameRecord(a, b string) bool {
	return slices.Contains(triage.DeliveryLoci, a) == slices.Contains(triage.DeliveryLoci, b)
}

// --- the confounder split ----------------------------------------------------

// A confounder is not contradicting evidence for every hypothesis, and the
// axis that decides is already in observe.
//
// **AnomalyConfounder's fourteen members are two kinds of thing.** Nine are
// limits on what the record could see — a truncated history, a stage label that
// is not a step identity, a stall whose cause is unrecorded — and they weaken
// *every* claim about the run, benign included, because they say the comparison
// was bounded. Five are *configured behaviours that would produce the
// observation on purpose*: on_error_drop, max_input_count, sampling_cap,
// device_writer_output and parquet_input. Those are evidence **for**
// benign_variation and against everything else, because a configured drop is an
// explanation rather than a doubt.
//
// **The split is observe.RecordConfounders' complement and it is asserted
// rather than assumed.** That list exists for a different reason — which
// confounders the execution record can establish without reading a config — and
// the two axes coincide today by construction rather than by guarantee:
// something a detector can only learn from the configuration is, so far,
// exactly something the configuration chose to do. TestConfounderSplitIsThe
// ComplementOfRecordConfounders fails the day that stops being true, which is
// the point of writing it down rather than calling observe.RecordConfounders
// directly.
var configuredBehaviourConfounders = []string{
	observe.ConfounderOnErrorDrop,
	observe.ConfounderMaxInputCount,
	observe.ConfounderSamplingCap,
	observe.ConfounderDeviceWriterOutput,
	observe.ConfounderParquetInput,
}

// benignExplanation reports whether a confounder is a configured behaviour that
// explains the observation rather than a limit on seeing it.
func benignExplanation(name string) bool {
	return slices.Contains(configuredBehaviourConfounders, name)
}

// --- the anomaly signal map --------------------------------------------------

// The (subject, signal) → locus table is triage's (triage.LocusOf), because
// triage places a feed anomaly to decide whether a delivery locus holds and the
// ranker places the same anomaly to cite it: one question, one table.

// evidenceabilityOf returns the table's evidence-column answer for a class name.
func evidenceabilityOf(name string) Evidenceability {
	for i := range causeClasses {
		if causeClasses[i].Name == name {
			return causeClasses[i].Evidenceability
		}
	}
	return None
}

// Evidenceable reports whether the table answers anything other than *no* for this
// class. It is the primary sort key of a ranking, and the reason is a defect the
// floor's first run produced rather than a preference.
//
// **Counting alone ranked a class the record cannot evidence at all first.** On
// a run whose header was terminal with no worker row, source_delivery_failure
// scored 1 supporting against 1 contradicting — 0.50 — and outranked
// transport_failure at 0.40, because transport_failure sits at four loci and was
// charged one contradicting item for each of the three that did not fire.
// **The ratio punishes a class for having more evidence positions and rewards a
// class for having one**, which is the opposite of what the positions are worth.
//
// The fix is not a weight. The table's own evidence column already answers *can the
// record evidence this class*, and *no* is a different kind of answer from a
// small number: a class the record cannot evidence is not a weak candidate, it
// is one the substrate cannot speak to. So it is a two-tier partition read off
// the gate's table, with the ratio ordering within each tier — no coefficient,
// and nothing that needs sourcing.
func (c CauseClass) Evidenceable() bool { return c.Evidenceability != None }
