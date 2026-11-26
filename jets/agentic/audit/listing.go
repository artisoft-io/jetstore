package audit

// One page of the incident list and of the proposal list, with the client and
// the object type of each row, for the supervision screens.
//
// # Where a row's client comes from
//
// Neither an incident nor a proposal records a client. An incident is written
// on a session, and a session is registered: a pipeline run's in
// pipeline_execution_status, a delivery's in input_registry, and both in
// session_registry. The client and the object type are read from those
// registrations, in that order, when the page is read. A proposal an incident
// triggered has its incident's; a proposal an authoring run produced has none.
//
// **The object type is the file's, file_key_staging.object_type by file key,
// and not the registry's object_type column.** input_registry.object_type and
// pipeline_execution_status.main_object_type hold the domain key the input is
// grouped under (the IDE labels both "Domain Key"), which takes the same values
// an object type does and is often a different one. A delivery's object type is
// its file's; a run's is its main input file's (main_input_file_key).
// file_key_staging is unique by file key. An input that is not a staged file (a
// domain table) reads "".
//
// **A registration the purge has removed leaves the row without a client**, so
// it reads "" and a client filter does not show it. That is the same thing the
// IDE's client filter does on every other table, where `client = <selection>`
// excludes a row whose client is null.
//
// The registry tables are not part of this package's schema, so a database
// that has the agentic tables and not the registry (this package's own test
// database) reads every client as "". Which tables exist is asked first, and
// only those are read.

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ListPage asks for one page of a supervision list.
type ListPage struct {
	// Client keeps the rows whose session is registered to this client; ""
	// keeps every row.
	Client string
	// Limit is the page size, at most the list's ceiling (500); 0 means the
	// ceiling. Offset is the number of rows skipped.
	Limit  int
	Offset int
}

