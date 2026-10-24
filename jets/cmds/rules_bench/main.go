// Command rules_bench benchmarks the JetRules engine, C++, on a bench spec: it
// builds the engine as a pipeline does, runs every input session through the
// pipeline's rule loop (compute_pipes.RunRuleSets) with its timing sink set,
// extracts the output through the pipeline's extraction, and writes one JSON
// result per run. Package jets/rulesbench does the work.
//
// It opens no database connection: the workspace is read from
// $WORKSPACES_HOME/$WORKSPACE, compiled.
//
//	rules_bench -out_dir <dir> [-repetitions N] [-pool N] <spec.json>
//
// It benchmarked the Go engine beside C++ until the Go engine's retirement in
// October 2026; -engine go, or a spec naming go, is now refused.
//
// The exit status is 0 when every result was written, whatever the sessions'
// outcomes; 1 when the bench could not run or a result could not be written;
// 2 for a usage error. See README.md, and scripts/run_protocol.sh for the
// protocol a measured run follows.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"syscall"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_native_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulesbench"
	"github.com/artisoft-io/jetstore/jets/rulesbench/graphdump"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/utils"
)

var (
	specFlag     = flag.String("spec", "", "the bench spec (or give it as the argument)")
	outDir       = flag.String("out_dir", "", "directory the results are written to (required)")
	engineFlag   = flag.String("engine", "", "override the spec's engine: native, the only one (go is refused, the Go engine being retired)")
	repetitions  = flag.Int("repetitions", 0, "override the spec's repetitions")
	warmup       = flag.Int("warmup", 0, "override the spec's warmup runs; -warmup 0 turns them off")
	poolSize     = flag.Int("pool", 0, "override the spec's pool_size: sessions run concurrently by N workers on one engine")
	maxVisits    = flag.Int("max_vertex_visits", 0, "override the spec's max_vertex_visits")
	maxLooping   = flag.Int("max_looping", -1, "override the spec's max_looping; -1 keeps it")
	perLoop      = flag.Bool("per_loop", false, "record every loop's ExecuteRules time, not only the sum and maximum")
	countTriples = flag.Bool("count_triples", false, "count every triple of each session by iteration after extraction (untimed)")
	sampleMs     = flag.Int("sample_ms", 10, "goroutine sampling period in ms; 0 turns sampling off")
	rlimitDataMB = flag.Int("rlimit_data_mb", 0, "memory ceiling in MiB applied to this process as RLIMIT_DATA; 0 is none")
	requireTuned = flag.Bool("require_tuned", false, "refuse to run unless every allowed CPU uses the performance governor and perf_event_paranoid <= 1")
	dumpDir      = flag.String("dump_dir", "", "write the canonical graph dump after each rule set, one dump tree per repetition: <dir>/<r01|w01>/<engine>/<session>/<NN>_<ruleset>.tsv; not a timing run")
	equivalence  = flag.Bool("equivalence_only", false, "an untimed run, for comparing what the engine computes: profiles, -per_loop, -sample_ms and -require_tuned are refused, sampling is off, and each result records equivalence_only")
	observeDir   = flag.String("observe", "", "an untimed run that writes, per session, what each rule set's network holds: per vertex its beta rows when the rule set ended and at their most, its visits and the rules it serves, to <dir>/<r01|w01>/<engine>/<session>.json; the timing outputs are refused as with -equivalence_only")
	_            = flag.String("legacy", "", "refused: it chose a retraction mode, and the LEGACY retraction mode was deleted in October 2026")
	rowsDir      = flag.String("rows_dir", "", "write every extracted row, one tree per repetition: <dir>/<r01|w01>/<engine>/<session>/<class>.rows, cells typed, in extraction order; not a timing run")

	cpuProfile     = flag.String("cpuprofile", "", "write a Go CPU profile of the runs to this file")
	memProfile     = flag.String("memprofile", "", "write a Go allocation profile of the runs to this file")
	memProfileRate = flag.Int("memprofilerate", 0, "runtime.MemProfileRate while profiling memory; 0 keeps Go's default (512 KiB)")
	blockProfile   = flag.String("blockprofile", "", "write a Go blocking profile of the runs to this file (sets the block profile rate to 1)")
	mutexProfile   = flag.String("mutexprofile", "", "write a Go mutex contention profile of the runs to this file (sets the mutex profile fraction to 1)")
	traceFile      = flag.String("trace", "", "write a Go execution trace of the runs to this file")
)

