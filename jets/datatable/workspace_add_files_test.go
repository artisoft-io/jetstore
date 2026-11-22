package datatable

// Adding files to a workspace from the IDE (workspace_add_files.go): each file
// is checked as a save checks it, the files of one request together, and the
// request is all or nothing. These tests write through a recorder in place of
// wsfile.SaveContent, so they need no database; the database half, the
// override it records and its revert, is wsfile's (file_content_test.go).

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const addTestWorkspace = "add_ws"

// addWorkspace makes a workspace under a fresh WORKSPACES_HOME holding files,
// and returns its root.
func addWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	root := filepath.Join(home, addTestWorkspace)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// row is one data row of a request, its name escaped as the IDE escapes it.
func row(name, content string, replace bool) map[string]any {
	return map[string]any{"file_name": url.QueryEscape(name), "file_content": content, "replace": replace}
}

// recorder is a write that records what it was asked to write.
type recorder struct{ written []string }

func (r *recorder) write(name, _ string) error {
	r.written = append(r.written, name)
	return nil
}

// add runs the add action's checked half over rows, writing through a recorder.
func add(t *testing.T, root string, rows ...map[string]any) ([]AddedFileResult, int, error, []string) {
	t.Helper()
	files, err := readAddedFiles(rows)
	if err != nil {
		t.Fatalf("reading the request: %v", err)
	}
	var rec recorder
	results, status, err := addCheckedFiles(addTestWorkspace, root, files, rec.write)
	return results, status, err, rec.written
}

func statuses(results []AddedFileResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Path + "=" + r.Status
	}
	return out
}

const validRunbook = `{"format": 1, "runbook": "health_check",
  "steps": [{"id": "count", "type": "query", "returns": {"n": "integer"}}]}`

// A runbook and its folder of SQL, in a folder the workspace does not have, are
// added together, in the order sent, and each is answered as sent.
func TestAddWorkspaceFilesAddsARunbookAndItsFolder(t *testing.T) {
	root := addWorkspace(t, map[string]string{"runbooks/other.rb.json": validRunbook})
	results, status, err, written := add(t, root,
		row("runbooks/health_check.rb.json", validRunbook, false),
		row("runbooks/health_check/count.sql", "SELECT count(*) AS n FROM jetsapi.input_registry", false),
	)
	if err != nil || status != http.StatusOK {
		t.Fatalf("refused: %d %v", status, err)
	}
	want := []string{"runbooks/health_check.rb.json", "runbooks/health_check/count.sql"}
	if !reflect.DeepEqual(written, want) {
		t.Errorf("wrote %v, want %v", written, want)
	}
	if got := statuses(results); !reflect.DeepEqual(got, []string{
		"runbooks/health_check.rb.json=new", "runbooks/health_check/count.sql=new"}) {
		t.Errorf("statuses %v", got)
	}
	if results[1].FileName != url.QueryEscape("runbooks/health_check/count.sql") || results[1].Warnings == nil {
		t.Errorf("result %+v, want the name as sent and an empty warnings list", results[1])
	}
}

// One refused file refuses the request, and the refusal names every file that
// is refused, with the validator's findings, the same as a save's.
func TestAddWorkspaceFilesIsAllOrNothing(t *testing.T) {
	root := addWorkspace(t, nil)
	results, status, err, written := add(t, root,
		row("runbooks/health_check.rb.json", validRunbook, false),
		row("runbooks/health_check/count.sql", "BEGIN;\nUPDATE t SET a = 1;\nCOMMIT;\n", false),
		row("runbooks/broken.rb.json", `{"format": 1,`, false),
	)
	if status != http.StatusBadRequest || err == nil {
		t.Fatalf("got %d %v, want a 400", status, err)
	}
	if len(written) != 0 {
		t.Errorf("wrote %v, want nothing", written)
	}
	for _, want := range []string{
		"nothing was added",
		"runbooks/health_check/count.sql cannot be saved",
		"line 1: a runbook step may not run this statement: BEGIN is transaction control",
		"the file is not a valid json file",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "health_check.rb.json cannot") {
		t.Errorf("the refusal names a file that passed:\n%v", err)
	}
	if got := statuses(results); got[0] != "runbooks/health_check.rb.json=new" || got[1] != "runbooks/health_check/count.sql=refused" {
		t.Errorf("statuses %v", got)
	}
}

