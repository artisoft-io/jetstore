package main

// The Detection screen's two actions, driven through the fake store. What is
// asserted is the part of the behaviour that is this endpoint's: who is
// recorded as asking, which window reaches the task, that an undeployed task is
// a 409 and not an attempt, and that ECS is asked only about unfinished tasks.

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
	"github.com/artisoft-io/jetstore/jets/awsi"
)

type detectionFake struct {
	runs      []detectrun.Run
	target    awsi.AgenticDetectTarget
	deployed  bool
	cfgErr    error
	runErr    error
	taskArn   string
	states    map[string]awsi.AgenticDetectTaskState
	statesErr error

	// What the dispatch handed over.
	ranEnv      map[string]string
	ranCommand  []string
	requested   []string // taskArn, requestedBy, since
	describedOf []string
}

func (f *fakeOps) ListDetectionRuns(context.Context, int) ([]detectrun.Run, error) {
	return f.detection.runs, nil
}
func (f *fakeOps) RequestDetectionRun(_ context.Context, arn, by, since string) error {
	f.detection.requested = []string{arn, by, since}
	return nil
}
func (f *fakeOps) DetectionTarget() (awsi.AgenticDetectTarget, bool, error) {
	return f.detection.target, f.detection.deployed, f.detection.cfgErr
}
func (f *fakeOps) RunDetectionTask(_ context.Context, _ awsi.AgenticDetectTarget, env map[string]string, command []string) (string, error) {
	f.detection.ranEnv, f.detection.ranCommand = env, command
	return f.detection.taskArn, f.detection.runErr
}
func (f *fakeOps) DescribeDetectionTasks(_ context.Context, _ awsi.AgenticDetectTarget, arns []string) (map[string]awsi.AgenticDetectTaskState, error) {
	f.detection.describedOf = arns
	return f.detection.states, f.detection.statesErr
}

func TestRunDetectionStartsTheTaskAsTheUser(t *testing.T) {
	ops := &fakeOps{detection: detectionFake{deployed: true, taskArn: "arn:aws:ecs:r:1:task/c/t1"}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "run_detection"}, "kim@example.com")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	if got := res["taskArn"]; got != "arn:aws:ecs:r:1:task/c/t1" {
		t.Errorf("taskArn %v", got)
	}
	d := ops.detection
	if d.ranEnv[detectrun.TriggerVariable] != detectrun.TriggerManual ||
		d.ranEnv[detectrun.RequestedByVariable] != "kim@example.com" {
		t.Errorf("the task was not told who asked: %v", d.ranEnv)
	}
	if !slices.Equal(d.ranCommand, []string{"-since", "26h"}) {
		t.Errorf("an unset window must be the scheduled run's, got %v", d.ranCommand)
	}
	if !slices.Equal(d.requested, []string{"arn:aws:ecs:r:1:task/c/t1", "kim@example.com", "26h"}) {
		t.Errorf("the request was recorded as %v", d.requested)
	}
}

func TestRunDetectionWindow(t *testing.T) {
	for _, tc := range []struct {
		since string
		want  string
		ok    bool
	}{
		{"720h", "720h", true},
		{"168h", "168h", true},
		{"90m", "2h", true},
		{"30m", "", false},
		{"2161h", "", false},
		{"a week", "", false},
	} {
		ops := &fakeOps{detection: detectionFake{deployed: true, taskArn: "arn:x"}}
		_, code, err := dispatch(t, ops, &AgenticAction{Action: "run_detection", Since: tc.since}, "a@b")
		if tc.ok {
			if err != nil || !slices.Equal(ops.detection.ranCommand, []string{"-since", tc.want}) {
				t.Errorf("since %q: err %v command %v, want -since %s", tc.since, err, ops.detection.ranCommand, tc.want)
			}
			continue
		}
		if err == nil || code != http.StatusBadRequest || ops.detection.ranCommand != nil {
			t.Errorf("since %q: code %d err %v, and the task was started: %v", tc.since, code, err, ops.detection.ranCommand != nil)
		}
	}
}

func TestRunDetectionNotDeployedIsAConflictAndStartsNothing(t *testing.T) {
	ops := &fakeOps{}
	_, code, err := dispatch(t, ops, &AgenticAction{Action: "run_detection"}, "a@b")
	if err == nil || code != http.StatusConflict {
		t.Fatalf("code %d err %v, want 409", code, err)
	}
	if ops.detection.ranCommand != nil || ops.detection.requested != nil {
		t.Error("a task was started or recorded on a deployment without one")
	}
}

func TestRunDetectionEcsFailureRecordsNothing(t *testing.T) {
	ops := &fakeOps{detection: detectionFake{deployed: true, runErr: errors.New("no capacity")}}
	_, code, err := dispatch(t, ops, &AgenticAction{Action: "run_detection"}, "a@b")
	if err == nil || code != http.StatusBadGateway {
		t.Fatalf("code %d err %v, want 502", code, err)
	}
	if ops.detection.requested != nil {
		t.Error("a run that ECS refused was recorded as requested")
	}
}

func TestListDetectionRunsAsksEcsOnlyAboutUnfinishedTasks(t *testing.T) {
	now := time.Now()
	ops := &fakeOps{detection: detectionFake{
		deployed: true,
		target:   awsi.AgenticDetectTarget{ScheduleHourUtc: "8"},
		runs: []detectrun.Run{
			{TaskRef: "arn:t1", Command: detectrun.CommandAnomalyDetect, Status: detectrun.StatusRequested, RequestedAt: &now},
			{TaskRef: "arn:t1", Command: detectrun.CommandShadowTriage, Status: detectrun.StatusRequested, RequestedAt: &now},
			{TaskRef: "arn:t2", Command: detectrun.CommandAnomalyDetect, Status: detectrun.StatusOk, StartedAt: &now},
			{TaskRef: "local:h:1:2", Command: detectrun.CommandAnomalyDetect, Status: detectrun.StatusRunning},
		},
		states: map[string]awsi.AgenticDetectTaskState{"arn:t1": {LastStatus: "PROVISIONING"}},
	}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "list_detection_runs"}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	if !slices.Equal(ops.detection.describedOf, []string{"arn:t1"}) {
		t.Errorf("ECS was asked about %v, want only the unfinished ECS task, once", ops.detection.describedOf)
	}
	if res["deployed"] != true || res["scheduleHourUtc"] != "8" {
		t.Errorf("deployment not reported: %v %v", res["deployed"], res["scheduleHourUtc"])
	}
	if n := len(res["runs"].([]map[string]any)); n != 4 {
		t.Errorf("%d runs, want 4", n)
	}
}

func TestListDetectionRunsUndeployedAsksEcsNothing(t *testing.T) {
	ops := &fakeOps{detection: detectionFake{runs: []detectrun.Run{
		{TaskRef: "arn:t1", Command: detectrun.CommandAnomalyDetect, Status: detectrun.StatusRunning},
	}}}
	res, _, err := dispatch(t, ops, &AgenticAction{Action: "list_detection_runs"}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	if ops.detection.describedOf != nil || res["deployed"] != false {
		t.Errorf("an undeployed listing asked ECS about %v", ops.detection.describedOf)
	}
}
