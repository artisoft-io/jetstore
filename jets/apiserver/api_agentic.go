package main

// The agentic supervision endpoint (task K.3, gap 11) — proposal staging,
// approval decisions and run transcripts, for the Workspace IDE.
//
// **This route was deliberately not registered by K.2**, which built
// `audit.ReadTranscript` and stopped. The reasoning is worth keeping where the
// route is rather than only in a tracking document: a route registered before
// its consumer exists fixes its shape by guess, and `jets/apiserver/server.go`
// is the known textual collision surface with the UI project. So the shape below
// is the shape the screens actually call, and it arrived with them.
//
// **This is a JetStore route serving JetStore's own UI, not MCP over HTTP.**
// The distinction matters because the agentic project has twice deferred
// exposing the *tool catalogue* over HTTP (I-69), and that deferral does not
// carry here and is not weakened by this file. Nothing below dispatches a tool,
// runs a loop, or reaches a model: every action is a read of, or a decision
// about, rows that already exist. A future MCP-over-HTTP adapter would need
// identity, per-request workspace resolution and capability enforcement in the
// adapter rather than in the loop; this endpoint needs only the last of those,
// and gets it from the same `authh` + capability pattern every other route here
// uses.
//
// # The action envelope, and why not REST
//
// One POST route with an action name, matching `/inferServer` and `/dataTable`.
// Three reasons, in the order they mattered: the client already has a POST
// helper that consumes the refreshed token out of every response body
// (`jetsclient_ide/src/api/client.ts`), and a GET route would need a second one;
// `authh` mints a token per request, so a GET that returned a bare array would
// have nowhere to put it; and every existing route here is shaped this way, so
// a REST island would be the odd one out for no gain.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// AgentSupervisionCapability is required by every action on this endpoint.
//
// **A new capability rather than a reused one, and the argument is the read
// side rather than the write side.** Approving a change proposal is plainly not
// `workspace_ide` — editing a rule file and authorising an agent's change are
// different authorities. What settles it is that the *reads* are governance
// records: a transcript is the evidence trail for what an agent did, and
// `jetstore_read` is documented as "read data in JetStore", which is client
// data. Seeded in jets_init_db.sql; TestAgentSupervisionCapabilityIsSeeded
// checks that this constant and that file still agree.
const AgentSupervisionCapability = "agent_supervision"

// AgenticAction is the request envelope. Fields are read per action and ignored
// otherwise; the alternative, a `json.RawMessage` per action, buys nothing at
// six actions and costs a second unmarshal in each.
type AgenticAction struct {
	Action string `json:"action"`

	// list_proposals
	States []string `json:"states"`
	Limit  int      `json:"limit"`

	// get_proposal, record_approval
	ProposalId string `json:"proposalId"`

	// read_transcript
	RunId string `json:"runId"`

	// list_incidents. A separate field from `States` rather than a reused one:
	// the two vocabularies are different — nine approval states against eleven
	// incident statuses — and a filter sent under the wrong name would be
	// refused by the wrong validator with a message naming the wrong taxonomy.
	Statuses []string `json:"statuses"`

	// get_incident
	IncidentId string `json:"incidentId"`

	// record_approval. There is no `actor` and no `tier`: the actor is the
	// authenticated user and the tier is read from the run. A request that
	// could name either would let a caller sign a decision as somebody else,
	// which is the failure an approval record exists to make impossible.
	FromState string `json:"fromState"`
	ToState   string `json:"toState"`
	Rationale string `json:"rationale"`
}

// agenticOps is the data access this endpoint needs, as an interface so the
// dispatch below is testable without Postgres.
//
// **The seam is not ceremony; it is what makes two behaviours checkable.** The
// state conflict must reach the client as 409 rather than 500 — K.1 made
// `ErrStateConflict` a distinct type precisely so a caller could act on it, and
// a mapping that only a live race can exercise is a mapping nobody exercises.
// And a transcript's defects must reach the client whenever its events do; a
// fake that returns a damaged chain is the only cheap way to assert that,
// because the append-only trigger makes a real one impossible to produce.
type agenticOps interface {
	ListProposals(ctx context.Context, states []string, limit int) ([]audit.ProposalSummary, error)
	ReadProposal(ctx context.Context, proposalId string) (*audit.Proposal, error)
	ApprovalsFor(ctx context.Context, subjectRef string) ([]audit.Approval, error)
	ReadTranscript(ctx context.Context, runId string) (*audit.Transcript, error)
	RunTier(ctx context.Context, runId string) (string, error)
	RecordApproval(ctx context.Context, a *audit.Approval) (int, error)
	// The incident half (task AE.1), read-only. There is no write here at all:
	// an incident's classification transitions are what I-276 asks for an actor
	// on, and that widening is `AB.2`'s.
	ListIncidents(ctx context.Context, statuses []string, limit int) ([]audit.IncidentSummary, error)
	ReadIncident(ctx context.Context, incidentId string) (*audit.Incident, error)
}

