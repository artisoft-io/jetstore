package datatable

// The rules unit test executor pool, and the recording of what each run did.
// Project rules_unit_test_harness, Phase 1, track AC: AC.3 (the pool) and AC.4
// (recording), definition §5.4, §5.5 and §6.
//
// # One pool per apiserver, one process per test
//
// The pool is a process-wide FIFO queue served by a fixed number of workers, so
// two batches started together share the workers rather than doubling them
// (§5.5, item 1). Each worker runs one harness process per test: the engine,
// the LEGACY mode and rule_config are fixed when the engine is built, and a fault
// in the C++ engine ends the process it is in -- so it must not be the apiserver.
//
// # The defaults, and why they are small
//
// Measured at AB.1 (2026-10-11): an engine loads in 2 to 200 ms and the load is
// CPU-bound, so on the apiserver's one vCPU a pool buys isolation -- a runaway
// test held to its deadline does not stall the queue -- rather than overlap. A
// real test takes 8 to 71 MB private memory plus about 48 MB of shared binary
// pages. Two processes at 512 MiB each leave at least 3 GiB of the 4 GiB task to
// the apiserver.
//
//   - JETS_RULES_UNIT_TEST_POOL_SIZE: workers, default 2.
//   - JETS_RULES_UNIT_TEST_MEMORY_MB: each harness's RLIMIT_DATA, default 512.
//     Passed to the harness as -rlimit_data_mb, which it applies to itself
//     before anything else; the apiserver also sets it on the child right after
//     starting it, as a backstop. RLIMIT_AS is not usable: the Go runtime and
//     the engine reserve 1.5 to 2.6 GB of address space before loading anything.
//   - JETS_RULES_UNIT_TEST_TIMEOUT: each process's deadline, a Go duration,
//     default 10m. It starts when the process starts, not when the run is queued.
//   - JETS_RULES_UNIT_TEST_HARNESS: the harness binary. Unset, it is
//     rules_unit_test beside the apiserver's executable -- locateValidatorScript's
//     rule (jets/workspace/asset_validation.go).
//
// # The command line
//
//	rules_unit_test -out <result.json> -rlimit_data_mb <N> <test_path>
//
// with WORKSPACES_HOME and WORKSPACE set to the run's workspace, every
// secret-bearing variable of the apiserver's environment removed (harnessEnviron),
// and the working directory the workspace root; test_path is relative to that root, as the compile recorded
// it. The harness writes the result file and exits non-zero only when it could
// not; its standard output and error are the log.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Environment variables that configure the pool and locate the harness.
const (
	RulesUnitTestHarnessEnvVar  = "JETS_RULES_UNIT_TEST_HARNESS"
	RulesUnitTestPoolSizeEnvVar = "JETS_RULES_UNIT_TEST_POOL_SIZE"
	RulesUnitTestMemoryEnvVar   = "JETS_RULES_UNIT_TEST_MEMORY_MB"
	RulesUnitTestTimeoutEnvVar  = "JETS_RULES_UNIT_TEST_TIMEOUT"

	// RulesUnitTestHarnessName is the harness binary's name beside the apiserver.
	RulesUnitTestHarnessName = "rules_unit_test"
)

// Defaults, from AB.1's measurement (see the file comment).
const (
	DefaultRulesUnitTestPoolSize = 2
	DefaultRulesUnitTestMemoryMB = 512
	DefaultRulesUnitTestTimeout  = 10 * time.Minute
)

// RulesUnitTestLogCap is the most of a harness log a run row keeps: the tail,
// because the end of a log is where a failure is. Below the 4,000,000 characters
// the log dialog accepts (definition F67), with room for the line saying how
// much was cut.
const RulesUnitTestLogCap = 1_000_000

// RulesUnitTestPoolConfig is the pool's configuration.
type RulesUnitTestPoolConfig struct {
	Size     int
	MemoryMB int
	Timeout  time.Duration
	// LogCap is the log tail kept, in bytes; RulesUnitTestLogCap when zero.
	LogCap int
}

