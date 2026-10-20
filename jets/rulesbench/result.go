package rulesbench

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// ResultSchema names the shape of a run's JSON result. It changes when a field
// changes meaning.
const ResultSchema = "rules_bench/1"

// RunResult is one run: one engine, one repetition, every session. Every time
// is in nanoseconds of wall time, read on the monotonic clock.
type RunResult struct {
	Schema     string `json:"schema"`
	Bench      string `json:"bench"`
	SpecPath   string `json:"spec_path"`
	Engine     string `json:"engine"`
	LegacyMode string `json:"legacy_mode"`
	// Repetition counts from 1 within its kind: warmup runs and measured runs
	// are numbered apart.
	Repetition int  `json:"repetition"`
	Warmup     bool `json:"warmup"`
	// Order is this run's position among all the runs of the process, from 1,
	// which says how the engines were interleaved.
	Order           int `json:"order"`
	PoolSize        int `json:"pool_size"`
	MaxVertexVisits int `json:"max_vertex_visits"`
	// MaxLooping is the override in force, 0 when every rule set's $max_looping applies.
	MaxLooping int    `json:"max_looping"`
	Status     string `json:"status"` // ok, or error with Reason
	Reason     string `json:"reason,omitempty"`

	StartedAt time.Time `json:"started_at"`
	// WallNs runs from handing the first session to a worker to the last
	// session's end.
	WallNs int64 `json:"wall_ns"`

	Load     LoadResult      `json:"load"`
	Totals   Totals          `json:"totals"`
	Memory   MemoryResult    `json:"memory"`
	Sessions []SessionResult `json:"sessions"`
	Env      *Environment    `json:"env"`
}

// LoadResult is the engine's construction, done once per engine per process:
// every run of that engine in the process repeats the same numbers.
type LoadResult struct {
	// EngineNs is NewJetRuleEngine, which loads the process's rule sets.
	EngineNs int64 `json:"engine_ns"`
	// ConfigNs is the rest of the pipeline's construction: JetResources, the
	// source period and rule_config into the meta graph, and max_vertex_visits.
	ConfigNs int64 `json:"config_ns"`
	// MaxVertexVisitsSeen is true when an rdf session of the engine reads
	// (jets:iState jets:max_vertex_visits n) as the int n, checked once after the
	// load on a session that is then released. A load where it is false fails.
	MaxVertexVisitsSeen bool `json:"max_vertex_visits_seen"`
	// MaxVertexVisitsFrom says where the cap came from: flag, spec or
	// rule_config. RuleConfigMaxVertexVisitsReplaced is rule_config's value when
	// the flag or the spec replaced it.
	MaxVertexVisitsFrom               string `json:"max_vertex_visits_from"`
	RuleConfigMaxVertexVisitsReplaced int    `json:"rule_config_max_vertex_visits_replaced,omitempty"`
	// EnginesInProcess are the engines built in the process, in run order. A
	// process with both holds both engines' memory, so MaxRSSKB is per process.
	EnginesInProcess []string `json:"engines_in_process"`
}

// Totals sums a run's sessions.
type Totals struct {
	Sessions  int `json:"sessions"`
	InputRows int `json:"input_rows"`
	Loops     int `json:"loops"`
	// Errors counts sessions that ended with an error, a maximum loop count
	// reached, or an exception.
	Errors int `json:"errors"`
	// MaxLoopReached and VertexCapReached count the sessions where a cap bound.
	MaxLoopReached   int `json:"max_loop_reached"`
	VertexCapReached int `json:"vertex_cap_reached"`
	Extracted        int `json:"extracted"`
	// SessionsPerSec and RowsPerSec are throughput over WallNs.
	SessionsPerSec float64 `json:"sessions_per_sec"`
	RowsPerSec     float64 `json:"rows_per_sec"`
	// The phases summed over sessions; with a pool they add up to more than WallNs.
	RdfSessionNs  int64 `json:"rdf_session_ns"`
	InputAssertNs int64 `json:"input_assert_ns"`
	ReteSessionNs int64 `json:"rete_session_ns"`
	ExecuteNs     int64 `json:"execute_ns"`
	ControlNs     int64 `json:"control_ns"`
	ExtractionNs  int64 `json:"extraction_ns"`
	ReleaseNs     int64 `json:"release_ns"`
	SessionNs     int64 `json:"session_ns"`
}

// MemoryResult is the process's memory over a run.
type MemoryResult struct {
	// Go is the Go runtime's statistics, the difference over the run. On the
	// C++ engine it counts the adaptor and the bridge only: the engine's own heap
	// is not the Go heap.
	Go MemDelta `json:"go"`
	// PeakGoroutines is the highest runtime.NumGoroutine sampled during the run,
	// every SampleMs; 0 when sampling is off. GoroutinesBefore and After are read
	// at the run's start and end.
	PeakGoroutines   int `json:"peak_goroutines"`
	SampleMs         int `json:"sample_ms"`
	GoroutinesBefore int `json:"goroutines_before"`
	GoroutinesAfter  int `json:"goroutines_after"`
	// MaxRSSKB is getrusage's ru_maxrss after the run: the process's peak since
	// it started, in KiB, which covers both engines' heaps. RSSKB is the resident
	// set at the run's end, from /proc/self/statm.
	MaxRSSKB int64 `json:"max_rss_kb"`
	RSSKB    int64 `json:"rss_kb"`
}

