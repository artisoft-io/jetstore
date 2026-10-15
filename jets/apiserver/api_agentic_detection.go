package main

// The Detection screen's two actions on the agentic endpoint (agentic_ai I-861):
// list the recorded runs of the detection commands, and start the deployed
// detection task now rather than at its scheduled hour.
//
// **Starting the task acts on nothing a person supervises.** The task runs the
// detectors and shadow-mode triage, which write anomalies and incidents and stop
// at `diagnosed`; it is what the daily rule already does unattended. So it sits
// behind the same agent_supervision capability as the rest of this endpoint, and
// the request is in the audit log with the user's email like every action here.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
	"github.com/artisoft-io/jetstore/jets/awsi"
)

// detectionDefaultSince is the window a manual run judges when none is asked
// for: the scheduled run's, so a manual run is the scheduled one early.
const detectionDefaultSince = "26h"

// detectionMaxSince bounds a manual window. A first run over a new deployment
// wants history, and ninety days is past the six months the run header is kept
// only for the step-regression baseline, which reaches a further 30 days back
// on its own.
const detectionMaxSince = 90 * 24 * time.Hour

// detectionStateLimit bounds how many unfinished tasks one listing asks ECS
// about.
const detectionStateLimit = 20

// detectionSince validates a requested window and writes it the way the
// commands' -since flag reads it.
func detectionSince(v string) (string, error) {
	if v == "" {
		return detectionDefaultSince, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return "", fmt.Errorf("the window %q is not a duration such as 26h or 720h", v)
	}
	if d < time.Hour || d > detectionMaxSince {
		return "", fmt.Errorf("the window %q is outside 1h to %s", v, detectionMaxSince)
	}
	return fmt.Sprintf("%dh", int(d.Round(time.Hour)/time.Hour)), nil
}

func listDetectionRuns(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	target, deployed, cfgErr := ops.DetectionTarget()
	runs, err := ops.ListDetectionRuns(ctx, a.Limit)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	out := make([]map[string]any, 0, len(runs))
	unfinished := []string{}
	seen := map[string]bool{}
	for _, r := range runs {
		out = append(out, map[string]any{
			"key":          r.Key,
			"taskRef":      r.TaskRef,
			"command":      r.Command,
			"trigger":      r.Trigger,
			"requestedBy":  r.RequestedBy,
			"status":       r.Status,
			"since":        r.Since,
			"requestedAt":  timePtrOrEmpty(r.RequestedAt),
			"startedAt":    timePtrOrEmpty(r.StartedAt),
			"finishedAt":   timePtrOrEmpty(r.FinishedAt),
			"summary":      r.Summary,
			"errorMessage": r.ErrorMessage,
		})
		// ECS is asked only about tasks a row says are not over: those are the
		// runs where the record cannot say what happened, because the command
		// never started or never got to write its end -- an image that will not
		// pull, a container killed for memory.
		if strings.HasPrefix(r.TaskRef, "arn:") && !seen[r.TaskRef] &&
			(r.Status == detectrun.StatusRequested || r.Status == detectrun.StatusRunning) &&
			len(unfinished) < detectionStateLimit {
			seen[r.TaskRef] = true
			unfinished = append(unfinished, r.TaskRef)
		}
	}
	result := map[string]any{
		"deployed":        deployed,
		"scheduleHourUtc": target.ScheduleHourUtc,
		"defaultSince":    detectionDefaultSince,
		"runs":            out,
		"taskStates":      map[string]awsi.AgenticDetectTaskState{},
	}
	if cfgErr != nil {
		result["deploymentError"] = cfgErr.Error()
	}
	if deployed && len(unfinished) > 0 {
		states, err := ops.DescribeDetectionTasks(ctx, target, unfinished)
		if err != nil {
			// The rows are still worth showing; say why the states are not.
			result["taskStatesError"] = err.Error()
		} else {
			result["taskStates"] = states
		}
	}
	return &result, http.StatusOK, nil
}

func runDetection(ctx context.Context, ops agenticOps, a *AgenticAction, actor string) (*map[string]any, int, error) {
	since, err := detectionSince(a.Since)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	target, deployed, cfgErr := ops.DetectionTarget()
	if cfgErr != nil {
		return nil, http.StatusInternalServerError, cfgErr
	}
	if !deployed {
		return nil, http.StatusConflict, fmt.Errorf(
			"the detection task is not part of this deployment: the stack builds it only when %s is set",
			awsi.AgenticDetectHourVariable)
	}
	env := map[string]string{
		detectrun.TriggerVariable:     detectrun.TriggerManual,
		detectrun.RequestedByVariable: actor,
	}
	arn, err := ops.RunDetectionTask(ctx, target, env, []string{"-since", since})
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	// Recorded after the start and not before, because the row is keyed on the
	// ARN ECS returns. A failure here leaves a task that will record itself when
	// it starts, as a scheduled one does, so it is reported and not undone.
	if err := ops.RequestDetectionRun(ctx, arn, actor, since); err != nil {
		return &map[string]any{"taskArn": arn, "since": since, "recordError": err.Error()}, http.StatusOK, nil
	}
	return &map[string]any{"taskArn": arn, "since": since}, http.StatusOK, nil
}

func timePtrOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return timeOrEmpty(*t)
}

// The production implementations, on pgOps for the dispatch's single seam.

func (o pgOps) ListDetectionRuns(ctx context.Context, limit int) ([]detectrun.Run, error) {
	return detectrun.List(ctx, o.db, limit)
}
func (o pgOps) RequestDetectionRun(ctx context.Context, taskArn, requestedBy, since string) error {
	return detectrun.Request(ctx, o.db, taskArn, requestedBy, since)
}
func (o pgOps) DetectionTarget() (awsi.AgenticDetectTarget, bool, error) {
	return awsi.AgenticDetectTargetFromEnv()
}
func (o pgOps) RunDetectionTask(ctx context.Context, t awsi.AgenticDetectTarget, env map[string]string, command []string) (string, error) {
	return awsi.RunAgenticDetectTask(ctx, t, env, command)
}
func (o pgOps) DescribeDetectionTasks(ctx context.Context, t awsi.AgenticDetectTarget, arns []string) (map[string]awsi.AgenticDetectTaskState, error) {
	return awsi.DescribeAgenticDetectTasks(ctx, t, arns)
}
