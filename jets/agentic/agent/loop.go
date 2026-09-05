// Package agent is the authoring loop: propose, verify, repair, emit.
//
// Nothing in it is novel and that is the point — the value is that every
// transition is auditable and every verdict comes from a real verifier rather
// than from the model's opinion of its own output.
//
//	propose ──► verify ──► (pass) ──► artifact
//	   ▲           │
//	   │           └──► (fail) ──► repair prompt ──► propose'
//	   │                                │
//	   └────────────────────────────────┘   bounded by the budget
//
// **The loop does not know what a verifier is**, which is the property that
// keeps it from growing a case per artifact type. It names a tool in the
// registry and interprets a *verdict*, and the verdict contract is two fields:
// `valid`, and `diagnostics` when it is false. Both verifiers already satisfy
// it — tools.ValidationReport for .pc.json and tools.CompileReport for .jr —
// and TestVerdictContract asserts they still do, because the loop depends on
// that agreement rather than on either type.
//
// Diagnostics are carried into the repair prompt as raw JSON rather than
// flattened to strings. The two verifiers disagree about what a diagnostic is
// — one has a step index, the other a file, a line and a severity — and the
// lowest common denominator would discard exactly the part that tells a model
// where to look. Passing them through preserves everything and commits the
// loop to nothing.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/prompt"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
)

// Outcome is how a run ended. The three values are the terminal `outcome`
// events of the audit store.
type Outcome string

const (
	// OutcomeSucceeded means a proposal passed its verifier.
	OutcomeSucceeded Outcome = "succeeded"
	// OutcomeExhausted means the budget ran out with no proposal passing.
	// Distinct from failed on purpose: it says the loop worked and the model
	// did not converge, which is the compile-pass-rate denominator.
	OutcomeExhausted Outcome = "exhausted"
	// OutcomeInterrupted means the caller stopped the run — a cancelled
	// context from outside, not a budget of ours running out. Distinct from
	// exhausted because an interrupted run says nothing about the model and
	// must not count in the compile-pass denominator, and distinct from failed
	// because nothing went wrong.
	OutcomeInterrupted Outcome = "interrupted"
	// OutcomeFailed means the run could not continue — the model was
	// unreachable, the verifier errored, the budget was invalid.
	OutcomeFailed Outcome = "failed"
)

// Task is one authoring request: what to write, what shape it must take, and
// what will judge it.
type Task struct {
	// Instruction is the human-readable request, in English.
	Instruction string
	// Schema constrains generation and validates the answer. For Phase 1's
	// narrow tasks this is one $defs entry of the cpipes contract rather than
	// a whole document, which is what makes the task narrow at the wire and
	// not merely in the instruction text.
	Schema json.RawMessage
	// System is the system prompt. Optional.
	System string
	// Verifier is the registry tool that judges the artifact.
	Verifier string
	// VerifierArgs wraps a proposed artifact into that tool's arguments. It is
	// a function because the two verifiers disagree about the argument name —
	// `config` for cpipes, `rule_text` for the compiler — and teaching the
	// loop that difference is exactly what the registry exists to avoid.
	VerifierArgs func(artifact json.RawMessage) (json.RawMessage, error)
	// ContextTokens is the serving context this task will run against. Zero
	// takes the deployed default. It is on the task rather than the loop
	// because it bounds what may be *asked*, and gap 12 may move it.
	ContextTokens int
}

func (t *Task) validate() error {
	switch {
	case strings.TrimSpace(t.Instruction) == "":
		return errors.New("agent: the task has no instruction")
	case len(t.Schema) == 0:
		return errors.New("agent: the task has no schema; it is what constrains generation and what validates the answer")
	case t.Verifier == "":
		return errors.New("agent: the task names no verifier; an unverified proposal is not what this loop produces")
	case t.VerifierArgs == nil:
		return errors.New("agent: the task has no VerifierArgs, so a proposal cannot be handed to its verifier")
	}
	// A schema larger than the model's context is refused here rather than
	// discovered at the server, where it is either truncated — silently
	// changing what constrains generation — or rejected after the request has
	// been paid for. ContextTokens of zero takes the deployed default.
	if err := prompt.Fits(t.Schema, t.ContextTokens, 0); err != nil {
		return fmt.Errorf("agent: this task cannot be asked: %w", err)
	}
	return nil
}

