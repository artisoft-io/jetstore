package retention

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// Purged is what can be said of an incident the database does not hold.
type Purged struct {
	IncidentId string
	// SessionId is the session the incident's id names, the one it was
	// written on.
	SessionId string
	// Deleted reports that the database records that session's deletion: a
	// deletion row, or an archived run. Without it the session is merely
	// absent, which is also what a mistyped id looks like.
	Deleted bool
}

// Sentence says what became of the incident, for a screen.
func (p *Purged) Sentence() string {
	if p.Deleted {
		return fmt.Sprintf("Incident %s was purged: the session it was written on, %s, has been deleted, and no "+
			"delivery or run still held stands for it.", p.IncidentId, p.SessionId)
	}
	return fmt.Sprintf("Incident %s is not in the database, and neither is the session it names, %s: it was "+
		"purged once no session it stood for was held, or the link is mistyped.", p.IncidentId, p.SessionId)
}

// SessionOfIncidentId reads the session and locus an incident id names: an
// incident's id is inc_<session>_<locus>, followed at a coded locus by the
// code's slug. ok is false for an id of any other shape.
func SessionOfIncidentId(id string) (session, locus string, ok bool) {
	rest, found := strings.CutPrefix(id, "inc_")
	if !found {
		return "", "", false
	}
	best := -1
	for _, l := range audit.IncidentLoci {
		for from := 0; ; {
			i := strings.Index(rest[from:], "_"+l)
			if i < 0 {
				break
			}
			i += from
			end := i + 1 + len(l)
			if i > 0 && (end == len(rest) || rest[end] == '_') && (best < 0 || i < best) {
				best, locus = i, l
			}
			from = i + 1
		}
	}
	if best < 0 {
		return "", "", false
	}
	return rest[:best], locus, true
}

// ReadPurged says, for incident ids the database holds no incident under,
// which of them name a session it no longer holds either (SessionHeldSQL):
// incidents the retention sweep deleted, or ids of that shape that never
// named an incident. An id that is held, that names a held session, or that is
// not an incident's id is left out. It writes nothing.
func ReadPurged(ctx context.Context, db observe.DB, ids []string) ([]Purged, error) {
	var out []Purged
	sessions := map[string]string{}
	var named []string
	for _, id := range ids {
		if s, _, ok := SessionOfIncidentId(id); ok && !slices.Contains(named, id) {
			sessions[id] = s
			named = append(named, id)
		}
	}
	if len(named) == 0 {
		return nil, nil
	}
	var incident, anomaly, core, deletions, archive bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.incident') IS NOT NULL,
	       to_regclass('jetsapi.anomaly') IS NOT NULL,
	       to_regclass('jetsapi.session_registry') IS NOT NULL
	         AND to_regclass('jetsapi.pipeline_execution_status') IS NOT NULL
	         AND to_regclass('jetsapi.input_registry') IS NOT NULL,
	       to_regclass('jetsapi.session_deletion') IS NOT NULL,
	       to_regclass('jetsapi.pipeline_execution_status_archive') IS NOT NULL`).Scan(&incident, &anomaly, &core,
		&deletions, &archive); err != nil {
		return nil, fmt.Errorf("while asking which tables say an incident was purged: %w", err)
	}
	if !incident || !anomaly || !core {
		return nil, nil
	}
	deleted := `false`
	switch {
	case deletions && archive:
		deleted = `EXISTS (SELECT 1 FROM jetsapi.session_deletion d WHERE d.session_id = u.session_id AND d.status = 'done')
		    OR EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status_archive a WHERE a.session_id = u.session_id)`
	case deletions:
		deleted = `EXISTS (SELECT 1 FROM jetsapi.session_deletion d WHERE d.session_id = u.session_id AND d.status = 'done')`
	}
	ss := make([]string, len(named))
	for i, id := range named {
		ss[i] = sessions[id]
	}
	rows, err := db.Query(ctx, `SELECT u.incident_id, u.session_id, `+deleted+`
	  FROM unnest($1::text[], $2::text[]) AS u(incident_id, session_id)
	 WHERE NOT EXISTS (SELECT 1 FROM jetsapi.incident i WHERE i.incident_id = u.incident_id)
	   AND NOT `+SessionHeldSQL("u.session_id")+`
	 ORDER BY u.incident_id`, named, ss)
	if err != nil {
		return nil, fmt.Errorf("while asking whether %d incidents were purged: %w", len(named), err)
	}
	defer rows.Close()
	for rows.Next() {
		var p Purged
		if err := rows.Scan(&p.IncidentId, &p.SessionId, &p.Deleted); err != nil {
			return nil, fmt.Errorf("while scanning a purged incident: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// incidentIdRE finds incident ids named in prose: a transition's rationale
// names the incident a later one was raised after.
var incidentIdRE = regexp.MustCompile(`\binc_[A-Za-z0-9_.:-]+`)

// IncidentIdsIn returns the incident ids a text names, in order, once each, a
// trailing full stop or colon not taken as part of one.
func IncidentIdsIn(text string) []string {
	var out []string
	for _, m := range incidentIdRE.FindAllString(text, -1) {
		m = strings.TrimRight(m, ".:")
		if _, _, ok := SessionOfIncidentId(m); ok && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}
