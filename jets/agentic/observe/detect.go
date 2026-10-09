package observe

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// The two detectors, section 12.2 rows 2 and 3. Which two is a measurement
// rather than a preference: row 1's numerator sums to zero across four
// production environments and 22.6 billion input records (F72), and row 4's
// predicate fires four times in thirty days across all four, so a family
// cannot be proven on it. Rows 5 and 10 read
// jetsapi.pipeline_execution_channel_details, which is deployed nowhere.
//
// Two rather than one because one detector proves the path and two prove it is
// a family. The two chosen differ on the axis that matters for the shape N.3
// decided: row 2 is a predicate over one row's own columns and row 3 is a
// comparison against a windowed aggregate, so between them they exercise both
// halves of "SQL plus a Go emitter".
//
// # Neither threshold varies by domain class, which is the load-bearing property
//
// Section 12.4's decision is reversible on one condition: a threshold that
// depends on what an Anomaly is *about* rather than on which column moved. Both
// detectors here take a scalar that is the same for every client, process and
// object type — a ratio, a minimum input, a minimum history — so the condition
// does not fire and the SQL shape stands. A reader checking that claim should
// check it against the fields of the two structs below, which is the whole of
// what either takes.
//
// # The thresholds are unsourced and are flagged rather than defended
//
// Section 12.5 measured the population, the retention regimes and the signal
// availability. It did not measure the distribution of output/input ratios,
// because no query asked for one, so nothing in this repository says what a
// normal ratio is for a JetStore step. The defaults below are starting points
// with a stated bias — expensive to miss, cheap to over-report — and the first
// deployment to run this is what replaces them.

// Evidence is one read of the execution record, and what the read could not
// see. Both detectors take one rather than a list of arguments, and that is
// the shape R-19 was open about: the extraction's four exported functions
// turned out to be the right reads, and what a detector wanted that was not
// there was the composition of them — a baseline window that provably excludes
// the run being judged, an Extent beside the rows rather than inside one of
// the two results, and the configuration of each session the rows came from.
type Evidence struct {
	// Extent is the deployment's own limits, read once. A detector consults it
	// for what it could not see rather than for what it found.
	Extent *Extent
	// Workers are the rows to judge.
	Workers *WorkerSet
	// Prior is the baseline StepRegression compares against. Its window ends
	// where Workers' window begins. Nil when no baseline was asked for.
	Prior *BaselineSet
	// Configs is one entry per distinct session in Workers, present whether or
	// not the configuration could be read — the absence of an entry and an
	// entry reporting Read == false are different claims and only the second
	// is honest about having asked.
	Configs map[string]*ConfigConfounders
}

// Gather reads everything the two detectors need in one call.
//
// detect bounds the rows to judge. baselineFor is how far back before
// detect.Since the baseline reaches; zero means no baseline is read, which is
// legitimate — row 2 needs none — and leaves Prior nil.
//
// The baseline window is closed at detect.Since by construction rather than by
// the caller's arithmetic, which is what makes StepRegression's non-overlap
// check something a caller cannot fail by accident. It also drops the session
// filter: a baseline restricted to the run being judged is not a baseline.
func Gather(ctx context.Context, db DB, detect Window, baselineFor time.Duration) (*Evidence, error) {
	ev := &Evidence{Configs: map[string]*ConfigConfounders{}}
	var err error
	if ev.Extent, err = ReadExtent(ctx, db); err != nil {
		return nil, err
	}
	if ev.Workers, err = Workers(ctx, db, detect); err != nil {
		return nil, err
	}
	if baselineFor > 0 {
		prior := Window{
			Since:       detect.Since.Add(-baselineFor),
			Until:       detect.Since,
			Client:      detect.Client,
			ProcessName: detect.ProcessName,
		}
		if ev.Prior, err = StepBaselines(ctx, db, prior); err != nil {
			return nil, err
		}
	}
	for i := range ev.Workers.Rows {
		id := ev.Workers.Rows[i].SessionId
		if _, seen := ev.Configs[id]; seen {
			continue
		}
		cc, err := ReadConfigConfounders(ctx, db, id)
		if err != nil {
			return nil, err
		}
		ev.Configs[id] = cc
	}
	return ev, nil
}

