package compute_pipes

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// columnsRow is a pgx.Row that fills Scan's destinations by the statement's SELECT
// list: destination i receives the value of the i-th selected column. So a Scan
// whose destinations are not in the SELECT list's order puts values in the wrong
// fields, which the test below sees.
type columnsRow struct {
	columns []string
	values  map[string]any
}

func (r columnsRow) Scan(dest ...any) error {
	if len(dest) != len(r.columns) {
		return fmt.Errorf("%d destinations for %d selected columns", len(dest), len(r.columns))
	}
	for i, d := range dest {
		v, ok := r.values[r.columns[i]]
		if !ok {
			return fmt.Errorf("no value for column %s", r.columns[i])
		}
		switch d := d.(type) {
		case *string:
			*d = v.(string)
		case *int:
			*d = v.(int)
		case *sql.NullString:
			*d = sql.NullString{String: v.(string), Valid: true}
		default:
			return fmt.Errorf("column %s: unexpected destination %T", r.columns[i], d)
		}
	}
	return nil
}

type columnsDB struct {
	values map[string]any
}

func (db columnsDB) QueryRow(_ context.Context, stmt string, _ ...any) pgx.Row {
	upper := strings.ToUpper(stmt)
	list := stmt[strings.Index(upper, "SELECT")+len("SELECT") : strings.Index(upper, "FROM")]
	var columns []string
	for _, c := range strings.Split(list, ",") {
		columns = append(columns, strings.TrimSpace(c))
	}
	return columnsRow{columns: columns, values: db.values}
}

// The sharding start reads process_config.state_machine_name, among the rest, from
// the one row queryPipelineConfig scans; validateNoRulesOnCpipesSM depends on it.
func TestQueryPipelineConfigReadsEachColumnIntoItsField(t *testing.T) {
	db := columnsDB{values: map[string]any{
		"ir.client": "client", "ir.org": "org", "ir.object_type": "object_type", "ir.source_period_key": 11,
		"ir.schema_provider_json": "{}", "ir.table_name": "table_name", "ir.source_type": "file",
		"pe.pipeline_config_key": 12, "pe.process_name": "a_process", "pe.input_session_id": "session",
		"pe.user_email": "user", "pc.main_rules": "pipes_config/p.pc.json", "pc.env_json": `{"a":"b"}`,
		"pc.state_machine_name": "cpipesSM", "sp.month_period": 1, "sp.week_period": 2, "sp.day_period": 3,
		"sp.year": 2026, "sp.month": 10, "sp.day": 22,
	}}
	r, err := queryPipelineConfig(context.Background(), db, 7)
	if err != nil {
		t.Fatal(err)
	}
	want := pipelineConfigRow{
		client: "client", org: "org", objectType: "object_type", sourcePeriodKey: 11,
		schemaProviderJson: "{}", tableName: "table_name", sourceType: "file",
		pipelineConfigKey: 12, processName: "a_process", inputSessionId: "session", operatorEmail: "user",
		cpipesConfigFN: sql.NullString{String: "pipes_config/p.pc.json", Valid: true},
		envJson:        sql.NullString{String: `{"a":"b"}`, Valid: true},
		stateMachine:   sql.NullString{String: "cpipesSM", Valid: true},
		monthPeriod:    1, weekPeriod: 2, dayPeriod: 3, year: 2026, month: 10, day: 22,
	}
	if r != want {
		t.Errorf("row %+v\nwant %+v", r, want)
	}
}
