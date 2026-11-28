package delegate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// A report command's buckets reach awsi.MultiPartCopy only as names it can
// address: a placeholder nothing substituted is refused, naming the command,
// and the deployment's own bucket is resolved before the copy.
func TestCopyBuckets(t *testing.T) {
	const own = "deployment-bucket"
	cmd := func(src, dest, comment string) *compute_pipes.ReportCmdSpec {
		return &compute_pipes.ReportCmdSpec{
			Type:    "s3_copy_file",
			Comment: comment,
			S3CopyFileConfig: &compute_pipes.S3CopyFileSpec{
				SourceBucket: src, SourceKey: "in/a.csv",
				DestinationBucket: dest, DestinationKey: "out/a.csv",
			},
		}
	}
	for _, tc := range []struct {
		name              string
		cmd               *compute_pipes.ReportCmdSpec
		own               string
		wantSrc, wantDest string
		wantErr           []string // substrings of the error, nil when none
	}{
		{"both external", cmd("src-bucket", "dest-bucket", ""), own, "src-bucket", "dest-bucket", nil},
		{"empty is the own bucket", cmd("", "dest-bucket", ""), own, own, "dest-bucket", nil},
		{"sentinel is the own bucket", cmd("src-bucket", "jetstore_bucket", ""), own, "src-bucket", own, nil},
		{"sentinel with spaces is the own bucket", cmd(" jetstore_bucket ", "dest-bucket", ""), own, own, "dest-bucket", nil},
		{"external with spaces is trimmed", cmd(" src-bucket", "dest-bucket ", ""), own, "src-bucket", "dest-bucket", nil},
		{"braced placeholder in src", cmd("${OUT_BUCKET}", "dest-bucket", ""), own, "", "",
			[]string{"report_cmds[3] (s3_copy_file)", "src_bucket", "${OUT_BUCKET}", "unresolved"}},
		{"bare placeholder in dest", cmd("src-bucket", "out-$CLIENT", "copy the extract"), own, "", "",
			[]string{`report_cmds[3] (s3_copy_file, "copy the extract")`, "dest_bucket", "out-$CLIENT", "unresolved"}},
		{"own bucket not set", cmd("jetstore_bucket", "dest-bucket", ""), "", "", "",
			[]string{"report_cmds[3] (s3_copy_file)", "src_bucket", "JETS_BUCKET is not set"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := *tc.cmd.S3CopyFileConfig
			got, err := copyBuckets(3, tc.cmd, tc.own)
			if *tc.cmd.S3CopyFileConfig != before {
				t.Errorf("copyBuckets changed the command: %+v", *tc.cmd.S3CopyFileConfig)
			}
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("copyBuckets = %+v, want an error", got)
				}
				for _, w := range tc.wantErr {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("error %q does not name %q", err, w)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("copyBuckets: %v", err)
			}
			if got.SourceBucket != tc.wantSrc || got.DestinationBucket != tc.wantDest {
				t.Errorf("buckets = %q, %q, want %q, %q", got.SourceBucket, got.DestinationBucket, tc.wantSrc, tc.wantDest)
			}
			if got.SourceKey != before.SourceKey || got.DestinationKey != before.DestinationKey {
				t.Errorf("keys changed: %q, %q", got.SourceKey, got.DestinationKey)
			}
		})
	}
}

// reportCmdsJson is a schema provider as input_registry.schema_provider_json
// holds it: an env, and report_cmds whose when (when given) compares the env
// variable $COPY_TARGET with a literal.
func reportCmdsJson(t *testing.T, target string, cmds ...string) string {
	t.Helper()
	sp := fmt.Sprintf(`{"env": {"$COPY_TARGET": %q}, "report_cmds": [%s]}`, target, strings.Join(cmds, ","))
	if !json.Valid([]byte(sp)) {
		t.Fatalf("invalid schema provider json: %s", sp)
	}
	return sp
}

// copyCmd is one s3_copy_file report command copying key to key, run only when
// $COPY_TARGET is target, or always when target is empty.
func copyCmd(key, target string) string {
	when := ""
	if target != "" {
		when = fmt.Sprintf(`, "when": {"lhs": {"type": "select", "expr": "$COPY_TARGET"}, "op": "==",
			"rhs": {"type": "value", "expr": "'%s'"}}`, target)
	}
	return fmt.Sprintf(`{"type": "s3_copy_file", "s3_copy_file_config": {"src_bucket": "src-bucket",
		"src_key": %q, "dest_bucket": "dest-bucket", "dest_key": %q}%s}`, key, key, when)
}

