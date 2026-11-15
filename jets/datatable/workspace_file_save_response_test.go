package datatable

// What SaveWorkspaceFileContent answers once the user's capability is
// established (saveCheckedWorkspaceFile): a save with warnings only is written
// and returns them, a save with an error is refused and not written, and the
// response keeps the field it always had.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

func TestSaveResponseCarriesTheWarnings(t *testing.T) {
	const name = "pipes_config/example.pc.json"
	escaped := "pipes_config%2Fexample.pc.json"
	misspelt := strings.Replace(goodPipelineConfig, `"output_tables"`, `"output_table"`, 1)

	writes := 0
	write := func() error { writes++; return nil }

	results, status, err := saveCheckedWorkspaceFile(escaped, name, misspelt, nil, write)
	if err != nil || status != http.StatusOK || writes != 1 {
		t.Fatalf("a save with a warning only: status %d, err %v, %d writes; want 200, nil, 1", status, err, writes)
	}
	body, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		FileName string               `json:"file_name"`
		Warnings []wsvalidate.Finding `json:"warnings"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.FileName != escaped {
		t.Errorf("file_name %q, want %q as the request sent it", response.FileName, escaped)
	}
	if len(response.Warnings) != 1 {
		t.Fatalf("warnings %+v, want one", response.Warnings)
	}
	w := response.Warnings[0]
	if w.Severity != wsvalidate.Warning || w.Code != "cpipes_config_unknown_field" || w.Path != "/output_table" ||
		!strings.Contains(w.Message, `"output_table"`) {
		t.Errorf("warning %+v, want the unknown field output_table at /output_table", w)
	}

	// No finding: the field is there, and empty rather than null.
	results, status, err = saveCheckedWorkspaceFile(escaped, name, goodPipelineConfig, nil, write)
	if err != nil || status != http.StatusOK || writes != 2 {
		t.Fatalf("a clean save: status %d, err %v, %d writes", status, err, writes)
	}
	if body, _ = json.Marshal(results); !strings.Contains(string(body), `"warnings":[]`) {
		t.Errorf("a clean save answers %s, want an empty warnings list", body)
	}

	// An error refuses the save, and nothing is written.
	refused := strings.Replace(goodPipelineConfig, `"rdf_type": "double"`, `"rdf_type": "money"`, 1)
	results, status, err = saveCheckedWorkspaceFile(escaped, name, refused, nil, write)
	if err == nil || status != http.StatusBadRequest || results != nil || writes != 2 {
		t.Errorf("a save with an error: status %d, err %v, results %v, %d writes; want 400, an error, nil, 2",
			status, err, results, writes)
	} else if !strings.Contains(err.Error(), "/output_tables/0/columns/2/rdf_type") {
		t.Errorf("the refusal %q does not say where", err)
	}

	// A failed write is the write's error, with a status the client reads as a
	// failure: the HTTP layer sends the status it is given, and the IDE reads it.
	failed := errors.New("disk full")
	results, status, err = saveCheckedWorkspaceFile(escaped, name, goodPipelineConfig, nil, func() error { return failed })
	if !errors.Is(err, failed) || status != http.StatusInternalServerError || results != nil {
		t.Errorf("a failed write: status %d, err %v, results %v; want 500, the write's error, nil", status, err, results)
	}
}
