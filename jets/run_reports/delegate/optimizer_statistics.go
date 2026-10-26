package delegate

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/jackc/pgx/v5"
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
// The function merges the partials exactly and writes
// jetstore/workspaces/<workspace>/jet_rules/optimizer_statistics.json, and lists
// that key, so that RunReports syncs it into the workspace and records it as an
// override, as it does for a lookup a report has written.
//
// **The session replaces only the rule sets it covers.** Every other rule set
// is kept from the workspace's current file, so that two processes, each with
// its own statistics process, accumulate one file in either order. The current
// file is the pending override (jetsapi.workspace_changes), else the deployed
// workspace's copy; the report step sees no compiled workspace, so a file
// committed after the deployment is not seen until the next one.
// optimizerStatisticsExclude names rule sets left out of the file, kept from
// neither the session nor the current file: they compile in the score order. The file takes effect
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

// optimizerStatsCurrent reads the workspace's current statistics file, nil when
// there is none; a test replaces it.
var optimizerStatsCurrent = struct {
	override func(dbpool *pgxpool.Pool, workspaceName string) ([]byte, error)
	deployed func(workspaceName string) ([]byte, error)
}{
	override: func(dbpool *pgxpool.Pool, workspaceName string) ([]byte, error) {
		if dbpool == nil {
			return nil, nil
		}
		var data []byte
		err := dbpool.QueryRow(context.Background(), `SELECT data FROM jetsapi.workspace_changes
			WHERE workspace_name = $1 AND file_name = $2`, workspaceName, optstats.FileName).Scan(&data)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return data, err
	},
	deployed: func(workspaceName string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(workspaceHome, workspaceName, optstats.FileName))
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return data, err
	},
}

// currentOptimizerStatistics is the workspace's statistics file as the next
// compile would read it, and where it was read from; nil when there is none.
func currentOptimizerStatistics(dbpool *pgxpool.Pool, workspaceName string) (*optstats.File, string, error) {
	for _, src := range []struct {
		name string
		read func() ([]byte, error)
	}{
		{"the pending override", func() ([]byte, error) { return optimizerStatsCurrent.override(dbpool, workspaceName) }},
		{"the deployed workspace", func() ([]byte, error) { return optimizerStatsCurrent.deployed(workspaceName) }},
	} {
		data, err := src.read()
		if err != nil {
			return nil, "", fmt.Errorf("while reading the current %s from %s: %v", optstats.FileName, src.name, err)
		}
		if data == nil {
			continue
		}
		f, err := optstats.Decode(strings.NewReader(string(data)))
		if err != nil {
			return nil, "", fmt.Errorf("the current %s, from %s: %v", optstats.FileName, src.name, err)
		}
		return f, src.name, nil
	}
	return nil, "", nil
}

// MergeOptimizerStatistics merges the partial statistics of the report's session,
// ca.ProcessName and ca.SessionId, writes them with the current file's other rule
// sets as the workspace's statistics file, and adds its key to updatedKeys. It
// fails when the session has no partial file, when one does not merge (another
// format version, or another network), or when the current file cannot be read.
func MergeOptimizerStatistics(dbpool *pgxpool.Pool, ca *CommandArguments, _, _ string, updatedKeys *[]string) error {
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
	current, source, err := currentOptimizerStatistics(dbpool, ca.WorkspaceName)
	if err != nil {
		return fmt.Errorf("%s: %v", MergeOptimizerStatisticsFunc, err)
	}
	exclude := map[string]bool{}
	if ca.CurrentReportDirectives != nil {
		for _, name := range ca.CurrentReportDirectives.OptimizerStatisticsExclude {
			exclude[name] = true
		}
	}
	out := optstats.New()
	var kept, excluded []string
	if current != nil {
		for name, rs := range current.RuleSets {
			_, replaced := merged.RuleSets[name]
			switch {
			case exclude[name]:
				excluded = append(excluded, name)
			case !replaced:
				out.RuleSets[name] = rs
				kept = append(kept, name)
			}
		}
	}
	for name, rs := range merged.RuleSets {
		if exclude[name] {
			excluded = append(excluded, name)
			continue
		}
		out.RuleSets[name] = rs
	}
	if len(out.RuleSets) == 0 {
		return fmt.Errorf("%s: every rule set of the session is in optimizerStatisticsExclude and no other is kept: nothing to write",
			MergeOptimizerStatisticsFunc)
	}
	sort.Strings(kept)
	sort.Strings(excluded)
	if current != nil {
		log.Printf("%s: %d rule sets from the session, %d kept from %s %v, %d excluded %v",
			MergeOptimizerStatisticsFunc, len(merged.RuleSets), len(kept), source, kept, len(excluded), excluded)
	}
	buf, err := optstats.Marshal(out)
	if err != nil {
		return fmt.Errorf("%s: %v", MergeOptimizerStatisticsFunc, err)
	}
	key := optstats.WorkspaceKey(ca.WorkspaceName)
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