// Budget bounds a run: two caps and a meter.
//
// The caps stop a run and are enforced here. Token spend is accumulated and
// recorded rather than capped, because what §4.3 needs is spend *comparable at
// equal budget* between sampling policies, and a token ceiling would truncate
// the very runs being compared.
//
// **A budget bounds one run and nothing else.** Nothing stops a user starting a
// hundred runs; that is a quota, it belongs with gap 7's tier machinery, and
// enforcing it here would put a control in the wrong layer.
type Budget struct {
	// MaxIterations bounds the propose-verify-repair cycle. Exceeding it ends
	// the run as exhausted.
	MaxIterations int
	// WallClock bounds the whole run, including verification and the time
	// between calls — it is the only cap that bounds a call the per-request
	// timeout missed. Zero means unbounded, which is what a test wants and
	// what production should not have.
	WallClock time.Duration
}

// Verdict is the contract the loop depends on, and the whole of what it
// understands about verification.
type Verdict struct {
	Valid       bool              `json:"valid"`
	Diagnostics []json.RawMessage `json:"diagnostics,omitempty"`
}

// Auditor is the loop's transcript. It is an interface so the loop can be
// tested without Postgres and so D.4 can supply the transactional
// write-before-act implementation without the loop changing.
//
// Appends within one run must be serialised by the caller: two appends racing
// on one run_id collide on UNIQUE (run_id, seq). A single-threaded loop
// satisfies that trivially; generate-and-filter does not, which is why each
// candidate takes its own run_id rather than sharing one.
type Auditor interface {
	Append(ctx context.Context, ev *audit.Event) error
}

// Inferencer is the model call, narrowed to what the loop uses.
type Inferencer interface {
	Chat(ctx context.Context, req *infer.Request) (*infer.Response, error)
}

// Verifier is the registry, narrowed the same way.
type Verifier interface {
	Call(ctx context.Context, ws *tools.Workspace, name string, args json.RawMessage) (any, error)
}

// Loop runs one task to a verdict.
type Loop struct {
	Infer     Inferencer
	Registry  Verifier
	Workspace *tools.Workspace
	Audit     Auditor
	// Recorder persists the run. When set, Start runs to completion before the
	// first model call and Finish runs after the last — that ordering is the
	// write-before-act contract and is why this is a field on the loop rather
	// than something a caller is trusted to do around it. Nil means the run is
	// not persisted, which is what the tests and a dry run use.
	Recorder Recorder
	// Guard is the durable stop. When set, it is consulted before the first
	// model call and before every tool call that writes. Nil means unguarded,
	// which is what the tests use — and what production must not be.
	Guard Guard
	// TierGate compares the tier an action requires against the authority this
	// run is operating at, and refuses below it (AJ.1, gap 7b). Consulted
	// before every tool call and nowhere else — see the note at the Guard check
	// in Run for why there is no tier check before the first model call. Nil
	// means ungated, on Guard's precedent and for the same reason: it is what
	// the tests are, and what production must not be.
	TierGate TierGate
	Budget   Budget

	// RunId correlates every audit event of this run. One run, one id; a
	// fan-out gives each candidate its own.
	RunId string
	// Actor and Tier are recorded on every event. Tier is recorded and not
	// enforced in Phase 1 — enforcement is gap 7 — and recording it now is
	// what makes that later work possible without backfilling.
	Actor string
	Tier  string
}

