package rulesbench

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
)

// Options says what to run and how. The zero value of an override keeps the spec's.
type Options struct {
	Spec     *Spec
	SpecPath string
	// Factories are the engines; a nil one cannot be run.
	Factories harness.Factories

	// Engines, Repetitions, Warmup, PoolSize, MaxVertexVisits and MaxLooping
	// override the spec when set: the ints when above 0, the pointers when not
	// nil, so that a warmup of 0 can override a spec's.
	Engines         []string
	Repetitions     int
	Warmup          *int
	PoolSize        int
	MaxVertexVisits int
	MaxLooping      *int

	// PerLoop records each loop's ExecuteRules time, not only their sum and maximum.
	PerLoop bool
	// CountTriples counts every triple a session reads, by iteration, after
	// extraction. It is timed in no phase, and costs a walk of the whole graph.
	CountTriples bool
	// SampleMs is the period of the goroutine sampler; 0 turns it off.
	SampleMs int

	// AfterRuleSet, when not nil, is called after each rule set of each session,
	// before its rete session is released. It is where the graph dump is wired
	// in (NewDumper). Its time is the session's DumpNs, and is in no other phase.
	AfterRuleSet func(p DumpPoint, rdfSession compute_pipes.JetRdfSession) error
	// OnExtractedRow, when not nil, receives every row a session extracts, on the
	// class's drain goroutine; p's Index and RuleSet are unset. RowWriter.Write is
	// one. It is called inside the session's ExtractionNs.
	OnExtractedRow func(p DumpPoint, className string, row []any)

	// BeforeRuns and AfterRuns, when not nil, bracket the measured and warmup
	// runs, after every engine is built: where the command starts and stops its
	// profiles, so that a profile holds the runs and not the load.
	BeforeRuns func()
	AfterRuns  func()

	// Env is recorded in every result; nil records it here.
	Env *Environment
	// OnResult receives each run's result as it ends.
	OnResult func(*RunResult)
}

// prepared is the input, read and cast once, before any engine is built: a
// pipeline casts its input when it reads it, so casting is not in any phase.
type prepared struct {
	name string
	rows []*compute_pipes.CastRow
}

// engineState is one engine, built once for the process.
type engineState struct {
	name       string
	engine     compute_pipes.JetRuleEngine
	spec       *compute_pipes.JetrulesSpec
	load       LoadResult
	legacyMode string
}

