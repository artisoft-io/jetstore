package observe

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"time"
)

// The two detectors, section 12.2 rows 2 and 3. Which two is a measurement
// rather than a preference: row 1's numerator sums to zero across four
// production environments and 22.6 billion input records, and row 4's
// predicate fires four times in thirty days across all four, so a family
// cannot be proven on it. Rows 5 and 10 read
// jetsapi.pipeline_execution_channel_details, which is deployed nowhere.
//
// Two rather than one because one detector proves the path and two prove it is
// a family. The two chosen differ on the axis that matters for the decided
// detector shape: row 2 is a predicate over one row's own columns and row 3 is a
// comparison against a windowed aggregate, so between them they exercise both
// halves of "SQL plus a Go emitter".
//
// # Neither threshold varies by domain class, which is the load-bearing property
//
// Section 12.4's decision is reversible on one condition: a threshold that
// depends on what an Anomaly is *about* rather than on which column moved. Both
// detectors here take scalars that are the same for every client, process and
// object type — a ratio, a minimum input, a minimum history, a drop — so the
// condition does not fire and the SQL shape stands. The volume detector's
// effective threshold does differ by step, but it is read from that step's own
// history in the execution record, not declared per domain class: the step's
// past is the only thing it knows about what the step does. A reader checking
// that claim should check it against the fields of the two structs below,
// which is the whole of what either takes.
//
// # The thresholds are unsourced and are flagged rather than defended
//
// Section 12.5 measured the population, the retention regimes and the signal
// availability. It did not measure the distribution of output/input ratios,
// because no query asked for one, so nothing in this repository says what a
// normal ratio is for a JetStore step. The defaults below are starting points
// with a stated bias — expensive to miss, cheap to over-report — and the first
// deployment to run this is what replaces them.
//
// It did, for one of them. At the first deployment to run the volume detector,
// the single floor of 0.5 flagged every step whose job is to reduce or to join,
// and none of those anomalies was a loss. What that showed is that one number
// for every step is the wrong shape, not that 0.5 is the wrong number, so the
// volume detector now judges each step against its own history and keeps the
// floor for steps that have none. The new numbers, ten runs of history, three
// runs before a history counts and a drop to half the step's median, are as
// unsourced as the floor was: nothing yet measures how much a step's ratio
// varies from run to run.

