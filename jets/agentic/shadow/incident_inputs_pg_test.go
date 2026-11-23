package shadow

// The inputs an incident's session registered, against a real Postgres: the
// file key a feed delivery was registered under, a domain table listed after
// it, and nothing for a session that registered nothing.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run IncidentInputs ./jets/agentic/shadow/

import (
	"context"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
)

func TestIncidentInputsReadTheSessionsRegistrations(t *testing.T) {
	pool := freshDB(t, "shadow_incident_inputs", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	d := feedfixture.Feeds()[0].Deliveries[0]

	got, err := audit.IncidentInputs(ctx, pool, d.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].FileKey != d.FileKey || got[0].SourceType != "file" {
		t.Fatalf("inputs of %s = %+v, want the one file %s", d.SessionID, got, d.FileKey)
	}

	// A domain table registered under the same session sorts after the file.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.input_registry
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
		SELECT client, org, object_type, 'a_domain_table', source_period_key, 'a_domain_table', 'domain_table', session_id, 'system', last_update
		  FROM jetsapi.input_registry WHERE session_id = $1`, d.SessionID); err != nil {
		t.Fatal(err)
	}
	got, err = audit.IncidentInputs(ctx, pool, d.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SourceType != "file" || got[1].SourceType != "domain_table" ||
		got[1].FileKey != "a_domain_table" {
		t.Fatalf("inputs = %+v, want the file then the domain table", got)
	}

	got, err = audit.IncidentInputs(ctx, pool, "no-such-session")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("a session with no registration: %+v, %v; want an empty list", got, err)
	}
}
