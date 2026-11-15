// Package directives holds a workspace's report directives, the content of its
// `reports/config.json`, and the checks made of them before any report runs.
//
// It is its own package so that the two places that read the file share one
// decoding and one check: run_reports, which follows the directives
// (jets/run_reports/delegate), and the workspace IDE's save path, which refuses
// a file run_reports would refuse (jets/datatable). The delegate imports
// jets/datatable, so the save path cannot import the delegate; both import
// this package, which imports nothing of JetStore but jets/sqlscript, the
// reader of a reports file (report_files.go), which imports nothing that
// imports this package.
package directives

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ConfigFile is the workspace-relative path of the report directives.
const ConfigFile = "reports/config.json"

type StringSubstitution struct {
	Replace string `json:"replace"`
	With    string `json:"with"`
}

type SentinelConfig struct {
	FilePathSubstitution []StringSubstitution `json:"filePathSubstitution"`
}

type ReportDirectives struct {
	// InputPath is original fileKey, unless overriten in config file, used to emit sentinel file
	FilePathSubstitution []StringSubstitution         `json:"filePathSubstitution"`
	ReportScripts        []string                     `json:"reportScripts"`
	UpdateLookupTables   bool                         `json:"updateLookupTables"`
	EmitSentinelFile     *SentinelConfig              `json:"emitSentinelFile"`
	OutputS3Prefix       string                       `json:"outputS3Prefix"`
	InputPath            string                       `json:"inputPath"`
	OutputPath           string                       `json:"outputPath"`
	ReportProperties     map[string]ReportProperty    `json:"reportProperties"`
	StatementProperties  map[string]StatementProperty `json:"statementProperties"`
	RegisterReports      []RegisterReportSpec         `json:"registerReport"`
	UseQueryExportToS3   bool                         `json:"use_query_export_to_s3"`
	// OptimizerStatisticsExclude names the rule sets merge_optimizer_statistics
	// leaves out of the workspace's statistics file, so that they compile in the
	// score order (jets/run_reports/delegate/optimizer_statistics.go).
	OptimizerStatisticsExclude []string `json:"optimizerStatisticsExclude"`
}

// Type: report, script, function
// case function, use a deployment-specific function
type ReportProperty struct {
	Type            string            `json:"reportOrScript"`
	UpdatedFileKeys []string          `json:"updatedFileKeys"`
	RunWhen         []RunWhenCriteria `json:"runWhen"`
}

type StatementProperty struct {
	OutputFormat string            `json:"outputFormat"`
	RunWhen      []RunWhenCriteria `json:"runWhen"`
}

type RunWhenCriteria struct {
	FileKeyComponent        string `json:"fileKeyComponent"`
	HasValue                string `json:"hasValue"`
	HasNotValue             string `json:"hasNotValue"`
	HasNonZeroOutputRecords bool   `json:"hasOutputRecordsOnly"`
}

type RegisterReportSpec struct {
	TableName  string `json:"table_name"`
	Org        string `json:"org"`
	ObjectType string `json:"object_type"`
	SourceType string `json:"source_type"`
}

// Decode reads a `reports/config.json`: report names (a process name, or the
// -reportName argument) to their directives. It is how run_reports reads the
// file, and an error here stops run_reports before any report runs.
func Decode(content []byte) (map[string]ReportDirectives, error) {
	config := map[string]ReportDirectives{}
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// Check refuses a criterion that checks nothing, or that gives a value with
// no component to compare it to.
func (c *RunWhenCriteria) Check() error {
	if c.FileKeyComponent != "" {
		return nil
	}
	switch {
	case len(c.HasValue) > 0 || len(c.HasNotValue) > 0:
		return fmt.Errorf("a runWhen criterion has hasValue or hasNotValue and no fileKeyComponent")
	case !c.HasNonZeroOutputRecords:
		return fmt.Errorf("a runWhen criterion has no fileKeyComponent and no hasOutputRecordsOnly, so it checks nothing")
	}
	return nil
}

// RunWhenError is a runWhen criterion that Check refuses, and where it is:
// Section is reportProperties or statementProperties, Entry the file or
// statement name it is keyed by, and Index its place in the runWhen list.
type RunWhenError struct {
	Section string
	Entry   string
	Index   int
	Err     error
}

func (e *RunWhenError) Error() string {
	return fmt.Sprintf("%s, %s %q, runWhen[%d]: %v", ConfigFile, e.Section, e.Entry, e.Index, e.Err)
}

func (e *RunWhenError) Unwrap() error { return e.Err }

// RunWhenErrors is every runWhen criterion of the directives that Check
// refuses, in reportProperties and then statementProperties, each in name
// order.
func (rd *ReportDirectives) RunWhenErrors() []*RunWhenError {
	if rd == nil {
		return nil
	}
	var errs []*RunWhenError
	collect := func(section, entry string, criteria []RunWhenCriteria) {
		for i := range criteria {
			if err := criteria[i].Check(); err != nil {
				errs = append(errs, &RunWhenError{Section: section, Entry: entry, Index: i, Err: err})
			}
		}
	}
	for _, name := range sortedKeys(rd.ReportProperties) {
		collect("reportProperties", name, rd.ReportProperties[name].RunWhen)
	}
	for _, name := range sortedKeys(rd.StatementProperties) {
		collect("statementProperties", name, rd.StatementProperties[name].RunWhen)
	}
	return errs
}

// CheckRunWhen refuses directives with a runWhen criterion that checks nothing
// (RunWhenCriteria.Check), in reportProperties or statementProperties, naming
// the first. run_reports calls it before any report runs: like a statement
// with no output format, such a criterion is a configuration error, and is not
// taken as a criterion that holds or fails.
func (rd *ReportDirectives) CheckRunWhen() error {
	if errs := rd.RunWhenErrors(); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
