package wsvalidate

import "testing"

// ErrorsOnly is the one behaviour in this package, and it is what decides
// whether a save is refused — so it is worth a test even though it is six lines.
func TestErrorsOnly(t *testing.T) {
	in := []Finding{
		{Severity: Warning, Code: "a", Message: "warn"},
		{Severity: Error, Code: "b", Message: "err", Path: "/x"},
		{Severity: Warning, Code: "c", Message: "warn"},
	}
	got := ErrorsOnly(in)
	if len(got) != 1 || got[0].Code != "b" || got[0].Path != "/x" {
		t.Errorf("expected the single error with its path, got %v", got)
	}
	// Never nil: a caller doing len() on the result should not have to care.
	if ErrorsOnly(nil) == nil {
		t.Error("ErrorsOnly(nil) should return an empty slice, not nil")
	}
}
