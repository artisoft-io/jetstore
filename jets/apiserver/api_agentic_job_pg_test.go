package main

// A job's failure through the endpoint, against a real Postgres: the purge,
// which has no client, failing on three days is one incident at job_failed
// whose span is its condition's three runs, and the list and the page name
// the job.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run JobIncident ./jets/apiserver/

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

func TestAJobIncidentIsItsConditionAndNamesTheJob(t *testing.T) {
	pool := runConditionAPIDB(t, "apiserver_job_incident")
	ctx := context.Background()
	var sessions []string
	var starts []time.Time
	for i := range 3 {
		s := fmt.Sprintf("purge-job-%d", i+1)
		start := runConditionStart.Add(time.Duration(i) * 24 * time.Hour)
		// A job's failure row: no configuration, no input session, no client.
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, merged_input_registry_keys, client, process_name, main_object_type,
			 session_id, source_period_key, status, failure_details, failure_class, failure_source,
			 user_email, start_time, last_update)
			VALUES (0, '{}', '', 'purge_data', '', $1, 1, 'failed', 'the purge failed', 'job_failed', 'job',
			        'system', $2, $2)`, s, start); err != nil {
			t.Fatal(err)
		}
		sessions, starts = append(sessions, s), append(starts, start)
	}
	triageRuns(t, pool, sessions...)
	id := shadow.IncidentIdFor(sessions[0], triage.LocusJobFailed)

	page := listIncidentsPage(t, pool, AgenticAction{})
	if n := int(page["total"].(float64)); n != 1 {
		t.Fatalf("three failures of one job raise %d incidents, want 1: %v", n, incidentIds(page))
	}
	row := listedIncident(t, page, id)
	if row["locus"] != triage.LocusJobFailed || row["job"] != "purge_data" || row["count"].(float64) != 3 ||
		row["firstSeen"] != rfc(starts[0]) || row["lastSeen"] != rfc(starts[2]) || row["fromRunCondition"] != true {
		t.Errorf("the job's incident on the list: %v", row)
	}

	inc := readIncident(t, pool, id)
	if inc["job"] != "purge_data" {
		t.Errorf("the job's incident page does not name the job: %v", inc["job"])
	}
	c, ok := inc["runCondition"].(map[string]any)
	if !ok {
		t.Fatalf("the job's incident carries no condition: %v", inc)
	}
	if c["runsHeld"].(float64) != 3 || c["process"] != "purge_data" || c["client"] != "" || c["open"] != true {
		t.Errorf("the job's condition: %v", c)
	}

	// A later failure's session finds the condition's incident.
	ms := findIncidents(t, pool, sessions[2])["matches"].([]any)
	if len(ms) != 1 || ms[0].(map[string]any)["incidentId"] != id ||
		ms[0].(map[string]any)["how"] != SessionIncidentCondition {
		t.Errorf("the third failure's session matches %v; want the condition's incident %s", ms, id)
	}
}
