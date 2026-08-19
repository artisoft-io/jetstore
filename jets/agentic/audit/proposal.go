package audit

import (
	"context"
	"fmt"
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