// RulesUnitTestPoolConfigFromEnv reads the configuration, falling back to the
// default -- with a log line -- for a value that does not parse.
func RulesUnitTestPoolConfigFromEnv() RulesUnitTestPoolConfig {
	c := RulesUnitTestPoolConfig{
		Size:     DefaultRulesUnitTestPoolSize,
		MemoryMB: DefaultRulesUnitTestMemoryMB,
		Timeout:  DefaultRulesUnitTestTimeout,
		LogCap:   RulesUnitTestLogCap,
	}
	positive := func(name string, dst *int) {
		v := os.Getenv(name)
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			log.Printf("Warning: %s=%q is not a positive integer, using %d", name, v, *dst)
			return
		}
		*dst = n
	}
	positive(RulesUnitTestPoolSizeEnvVar, &c.Size)
	positive(RulesUnitTestMemoryEnvVar, &c.MemoryMB)
	if v := os.Getenv(RulesUnitTestTimeoutEnvVar); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			log.Printf("Warning: %s=%q is not a positive duration, using %v", RulesUnitTestTimeoutEnvVar, v, c.Timeout)
		} else {
			c.Timeout = d
		}
	}
	return c
}

// locateRulesUnitTestHarness returns the harness binary: the environment
// variable when set, else rules_unit_test beside the running executable.
func locateRulesUnitTestHarness() (string, error) {
	if p := os.Getenv(RulesUnitTestHarnessEnvVar); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s=%s: %w", RulesUnitTestHarnessEnvVar, p, err)
		}
		return p, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate %s beside the executable: %w", RulesUnitTestHarnessName, err)
	}
	p := filepath.Join(filepath.Dir(exe), RulesUnitTestHarnessName)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s is not beside %s (%w); set %s to its path",
			RulesUnitTestHarnessName, exe, err, RulesUnitTestHarnessEnvVar)
	}
	return p, nil
}

type rulesUnitTestJob struct {
	dbpool *pgxpool.Pool
	run    RulesUnitTestRun
}

// RulesUnitTestPool runs queued rules unit tests, at most Size at a time, in the
// order they were submitted.
type RulesUnitTestPool struct {
	config  RulesUnitTestPoolConfig
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []rulesUnitTestJob
	pending sync.WaitGroup
}

// NewRulesUnitTestPool starts the pool's workers.
func NewRulesUnitTestPool(config RulesUnitTestPoolConfig) *RulesUnitTestPool {
	if config.Size < 1 {
		config.Size = DefaultRulesUnitTestPoolSize
	}
	if config.MemoryMB < 1 {
		config.MemoryMB = DefaultRulesUnitTestMemoryMB
	}
	if config.Timeout <= 0 {
		config.Timeout = DefaultRulesUnitTestTimeout
	}
	if config.LogCap <= 0 {
		config.LogCap = RulesUnitTestLogCap
	}
	p := &RulesUnitTestPool{config: config}
	p.cond = sync.NewCond(&p.mu)
	for range config.Size {
		go p.worker()
	}
	log.Printf("Rules unit test pool: %d workers, %d MiB data limit and %v deadline per harness process",
		config.Size, config.MemoryMB, config.Timeout)
	return p
}

// Submit queues the runs behind those already queued and returns at once.
func (p *RulesUnitTestPool) Submit(dbpool *pgxpool.Pool, runs []RulesUnitTestRun) {
	p.mu.Lock()
	for _, r := range runs {
		p.pending.Add(1)
		p.queue = append(p.queue, rulesUnitTestJob{dbpool: dbpool, run: r})
	}
	p.mu.Unlock()
	p.cond.Broadcast()
}

// wait returns when every submitted run has been recorded. For tests.
func (p *RulesUnitTestPool) wait() { p.pending.Wait() }

func (p *RulesUnitTestPool) worker() {
	for {
		p.mu.Lock()
		for len(p.queue) == 0 {
			p.cond.Wait()
		}
		job := p.queue[0]
		p.queue = p.queue[1:]
		p.mu.Unlock()
		p.execute(job)
		p.pending.Done()
	}
}