// pgOps is the production implementation over the server's pool.
type pgOps struct{ db pgxPool }

// pgxPool is the slice of *pgxpool.Pool used here — read, and open a
// transaction. Named rather than taking the concrete type so a test may
// substitute one.
type pgxPool interface {
	audit.Querier
	Begin(ctx context.Context) (pgx.Tx, error)
}

func (o pgOps) ListProposals(ctx context.Context, states []string, limit int) ([]audit.ProposalSummary, error) {
	return audit.ListProposals(ctx, o.db, states, limit)
}
func (o pgOps) ReadProposal(ctx context.Context, id string) (*audit.Proposal, error) {
	return audit.ReadProposal(ctx, o.db, id)
}
func (o pgOps) ApprovalsFor(ctx context.Context, subjectRef string) ([]audit.Approval, error) {
	return audit.ApprovalsFor(ctx, o.db, subjectRef)
}
func (o pgOps) ReadTranscript(ctx context.Context, runId string) (*audit.Transcript, error) {
	return audit.ReadTranscript(ctx, o.db, runId)
}
func (o pgOps) RunTier(ctx context.Context, runId string) (string, error) {
	return audit.RunTier(ctx, o.db, runId)
}
func (o pgOps) RecordApproval(ctx context.Context, a *audit.Approval) (int, error) {
	return audit.RecordApproval(ctx, o.db, a)
}
func (o pgOps) ListIncidents(ctx context.Context, statuses []string, limit int) ([]audit.IncidentSummary, error) {
	return audit.ListIncidents(ctx, o.db, statuses, limit)
}
func (o pgOps) ReadIncident(ctx context.Context, id string) (*audit.Incident, error) {
	return audit.ReadIncident(ctx, o.db, id)
}

// DoAgenticAction ----------------------------------------------------------
// Entry point function. Authentication is `authh`'s; the capability check is
// here, following DoInferServerAction.
func (server *Server) DoAgenticAction(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}
	token := user.ExtractToken(r)
	userEmail, _ := user.ExtractTokenID(token)
	server.AuditLogger.Info(string(body), zap.String("user", userEmail),
		zap.String("time", time.Now().Format(time.RFC3339)))

	jetsUser, err := user.GetUserByToken(server.dbpool, token)
	if err != nil {
		log.Printf("while GetUserByToken: %v", err)
		ERROR(w, http.StatusUnauthorized, errors.New("error: unauthorized, cannot get user info"))
		return
	}
	if !jetsUser.HasCapability(AgentSupervisionCapability) {
		log.Printf("user %s attempted an agentic action without the %s capability",
			userEmail, AgentSupervisionCapability)
		ERROR(w, http.StatusForbidden,
			errors.New("error: unauthorized, user do not have required capability"))
		return
	}

	action := AgenticAction{}
	if err = json.Unmarshal(body, &action); err != nil {
		ERROR(w, http.StatusUnprocessableEntity, err)
		return
	}

	results, code, err := agenticDispatch(r.Context(), pgOps{server.dbpool}, &action, jetsUser.Email)
	if err != nil {
		log.Printf("Error: %v", err)
		ERROR(w, code, err)
		return
	}
	addToken(r, results)
	JSON(w, http.StatusOK, results)
}