// Run runs the bench and returns every run's result, warmup runs included, in
// the order they ran. An error means nothing could be run: an invalid input, an
// engine that does not build. A session that fails is recorded in its run's
// result instead.
func Run(opts Options) ([]*RunResult, error) {
	spec := opts.Spec
	if spec == nil {
		return nil, errors.New("no bench spec")
	}
	engines := spec.EnginesOrDefault()
	if len(opts.Engines) > 0 {
		engines = opts.Engines
	}
	reps := firstPositive(opts.Repetitions, spec.Repetitions, 1)
	warmup := spec.Warmup
	if opts.Warmup != nil {
		warmup = *opts.Warmup
	}
	if warmup < 0 {
		return nil, errors.New("warmup cannot be negative")
	}
	pool := firstPositive(opts.PoolSize, spec.PoolSize, 1)
	cfg := spec.JetrulesConfig
	ruleConfig, fromRuleConfig, err := takeMaxVertexVisits(cfg.RuleConfig)
	if err != nil {
		return nil, err
	}
	cfg.RuleConfig = ruleConfig
	mvv, mvvFrom := fromRuleConfig, "rule_config"
	switch {
	case opts.MaxVertexVisits > 0:
		mvv, mvvFrom = opts.MaxVertexVisits, "flag"
	case spec.MaxVertexVisits > 0:
		mvv, mvvFrom = spec.MaxVertexVisits, "spec"
	}
	if mvv == 0 {
		return nil, errors.New("max_vertex_visits is required, in the spec, with -max_vertex_visits, or in rule_config: Go's default is 10,000 and C++ has none (rules_benchmark F9)")
	}
	maxLooping := spec.JetrulesConfig.MaxLooping
	if spec.MaxLooping != nil {
		maxLooping = *spec.MaxLooping
	}
	if opts.MaxLooping != nil {
		maxLooping = *opts.MaxLooping
	}
	for _, e := range engines {
		if factoryFor(opts.Factories, e) == nil {
			return nil, fmt.Errorf("the %s engine is not available", e)
		}
	}
	env := opts.Env
	if env == nil {
		env = RecordEnvironment()
	}

	// The input, read and cast before any engine exists.
	cfg.MaxLooping = maxLooping
	jspec, err := harness.JetrulesSpecOf(&cfg)
	if err != nil {
		return nil, err
	}
	sessions, err := spec.Sessions(filepath.Dir(opts.SpecPath))
	if err != nil {
		return nil, err
	}
	inputs := make([]prepared, len(sessions))
	for i, s := range sessions {
		inputs[i].name = s.Name
		for _, f := range s.Files {
			t, err := harness.ReadInputTable(f, f)
			if err != nil {
				return nil, fmt.Errorf("session %q: %v", s.Name, err)
			}
			for j, cells := range t.Rows {
				cr, err := compute_pipes.CastTextRow(jspec, t.Columns, cells)
				if err != nil {
					return nil, fmt.Errorf("session %q: %s line %d: %v", s.Name, f, t.Lines[j], err)
				}
				inputs[i].rows = append(inputs[i].rows, cr)
			}
		}
	}
	classes, err := extractClasses(spec.OutputClasses)
	if err != nil {
		return nil, err
	}

	// Every engine is built before the first run, as a pipeline builds its engine
	// before its first session.
	states := make([]*engineState, 0, len(engines))
	defer func() {
		for _, st := range states {
			st.engine.Release()
		}
	}()
	for _, e := range engines {
		st, err := loadEngine(e, factoryFor(opts.Factories, e), &cfg, mvv)
		if err != nil {
			return nil, err
		}
		st.load.EnginesInProcess = engines
		st.load.MaxVertexVisitsFrom = mvvFrom
		if fromRuleConfig > 0 && mvvFrom != "rule_config" {
			st.load.RuleConfigMaxVertexVisitsReplaced = fromRuleConfig
		}
		states = append(states, st)
	}

	if opts.BeforeRuns != nil {
		opts.BeforeRuns()
	}
	var results []*RunResult
	order := 0
	runAll := func(n int, isWarmup bool) {
		for rep := 1; rep <= n; rep++ {
			for _, st := range states {
				order++
				r := runOnce(&opts, st, inputs, classes, pool, isWarmup, rep, order)
				r.Bench, r.SpecPath, r.Env = spec.Name(), opts.SpecPath, env
				r.MaxVertexVisits, r.MaxLooping = mvv, maxLooping
				results = append(results, r)
				if opts.OnResult != nil {
					opts.OnResult(r)
				}
			}
		}
	}
	runAll(warmup, true)
	runAll(reps, false)
	if opts.AfterRuns != nil {
		opts.AfterRuns()
	}
	return results, nil
}

// takeMaxVertexVisits returns rule_config without its jets:max_vertex_visits,
// and the value it held, 0 when none: the bench sets the cap itself, once, so
// that two values never meet in the meta graph. The maps are copied, not
// changed.
func takeMaxVertexVisits(ruleConfig []map[string]any) ([]map[string]any, int, error) {
	const key = "jets:max_vertex_visits"
	out := make([]map[string]any, 0, len(ruleConfig))
	found := 0
	for _, rc := range ruleConfig {
		v, ok := rc[key]
		if !ok {
			out = append(out, rc)
			continue
		}
		value, rdfType, err := compute_pipes.ExtractRdfNodeInfoJson(v)
		if err != nil {
			return nil, 0, fmt.Errorf("rule_config %s: %v", key, err)
		}
		n, err := strconv.Atoi(value)
		if err != nil || rdfType != "int" || n <= 0 {
			return nil, 0, fmt.Errorf("rule_config %s must be a positive int, not %q of type %s", key, value, rdfType)
		}
		if found > 0 && n != found {
			return nil, 0, fmt.Errorf("rule_config sets %s twice, to %d and %d", key, found, n)
		}
		found = n
		cp := maps.Clone(rc)
		delete(cp, key)
		out = append(out, cp)
	}
	return out, found, nil
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}

func factoryFor(f harness.Factories, engine string) compute_pipes.JetRulesFactory {
	switch engine {
	case rulestest.EngineGo:
		return f.Go
	case rulestest.EngineNative:
		return f.Native
	}
	return nil
}