// execute runs one test and records it. A panic here would end the apiserver,
// so it is recovered into the run's row.
func (p *RulesUnitTestPool) execute(job rulesUnitTestJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("rules unit test run %d: panic in the executor: %v", job.run.Key, r)
			recordRulesUnitTestFailure(job.dbpool, job.run.Key, fmt.Sprintf("the executor failed: %v", r), "", 0)
		}
	}()
	ctx := context.Background()
	tag, err := job.dbpool.Exec(ctx, `UPDATE jetsapi.rules_unit_test_execution
		SET status = $1, last_update = DEFAULT WHERE key = $2 AND status = $3`,
		RulesUnitTestExecuting, job.run.Key, RulesUnitTestQueued)
	if err != nil {
		log.Printf("rules unit test run %d: while marking it executing: %v", job.run.Key, err)
		return
	}
	if tag.RowsAffected() == 0 {
		// Swept, deleted or already ended: nothing is waiting for this run.
		log.Printf("rules unit test run %d is no longer queued, not run", job.run.Key)
		return
	}
	outcome := p.runHarness(job.run)
	if err := recordRulesUnitTestOutcome(job.dbpool, job.run.Key, outcome); err != nil {
		log.Printf("rules unit test run %d: while recording its outcome: %v", job.run.Key, err)
		recordRulesUnitTestFailure(job.dbpool, job.run.Key,
			fmt.Sprintf("the outcome could not be recorded: %v", err), outcome.log, outcome.durationMs)
	}
}

// rulesUnitTestOutcome is what one harness process produced.
type rulesUnitTestOutcome struct {
	result     *rulestest.Result
	reason     string // set when there is no result
	log        string
	durationMs int64
}

// runHarness runs one harness process to completion or to its deadline.
func (p *RulesUnitTestPool) runHarness(run RulesUnitTestRun) (out rulesUnitTestOutcome) {
	harness, err := locateRulesUnitTestHarness()
	if err != nil {
		out.reason = "the harness cannot be found: " + err.Error()
		return
	}
	dir, err := os.MkdirTemp("", "rules_unit_test_")
	if err != nil {
		out.reason = "cannot create the result directory: " + err.Error()
		return
	}
	defer os.RemoveAll(dir)
	resultPath := filepath.Join(dir, "result.json")

	workspacesHome := os.Getenv("WORKSPACES_HOME")
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, harness,
		"-out", resultPath,
		"-rlimit_data_mb", strconv.Itoa(p.config.MemoryMB),
		run.TestPath)
	cmd.Dir = filepath.Join(workspacesHome, run.WorkspaceName)
	cmd.Env = harnessEnviron(os.Environ(), workspacesHome, run.WorkspaceName)
	logBuf := newTailBuffer(p.config.LogCap)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	prepareHarnessCommand(cmd)

	start := time.Now()
	err = cmd.Start()
	if err != nil {
		out.reason = "the harness could not start: " + err.Error()
		out.log = logBuf.String()
		return
	}
	limitHarnessMemory(cmd.Process.Pid, p.config.MemoryMB)
	waitErr := cmd.Wait()
	out.durationMs = time.Since(start).Milliseconds()
	out.log = logBuf.String()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.reason = fmt.Sprintf("timed out: the harness was killed at its %v deadline", p.config.Timeout)
		return
	}
	result, readErr := rulestest.ReadResultFile(resultPath)
	if readErr == nil {
		if waitErr != nil {
			log.Printf("rules unit test run %d: the harness wrote a result and then exited with %v", run.Key, waitErr)
		}
		out.result = result
		return
	}
	out.reason = describeHarnessExit(waitErr, readErr, out.log, p.config.MemoryMB)
	return
}

