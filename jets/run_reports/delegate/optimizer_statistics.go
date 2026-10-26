package delegate

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The rule optimizer's statistics, the pipeline route (rules_benchmark Phase 9,
// CC). A statistics process is a pipeline whose jetrules step sets
// optimizer_statistics: each node writes a partial file under the session's
// stage folder (optstats.PartialsPrefix). Its report, in the workspace's
// reports/config.json, names this function in reportScripts and sets
// updateLookupTables:
//
//	"<process name>": {
//	  "reportScripts": ["merge_optimizer_statistics"],
//	  "updateLookupTables": true,
//	  "optimizerStatisticsExclude": ["jet_rules/main/x.jr"]
//	}
//
// The function merges the partials exactly and writes the process's own file,
// jetstore/workspaces/<workspace>/jet_rules/<process>.optimizer_statistics.json,
// and lists that key, so that RunReports syncs it into the workspace and records
// it as an override, as it does for a lookup a report has written.
//
// **Each statistics process owns its file, and reads no other** (optstats,
// workspace.go). Two rule processes, each with its own statistics process, keep
// their statistics apart: a run replaces its process's file and nothing else, so
// no run can drop another's rule sets, whatever the order or timing, and a
// commit or revert of a file is an ordinary workspace edit. The compiler reads
// every file. optimizerStatisticsExclude names rule sets left out of the
// process's file: they compile in the score order unless another file has them. The file takes effect
// at the workspace's next compile: RunReports compiles unless
// SkipCompileWorkspace is set, which the stock cpipes route's run_reports Lambda
// does, and otherwise a compile from the UI or a later deployment's start-up
// compile applies the override. The compiler then orders the rules by the
// statistics.

// MergeOptimizerStatisticsFunc is the function's name in reportScripts.
const MergeOptimizerStatisticsFunc = "merge_optimizer_statistics"

// builtinReportFuncs are the report functions every JetStore has, beside a
// deployment's own (CommandArguments.InstallSpecificFunc), which take precedence.
var builtinReportFuncs = map[string]InstallationSpecificReportFunc{
	MergeOptimizerStatisticsFunc: MergeOptimizerStatistics,
}

// reportFunc is the function a report script names: the deployment's own, else
// a built-in one, else nil.
func (ca *CommandArguments) reportFunc(reportScript string) InstallationSpecificReportFunc {
	if f := ca.InstallSpecificFunc[reportScript]; f != nil {
		return f
	}
	return builtinReportFuncs[reportScript]
}

// optimizerStatsS3 is the S3 the function reads and writes, in the jetstore
// bucket; a test replaces it.
var optimizerStatsS3 = struct {
	list     func(prefix string) ([]string, error)
	download func(key string) ([]byte, error)
	upload   func(key string, buf []byte) error
}{
	list: func(prefix string) ([]string, error) {
		objs, err := awsi.ListS3Objects("", &prefix)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(objs))
		for _, o := range objs {
			keys = append(keys, o.Key)
		}
		return keys, nil
	},
	download: awsi.DownloadBufFromS3,
	upload:   func(key string, buf []byte) error { return awsi.UploadBufToS3("", key, buf) },
}

