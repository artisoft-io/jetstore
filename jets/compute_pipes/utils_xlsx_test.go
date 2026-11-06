package compute_pipes

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTestWorkbook writes a minimal xlsx file to dir, one worksheet per entry of
// sheets, in order, each cell an inline string. It holds only the parts xlsxreader
// needs, so the tests need no xlsx writer.
func writeTestWorkbook(t *testing.T, dir string, names []string, sheets [][][]string) string {
	t.Helper()
	path := filepath.Join(dir, "workbook.xlsx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	put := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
	const rns = `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	var wbSheets, rels strings.Builder
	for i, name := range names {
		fmt.Fprintf(&wbSheets, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, name, i+1, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
		var data strings.Builder
		for r, row := range sheets[i] {
			fmt.Fprintf(&data, `<row r="%d">`, r+1)
			for c, v := range row {
				fmt.Fprintf(&data, `<c r="%c%d" t="inlineStr"><is><t>%s</t></is></c>`, 'A'+c, r+1, v)
			}
			data.WriteString(`</row>`)
		}
		put(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1),
			`<?xml version="1.0" encoding="UTF-8"?><worksheet `+ns+`><sheetData>`+data.String()+`</sheetData></worksheet>`)
	}
	put("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?><workbook `+ns+` `+rns+`><sheets>`+wbSheets.String()+`</sheets></workbook>`)
	put("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+rels.String()+`</Relationships>`)
	put("xl/styles.xml", `<?xml version="1.0" encoding="UTF-8"?><styleSheet `+ns+`/>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// An instructions tab first and the data second: the shape that showed the row read
// ignoring currentSheet.
func instructionsThenData(t *testing.T) string {
	return writeTestWorkbook(t, t.TempDir(),
		[]string{"Instructions", "Data"},
		[][][]string{
			{{"How to fill", "this workbook"}, {"Step 1", "one row per field"}},
			{{"layout_name", "field"}, {"LAYOUT_A", "FIELD_1"}, {"LAYOUT_B", "FIELD_2"}},
		})
}

func TestXlsxSheetPos(t *testing.T) {
	sheets := []string{"Instructions", "Data"}
	tests := []struct {
		name    string
		data    map[string]any
		want    int
		wantErr bool
	}{
		{"nil map", nil, 0, false},
		{"no currentSheet", map[string]any{}, 0, false},
		{"by name", map[string]any{"currentSheet": "Data"}, 1, false},
		{"first by name", map[string]any{"currentSheet": "Instructions"}, 0, false},
		{"unknown name", map[string]any{"currentSheet": "data"}, 0, true},
		{"position as string", map[string]any{"currentSheet": "1"}, 1, false},
		{"position as number", map[string]any{"currentSheet": float64(1)}, 1, false},
		{"fractional number", map[string]any{"currentSheet": 1.5}, 0, true},
		{"position past the end", map[string]any{"currentSheet": "2"}, 0, true},
		{"negative position", map[string]any{"currentSheet": float64(-1)}, 0, true},
		{"not a name or position", map[string]any{"currentSheet": true}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := XlsxSheetPos(sheets, tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("XlsxSheetPos() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("XlsxSheetPos() = %d, want %d", got, tt.want)
			}
		})
	}
	if _, err := XlsxSheetPos(nil, nil); err == nil {
		t.Error("XlsxSheetPos() with no worksheet: want an error")
	}
}

// xlsxTestContext is the least ComputePipesContext ReadXlsxFile reads: a sharding
// node on an xlsx main input with the given columns.
func xlsxTestContext(inputColumns []string) *ComputePipesContext {
	return &ComputePipesContext{
		CpConfig: &ComputePipesConfig{
			CommonRuntimeArgs: &ComputePipesCommonArgs{
				CpipesMode:    "sharding",
				SourcesConfig: SourcesConfigSpec{MainInput: &InputSourceSpec{InputColumns: inputColumns}},
			},
			ClusterConfig: &ClusterSpec{},
			PipesConfig:   []PipeSpec{{InputChannel: InputChannelConfig{FileConfig: FileConfig{Format: "xlsx", Compression: "none"}}}},
		},
		Done: make(chan struct{}),
	}
}

// readXlsxRows runs ReadXlsxFile on a copy of path, as the sharding step does, and
// returns the rows it sends. ReadXlsxFile removes the file it reads.
func readXlsxRows(t *testing.T, path string, inputColumns []string, formatDataJson string) [][]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), "local.xlsx")
	if err := os.WriteFile(local, b, 0o644); err != nil {
		t.Fatal(err)
	}
	sheetInfo, err := ParseInputFormatDataXlsx(&formatDataJson)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan []any, 16)
	if _, _, err := xlsxTestContext(inputColumns).ReadXlsxFile(&FileName{LocalFileName: local}, sheetInfo, nil, nil, ch, nil); err != nil {
		t.Fatal(err)
	}
	close(ch)
	var rows [][]any
	for r := range ch {
		rows = append(rows, r)
	}
	return rows
}

// The header read and the row read must read the same worksheet. Before XlsxSheetPos
// the row read always took the first one, so a workbook whose data follows an
// instructions tab loaded the instructions under the data's headers.
func TestXlsxHeadersAndRowsReadTheSameSheet(t *testing.T) {
	path := instructionsThenData(t)
	wantData := [][]any{{"LAYOUT_A", "FIELD_1"}, {"LAYOUT_B", "FIELD_2"}}
	for _, formatData := range []string{
		`{"currentSheet": "Data"}`,
		`{"currentSheet": "1"}`,
		`{"currentSheet": 1}`,
	} {
		t.Run(formatData, func(t *testing.T) {
			headers, err := GetRawHeadersXlsx(false, path, formatData)
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{"layout_name", "field"}; !reflect.DeepEqual(headers, want) {
				t.Fatalf("headers = %v, want %v", headers, want)
			}
			if rows := readXlsxRows(t, path, headers, formatData); !reflect.DeepEqual(rows, wantData) {
				t.Errorf("rows = %v, want %v", rows, wantData)
			}
		})
	}
}

// Without input format data both reads take the first worksheet, as before. The
// header read used to write into the nil map ParseInputFormatDataXlsx returns for
// an empty string, and panic.
func TestXlsxNoFormatDataReadsTheFirstSheet(t *testing.T) {
	path := instructionsThenData(t)
	headers, err := GetRawHeadersXlsx(false, path, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"How to fill", "this workbook"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("headers = %v, want %v", headers, want)
	}
	want := [][]any{{"Step 1", "one row per field"}}
	if rows := readXlsxRows(t, path, headers, ""); !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

func TestXlsxUnknownSheetFailsBothReads(t *testing.T) {
	path := instructionsThenData(t)
	formatData := `{"currentSheet": "Missing"}`
	if _, err := GetRawHeadersXlsx(false, path, formatData); err == nil {
		t.Error("GetRawHeadersXlsx: want an error for a sheet the workbook does not have")
	}
	b, _ := os.ReadFile(path)
	local := filepath.Join(t.TempDir(), "local.xlsx")
	os.WriteFile(local, b, 0o644)
	sheetInfo, _ := ParseInputFormatDataXlsx(&formatData)
	if _, _, err := xlsxTestContext([]string{"a", "b"}).ReadXlsxFile(&FileName{LocalFileName: local}, sheetInfo, nil, nil, make(chan []any, 4), nil); err == nil {
		t.Error("ReadXlsxFile: want an error for a sheet the workbook does not have")
	}
}