func extractClasses(ocs []OutputClass) ([]compute_pipes.ExtractClass, error) {
	out := make([]compute_pipes.ExtractClass, 0, len(ocs))
	for _, oc := range ocs {
		columns, err := compute_pipes.GetDomainProperties(oc.ClassName, oc.DirectPropertiesOnly)
		if err != nil {
			return nil, fmt.Errorf("output class %s: %v", oc.ClassName, err)
		}
		out = append(out, compute_pipes.ExtractClass{ClassName: oc.ClassName, Columns: append(columns, oc.Columns...)})
	}
	return out, nil
}

// loadEngine builds an engine in the order a pipeline does
// (NewJetrulesTransformationPipe, then executeRules' first call): the engine,
// its JetResources, the source period and rule_config into the meta graph, and
// then the vertex-visit cap. The engine is built in production's retraction
// mode: no option is passed. It then checks, on a throwaway rdf session, that
// a session reads the cap.
func loadEngine(name string, factory compute_pipes.JetRulesFactory, cfg *rulestest.JetrulesConfig, mvv int) (st *engineState, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("the %s engine panicked while loading: %v\n%s", name, p, debug.Stack())
		}
	}()
	st = &engineState{name: name}
	st.spec, err = harness.JetrulesSpecOf(cfg)
	if err != nil {
		return nil, err
	}
	t0 := time.Now()
	re, err := factory.NewJetRuleEngine(nil, cfg.ProcessName, cfg.IsDebug)
	if err != nil {
		return nil, fmt.Errorf("while building the %s engine for %s: %v", name, cfg.ProcessName, err)
	}
	st.load.EngineNs = time.Since(t0).Nanoseconds()
	st.engine = re
	t1 := time.Now()
	jr := re.JetResources()
	envMap := map[string]any{}
	if err := compute_pipes.AssertSourcePeriodInfo(re, st.spec, envMap); err != nil {
		re.Release()
		return nil, fmt.Errorf("%s engine: while asserting the source period: %v", name, err)
	}
	if err := compute_pipes.AssertRuleConfiguration(re, st.spec, envMap); err != nil {
		re.Release()
		return nil, fmt.Errorf("%s engine: while asserting rule_config: %v", name, err)
	}
	if err := re.Insert(jr.Jets__istate, jr.Jets__max_vertex_visits, re.GetMetaResourceManager().NewIntLiteral(mvv)); err != nil {
		re.Release()
		return nil, fmt.Errorf("%s engine: while setting max_vertex_visits: %v", name, err)
	}
	st.load.ConfigNs = time.Since(t1).Nanoseconds()
	st.legacyMode = rulestest.LegacyModeLegacy
	if m, ok := re.(interface {
		RetractionMode() compute_pipes.RetractionMode
	}); ok {
		st.legacyMode = m.RetractionMode().Label()
	}
	// The check, on a session no run uses. Contains rather than an iterator: the
	// native adaptor's iterators need a rete session to exist (rules_benchmark
	// I-13), and an early stop on a Go iterator leaves goroutines blocked (I-1).
	rdfSession, err := re.NewRdfSession()
	if err != nil {
		re.Release()
		return nil, fmt.Errorf("%s engine: while creating an rdf session: %v", name, err)
	}
	seen := rdfSession.Contains(jr.Jets__istate, jr.Jets__max_vertex_visits, rdfSession.GetResourceManager().NewIntLiteral(mvv))
	rdfSession.Release()
	if !seen {
		re.Release()
		return nil, fmt.Errorf("%s engine: an rdf session does not read (jets:iState jets:max_vertex_visits %d)", name, mvv)
	}
	st.load.MaxVertexVisitsSeen = true
	return st, nil
}

// task is one session handed to a worker, as WorkersTaskCh hands a bundle.
type task struct {
	index int
}