// Result is what a run produced.
type Result struct {
	Outcome Outcome
	// Artifact is the proposal that passed, and is nil unless the outcome is
	// succeeded.
	Artifact json.RawMessage
	// Iterations is how many propose-verify cycles ran.
	Iterations int
	// TokenSpend is the sum over every model call, including the ones whose
	// proposals were rejected. Rejected attempts cost tokens and a budget that
	// ignored them would not be a budget.
	TokenSpend int
	// LastDiagnostics is the verdict of the final failed attempt, kept so a
	// caller can report why a run exhausted rather than only that it did.
	LastDiagnostics []json.RawMessage
	// ProposalId names the change_proposal row a successful run produced, and
	// is empty when the run did not succeed or was not persisted.
	ProposalId string
}

// Run executes the loop. It returns a Result for every outcome the loop
// understands — including exhausted, which is a legitimate answer and not an
// error — and an error only when the run could not be conducted.
func (l *Loop) Run(ctx context.Context, task *Task) (*Result, error) {
	if err := task.validate(); err != nil {
		return nil, err
	}
	if l.Budget.MaxIterations <= 0 {
		return nil, errors.New("agent: the budget allows no iterations; an unbounded loop is what the budget exists to prevent")
	}
	if l.RunId == "" {
		return nil, errors.New("agent: the run has no id, so its audit events could not be correlated")
	}

	result := &Result{Outcome: OutcomeFailed}

	// The kill switch, before anything else spends money or writes a record.
	// A revoked identity should not leave an audit trail of a run it was never
	// allowed to start, so this precedes even the intent.
	//
	// **There is deliberately no tier check here, and the asymmetry is the
	// point rather than an omission (AJ.1).** Guard asks whether an identity
	// may act at all, which is answerable with no action in hand. A tier gate
	// asks whether this run's authority reaches *this* action, and before the
	// first model call there is no action to ask about — the loop does not yet
	// know which tool it will call, and the task names a verifier rather than
	// requiring one. Gating on the task's declared verifier here would refuse
	// runs that were going to exhaust before reaching it, and would put a
	// second, earlier answer beside the one in verify with nothing keeping the
	// two in step.
	if l.Guard != nil {
		if err := l.Guard.Allowed(ctx); err != nil {
			l.event(ctx, audit.EventError, "", errorPayload("capability", err.Error()))
			result.Outcome = OutcomeFailed
			return result, fmt.Errorf("agent: refused before the first model call: %w", err)
		}
	}

	// Write before act. The commit inside Start is the acknowledgement: after
	// it returns, a process that dies leaves a durable record of what it was
	// about to do. A failure here stops the run rather than proceeding
	// unrecorded — acting without a record is the one thing this contract
	// exists to prevent, so it is a hard failure and not a logged one.
	if l.Recorder != nil {
		if err := l.Recorder.Start(ctx, intentPayload(task, l.Budget)); err != nil {
			return nil, fmt.Errorf("agent: refusing to act on an unrecorded run: %w", err)
		}
	}

	// The wall-clock cap is a deadline on a derived context, so it bounds the
	// model call, the verification and the time between them alike — the
	// per-request timeout in the inference client bounds only one call, and a
	// run can exceed its budget without any single call doing so.
	parent := ctx
	if l.Budget.WallClock > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, l.Budget.WallClock)
		defer cancel()
	}

	req := &infer.Request{System: task.System, User: task.Instruction, Schema: task.Schema}

	for i := 1; i <= l.Budget.MaxIterations; i++ {
		// Check before spending rather than only after: the deadline may have
		// passed during the previous verification, and starting a call that
		// cannot finish wastes the tokens it costs.
		if out, stop := l.overrun(ctx, parent); stop {
			result.Outcome = out
			l.finish(ctx, out, result)
			return result, nil
		}
		result.Iterations = i

		resp, err := l.Infer.Chat(ctx, req)
		if err != nil {
			var schemaErr *infer.SchemaError
			if errors.As(err, &schemaErr) {
				// The model answered and the answer was the wrong shape. That
				// is a repairable failure, not a broken run: record it and
				// feed it back like any other verdict. The tokens it cost
				// count against the budget — they were spent.
				result.TokenSpend += schemaErr.Tokens()
				l.event(ctx, audit.EventError, "", errorPayload("schema", schemaErr.Error()))
				req = repairFromSchema(task, schemaErr)
				continue
			}
			// A call that died because a deadline passed is the budget
			// binding, not the model failing. Classifying it as a failure
			// would put budget exhaustion into the population §4.4 measures
			// the model against.
			if out, stop := l.overrun(ctx, parent); stop {
				result.Outcome = out
				l.finish(ctx, out, result)
				return result, nil
			}
			l.event(ctx, audit.EventError, "", errorPayload("inference", err.Error()))
			l.finish(ctx, OutcomeFailed, result)
			return result, fmt.Errorf("agent: the model call failed on iteration %d: %w", i, err)
		}
		result.TokenSpend += resp.Tokens()

		artifact := json.RawMessage(resp.Content)
		l.event(ctx, audit.EventDecision, "", decisionPayload(i, artifact))

		verdict, err := l.verify(ctx, task, artifact)
		if err != nil {
			// A refusal is already on the transcript with its own kind and its
			// own operands, and the verifier was never reached — so it must not
			// be appended a second time as a verifier failure, and must not be
			// reported as one. See the note on refusal.
			var ref *refusal
			if errors.As(err, &ref) {
				l.finish(ctx, OutcomeFailed, result)
				return result, fmt.Errorf("agent: refused before the tool call on iteration %d: %w", i, err)
			}
			l.event(ctx, audit.EventError, task.Verifier, errorPayload("verifier", err.Error()))
			l.finish(ctx, OutcomeFailed, result)
			return result, fmt.Errorf("agent: the verifier failed on iteration %d: %w", i, err)
		}
		if verdict.Valid {
			result.Outcome = OutcomeSucceeded
			result.Artifact = artifact
			result.LastDiagnostics = nil
			// The proposal is the run's output and is written before the run
			// is closed, so a run that reports success always has the thing it
			// succeeded at. A failure to record it is not fatal — the artifact
			// is on the transcript as a decision event either way — but it is
			// reported, because a caller expecting a proposal id should learn
			// it did not get one.
			if l.Recorder != nil {
				id, err := l.Recorder.Propose(ctx, artifact)
				if err != nil {
					l.event(ctx, audit.EventError, "", errorPayload("proposal", err.Error()))
				}
				result.ProposalId = id
			}
			l.finish(ctx, OutcomeSucceeded, result)
			return result, nil
		}

		result.LastDiagnostics = verdict.Diagnostics
		req = repairFromVerdict(task, artifact, verdict)
	}

	result.Outcome = OutcomeExhausted
	l.finish(ctx, OutcomeExhausted, result)
	return result, nil
}