// VolumeCollapse is section 12.2 row 2 at the worker grain: a step that
// completed and emitted materially fewer rows than it read.
//
// It fires only on a worker that reached 'completed'. A worker that failed has
// already reported itself through its status, and an anomaly beside that
// failure is the second copy of a signal rather than a new one; a worker still
// in progress has NULL counts (F99) and is row 4's observation, not this one.
type VolumeCollapse struct {
	// MinRatio fires the detector when output_records_count / input_records_count
	// is below it. Unsourced; see the file comment.
	MinRatio float64
	// MinInput ignores workers that read fewer rows than this. A ratio over a
	// handful of rows is arithmetic rather than evidence — one row of two is a
	// 50% collapse — and the corpus has steps that legitimately read a few
	// configuration rows. Unsourced.
	MinInput int64
	// Generation is the trailing number of the detector_ref. Bumping it makes
	// the emitter re-emit over rows it has already judged, which is what a
	// changed threshold needs and what a deterministic anomaly_id would
	// otherwise prevent.
	Generation int
}

// DefaultVolumeCollapse is the starting point, not a recommendation.
func DefaultVolumeCollapse() VolumeCollapse {
	return VolumeCollapse{MinRatio: 0.5, MinInput: 1000, Generation: 1}
}

// Ref is the detector_ref written on every anomaly this detector emits.
func (d VolumeCollapse) Ref() string { return fmt.Sprintf("volume_collapse@%d", d.Generation) }

// VolumeCollapseConfounders are the members of the confounder vocabulary that
// bear on this signal and cannot be read off the execution record. Four of the
// five config-sourced members are here; parquet_input is the fifth and is not
// among them, because the input format is a reason a *bad-record* count is
// zero (row 1) and not a reason an output count is low.
var VolumeCollapseConfounders = []string{
	ConfounderOnErrorDrop, ConfounderMaxInputCount,
	ConfounderSamplingCap, ConfounderDeviceWriterOutput,
}

// Detect judges every worker row in ev and returns the anomalies, in the
// record's own order. A session with no ConfigConfounders entry produces an
// anomaly whose basis says no configuration was consulted — the one thing
// I-168 rules out is saying nothing, because an empty anomaly_confounders is a
// claim rather than an omission.
func (d VolumeCollapse) Detect(ev *Evidence) []Anomaly {
	if ev == nil || ev.Workers == nil {
		return nil
	}
	set := ev.Workers
	var out []Anomaly
	for i := range set.Rows {
		r := &set.Rows[i]
		if r.Status != StatusCompleted {
			continue
		}
		// F99: the six counts are NULL for exactly as long as the worker is
		// running, and a coalesce written for tidiness here would turn a
		// stalled worker into a total collapse.
		if r.InputRecords == nil || r.OutputRecords == nil {
			continue
		}
		in, got := *r.InputRecords, *r.OutputRecords
		if in < d.MinInput {
			continue
		}
		ratio := float64(got) / float64(in)
		if ratio >= d.MinRatio {
			continue
		}

		cc := ev.Configs[r.SessionId]
		conf := d.recordConfounders(r, ev)
		conf = append(conf, cc.Bearing(VolumeCollapseConfounders)...)
		shortfall := 1 - ratio
		expectedMin := strconv.FormatInt(int64(d.MinRatio*float64(in)), 10)

		out = append(out, Anomaly{
			AnomalyId:          fmt.Sprintf("%s/%d", d.Ref(), r.Key),
			DetectedAt:         time.Now().UTC(),
			SessionId:          r.SessionId,
			SubjectType:        SubjectWorker,
			SubjectRef:         strconv.FormatInt(r.Key, 10),
			SignalType:         SignalVolume,
			ObservedValue:      strconv.FormatInt(got, 10),
			ExpectedMin:        &expectedMin,
			ExpectedBasis:      d.basis(r, in, got, cc),
			DeviationMagnitude: &shortfall,
			Confounders:        conf,
			DetectorRef:        d.Ref(),
		})
	}
	return out
}

