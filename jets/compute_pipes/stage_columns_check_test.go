package compute_pipes

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckStageFileColumns(t *testing.T) {
	channel := []string{"id", "first_name", "last_name", "amount", "category", "session_id"}
	cases := []struct {
		name   string
		header []string
		errHas string // empty: no error
	}{
		{"same columns", channel, ""},
		// Written before first_name and last_name were added to the class: every
		// column after them is shifted.
		{"columns inserted", []string{"id", "amount", "category", "session_id"},
			"column 2 is 'amount' in the file and 'first_name' in the channel"},
		{"trailing column missing", channel[:5], ""},
		{"extra trailing column", append(append([]string{}, channel...), "extra"), ""},
		{"renamed column", []string{"id", "first", "last_name", "amount", "category", "session_id"}, "column 2"},
	}
	for _, c := range cases {
		err := checkStageFileColumns("stage/c/D/2900W/1/part-0001.csv", "D", c.header, channel)
		switch {
		case c.errHas == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.name, err)
		case c.errHas != "" && (err == nil || !strings.Contains(err.Error(), c.errHas)):
			t.Errorf("%s: got %v, want an error with %q", c.name, err, c.errHas)
		case err != nil && !strings.Contains(err.Error(), "stage/c/D/2900W/1/part-0001.csv"):
			t.Errorf("%s: error %v does not name the file", c.name, err)
		}
	}
}

func TestStageColumnsToCheck(t *testing.T) {
	spec := &ChannelSpec{Name: "D", Columns: []string{"a", "b"}}
	byFileKey := &InputChannelConfig{Name: "D", Type: "stage"}
	byFileKey.FileKey = "$CLIENT/D/${WEEK_PERIOD}W"
	byFileKey.Format = "csv"
	if got := stageColumnsToCheck(byFileKey, spec); len(got) != 2 {
		t.Errorf("csv stage read by file_key: got %v, want the spec's columns", got)
	}
	headerless := *byFileKey
	headerless.Format = "headerless_csv"
	byStep := &InputChannelConfig{Name: "D", Type: "stage", ReadStepId: "s1"}
	byStep.Format = "csv"
	same := &ChannelSpec{Name: "D", Columns: []string{"a"}, SameColumnsAsInput: true}
	dynamic := &ChannelSpec{Name: "D", Columns: []string{"a"}, HasDynamicColumns: true}
	for name, got := range map[string][]string{
		"headerless":            stageColumnsToCheck(&headerless, spec),
		"read_step_id":          stageColumnsToCheck(byStep, spec),
		"same_columns_as_input": stageColumnsToCheck(byFileKey, same),
		"has_dynamic_columns":   stageColumnsToCheck(byFileKey, dynamic),
		"no spec":               stageColumnsToCheck(byFileKey, nil),
	} {
		if got != nil {
			t.Errorf("%s: got %v, want no check", name, got)
		}
	}
}

// The check as ReadCsvFile applies it: a staged file in the old layout is refused
// before any row is read, and one in the channel's layout reads.
func TestReadCsvFileRefusesAStagedFileInAnotherLayout(t *testing.T) {
	channel := []string{"id", "first_name", "amount"}
	read := func(data string) (int64, error) {
		cpCtx := ComputePipesContextTestBuilder{
			Compression: "none", CpipesMode: "reducing", Delimiter: ',', Format: "csv", InputColumns: channel,
		}.build()
		ch := make(chan []any, 10)
		count, _, err := cpCtx.ReadCsvFile(&FileName{
			InFileKeyInfo: FileKeyInfo{key: "stage/c/D/2900W/1/part-0001.csv", size: len(data)},
			StageColumns:  channel, StageChannel: "D",
		}, bytes.NewReader([]byte(data)), nil, nil, ch, nil)
		close(ch)
		return count, err
	}
	if _, err := read("id,amount\n1,a text value\n"); err == nil ||
		!strings.Contains(err.Error(), "column 2 is 'amount' in the file and 'first_name' in the channel") {
		t.Errorf("old layout: got %v, want the column named", err)
	}
	if count, err := read("id,first_name,amount\n1,Ann,12.5\n2,Bob,3\n"); err != nil || count != 2 {
		t.Errorf("same layout: got %d rows, %v; want 2 rows", count, err)
	}
}
