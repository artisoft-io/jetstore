package shadow

import (
	"context"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/agentic/feedproposal"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// DraftFeedProposals is the advisory step after incidents are written: for a
// feed whose deliveries look cumulative under an incremental declaration, it
// records a change proposal in `draft` with the incident as its trigger.
//
// It sits below the ceiling for the same reason the writer does. A proposal in
// draft is a statement that a change could be made, not a remediation: it has
// no subject in jetsapi.remediation, it moves no incident, and nothing here
// moves it out of draft. The incident lifecycle's `remediation_proposed` is a
// status an incident is moved to, and this step moves none, so the
// attestation's questions are unchanged; Attest also counts the drafts and
// reports any incident-triggered proposal whose state no recorded decision
// explains.
//
// The feeds assessed are those carrying a feed anomaly on one of sessions, or
// every feed when sessions is empty; each is judged over its whole history,
// since the case is a run of deliveries rather than one. A dry run assesses
// and writes nothing.
func (w *Writer) DraftFeedProposals(ctx context.Context, db DB, sessions []string) (*feedproposal.Result, error) {
	o := feedproposal.DefaultOptions(w.ModelVersion)
	o.DryRun = w.DryRun
	var tables []string
	if len(sessions) > 0 {
		var err error
		if tables, err = feedsOfSessions(ctx, db, sessions); err != nil {
			return nil, err
		}
		if len(tables) == 0 {
			return &feedproposal.Result{}, nil
		}
	}
	return feedproposal.DraftAll(ctx, db, tables, o)
}

const feedsOfSessionsSQL = `SELECT DISTINCT anomaly_subject_ref FROM jetsapi.anomaly
 WHERE anomaly_subject_type = $1 AND anomaly_session_id = ANY($2::text[])
 ORDER BY 1`

func feedsOfSessions(ctx context.Context, db DB, sessions []string) ([]string, error) {
	rows, err := db.Query(ctx, feedsOfSessionsSQL, observe.SubjectFeed, sessions)
	if err != nil {
		return nil, fmt.Errorf("while reading the feeds of %d sessions: %w", len(sessions), err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("while scanning a feed: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