// runOnce is one run: every session on one engine. With a pool of n it starts
// n workers reading one task channel of capacity 1 and sharing the engine,
// which is a pipeline's arrangement (JrPoolManager, JrPoolWorker.DoWork): the
// pool here hands out sessions rather than input bundles, and collects results
// rather than writing output channels.
func runOnce(opts *Options, st *engineState, inputs []prepared, classes []compute_pipes.ExtractClass,
	pool int, isWarmup bool, rep, order int) *RunResult {
	r := &RunResult{
		Schema:     ResultSchema,
		Engine:     st.name,
		LegacyMode: st.legacyMode,
		Repetition: rep,
		Warmup:     isWarmup,
		Order:      order,
		PoolSize:   pool,
		Status:     "ok",
		Load:       st.load,
		Sessions:   make([]SessionResult, len(inputs)),
	}
	r.Memory.SampleMs = opts.SampleMs
	run := runTag{repetition: rep, warmup: isWarmup}
	// A collection before each run, so that the previous run's garbage is not
	// collected in this one's time.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	r.Memory.GoroutinesBefore = runtime.NumGoroutine()
	stopSampler := startSampler(opts.SampleMs, &r.Memory.PeakGoroutines)

	r.StartedAt = time.Now().UTC()
	start := time.Now()
	tasks := make(chan task, 1)
	var wg sync.WaitGroup
	for w := 0; w < pool; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			// One extractor per worker, as a pipeline prepares one JrPoolWorker per
			// worker: what it prepares is not in any session's extraction time.
			var x *compute_pipes.SessionExtractor
			// point is the session this worker extracts, which emit reads on a drain
			// goroutine: written before the session's rows are sent, and not again
			// until Extract has received every row back.
			var point DumpPoint
			if len(classes) > 0 {
				var emit func(string, []any)
				if opts.OnExtractedRow != nil {
					emit = func(className string, row []any) { opts.OnExtractedRow(point, className, row) }
				}
				x = compute_pipes.NewSessionExtractor(st.spec, classes, emit)
				defer x.Close()
			}
			for t := range tasks {
				point = DumpPoint{Engine: st.name, Session: inputs[t.index].name, Repetition: rep, Warmup: isWarmup}
				sr := &r.Sessions[t.index]
				sr.Worker = worker
				sr.StartNs = time.Since(start).Nanoseconds()
				runSession(opts, st, &inputs[t.index], x, run, pool == 1, sr)
			}
		}(w)
	}
	for i := range inputs {
		tasks <- task{index: i}
	}
	close(tasks)
	wg.Wait()
	r.WallNs = time.Since(start).Nanoseconds()

	stopSampler()
	r.Memory.GoroutinesAfter = runtime.NumGoroutine()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	r.Memory.Go = memDelta(&before, &after)
	r.Memory.MaxRSSKB = maxRSSKB()
	r.Memory.RSSKB = rssKB()

	for i := range r.Sessions {
		s := &r.Sessions[i]
		t := &r.Totals
		t.Sessions++
		t.InputRows += s.InputRows
		t.Loops += s.Loops
		if s.Err != "" || s.ExecuteErr != "" || s.MaxLoopReached || len(s.Exceptions) > 0 {
			t.Errors++
		}
		if s.MaxLoopReached {
			t.MaxLoopReached++
		}
		for _, rs := range s.RuleSets {
			if rs.VertexCapReached {
				t.VertexCapReached++
				break
			}
		}
		for _, n := range s.Extracted {
			t.Extracted += n
		}
		t.RdfSessionNs += s.RdfSessionNs
		t.InputAssertNs += s.InputAssertNs
		t.ExtractionNs += s.ExtractionNs
		t.ReleaseNs += s.ReleaseNs
		t.SessionNs += s.TotalNs
		for _, rs := range s.RuleSets {
			t.ReteSessionNs += rs.ReteSessionNs
			t.ExecuteNs += rs.ExecuteNs
			t.ControlNs += rs.ControlNs
		}
		if reason := sessionFailure(s); reason != "" && r.Reason == "" {
			r.Status, r.Reason = "error", reason
		}
	}
	if r.WallNs > 0 {
		secs := float64(r.WallNs) / 1e9
		r.Totals.SessionsPerSec = float64(r.Totals.Sessions) / secs
		r.Totals.RowsPerSec = float64(r.Totals.InputRows) / secs
	}
	return r
}

// sessionFailure is why a session failed, or "" when it did not. A session that
// fails makes its run's status error, as the command's README says: an err, an
// execute_err or exceptions. Only err raised it before, so a run whose engine threw
// reported ok (rules_benchmark I-34). A session that reached max_looping is counted
// in Totals.Errors and does not fail the run. A session that reached the vertex cap
// does: the engine reports the cap as an execute error (runSession sets
// VertexCapReached from it), so a run whose spec sets a low max_vertex_visits on
// purpose, which reported ok before, now reports error.
func sessionFailure(s *SessionResult) string {
	switch {
	case s.Err != "":
		return fmt.Sprintf("session %q: %s", s.Name, s.Err)
	case s.ExecuteErr != "":
		return fmt.Sprintf("session %q: execute: %s", s.Name, s.ExecuteErr)
	case len(s.Exceptions) > 0:
		return fmt.Sprintf("session %q: %d exception(s), the first: %s", s.Name, len(s.Exceptions), s.Exceptions[0])
	}
	return ""
}

