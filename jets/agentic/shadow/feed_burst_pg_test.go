package shadow

// One incident per burst, end to end against a real Postgres: the same check
// fires on several deliveries of one feed registered the same day, and shadow
// triage raises one incident, on the last of them.
//
//	docker run -d --rm --name shadow-feed-pg -e POSTGRES_PASSWORD=pw -p 55892:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run Burst ./jets/agentic/shadow/

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestABurstIsOneIncidentOnItsLastDelivery(t *testing.T) {
	pool := freshDB(t, "shadow_feed_burst", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	detectFeeds(t, pool)

	// A back-dating finding from the fixture, copied onto deliveries registered
	// the day after the fixture's clock, when the fixture registers nothing.
	var base *observe.Anomaly
	rows, err := pool.Query(ctx, `SELECT DISTINCT anomaly_session_id FROM jetsapi.anomaly
		WHERE anomaly_detector_ref LIKE $1 ORDER BY 1 LIMIT 1`, feedprofile.DeliveryBackdated+"@%")
	if err != nil {
		t.Fatal(err)
	}
	var baseSession string
	for rows.Next() {
		if err := rows.Scan(&baseSession); err != nil {
			t.Fatal(err)
		}
	}
	rows.Close()
	anoms, err := observe.ReadAnomalies(ctx, pool, baseSession)
	if err != nil {
		t.Fatal(err)
	}
	for i := range anoms {
		if strings.HasPrefix(anoms[i].DetectorRef, feedprofile.DeliveryBackdated+"@") {
			base = &anoms[i]
		}
	}
	if base == nil {
		t.Fatal("the fixture wrote no delivery_backdated anomaly")
	}
	day := feedfixture.Clock.Add(24 * time.Hour).Truncate(24 * time.Hour).UnixMilli()
	plant := func(k int64) string {
		a := *base
		a.SessionId = strconv.FormatInt(day+k*1000, 10)
		a.AnomalyId = base.AnomalyId + "/burst" + strconv.FormatInt(k, 10)
		if err := observe.InsertAnomaly(ctx, pool, &a); err != nil {
			t.Fatal(err)
		}
		return a.SessionId
	}
	burst := []string{plant(1), plant(2), plant(3)}
	locus := triage.LocusDeliveryDepartsProfile
	last := IncidentIdFor(burst[2], locus)

	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	for i, s := range burst {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
		if f := findingOf(res, locus); f.Verdict != triage.Present {
			t.Errorf("session %s: %s is %s, want present on every delivery of the burst", s, locus, f.Verdict)
		}
		if i < 2 {
			if len(res.Written) != 0 || len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], last) {
				t.Errorf("session %s: written %v, grouped %v; want grouped into %s", s, res.Written, res.Grouped, last)
			}
			continue
		}
		if len(res.Written) != 1 || res.Written[0].IncidentId != last || len(res.Grouped) != 0 {
			t.Errorf("the last delivery: written %v, grouped %v; want %s written", res.Written, res.Grouped, last)
		}
	}
	if got := incidentsOn(t, pool, burst, locus); !slices.Equal(got, []string{last}) {
		t.Errorf("incidents on the burst %v, want only %s", got, last)
	}
	trs, err := audit.IncidentTransitionsFor(ctx, pool, last)
	if err != nil {
		t.Fatal(err)
	}
	var triaged string
	for _, tr := range trs {
		if tr.ToStatus == audit.IncidentTriaged {
			triaged = tr.Rationale
		}
	}
	for _, want := range []string{"fired on 3 deliveries registered on", burst[0], burst[1],
		"this incident stands for all of them"} {
		if !strings.Contains(triaged, want) {
			t.Errorf("the incident's basis does not say %q:\n  %s", want, triaged)
		}
	}

	// A later delivery of the same day, triaged by a later run, leads the burst
	// now, and is raised as the incident already written rather than a second.
	late := plant(4)
	res, err := w.Run(ctx, pool, late)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 || len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], last) {
		t.Errorf("the later delivery: written %v, grouped %v; want grouped into %s", res.Written, res.Grouped, last)
	}
	if got := incidentsOn(t, pool, append(burst, late), locus); !slices.Equal(got, []string{last}) {
		t.Errorf("incidents on the burst after the later run %v, want only %s", got, last)
	}

	// A second run over the burst changes nothing.
	for _, s := range append(burst, late) {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 {
			t.Errorf("a second run over %s wrote %v", s, res.Written)
		}
	}
}

func findingOf(res *SessionResult, locus string) triage.Finding {
	for _, f := range res.Report.Findings {
		if f.Locus == locus {
			return f
		}
	}
	return triage.Finding{}
}

func incidentsOn(t *testing.T, pool *pgxpool.Pool, sessions []string, locus string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT incident_id FROM jetsapi.incident
		WHERE incident_session_id = ANY($1::text[]) AND incident_locus = $2 ORDER BY 1`, sessions, locus)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}