// MemDelta is the difference of runtime.MemStats fields over an interval.
type MemDelta struct {
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	Mallocs         uint64 `json:"mallocs"`
	Frees           uint64 `json:"frees"`
	NumGC           uint32 `json:"num_gc"`
	PauseTotalNs    uint64 `json:"pause_total_ns"`
	// HeapInuseBytes is the value at the end, not a difference.
	HeapInuseBytes uint64 `json:"heap_inuse_bytes"`
}

// SessionResult is one input session of a run.
type SessionResult struct {
	Name string `json:"name"`
	// Worker is the pool worker that ran it, from 0.
	Worker    int `json:"worker"`
	InputRows int `json:"input_rows"`
	// StartNs is when the session started, from the run's start.
	StartNs int64 `json:"start_ns"`
	// The phases: NewRdfSession; the input assertion; RunRuleSets as a whole
	// (which includes the input assertion and the rule sets below, and leaves out
	// DumpNs and CountersNs); extraction, by an extractor built once per worker;
	// Release; and the session from before NewRdfSession to after Release, less
	// DumpNs and CountersNs.
	RdfSessionNs  int64 `json:"rdf_session_ns"`
	InputAssertNs int64 `json:"input_assert_ns"`
	RuleSetsNs    int64 `json:"rule_sets_ns"`
	ExtractionNs  int64 `json:"extraction_ns"`
	ReleaseNs     int64 `json:"release_ns"`
	TotalNs       int64 `json:"total_ns"`
	// DumpNs is the time spent in Options.AfterRuleSet, which is the graph dump
	// when there is one; it is in no other phase. A run with a dump is not a
	// timing run.
	DumpNs int64 `json:"dump_ns,omitempty"`
	// CountersNs is the time the bench spent in its own timing callbacks, mostly
	// reading the Go engine's counters after each rule set. It is in no other
	// phase: RuleSetsNs and TotalNs leave it out.
	CountersNs int64 `json:"counters_ns"`

	Loops          int      `json:"loops"`
	Completed      bool     `json:"completed"`
	MaxLoopReached bool     `json:"max_loop_reached"`
	ExecuteErr     string   `json:"execute_err,omitempty"`
	Exceptions     []string `json:"exceptions,omitempty"`
	Err            string   `json:"err,omitempty"`

	RuleSets  []RuleSetResult `json:"rule_sets"`
	Extracted map[string]int  `json:"extracted,omitempty"`
	// Graph is the session's graph sizes after its last rule set, where the
	// engine counts them (Go). TriplesTotal is a count by iteration over every
	// triple the session reads, meta graph included, made after extraction and
	// timed in no phase; it is set with Options.CountTriples, on both engines.
	Graph        *compute_pipes.GraphSizes `json:"graph,omitempty"`
	TriplesTotal *int                      `json:"triples_total,omitempty"`
	// GoroutinesBefore and After are runtime.NumGoroutine around the session;
	// with a pool of 1 a difference is goroutines the session left behind.
	GoroutinesBefore int `json:"goroutines_before"`
	GoroutinesAfter  int `json:"goroutines_after"`
	// Memory is the Go runtime's statistics over the session, with a pool of 1
	// only: with more workers the process's statistics mix their sessions.
	Memory *MemDelta `json:"memory,omitempty"`
}

// RuleSetResult is one rule set of a session.
type RuleSetResult struct {
	RuleSet string `json:"rule_set"`
	// The phases of compute_pipes.RuleSetTiming.
	ReteSessionNs   int64 `json:"rete_session_ns"`
	PrepareNs       int64 `json:"prepare_ns"`
	ExecuteNs       int64 `json:"execute_ns"` // ExecuteRules, summed over the loops
	ExecuteMaxNs    int64 `json:"execute_max_ns"`
	ControlNs       int64 `json:"control_ns"`
	ExceptionScanNs int64 `json:"exception_scan_ns"`
	TotalNs         int64 `json:"total_ns"`
	// LoopNs is each loop's ExecuteRules, loop 0 first, with Options.PerLoop.
	LoopNs []int64 `json:"loop_ns,omitempty"`
	// LoopsRun is the number of ExecuteRules calls; LastLoop is the last loop
	// value, as compute_pipes.RuleSetOutcome.Loops has it.
	LoopsRun       int    `json:"loops_run"`
	LastLoop       int    `json:"last_loop"`
	MaxLooping     int    `json:"max_looping"`
	Completed      bool   `json:"completed"`
	MaxLoopReached bool   `json:"max_loop_reached"`
	ExecuteErr     string `json:"execute_err,omitempty"`
	// VertexCapReached is true when ExecuteRules stopped on max_vertex_visits.
	VertexCapReached bool `json:"vertex_cap_reached"`
	// Graph and Rete are the engine's own counters at the rule set's end, where
	// it keeps them (Go): see compute_pipes.GraphSizer and ReteCounterReader.
	Graph *compute_pipes.GraphSizes   `json:"graph,omitempty"`
	Rete  *compute_pipes.ReteCounters `json:"rete,omitempty"`
}