// dumpWriter writes the canonical graph dump with graphdump's default options
// (the meta graph left out), and dumpPath is the dump tree's layout, the one
// jets/cmds/rules_bench_diff reads.
var (
	dumpWriter rulesbench.DumpWriter = func(w io.Writer, s compute_pipes.JetRdfSession) error {
		return graphdump.WriteSession(w, s, graphdump.Options{})
	}
	dumpPath rulesbench.DumpPathFunc = graphdump.Path
)

func main() {
	utils.UseJetStoreLogger()
	flag.Parse()
	if err := applyMemoryCeiling(*rlimitDataMB); err != nil {
		log.Printf("rules_bench: %v", err)
		os.Exit(1)
	}
	specPath := *specFlag
	if specPath == "" && flag.NArg() == 1 {
		specPath = flag.Arg(0)
	}
	if *outDir == "" || specPath == "" || flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: rules_bench -out_dir <dir> [flags] <spec.json>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	if err := run(specPath); err != nil {
		log.Printf("rules_bench: %v", err)
		os.Exit(1)
	}
}

func run(specPath string) error {
	if os.Getenv("WORKSPACES_HOME") == "" || os.Getenv("WORKSPACE") == "" {
		return errors.New("WORKSPACES_HOME and WORKSPACE must name the compiled workspace the bench runs against")
	}
	given := visitedFlags(flag.CommandLine)
	if err := refuseLegacyFlag(given); err != nil {
		return err
	}
	if *equivalence || *observeDir != "" {
		untimed := "-equivalence_only"
		if *observeDir != "" {
			untimed = "-observe"
		}
		if err := refuseTimingFlags(given, untimed); err != nil {
			return err
		}
	}
	spec, err := rulesbench.LoadSpec(specPath)
	if err != nil {
		return err
	}
	if abs, err := filepath.Abs(specPath); err == nil {
		specPath = abs
	}
	env := rulesbench.RecordEnvironment()
	// A preloaded library the loader ignored would label a run with an
	// allocator it did not use.
	if missing := env.PreloadNotMapped(); len(missing) > 0 {
		return fmt.Errorf("LD_PRELOAD names %v, which the loader did not map into this process", missing)
	}
	if *requireTuned {
		paranoid := env.PerfEventParanoid
		if !env.GovernorsOK() || !(paranoid == "-1" || paranoid == "0" || paranoid == "1") {
			return fmt.Errorf("-require_tuned: governors %v, perf_event_paranoid %q; want performance on every allowed CPU and 1 or less", env.Governors, paranoid)
		}
	}
	// The options before anything is written: a refused run leaves out_dir
	// untouched (Run checks them again).
	modeOpts := rulesbench.Options{EquivalenceOnly: *equivalence, ObserveDir: *observeDir, PerLoop: *perLoop}
	if !*equivalence && *observeDir == "" {
		modeOpts.SampleMs = *sampleMs
	}
	if err := rulesbench.CheckOptions(modeOpts); err != nil {
		return err
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(*outDir, "env.json"), env); err != nil {
		return err
	}

	opts := rulesbench.Options{
		Spec:            spec,
		SpecPath:        specPath,
		Factory:         jetrules_native_adaptor.NewJetRulesFactory(),
		Repetitions:     *repetitions,
		PoolSize:        *poolSize,
		MaxVertexVisits: *maxVisits,
		PerLoop:         *perLoop,
		CountTriples:    *countTriples,
		SampleMs:        *sampleMs,
		EquivalenceOnly: *equivalence,
		ObserveDir:      *observeDir,
		Env:             env,
	}
	if *equivalence || *observeDir != "" {
		opts.SampleMs = 0
	}
	// A flag given as 0 overrides the spec's warmup; a flag not given keeps it.
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "warmup" {
			opts.Warmup = warmup
		}
	})
	if *engineFlag != "" {
		opts.Engines = strings.Split(*engineFlag, ",")
	}
	if *maxLooping >= 0 {
		opts.MaxLooping = maxLooping
	}
	if *dumpDir != "" {
		opts.AfterRuleSet = rulesbench.NewDumper(*dumpDir, dumpPath, dumpWriter)
	}
	var rows *rulesbench.RowWriter
	if *rowsDir != "" {
		rows = rulesbench.NewRowWriter(*rowsDir)
		opts.OnExtractedRow = rows.Write
		opts.AfterExtraction = rows.SessionDone
	}
	stopProfiles := func() error { return nil }
	opts.BeforeRuns = func() {
		var err error
		if stopProfiles, err = startProfiles(); err != nil {
			log.Printf("rules_bench: %v", err)
			os.Exit(1)
		}
	}
	var profileErr error
	opts.AfterRuns = func() { profileErr = stopProfiles() }
	var writeErr error
	opts.OnResult = func(r *rulesbench.RunResult) {
		kind := "r"
		if r.Warmup {
			kind = "w"
		}
		name := fmt.Sprintf("%s_%03d_%s_%s%02d.json", fileSafe(r.Bench), r.Order, r.Engine, kind, r.Repetition)
		if err := writeJSON(filepath.Join(*outDir, name), r); err != nil && writeErr == nil {
			writeErr = err
		}
		fmt.Printf("rules_bench: %s %s %s%02d pool %d: %s, %d sessions, %d rows, wall %s, %d errors, release rete %s rdf %s\n",
			r.Bench, r.Engine, kind, r.Repetition, r.PoolSize, r.Status, r.Totals.Sessions, r.Totals.InputRows,
			time.Duration(r.WallNs), r.Totals.Errors,
			time.Duration(r.Totals.ReteReleaseNs), time.Duration(r.Totals.ReleaseNs))
	}
	_, err = rulesbench.Run(opts)
	if rows != nil {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("rows_dir: %w", cerr)
		}
	}
	if err != nil {
		return err
	}
	if profileErr != nil {
		return profileErr
	}
	return writeErr
}

