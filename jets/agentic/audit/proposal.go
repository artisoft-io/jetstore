package audit

import (
	"context"
	"fmt"
	"time"
)

// What a run proposes (plan §3.6, task D.7).
//
// **Phase 1 writes a row and nothing to git.** Staged branch writes are the
// analysis's §5.2 "Write — staged" class and arrive with the Phase-2 approval
// screens. A copilot that can commit before anyone can review the commit has
// the supervision layer in the wrong order, and that ordering is the whole
// argument for shipping the authoring copilot first.
//
// **What `draft` means here, because it is not obvious.** Appendix A.2.10
// requires generated_tests, affected_pipelines and an impact analysis. A
// Phase-1 authoring run has none of them: it produced one transformation, not a
// change set with tests and a blast radius. So a Phase-1 proposal is created in
// `draft` with empty arrays — honest about having nothing, and distinguishable
// from a proposal that was never asked for the information. The approval
// lifecycle fills them before the proposal may leave draft, which is what the
// state is for.

// Proposal is one row of jetsapi.change_proposal, in the subset a Phase-1
// authoring run can honestly populate. The remaining columns of the table are
// the approval lifecycle's to fill.
type Proposal struct {
	ProposalId string
	// Trigger is what prompted it. For an authoring run this is the run.
	Trigger    string
	TriggerRef string
	// Artifact is what the model produced and the verifier accepted. It is
	// stored as the rationale's companion rather than in a column of its own:
	// A.2.10 models a change as a diff against a repository, and Phase 1 has no
	// repository to diff against yet.
	Rationale            string
	AffectedPipelines    []string
	GeneratedTests       []string
	ImpactAffectedAssets []string
	ApprovalState        string
	ModelVersion         string
}

