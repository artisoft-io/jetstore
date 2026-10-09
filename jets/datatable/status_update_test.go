package datatable

import "testing"

// A node running as an ecs task reports its error only in pipeline_execution_details,
// the state machine only gets the task stopped reason (CPED-180).
func TestMergeFailureDetails(t *testing.T) {
	ecsReason := "Essential container in task exited from family:JetStoreDEIDStackcpipesTaskDefinitionB56DEB92"
	nodeErr := "1791184366588 while merging files using s3 copy: operation error S3: CreateMultipartUpload, AccessDenied"
	tests := []struct {
		name           string
		nodeErrMessage string
		failureDetails string
		want           string
	}{
		{"ecs task failure", nodeErr, ecsReason, nodeErr + " (" + ecsReason + ")"},
		{"no node error", "", ecsReason, ecsReason},
		{"no failure details", nodeErr, "", nodeErr},
		{"lambda carries the node error", nodeErr, nodeErr, nodeErr},
		{"lambda error is part of the node errors", nodeErr + ",another error", nodeErr, nodeErr + ",another error"},
		{"failure details contains the node error", nodeErr, "cp_node: " + nodeErr, "cp_node: " + nodeErr},
		{"both empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeFailureDetails(tt.nodeErrMessage, tt.failureDetails); got != tt.want {
				t.Errorf("mergeFailureDetails() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The failure source says when the details are no longer only the decoded text.
func TestApplyNodeErrorMessage(t *testing.T) {
	ecsReason := "Essential container in task exited from family:JetStoreDEIDStackcpipesTaskDefinitionB56DEB92"
	nodeErr := "1791184366588 while merging files using s3 copy: AccessDenied"

	ca := &StatusUpdate{FailureDetails: ecsReason, FailureSource: FailureSourceEcsStoppedReason}
	ca.applyNodeErrorMessage(nodeErr)
	if ca.FailureDetails != nodeErr+" ("+ecsReason+")" || ca.FailureSource != FailureSourceNodeErrorMessage {
		t.Errorf("ecs task: got %q source %q", ca.FailureDetails, ca.FailureSource)
	}

	ca = &StatusUpdate{FailureDetails: nodeErr, FailureSource: FailureSourceLambdaErrorMessage}
	ca.applyNodeErrorMessage(nodeErr)
	if ca.FailureDetails != nodeErr || ca.FailureSource != FailureSourceLambdaErrorMessage {
		t.Errorf("lambda: got %q source %q", ca.FailureDetails, ca.FailureSource)
	}

	ca = &StatusUpdate{FailureDetails: ecsReason, FailureSource: FailureSourceEcsStoppedReason}
	ca.applyNodeErrorMessage("")
	if ca.FailureDetails != ecsReason || ca.FailureSource != FailureSourceEcsStoppedReason {
		t.Errorf("no node error: got %q source %q", ca.FailureDetails, ca.FailureSource)
	}
}
