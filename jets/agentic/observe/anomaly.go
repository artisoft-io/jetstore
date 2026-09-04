package observe

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// The Anomaly vocabularies. The generated CHECK constraints in
// jets/agentic/audit/agent_audit.sql are the authority; TestVocabulariesMatchDDL
// asserts these constants against them so a drift fails the suite rather than a
// production insert — which is the same guard audit.TestEventTypes gives the
// audit store's event taxonomy.
//
// Three of the signal types and two of the subject types extend the source
// proposal's Appendix A.2.6, because its ten signals are a warehouse's
// data-quality signals and cannot name a step that regressed, a worker that
// stalled or data that did not arrive (N.2, I-127).
const (
	SignalVolume         = "volume"
	SignalFreshness      = "freshness"
	SignalSchema         = "schema"
	SignalDistribution   = "distribution"
	SignalRuleBreach     = "rule_breach"
	SignalCost           = "cost"
	SignalDuration       = "duration"
	SignalRejectionRate  = "rejection_rate"
	SignalCardinality    = "cardinality"
	SignalReferential    = "referential"
	SignalStepRegression = "step_regression"
	SignalWorkerStall    = "worker_stall"
	SignalArrival        = "arrival"
)

const (
	SubjectFeed     = "feed"
	SubjectPipeline = "pipeline"
	SubjectStage    = "stage"
	SubjectRun      = "run"
	SubjectTable    = "table"
	SubjectColumn   = "column"
	SubjectWorker   = "worker"
	SubjectEdge     = "edge"
)

const (
	ConfounderParseErrorsOnly          = "parse_errors_only"
	ConfounderParquetInput             = "parquet_input"
	ConfounderOnErrorDrop              = "on_error_drop"
	ConfounderMaxInputCount            = "max_input_count"
	ConfounderSamplingCap              = "sampling_cap"
	ConfounderDeviceWriterOutput       = "device_writer_output"
	ConfounderMergeRowCountUnknown     = "merge_row_count_unknown"
	ConfounderStepLabelAmbiguous       = "step_label_ambiguous"
	ConfounderStallCauseUnknown        = "stall_cause_unknown"
	ConfounderCrossStepJoinUnavailable = "cross_step_join_unavailable"
	ConfounderHistoryTruncated         = "history_truncated"
	ConfounderNoPhysicalLocation       = "no_physical_location"
	ConfounderStagePrefixReused        = "stage_prefix_reused"
	ConfounderLocationAimedNotReached  = "location_aimed_not_reached"
)

var (
	signalTypes = []string{
		SignalVolume, SignalFreshness, SignalSchema, SignalDistribution,
		SignalRuleBreach, SignalCost, SignalDuration, SignalRejectionRate,
		SignalCardinality, SignalReferential, SignalStepRegression,
		SignalWorkerStall, SignalArrival,
	}
	subjectTypes = []string{
		SubjectFeed, SubjectPipeline, SubjectStage, SubjectRun,
		SubjectTable, SubjectColumn, SubjectWorker, SubjectEdge,
	}
	confounders = []string{
		ConfounderParseErrorsOnly, ConfounderParquetInput, ConfounderOnErrorDrop,
		ConfounderMaxInputCount, ConfounderSamplingCap, ConfounderDeviceWriterOutput,
		ConfounderMergeRowCountUnknown, ConfounderStepLabelAmbiguous,
		ConfounderStallCauseUnknown, ConfounderCrossStepJoinUnavailable,
		ConfounderHistoryTruncated, ConfounderNoPhysicalLocation,
		ConfounderStagePrefixReused, ConfounderLocationAimedNotReached,
	}
)

// RecordConfounders are the confounders the execution record can establish on
// its own. The rest — on_error_drop, max_input_count, sampling_cap,
// device_writer_output and parquet_input — are properties of a pipeline's
// configuration and appear nowhere in the record, so a detector that needs one
// has to read it from the config and cannot get it here.
var RecordConfounders = []string{
	ConfounderStepLabelAmbiguous, ConfounderStallCauseUnknown,
	ConfounderCrossStepJoinUnavailable, ConfounderHistoryTruncated,
	ConfounderMergeRowCountUnknown, ConfounderNoPhysicalLocation,
	ConfounderStagePrefixReused, ConfounderLocationAimedNotReached,
	ConfounderParseErrorsOnly,
}

// IsConfounder reports whether name is in the confounder vocabulary.
//
// It is exported for jets/agentic/triage (AC.1), which writes the same fourteen
// members onto an incident. jetsapi.incident's incident_confounders_ck admits
// exactly this vocabulary, so a second list there would be a second thing to
// keep in step and, worse, would make an incident's qualifiers incomparable
// with those of the anomaly that gave rise to it.
func IsConfounder(name string) bool { return slices.Contains(confounders, name) }