// RecordProposal inserts a proposal. It is deliberately not part of the
// write-before-act transaction: a proposal is what a run *produced*, so it is
// written after the work rather than before it, and its durability matters less
// than the audit trail's — the trail records that the proposal was made even if
// this row is lost.
func RecordProposal(ctx context.Context, db Execer, p *Proposal) error {
	if p == nil || p.ProposalId == "" {
		return fmt.Errorf("a proposal must carry an id before it can be recorded")
	}
	if p.ApprovalState == "" {
		return fmt.Errorf("proposal %s has no approval state; a proposal outside the lifecycle cannot be approved or rejected", p.ProposalId)
	}
	// Empty rather than nil for the NOT NULL array columns: Postgres
	// distinguishes them and "asked, and there are none" is a different claim
	// from "never asked".
	_, err := db.Exec(ctx,
		`INSERT INTO jetsapi.change_proposal
		   (proposal_id, trigger, trigger_ref, rationale, affected_pipelines,
		    generated_tests, impact_affected_assets, impact_clinical_relevance_touched,
		    approval_state, proposal_model_version)
		 VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,$10)`,
		p.ProposalId, p.Trigger, p.TriggerRef, p.Rationale,
		nonNil(p.AffectedPipelines), nonNil(p.GeneratedTests), nonNil(p.ImpactAffectedAssets),
		// Phase 1 authors transformations, not clinical logic, and has no way
		// to judge this. False is the honest default and the approval
		// lifecycle is where a human sets it — §10 escalates on this flag, so
		// guessing true would raise noise and guessing it away would be worse.
		false,
		p.ApprovalState, p.ModelVersion)
	if err != nil {
		return fmt.Errorf("while recording proposal %s: %w", p.ProposalId, err)
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Reading proposals back (task K.3, gap 11) — the staging half of criterion
// 31's "a staged proposal is visible and approvable in the IDE".
//
// **K.2's finding, one table over.** The audit store had a write path and no
// read path, so the sufficiency claim its criterion made had never been
// exercised. `change_proposal` was in exactly the same condition: RecordProposal
// above has been writing rows since Phase 1's D.7 and nothing has ever selected
// one. The two functions below are what makes "visible" a property of the code
// rather than of the schema.
//
// **Neither of them verifies anything, and that is the difference from
// ReadTranscript.** A transcript is hash-chained, so reading it without checking
// it offers the appearance of a property; a proposal row is not chained and
// claims nothing of the kind. The integrity story for a proposal is the approval
// chain beside it — ApprovalsFor and the `approval` events — which is why the
// detail screen shows all three together and none of them alone.

// ProposalSummary is one row of a staging list: enough to decide which proposal
// to open, and no more.
//
// The list deliberately omits `rationale` and every array column. A staging
// screen shows tens of rows and a rationale is a paragraph; fetching them to
// truncate them in the browser would move the cost to the wrong place.
type ProposalSummary struct {
	ProposalId    string
	Trigger       string
	TriggerRef    string
	ApprovalState string
	ModelVersion  string
	// ClinicalRelevanceTouched is in the summary because §10 escalates on it —
	// it changes who should be looking at the row, so it belongs where the row
	// is chosen rather than one click in.
	ClinicalRelevanceTouched bool
	// AffectedPipelineCount and GeneratedTestCount are counts rather than the
	// arrays. A draft carries empty arrays honestly (see the note at the head
	// of this file), and "0 tests" is the single most useful thing a reviewer
	// can know before opening one.
	AffectedPipelineCount int
	GeneratedTestCount    int
	// LastDecisionAt is when this proposal was last moved, from approval_event,
	// or the zero time when it has never been decided. It is the one column
	// here that is not on change_proposal, and it is the column a reviewer
	// sorts by in their head.
	LastDecisionAt time.Time
}

// ListProposals returns the staging list, newest decision first and never
// decided last, bounded by limit.
//
// **The state filter is a set rather than a single value**, because the useful
// views are "everything still open" and "everything at all", and neither is one
// state. An empty filter means every state.
func ListProposals(ctx context.Context, db Querier, states []string, limit int) ([]ProposalSummary, error) {
	if limit <= 0 || limit > maxProposalPage {
		limit = maxProposalPage
	}
	for _, s := range states {
		if !KnownState(s) {
			return nil, fmt.Errorf("%q is not an approval state; the vocabulary is the nine of A.4", s)
		}
	}
	// The LEFT JOIN is over a max, not the rows: a proposal with three
	// decisions must still produce one row here, and approval_event_subject_idx
	// covers (subject_ref, decided_at).
	rows, err := db.Query(ctx,
		`SELECT p.proposal_id, p.trigger, coalesce(p.trigger_ref, ''), p.approval_state,
		        p.proposal_model_version, p.impact_clinical_relevance_touched,
		        coalesce(array_length(p.affected_pipelines, 1), 0),
		        coalesce(array_length(p.generated_tests, 1), 0),
		        a.last_decision_at
		   FROM jetsapi.change_proposal p
		   LEFT JOIN (
		         SELECT subject_ref, max(decided_at) AS last_decision_at
		           FROM jetsapi.approval_event
		          GROUP BY subject_ref) a
		     ON a.subject_ref = p.proposal_id
		  WHERE cardinality($1::text[]) = 0 OR p.approval_state = ANY($1::text[])
		  ORDER BY a.last_decision_at DESC NULLS LAST, p.proposal_id
		  LIMIT $2`, nonNil(states), limit)
	if err != nil {
		return nil, fmt.Errorf("while listing proposals: %w", err)
	}
	defer rows.Close()
	var out []ProposalSummary
	for rows.Next() {
		var p ProposalSummary
		var decided *time.Time
		if err := rows.Scan(&p.ProposalId, &p.Trigger, &p.TriggerRef, &p.ApprovalState,
			&p.ModelVersion, &p.ClinicalRelevanceTouched, &p.AffectedPipelineCount,
			&p.GeneratedTestCount, &decided); err != nil {
			return nil, fmt.Errorf("while scanning a proposal summary: %w", err)
		}
		if decided != nil {
			p.LastDecisionAt = *decided
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// maxProposalPage bounds a staging list. There is no paging cursor and this is
// not one: it is a ceiling on a screen that is meant to be worked through, and
// a deployment that reaches it has a triage problem rather than a paging
// problem. Named so the number is arguable.
const maxProposalPage = 500

// ErrNoProposal is returned when no such proposal exists. Distinct because it
// is the one failure a caller acts on — a stale link or a mistyped id — while
// the rest are outages.
type ErrNoProposal struct{ ProposalId string }

func (e *ErrNoProposal) Error() string {
	return fmt.Sprintf("no proposal %s", e.ProposalId)
}

// ReadProposal returns one proposal in full.
//
// It returns the same Proposal struct RecordProposal writes, which means the
// columns the approval lifecycle owns — code_diff, ci_result, assumptions_made
// — are **not** read back, because that struct does not carry them. That is a
// deliberate narrowing rather than an oversight: nothing writes those columns
// yet, so a screen rendering them would render nine permanently empty fields
// and teach the reader that the record is emptier than it is. When something
// writes them, this struct grows and so does the screen.
func ReadProposal(ctx context.Context, db Querier, proposalId string) (*Proposal, error) {
	if proposalId == "" {
		return nil, fmt.Errorf("cannot read a proposal without an id")
	}
	rows, err := db.Query(ctx,
		`SELECT proposal_id, trigger, coalesce(trigger_ref, ''), coalesce(rationale, ''),
		        affected_pipelines, generated_tests, impact_affected_assets,
		        approval_state, proposal_model_version
		   FROM jetsapi.change_proposal
		  WHERE proposal_id = $1`, proposalId)
	if err != nil {
		return nil, fmt.Errorf("while reading proposal %s: %w", proposalId, err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("while reading proposal %s: %w", proposalId, err)
		}
		return nil, &ErrNoProposal{ProposalId: proposalId}
	}
	var p Proposal
	if err := rows.Scan(&p.ProposalId, &p.Trigger, &p.TriggerRef, &p.Rationale,
		&p.AffectedPipelines, &p.GeneratedTests, &p.ImpactAffectedAssets,
		&p.ApprovalState, &p.ModelVersion); err != nil {
		return nil, fmt.Errorf("while scanning proposal %s: %w", proposalId, err)
	}
	return &p, rows.Err()
}