// overrun says whether the run must stop, and which of the two reasons it is.
//
// The distinction matters more than it looks. A run stopped by *our* wall-clock
// cap is exhausted — the budget bound it, exactly as an iteration cap would,
// and it belongs in the compile-pass rate's denominator. A run stopped because
// the *caller's* context was cancelled is interrupted: nothing went wrong, the
// model was not given a fair attempt, and counting it against the model would
// be measuring the operator rather than the copilot.
func (l *Loop) overrun(ctx, parent context.Context) (Outcome, bool) {
	// The parent is checked first: when both are done, the caller's
	// cancellation is the cause and our deadline is a consequence of it.
	if parent.Err() != nil {
		return OutcomeInterrupted, true
	}
	if ctx.Err() != nil {
		return OutcomeExhausted, true
	}
	return "", false
}

// verify dispatches to the named tool and reads a verdict out of whatever it
// returned. The round trip through JSON is deliberate: it is what lets the
// loop accept any report satisfying the verdict contract without importing
// its type.
func (l *Loop) verify(ctx context.Context, task *Task, artifact json.RawMessage) (*Verdict, error) {
	args, err := task.VerifierArgs(artifact)
	if err != nil {
		return nil, fmt.Errorf("while building the verifier's arguments: %w", err)
	}
	// Re-checked before the tool call rather than trusted from the start of the
	// run: a revocation part-way through a long run should take effect at the
	// next check, which is the whole of what "durable" buys over a flag in
	// memory. Every Phase-1 tool is read-only, so today this guards a call that
	// changes nothing — it is here because the first write tool must not be the
	// thing that introduces the check.
	if l.Guard != nil {
		if err := l.Guard.Allowed(ctx); err != nil {
			// **The refusal goes on the transcript, and did not until AJ.1.**
			// The same check before the first model call has always appended an
			// error event; this one returned silently, so a run stopped by a
			// mid-run revocation left a trail ending at a decision with no
			// reason beside it. A refusal nothing records is indistinguishable
			// from a check that never ran, which is the distinction criterion
			// 47's attestation was built around.
			l.event(ctx, audit.EventError, task.Verifier, errorPayload("capability", err.Error()))
			return nil, &refusal{Kind: "capability", Err: err}
		}
	}
	// The autonomy gate (AJ.1, gap 7b, criterion 51). It sits behind the kill
	// switch because the two refuse different things and the cheaper, broader
	// one should answer first: an identity that may not act at all need not
	// have its authority compared against anything.
	if l.TierGate != nil {
		if err := l.permitTool(ctx, task.Verifier); err != nil {
			l.event(ctx, audit.EventError, task.Verifier, tierRefusalPayload(err))
			return nil, &refusal{Kind: "autonomy_tier", Err: err}
		}
	}
	raw, err := l.Registry.Call(ctx, l.Workspace, task.Verifier, args)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("while encoding the verifier's report: %w", err)
	}
	l.event(ctx, audit.EventToolCall, task.Verifier, toolCallPayload(args, encoded))

	var verdict Verdict
	if err := json.Unmarshal(encoded, &verdict); err != nil {
		return nil, fmt.Errorf("the verifier's report does not satisfy the verdict contract "+
			"(an object with a boolean `valid`): %w", err)
	}
	return &verdict, nil
}

