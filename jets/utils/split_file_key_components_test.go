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

// A year, month or day that is present and not a number is 0, as the log line
// says, not the 1970, 1 or 1 an absent one gets: FileKeyDate, and through it a
// job failure's period and a schema event's source period, tell a bad date from
// a real one by the 0.
func TestSplitFileKeyNonNumericDateIsZero(t *testing.T) {
	key := "client=c/year=abc/month=x/day=y/in.csv"
	c := SplitFileKeyIntoComponents(map[string]any{}, &key)
	if c["year"] != 0 || c["month"] != 0 || c["day"] != 0 {
		t.Errorf("year, month, day = %v, %v, %v; want 0, 0, 0", c["year"], c["month"], c["day"])
	}
	absent := "client=c/in.csv"
	c = SplitFileKeyIntoComponents(map[string]any{}, &absent)
	if c["year"] != 1970 || c["month"] != 1 || c["day"] != 1 {
		t.Errorf("absent year, month, day = %v, %v, %v; want 1970, 1, 1", c["year"], c["month"], c["day"])
	}
}

func TestFileKeyDate(t *testing.T) {
	for key, want := range map[string]string{
		"a/year=2025/month=3/day=7/f.csv":  "2025-03-07",
		"a/year=2025/month=12/f.csv":       "2025-12-01",
		"a/year=2025/f.csv":                "2025-01-01",
		"a/month=3/day=7/f.csv":            "", // no year
		"a/year=abc/f.csv":                 "", // not a number: 0
		"a/year=2025/month=x/f.csv":        "",
		"a/year=2025/month=13/f.csv":       "",
		"a/year=2025/month=2/day=30/f.csv": "",
		"a/year=0/f.csv":                   "",
		"":                                 "",
	} {
		d, ok := FileKeyDate(key)
		got := ""
		if ok {
			got = d.Format("2006-01-02")
		}
		if got != want {
			t.Errorf("FileKeyDate(%q) = %q, %v; want %q", key, got, ok, want)
		}
	}
}