func planOf(t *testing.T, sp string) (reportCmdsPlan, error) {
	t.Helper()
	var short struct {
		Env        map[string]any                `json:"env"`
		ReportCmds []compute_pipes.ReportCmdSpec `json:"report_cmds"`
	}
	if err := json.Unmarshal([]byte(sp), &short); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return planReportCmds(short.Env, short.ReportCmds, "deployment-bucket")
}

func taskKeys(plan reportCmdsPlan) []string {
	keys := make([]string, 0, len(plan.tasks))
	for _, task := range plan.tasks {
		keys = append(keys, task.SourceKey)
	}
	return keys
}

// A report command's when decides whether that command runs, not whether any
// command runs: with one command whose when holds and one whose when does not,
// only the first is dispatched; a command with no when always is.
func TestPlanReportCmdsWhenSelectsEachCommand(t *testing.T) {
	sp := reportCmdsJson(t, "outbound",
		copyCmd("a.csv", "outbound"),
		copyCmd("b.csv", "inbound"),
		copyCmd("c.csv", ""))
	plan, err := planOf(t, sp)
	if err != nil {
		t.Fatalf("planReportCmds: %v", err)
	}
	if got := strings.Join(taskKeys(plan), ","); got != "a.csv,c.csv" {
		t.Errorf("dispatched %q, want a.csv,c.csv", got)
	}
	for _, task := range plan.tasks {
		if task.SourceBucket != "src-bucket" || task.DestinationBucket != "dest-bucket" {
			t.Errorf("task %s buckets = %q, %q", task.SourceKey, task.SourceBucket, task.DestinationBucket)
		}
	}

	// The same commands with the env selecting the other one.
	plan, err = planOf(t, reportCmdsJson(t, "inbound",
		copyCmd("a.csv", "outbound"),
		copyCmd("b.csv", "inbound"),
		copyCmd("c.csv", "")))
	if err != nil {
		t.Fatalf("planReportCmds: %v", err)
	}
	if got := strings.Join(taskKeys(plan), ","); got != "b.csv,c.csv" {
		t.Errorf("dispatched %q, want b.csv,c.csv", got)
	}
}

// When no command's when holds, nothing is dispatched and nothing is an error.
func TestPlanReportCmdsAllWhenFalse(t *testing.T) {
	plan, err := planOf(t, reportCmdsJson(t, "none",
		copyCmd("a.csv", "outbound"),
		copyCmd("b.csv", "inbound")))
	if err != nil {
		t.Fatalf("planReportCmds: %v", err)
	}
	if len(plan.tasks) != 0 || plan.workerPoolSize != 0 {
		t.Errorf("plan = %+v, want no task and no pool", plan)
	}

	// RunSchemaProviderReportsCmds then returns before any S3 client or worker
	// pool, closing errCh with nothing sent, as it does when there are no
	// report_cmds at all.
	for _, sp := range []string{
		reportCmdsJson(t, "none", copyCmd("a.csv", "outbound"), copyCmd("b.csv", "inbound")),
		reportCmdsJson(t, "none"),
	} {
		ca := &CommandArguments{SessionId: "1", SchemaProviderJson: sp}
		errCh := make(chan error, 4)
		ca.RunSchemaProviderReportsCmds(context.Background(), nil, errCh)
		for err := range errCh {
			t.Errorf("RunSchemaProviderReportsCmds sent %v, want nothing", err)
		}
	}
}