// secretWords are the underscore-separated words that mark a variable as
// carrying a secret, or a way to fetch one: JETS_DSN_SECRET, JETS_DSN_JSON_VALUE,
// API_SECRET, JETS_ADMIN_PWD, AWS_SESSION_TOKEN, the ECS task role's
// AWS_CONTAINER_CREDENTIALS_RELATIVE_URI. A whole word, so OLDPWD is kept and
// PWD is not.
var secretWords = map[string]bool{
	"DSN": true, "SECRET": true, "SECRETS": true, "PASSWORD": true, "PASSWD": true, "PWD": true,
	"TOKEN": true, "CREDENTIALS": true,
}

// secretNames are the secret-bearing names no word gives away.
var secretNames = map[string]bool{
	"AWS_ACCESS_KEY_ID": true, "AWS_ROLE_ARN": true, "AWS_PROFILE": true,
}

// isSecretEnvName reports whether a variable must not reach the harness.
func isSecretEnvName(name string) bool {
	upper := strings.ToUpper(name)
	if secretNames[upper] || strings.Contains(upper, "ENCRYPTION_KEY") {
		return true
	}
	for _, w := range strings.Split(upper, "_") {
		if secretWords[w] {
			return true
		}
	}
	return false
}

// harnessEnviron is the harness's environment: the apiserver's, less every
// variable that carries a secret, with the run's workspace set (I-19).
//
// **The harness opens no database and calls no AWS service, so it needs none of
// them -- and it links jets/user**, through compute_pipes, jets/workspace and
// jets/datatable/git, whose init reads JETS_ENCRYPTION_KEY, API_SECRET,
// JETS_ENCRYPTION_KEY_SECRET and AWS_API_SECRET, and with the last two set calls
// AWS Secrets Manager at process start (jets/user/git_profile.go). Left in the
// environment, every test would fetch two secrets it never uses. What the
// harness does read -- WORKSPACES_HOME, WORKSPACE, LD_LIBRARY_PATH, GLOG_*, the
// non-secret JETS_* settings -- passes unchanged.
func harnessEnviron(base []string, workspacesHome, workspaceName string) []string {
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if isSecretEnvName(name) || name == "WORKSPACES_HOME" || name == "WORKSPACE" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "WORKSPACES_HOME="+workspacesHome, "WORKSPACE="+workspaceName)
}

// memoryLogMarkers are what a process says when an allocation fails under
// RLIMIT_DATA: the Go runtime, C++'s std::bad_alloc, and libc.
var memoryLogMarkers = []string{"out of memory", "bad_alloc", "Cannot allocate memory", "cannot allocate memory"}

// describeHarnessExit says why a run produced no result. **A memory limit is
// inferred, not observed**: RLIMIT_DATA makes an allocation fail rather than
// killing the process, so the process ends by a clean error, an abort or a Go
// fatal error, and only its log says why.
func describeHarnessExit(waitErr, readErr error, logText string, memoryMB int) string {
	how := "exit status 0"
	var exitErr *exec.ExitError
	switch {
	case errors.As(waitErr, &exitErr):
		how = exitErr.ProcessState.String() // "exit status 2", "signal: aborted (core dumped)"
	case waitErr != nil:
		how = waitErr.Error()
	}
	for _, m := range memoryLogMarkers {
		if strings.Contains(logText, m) {
			return fmt.Sprintf("memory limit, inferred from the log (%q) under the %d MiB data limit: the harness ended (%s) without writing a result",
				m, memoryMB, how)
		}
	}
	if errors.Is(readErr, os.ErrNotExist) {
		return fmt.Sprintf("the harness ended (%s) without writing a result; see the log", how)
	}
	return fmt.Sprintf("the harness ended (%s) and its result file could not be read: %v", how, readErr)
}

