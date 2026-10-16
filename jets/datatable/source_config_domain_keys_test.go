package datatable

import (
	"slices"
	"testing"
)

// The delete is not a domain-keys write: its row is the screen's whole state,
// and the hook used to panic on it (2026-10-16).
func TestWritesDomainKeysNamesTheInsertAndTheUpdateOnly(t *testing.T) {
	for table, want := range map[string]bool{
		"source_config":        true,
		"update/source_config": true,
		"delete/source_config": false,
	} {
		if got := writesDomainKeys(table); got != want {
			t.Errorf("writesDomainKeys(%q) = %v, want %v", table, got, want)
		}
	}
}

func TestPopulateDomainKeys(t *testing.T) {
	cases := []struct {
		name string
		json any
		want []string
	}{
		{"absent", nil, []string{"PharmacyClaim"}},
		{"a string", `"PharmacyClaim"`, []string{"PharmacyClaim"}},
		{"an array", `["a", "b"]`, []string{"PharmacyClaim"}},
		{"an object, less the instruction", `{"Eligibility":"Member_ID","jets:hashing_override":"none"}`, []string{"Eligibility"}},
	}
	for _, c := range cases {
		row := map[string]any{"object_type": "PharmacyClaim", "domain_keys_json": c.json}
		if err := populateDomainKeys(row); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := row["domain_keys"].([]string)
		slices.Sort(got)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: domain_keys = %v, want %v", c.name, got, c.want)
		}
	}
}

// The shape of the request that panicked: a selected row's columns arrive as
// single-element arrays. It is refused with an error rather than a panic.
func TestPopulateDomainKeysRefusesArrayValues(t *testing.T) {
	rows := []map[string]any{
		{"object_type": []any{"PharmacyClaim"}, "domain_keys_json": []any{`{"Eligibility":["Member First Name"]}`}},
		{"object_type": "PharmacyClaim", "domain_keys_json": []any{`{"Eligibility":["Member First Name"]}`}},
	}
	for i, row := range rows {
		if err := populateDomainKeys(row); err == nil {
			t.Errorf("row %d: want an error, got domain_keys %v", i, row["domain_keys"])
		}
	}
}
