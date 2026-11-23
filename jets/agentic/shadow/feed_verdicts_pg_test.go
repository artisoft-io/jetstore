package shadow

// A person's verdict on a shadow incident, against a real Postgres: the feed
// fixture's incidents are raised at diagnosed, a person suppresses one and
// reclassifies another, and the record reads them back as verdicts while the
// shadow-mode attestation still holds.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run PersonsVerdict ./jets/agentic/shadow/

import (
	"context"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
)

func TestAPersonsVerdictOnADiagnosedShadowIncident(t *testing.T) {
	pool := freshDB(t, "shadow_feed_verdicts", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if detectFeeds(t, pool) == 0 {
		t.Fatal("the detectors wrote no anomaly")
	}
	sel, err := SelectSessions(ctx, pool, feedfixture.Clock.Add(-time.Hour), "", "", 1000, true, true)
	if err != nil {
		t.Fatal(err)
	}
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	var written []string
	for _, s := range sel.Sessions {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
		for _, wr := range res.Written {
			if wr.Status != audit.IncidentDiagnosed {
				t.Fatalf("%s is at %s; shadow triage raises feed incidents at diagnosed", wr.IncidentId, wr.Status)
			}
			written = append(written, wr.IncidentId)
		}
	}
	if len(written) < 2 {
		t.Fatalf("%d incidents written, want at least two", len(written))
	}

	// An agent may not give the verdict.
	if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
		IncidentEventId: "ie_agent_verdict", IncidentRef: written[0],
		FromStatus: audit.IncidentDiagnosed, ToStatus: audit.IncidentSuppressedAsBenign,
		Actor: "shadow_triage", ActorKind: audit.ActorAgent, Rationale: "self-assessed",
	}); err == nil {
		t.Fatal("an agent suppressed a diagnosed incident")
	}

	// A person may: a false positive, and an agreement recorded as a reclassification.
	since := time.Now().Add(-time.Minute)
	for i, tr := range []*audit.IncidentTransition{
		{IncidentEventId: "ie_person_benign", IncidentRef: written[0],
			FromStatus: audit.IncidentDiagnosed, ToStatus: audit.IncidentSuppressedAsBenign,
			Actor: "reviewer@example.com", ActorKind: audit.ActorHuman,
			Rationale: "the feed's ordinary span is wider than declared"},
		{IncidentEventId: "ie_person_agrees", IncidentRef: written[1],
			FromStatus: audit.IncidentDiagnosed, ToStatus: audit.IncidentReclassified,
			Actor: "reviewer@example.com", ActorKind: audit.ActorHuman,
			ClassificationAfter: "source_content_change", Rationale: "the file did repeat"},
	} {
		if _, err := audit.RecordIncidentTransition(ctx, pool, tr); err != nil {
			t.Fatalf("verdict %d: %v", i, err)
		}
	}
	verdicts, err := audit.HumanVerdicts(ctx, pool, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(verdicts) != 2 {
		t.Errorf("%d human verdicts read back, want 2", len(verdicts))
	}
	a, err := Attest(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Holds() {
		t.Errorf("a person's verdicts broke the shadow-mode attestation:\n%s", a.Report())
	}
}