// Evidence is one read of the execution record, and what the read could not
// see. Both detectors take one rather than a list of arguments, and that is
// the shape that was left open: the extraction's four exported functions
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
	// Volume is the history VolumeCollapse judges each step against, set by
	// VolumeCollapse.ReadHistory. Nil when it was not read, and the volume
	// detector then applies its floor to every worker and says so.
	Volume *VolumeHistory
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
// completed and emitted materially fewer rows than it usually does for what it
// read.
//
// It fires only on a worker that reached 'completed'. A worker that failed has
// already reported itself through its status, and an anomaly beside that
// failure is the second copy of a signal rather than a new one; a worker still
// in progress has NULL counts and is row 4's observation, not this one.
//
// # Each step is judged against its own history
//
// A worker's output/input ratio is compared with the median ratio of the same
// (client, process, step) over its last HistoryRuns runs that started before
// this one, and the detector fires when the ratio falls below (1 - MaxDrop) of
// that median. Only where the step has fewer than MinPriorRuns such runs is the
// absolute floor, MinRatio, applied instead, and the anomaly's basis says which
// of the two comparisons was made.
//
// The floor alone judged every step by one number, and a ratio says nothing
// until it is known what the step does: a step that reduces claims to the few
// that qualify, or joins a small input against a large reference set it reads
// in the same worker, writes a small fraction of what it read on every run. At
// the first deployment to run this detector, the floor flagged every such step
// in its window and none of them was a loss. A step's own history knows what the
// step does, so the ratio that is normal for it is the one it is judged by.
//
// A prior run enters the history only if every worker of the step completed
// with its counts and the run read at least MinInput rows in all, so a partly
// failed run or a trivially small one does not move the median. The history is
// read from the worker rows, because those carry the counts; a run whose session
// has been deleted keeps its header in jetsapi.pipeline_execution_history but not
// its worker rows, so its ratio is gone. Where the floor is applied because the
// history is short and the history view shows archived runs of the process,
// the anomaly says so and carries history_truncated: the step may be new, or its
// history may have been purged, and the two are told apart rather than guessed.
type VolumeCollapse struct {
	// MinRatio is the absolute floor: with too little history, the detector
	// fires when output_records_count / input_records_count is below it.
	// Unsourced; see the file comment.
	MinRatio float64
	// MinInput ignores workers that read fewer rows than this, and leaves out
	// of the history runs of the step that read fewer in all. A ratio over a
	// handful of rows is arithmetic rather than evidence — one row of two is a
	// 50% collapse — and the corpus has steps that legitimately read a few
	// configuration rows. Unsourced.
	MinInput int64
	// HistoryRuns is how many of the step's most recent prior runs the
	// baseline is the median of. Unsourced.
	HistoryRuns int
	// MinPriorRuns is how many prior runs the step must have before its own
	// history is the basis; with fewer, the floor is applied. Unsourced.
	MinPriorRuns int
	// MaxDrop is how far below its own median a step's ratio may fall before
	// the detector fires: 0.5 fires below half the median. Unsourced.
	MaxDrop float64
	// Generation is the trailing number of the detector_ref. Bumping it makes
	// the emitter re-emit over rows it has already judged, which is what a
	// changed threshold needs and what a deterministic anomaly_id would
	// otherwise prevent. Generation 2 is the first to judge a step against its
	// own history, so a run of it re-judges every row generation 1 judged
	// against the floor alone.
	Generation int
}

