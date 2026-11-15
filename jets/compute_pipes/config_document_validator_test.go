package compute_pipes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The save check refuses an output table column type exactly when the start
// does (validateOutputTableColumnTypes, which ValidatePipeSpecConfig calls), for
// every name in the closed list and for names outside it, on every column name.
func TestConfigDocumentColumnTypesAgreeWithTheStart(t *testing.T) {
	types := append(schema.PgTypeNames(), "", "money", "TEXT", "text[]", "text); DROP TABLE x; --")
	for _, column := range []string{"jets:key", "session_id", "amount"} {
		for _, rdfType := range types {
			cfg := &ComputePipesConfig{
				ClusterConfig: &ClusterSpec{},
				OutputTables: []*TableSpec{{Key: "out", Name: "public.out",
					Columns: []TableColumnSpec{{Name: column, RdfType: rdfType}}}},
			}
			raw, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			startRefuses := validateOutputTableColumnTypes(cfg) != nil
			saveRefuses := len(wsvalidate.ErrorsOnly(ValidateConfigDocument(string(raw)))) > 0
			if startRefuses != saveRefuses {
				t.Errorf("column %q, rdf_type %q: the start refuses it: %v, the save refuses it: %v",
					column, rdfType, startRefuses, saveRefuses)
			}
		}
	}
}

// The save check refuses a document exactly when ValidatePipeSpecConfig's checks
// of the document as a whole do. Run with no step, ValidatePipeSpecConfig makes
// those checks and nothing else, so this holds configDocumentChecks, two of which
// are written out again, equal to the start's.
func TestConfigDocumentChecksAgreeWithTheStart(t *testing.T) {
	const good = `{"cluster_config": {}, "pipes_config": [{"type": "fan_out",
	  "input_channel": {"type": "input", "name": "input_row"}, "apply": []}]}`
	for what, c := range map[string]struct{ content, code string }{
		"nothing wrong": {good, ""},
		"an empty conditional step": {
			`{"cluster_config": {}, "conditional_pipes_config": [{"step_name": "a", "pipes_config": []}]}`,
			"cpipes_config_conditional_step"},
		"a column_encodings entry with no column": {
			`{"cluster_config": {}, "channels": [{"name": "c", "columns": ["a"], "column_encodings": [{"entity_encoding": "json"}]}]}`,
			"cpipes_config_column_encoding"},
		"a duplicate text template key": {
			`{"cluster_config": {}, "text_templates": [{"key": "t", "elements": []}, {"key": "t", "elements": []}]}`,
			"cpipes_config_text_template"},
		"a writer setting on an input channel": {
			`{"cluster_config": {}, "pipes_config": [{"type": "fan_out",
			  "input_channel": {"type": "input", "name": "input_row", "quote_all_records": true}, "apply": []}]}`,
			"cpipes_config_input_writer_field"},
		"latest_session_only off a staged file_key read": {
			`{"cluster_config": {}, "pipes_config": [{"type": "fan_out",
			  "input_channel": {"type": "input", "name": "input_row", "latest_session_only": true}, "apply": []}]}`,
			"cpipes_config_latest_session_only"},
		"the Go rule engine, in a conditional_config override": {
			`{"cluster_config": {}, "pipes_config": [{"type": "fan_out",
			  "input_channel": {"type": "input", "name": "input_row"}, "apply": [{"type": "map_record",
			  "conditional_config": [{"then": {"type": "jetrules",
			  "jetrules_config": {"process_name": "p", "use_jet_rules_go": true}}}]}]}]}`,
			"cpipes_config_go_engine"},
	} {
		var cfg ComputePipesConfig
		if err := json.Unmarshal([]byte(c.content), &cfg); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		startRefuses := (&CpipesStartup{}).ValidatePipeSpecConfig(&cfg, nil) != nil
		errs := wsvalidate.ErrorsOnly(ValidateConfigDocument(c.content))
		if startRefuses != (len(errs) > 0) {
			t.Errorf("%s: the start refuses it: %v, the save errors: %+v", what, startRefuses, errs)
		}
		if c.code == "" {
			continue
		}
		if len(errs) != 1 || errs[0].Code != c.code || !strings.HasPrefix(errs[0].Message, "the pipeline cannot start: ") {
			t.Errorf("%s: errors %+v, want one %s", what, errs, c.code)
		}
	}
}

// Every field the decoder would ignore is named, with its JSON Pointer, however
// deep, and nothing it reads is: a field promoted from an embedded struct, a name
// matched without regard to case, and the keys of a map, which are data.
func TestConfigDocumentNamesEveryUnknownField(t *testing.T) {
	const content = `{
	  "cluster_config": {"max_nbr_partitions": 1, "nbr_nodes": 2},
	  "Output_Tables": [{"key": "out", "name": "public.out", "columns": [{"name": "a", "rdf_type": "text", "typ": "x"}]}],
	  "use_ecs_tasks": true,
	  "pipes_config": [{"type": "fan_out",
	    "input_channel": {"type": "input", "name": "input_row", "delimiter": 44, "domain_keys": {"anything": {"x": 1}}, "a/b~c": 1},
	    "apply": [{"type": "map_record", "output_channel": {"name": "o", "device_writer_type": "csv_writer"}}]}]
	}`
	want := []string{
		"/Output_Tables/0/columns/0/typ",
		"/cluster_config/nbr_nodes",
		"/pipes_config/0/apply/0/output_channel/device_writer_type",
		"/pipes_config/0/input_channel/a~1b~0c",
		"/use_ecs_tasks",
	}
	var got []string
	for _, f := range ValidateConfigDocument(content) {
		if f.Code != "cpipes_config_unknown_field" {
			continue
		}
		if f.Severity != wsvalidate.Warning {
			t.Errorf("%s: severity %s, want a warning", f.Path, f.Severity)
		}
		got = append(got, f.Path)
		if f.Path == "/pipes_config/0/input_channel/a~1b~0c" && f.Message != `ignored when the pipeline runs: unknown field "a/b~c"` {
			t.Errorf("message %q does not name the field as written", f.Message)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("unknown fields\n got %v\nwant %v", got, want)
	}
	if fields := unknownConfigFields([]byte(`{"cluster_config": {}}`)); len(fields) != 0 {
		t.Errorf("a document with no unknown field: %v", fields)
	}
}

// The walk names an unknown field exactly when the decoder refuses one, and the
// one it refuses is among those named, on every .pc.json this repository carries
// and, with JETS_PC_CORPUS_DIR set, on the rule workspaces' (see
// jets/datatable/pipeline_config_save_test.go).
func TestUnknownFieldsAgreeWithTheDecoder(t *testing.T) {
	roots := []string{".."}
	if dir := os.Getenv("JETS_PC_CORPUS_DIR"); dir != "" {
		roots = append(roots, dir)
	}
	files := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".pc.json") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if json.Unmarshal(raw, &ComputePipesConfig{}) != nil {
				return nil
			}
			files++
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			decErr := dec.Decode(&ComputePipesConfig{})
			walked := unknownConfigFields(raw)
			if (decErr != nil) != (len(walked) > 0) {
				t.Errorf("%s: the decoder says %v, the walk names %v", path, decErr, walked)
				return nil
			}
			if decErr != nil {
				named := false
				for _, p := range walked {
					named = named || strings.HasSuffix(decErr.Error(), fmt.Sprintf("%q", lastPointerSegment(p)))
				}
				if !named {
					t.Errorf("%s: the decoder refuses %v, which the walk does not name: %v", path, decErr, walked)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	if files == 0 {
		t.Fatal("no .pc.json found")
	}
	t.Logf("%d .pc.json files", files)
}