// A name that escapes the workspace, or is not written as the tree writes a
// path, is refused for its name, before anything else is asked of it.
func TestAddWorkspaceFilesRefusesANameOutsideTheWorkspace(t *testing.T) {
	root := addWorkspace(t, nil)
	for name, want := range map[string]string{
		"../outside.sql":             "escapes the workspace",
		"runbooks/../../outside.sql": "escapes the workspace",
		"runbooks/./a.sql":           "escapes the workspace or names the folder",
		"/etc/runbooks/a.sql":        "does not start with /",
		"runbooks\\a.sql":            "forward slashes",
		"runbooks//a.sql":            "empty folder name",
		"runbooks/":                  "names a folder",
		"runbooks/a\n.sql":           "control character",
		"runbooks/ spaced /a.sql":    "starts or ends with a space",
	} {
		results, status, err, written := add(t, root, row(name, "SELECT 1", false))
		if status != http.StatusBadRequest || len(written) != 0 {
			t.Errorf("%q: %d, wrote %v, want refused", name, status, written)
			continue
		}
		if results[0].Status != AddedRefused || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %s %v, want refused saying %q", name, results[0].Status, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside.sql")); err == nil {
		t.Error("a file was written outside the workspace")
	}
}

// A file the tree would not list cannot be added, as it could not be opened or
// deleted from the IDE: adding is no wider than editing. So is a file the IDE
// shows read-only.
func TestAddWorkspaceFilesRefusesAFileTheIdeDoesNotList(t *testing.T) {
	root := addWorkspace(t, nil)
	for _, name := range []string{"runbooks/notes.txt", "scratch/a.sql", "lookup.db", ".git/config", "jet_rules/a.json"} {
		results, status, _, written := add(t, root, row(name, "x", false))
		if status != http.StatusBadRequest || len(written) != 0 || results[0].Status != AddedNotListed {
			t.Errorf("%s: %d %s, wrote %v, want not listed", name, status, results[0].Status, written)
		}
	}
	results, status, err, _ := add(t, root, row("pipes_config/README.md", "# notes", false))
	if status != http.StatusBadRequest || results[0].Status != AddedRefused || !strings.Contains(err.Error(), "read-only") {
		t.Errorf("a read-only file: %d %s %v, want refused as read-only", status, results[0].Status, err)
	}
}

// A file that is there is replaced only when its row asks; a new file from the
// IDE never asks, so it never overwrites one.
func TestAddWorkspaceFilesReplacesOnlyWhenAsked(t *testing.T) {
	root := addWorkspace(t, map[string]string{"runbooks/health_check/count.sql": "SELECT 1"})
	results, status, err, written := add(t, root, row("runbooks/health_check/count.sql", "SELECT 2", false))
	if status != http.StatusBadRequest || len(written) != 0 || results[0].Status != AddedExists ||
		!strings.Contains(err.Error(), "a file of that name is there") {
		t.Errorf("unasked: %d %s %v, wrote %v, want refused", status, results[0].Status, err, written)
	}
	results, status, err, written = add(t, root, row("runbooks/health_check/count.sql", "SELECT 2", true))
	if err != nil || status != http.StatusOK || results[0].Status != AddedReplaces || len(written) != 1 {
		t.Errorf("asked: %d %s %v, wrote %v, want replaced", status, results[0].Status, err, written)
	}
}

// A folder in the file's place, a file in place of one of its folders, and a
// name sent twice refuse the request.
func TestAddWorkspaceFilesRefusesWhatStandsInTheWay(t *testing.T) {
	root := addWorkspace(t, map[string]string{
		"runbooks/health_check.sql/a.sql": "SELECT 1",
		"runbooks/plain.sql":              "SELECT 1",
	})
	for name, want := range map[string]string{
		"runbooks/health_check.sql": "a folder of that name is there",
		"runbooks/plain.sql/b.sql":  "runbooks/plain.sql is a file, not a folder",
	} {
		results, _, err, written := add(t, root, row(name, "SELECT 1", true))
		if len(written) != 0 || results[0].Status != AddedRefused || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %s %v, wrote %v, want refused saying %q", name, results[0].Status, err, written, want)
		}
	}
	results, _, err, _ := add(t, root, row("runbooks/x.sql", "SELECT 1", false), row("runbooks/x.sql", "SELECT 2", false))
	if results[1].Status != AddedRefused || !strings.Contains(err.Error(), "named twice, as file 1 and file 2") {
		t.Errorf("a name sent twice: %s %v", results[1].Status, err)
	}
}

