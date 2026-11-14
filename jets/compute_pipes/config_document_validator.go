package compute_pipes

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// ValidateConfigDocument checks a pipeline configuration (.pc.json) when the
// workspace IDE saves it: the `.pc.json` row of the save path's validator table
// (workspaceFileValidators, jets/datatable/workspace_file_validators.go).
//
// **An error is only what certainly fails when the pipeline starts**, whatever
// the run's environment, because an error refuses the save:
//
//   - the document does not decode into ComputePipesConfig. Both start paths
//     decode it with a plain json.Unmarshal and stop on its error
//     (loadStartConfig, UnmarshalComputePipesConfig);
//   - it has no cluster_config, which both start paths refuse by name;
//   - an output table's column names an rdf_type outside schema.PgTypeNames.
//     ValidatePipeSpecConfig refuses it at every start, before any step runs
//     (validateOutputTableColumnTypes), with the same rule: a column with an
//     rdf_type, whatever its name, must name a type of the list.
//
// Warnings, which do not refuse the save:
//
//   - an output table's column with no rdf_type. Creating the table needs one,
//     but a table that exists and is not checked for schema changes never
//     reads it; jets:key and session_id take their type from their name.
//   - a field ComputePipesConfig does not know, which the start ignores: a
//     misspelt key is a setting that silently does not apply. Only the first
//     is named, which is what the decoder reports.
//
// **The step-by-step startup validator is deliberately not run here**
// (ValidateCpipesConfig, jets/agentic/tools/tools.go). It resolves each step
// against a fixed single-shard environment, so a condition on a variable the
// run sets, or an output format the run's input decides, is reported as an
// error a run would not meet: on 2026-11-14 it flagged 4 of the 51 pipeline
// configurations in four workspaces, each for one of those two reasons. Its
// findings could only be warnings here, and the save path reports errors only
// (checkWorkspaceFile), so running every step on each save would buy nothing a
// user sees.
func ValidateConfigDocument(content string) []wsvalidate.Finding {
	var cfg ComputePipesConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "cpipes_config_decode",
			Message:  fmt.Sprintf("the pipeline cannot start: the configuration does not decode: %v", err),
		}}
	}
	var findings []wsvalidate.Finding
	if cfg.ClusterConfig == nil {
		findings = append(findings, wsvalidate.Finding{
			Severity: wsvalidate.Error,
			Code:     "cpipes_config_cluster",
			Message:  "the pipeline cannot start: cluster_config is required",
			Path:     "/cluster_config",
		})
	}
	for i, tbl := range cfg.OutputTables {
		if tbl == nil {
			continue
		}
		for j := range tbl.Columns {
			c := &tbl.Columns[j]
			path := fmt.Sprintf("/output_tables/%d/columns/%d/rdf_type", i, j)
			if c.RdfType == "" {
				if c.Name != "jets:key" && c.Name != "session_id" {
					findings = append(findings, wsvalidate.Finding{
						Severity: wsvalidate.Warning,
						Code:     "cpipes_config_column_type",
						Message: fmt.Sprintf("output table %q, column %q has no rdf_type: creating the table will fail",
							tbl.Key, c.Name),
						Path: path,
					})
				}
				continue
			}
			if _, err := schema.ToPgType(c.RdfType); err != nil {
				findings = append(findings, wsvalidate.Finding{
					Severity: wsvalidate.Error,
					Code:     "cpipes_config_column_type",
					Message: fmt.Sprintf("the pipeline cannot start: output table %q, column %q: %v",
						tbl.Key, c.Name, err),
					Path: path,
				})
			}
		}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(content)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ComputePipesConfig{}); err != nil {
		findings = append(findings, wsvalidate.Finding{
			Severity: wsvalidate.Warning,
			Code:     "cpipes_config_unknown_field",
			Message:  fmt.Sprintf("ignored when the pipeline runs: %v", err),
		})
	}
	return findings
}
