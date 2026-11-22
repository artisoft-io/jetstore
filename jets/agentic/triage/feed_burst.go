package triage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// # One incident per burst
//
// A backfill registers many past files of one feed on one day, and a feed
// detector judges each as a delivery. When the files depart from the profile
// in the same way, the detector writes one anomaly per file, each on its own
// session, and each is a correct finding about one file. Together they are one
// event. Written one incident per session, a backfill of seventy weeks is
// seventy incidents for a person to judge.
//
// So the findings of one detector on one feed whose deliveries were registered
// on the same day form a burst, and the burst is raised once, on its last
// delivery's session: the latest registration of the day, which is the state
// the feed was left in. The other sessions are listed in that incident's basis
// as the evidence it stands for.
//
// **The verdict is not changed, only where the incident is written.** Every
// session of a burst still answers present at its delivery locus, because the
// anomaly is there. A finding whose anomalies all belong to bursts that another
// session leads carries GroupedInto, and the shadow writer writes no incident
// for it. A session that leads a burst, or carries an anomaly that belongs to
// no burst, is written as before.
//
// A delivery's registration day is the UTC day of its session id, read as the
// epoch milliseconds a registration session id is. A session id that is not a
// number has no arrival, as for the feed detectors, and its anomaly forms no
// burst.

// Burst is the burst one feed anomaly belongs to: the anomalies of the same
// detector on the same feed whose sessions were registered on the same UTC day.
type Burst struct {
	// Feed is the anomaly's subject: the feed's table name.
	Feed string
	// Detector is the anomaly's detector_ref, generation included.
	Detector string
	// Day is the UTC registration day.
	Day time.Time
	// Sessions are the burst's sessions in arrival order, the anomaly's own
	// included, each once.
	Sessions []string
}

// Grouped reports whether the burst holds more than one delivery.
func (b *Burst) Grouped() bool { return b != nil && len(b.Sessions) > 1 }

// Lead is the burst's last session, which its incident is written on.
func (b *Burst) Lead() string {
	if b == nil || len(b.Sessions) == 0 {
		return ""
	}
	return b.Sessions[len(b.Sessions)-1]
}

// Others returns the burst's sessions other than one.
func (b *Burst) Others(session string) []string {
	var out []string
	for _, s := range b.Sessions {
		if s != session {
			out = append(out, s)
		}
	}
	return out
}

// describe renders the burst for a basis.
func (b *Burst) describe() string {
	return fmt.Sprintf("%s on feed %s fired on %d deliveries registered on %s (UTC), sessions %s",
		b.Detector, b.Feed, len(b.Sessions), b.Day.Format(time.DateOnly), strings.Join(b.Sessions, ", "))
}

// dayOf is the UTC day of a numeric session id, as epoch milliseconds, in SQL.
// The CASE guards the cast: Postgres does not promise to test the pattern before
// it casts.
const dayOf = `CASE WHEN %[1]s ~ '^[0-9]{1,18}$' THEN %[1]s::bigint / 86400000 END`

// feedBurstSQL returns, for each feed anomaly on the session, every session
// carrying an anomaly of the same detector on the same feed registered on the
// same UTC day, its own included, in arrival order.
var feedBurstSQL = `SELECT m.anomaly_id, m.anomaly_subject_ref, m.anomaly_detector_ref,
       ` + fmt.Sprintf(dayOf, "m.anomaly_session_id") + `, b.anomaly_session_id
  FROM jetsapi.anomaly m
  JOIN jetsapi.anomaly b
    ON b.anomaly_subject_type = m.anomaly_subject_type
   AND b.anomaly_subject_ref = m.anomaly_subject_ref
   AND b.anomaly_detector_ref = m.anomaly_detector_ref
   AND ` + fmt.Sprintf(dayOf, "b.anomaly_session_id") + ` = ` + fmt.Sprintf(dayOf, "m.anomaly_session_id") + `
 WHERE m.anomaly_session_id = $1 AND m.anomaly_subject_type = $2
 GROUP BY m.anomaly_id, m.anomaly_subject_ref, m.anomaly_detector_ref, m.anomaly_session_id, b.anomaly_session_id
 ORDER BY m.anomaly_id, b.anomaly_session_id::bigint`

// ReadFeedBursts returns the burst of each feed anomaly on the session, by
// anomaly id. An anomaly whose session id is not a number has none.
func ReadFeedBursts(ctx context.Context, db observe.DB, sessionId string) (map[string]*Burst, error) {
	rows, err := db.Query(ctx, feedBurstSQL, sessionId, observe.SubjectFeed)
	if err != nil {
		return nil, fmt.Errorf("while reading the bursts of the feed anomalies on session %q: %w", sessionId, err)
	}
	defer rows.Close()
	out := map[string]*Burst{}
	for rows.Next() {
		var id, feed, detector, peer string
		var day int64
		if err := rows.Scan(&id, &feed, &detector, &day, &peer); err != nil {
			return nil, fmt.Errorf("while scanning a burst of session %q: %w", sessionId, err)
		}
		b := out[id]
		if b == nil {
			b = &Burst{Feed: feed, Detector: detector, Day: time.UnixMilli(day * 86400000).UTC()}
			out[id] = b
		}
		b.Sessions = append(b.Sessions, peer)
	}
	return out, rows.Err()
}

// BurstOf returns the burst a feed anomaly on the session belongs to, or nil.
func (f *FeedEvidence) BurstOf(anomalyId string) *Burst {
	if f == nil {
		return nil
	}
	return f.Bursts[anomalyId]
}
