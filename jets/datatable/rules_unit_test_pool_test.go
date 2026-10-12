package datatable

// The executor pool (AC.3) and the recording of a run (AC.4), against fake
// harnesses: shell scripts named through JETS_RULES_UNIT_TEST_HARNESS that write
// a result, exit without one, overrun the deadline, or allocate past the memory
// ceiling. So the pool is tested without the C++ engine (plan §5). Needs
// JETS_TEST_DSN like rules_unit_test_runs_test.go, whose fixture it uses; the
// memory tests need python3.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeHarness writes an executable script and names it as the harness. The
// script sees the pool's command line: $1 -out, $2 the result path,
// $3 -rlimit_data_mb, $4 the limit, $5 the test path.
func fakeHarness(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake_rules_unit_test")
	if err := os.WriteFile(p, []byte("#!/bin/bash\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(RulesUnitTestHarnessEnvVar, p)
}

// queueRuns inserts n queued runs of rutTestA and returns them as the post-hook
// hands them over.
func (f *rutFixture) queueRuns(t *testing.T, n int) []RulesUnitTestRun {
	t.Helper()
	runs := make([]RulesUnitTestRun, n)
	for i := range runs {
		var key int
		err := f.pool.QueryRow(context.Background(), `INSERT INTO jetsapi.rules_unit_test_execution
			(workspace_name, test_path, test_name, legacy_mode, status, user_email)
			VALUES ($1, $2, $3, 'legacy', 'queued', 'u') RETURNING key`,
			rutActiveWorkspace, rutTestA, fmt.Sprintf("run %d", i)).Scan(&key)
		if err != nil {
			t.Fatal(err)
		}
		runs[i] = RulesUnitTestRun{Key: key, WorkspaceName: rutActiveWorkspace, TestPath: rutTestA, TestName: fmt.Sprintf("run %d", i)}
	}
	return runs
}

type rutRow struct {
	status, reason, engine, legacy, log    string
	loadMs, runMs, durationMs, sess, loops int64
}

func (f *rutFixture) row(t *testing.T, key int) rutRow {
	t.Helper()
	var r rutRow
	err := f.pool.QueryRow(context.Background(), `SELECT status, coalesce(reason, ''), coalesce(engine, ''),
			coalesce(legacy_mode, ''), coalesce(log, ''), coalesce(load_ms, -1), coalesce(run_ms, -1),
			coalesce(duration_ms, -1), coalesce(sessions, -1), coalesce(loops, -1)
		FROM jetsapi.rules_unit_test_execution WHERE key = $1`, key).
		Scan(&r.status, &r.reason, &r.engine, &r.legacy, &r.log, &r.loadMs, &r.runMs, &r.durationMs, &r.sess, &r.loops)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func runPool(t *testing.T, f *rutFixture, config RulesUnitTestPoolConfig, runs []RulesUnitTestRun) {
	t.Helper()
	p := NewRulesUnitTestPool(config)
	done := make(chan struct{})
	p.Submit(f.pool, runs)
	go func() { p.wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the pool did not finish its runs within a minute")
	}
}

const passResult = `{
  "test_path": "jet_rules/unit_tests/a/a.tst.json", "test_name": "test a",
  "status": "pass", "engine": "native+go", "legacy_mode": "fixed-1",
  "load_ms": 12, "run_ms": 7, "duration_ms": 30, "sessions": 2, "loops": 5,
  "a_field_a_newer_harness_writes": true,
  "checks": [
    {"session_name": "s1", "kind": "expected", "check": "[[\"?x\",\"rdf:type\",\"hc:A\"]]", "outcome": "pass", "found": "?x=hc:a1", "engine": "native"},
    {"session_name": "s1", "kind": "expected", "check": "[[\"?x\",\"rdf:type\",\"hc:A\"]]", "outcome": "pass", "found": "?x=hc:a1", "engine": "go"},
    {"session_name": "s2", "kind": "count", "check": "{\"count\":1}", "outcome": "pass", "found": "1"}
  ]
}`

// A harness that writes a result: the run row takes the result's fields and the
// apiserver's duration, the checks are recorded with their engine, the log is
// both streams, and the command line is the one the file comment documents.
func TestPoolRecordsAResult(t *testing.T) {
	f := newRutFixture(t)
	seen := filepath.Join(t.TempDir(), "seen")
	// The apiserver task's secret-bearing variables (cdk/jetstore_one/stack),
	// and what the harness needs beside them.
	secrets := []string{"API_SECRET", "AWS_API_SECRET", "JETS_ENCRYPTION_KEY", "JETS_ENCRYPTION_KEY_SECRET",
		"JETS_DSN_SECRET", "JETS_DSN_JSON_VALUE", "JETS_ADMIN_PWD", "AWS_JETS_ADMIN_PWD_SECRET",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"}
	for _, name := range secrets {
		t.Setenv(name, "must-not-reach-the-harness")
	}
	t.Setenv("GLOG_v", "2")
	t.Setenv("JETS_BUCKET", "a-bucket")
	t.Setenv("LD_LIBRARY_PATH", "/usr/local/lib")
	fakeHarness(t, `env > `+seen+`.env
echo "args: $*" > `+seen+`
echo "cwd: $PWD" >> `+seen+`
echo "env: $WORKSPACES_HOME $WORKSPACE" >> `+seen+`
echo "to stdout"
echo "to stderr" >&2
cat > "$2" <<'JSON'
`+passResult+`
JSON`)
	runs := f.queueRuns(t, 1)
	runPool(t, f, RulesUnitTestPoolConfig{Size: 1, MemoryMB: 300, Timeout: 30 * time.Second}, runs)

	r := f.row(t, runs[0].Key)
	if r.status != RulesUnitTestPass || r.engine != "native+go" || r.legacy != "fixed-1" ||
		r.loadMs != 12 || r.runMs != 7 || r.sess != 2 || r.loops != 5 || r.reason != "" {
		t.Errorf("run row %+v does not carry the result", r)
	}
	if r.durationMs < 0 {
		t.Errorf("duration_ms not recorded")
	}
	if !strings.Contains(r.log, "to stdout") || !strings.Contains(r.log, "to stderr") {
		t.Errorf("log %q lacks a stream", r.log)
	}
	rows, err := f.pool.Query(context.Background(), `SELECT session_name, kind, "check", outcome, found, coalesce(engine, '')
		FROM jetsapi.rules_unit_test_check WHERE execution_key = $1 ORDER BY key`, runs[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var s, k, c, o, fd, e string
		if err = rows.Scan(&s, &k, &c, &o, &fd, &e); err != nil {
			t.Fatal(err)
		}
		got = append(got, strings.Join([]string{s, k, o, fd, e}, "|"))
	}
	rows.Close()
	want := []string{"s1|expected|pass|?x=hc:a1|native", "s1|expected|pass|?x=hc:a1|go", "s2|count|pass|1|"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("checks\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	env, err := os.ReadFile(seen + ".env")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(env), "must-not-reach-the-harness") {
		t.Errorf("a secret reached the harness:\n%s", env)
	}
	for _, w := range []string{"\nGLOG_v=2\n", "\nJETS_BUCKET=a-bucket\n", "\nLD_LIBRARY_PATH=/usr/local/lib\n"} {
		if !strings.Contains("\n"+string(env), w) {
			t.Errorf("the harness environment lacks %q", strings.TrimSpace(w))
		}
	}
	b, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(os.Getenv("WORKSPACES_HOME"), rutActiveWorkspace)
	for _, w := range []string{"args: -out ", " -rlimit_data_mb 300 " + rutTestA, "cwd: " + ws,
		"env: " + os.Getenv("WORKSPACES_HOME") + " " + rutActiveWorkspace} {
		if !strings.Contains(string(b), w) {
			t.Errorf("the harness saw\n%s\nwhich lacks %q", b, w)
		}
	}
}

// Every way a run can end without a usable result is an error with a reason
// that says which, and the log.
func TestPoolErrorsWithoutAResult(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is needed for the memory cases")
	}
	alloc := `python3 -c '
import sys, time
try:
    x = bytearray(512 * 1024 * 1024)
except MemoryError:
    print("fatal error: out of memory", file=sys.stderr)
    sys.exit(2)
print("allocated")'`
	cases := []struct {
		name, body string
		timeout    time.Duration
		reason     []string
		logHas     string
		maxSeconds float64
	}{
		{"no result file", `echo "about to fail" >&2; exit 3`, 0,
			[]string{"ended (exit status 3)", "without writing a result"}, "about to fail", 10},
		{"exit 0 and no result", `echo "forgot"`, 0,
			[]string{"ended (exit status 0)", "without writing a result"}, "forgot", 10},
		{"unreadable result", `echo '{"status":"queued"}' > "$2"`, 0,
			[]string{"exit status 0", "could not be read", `status "queued"`}, "", 10},
		{"killed by a signal", `echo "aborting"; kill -ABRT $$`, 0,
			[]string{"signal: aborted", "without writing a result"}, "aborting", 10},
		// A child of the harness keeps the log pipe open; the deadline kills the
		// whole process group, so the run ends at the deadline -- not at 30 s, and
		// not at the deadline plus WaitDelay, which is what killing the harness
		// alone costs.
		{"overruns its deadline", `echo "looping"; sleep 30 & wait`, time.Second,
			[]string{"timed out", "1s deadline"}, "looping", 4},
		// The harness applies -rlimit_data_mb to itself first, as AB.9 will.
		{"past its ceiling, self-limited", `ulimit -d $(( $4 * 1024 )); ` + alloc, 0,
			[]string{"memory limit, inferred from the log", `"out of memory"`, "64 MiB", "exit status 2"}, "fatal error: out of memory", 10},
		// The harness ignores the flag; the apiserver's prlimit holds it anyway.
		{"past its ceiling, backstop", `sleep 0.3; ` + alloc, 0,
			[]string{"memory limit, inferred from the log", "exit status 2"}, "fatal error: out of memory", 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRutFixture(t)
			fakeHarness(t, c.body)
			runs := f.queueRuns(t, 1)
			timeout := c.timeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			start := time.Now()
			runPool(t, f, RulesUnitTestPoolConfig{Size: 1, MemoryMB: 64, Timeout: timeout}, runs)
			if s := time.Since(start).Seconds(); s > c.maxSeconds {
				t.Errorf("the run took %.1f s", s)
			}
			r := f.row(t, runs[0].Key)
			if r.status != RulesUnitTestError {
				t.Errorf("status %q, want error", r.status)
			}
			for _, w := range c.reason {
				if !strings.Contains(r.reason, w) {
					t.Errorf("reason %q lacks %q", r.reason, w)
				}
			}
			if !strings.Contains(r.log, c.logHas) {
				t.Errorf("log %q lacks %q", r.log, c.logHas)
			}
			if r.durationMs < 0 {
				t.Errorf("duration_ms not recorded")
			}
		})
	}
}

// A missing harness ends every run as an error naming the variable.
func TestPoolWithoutAHarness(t *testing.T) {
	f := newRutFixture(t)
	t.Setenv(RulesUnitTestHarnessEnvVar, filepath.Join(t.TempDir(), "absent"))
	runs := f.queueRuns(t, 2)
	runPool(t, f, RulesUnitTestPoolConfig{Size: 2}, runs)
	for _, run := range runs {
		if r := f.row(t, run.Key); r.status != RulesUnitTestError || !strings.Contains(r.reason, RulesUnitTestHarnessEnvVar) {
			t.Errorf("run %d is (%q, %q)", run.Key, r.status, r.reason)
		}
	}
}

// Six runs on a pool of two: never more than two at once, at some point two,
// started in the order submitted, and each row queued until its slot is free.
func TestPoolRunsAtMostItsSizeInOrder(t *testing.T) {
	f := newRutFixture(t)
	trace := filepath.Join(t.TempDir(), "trace")
	fakeHarness(t, `echo "start $(date +%s%N) $PPID" >> `+trace+`
sleep 0.4
echo "end $(date +%s%N)" >> `+trace+`
echo '{"status":"pass","checks":[]}' > "$2"`)
	runs := f.queueRuns(t, 6)
	p := NewRulesUnitTestPool(RulesUnitTestPoolConfig{Size: 2, Timeout: 30 * time.Second})
	p.Submit(f.pool, runs)

	// While the first two run, the last four are still queued.
	time.Sleep(200 * time.Millisecond)
	var queued, executing int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE status = 'queued'),
		count(*) FILTER (WHERE status = 'executing') FROM jetsapi.rules_unit_test_execution`).Scan(&queued, &executing)
	if executing != 2 || queued != 4 {
		t.Errorf("200 ms in: %d executing and %d queued, want 2 and 4", executing, queued)
	}
	p.wait()

	b, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	type ev struct {
		at    int64
		delta int
	}
	var evs []ev
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		fs := strings.Fields(l)
		at, _ := strconv.ParseInt(fs[1], 10, 64)
		d := 1
		if fs[0] == "end" {
			d = -1
		}
		evs = append(evs, ev{at, d})
	}
	sort.Slice(evs, func(i, j int) bool { return evs[i].at < evs[j].at })
	running, peak := 0, 0
	for _, e := range evs {
		running += e.delta
		peak = max(peak, running)
	}
	if peak != 2 {
		t.Errorf("peak concurrency %d, want 2", peak)
	}
	// FIFO: each run's executing time is no earlier than the one before it.
	var prev time.Time
	for i, run := range runs {
		if r := f.row(t, run.Key); r.status != RulesUnitTestPass {
			t.Errorf("run %d ended %q %q", i, r.status, r.reason)
		}
		var started time.Time
		_ = f.pool.QueryRow(context.Background(), `SELECT last_update - make_interval(secs => duration_ms / 1000.0)
			FROM jetsapi.rules_unit_test_execution WHERE key = $1`, run.Key).Scan(&started)
		if i > 1 && started.Before(prev.Add(-50*time.Millisecond)) {
			t.Errorf("run %d started at %v, before run %d at %v", i, started, i-1, prev)
		}
		prev = started
	}
}

// A run that is no longer queued when its turn comes -- swept by a restart of
// another apiserver, say -- is not run.
func TestPoolSkipsARunNoLongerQueued(t *testing.T) {
	f := newRutFixture(t)
	ran := filepath.Join(t.TempDir(), "ran")
	fakeHarness(t, `touch `+ran+`; echo '{"status":"pass"}' > "$2"`)
	runs := f.queueRuns(t, 1)
	if _, err := f.pool.Exec(context.Background(), `UPDATE jetsapi.rules_unit_test_execution
		SET status = 'error', reason = 'interrupted'`); err != nil {
		t.Fatal(err)
	}
	runPool(t, f, RulesUnitTestPoolConfig{Size: 1}, runs)
	if _, err := os.Stat(ran); err == nil {
		t.Error("the harness ran for a run that was not queued")
	}
	if r := f.row(t, runs[0].Key); r.reason != "interrupted" {
		t.Errorf("the run's row was overwritten: %+v", r)
	}
}

// A log larger than the cap keeps its tail, opening with a line saying how much
// was cut; NUL bytes and invalid UTF-8 a crash can print do not stop the row
// being written.
func TestPoolKeepsTheLogTail(t *testing.T) {
	f := newRutFixture(t)
	fakeHarness(t, `for i in $(seq 1 3000); do echo "line $i of the harness log"; done
printf 'bad \xff\xfe bytes and a \x00 nul\n'
echo "the last line"
echo '{"status":"fail","reason":"1 of 2 checks failed","checks":[]}' > "$2"`)
	runs := f.queueRuns(t, 1)
	runPool(t, f, RulesUnitTestPoolConfig{Size: 1, LogCap: 4096, Timeout: 30 * time.Second}, runs)
	r := f.row(t, runs[0].Key)
	if r.status != RulesUnitTestFail || r.reason != "1 of 2 checks failed" {
		t.Errorf("run is (%q, %q)", r.status, r.reason)
	}
	if !strings.HasPrefix(r.log, "[log truncated: the first ") {
		t.Errorf("log does not open with the truncation line: %.80q", r.log)
	}
	if !strings.HasSuffix(r.log, "the last line\n") || !strings.Contains(r.log, "\nline 3000 of the harness log\n") {
		t.Errorf("log does not keep the tail: ...%q", r.log[max(0, len(r.log)-200):])
	}
	if body := r.log[strings.IndexByte(r.log, '\n')+1:]; len(body) > 4096 || !strings.HasPrefix(body, "line ") {
		t.Errorf("kept %d bytes starting %.20q, want at most 4096 starting at a line", len(body), body)
	}
}

func TestTailBuffer(t *testing.T) {
	b := newTailBuffer(10)
	_, _ = b.Write([]byte("short"))
	if b.String() != "short" {
		t.Errorf("under the cap: %q", b.String())
	}
	_, _ = b.Write([]byte("éé and more"))
	s := b.String()
	if !strings.HasPrefix(s, "[log truncated: the first ") || !strings.HasSuffix(s, " and more") {
		t.Errorf("over the cap: %q", s)
	}
}

func TestHarnessEnvironment(t *testing.T) {
	got := harnessEnviron([]string{
		"PATH=/bin", "OLDPWD=/tmp", "PWD=/x", "JETS_ADMIN_PWD=p", "MY_PASSWORD=p", "JETS_DSN_SECRET=arn",
		"jets_encryption_key=k", "AWS_REGION=us-east-1", "AWS_PROFILE=dev", "AWS_ACCESS_KEY_ID=id",
		"WORKSPACE=stale", "JETS_VERSION=1", "GLOG_v=3",
	}, "/home", "ws")
	want := "PATH=/bin OLDPWD=/tmp AWS_REGION=us-east-1 JETS_VERSION=1 GLOG_v=3 WORKSPACES_HOME=/home WORKSPACE=ws"
	if strings.Join(got, " ") != want {
		t.Errorf("got  %s\nwant %s", strings.Join(got, " "), want)
	}
}
