package utils

import "testing"

// FileKeyCarries tells what the key carries from what
// SplitFileKeyIntoComponents defaults, which the map alone does not.
func TestFileKeyCarries(t *testing.T) {
	key := "client=Acme/vendor=Ex/year=2026/month=ab/file.csv"
	for name, want := range map[string]bool{
		"client":   true,
		"vendor":   true,
		"org":      true, // a vendor component carries org
		"year":     true,
		"month":    true, // carried, though not a number
		"day":      false,
		"file.csv": false,
	} {
		if got := FileKeyCarries(key, name); got != want {
			t.Errorf("FileKeyCarries(%q) = %v, want %v", name, got, want)
		}
	}
	components := SplitFileKeyIntoComponents(map[string]any{}, &key)
	if components["day"] != 1 || components["month"] != 0 {
		t.Errorf("SplitFileKeyIntoComponents defaults changed: month %v, day %v", components["month"], components["day"])
	}
}

// FileKeyComponent gives the text the key carries, before year, month and day
// are typed.
func TestFileKeyComponent(t *testing.T) {
	key := "client=Acme/year=2026/month=ab/day=/file.csv"
	for name, want := range map[string]struct {
		text    string
		carried bool
	}{
		"client": {"Acme", true},
		"year":   {"2026", true},
		"month":  {"ab", true},
		"day":    {"", true},
		"org":    {"", false},
	} {
		text, carried := FileKeyComponent(key, name)
		if text != want.text || carried != want.carried {
			t.Errorf("FileKeyComponent(%q) = %q, %v, want %q, %v", name, text, carried, want.text, want.carried)
		}
	}
}