// runTag is the repetition a run is, which a dump point carries.
type runTag struct {
	repetition int
	warmup     bool
}

// sessionTimer is the RuleSetsTimer of one session. The time it spends in its
// own callbacks is kept apart: the dump in dump, and the rest, which is mostly
// the walk that reads the Go engine's counters, in counters. RunRuleSets runs
// both inside the call the session's RuleSetsNs measures, so runSession
// subtracts them.
type sessionTimer struct {
	opts     *Options
	engine   string
	session  string
	run      runTag
	sr       *SessionResult
	dump     time.Duration
	counters time.Duration
	dumpErr  error
}

func (t *sessionTimer) InputAsserted(d time.Duration) {
	t.sr.InputAssertNs = d.Nanoseconds()
}

func (t *sessionTimer) RuleSetDone(rdfSession compute_pipes.JetRdfSession, reteSession compute_pipes.JetReteSession,
	timing *compute_pipes.RuleSetTiming) {
	c0 := time.Now()
	rs := RuleSetResult{
		RuleSet:         timing.RuleSet,
		ReteSessionNs:   timing.ReteSession.Nanoseconds(),
		PrepareNs:       timing.Prepare.Nanoseconds(),
		ControlNs:       timing.Control.Nanoseconds(),
		ExceptionScanNs: timing.ExceptionScan.Nanoseconds(),
		TotalNs:         timing.Total.Nanoseconds(),
		LoopsRun:        len(timing.Loops),
	}
	for _, d := range timing.Loops {
		ns := d.Nanoseconds()
		rs.ExecuteNs += ns
		rs.ExecuteMaxNs = max(rs.ExecuteMaxNs, ns)
		if t.opts.PerLoop {
			rs.LoopNs = append(rs.LoopNs, ns)
		}
	}
	if gs, ok := rdfSession.(compute_pipes.GraphSizer); ok {
		g := gs.GraphSizes()
		rs.Graph = &g
	}
	if rc, ok := reteSession.(compute_pipes.ReteCounterReader); ok {
		c := rc.ReteCounters()
		rs.Rete = &c
	}
	t.sr.RuleSets = append(t.sr.RuleSets, rs)
	t.counters += time.Since(c0)
	if t.opts.AfterRuleSet != nil && t.dumpErr == nil {
		d0 := time.Now()
		t.dumpErr = t.opts.AfterRuleSet(DumpPoint{
			Engine:     t.engine,
			Session:    t.session,
			Index:      len(t.sr.RuleSets) - 1,
			RuleSet:    timing.RuleSet,
			Repetition: t.run.repetition,
			Warmup:     t.run.warmup,
		}, rdfSession)
		t.dump += time.Since(d0)
	}
}

