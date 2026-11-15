package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The names that were mapped before the list closed keep their mapping.
func TestToPgTypeKnownNamesUnchanged(t *testing.T) {
	for dt, want := range map[string]string{
		"int": "integer", "bool": "integer", "int32": "integer",
		"uint": "bigint", "long": "bigint", "ulong": "bigint", "int64": "bigint", "uint64": "bigint", "uint32": "bigint",
		"double":   "double precision",
		"resource": "text", "volatile_resource": "text", "text": "text", "string": "text",
		"date":     "date",
		"datetime": "timestamp without time zone",
		// passed through as written before, and kept
		"jsonb": "jsonb", "timestamptz": "timestamptz", "bytea": "bytea", "boolean": "boolean",
		"double precision": "double precision",
	} {
		got, err := ToPgType(dt)
		if err != nil || got != want {
			t.Errorf("ToPgType(%q) = %q, %v; want %q", dt, got, err, want)
		}
	}
}

// What ToPgType returns maps to itself, so a definition carrying an already
// mapped type (compute pipes adds columns that way) is accepted.
func TestToPgTypeIsIdempotent(t *testing.T) {
	for _, dt := range PgTypeNames() {
		once, err := ToPgType(dt)
		if err != nil {
			t.Fatal(err)
		}
		twice, err := ToPgType(once)
		if err != nil || twice != once {
			t.Errorf("ToPgType(ToPgType(%q)) = %q, %v; want %q", dt, twice, err, once)
		}
	}
}

func TestToPgTypeRefusesTheRest(t *testing.T) {
	for _, dt := range []string{
		"",
		"text); UPDATE jetsapi.users SET user_email = 'x'; CREATE TABLE z (y text",
		"text[]",
		"TEXT",
		"numeric",
		"varchar(10)",
		"text DEFAULT 'x'",
		"integer NOT NULL",
	} {
		got, err := ToPgType(dt)
		if err == nil {
			t.Errorf("ToPgType(%q) = %q; want an error", dt, got)
			continue
		}
		if !strings.Contains(err.Error(), "must be one of") {
			t.Errorf("ToPgType(%q): %v; want the accepted names listed", dt, err)
		}
	}
}

func TestPgTypeNamesSorted(t *testing.T) {
	names := PgTypeNames()
	if !slices.IsSorted(names) || len(names) != len(pgTypes) {
		t.Errorf("PgTypeNames() = %v", names)
	}
}

// Every type jets_schema.json names is in the list: the system tables are
// migrated through the same function.
func TestToPgTypeCoversTheSystemSchema(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "jets_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var defs []TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	for _, def := range defs {
		for _, col := range def.Columns {
			if _, err := ToPgType(col.DataType); err != nil {
				t.Errorf("%s.%s column %s: %v", def.SchemaName, def.TableName, col.ColumnName, err)
			}
		}
	}
}