// timingFlags are the flags that ask for a measurement, which an
// -equivalence_only or -observe run refuses rather than produce numbers from an
// untimed run.
var timingFlags = []string{"cpuprofile", "memprofile", "memprofilerate", "blockprofile",
	"mutexprofile", "trace", "per_loop", "sample_ms", "require_tuned"}

// visitedFlags are the names of the flags fs was given.
func visitedFlags(fs *flag.FlagSet) map[string]bool {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	return given
}

// refuseLegacyFlag refuses -legacy, whatever its value: it chose a retraction
// mode, and there is one behaviour since the LEGACY retraction mode was deleted.
func refuseLegacyFlag(given map[string]bool) error {
	if given["legacy"] {
		return fmt.Errorf("-legacy chose a retraction mode: %s; remove it", rulestest.LegacyRetractionDeleted)
	}
	return nil
}

// refuseTimingFlags refuses, in the untimed run named by untimed, the timing
// flags among given.
func refuseTimingFlags(given map[string]bool, untimed string) error {
	var refused []string
	for _, name := range timingFlags {
		if given[name] {
			refused = append(refused, "-"+name)
		}
	}
	if len(refused) > 0 {
		return fmt.Errorf("%s is an untimed run and refuses %s", untimed, strings.Join(refused, ", "))
	}
	return nil
}

// startProfiles starts the Go profiles the flags ask for, and returns the
// function that stops them and writes the ones written at the end.
func startProfiles() (func() error, error) {
	var stops []func() error
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			return nil, err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, err
		}
		stops = append(stops, func() error { pprof.StopCPUProfile(); return f.Close() })
	}
	if *traceFile != "" {
		f, err := os.Create(*traceFile)
		if err != nil {
			return nil, err
		}
		if err := trace.Start(f); err != nil {
			f.Close()
			return nil, err
		}
		stops = append(stops, func() error { trace.Stop(); return f.Close() })
	}
	if *memProfile != "" && *memProfileRate > 0 {
		runtime.MemProfileRate = *memProfileRate
	}
	if *blockProfile != "" {
		runtime.SetBlockProfileRate(1)
	}
	if *mutexProfile != "" {
		runtime.SetMutexProfileFraction(1)
	}
	writeProfile := func(name, path string) func() error {
		return func() error {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			if err := pprof.Lookup(name).WriteTo(f, 0); err != nil {
				f.Close()
				return err
			}
			return f.Close()
		}
	}
	if *memProfile != "" {
		stops = append(stops, writeProfile("allocs", *memProfile))
	}
	if *blockProfile != "" {
		stops = append(stops, writeProfile("block", *blockProfile))
	}
	if *mutexProfile != "" {
		stops = append(stops, writeProfile("mutex", *mutexProfile))
	}
	return func() error {
		var first error
		for _, stop := range stops {
			if err := stop(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}, nil
}

func applyMemoryCeiling(mb int) error {
	if mb < 0 {
		return errors.New("-rlimit_data_mb must not be negative")
	}
	if mb == 0 {
		return nil
	}
	limit := uint64(mb) << 20
	if err := syscall.Setrlimit(syscall.RLIMIT_DATA, &syscall.Rlimit{Cur: limit, Max: limit}); err != nil {
		return fmt.Errorf("while setting the memory ceiling of %d MiB: %v", mb, err)
	}
	log.Printf("rules_bench: memory ceiling %d MiB (RLIMIT_DATA)", mb)
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fileSafe(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, s)
}