// runSession runs one session as executeRules does: an rdf session, the rule
// sets through RunRuleSets with the input asserted in its assertInput, the
// extraction, and the release.
func runSession(opts *Options, st *engineState, in *prepared, extractor *compute_pipes.SessionExtractor,
	run runTag, perSessionMem bool, sr *SessionResult) {
	sr.Name = in.name
	sr.InputRows = len(in.rows)
	re := st.engine
	defer func() {
		if p := recover(); p != nil {
			log.Printf("rules_bench: recovered on the %s engine, session %q: %v\n%s", st.name, in.name, p, debug.Stack())
			sr.Err = fmt.Sprintf("the %s engine panicked: %v", st.name, p)
		}
	}()
	var before runtime.MemStats
	if perSessionMem {
		runtime.ReadMemStats(&before)
	}
	sr.GoroutinesBefore = runtime.NumGoroutine()
	timer := &sessionTimer{opts: opts, engine: st.name, session: in.name, run: run, sr: sr}

	t0 := time.Now()
	rdfSession, err := re.NewRdfSession()
	sr.RdfSessionNs = time.Since(t0).Nanoseconds()
	if err != nil {
		sr.Err = fmt.Sprintf("while creating the rdf session: %v", err)
		return
	}
	released := false
	defer func() {
		if !released {
			rdfSession.Release()
		}
	}()
	t1 := time.Now()
	outcome := compute_pipes.RunRuleSets(rdfSession, re, &compute_pipes.RuleSetsConfig{
		MaxLooping: st.spec.MaxLooping,
		IsDebug:    st.spec.IsDebug,
		Timing:     timer,
	}, func(rdfSession compute_pipes.JetRdfSession) error {
		for i, row := range in.rows {
			if err := compute_pipes.AssertCastRow(st.spec, rdfSession, row); err != nil {
				return fmt.Errorf("input row %d: %w", i+1, err)
			}
		}
		return nil
	})
	sr.RuleSetsNs = (time.Since(t1) - timer.dump - timer.counters).Nanoseconds()
	sr.DumpNs = timer.dump.Nanoseconds()
	sr.CountersNs = timer.counters.Nanoseconds()
	for i := range outcome.RuleSets {
		o := &outcome.RuleSets[i]
		sr.Loops += o.Loops
		if i < len(sr.RuleSets) {
			rs := &sr.RuleSets[i]
			rs.LastLoop, rs.MaxLooping = o.Loops, o.MaxLooping
			rs.Completed, rs.MaxLoopReached = o.Completed, o.MaxLoopReached
			if o.ExecuteErr != nil {
				rs.ExecuteErr = o.ExecuteErr.Error()
				rs.VertexCapReached = strings.Contains(strings.ToLower(rs.ExecuteErr), "vertex visit")
			}
		}
	}
	sr.Completed, sr.MaxLoopReached, sr.Exceptions = outcome.Completed, outcome.MaxLoopReached, outcome.Exceptions
	if outcome.ExecuteErr != nil {
		sr.ExecuteErr = outcome.ExecuteErr.Error()
	}
	switch {
	case outcome.Err != nil:
		sr.Err = outcome.Err.Error()
	case timer.dumpErr != nil:
		sr.Err = fmt.Sprintf("graph dump: %v", timer.dumpErr)
	}
	if n := len(sr.RuleSets); n > 0 {
		sr.Graph = sr.RuleSets[n-1].Graph
	}

	if outcome.Err == nil && extractor != nil {
		t2 := time.Now()
		counts, err := extractor.Extract(rdfSession)
		sr.ExtractionNs = time.Since(t2).Nanoseconds()
		sr.Extracted = counts
		if err != nil && sr.Err == "" {
			sr.Err = err.Error()
		}
	}
	// Untimed from here to the release.
	var countDur time.Duration
	if opts.CountTriples {
		c0 := time.Now()
		n := 0
		it := rdfSession.Find()
		for !it.IsEnd() {
			n++
			it.Next()
		}
		it.Release()
		sr.TriplesTotal = &n
		countDur = time.Since(c0)
	}
	t3 := time.Now()
	rdfSession.Release()
	released = true
	sr.ReleaseNs = time.Since(t3).Nanoseconds()
	sr.TotalNs = (time.Since(t0) - timer.dump - timer.counters - countDur).Nanoseconds()
	sr.GoroutinesAfter = runtime.NumGoroutine()
	if perSessionMem {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		d := memDelta(&before, &after)
		sr.Memory = &d
	}
}

func memDelta(a, b *runtime.MemStats) MemDelta {
	return MemDelta{
		TotalAllocBytes: b.TotalAlloc - a.TotalAlloc,
		Mallocs:         b.Mallocs - a.Mallocs,
		Frees:           b.Frees - a.Frees,
		NumGC:           b.NumGC - a.NumGC,
		PauseTotalNs:    b.PauseTotalNs - a.PauseTotalNs,
		HeapInuseBytes:  b.HeapInuse,
	}
}

// startSampler samples runtime.NumGoroutine every ms milliseconds into peak,
// and returns the function that stops it. With ms 0 it does nothing.
func startSampler(ms int, peak *int) func() {
	if ms <= 0 {
		return func() {}
	}
	*peak = runtime.NumGoroutine()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Duration(ms) * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				*peak = max(*peak, runtime.NumGoroutine())
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

// maxRSSKB is getrusage's ru_maxrss, in KiB on Linux.
func maxRSSKB() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Maxrss
}

// rssKB is the resident set now, from /proc/self/statm.
func rssKB() int64 {
	s := readTrimmed("/proc/self/statm")
	f := strings.Fields(s)
	if len(f) < 2 {
		return 0
	}
	var pages int64
	if _, err := fmt.Sscan(f[1], &pages); err != nil {
		return 0
	}
	return pages * int64(syscall.Getpagesize()) / 1024
}
