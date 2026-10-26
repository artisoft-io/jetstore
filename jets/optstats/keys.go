package optstats

import (
	"fmt"
	"strings"
)

// Where the pipeline route keeps the statistics (rules_benchmark Phase 9, CC). A
// jetrules step with optimizer_statistics on writes one partial file per node
// under its session's stage folder, and the run_reports function
// merge_optimizer_statistics merges them into the process's own file in the
// workspace (workspace.go), which run_reports then syncs into the workspace as an
// override, applied at the workspace's next compile.

// WorkspacesS3Prefix is the S3 folder of the workspaces' overrides: a key
// <WorkspacesS3Prefix>/<workspace>/<file> is synced to <file> in the workspace
// (awsi.SyncS3Files).
const WorkspacesS3Prefix = "jetstore/workspaces"

// PartialsPrefix is the S3 prefix of a session's partial files, ending in /:
// <stage>/process_name=<process>/session_id=<session>/optimizer_statistics/, beside
// the folders of the session's steps.
func PartialsPrefix(stagePrefix, processName, sessionId string) string {
	return fmt.Sprintf("%s/process_name=%s/session_id=%s/optimizer_statistics/",
		strings.TrimSuffix(stagePrefix, "/"), processName, sessionId)
}

// PartialKey is the key of one node's partial file: the node's partition label,
// or none, and its node id, which together name a node of a step uniquely.
func PartialKey(stagePrefix, processName, sessionId, jetsPartition string, nodeId int) string {
	if jetsPartition == "" {
		jetsPartition = "none"
	}
	return fmt.Sprintf("%spartition=%s_node=%d.json", PartialsPrefix(stagePrefix, processName, sessionId),
		jetsPartition, nodeId)
}

// WorkspaceKey is the S3 key of a workspace's statistics file:
// jetstore/workspaces/<workspace>/jet_rules/optimizer_statistics.json.
func WorkspaceKey(workspaceName string) string {
	return WorkspacesS3Prefix + "/" + workspaceName + "/" + FileName
}

// WorkspaceProcessKey is the S3 key of a statistics process's file in a
// workspace: jetstore/workspaces/<workspace>/jet_rules/<process>.optimizer_statistics.json.
func WorkspaceProcessKey(workspaceName, processName string) (string, error) {
	name, err := ProcessFileName(processName)
	if err != nil {
		return "", err
	}
	return WorkspacesS3Prefix + "/" + workspaceName + "/" + name, nil
}