// recordConfounders are the ones this worker row establishes on its own.
func (d VolumeCollapse) recordConfounders(r *WorkerRow, ev *Evidence) []string {
	var out []string
	if r.StepId == "" {
		// F52: ten reducing steps in the corpus carry an empty label, so the
		// anomaly cannot say which step of the config this row is.
		out = append(out, ConfounderStepLabelAmbiguous)
	}
	if ev.Workers.Truncated {
		out = append(out, ConfounderHistoryTruncated)
	}
	// Without the per-channel child table there is no way to say which DAG
	// edge lost the rows, so a collapse cannot be attributed inside the
	// worker. The table is deployed nowhere as of 2026-08-25 (F71, I-132), so
	// this is the ordinary case rather than the exception.
	if ev.Extent != nil && !ev.Extent.ChannelDetails {
		out = append(out, ConfounderCrossStepJoinUnavailable)
	}
	return out
}

func (d VolumeCollapse) basis(r *WorkerRow, in, got int64, cc *ConfigConfounders) string {
	step := r.StepId
	if step == "" {
		step = "(unlabelled)"
	}
	s := fmt.Sprintf("worker row %d of session %s, step %s, shard %d: output_records_count %d "+
		"against input_records_count %d on the same row, %.1f%% of what it read, against a floor of "+
		"%.0f%%. The comparison is within one row and needs no history",
		r.Key, r.SessionId, step, r.ShardId, got, in, 100*float64(got)/float64(in), 100*d.MinRatio)
	switch {
	case cc == nil:
		s += ". No configuration was consulted, so on_error: drop, max_input_count, sampling_max_count " +
			"and device_writer_type are unruled-out and unreported"
	default:
		s += ". " + cc.Note
		if cc.Read && len(cc.Bearing(VolumeCollapseConfounders)) == 0 {
			s += "; none of on_error: drop, max_input_count, sampling_max_count or device_writer_type " +
				"appears in it"
		}
	}
	return s
}

// StepRegression is section 12.2 row 3: a step that failed having previously
// succeeded on the same source.
//
// The source is (client, process_name, main_object_type, cpipes_step_id) and
// not the input file key, which for a periodic feed differs every run. That is
// the whole reason this detector needs the run header at all: three of the four
// keys are on the worker row (F98) and main_object_type is not.
type StepRegression struct {
	// MinPriorRuns is how many prior runs of the step the baseline must hold
	// before a failure is called a regression. One prior success is not a
	// history, and a step that has run twice has no normal to depart from.
	// Unsourced.
	MinPriorRuns int64
	Generation   int
}

// DefaultStepRegression is the starting point, not a recommendation.
func DefaultStepRegression() StepRegression {
	return StepRegression{MinPriorRuns: 3, Generation: 1}
}

// Ref is the detector_ref written on every anomaly this detector emits.
func (d StepRegression) Ref() string { return fmt.Sprintf("step_regression@%d", d.Generation) }