// refusal marks an error the loop raised itself, before the tool ran, rather
// than one the tool returned.
//
// **It exists because the transcript was blaming the verifier for both, and
// AJ.1 is where that became visible.** A refusal from the kill switch or the
// autonomy gate is appended at the point of refusal, with its own kind and its
// own operands; Run then saw an error coming out of verify and appended a
// second error event reading `"kind":"verifier"`, so a run stopped by a
// governance control produced a trail whose last word named a tool that was
// never called. The wrapper is what lets Run tell a refusal from a failure and
// say which it was — which is the same distinction the payload's kinds make one
// level down, and criterion 51's last clause is worth nothing if the record
// then contradicts itself.
type refusal struct {
	// Kind matches the payload's kind, so the event and the returned error
	// agree about what stopped the run.
	Kind string
	Err  error
}

func (r *refusal) Error() string { return r.Err.Error() }
func (r *refusal) Unwrap() error { return r.Err }

// repairFromVerdict builds the next prompt from the diagnostics. This is Rung
// 3 of the analysis's §7, and the reason it is worth more than its size: the
// diagnostics were written for humans, so they are already the right feedback
// text, and passing them as JSON keeps the file, line and step a model needs
// to locate the problem.
func repairFromVerdict(task *Task, artifact json.RawMessage, verdict *Verdict) *infer.Request {
	var b strings.Builder
	b.WriteString(task.Instruction)
	b.WriteString("\n\nYour previous answer was rejected by the verifier.\n\nPrevious answer:\n")
	b.Write(indentJSON(artifact))
	b.WriteString("\n\nDiagnostics:\n")
	for _, d := range verdict.Diagnostics {
		b.WriteString("- ")
		b.Write(indentJSON(d))
		b.WriteString("\n")
	}
	b.WriteString("\nCorrect the answer. Change only what the diagnostics require; " +
		"return the whole artifact, not a patch.")
	return &infer.Request{System: task.System, User: b.String(), Schema: task.Schema}
}

