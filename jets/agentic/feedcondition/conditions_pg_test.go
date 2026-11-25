package feedcondition_test

// A read-only report of a database's feed conditions, for a database where
// incidents were written one per delivery before conditions, and must not be
// changed: it shows what those incidents are as conditions.
//
// It runs only when JETS_FEED_CONDITIONS_DSN names a database, and reads it in
// a READ ONLY transaction, so no write can reach it. It reads four tables:
// jetsapi.anomaly, input_registry (session_id, file_key, table_name,
// source_type, key), incident and incident_event, so a copy of those four is
// enough.
//
//	JETS_FEED_CONDITIONS_DSN=postgres://user:pw@localhost:5432/copy \
//	  go test -count=1 -v -run ConditionsOfADatabase ./jets/agentic/feedcondition/
//
// The log holds one JSON line per feed and one for the totals.

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// conditionRow is one condition as the report prints it.
type conditionRow struct {
	Locus      string   `json:"locus"`
	Deliveries int      `json:"deliveries"`
	First      string   `json:"firstSession"`
	Last       string   `json:"lastSession"`
	Open       bool     `json:"open"`
	End        string   `json:"end,omitempty"`
	Detectors  []string `json:"detectors"`
	Incidents  []string `json:"incidents"`
	Verdicts   []string `json:"verdicts"`
}

type feedReport struct {
	Feed       string         `json:"feed"`
	Conditions []conditionRow `json:"conditions"`
}

type totals struct {
	Feeds      int `json:"feeds"`
	Conditions int `json:"conditions"`
	// ByDetector counts the conditions each detector fired in; a condition
	// whose deliveries two detectors fired on counts under both.
	ByDetector map[string]int `json:"conditionsByDetector"`
	ByLocus    map[string]int `json:"conditionsByLocus"`
	// IncidentsOnConditions counts the incidents on conditions' deliveries, and
	// ConditionsBy the conditions by how many incidents sit on them.
	IncidentsOnConditions int         `json:"incidentsOnConditions"`
	ConditionsBy          map[int]int `json:"conditionsByIncidentCount"`
}

func TestConditionsOfADatabase(t *testing.T) {
	dsn := os.Getenv("JETS_FEED_CONDITIONS_DSN")
	if dsn == "" {
		t.Skip("JETS_FEED_CONDITIONS_DSN not set; this is a report over a named database, not a test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	feeds, err := feedcondition.ReadAll(ctx, tx, nil)
	if err != nil {
		t.Fatal(err)
	}
	tot := totals{Feeds: len(feeds), ByDetector: map[string]int{}, ByLocus: map[string]int{}, ConditionsBy: map[int]int{}}
	for _, f := range feeds {
		r := feedReport{Feed: f.Feed}
		for i := range f.Conditions {
			c := &f.Conditions[i]
			row := conditionRow{Locus: c.Locus, Deliveries: len(c.Members), First: c.First().SessionId,
				Last: c.Last().SessionId, Open: c.Open(), End: c.End, Incidents: []string{}, Verdicts: []string{}}
			seen := map[string]bool{}
			for _, m := range c.Members {
				for _, d := range m.Detectors {
					if !seen[d] {
						seen[d] = true
						row.Detectors = append(row.Detectors, d)
						tot.ByDetector[d]++
					}
				}
			}
			sort.Strings(row.Detectors)
			for _, inc := range f.IncidentsOn(c) {
				row.Incidents = append(row.Incidents, inc.IncidentId)
				if inc.Verdict != nil {
					row.Verdicts = append(row.Verdicts, inc.IncidentId+": "+inc.Verdict.ToStatus)
				}
			}
			tot.Conditions++
			tot.ByLocus[c.Locus]++
			tot.IncidentsOnConditions += len(row.Incidents)
			tot.ConditionsBy[len(row.Incidents)]++
			r.Conditions = append(r.Conditions, row)
		}
		b, _ := json.Marshal(r)
		t.Log(string(b))
	}
	b, _ := json.Marshal(tot)
	t.Log(string(b))
}
