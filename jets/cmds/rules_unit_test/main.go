// Command rules_unit_test runs one rules unit test (a .tst.json under a
// workspace's jet_rules/unit_tests/) and writes its result file.
//
// It is the harness process of the rules_unit_test_harness project: the
// apiserver launches it once per test, from a pool, and records the result file
// it writes; a rule author runs the same binary from a shell against a
// checked-out workspace. **It opens no database connection**: the workspace is
// read from $WORKSPACES_HOME/$WORKSPACE, compiled, and the result goes to a file.
//
//	rules_unit_test -out result.json [-engine native|go] [-rlimit_data_mb N] <test.tst.json>
//
// The exit status is 0 whenever a result file was written, whatever the test's
// status; it is 1 only when it could not be, and 2 for a usage error. See README.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_native_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// MemoryEnv names the memory ceiling when -rlimit_data_mb is not given.
const MemoryEnv = "JETS_RULES_UNIT_TEST_MEMORY_MB"

// glogVerbosity is the C++ engine's log level for is_debug: the level that
// traces rule firings, as the usi_ws assessment's harness used it.
const glogVerbosity = "35"

var (
	outPath     = flag.String("out", "", "path of the result file to write (required)")
	testFlag    = flag.String("test", "", "the .tst.json, absolute or relative to the workspace root (or give it as the argument)")
	engine      = flag.String("engine", "", "override the test's engine: native or go (turns compare_engines off)")
	rlimitDataM = flag.Int("rlimit_data_mb", 0, "memory ceiling in MiB applied to this process as RLIMIT_DATA; 0 reads "+MemoryEnv+", absent means none")
)

func main() {
	// JSON logging, as every shipped main has (utils.UseJetStoreLogger). Before
	// the memory ceiling, whose own message is logged: the logger's allocation is
	// a few kilobytes against a ceiling of hundreds of megabytes.
	utils.UseJetStoreLogger()
	flag.Parse()
	// The memory ceiling comes first, before anything allocates in earnest: the
	// apiserver shares its memory with every test it runs (definition R-8).
	// RLIMIT_DATA, not RLIMIT_AS, which a Go process exceeds before loading
	// anything (rules_unit_test_harness F91).
	limitErr := applyMemoryCeiling()

	testPath := *testFlag
	if testPath == "" && flag.NArg() == 1 {
		testPath = flag.Arg(0)
	}
	if *outPath == "" || testPath == "" || flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: rules_unit_test -out <result.json> [-engine native|go] [-rlimit_data_mb N] <test.tst.json>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	result := run(testPath, limitErr)
	if err := rulestest.WriteResultFile(*outPath, result); err != nil {
		log.Printf("rules_unit_test: %v", err)
		os.Exit(1)
	}
	fmt.Printf("rules_unit_test: %s %s on %s: %s %s\n", result.TestPath, result.LegacyMode, result.Engine, result.Status, result.Reason)
}

func run(testPath string, limitErr error) *rulestest.Result {
	home, ws := os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE")
	if limitErr != nil {
		return &rulestest.Result{TestPath: testPath, Status: rulestest.StatusError, Reason: limitErr.Error(),
			LegacyMode: rulestest.LegacyModeLegacy, Checks: []rulestest.CheckResult{}}
	}
	if home == "" || ws == "" {
		return &rulestest.Result{TestPath: testPath, Status: rulestest.StatusError,
			Reason:     "WORKSPACES_HOME and WORKSPACE must name the workspace whose build/ the test runs against",
			LegacyMode: rulestest.LegacyModeLegacy, Checks: []rulestest.CheckResult{}}
	}
	root := filepath.Join(home, ws)
	reexecForDebug(root, testPath)
	return harness.Run(harness.Options{
		WorkspaceRoot: root,
		TestPath:      testPath,
		Engine:        *engine,
		Factories: harness.Factories{
			Native: jetrules_native_adaptor.NewJetRulesFactory(),
			Go:     jetrules_go_adaptor.NewJetRulesFactory(),
		},
	})
}

func applyMemoryCeiling() error {
	mb := *rlimitDataM
	if mb == 0 {
		if v := os.Getenv(MemoryEnv); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("%s=%q is not a number of MiB", MemoryEnv, v)
			}
			mb = n
		}
	}
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
	log.Printf("rules_unit_test: memory ceiling %d MiB (RLIMIT_DATA)", mb)
	return nil
}

// reexecForDebug restarts the process with GLOG_v set when the test sets
// is_debug and runs on C++. glog reads its flags from the environment when
// libjets.so loads, before main, so setting the variable here would be too
// late. An explicit GLOG_v is left as it is, so a caller can choose the level.
func reexecForDebug(root, testPath string) {
	if os.Getenv("GLOG_v") != "" {
		return
	}
	path := testPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	td, _, err := rulestest.LoadTestDefinition(path)
	if err != nil || td == nil || !td.JetrulesConfig.IsDebug {
		return
	}
	engines, err := harness.EnginesFor(td, *engine)
	if err != nil {
		return
	}
	native := false
	for _, e := range engines {
		native = native || e == rulestest.EngineNative
	}
	if !native {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("rules_unit_test: is_debug: cannot find this executable to restart with GLOG_v: %v", err)
		return
	}
	env := append(os.Environ(), "GLOG_v="+glogVerbosity, "GLOG_logtostderr=1")
	log.Printf("rules_unit_test: is_debug: restarting with GLOG_v=%s for the C++ engine's trace", glogVerbosity)
	if err := syscall.Exec(exe, os.Args, env); err != nil {
		log.Printf("rules_unit_test: is_debug: restart failed, continuing without the C++ trace: %v", err)
	}
}