// MergeOptimizerStatistics merges the partial statistics of the report's session,
// ca.ProcessName and ca.SessionId, writes them, less the excluded rule sets, as
// the process's statistics file in the workspace, and adds its key to
// updatedKeys. It fails when the process name cannot name a file, when the
// session has no partial file, when one does not merge (another format version,
// or another network), or when every rule set is excluded.
func MergeOptimizerStatistics(_ *pgxpool.Pool, ca *CommandArguments, _, _ string, updatedKeys *[]string) error {
	if ca.ProcessName == "" || ca.SessionId == "" || ca.WorkspaceName == "" {
		return fmt.Errorf("%s: needs the process name, the session id and the workspace", MergeOptimizerStatisticsFunc)
	}
	key, err := optstats.WorkspaceProcessKey(ca.WorkspaceName, ca.ProcessName)
	if err != nil {
		return fmt.Errorf("%s: %v", MergeOptimizerStatisticsFunc, err)
	}
	prefix := optstats.PartialsPrefix(awsi.JetStoreStagePrefix(), ca.ProcessName, ca.SessionId)
	keys, err := optimizerStatsS3.list(prefix)
	if err != nil {
		return fmt.Errorf("%s: while listing %s: %v", MergeOptimizerStatisticsFunc, prefix, err)
	}
	sort.Strings(keys)
	var files []*optstats.File
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}
		buf, err := optimizerStatsS3.download(key)
		if err != nil {
			return fmt.Errorf("%s: while reading %s: %v", MergeOptimizerStatisticsFunc, key, err)
		}
		f, err := optstats.Decode(strings.NewReader(string(buf)))
		if err != nil {
			return fmt.Errorf("%s: %s: %v", MergeOptimizerStatisticsFunc, key, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return fmt.Errorf("%s: no partial statistics under %s: does the process's jetrules step set optimizer_statistics?",
			MergeOptimizerStatisticsFunc, prefix)
	}
	merged, err := optstats.Merge(files...)
	if err != nil {
		return fmt.Errorf("%s: %v", MergeOptimizerStatisticsFunc, err)
	}
	if len(merged.RuleSets) == 0 {
		return fmt.Errorf("%s: the %d partial files under %s hold no session", MergeOptimizerStatisticsFunc, len(files), prefix)
	}
	exclude := map[string]bool{}
	if ca.CurrentReportDirectives != nil {
		for _, name := range ca.CurrentReportDirectives.OptimizerStatisticsExclude {
			exclude[name] = true
		}
	}
	out := optstats.New()
	var excluded, unmatched []string
	for name, rs := range merged.RuleSets {
		if exclude[name] {
			excluded = append(excluded, name)
			continue
		}
		out.RuleSets[name] = rs
	}
	for name := range exclude {
		if _, ok := merged.RuleSets[name]; !ok {
			unmatched = append(unmatched, name)
		}
	}
	sort.Strings(excluded)
	sort.Strings(unmatched)
	if len(out.RuleSets) == 0 {
		return fmt.Errorf("%s: every rule set of the session is in optimizerStatisticsExclude: nothing to write",
			MergeOptimizerStatisticsFunc)
	}
	log.Printf("%s: %d rule sets from the session, %d excluded %v", MergeOptimizerStatisticsFunc,
		len(merged.RuleSets), len(excluded), excluded)
	if len(unmatched) > 0 {
		log.Printf("%s: WARNING optimizerStatisticsExclude names %d rule sets the session does not have: %v",
			MergeOptimizerStatisticsFunc, len(unmatched), unmatched)
	}
	buf, err := optstats.Marshal(out)
	if err != nil {
		return fmt.Errorf("%s: %v", MergeOptimizerStatisticsFunc, err)
	}
	if err := optimizerStatsS3.upload(key, buf); err != nil {
		return fmt.Errorf("%s: while writing %s: %v", MergeOptimizerStatisticsFunc, key, err)
	}
	log.Printf("%s: merged %d partial files into %s: %d rule sets", MergeOptimizerStatisticsFunc, len(files), key, len(out.RuleSets))
	*updatedKeys = append(*updatedKeys, key)
	return nil
}

// workspaceOverride is how RunReports syncs an updated key into the workspace:
// the prefix trimmed from the key to give the workspace file's name, and the
// content type its override row is labelled with. A key under the workspace's
// own S3 folder, jetstore/workspaces/<workspace>/, is named from there and
// labelled by its top directory, as the IDE labels a file it saves (lookups,
// jet_rules, ...). Any other key is named from the report's output path and
// labelled lookups, as every key was before.
func workspaceOverride(key, workspaceName, outputPath string) (trimPrefix, contentType string) {
	wsPrefix := optstats.WorkspacesS3Prefix + "/" + workspaceName + "/"
	if workspaceName == "" || !strings.HasPrefix(key, wsPrefix) {
		return outputPath + "/", "lookups"
	}
	name := strings.TrimPrefix(key, wsPrefix)
	contentType = "lookups"
	if i := strings.Index(name, "/"); i > 0 {
		contentType = name[:i]
	}
	return wsPrefix, contentType
}