func (p ListPage) bounded(ceiling int) (limit, offset int) {
	limit, offset = p.Limit, p.Offset
	if limit <= 0 || limit > ceiling {
		limit = ceiling
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// registration is the SQL that reads a session's client and object type from
// whichever registry tables the database has.
type registration struct{ client, objectType string }

func readRegistration(ctx context.Context, db Querier, session string) (registration, error) {
	rows, err := db.Query(ctx, `SELECT to_regclass('jetsapi.pipeline_execution_status') IS NOT NULL,
	       to_regclass('jetsapi.input_registry') IS NOT NULL,
	       to_regclass('jetsapi.session_registry') IS NOT NULL,
	       to_regclass('jetsapi.file_key_staging') IS NOT NULL`)
	if err != nil {
		return registration{}, fmt.Errorf("while asking which registry tables exist: %w", err)
	}
	var runs, inputs, sessions, staged bool
	if rows.Next() {
		err = rows.Scan(&runs, &inputs, &sessions, &staged)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		return registration{}, fmt.Errorf("while asking which registry tables exist: %w", err)
	}
	var clients, types []string
	client := func(table string) {
		clients = append(clients, fmt.Sprintf(
			"(SELECT max(r.client) FROM jetsapi.%s r WHERE r.session_id = %s)", table, session))
	}
	if runs {
		client("pipeline_execution_status")
		if staged {
			types = append(types, fmt.Sprintf(`(SELECT max(f.object_type) FROM jetsapi.pipeline_execution_status r
				JOIN jetsapi.file_key_staging f ON f.file_key = r.main_input_file_key
				WHERE r.session_id = %s)`, session))
		}
	}
	if inputs {
		client("input_registry")
		if staged {
			types = append(types, fmt.Sprintf(`(SELECT max(f.object_type) FROM jetsapi.input_registry r
				JOIN jetsapi.file_key_staging f ON f.file_key = r.file_key
				WHERE r.session_id = %s)`, session))
		}
	}
	if sessions {
		client("session_registry")
	}
	join := func(parts []string) string {
		return "coalesce(" + strings.Join(append(parts, "''"), ", ") + ")"
	}
	return registration{client: join(clients), objectType: join(types)}, nil
}

// ListIncidentPage returns one page of the incident list, most recently
// detected first, and how many incidents the filters keep in all. An empty
// status filter means every status. See ListIncidents for the order.
func ListIncidentPage(ctx context.Context, db Querier, statuses []string, page ListPage) ([]IncidentSummary, int, error) {
	limit, offset := page.bounded(maxIncidentPage)
	for _, s := range statuses {
		if !KnownIncidentStatus(s) {
			return nil, 0, fmt.Errorf("%q is not an incident status; the vocabulary is IncidentStatus's eleven", s)
		}
	}
	reg, err := readRegistration(ctx, db, "i.incident_session_id")
	if err != nil {
		return nil, 0, err
	}
	kept := `WITH c AS (
	  SELECT i.incident_id, i.incident_session_id, coalesce(i.incident_run_ref, '') AS run_ref,
	         i.incident_detected_at, i.incident_locus, coalesce(i.classification, '') AS classification,
	         i.severity, i.status, coalesce(i.incident_step_ref, '') AS step_ref, i.incident_shard_ref,
	         i.incident_confounders, i.incident_model_version,
	         ` + reg.client + ` AS client, ` + reg.objectType + ` AS object_type
	    FROM jetsapi.incident i
	   WHERE cardinality($1::text[]) = 0 OR i.status = ANY($1::text[]))`
	var total int
	rows, err := db.Query(ctx, kept+` SELECT count(*) FROM c WHERE $2 = '' OR c.client = $2`,
		nonNil(statuses), page.Client)
	if err != nil {
		return nil, 0, fmt.Errorf("while counting incidents: %w", notDeployed(err))
	}
	if rows.Next() {
		err = rows.Scan(&total)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		return nil, 0, fmt.Errorf("while counting incidents: %w", notDeployed(err))
	}
	rows, err = db.Query(ctx, kept+`
	SELECT c.incident_id, c.incident_session_id, c.run_ref, c.incident_detected_at, c.incident_locus,
	       c.classification, c.severity, c.status, c.step_ref, c.incident_shard_ref,
	       c.incident_confounders, c.incident_model_version, coalesce(h.n, 0), c.client, c.object_type
	  FROM c
	  LEFT JOIN (
	        SELECT hypothesis_incident_ref, count(*) AS n
	          FROM jetsapi.hypothesis
	         GROUP BY hypothesis_incident_ref) h
	    ON h.hypothesis_incident_ref = c.incident_id
	 WHERE $2 = '' OR c.client = $2
	 ORDER BY c.incident_detected_at DESC, c.incident_id
	 LIMIT $3 OFFSET $4`, nonNil(statuses), page.Client, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("while listing incidents: %w", notDeployed(err))
	}
	defer rows.Close()
	var out []IncidentSummary
	for rows.Next() {
		var s IncidentSummary
		if err := rows.Scan(&s.IncidentId, &s.SessionId, &s.RunRef, &s.DetectedAt, &s.Locus,
			&s.Classification, &s.Severity, &s.Status, &s.StepRef, &s.ShardRef,
			&s.Confounders, &s.ModelVersion, &s.HypothesisCount, &s.Client, &s.ObjectType); err != nil {
			return nil, 0, fmt.Errorf("while scanning an incident summary: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("while listing incidents: %w", notDeployed(err))
	}
	return out, total, nil
}

// ListProposalPage returns one page of the staging list, newest decision first
// and never decided last, and how many proposals the filters keep in all. An
// empty state filter means every state.
func ListProposalPage(ctx context.Context, db Querier, states []string, page ListPage) ([]ProposalSummary, int, error) {
	limit, offset := page.bounded(maxProposalPage)
	for _, s := range states {
		if !KnownState(s) {
			return nil, 0, fmt.Errorf("%q is not an approval state; the vocabulary has nine states", s)
		}
	}
	reg, err := readRegistration(ctx, db, "i.incident_session_id")
	if err != nil {
		return nil, 0, err
	}
	// The incident an incident-triggered proposal names carries its session;
	// any other proposal joins no incident, and its registration reads "".
	kept := `WITH c AS (
	  SELECT p.proposal_id, p.trigger, coalesce(p.trigger_ref, '') AS trigger_ref, p.approval_state,
	         p.proposal_model_version, p.impact_clinical_relevance_touched,
	         coalesce(array_length(p.affected_pipelines, 1), 0) AS pipelines,
	         coalesce(array_length(p.generated_tests, 1), 0) AS tests,
	         ` + reg.client + ` AS client, ` + reg.objectType + ` AS object_type
	    FROM jetsapi.change_proposal p
	    LEFT JOIN jetsapi.incident i
	      ON p.trigger = '` + TriggerIncident + `' AND i.incident_id = p.trigger_ref
	   WHERE cardinality($1::text[]) = 0 OR p.approval_state = ANY($1::text[]))`
	var total int
	rows, err := db.Query(ctx, kept+` SELECT count(*) FROM c WHERE $2 = '' OR c.client = $2`,
		nonNil(states), page.Client)
	if err != nil {
		return nil, 0, fmt.Errorf("while counting proposals: %w", err)
	}
	if rows.Next() {
		err = rows.Scan(&total)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		return nil, 0, fmt.Errorf("while counting proposals: %w", err)
	}
	// The LEFT JOIN is over a max, not the rows: a proposal with three
	// decisions must still produce one row here, and approval_event_subject_idx
	// covers (subject_ref, decided_at).
	rows, err = db.Query(ctx, kept+`
	SELECT c.proposal_id, c.trigger, c.trigger_ref, c.approval_state, c.proposal_model_version,
	       c.impact_clinical_relevance_touched, c.pipelines, c.tests, a.last_decision_at,
	       c.client, c.object_type
	  FROM c
	  LEFT JOIN (
	        SELECT subject_ref, max(decided_at) AS last_decision_at
	          FROM jetsapi.approval_event
	         GROUP BY subject_ref) a
	    ON a.subject_ref = c.proposal_id
	 WHERE $2 = '' OR c.client = $2
	 ORDER BY a.last_decision_at DESC NULLS LAST, c.proposal_id
	 LIMIT $3 OFFSET $4`, nonNil(states), page.Client, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("while listing proposals: %w", err)
	}
	defer rows.Close()
	var out []ProposalSummary
	for rows.Next() {
		var p ProposalSummary
		var decided *time.Time
		if err := rows.Scan(&p.ProposalId, &p.Trigger, &p.TriggerRef, &p.ApprovalState,
			&p.ModelVersion, &p.ClinicalRelevanceTouched, &p.AffectedPipelineCount,
			&p.GeneratedTestCount, &decided, &p.Client, &p.ObjectType); err != nil {
			return nil, 0, fmt.Errorf("while scanning a proposal summary: %w", err)
		}
		if decided != nil {
			p.LastDecisionAt = *decided
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
