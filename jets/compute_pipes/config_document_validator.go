package compute_pipes

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

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
//   - a check ValidatePipeSpecConfig makes of the document as a whole, before
//     any step (configDocumentChecks): an empty conditional step, a
//     column_encodings entry with no column, a text template that does not
//     compile or a duplicate template key, a writer setting on an input
//     channel, latest_session_only off a staged file_key read, the retired Go
//     rule engine, and an output table column whose rdf_type is outside
//     schema.PgTypeNames. Each reads the document and nothing else: not the
//     env, not a step's when, not the schema event. Both start paths call
//     ValidatePipeSpecConfig before a step runs and neither succeeds without
//     it, so a document failing one fails at every start (2026-11-14; until
//     then only the column types were checked here).
//
// Warnings, which do not refuse the save:
//
//   - an output table's column with no rdf_type. Creating the table needs one,
//     but a table that exists and is not checked for schema changes never
//     reads it. jets:key and session_id are skipped, here and at the start
//     (validateTableColumnTypes): their type is fixed and their rdf_type is
//     never read;
//   - every field ComputePipesConfig does not know, which the start ignores: a
//     misspelt key is a setting that silently does not apply. Each is named
//     with its JSON Pointer (unknownConfigFields); until 2026-11-14 only the
//     first was, as the decoder reports it.
//
// **The step-by-step startup validator is deliberately not run here**
// (ValidateCpipesConfig, jets/agentic/tools/tools.go). It resolves each step
// against a fixed single-shard environment, which is not the run's, and on
// 2026-11-14 it flagged 4 of the 51 pipeline configurations in four workspaces,
// each of which runs, for one of two reasons:
//
//   - a when (of a conditional step, or of a transformation's
//     conditional_config) compares a variable that the run sets, from the
//     process's env_json or an earlier step, with a number. Unset, the variable
//     stays its own text and the comparison fails (3 configurations, 8 steps).
//     Skipping a step whose when cannot be evaluated would remove those, and
//     leave the step unchecked;
//   - a partition_writer with no device_writer_type takes it from its schema
//     provider's format, and the main_input provider's format is supplied at
//     the start, by the schema event or the source configuration: with none in
//     the document the step reports an unsupported format "" (1
//     configuration). Telling that apart from a real one needs
//     ValidatePipeSpecConfig to say "no format yet", which the start does not
//     need.
//
// So it could only warn here, and it would warn falsely on those four; with the
// two undecidable cases removed it flags nothing in the corpus that the checks
// above do not.
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
	for _, check := range configDocumentChecks {
		if err := check.check(&cfg); err != nil {
			findings = append(findings, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     check.code,
				Message:  fmt.Sprintf("the pipeline cannot start: %v", err),
			})
		}
	}
	for i, tbl := range cfg.OutputTables {
		if tbl == nil {
			continue
		}
		for j := range tbl.Columns {
			c := &tbl.Columns[j]
			if hasFixedColumnType(c.Name) {
				continue
			}
			path := fmt.Sprintf("/output_tables/%d/columns/%d/rdf_type", i, j)
			if c.RdfType == "" {
				findings = append(findings, wsvalidate.Finding{
					Severity: wsvalidate.Warning,
					Code:     "cpipes_config_column_type",
					Message: fmt.Sprintf("output table %q, column %q has no rdf_type: creating the table will fail",
						tbl.Key, c.Name),
					Path: path,
				})
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
	unknown := unknownConfigFields([]byte(content))
	for _, path := range unknown {
		findings = append(findings, wsvalidate.Finding{
			Severity: wsvalidate.Warning,
			Code:     "cpipes_config_unknown_field",
			Message:  fmt.Sprintf("ignored when the pipeline runs: unknown field %q", lastPointerSegment(path)),
			Path:     path,
		})
	}
	if len(unknown) == 0 {
		// The decoder is the authority on what it ignores; the walk is how every
		// such field is named at once. Should they ever disagree, the decoder's
		// own report is kept rather than lost.
		dec := json.NewDecoder(bytes.NewReader([]byte(content)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ComputePipesConfig{}); err != nil {
			findings = append(findings, wsvalidate.Finding{
				Severity: wsvalidate.Warning,
				Code:     "cpipes_config_unknown_field",
				Message:  fmt.Sprintf("ignored when the pipeline runs: %v", err),
			})
		}
	}
	return findings
}

// configDocumentChecks are the checks ValidatePipeSpecConfig makes of the
// document as a whole, in its order, each with the finding code the save check
// reports it under. The output table column types are not here: the save check
// makes that one per column, with each column's path.
//
// The first two are written out again here because ValidatePipeSpecConfig
// makes them inline; TestConfigDocumentChecksAgreeWithTheStart holds the two
// sets equal by running ValidatePipeSpecConfig with no step, which is exactly
// these checks.
var configDocumentChecks = []struct {
	code  string
	check func(*ComputePipesConfig) error
}{
	{"cpipes_config_conditional_step", validateConditionalStepsHavePipes},
	{"cpipes_config_column_encoding", validateColumnEncodingsNameAColumn},
	{"cpipes_config_text_template", validateTextTemplates},
	{"cpipes_config_input_writer_field", validateInputChannelsCarryNoWriterFields},
	{"cpipes_config_latest_session_only", validateLatestSessionOnly},
	{"cpipes_config_go_engine", validateNoGoEngine},
}

// validateConditionalStepsHavePipes is ValidatePipeSpecConfig's first check.
func validateConditionalStepsHavePipes(cpConfig *ComputePipesConfig) error {
	for i := range cpConfig.ConditionalPipesConfig {
		if len(cpConfig.ConditionalPipesConfig[i].PipesConfig) == 0 {
			return fmt.Errorf("configuration error: conditional_pipes_config step %d has an empty pipes_config", i)
		}
	}
	return nil
}

// validateColumnEncodingsNameAColumn is ValidatePipeSpecConfig's second check.
func validateColumnEncodingsNameAColumn(cpConfig *ComputePipesConfig) error {
	for i := range cpConfig.Channels {
		for _, encoding := range cpConfig.Channels[i].ColumnEncodings {
			if encoding == nil || len(encoding.Column) == 0 {
				return fmt.Errorf("configuration error: channel '%s' has a column_encodings entry with no column specified",
					cpConfig.Channels[i].Name)
			}
		}
	}
	return nil
}

// unknownConfigFields returns the JSON Pointer of every object member of the
// document that ComputePipesConfig has no field for, which json.Unmarshal
// ignores, sorted. It walks the decoded document against the struct the way
// the decoder matches names: the json tag or the field name, promoted through
// an embedded struct, exact or case-insensitive. A map's keys are data rather
// than fields, and a value of type any, or of a type that decodes itself, is
// not walked. A document that does not decode as JSON has none.
func unknownConfigFields(content []byte) []string {
	return UnknownJSONFields(content, reflect.TypeFor[ComputePipesConfig]())
}

// UnknownJSONFields is unknownConfigFields for a document decoded into a
// value of type t: the JSON Pointer of every object member json.Unmarshal
// ignores, sorted. The save check of a workspace's reports/config.json uses it
// too (jets/datatable), for its own document type.
func UnknownJSONFields(content []byte, t reflect.Type) []string {
	var doc any
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil
	}
	var unknown []string
	walkUnknownFields(doc, t, "", &unknown)
	sort.Strings(unknown)
	return unknown
}

var (
	jsonUnmarshalerType = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

func walkUnknownFields(value any, t reflect.Type, path string, unknown *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(jsonUnmarshalerType) || reflect.PointerTo(t).Implements(textUnmarshalerType) {
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return
		}
		fields := jsonFieldsOf(t)
		for key, member := range object {
			memberPath := path + "/" + escapePointerSegment(key)
			fieldType, found := fields[key]
			if !found {
				for name, ft := range fields {
					if strings.EqualFold(name, key) {
						fieldType, found = ft, true
						break
					}
				}
			}
			if !found {
				*unknown = append(*unknown, memberPath)
				continue
			}
			walkUnknownFields(member, fieldType, memberPath, unknown)
		}
	case reflect.Slice, reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return
		}
		for i, item := range items {
			walkUnknownFields(item, t.Elem(), fmt.Sprintf("%s/%d", path, i), unknown)
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return
		}
		for key, member := range object {
			walkUnknownFields(member, t.Elem(), path+"/"+escapePointerSegment(key), unknown)
		}
	}
}

// jsonFieldsOf maps the JSON names of a struct's fields to their types, the
// fields of an embedded struct without a tag promoted into it. A name met at a
// shallower depth wins, as it does for the decoder.
func jsonFieldsOf(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	level := []reflect.Type{t}
	for len(level) > 0 {
		var next []reflect.Type
		names := make(map[string]reflect.Type)
		for _, st := range level {
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				tag := f.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, _, _ := strings.Cut(tag, ",")
				if f.Anonymous && name == "" {
					ft := f.Type
					if ft.Kind() == reflect.Pointer {
						ft = ft.Elem()
					}
					if ft.Kind() == reflect.Struct {
						next = append(next, ft)
						continue
					}
				}
				if !f.IsExported() {
					continue
				}
				if name == "" {
					name = f.Name
				}
				if _, seen := fields[name]; !seen {
					names[name] = f.Type
				}
			}
		}
		for name, ft := range names {
			fields[name] = ft
		}
		level = next
	}
	return fields
}

// escapePointerSegment escapes a JSON Pointer segment (RFC 6901 §3).
func escapePointerSegment(segment string) string {
	return strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
}

// lastPointerSegment is the member name a JSON Pointer ends with, unescaped.
func lastPointerSegment(path string) string {
	segment := path[strings.LastIndex(path, "/")+1:]
	return strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
}