// The files of one request are checked together: a form drawing a table the
// same request adds has no warning, where the form alone has one.
func TestAddWorkspaceFilesChecksTheRequestsFilesTogether(t *testing.T) {
	root := addWorkspace(t, map[string]string{"table_configs/sampleTable.tc.json": `{}`})
	files, err := readAddedFiles([]map[string]any{row("user_flows/sampleUF.form.json", formDrawingTables, false)})
	if err != nil {
		t.Fatal(err)
	}
	alone := checkAddedFiles(addTestWorkspace, root, files)
	if alone[0].Status != AddedNew || len(alone[0].Warnings) != 1 || alone[0].Warnings[0].Code != "form_table_missing" {
		t.Fatalf("the form alone: %+v, want new with one warning", alone[0])
	}
	files, err = readAddedFiles([]map[string]any{
		row("user_flows/sampleUF.form.json", formDrawingTables, false),
		row("table_configs/missingTable.tc.json", `{}`, false),
	})
	if err != nil {
		t.Fatal(err)
	}
	together := checkAddedFiles(addTestWorkspace, root, files)
	if together[0].Status != AddedNew || len(together[0].Warnings) != 0 {
		t.Errorf("the form with its table: %+v, want new with no warning", together[0])
	}
}

// The check answers what an add would do and writes nothing: a file that is
// there is reported as replaced whatever its row says, which is what the IDE
// shows before asking.
func TestCheckAddedFilesWritesNothing(t *testing.T) {
	root := addWorkspace(t, map[string]string{"runbooks/health_check/count.sql": "SELECT 1"})
	files, err := readAddedFiles([]map[string]any{
		row("runbooks/health_check/count.sql", "SELECT 2", false),
		row("runbooks/health_check/more.sql", "SELECT 3", false),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := statuses(checkAddedFiles(addTestWorkspace, root, files))
	if !reflect.DeepEqual(got, []string{"runbooks/health_check/count.sql=replaces", "runbooks/health_check/more.sql=new"}) {
		t.Errorf("statuses %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "runbooks/health_check/more.sql")); err == nil {
		t.Error("the check wrote a file")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "runbooks/health_check/count.sql")); string(b) != "SELECT 1" {
		t.Errorf("the check changed a file: %q", b)
	}
}

// A request with no file, a row with no name, or content that is not text is
// refused whole, before anything is checked.
func TestReadAddedFilesRefusesAMalformedRequest(t *testing.T) {
	for _, rows := range [][]map[string]any{
		nil,
		{{"file_content": "x"}},
		{{"file_name": "runbooks%2Fa.sql", "file_content": 12}},
		{{"file_name": "runbooks%ZZa.sql", "file_content": "x"}},
	} {
		if _, err := readAddedFiles(rows); err == nil {
			t.Errorf("%v was read, want refused", rows)
		}
	}
}

// A write that fails part way says how far it got.
func TestAddCheckedFilesReportsAWriteThatFailsPartWay(t *testing.T) {
	root := addWorkspace(t, nil)
	files, err := readAddedFiles([]map[string]any{
		row("runbooks/a.sql", "SELECT 1", false), row("runbooks/b.sql", "SELECT 2", false),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, status, err := addCheckedFiles(addTestWorkspace, root, files, func(name, _ string) error {
		if name == "runbooks/b.sql" {
			return os.ErrPermission
		}
		return nil
	})
	if status != http.StatusInternalServerError || err == nil ||
		!strings.Contains(err.Error(), "1 of 2 files added, then runbooks/b.sql could not be") {
		t.Errorf("got %d %v", status, err)
	}
}