// repairFromSchema is the same move one step earlier — the answer never
// reached a verifier because it was not the right shape.
func repairFromSchema(task *Task, err *infer.SchemaError) *infer.Request {
	var b strings.Builder
	b.WriteString(task.Instruction)
	b.WriteString("\n\nYour previous answer did not satisfy the required schema.\n\nPrevious answer:\n")
	b.WriteString(err.Content)
	b.WriteString("\n\nThe problem:\n")
	b.WriteString(err.Err.Error())
	b.WriteString("\n\nReturn a single JSON value satisfying the schema, and nothing else.")
	return &infer.Request{System: task.System, User: b.String(), Schema: task.Schema}
}

// event appends to the transcript. A failure to record is discarded rather
// than aborting the run: losing the audit trail is bad, and killing a run that
// is otherwise fine because its transcript hiccuped is worse. The intent event
// is the exception and is D.4's, because that one must be durable before
// anything acts.
//
// **Discarded, not recorded — corrected at K.2, 2026-08-23.** This comment
// previously said the failure was "logged into the result", and it is not:
// Result has no field for it and never had. The distinction matters because the
// audit trigger assigns seq as max(seq)+1, so an append that never lands leaves
// no gap either — a dropped event is invisible from both ends, and
// audit.Verify is correspondingly silent about it. The trade is still the right
// one; what was wrong was the claim that it left a trace. See the note under
// "What this proves" in jets/agentic/audit/transcript.go.
func (l *Loop) event(ctx context.Context, eventType, toolName string, payload []byte) {
	if l.Audit == nil {
		return
	}
	_ = l.Audit.Append(ctx, &audit.Event{
		RunId:     l.RunId,
		EventType: eventType,
		Actor:     l.Actor,
		Tier:      l.Tier,
		ToolName:  toolName,
		Payload:   payload,
	})
}

func (l *Loop) finish(ctx context.Context, outcome Outcome, result *Result) {
	// The outcome event first: it is the durable record, appended and
	// immutable. The run row is the mutable summary and follows, so a failure
	// to update it costs a summary rather than the trail.
	l.event(ctx, audit.EventOutcome, "", outcomePayload(outcome, result))
	if l.Recorder != nil {
		_ = l.Recorder.Finish(ctx, outcome, result.TokenSpend)
	}
}

func decisionPayload(iteration int, artifact json.RawMessage) []byte {
	b, _ := json.Marshal(map[string]any{"iteration": iteration, "artifact": artifact})
	return b
}

func toolCallPayload(args, report json.RawMessage) []byte {
	b, _ := json.Marshal(map[string]any{"request": args, "response": report})
	return b
}

func errorPayload(kind, message string) []byte {
	b, _ := json.Marshal(map[string]any{"kind": kind, "message": message})
	return b
}

// tierRefusalPayload is criterion 51's last clause: the refusal, in the audit
// record, with both operands.
//
// **The kind is distinct from "capability" on purpose.** Both refusals stop a
// tool call and they mean different things — one says the identity has been
// stopped, the other says this run's authority does not reach this action — and
// a reader counting refusals wants to tell them apart without parsing English.
// The operands are separate keys rather than interpolated into the message for
// the same reason: a query asking which actions were refused at which tier
// should read columns, not sentences.
func tierRefusalPayload(err error) []byte {
	m := map[string]any{"kind": "autonomy_tier", "message": err.Error()}
	var low *ErrTierTooLow
	if errors.As(err, &low) {
		m["action"] = low.Action
		m["required_tier"] = low.Required
		m["current_tier"] = low.Current
	} else {
		// The comparison could not be made rather than having been made and
		// refused. Saying so is what keeps "not authorised" and "could not
		// tell" apart in the record.
		m["comparison"] = "not made"
	}
	b, _ := json.Marshal(m)
	return b
}

func outcomePayload(outcome Outcome, result *Result) []byte {
	b, _ := json.Marshal(map[string]any{
		"outcome":     outcome,
		"iterations":  result.Iterations,
		"token_spend": result.TokenSpend,
	})
	return b
}

// indentJSON pretty-prints when it can and passes the bytes through when it
// cannot. A repair prompt quoting a malformed answer verbatim is more useful
// than one quoting an error about it.
func indentJSON(raw json.RawMessage) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return raw
	}
	return out.Bytes()
}
