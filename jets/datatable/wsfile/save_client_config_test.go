package wsfile

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
)

// The two lists getColumns returns must differ in exactly one way: an array
// column carries a `::text` cast in the select list and must not carry one in
// the INSERT column list. A single list served both until 2026-09-12, and every
// export of a table with an array column was an init script PostgreSQL refuses
// to parse.
func TestGetColumnsSeparatesSelectFromInsert(t *testing.T) {
	jetsSchema := map[string]schema.TableDefinition{
		"source_config": {
			TableName: "source_config",
			Columns: []schema.ColumnDefinition{
				{ColumnName: "key", DataType: "int"},
				{ColumnName: "object_type", DataType: "text"},
				{ColumnName: "domain_keys", DataType: "text", IsArray: true},
				{ColumnName: "user_email", DataType: "text"},
				{ColumnName: "last_update", DataType: "datetime"},
			},
		},
	}
	selectList, columnNames, err := getColumns(&jetsSchema, true, "source_config")
	if err != nil {
		t.Fatalf("getColumns: %v", err)
	}
	wantSelect := "object_type,domain_keys::text,user_email"
	wantColumns := "object_type,domain_keys,user_email"
	if got := strings.Join(selectList, ","); got != wantSelect {
		t.Errorf("select list = %q, want %q", got, wantSelect)
	}
	if got := strings.Join(columnNames, ","); got != wantColumns {
		t.Errorf("column names = %q, want %q", got, wantColumns)
	}
	// skipKeyColumn false keeps `key`; `last_update` is never exported.
	selectList, columnNames, err = getColumns(&jetsSchema, false, "source_config")
	if err != nil {
		t.Fatalf("getColumns: %v", err)
	}
	if got := strings.Join(columnNames, ","); got != "key,object_type,domain_keys,user_email" {
		t.Errorf("column names = %q", got)
	}
	if strings.Contains(strings.Join(columnNames, ","), "::") {
		t.Errorf("an INSERT column list must carry no cast: %q", columnNames)
	}
	if !strings.Contains(strings.Join(selectList, ","), "domain_keys::text") {
		t.Errorf("the select list lost its cast: %q", selectList)
	}
}

func TestGetColumnsUnknownTable(t *testing.T) {
	jetsSchema := map[string]schema.TableDefinition{}
	if _, _, err := getColumns(&jetsSchema, true, "nope"); err == nil {
		t.Error("want an error for a table the schema does not define")
	}
}
