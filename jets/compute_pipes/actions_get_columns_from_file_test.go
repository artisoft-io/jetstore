package compute_pipes

import (
	"testing"
)

// AdjustFillers is the whole of what fail_on_empty_column_name decides: with the
// switch off an empty column name becomes FILLER and the run continues, with it
// on the sharding step fails before any worker starts.
//
// Note that the switch is read from the schema provider
// (`sp.FailOnEmptyColumnName`, actions_start_sharding_cp.go) and reaches
// AdjustFillers through FetchHeadersAndDelimiterFromFile, which is called only
// when the headers are fetched from the first input file. A run whose input
// columns come from source_config.input_columns_json or from the schema
// provider's own columns never fetches headers and never reaches this function,
// whatever the switch says.
func TestAdjustFillersOff(t *testing.T) {
	headers := []string{"member_id", "", "dob", ""}
	if err := AdjustFillers(false, &headers); err != nil {
		t.Fatalf("expecting no error with fail_on_empty_column_name off, got %v", err)
	}
	want := []string{"member_id", "FILLER", "dob", "FILLER"}
	for i := range want {
		if headers[i] != want[i] {
			t.Errorf("header %d: expecting %q, got %q", i, want[i], headers[i])
		}
	}
}

func TestAdjustFillersOn(t *testing.T) {
	headers := []string{"member_id", "", "dob", ""}
	err := AdjustFillers(true, &headers)
	if err == nil {
		t.Fatal("expecting an error with fail_on_empty_column_name on, got none")
	}
	// The position is reported one-based and names the first empty column.
	if err.Error() != "empty header found at position 2" {
		t.Errorf("unexpected message: %q", err.Error())
	}
}

func TestAdjustFillersNoEmptyHeader(t *testing.T) {
	headers := []string{"member_id", "dob"}
	if err := AdjustFillers(true, &headers); err != nil {
		t.Fatalf("expecting no error when no header is empty, got %v", err)
	}
}
