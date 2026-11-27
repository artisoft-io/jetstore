package main

// A run incident's main input source through the endpoint, against a real
// Postgres: two vendors' runs of one process, interleaved and all failing, are
// two conditions and two incidents, and the page of each names its source and
// its condition's runs, which are its own vendor's alone.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5461/postgres \
//	  go test -count=1 -run MainInputSource ./jets/apiserver/

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

func installInputRegistry(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	f, err := os.Open("../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		if defs[i].TableName == "input_registry" {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("installing input_registry: %v", err)
			}
			return
		}
	}
	t.Fatal("no input_registry in jets_schema.json")
}

func TestARunIncidentNamesItsMainInputSource(t *testing.T) {
	pool := runConditionAPIDB(t, "apiserver_main_input_source")
	installInputRegistry(t, pool)
	ctx := context.Background()
	var sessions []string
	org := map[string]string{}
	for i := range 4 {
		vendor := []string{"vendor_a", "vendor_b"}[i%2]
		s := fmt.Sprintf("loader-%s-%d", vendor, i/2+1)
		plantProcessRun(t, pool, "client2", "loader", s, runConditionStart.Add(time.Duration(i)*time.Hour), nil)
		var irKey int
		if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry
			(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
			VALUES ('client2', $1, 'claims', $2, 1, 'claims_table', 'file', $3, 'test@test') RETURNING key`,
			vendor, "in/"+s, "reg-"+s).Scan(&irKey); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_execution_status SET main_input_registry_key = $1
			WHERE session_id = $2`, irKey, s); err != nil {
			t.Fatal(err)
		}
		sessions, org[s] = append(sessions, s), vendor
	}
	triageRuns(t, pool, sessions...)

	page := listIncidentsPage(t, pool, AgenticAction{})
	for _, first := range []string{"loader-vendor_a-1", "loader-vendor_b-1"} {
		id := shadow.IncidentIdFor(first, triage.LocusWorkerFailed)
		row := listedIncident(t, page, id)
		if row["count"].(float64) != 2 || row["fromRunCondition"] != true {
			t.Errorf("%s on the list: %v; want its vendor's two runs", id, row)
		}
		inc := readIncident(t, pool, id)
		src, ok := inc["mainInputSource"].(map[string]any)
		if !ok || src["client"] != "client2" || src["org"] != org[first] || src["objectType"] != "claims" {
			t.Errorf("%s's main input source: %v", id, inc["mainInputSource"])
		}
		c, ok := inc["runCondition"].(map[string]any)
		if !ok {
			t.Fatalf("%s carries no run condition: %v", id, inc)
		}
		if cs, _ := c["source"].(map[string]any); cs == nil || cs["org"] != org[first] || c["runsHeld"].(float64) != 2 {
			t.Errorf("%s's run condition: %v", id, c)
		}
	}
	// The second vendor A run's session finds vendor A's incident.
	ms := findIncidents(t, pool, "loader-vendor_a-2")["matches"].([]any)
	want := shadow.IncidentIdFor("loader-vendor_a-1", triage.LocusWorkerFailed)
	found := false
	for _, m := range ms {
		if m.(map[string]any)["incidentId"] == want {
			found = true
		}
	}
	if !found {
		t.Errorf("loader-vendor_a-2 matches %v; want %s", ms, want)
	}
}