// The worker pool is sized from the commands that run, not from report_cmds,
// and so is the default worker_pool_size of each copy.
func TestPlanReportCmdsPoolSize(t *testing.T) {
	for _, tc := range []struct {
		name            string
		run, skip       int
		wantPool        int
		wantCopyWorkers int
	}{
		{"one of four runs", 1, 3, 1, 40},
		{"two of four run", 2, 2, 2, 20},
		{"all four run", 4, 0, 4, 10},
		{"twelve of fifteen run, pool capped", 12, 3, 10, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmds := make([]string, 0, tc.run+tc.skip)
			for i := range tc.skip {
				cmds = append(cmds, copyCmd(fmt.Sprintf("skip%d.csv", i), "inbound"))
			}
			for i := range tc.run {
				cmds = append(cmds, copyCmd(fmt.Sprintf("run%d.csv", i), "outbound"))
			}
			plan, err := planOf(t, reportCmdsJson(t, "outbound", cmds...))
			if err != nil {
				t.Fatalf("planReportCmds: %v", err)
			}
			if len(plan.tasks) != tc.run {
				t.Errorf("dispatched %d tasks, want %d", len(plan.tasks), tc.run)
			}
			if plan.workerPoolSize != tc.wantPool {
				t.Errorf("workerPoolSize = %d, want %d", plan.workerPoolSize, tc.wantPool)
			}
			for _, task := range plan.tasks {
				if !strings.HasPrefix(task.SourceKey, "run") {
					t.Errorf("dispatched %s, whose when does not hold", task.SourceKey)
				}
				if task.WorkerPoolSize != tc.wantCopyWorkers {
					t.Errorf("task %s worker_pool_size = %d, want %d", task.SourceKey, task.WorkerPoolSize, tc.wantCopyWorkers)
				}
			}
		})
	}

	// A worker_pool_size the command gives is kept.
	sp := reportCmdsJson(t, "outbound", `{"type": "s3_copy_file", "s3_copy_file_config": {"src_bucket": "s",
		"src_key": "k", "dest_bucket": "d", "dest_key": "k", "worker_pool_size": 3}}`)
	plan, err := planOf(t, sp)
	if err != nil {
		t.Fatalf("planReportCmds: %v", err)
	}
	if len(plan.tasks) != 1 || plan.tasks[0].WorkerPoolSize != 3 {
		t.Errorf("plan = %+v, want one task with worker_pool_size 3", plan)
	}
}

// Only the commands that run are checked: a command whose when does not hold
// is not refused for what it would have done, and one whose when holds is,
// before anything is dispatched. A when that cannot be built stops the run.
func TestPlanReportCmdsChecksOnlySelected(t *testing.T) {
	unresolved := `{"type": "s3_copy_file", "s3_copy_file_config": {"src_bucket": "${OUT_BUCKET}",
		"src_key": "x.csv", "dest_bucket": "d", "dest_key": "x.csv"}, "when": {"lhs": {"type": "select",
		"expr": "$COPY_TARGET"}, "op": "==", "rhs": {"type": "value", "expr": "'inbound'"}}}`
	unknown := `{"type": "no_such_command", "when": {"lhs": {"type": "select", "expr": "$COPY_TARGET"},
		"op": "==", "rhs": {"type": "value", "expr": "'inbound'"}}}`
	missing := `{"type": "s3_copy_file", "when": {"lhs": {"type": "select", "expr": "$COPY_TARGET"},
		"op": "==", "rhs": {"type": "value", "expr": "'inbound'"}}}`
	badWhen := `{"type": "s3_copy_file", "s3_copy_file_config": {"src_bucket": "s", "src_key": "k",
		"dest_bucket": "d", "dest_key": "k"}, "when": {"op": "=="}}`

	plan, err := planOf(t, reportCmdsJson(t, "outbound", copyCmd("a.csv", "outbound"), unresolved, unknown, missing))
	if err != nil {
		t.Fatalf("planReportCmds refused a command whose when does not hold: %v", err)
	}
	if got := strings.Join(taskKeys(plan), ","); got != "a.csv" {
		t.Errorf("dispatched %q, want a.csv", got)
	}

	for _, tc := range []struct {
		name    string
		cmd     string
		wantErr string
	}{
		{"unresolved bucket", unresolved, "report_cmds[1] (s3_copy_file)"},
		{"unknown type", unknown, "unknown report command type: no_such_command"},
		{"missing config", missing, "missing s3_copy_file_config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planOf(t, reportCmdsJson(t, "inbound", copyCmd("a.csv", ""), tc.cmd))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("planReportCmds = %+v, %v, want an error naming %q", plan, err, tc.wantErr)
			}
		})
	}

	if _, err := planOf(t, reportCmdsJson(t, "outbound", copyCmd("a.csv", "outbound"), badWhen)); err == nil ||
		!strings.Contains(err.Error(), "while building evaluator for report command 1") {
		t.Errorf("planReportCmds with an unbuildable when: %v, want the build error", err)
	}
}