// DefaultVolumeCollapse is the starting point, not a recommendation.
func DefaultVolumeCollapse() VolumeCollapse {
	return VolumeCollapse{MinRatio: 0.5, MinInput: 1000,
		HistoryRuns: 10, MinPriorRuns: 3, MaxDrop: 0.5, Generation: 2}
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

// VolumeRun is one run of one step, as its history: the run's key, when its
// first worker of the step started, and the step's input and output summed over
// its workers.
type VolumeRun struct {
	RunKey int64
	Start  time.Time
	Input  int64
	Output int64
}

// Ratio is the run's output/input ratio for the step.
func (r VolumeRun) Ratio() float64 { return float64(r.Output) / float64(r.Input) }

// ArchivedRuns is what jetsapi.pipeline_execution_history says of a process's
// runs whose sessions were deleted: how many, and the latest of them.
type ArchivedRuns struct {
	Count  int64
	Latest time.Time
}

// VolumeHistory is what VolumeCollapse.ReadHistory read: the runs of each step
// judged, and the archived runs of each process judged.
type VolumeHistory struct {
	// Runs is keyed by (client, process, step) and sorted by Start. It holds,
	// per step, the HistoryRuns most recent runs that started before the
	// earliest worker judged, and every run from then on, which is enough for
	// any worker judged to find its own HistoryRuns predecessors.
	Runs map[string][]VolumeRun
	// HistoryView is false where jetsapi.pipeline_execution_history does not
	// exist, so the archived runs could not be counted.
	HistoryView bool
	// Archived is keyed by (client, process).
	Archived map[string]ArchivedRuns
}

// One row per (client, process, step, run) that qualifies as history: every
// worker of the step in the run completed with its counts, and the run read at
// least $5 rows. The runs that started before $4, the earliest worker judged,
// are ranked newest first and only the first $6 kept; the runs from $4 on are all
// kept, since each worker is judged against the runs that preceded it, some of
// which may be among the rows judged too.
const volumeHistorySQL = `WITH runs AS (
  SELECT d.client, d.process_name, coalesce(d.cpipes_step_id, '') AS step_id,
         d.pipeline_execution_status_key AS run_key, min(d.start_time) AS run_start,
         sum(d.input_records_count)::bigint AS input, sum(d.output_records_count)::bigint AS output
    FROM jetsapi.pipeline_execution_details d
   WHERE d.start_time < $1
     AND d.process_name = ANY($2)
     AND d.client = ANY($3)
   GROUP BY 1, 2, 3, 4
  HAVING bool_and(d.status = 'completed'
                  AND d.input_records_count IS NOT NULL
                  AND d.output_records_count IS NOT NULL)
     AND sum(d.input_records_count) >= $5
), ranked AS (
  SELECT r.*, row_number() OVER (
           PARTITION BY r.client, r.process_name, r.step_id, r.run_start < $4
           ORDER BY r.run_start DESC, r.run_key DESC) AS rn
    FROM runs r
)
SELECT client, process_name, step_id, run_key, run_start, input, output
  FROM ranked
 WHERE run_start >= $4 OR rn <= $6
 ORDER BY client, process_name, step_id, run_start, run_key`

const archivedRunsSQL = `SELECT client, process_name, count(*), max(start_time)
  FROM jetsapi.pipeline_execution_history
 WHERE archived_at IS NOT NULL
   AND start_time < $1
   AND process_name = ANY($2)
   AND client = ANY($3)
 GROUP BY 1, 2`

// ReadHistory reads the history the worker rows in ev are judged against and
// sets it on ev. Without it, Detect applies the floor to every worker and says
// that no history was read.
//
// Only the processes of the completed workers in ev are read, and the read is
// bounded by runs rather than by time: a monthly step's ten runs span most of a
// year and a daily step's ten span ten days, and a step is judged against what
// it usually does rather than against what it did in a window chosen for
// another detector.
func (d VolumeCollapse) ReadHistory(ctx context.Context, db DB, ev *Evidence) error {
	if ev == nil || ev.Workers == nil {
		return nil
	}
	h := &VolumeHistory{Runs: map[string][]VolumeRun{}, Archived: map[string]ArchivedRuns{}}
	ev.Volume = h
	// Only the clients and processes of the workers Detect will judge, and only
	// the runs before the earliest of them are bounded by HistoryRuns. The
	// earliest worker rather than the window's start, because a window may have
	// none: a classifier judging one session reads all of it.
	var processes, clients []string
	var earliest time.Time
	for i := range ev.Workers.Rows {
		r := &ev.Workers.Rows[i]
		if r.Status != StatusCompleted {
			continue
		}
		if !slices.Contains(processes, r.ProcessName) {
			processes = append(processes, r.ProcessName)
		}
		if !slices.Contains(clients, r.Client) {
			clients = append(clients, r.Client)
		}
		if earliest.IsZero() || r.StartTime.Before(earliest) {
			earliest = r.StartTime
		}
	}
	if len(processes) == 0 {
		return nil
	}
	w := ev.Workers.Window
	rows, err := db.Query(ctx, volumeHistorySQL,
		w.until(), processes, clients, earliest, d.MinInput, d.HistoryRuns)
	if err != nil {
		return fmt.Errorf("while reading the volume history of each step: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var client, process, step string
		var run VolumeRun
		if err := rows.Scan(&client, &process, &step, &run.RunKey, &run.Start,
			&run.Input, &run.Output); err != nil {
			return fmt.Errorf("while scanning the volume history: %w", err)
		}
		k := volumeKey(client, process, step)
		h.Runs[k] = append(h.Runs[k], run)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("while reading the volume history: %w", err)
	}

	if err := db.QueryRow(ctx,
		`SELECT to_regclass('jetsapi.pipeline_execution_history') IS NOT NULL`).Scan(&h.HistoryView); err != nil {
		return fmt.Errorf("while checking for jetsapi.pipeline_execution_history: %w", err)
	}
	if !h.HistoryView {
		return nil
	}
	arows, err := db.Query(ctx, archivedRunsSQL, w.until(), processes, clients)
	if err != nil {
		return fmt.Errorf("while counting archived runs: %w", err)
	}
	defer arows.Close()
	for arows.Next() {
		var client, process string
		var a ArchivedRuns
		if err := arows.Scan(&client, &process, &a.Count, &a.Latest); err != nil {
			return fmt.Errorf("while scanning archived runs: %w", err)
		}
		h.Archived[client+"\x00"+process] = a
	}
	return arows.Err()
}

func volumeKey(client, process, step string) string {
	return client + "\x00" + process + "\x00" + step
}

// volumeBaseline is the history one worker is judged against.
type volumeBaseline struct {
	prior  []VolumeRun // oldest first, at most HistoryRuns
	median float64
}

// baselineFor returns the step's runs that precede the worker's run, at most
// HistoryRuns of them, and their median ratio. The worker's own run is never
// its own history: it is excluded by key, and a run is prior only if it
// started before the worker's run did.
func (d VolumeCollapse) baselineFor(r *WorkerRow, h *VolumeHistory) volumeBaseline {
	runs := h.Runs[volumeKey(r.Client, r.ProcessName, r.StepId)]
	start := r.StartTime
	for _, run := range runs {
		if run.RunKey == r.ExecutionStatusKey {
			start = run.Start
			break
		}
	}
	var prior []VolumeRun
	for _, run := range runs {
		if run.RunKey != r.ExecutionStatusKey && run.Start.Before(start) {
			prior = append(prior, run)
		}
	}
	if d.HistoryRuns > 0 && len(prior) > d.HistoryRuns {
		prior = prior[len(prior)-d.HistoryRuns:]
	}
	b := volumeBaseline{prior: prior}
	if len(prior) > 0 {
		ratios := make([]float64, len(prior))
		for i := range prior {
			ratios[i] = prior[i].Ratio()
		}
		sort.Float64s(ratios)
		n := len(ratios)
		if n%2 == 1 {
			b.median = ratios[n/2]
		} else {
			b.median = (ratios[n/2-1] + ratios[n/2]) / 2
		}
	}
	return b
}

// Detect judges every worker row in ev and returns the anomalies, in the
// record's own order. A session with no ConfigConfounders entry produces an
// anomaly whose basis says no configuration was consulted — the one thing
// ruled out is saying nothing, because an empty anomaly_confounders is a
// claim rather than an omission.
//
// Each worker is judged against its step's history when ev carries one with at
// least MinPriorRuns prior runs of the step, and against the floor otherwise;
// with no history read at all (ReadHistory was not called), every worker is
// judged against the floor and every basis says so.
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
		// The six counts are NULL for exactly as long as the worker is
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

		var b volumeBaseline
		if ev.Volume != nil {
			b = d.baselineFor(r, ev.Volume)
		}
		fromHistory := len(b.prior) >= d.MinPriorRuns && len(b.prior) > 0
		threshold := d.MinRatio
		if fromHistory {
			threshold = b.median * (1 - d.MaxDrop)
		}
		if ratio >= threshold {
			continue
		}

		cc := ev.Configs[r.SessionId]
		conf := d.recordConfounders(r, ev)
		var archived ArchivedRuns
		if ev.Volume != nil {
			archived = ev.Volume.Archived[r.Client+"\x00"+r.ProcessName]
		}
		if !fromHistory && archived.Count > 0 && !slices.Contains(conf, ConfounderHistoryTruncated) {
			conf = append(conf, ConfounderHistoryTruncated)
		}
		conf = append(conf, cc.Bearing(VolumeCollapseConfounders)...)
		// Against the floor, the deviation is the shortfall from reading
		// everything back; against a history, it is the shortfall from what
		// the step usually writes for what it reads.
		shortfall := 1 - ratio
		if fromHistory {
			shortfall = 1 - ratio/b.median
		}
		expectedMin := strconv.FormatInt(int64(threshold*float64(in)), 10)

		out = append(out, Anomaly{
			AnomalyId:          fmt.Sprintf("%s/%d", d.Ref(), r.Key),
			DetectedAt:         time.Now().UTC(),
			SessionId:          r.SessionId,
			SubjectType:        SubjectWorker,
			SubjectRef:         strconv.FormatInt(r.Key, 10),
			SignalType:         SignalVolume,
			ObservedValue:      strconv.FormatInt(got, 10),
			ExpectedMin:        &expectedMin,
			ExpectedBasis:      d.basis(r, in, got, b, fromHistory, ev.Volume, archived, cc),
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
		// Ten reducing steps in the corpus carry an empty label, so the
		// anomaly cannot say which step of the config this row is.
		out = append(out, ConfounderStepLabelAmbiguous)
	}
	if ev.Workers.Truncated {
		out = append(out, ConfounderHistoryTruncated)
	}
	// Without the per-channel child table there is no way to say which DAG
	// edge lost the rows, so a collapse cannot be attributed inside the
	// worker. The table is deployed nowhere as of 2026-08-25, so
	// this is the ordinary case rather than the exception.
	if ev.Extent != nil && !ev.Extent.ChannelDetails {
		out = append(out, ConfounderCrossStepJoinUnavailable)
	}
	return out
}

// The basis opens by naming which comparison was made, "against the step's own
// history" or "against the absolute floor", so a reader of the incident list can
// tell the two apart without reading to the end.
func (d VolumeCollapse) basis(r *WorkerRow, in, got int64, b volumeBaseline, fromHistory bool,
	h *VolumeHistory, archived ArchivedRuns, cc *ConfigConfounders) string {
	step := r.StepId
	if step == "" {
		step = "(unlabelled)"
	}
	ratio := float64(got) / float64(in)
	var s string
	if fromHistory {
		first, last := b.prior[0], b.prior[len(b.prior)-1]
		s = fmt.Sprintf("against the step's own history: worker row %d of session %s, step %s, shard %d "+
			"wrote output_records_count %d against input_records_count %d, a ratio of %.4g. The median "+
			"ratio of client %s, process %s, step %s over its %d prior runs, from %s to %s, is %.4g, and the "+
			"detector fires below %.0f%% of it, here %.4g; this run is at %.1f%% of its median. The absolute "+
			"floor of %.0f%% was not applied, because the step has a history",
			r.Key, r.SessionId, step, r.ShardId, got, in, ratio,
			r.Client, r.ProcessName, step, len(b.prior),
			first.Start.Format(time.RFC3339), last.Start.Format(time.RFC3339), b.median,
			100*(1-d.MaxDrop), b.median*(1-d.MaxDrop), 100*ratio/b.median, 100*d.MinRatio)
	} else {
		s = fmt.Sprintf("against the absolute floor: worker row %d of session %s, step %s, shard %d: "+
			"output_records_count %d against input_records_count %d on the same row, %.1f%% of what it "+
			"read, against a floor of %.0f%%",
			r.Key, r.SessionId, step, r.ShardId, got, in, 100*ratio, 100*d.MinRatio)
		switch {
		case h == nil:
			s += ". No history was read, so the step could not be judged against its own"
		default:
			s += fmt.Sprintf(". The step has %d prior runs with counts, fewer than the %d its own "+
				"history needs, so the floor was applied", len(b.prior), d.MinPriorRuns)
			switch {
			case !h.HistoryView:
				s += "; jetsapi.pipeline_execution_history does not exist here, so whether earlier " +
					"runs were deleted could not be read"
			case archived.Count > 0:
				s += fmt.Sprintf("; %d earlier runs of process %s are archived in "+
					"jetsapi.pipeline_execution_history, the latest at %s, and their worker counts were "+
					"deleted with their sessions, so the step may have a history that is gone",
					archived.Count, r.ProcessName, archived.Latest.Format(time.RFC3339))
			default:
				s += fmt.Sprintf("; no run of process %s is archived, so the step's history is short "+
					"rather than purged", r.ProcessName)
			}
		}
	}
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
// keys are on the worker row and main_object_type is not.
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
		// where the two detectors differ, and it is the answer to the silence rule —
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
			// be a minimum of and no distance to measure. The model made the three
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
