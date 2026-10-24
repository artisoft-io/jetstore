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
//	  "updateLookupTables": true
//	}
//
// The function merges the partials exactly into
// jetstore/workspaces/<workspace>/jet_rules/optimizer_statistics.json and lists
// that key, so that RunReports syncs it into the workspace as an override and
// compiles the workspace, as it does for a lookup a report has written. The
// compiler then orders the rules by the statistics.

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
// ca.ProcessName and ca.SessionId, into the workspace's statistics file, and adds
// its key to updatedKeys. It fails when the session has no partial file, or when
// one does not merge (another format version, or another network).
func MergeOptimizerStatistics(_ *pgxpool.Pool, ca *CommandArguments, _, _ string, updatedKeys *[]string) error {
	if ca.ProcessName == "" || ca.SessionId == "" || ca.WorkspaceName == "" {
		return fmt.Errorf("%s: needs the process name, the session id and the workspace", MergeOptimizerStatisticsFunc)
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
	buf, err := optstats.Marshal(merged)
	if err != nil {
		return fmt.Errorf("%s: %v", MergeOptimizerStatisticsFunc, err)
	}
	key := optstats.WorkspaceKey(ca.WorkspaceName)
	if err := optimizerStatsS3.upload(key, buf); err != nil {
		return fmt.Errorf("%s: while writing %s: %v", MergeOptimizerStatisticsFunc, key, err)
	}
	log.Printf("%s: merged %d partial files into %s: %d rule sets", MergeOptimizerStatisticsFunc, len(files), key, len(merged.RuleSets))
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