// Anomaly is one row of jetsapi.anomaly, the thirteen properties N.2 gave
// jetsa:Anomaly. The three nullable fields are nullable because four of the
// six derivable failure modes are within-run predicates with no range and no
// magnitude (I-126): a worker that never reached a terminal state has nothing
// to be an expected minimum of.
type Anomaly struct {
	AnomalyId  string
	DetectedAt time.Time
	SessionId  string

	SubjectType string // one of the Subject* constants
	SubjectRef  string
	SignalType  string // one of the Signal* constants

	ObservedValue string
	ExpectedMin   *string
	ExpectedMax   *string
	// ExpectedBasis says what the comparison was against, in words. It is
	// required, and BaselineSet.Describe is what a windowed detector should
	// put in it.
	ExpectedBasis string

	DeviationMagnitude *float64
	// Confounders is what the detector could not rule out. It is a closed
	// vocabulary rather than free text, so two detectors' qualifiers are
	// comparable; an empty slice is a claim, not an omission.
	Confounders []string
	// DetectorRef names the detector and its generation, so two generations of
	// the same detector are told apart.
	DetectorRef string
}

// Validate checks the vocabularies and the required fields in Go, before the
// CHECK constraints do it in Postgres. The database is the authority; this
// exists so the message names the field rather than the constraint.
func (a *Anomaly) Validate() error {
	if a.AnomalyId == "" {
		return fmt.Errorf("anomaly_id is required")
	}
	if a.SessionId == "" {
		return fmt.Errorf("anomaly_session_id is required")
	}
	if a.SubjectRef == "" {
		return fmt.Errorf("anomaly_subject_ref is required")
	}
	if a.ObservedValue == "" {
		return fmt.Errorf("anomaly_observed_value is required")
	}
	if a.ExpectedBasis == "" {
		return fmt.Errorf("anomaly_expected_basis is required: an anomaly that cannot say what it compared against is the signal an operator learns to ignore")
	}
	if a.DetectorRef == "" {
		return fmt.Errorf("anomaly_detector_ref is required")
	}
	if !slices.Contains(signalTypes, a.SignalType) {
		return fmt.Errorf("anomaly_signal_type %q is not in the vocabulary", a.SignalType)
	}
	if !slices.Contains(subjectTypes, a.SubjectType) {
		return fmt.Errorf("anomaly_subject_type %q is not in the vocabulary", a.SubjectType)
	}
	for _, c := range a.Confounders {
		if !slices.Contains(confounders, c) {
			return fmt.Errorf("anomaly_confounders holds %q, which is not in the vocabulary", c)
		}
	}
	return nil
}

const insertAnomalySQL = `INSERT INTO jetsapi.anomaly (
    anomaly_id, detected_at, anomaly_session_id,
    anomaly_subject_type, anomaly_subject_ref, anomaly_signal_type,
    anomaly_observed_value, anomaly_expected_min, anomaly_expected_max,
    anomaly_expected_basis, anomaly_deviation_magnitude, anomaly_confounders,
    anomaly_detector_ref)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

// InsertAnomaly is the "Go emitter" half of N.3's decided shape. It takes an
// Exec rather than a pool so a detector can write inside the transaction that
// read the evidence.
//
// jetsapi.anomaly is created by `update_db -migrateDb`, which calls
// audit.InstallSchema (jets/update_db/main.go:71) — the same command that
// creates jetsapi.pipeline_execution_channel_details, and which no production
// environment measured on 2026-08-25 had run. ReadExtent reports whether it is
// there.
func InsertAnomaly(ctx context.Context, db Exec, a *Anomaly) error {
	if err := a.Validate(); err != nil {
		return fmt.Errorf("invalid anomaly %q: %w", a.AnomalyId, err)
	}
	detectedAt := a.DetectedAt
	if detectedAt.IsZero() {
		detectedAt = time.Now().UTC()
	}
	conf := a.Confounders
	if conf == nil {
		conf = []string{}
	}
	_, err := db.Exec(ctx, insertAnomalySQL,
		a.AnomalyId, detectedAt, a.SessionId,
		a.SubjectType, a.SubjectRef, a.SignalType,
		a.ObservedValue, a.ExpectedMin, a.ExpectedMax,
		a.ExpectedBasis, a.DeviationMagnitude, conf,
		a.DetectorRef)
	if err != nil {
		return fmt.Errorf("while inserting anomaly %q: %w", a.AnomalyId, err)
	}
	return nil
}
