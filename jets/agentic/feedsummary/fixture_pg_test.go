package feedsummary

// The summary of every fixture incident, end to end against a real Postgres:
// the synthetic feed fixture is installed, the eight feed detectors run as
// library calls, shadow triage writes the incidents, and every incident is
// read back and summarised the way the incident read does it.
//
//	docker run -d --rm --name feedsummary-pg -e POSTGRES_PASSWORD=pw -p 55917:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55917/postgres \
//	  go test -count=1 -run Fixture ./jets/agentic/feedsummary/
//
// Run it with -count=1: the fixture reads jets_schema.json, which the test
// cache does not see.
//
// The model arm runs only when JETS_FEED_SUMMARY_MODEL names a model and an
// inference endpoint answers at JETS_FEED_SUMMARY_HOST (default
// http://localhost:11434). Its drafts are logged and counted, never shown
// anywhere else.

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaPath = "../../jets_schema.json"

// executionTables are what shadow triage reads beside the anomalies.
var executionTables = []string{
	"pipeline_execution_status", "pipeline_execution_details",
	"pipeline_execution_channel_details", "process_errors", "cpipes_execution_status",
}

func freshDB(t *testing.T, dbName string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{"DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)", "CREATE DATABASE " + dbName} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			admin.Close()
			t.Fatal(err)
		}
	}
	admin.Close()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + dbName
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f, err := os.Open(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		if slices.Contains(executionTables, defs[i].TableName) {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// detectFeeds runs the eight feed detectors over the installed fixture as of
// its clock and writes their anomalies, as the shadow package's end-to-end
// test does.
func detectFeeds(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	ctx := context.Background()
	var profiled, unprofiled []feedprofile.Feed
	clients := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		if clients[f.Client] {
			continue
		}
		clients[f.Client] = true
		p, u, err := feedprofile.Load(ctx, pool, f.Client)
		if err != nil {
			t.Fatal(err)
		}
		profiled, unprofiled = append(profiled, p...), append(unprofiled, u...)
	}
	in, err := observe.ReadFeedDeliveryInput(ctx, pool, profiled, unprofiled, feedfixture.Clock)
	if err != nil {
		t.Fatal(err)
	}
	written := 0
	for _, d := range []interface {
		Detect(*observe.FeedDeliveryInput) (observe.FeedDeliveryResult, error)
	}{observe.DefaultFeedOverdue(), observe.DefaultDeliverySize(), observe.DefaultDeliveryBackdated()} {
		res, err := d.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		n, err := observe.FeedDeliveryWrite(ctx, pool, res)
		if err != nil {
			t.Fatal(err)
		}
		written += n
	}
	o := observe.DefaultClaimSetOptions()
	o.Now = feedfixture.Clock
	verdicts, err := observe.DetectClaimSets(ctx, pool, append(slices.Clone(profiled), unprofiled...), o)
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := observe.WriteClaimSetAnomalies(ctx, pool, verdicts)
	if err != nil {
		t.Fatal(err)
	}
	return written + n
}

// fixtureIncidents installs the fixture, detects, triages, and returns every
// incident written, read back.
func fixtureIncidents(t *testing.T, dbName string) (*pgxpool.Pool, []*audit.Incident) {
	t.Helper()
	pool := freshDB(t, dbName)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	n := detectFeeds(t, pool)
	t.Logf("the detectors wrote %d anomalies", n)
	sessions, err := shadow.RecentFeedSessions(ctx, pool, feedfixture.Clock.Add(-time.Hour), "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	w := shadow.DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return feedfixture.Clock }
	for _, s := range sessions {
		if _, err := w.Run(ctx, pool, s); err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
	}
	rows, err := pool.Query(ctx, `SELECT incident_id FROM jetsapi.incident ORDER BY incident_id`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	var out []*audit.Incident
	for _, id := range ids {
		inc, err := audit.ReadIncident(ctx, pool, id, audit.RedactPHI)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, inc)
	}
	return pool, out
}

// TestEveryFixtureIncidentIsSummarised renders the summary of every fixture
// incident and checks that no member or key value reaches it: every fixture incident is a feed
// incident, is summarised, names its feed and its declaration's author, and
// carries no canary; the prompt the model arm would send carries none either;
// and the counter finds nothing untrue in any render.
func TestEveryFixtureIncidentIsSummarised(t *testing.T) {
	pool, incs := fixtureIncidents(t, "feedsummary_fixture")
	ctx := context.Background()
	if len(incs) == 0 {
		t.Fatal("no incident was written, so a summary of each proves nothing")
	}
	byLocus := map[string]int{}
	var held []int
	for _, inc := range incs {
		if !IsFeedIncident(inc.Locus) {
			t.Errorf("%s is at %s, which is not a delivery locus", inc.IncidentId, inc.Locus)
			continue
		}
		byLocus[inc.Locus]++
		in, err := Read(ctx, pool, inc)
		if err != nil {
			t.Errorf("%s: %v", inc.IncidentId, err)
			continue
		}
		s, err := Render(in)
		if err != nil {
			t.Errorf("%s: %v", inc.IncidentId, err)
			continue
		}
		if testing.Verbose() {
			t.Logf("%s\n%s\n", inc.IncidentId, s.Text)
		}
		if len(s.Violations) != 0 {
			t.Errorf("%s: %v", inc.IncidentId, s.Violations)
		}
		flat := strings.Join(strings.Fields(s.Text), " ")
		for _, want := range []string{in.Feed.Table, "The readings, best first:", "The case against every reading:"} {
			if !strings.Contains(flat, want) {
				t.Errorf("%s: the summary does not say %q:\n%s", inc.IncidentId, want, s.Text)
			}
		}
		if in.Declaration.Readable && !strings.Contains(flat, "declared by "+in.Declaration.DeclaredBy) {
			t.Errorf("%s: the summary does not name the declaration's author:\n%s", inc.IncidentId, s.Text)
		}
		if strings.Contains(flat, "could not be read back") {
			t.Errorf("%s: a finding's numbers could not be read back:\n%s", inc.IncidentId, s.Text)
		}
		if len(in.Readings) != len(inc.Hypotheses) {
			t.Errorf("%s: %d readings for %d hypotheses", inc.IncidentId, len(in.Readings), len(inc.Hypotheses))
		}
		if in.Condition == nil {
			t.Errorf("%s: no condition of its feed holds it", inc.IncidentId)
		} else {
			held = append(held, in.Condition.Held)
		}
		if c := CountUntrue(s.Text, in); c.Untrue() != 0 {
			t.Errorf("%s: the counter finds untrue statements in the render:\n%s\n%s", inc.IncidentId, c, s.Text)
		}
		system, user, err := Prompt(in)
		if err != nil {
			t.Fatal(err)
		}
		for _, canary := range feedfixture.Canaries() {
			if strings.Contains(s.Text, canary) {
				t.Errorf("%s: the summary carries the canary %q", inc.IncidentId, canary)
			}
			if strings.Contains(system, canary) || strings.Contains(user, canary) {
				t.Errorf("%s: the model arm's prompt carries the canary %q", inc.IncidentId, canary)
			}
		}
	}
	// Every (session, locus) with a feed finding is held by exactly one
	// incident's summary: the eight conditions of the fixture, of 8, 7, 6, 3,
	// 3, 1, 1 and 1 deliveries, computed here from the anomalies alone.
	slices.Sort(held)
	if want := []int{1, 1, 1, 3, 3, 6, 7, 8}; !slices.Equal(held, want) {
		t.Errorf("the summaries hold %v deliveries, want %v", held, want)
	}
	if n, sum := findingPositions(t, pool), sumOf(held); n != sum {
		t.Errorf("the summaries hold %d deliveries between them, and %d sessions carry a feed finding at a "+
			"delivery locus", sum, n)
	}
	t.Logf("%d incidents summarised: %v; the deliveries they hold: %v", len(incs), byLocus, held)

	// The canaries are in the database, so that their absence above is a
	// finding rather than an accident of an empty claim table.
	profiles, err := feedfixture.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	planted := 0
	for table, raw := range profiles {
		p, findings := feedprofile.Parse(raw)
		if len(findings) > 0 {
			t.Fatalf("the fixture profile of %s does not parse: %v", table, findings)
		}
		if p.ClaimSet == nil {
			continue
		}
		var n int
		q := "SELECT count(*) FROM " + pgx.Identifier{p.ClaimSet.Table}.Sanitize() + " WHERE " +
			pgx.Identifier{p.ClaimSet.MemberColumn}.Sanitize() + " LIKE '%' || $1 || '%'"
		if err := pool.QueryRow(ctx, q, feedfixture.MemberCanary).Scan(&n); err != nil {
			t.Fatal(err)
		}
		planted += n
	}
	if planted == 0 {
		t.Fatal("no claim row carries the member canary, so the canary check proves nothing")
	}
	t.Logf("%d claim rows carry the member canary", planted)
}

// TestTheModelArm drafts every fixture incident's summary with a model and
// counts each draft's untrue statements, with the render counted beside it.
// It runs only when a model is named and reachable.
func TestTheModelArm(t *testing.T) {
	model := os.Getenv("JETS_FEED_SUMMARY_MODEL")
	if model == "" {
		t.Skip("JETS_FEED_SUMMARY_MODEL not set; the model arm is not run")
	}
	host := os.Getenv("JETS_FEED_SUMMARY_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	pool, incs := fixtureIncidents(t, "feedsummary_arm")
	ctx := context.Background()
	d := InferDrafter{Client: &infer.Client{Host: host, Model: model, RequestTimeout: 30 * time.Minute,
		Options: map[string]any{"temperature": 0}}}
	drafts, untrueDrafts, untrue, renderUntrue := 0, 0, 0, 0
	byCat := map[string]int{}
	for _, inc := range incs {
		in, err := Read(ctx, pool, inc)
		if err != nil {
			t.Fatal(err)
		}
		s, err := Render(in)
		if err != nil {
			t.Fatal(err)
		}
		renderUntrue += CountUntrue(s.Text, in).Untrue()
		text, answered, c, err := DraftOne(ctx, d, in)
		if err != nil {
			t.Errorf("%s: %v", inc.IncidentId, err)
			continue
		}
		drafts++
		if c.Untrue() > 0 {
			untrueDrafts++
		}
		untrue += c.Untrue()
		for _, cat := range c.Categories {
			byCat[cat.Name+" ("+string(cat.Soundness)+")"] += len(cat.Findings)
		}
		t.Logf("%s, drafted by %s:\n%s\n%s", inc.IncidentId, answered, text, c)
	}
	t.Logf("model %s: %d of %d drafts carry a statement untrue of the fact set, %d statements in all; "+
		"the render: %d over %d incidents", model, untrueDrafts, drafts, untrue, renderUntrue, len(incs))
	for k, v := range byCat {
		t.Logf("  %-60s %d", k, v)
	}
}

// TestABurstIsOneIncident plants the case a backfill makes: the same check fires
// on two more deliveries of one feed registered the same day as a fixture
// incident's, just after it. Shadow triage raises neither: both join the
// incident of the condition they extend, written on its first delivery by an
// earlier run. The summary of that incident counts them among the deliveries it
// holds, and reading it changes nothing.
//
// Before conditions the summary read a burst, by detector and UTC day, and said
// the incident stood for the three deliveries of that day. A day's burst is a
// condition whose deliveries share a day, so the test now asks the condition's
// question: the incident holds two more deliveries than before, two of them
// sessions with no registration of the feed, and its summary says so.
func TestABurstIsOneIncident(t *testing.T) {
	pool, incs := fixtureIncidents(t, "feedsummary_burst")
	ctx := context.Background()
	var target *audit.Incident
	var base observe.Anomaly
	for _, inc := range incs {
		anoms, err := observe.ReadAnomalies(ctx, pool, inc.SessionId)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range anoms {
			if strings.HasPrefix(a.DetectorRef, feedprofile.DeliveryBackdated+"@") {
				target, base = inc, a
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		t.Fatal("no fixture incident carries a delivery_backdated finding")
	}
	in, err := Read(ctx, pool, target)
	if err != nil {
		t.Fatal(err)
	}
	if in.Condition == nil {
		t.Fatal("no condition holds the target incident")
	}
	heldBefore := in.Condition.Held
	ms, err := strconv.ParseInt(base.SessionId, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	var planted []string
	for k := int64(1); k <= 2; k++ {
		a := base
		a.SessionId = strconv.FormatInt(ms+k, 10)
		a.AnomalyId = base.AnomalyId + "/planted" + strconv.FormatInt(k, 10)
		if err := observe.InsertAnomaly(ctx, pool, &a); err != nil {
			t.Fatal(err)
		}
		planted = append(planted, a.SessionId)
	}
	w := shadow.DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return feedfixture.Clock }
	for _, s := range planted {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 || len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], target.IncidentId) {
			t.Errorf("session %s: written %v, grouped %v; want it grouped into %s",
				s, res.Written, res.Grouped, target.IncidentId)
		}
	}
	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident`).Scan(&before); err != nil {
		t.Fatal(err)
	}

	in, err = Read(ctx, pool, target)
	if err != nil {
		t.Fatal(err)
	}
	c := in.Condition
	if c == nil || c.Held != heldBefore+2 || c.Unregistered != 2 || c.Total != c.Held || len(c.Others) != 0 ||
		c.Unraised != 0 {
		t.Fatalf("the condition %+v, want %d deliveries held, two with no registration, and no other incident",
			c, heldBefore+2)
	}
	s, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(s.Text), " ")
	for _, want := range []string{"This incident stands for " + wordsOf(heldBefore+2) + " deliveries of the feed",
		"two of them are sessions with no registration of the feed", "raised as one incident: give it one verdict"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the summary does not say %q:\n%s", want, s.Text)
		}
	}
	if strings.Contains(flat, "belong to other incidents") {
		t.Errorf("the summary names other incidents where there are none:\n%s", s.Text)
	}
	if c := CountUntrue(s.Text, in); c.Untrue() != 0 {
		t.Errorf("the counter finds untrue statements in the render:\n%s", c)
	}
	var after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("reading the summary changed the incident count from %d to %d", before, after)
	}
	t.Logf("\n%s", s.Text)
}
