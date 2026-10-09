package compute_pipes

import (
	"strings"
	"testing"
)

// A merged file's name follows the format it is written in when the document
// asks for it with ${FILE_EXTENSION}, and is unchanged when it does not.
func TestMergedFileNameTakesTheFormatsExtension(t *testing.T) {
	env := map[string]any{"$SESSIONID": "s1"}
	cases := []struct{ name, format, want string }{
		{"member.${FILE_EXTENSION}", "csv", "member.csv"},
		{"member.${FILE_EXTENSION}", "headerless_csv", "member.csv"},
		{"member.${FILE_EXTENSION}", "parquet", "member.parquet"},
		{"member.${FILE_EXTENSION}", "parquet_select", "member.parquet"},
		{"member.${FILE_EXTENSION}", "fixed_width", "member.fixed_width"},
		{"member", "parquet", "member"},
		{"$SESSIONID.${FILE_EXTENSION}", "csv", "s1.csv"},
	}
	for _, c := range cases {
		got, err := MergedFileName(c.name, c.format, env)
		if err != nil || got != c.want {
			t.Errorf("MergedFileName(%q, %q) = %q, %v; want %q", c.name, c.format, got, err, c.want)
		}
	}
}

// A part and a merged file of one format take one extension: the rule is the
// partition writer's, keyed by the device writer that accepts the format.
func TestMergedFileExtensionAgreesWithThePartitionWriter(t *testing.T) {
	byWriter := map[string]string{"csv_writer": "csv", "parquet_writer": "parquet", "fixed_width_writer": "fixed_width"}
	formats := map[string][]string{
		"csv_writer":         {"csv", "headerless_csv", "xlsx", "headerless_xlsx"},
		"parquet_writer":     {"parquet", "parquet_select"},
		"fixed_width_writer": {"fixed_width"},
	}
	for writer, fs := range formats {
		for _, f := range fs {
			if got, ok := MergedFileExtension(f); !ok || got != byWriter[writer] {
				t.Errorf("MergedFileExtension(%q) = %q, %v; the partition writer gives %q", f, got, ok, byWriter[writer])
			}
		}
	}
}

// Asking for the extension of a format with none is refused rather than
// producing a name that ends in a dot.
func TestMergedFileNameRefusesAFormatWithNoExtension(t *testing.T) {
	_, err := MergedFileName("member.${FILE_EXTENSION}", "json", nil)
	if err == nil || !strings.Contains(err.Error(), "has no file extension") {
		t.Fatalf("want a refusal naming the format, got %v", err)
	}
}