// Detect judges the failed workers in ev against ev.Prior. It returns nothing
// when no baseline was gathered, which is not an error: row 2 needs none and a
// caller may legitimately want only that one.
//
// The two windows must not overlap, and that is enforced rather than
// documented: a baseline computed over a window containing the run being
// judged counts this run's own sibling shards as history, and a partial shard
// failure would then be its own precedent. Gather closes the baseline window
// at the detection window's start, so a caller using it cannot fail this by
// accident; a caller composing its own windows can.
func (d StepRegression) Detect(ev *Evidence) ([]Anomaly, error) {
	if ev == nil || ev.Workers == nil || ev.Prior == nil {
		return nil, nil
	}
	current, prior := ev.Workers, ev.Prior
	if prior.Window.until().After(current.Window.Since) {
		return nil, fmt.Errorf(
			"the baseline window ends at %s, after the detection window opens at %s: a baseline that "+
				"contains the run being judged makes this run its own precedent",
			prior.Window.until().Format(time.RFC3339), current.Window.Since.Format(time.RFC3339))
	}

	index := map[string]*StepBaseline{}
	for i := range prior.Baselines {
		b := &prior.Baselines[i]
		index[baselineKey(b.Client, b.ProcessName, b.MainObjectType, b.StepId)] = b
	}

	var out []Anomaly
	for i := range current.Rows {
		r := &current.Rows[i]
		if r.Status != StatusFailed {
			continue
		}
		// A row whose header is gone has no main_object_type and therefore no
		// source identity, so it cannot be matched to a baseline at all. It is
		// skipped rather than matched loosely — bucketing it under an empty
		// object type is the same invention StepBaselines refuses to make.
		if !r.HasHeader {
			continue
		}
		b := index[baselineKey(r.Client, r.ProcessName, r.MainObjectType, r.StepId)]
		if b == nil {
			continue
		}
		// A step that has only ever failed is a broken configuration, not a
		// regression, and the two want different remedies.
		if !b.EverSucceeded() || b.Runs < d.MinPriorRuns {
			continue
		}

		// The confounders are the baseline's own, which the extraction already
		// computed from the record. Row 3 needs nothing from the configuration:
		// section 12.3 names its qualifier as "which of cpipes_step_id's
		// ambiguities applies", and that is a column of the worker row. This is
		// where the two detectors differ, and it is the answer to I-168 —
		// the gap is a property of row 2 rather than of derivable rows.
		conf := append([]string(nil), b.Confounders...)

		out = append(out, Anomaly{
			AnomalyId:     fmt.Sprintf("%s/%d", d.Ref(), r.Key),
			DetectedAt:    time.Now().UTC(),
			SessionId:     r.SessionId,
			SubjectType:   SubjectWorker,
			SubjectRef:    strconv.FormatInt(r.Key, 10),
			SignalType:    SignalStepRegression,
			ObservedValue: StatusFailed,
			// No expected range and no deviation magnitude, and their absence
			// is the finding rather than an omission: what this compares is a
			// status against a history of statuses, so there is no number to
			// be a minimum of and no distance to measure. I-126 made the three
			// properties optional on the reasoning that within-run predicates
			// have no range; this is a windowed comparison that has none
			// either, and row 2 above is a within-run predicate that has one.
			ExpectedBasis: d.basis(r, b, prior),
			Confounders:   conf,
			DetectorRef:   d.Ref(),
		})
	}
	return out, nil
}

func (d StepRegression) basis(r *WorkerRow, b *StepBaseline, prior *BaselineSet) string {
	s := fmt.Sprintf("worker row %d of session %s failed; %s. The baseline window ends at %s, before "+
		"this run started, so it holds no part of the run being judged",
		r.Key, r.SessionId, prior.Describe(b), prior.Window.until().Format(time.RFC3339))
	if b.LastSuccess != nil {
		s += fmt.Sprintf("; the step last completed at %s", b.LastSuccess.Format(time.RFC3339))
	}
	if b.Failed > 0 {
		s += fmt.Sprintf("; it had already failed %d times in that window, so this is a step that fails "+
			"sometimes rather than one that had never failed", b.Failed)
	}
	return s
}

func baselineKey(client, process, objectType, stepId string) string {
	return client + "\x00" + process + "\x00" + objectType + "\x00" + stepId
}

const insertAnomalyIfNewSQL = insertAnomalySQL + ` ON CONFLICT (anomaly_id) DO NOTHING`

// InsertAnomalyIfNew is InsertAnomaly for a detector that runs on a schedule.
// The anomaly_id both detectors compose is deterministic — the detector, its
// generation and the worker row — so a second run over an overlapping window
// re-derives the same identifier rather than a second row, and the primary key
// is what makes the run idempotent. It reports whether the row was new, so a
// scheduled run can say how much of what it found it had already said.
func InsertAnomalyIfNew(ctx context.Context, db Exec, a *Anomaly) (bool, error) {
	if err := a.Validate(); err != nil {
		return false, fmt.Errorf("invalid anomaly %q: %w", a.AnomalyId, err)
	}
	detectedAt := a.DetectedAt
	if detectedAt.IsZero() {
		detectedAt = time.Now().UTC()
	}
	conf := a.Confounders
	if conf == nil {
		conf = []string{}
	}
	tag, err := db.Exec(ctx, insertAnomalyIfNewSQL,
		a.AnomalyId, detectedAt, a.SessionId,
		a.SubjectType, a.SubjectRef, a.SignalType,
		a.ObservedValue, a.ExpectedMin, a.ExpectedMax,
		a.ExpectedBasis, a.DeviationMagnitude, conf,
		a.DetectorRef)
	if err != nil {
		return false, fmt.Errorf("while inserting anomaly %q: %w", a.AnomalyId, err)
	}
	return tag.RowsAffected() == 1, nil
}