// agenticDispatch is the whole of this endpoint's behaviour, separated from the
// http plumbing so it can be driven by a fake store. `actor` is the
// authenticated user's email and is the only identity a decision may be
// recorded under.
func agenticDispatch(ctx context.Context, ops agenticOps, a *AgenticAction, actor string) (*map[string]any, int, error) {
	switch a.Action {
	case "list_proposals":
		return listProposals(ctx, ops, a)
	case "get_proposal":
		return getProposal(ctx, ops, a)
	case "read_transcript":
		return readTranscript(ctx, ops, a)
	case "record_approval":
		return recordApproval(ctx, ops, a, actor)
	case "list_incidents":
		return listIncidents(ctx, ops, a)
	case "get_incident":
		return getIncident(ctx, ops, a)
	default:
		return nil, http.StatusBadRequest, fmt.Errorf("unknown agentic action %q", a.Action)
	}
}

func listProposals(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	rows, err := ops.ListProposals(ctx, a.States, a.Limit)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		out = append(out, map[string]any{
			"proposalId":               p.ProposalId,
			"trigger":                  p.Trigger,
			"triggerRef":               p.TriggerRef,
			"approvalState":            p.ApprovalState,
			"modelVersion":             p.ModelVersion,
			"clinicalRelevanceTouched": p.ClinicalRelevanceTouched,
			"affectedPipelineCount":    p.AffectedPipelineCount,
			"generatedTestCount":       p.GeneratedTestCount,
			"lastDecisionAt":           timeOrEmpty(p.LastDecisionAt),
			// The transitions travel with each row so the list can show which
			// proposals are still movable without a request per row. It is the
			// same table the detail screen decides its buttons from.
			"transitions": transitionsOrEmpty(p.ApprovalState),
		})
	}
	// `states` is echoed back because the screen renders it as the active
	// filter, and echoing what the server understood is cheaper than trusting
	// that the request and the response agree.
	return &map[string]any{"proposals": out, "states": nonEmptyStrings(a.States)}, http.StatusOK, nil
}