// recordRulesUnitTestOutcome writes the run row and its check rows in one
// transaction. The duration is the apiserver's measurement of the process, which
// exists whether or not a result was written.
func recordRulesUnitTestOutcome(dbpool *pgxpool.Pool, key int, out rulesUnitTestOutcome) error {
	if out.result == nil {
		return recordRulesUnitTestFailure(dbpool, key, out.reason, out.log, out.durationMs)
	}
	r := out.result
	reason := r.Reason
	if len(r.Findings) > 0 {
		b, _ := json.Marshal(r.Findings)
		reason = strings.TrimSpace(reason + " findings: " + string(b))
	}
	if len(r.EngineDifferences) > 0 {
		reason = strings.TrimSpace(reason + " engines differ: " + strings.Join(r.EngineDifferences, "; "))
	}
	ctx := context.Background()
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE jetsapi.rules_unit_test_execution SET
			status = $1, reason = $2, engine = $3, legacy_mode = coalesce($4, legacy_mode),
			load_ms = $5, run_ms = $6, duration_ms = $7, sessions = $8, loops = $9, log = $10,
			last_update = DEFAULT
		WHERE key = $11`,
		r.Status, nullIfEmpty(reason), nullIfEmpty(r.Engine), nullIfEmpty(r.LegacyMode),
		r.LoadMs, r.RunMs, out.durationMs, r.Sessions, r.Loops, pgText(out.log), key)
	if err != nil {
		return fmt.Errorf("while updating the run: %v", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM jetsapi.rules_unit_test_check WHERE execution_key = $1`, key); err != nil {
		return fmt.Errorf("while clearing the run's checks: %v", err)
	}
	for _, c := range r.Checks {
		_, err = tx.Exec(ctx, `INSERT INTO jetsapi.rules_unit_test_check
			(execution_key, session_name, kind, "check", outcome, found, engine)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			key, nullIfEmpty(c.SessionName), c.Kind, pgText(c.Check), c.Outcome, nullIfEmpty(pgText(c.Found)), nullIfEmpty(c.Engine))
		if err != nil {
			return fmt.Errorf("while recording a check: %v", err)
		}
	}
	return tx.Commit(ctx)
}

// recordRulesUnitTestFailure ends a run as an error with no result.
func recordRulesUnitTestFailure(dbpool *pgxpool.Pool, key int, reason, logText string, durationMs int64) error {
	_, err := dbpool.Exec(context.Background(), `UPDATE jetsapi.rules_unit_test_execution SET
			status = $1, reason = $2, log = $3, duration_ms = $4, last_update = DEFAULT
		WHERE key = $5`, RulesUnitTestError, reason, pgText(logText), durationMs, key)
	if err != nil {
		log.Printf("rules unit test run %d: while recording it as an error (%s): %v", key, reason, err)
	}
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// pgText makes s storable in a PostgreSQL text column, which refuses invalid
// UTF-8 and the NUL character -- both of which a crashing process can print.
func pgText(s string) string {
	s = strings.ToValidUTF8(s, "�")
	return strings.ReplaceAll(s, "\x00", "")
}

// tailBuffer keeps the last limit bytes written to it and counts the rest, so a
// run that logs a gigabyte costs the apiserver limit bytes of memory.
type tailBuffer struct {
	mu      sync.Mutex
	limit   int
	buf     []byte
	dropped int64
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.limit; over > 0 {
		b.dropped += int64(over)
		b.buf = append(b.buf[:0:0], b.buf[over:]...)
	}
	return n, nil
}

// String is the tail, opening with a line saying how much was cut when anything
// was. The cut is moved forward to the next line start, or at least to a
// character boundary, so the kept text does not begin mid-character.
func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.dropped == 0 {
		return string(b.buf)
	}
	kept := b.buf
	dropped := b.dropped
	if i := bytes.IndexByte(kept, '\n'); i >= 0 && i < 4096 {
		dropped += int64(i + 1)
		kept = kept[i+1:]
	} else {
		for len(kept) > 0 && !utf8.RuneStart(kept[0]) {
			kept = kept[1:]
			dropped++
		}
	}
	return fmt.Sprintf("[log truncated: the first %d bytes of %d were cut; the last %d are kept]\n%s",
		dropped, dropped+int64(len(kept)), len(kept), kept)
}
