package datatable

// The `.pc.json` row of the save path (compute_pipes.ValidateConfigDocument):
// what it refuses, what it lets through with a warning, and that no pipeline
// configuration in use is refused by it.
//
// TestWorkspacePcJsonPassesTheSaveCheck walks the rule workspaces, which are
// separate repositories, so it skips unless JETS_PC_CORPUS_DIR names a
// directory holding them (the convention of jets/compute_pipes's corpus tests):
//
//	JETS_PC_CORPUS_DIR=<...>/workspaces go test -count=1 -run PcJson ./jets/datatable/

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

const goodPipelineConfig = `{
  "cluster_config": {"max_nbr_partitions": 1},
  "output_tables": [{"key": "out", "name": "public.out", "columns": [
    {"name": "jets:key", "rdf_type": "text"},
    {"name": "session_id"},
    {"name": "amount", "rdf_type": "double"},
    {"name": "codes", "rdf_type": "text", "as_array": true},
    {"name": "created", "rdf_type": "timestamptz"}
  ]}],
  "pipes_config": []
}`

func TestPipelineConfigGoesThroughTheSavePath(t *testing.T) {
	const name = "pipes_config/example.pc.json"
	if _, err := checkWorkspaceFile(name, goodPipelineConfig); err != nil {
		t.Fatalf("a valid pipeline configuration must save: %v", err)
	}

	for what, c := range map[string]struct{ content, want string }{
		"a column type outside the closed list": {
			strings.Replace(goodPipelineConfig, `"rdf_type": "double"`, `"rdf_type": "text); DROP TABLE x; --"`, 1),
			"/output_tables/0/columns/2/rdf_type: the pipeline cannot start",
		},
		"a type that is not a type name": {
			strings.Replace(goodPipelineConfig, `"rdf_type": "double"`, `"rdf_type": "money"`, 1),
			`column "amount"`,
		},
		"a check of the document as a whole": {
			strings.Replace(goodPipelineConfig, `"pipes_config": []`,
				`"conditional_pipes_config": [{"step_name": "a", "pipes_config": []}]`, 1),
			"the pipeline cannot start: configuration error: conditional_pipes_config step 0 has an empty pipes_config",
		},
		"no cluster_config": {
			strings.Replace(goodPipelineConfig, `"cluster_config": {"max_nbr_partitions": 1},`, "", 1),
			"/cluster_config: the pipeline cannot start: cluster_config is required",
		},
		"a value of the wrong JSON type": {
			strings.Replace(goodPipelineConfig, `"pipes_config": []`, `"pipes_config": {}`, 1),
			"the configuration does not decode",
		},
		"not JSON": {`{"cluster_config": `, "not a valid json"},
	} {
		_, err := checkWorkspaceFile(name, c.content)
		if err == nil {
			t.Errorf("%s was saved", what)
		} else if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v\nwant it to contain %q", what, err, c.want)
		}
	}

	// Warnings, which do not refuse the save and which it returns.
	for what, c := range map[string]struct{ content, code string }{
		"a column with no rdf_type": {
			strings.Replace(goodPipelineConfig, `"name": "amount", "rdf_type": "double"`, `"name": "amount"`, 1),
			"cpipes_config_column_type",
		},
		"a misspelt key": {
			strings.Replace(goodPipelineConfig, `"output_tables"`, `"output_table"`, 1),
			"cpipes_config_unknown_field",
		},
	} {
		warnings, err := checkWorkspaceFile(name, c.content)
		if err != nil {
			t.Errorf("%s was refused: %v", what, err)
		}
		if len(warnings) != 1 || warnings[0].Severity != wsvalidate.Warning || warnings[0].Code != c.code {
			t.Errorf("%s: warnings %+v, want one %s warning", what, warnings, c.code)
		}
	}
	// jets:key and session_id are created as text whatever rdf_type says, so a
	// type outside the list on either is not checked, here or at the start.
	for column, typed := range map[string]string{
		`{"name": "jets:key", "rdf_type": "text"}`: `{"name": "jets:key", "rdf_type": "txt"}`,
		`{"name": "session_id"}`:                   `{"name": "session_id", "rdf_type": "txt"}`,
	} {
		content := strings.Replace(goodPipelineConfig, column, typed, 1)
		if content == goodPipelineConfig {
			t.Fatalf("the replacement of %s did not apply", column)
		}
		if warnings, err := checkWorkspaceFile(name, content); err != nil || len(warnings) != 0 {
			t.Errorf("rdf_type txt on %s: %v, warnings %+v", column, err, warnings)
		}
	}
	if findings := validatorFor(name)(goodPipelineConfig); len(findings) != 0 {
		t.Errorf("the valid configuration has findings: %+v", findings)
	}
}

// pcJsonFindings runs every .pc.json under root through the save check, failing
// the test for each one it refuses, and returns the number of files and the
// errors and warnings found, per directory directly under root.
func pcJsonFindings(t *testing.T, root string) (files, errs, warns map[string]int) {
	t.Helper()
	files, errs, warns = map[string]int{}, map[string]int{}, map[string]int{}
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
		rel, _ := filepath.Rel(root, path)
		group := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[group]++
		if _, err := checkWorkspaceFile(rel, string(raw)); err != nil {
			t.Errorf("%s is refused by the save check: %v", rel, err)
		}
		for _, f := range validatorFor(rel)(string(raw)) {
			if f.Severity == wsvalidate.Error {
				errs[group]++
			} else {
				warns[group]++
			}
			t.Logf("%s: %s %s: %s %s", rel, f.Severity, f.Code, f.Path, f.Message)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files, errs, warns
}

// Every .pc.json this repository carries (the test workspace and the shipped
// workspace assets) saves. Needs no environment.
func TestInRepoPcJsonPassesTheSaveCheck(t *testing.T) {
	files, _, _ := pcJsonFindings(t, "..")
	total := 0
	for _, n := range files {
		total += n
	}
	if total == 0 {
		t.Fatal("no .pc.json under jets/")
	}
}

// No pipeline configuration in the rule workspaces is refused by the row. The
// counts are logged rather than asserted: the workspaces are living
// repositories, and the property is that nothing is refused.
func TestWorkspacePcJsonPassesTheSaveCheck(t *testing.T) {
	dir := os.Getenv("JETS_PC_CORPUS_DIR")
	if dir == "" {
		t.Skip("JETS_PC_CORPUS_DIR is not set; see the header of this file")
	}
	files, errs, warns := pcJsonFindings(t, dir)
	if len(files) == 0 {
		t.Fatalf("no .pc.json under %s", dir)
	}
	groups := make([]string, 0, len(files))
	for g := range files {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		t.Logf("%s: %d .pc.json, %d errors, %d warnings", g, files[g], errs[g], warns[g])
	}
}