func getProposal(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	p, err := ops.ReadProposal(ctx, a.ProposalId)
	if err != nil {
		var missing *audit.ErrNoProposal
		if errors.As(err, &missing) {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}
	approvals, err := ops.ApprovalsFor(ctx, p.ProposalId)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	decisions := make([]map[string]any, 0, len(approvals))
	for _, ap := range approvals {
		decisions = append(decisions, map[string]any{
			"approvalEventId": ap.ApprovalEventId,
			"runRef":          ap.RunRef,
			"fromState":       ap.FromState,
			"toState":         ap.ToState,
			"actor":           ap.Actor,
			"tierAtEvent":     ap.TierAtEvent,
			"decidedAt":       timeOrEmpty(ap.DecidedAt),
			"rationale":       ap.DecisionRationale,
		})
	}
	return &map[string]any{
		"proposal": map[string]any{
			"proposalId":     p.ProposalId,
			"trigger":        p.Trigger,
			"triggerRef":     p.TriggerRef,
			"rationale":      p.Rationale,
			"affected":       nonEmptyStrings(p.AffectedPipelines),
			"generatedTests": nonEmptyStrings(p.GeneratedTests),
			"affectedAssets": nonEmptyStrings(p.ImpactAffectedAssets),
			"approvalState":  p.ApprovalState,
			"modelVersion":   p.ModelVersion,
		},
		"approvals":   decisions,
		"transitions": transitionsOrEmpty(p.ApprovalState),
		"terminal":    audit.Terminal(p.ApprovalState),
	}, http.StatusOK, nil
}

func readTranscript(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	t, err := ops.ReadTranscript(ctx, a.RunId)
	if err != nil {
		var missing *audit.ErrNoTranscript
		if errors.As(err, &missing) {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}
	events := make([]map[string]any, 0, len(t.Events))
	for _, ev := range t.Events {
		events = append(events, map[string]any{
			"seq":       ev.Seq,
			"eventType": ev.EventType,
			"actor":     ev.Actor,
			"tier":      ev.Tier,
			"toolName":  ev.ToolName,
			// Passed through as raw json rather than re-encoded: these are the
			// bytes the chain hashed, and re-encoding here would render
			// something other than what was verified.
			"payload":   json.RawMessage(ev.Payload),
			"createdAt": timeOrEmpty(ev.CreatedAt),
		})
	}
	defects := make([]map[string]any, 0, len(t.Defects))
	for _, d := range t.Defects {
		defects = append(defects, map[string]any{
			"seq": d.Seq, "kind": string(d.Kind), "detail": d.Detail,
		})
	}
	// `verified` and `defects` are not optional and are not a separate action.
	// K.2's design decision is that there is no way to obtain the events
	// without the verdict, and an endpoint that split them would give the
	// client the choice K.2 refused to give the caller.
	return &map[string]any{
		"runId":    t.RunId,
		"events":   events,
		"defects":  defects,
		"verified": t.Verified(),
	}, http.StatusOK, nil
}

func recordApproval(ctx context.Context, ops agenticOps, a *AgenticAction, actor string) (*map[string]any, int, error) {
	if actor == "" {
		return nil, http.StatusUnauthorized,
			errors.New("an approval cannot be recorded without an authenticated actor")
	}
	p, err := ops.ReadProposal(ctx, a.ProposalId)
	if err != nil {
		var missing *audit.ErrNoProposal
		if errors.As(err, &missing) {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusInternalServerError, err
	}

	// **The client's fromState is checked against the row, not trusted as it.**
	// RecordApproval guards the UPDATE on from_state, so a stale value would be
	// caught there — but as a conflict, after the lifecycle check had already
	// been made against the wrong state. Reading the row first means the
	// transition is judged against what is actually there, and a stale screen
	// gets the same answer either way.
	if a.FromState != "" && a.FromState != p.ApprovalState {
		return nil, http.StatusConflict, &audit.ErrStateConflict{
			SubjectRef: p.ProposalId, Expected: a.FromState,
		}
	}
	if !audit.TransitionAllowed(p.ApprovalState, a.ToState) {
		return nil, http.StatusBadRequest, fmt.Errorf(
			"proposal %s is in %s, from which %q is not a permitted transition; permitted: %v",
			p.ProposalId, p.ApprovalState, a.ToState, transitionsOrEmpty(p.ApprovalState))
	}

	// The tier comes from the run, never from the request. See audit.RunTier.
	tier, err := ops.RunTier(ctx, p.TriggerRef)
	if err != nil {
		var missing *audit.ErrNoRun
		if errors.As(err, &missing) {
			return nil, http.StatusConflict, err
		}
		return nil, http.StatusInternalServerError, err
	}

	decidedAt := time.Now().UTC()
	ap := &audit.Approval{
		// The event id is minted here and not accepted from the client: it is
		// the primary key of an append-only governance record, and a caller
		// that could choose it could collide with an existing decision.
		ApprovalEventId:   fmt.Sprintf("apr_%d", decidedAt.UnixNano()),
		RunRef:            p.TriggerRef,
		SubjectRef:        p.ProposalId,
		FromState:         p.ApprovalState,
		ToState:           a.ToState,
		Actor:             actor,
		TierAtEvent:       tier,
		DecidedAt:         decidedAt,
		DecisionRationale: a.Rationale,
	}
	seq, err := ops.RecordApproval(ctx, ap)
	if err != nil {
		var conflict *audit.ErrStateConflict
		if errors.As(err, &conflict) {
			// 409 rather than 500: the proposal moved between the read above
			// and the write, which is a race the screen recovers from by
			// re-reading, and the one case a caller can act on.
			return nil, http.StatusConflict, err
		}
		return nil, http.StatusInternalServerError, err
	}
	return &map[string]any{
		"approvalEventId": ap.ApprovalEventId,
		"proposalId":      ap.SubjectRef,
		"fromState":       ap.FromState,
		"toState":         ap.ToState,
		"actor":           ap.Actor,
		"tierAtEvent":     ap.TierAtEvent,
		"decidedAt":       timeOrEmpty(ap.DecidedAt),
		// The seq the chain trigger assigned, so a caller can cite the audit
		// row its decision produced.
		"auditSeq":    seq,
		"transitions": transitionsOrEmpty(ap.ToState),
	}, http.StatusOK, nil
}

// The incident half (task AE.1, gap 11 residue) — two reads and no write.
//
// **Both actions carry the locus and the classification together**, which is the
// endpoint's share of R-27's mitigation. Plan §9.5 found the record supports a
// taxonomy of *locus* and not of *cause*, so `AB.1` put both on the row with the
// classification optional (I-289); a response that carried one without the other
// would let a screen render a locus in a column headed with a cause word, which
// is the risk arriving through the wire rather than through the prose.

func listIncidents(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	rows, err := ops.ListIncidents(ctx, a.Statuses, a.Limit)
	if err != nil {
		return nil, incidentErrorCode(err), err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, i := range rows {
		out = append(out, incidentRow(i))
	}
	return &map[string]any{
		"incidents": out,
		// Echoed for the same reason `list_proposals` echoes its states: what
		// the server understood is cheaper to render than what the request
		// hoped it would.
		"statuses": nonEmptyStrings(a.Statuses),
		// **The vocabularies travel with the list rather than being spelled in
		// the browser**, on the same argument as the transition sets: a client
		// copy of a controlled vocabulary is the copy nothing enforces, and this
		// one is nine values that a regeneration can change. The screen renders
		// a legend from it and falls back to the bare value for anything it has
		// no gloss for, so a tenth locus degrades rather than disappears.
		"loci":             audit.IncidentLoci,
		"incidentStatuses": audit.IncidentStatuses,
	}, http.StatusOK, nil
}

func getIncident(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	inc, err := ops.ReadIncident(ctx, a.IncidentId)
	if err != nil {
		var missing *audit.ErrNoIncident
		if errors.As(err, &missing) {
			return nil, http.StatusNotFound, err
		}
		return nil, incidentErrorCode(err), err
	}
	hs := make([]map[string]any, 0, len(inc.Hypotheses))
	for _, h := range inc.Hypotheses {
		hs = append(hs, map[string]any{
			"hypothesisId":  h.HypothesisId,
			"incidentRef":   h.IncidentRef,
			"cause":         h.Cause,
			"causeCategory": h.CauseCategory,
			"confidence":    h.Confidence,
			"rank":          h.Rank,
			// Both sides, always. §A.2.8 calls the contradicting side a
			// calibration control, and an endpoint that could return one
			// without the other would offer a caller the choice of showing a
			// claim nobody argued with.
			"supportingEvidence":    evidenceItems(h.SupportingEvidence),
			"contradictingEvidence": evidenceItems(h.ContradictingEvidence),
		})
	}
	row := incidentRow(inc.IncidentSummary)
	row["hypotheses"] = hs
	return &map[string]any{"incident": row}, http.StatusOK, nil
}

func incidentRow(i audit.IncidentSummary) map[string]any {
	return map[string]any{
		"incidentId": i.IncidentId,
		"sessionId":  i.SessionId,
		"detectedAt": timeOrEmpty(i.DetectedAt),
		"locus":      i.Locus,
		// "" when nothing has claimed a cause, which is a state the schema
		// deliberately admits rather than a missing value.
		"classification": i.Classification,
		"severity":       i.Severity,
		"status":         i.Status,
		"stepRef":        i.StepRef,
		// null rather than 0 when the incident localises to no shard: 0 is the
		// first shard, so a coalesced zero would invent a localisation.
		"shardRef":        i.ShardRef,
		"confounders":     nonEmptyStrings(i.Confounders),
		"modelVersion":    i.ModelVersion,
		"hypothesisCount": i.HypothesisCount,
	}
}

func evidenceItems(items []audit.Evidence) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, e := range items {
		out = append(out, map[string]any{
			"statement": e.Statement, "source": e.Source, "sourceRef": e.SourceRef,
		})
	}
	return out
}

// incidentErrorCode separates "this deployment has not been migrated" from every
// other database failure.
//
// **503 rather than 500, and the distinction is the whole reason the error type
// exists.** `jetsapi.incident` reaches a database only through
// `update_db -migrateDb` (P3 I-169), so on any deployment older than AB.1 the
// tables are simply absent — which is not a fault, not the caller's mistake, and
// fixable by a named command. A 500 would put it in the same bucket as an outage
// and a screen would report it as one.
func incidentErrorCode(err error) int {
	var notDeployed *audit.ErrTablesNotDeployed
	if errors.As(err, &notDeployed) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// transitionsOrEmpty returns [] rather than nil so the json is an empty array
// rather than null. A terminal state and an unknown one both produce it, and a
// client that has to distinguish `null` from `[]` before rendering a button row
// is a client with an avoidable branch in it.
func transitionsOrEmpty(state string) []string {
	if t := audit.Transitions(state); t != nil {
		return t
	}
	return []string{}
}

func nonEmptyStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// timeOrEmpty renders a timestamp as RFC3339 with nanoseconds, or "" for the
// zero time — which is how "never decided" reaches the screen. A zero time
// serialised as a date reads as 1 January year 1 and looks like a bug.
func timeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
