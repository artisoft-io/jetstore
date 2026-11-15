package dblogin

import (
	"strings"
	"testing"
)

// The per-schema grants Ensure and GrantSchema share: IDE may create tables in
// every schema but jetsapi, and a schema name is quoted.
func TestSchemaGrants(t *testing.T) {
	ide := strings.Join(ideSchemaGrants("jetsapi"), "\n")
	if !strings.Contains(ide, `REVOKE CREATE ON SCHEMA "jetsapi" FROM "jets_ide"`) || strings.Contains(ide, "GRANT CREATE") {
		t.Errorf("ideSchemaGrants(jetsapi):\n%s", ide)
	}
	ide = strings.Join(ideSchemaGrants(`odd"name`), "\n")
	if !strings.Contains(ide, `GRANT CREATE ON SCHEMA "odd""name" TO "jets_ide"`) {
		t.Errorf("ideSchemaGrants(odd\"name):\n%s", ide)
	}
	reader := readerSchemaGrants("added")
	if len(reader) != 2 || reader[0] != `GRANT USAGE ON SCHEMA "added" TO "jets_reader"` ||
		reader[1] != `GRANT SELECT ON ALL TABLES IN SCHEMA "added" TO "jets_reader"` {
		t.Errorf("readerSchemaGrants(added) = %q", reader)
	}
}
