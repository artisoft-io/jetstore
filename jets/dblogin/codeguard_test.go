package dblogin

import (
	"slices"
	"strings"
	"testing"
)

// The commands the guard must refuse, whatever else it refuses.
func TestRefusedCommandsCoverCodeCarryingDDL(t *testing.T) {
	for _, tag := range []string{
		"CREATE FUNCTION", "ALTER FUNCTION", "CREATE PROCEDURE", "ALTER PROCEDURE",
		"CREATE TRIGGER", "ALTER TRIGGER", "CREATE POLICY", "ALTER POLICY", "CREATE RULE",
	} {
		if !slices.Contains(RefusedCommands, tag) {
			t.Errorf("RefusedCommands lacks %s", tag)
		}
	}
	// The IDE login's work: none of it may be refused by tag.
	for _, tag := range []string{"CREATE TABLE", "ALTER TABLE", "CREATE INDEX", "DROP TABLE", "CREATE VIEW", "CREATE TABLE AS"} {
		if slices.Contains(RefusedCommands, tag) {
			t.Errorf("RefusedCommands refuses %s", tag)
		}
	}
}

// What a default or a view needs to number or stamp a row stays allowed, and
// the functions that run SQL or change something beyond the row do not.
func TestExpressionFunctionLists(t *testing.T) {
	for _, f := range []string{"nextval", "gen_random_uuid", "clock_timestamp", "random"} {
		if !slices.Contains(allowedVolatile, f) {
			t.Errorf("allowedVolatile lacks %s", f)
		}
	}
	for _, f := range []string{"set_config", "setval", "query_to_xml", "pg_terminate_backend", "lo_import", "pg_advisory_lock"} {
		if slices.Contains(allowedVolatile, f) {
			t.Errorf("allowedVolatile allows %s", f)
		}
	}
	for _, f := range []string{"table_to_xml", "schema_to_xml", "database_to_xml"} {
		if !slices.Contains(sqlRunners, f) {
			t.Errorf("sqlRunners lacks %s, which is stable and reads a table as the caller", f)
		}
	}
}

func TestGuardFunctionSQL(t *testing.T) {
	sql := guardFunctionSQL()
	if n := strings.Count(sql, "$jets_guard$"); n != 2 {
		t.Errorf("the body's delimiter appears %d times, want 2", n)
	}
	if strings.Contains(sql, "%!") {
		t.Errorf("a format verb went wrong:\n%s", sql)
	}
	for _, tag := range RefusedCommands {
		if !strings.Contains(sql, sqlText(tag)) {
			t.Errorf("the guard does not refuse %s", tag)
		}
	}
	// Every reference into the catalog is qualified or under the function's
	// own search_path, which the IDE login cannot change.
	if !strings.Contains(sql, "SET search_path = pg_catalog, pg_temp") {
		t.Error("the guard does not pin its search_path")
	}
	// It must recognise the IDE login and its group, and not the apiserver's
	// user, which is a member of the group but not named for it.
	if !strings.Contains(sql, `LIKE 'jets\_ide\_%'`) || !strings.Contains(sql, "rolname = 'jets_ide'") {
		t.Errorf("the guard does not recognise the IDE login:\n%s", ideRoleTest("r"))
	}
	if strings.Contains(sql, "pg_has_role") {
		t.Error("pg_has_role fails when a role is missing, and the guard runs on every DDL statement")
	}
}

func TestGuardVersionFollowsTheDefinition(t *testing.T) {
	v := guardVersion()
	if v != guardVersion() || !strings.HasPrefix(v, "jetstore code guard ") {
		t.Fatalf("guardVersion = %q", v)
	}
	saved := RefusedCommands
	t.Cleanup(func() { RefusedCommands = saved })
	RefusedCommands = append(slices.Clone(saved), "CREATE SOMETHING")
	if guardVersion() == v {
		t.Error("guardVersion does not change with the definition")
	}
}

func TestQueriesFormatCleanly(t *testing.T) {
	for name, q := range map[string]string{"callsQuery": callsQuery("rels", "types"), "ideCodeQuery": ideCodeQuery()} {
		if strings.Contains(q, "%!") || strings.Contains(q, "%%") {
			t.Errorf("%s: a format verb went wrong:\n%s", name, q)
		}
	}
}

func TestSQLTextArray(t *testing.T) {
	if got, want := sqlTextArray([]string{"a", "it's"}), `ARRAY['a', 'it''s']::text[]`; got != want {
		t.Errorf("sqlTextArray = %s, want %s", got, want)
	}
}

func TestCodeGuardSwitch(t *testing.T) {
	for value, off := range map[string]bool{"off": true, "OFF": true, " Off ": true, "": false, "on": false, "0": false, "false": false, "no": false} {
		t.Setenv(CodeGuardSwitch, value)
		if got := codeGuardSwitchedOff(); got != off {
			t.Errorf("%s=%q: switched off = %v, want %v", CodeGuardSwitch, value, got, off)
		}
	}
}
