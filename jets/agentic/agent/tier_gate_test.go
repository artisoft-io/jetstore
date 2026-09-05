package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
	"github.com/jackc/pgx/v5"
)

// Criterion 51: a required tier is compared against a run's current tier, the
// comparison refuses, and the refusal is in the audit record.

// tieredRegistry is a stubRegistry that can also say what a tool requires, so
// the optional-interface assertion in permitTool has something to find.
type tieredRegistry struct {
	stubRegistry
	minTier string
	err     error
}

func (r *tieredRegistry) MinTierOf(string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.minTier, nil
}

func tieredLoop(reg Verifier, rec Auditor, gate TierGate) *Loop {
	return &Loop{
		Infer:    &stubInfer{answers: []string{`{"a":1}`}},
		Registry: reg,
		Audit:    rec,
		TierGate: gate,
		Budget:   Budget{MaxIterations: 2},
		RunId:    "run-tier", Actor: "agent:test", Tier: "T1",
	}
}

// The whole criterion in one test: a tool requiring T3, a run at T1, a refusal,
// and the refusal on the transcript with both operands.
func TestCriterion51_ARunBelowTheRequiredTierIsRefused(t *testing.T) {
	reg := &tieredRegistry{
		stubRegistry: stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
		minTier:      "T3",
	}
	rec := &recorder{}
	l := tieredLoop(reg, rec, &FixedTierGate{Current: "T1"})

	res, err := l.Run(context.Background(), task())
	if err == nil {
		t.Fatal("a run at T1 must not reach a tool requiring T3")
	}
	if res.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want failed", res.Outcome)
	}
	// Refused *before* the call, which is what makes it a gate and not a
	// report: the registry recorded no dispatch.
	if len(reg.calls) != 0 {
		t.Errorf("the tool was called %d times after the refusal; none is permitted", len(reg.calls))
	}
	// The typed error carries both operands, so a caller need not parse the
	// sentence to know which way the comparison went.
	var low *ErrTierTooLow
	if !errors.As(err, &low) {
		t.Fatalf("error = %v, want an *ErrTierTooLow", err)
	}
	if low.Required != "T3" || low.Current != "T1" {
		t.Errorf("refusal carries required %q current %q, want T3 and T1", low.Required, low.Current)
	}
	if low.Action != "validate_cpipes_config" {
		t.Errorf("refusal names action %q, want the tool", low.Action)
	}

	// And the last clause of the criterion: it is in the audit record, with the
	// operands as fields.
	var refusal *audit.Event
	for _, ev := range rec.events {
		if ev.EventType == audit.EventError {
			refusal = ev
		}
	}
	if refusal == nil {
		t.Fatalf("no error event on the transcript; events = %v", rec.types())
	}
	var payload map[string]any
	if err := json.Unmarshal(refusal.Payload, &payload); err != nil {
		t.Fatalf("the refusal payload is not JSON: %v", err)
	}
	if payload["kind"] != "autonomy_tier" {
		t.Errorf(`payload kind = %v, want "autonomy_tier" — a tier refusal and a capability refusal must be countable apart`, payload["kind"])
	}
	if payload["required_tier"] != "T3" || payload["current_tier"] != "T1" {
		t.Errorf("payload records required %v current %v, want T3 and T1", payload["required_tier"], payload["current_tier"])
	}
}

// The other side of the comparison: equal authority passes, and so does more.
func TestTierGate_SufficientAuthorityPasses(t *testing.T) {
	for _, current := range []string{"T0", "T1", "T4"} {
		reg := &tieredRegistry{
			stubRegistry: stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
			minTier:      "T0",
		}
		l := tieredLoop(reg, &recorder{}, &FixedTierGate{Current: current})
		res, err := l.Run(context.Background(), task())
		if err != nil {
			t.Fatalf("a run at %s was refused a T0 tool: %v", current, err)
		}
		if res.Outcome != OutcomeSucceeded {
			t.Errorf("current %s: outcome = %q, want succeeded", current, res.Outcome)
		}
		if len(reg.calls) != 1 {
			t.Errorf("current %s: the tool was called %d times, want 1", current, len(reg.calls))
		}
	}
}

// Fail closed, three ways. Each is a refusal rather than an exemption, and each
// is distinguishable from a refusal on authority — the payload says the
// comparison was not made.
func TestTierGate_FailsClosed(t *testing.T) {
	cases := []struct {
		name string
		reg  Verifier
		gate TierGate
		want string
	}{
		{
			name: "a registry that cannot report a tier",
			reg:  &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
			gate: &FixedTierGate{Current: "T4"},
			want: "cannot say what tier",
		},
		{
			name: "a tool whose signature carries none",
			reg: &tieredRegistry{
				stubRegistry: stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
				err:          errors.New(`tool "validate_cpipes_config" carries no min_tier`),
			},
			gate: &FixedTierGate{Current: "T4"},
			want: "cannot determine the tier",
		},
		{
			name: "a run whose own tier is not in the vocabulary",
			reg: &tieredRegistry{
				stubRegistry: stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}},
				minTier:      "T0",
			},
			// The value the one non-test caller of this loop was carrying
			// before AJ.1, and the reason a Go-side vocabulary earns its place.
			gate: &FixedTierGate{Current: "assisted"},
			want: "cannot be read",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := &recorder{}
			_, err := tieredLoop(c.reg, rec, c.gate).Run(context.Background(), task())
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to mention %q", err, c.want)
			}
			var low *ErrTierTooLow
			if errors.As(err, &low) {
				t.Error("a comparison that could not be made was reported as an authority failure")
			}
			var refusal *audit.Event
			for _, ev := range rec.events {
				if ev.EventType == audit.EventError {
					refusal = ev
				}
			}
			if refusal == nil {
				t.Fatalf("no error event on the transcript; events = %v", rec.types())
			}
			var payload map[string]any
			if err := json.Unmarshal(refusal.Payload, &payload); err != nil {
				t.Fatalf("the refusal payload is not JSON: %v", err)
			}
			if payload["comparison"] != "not made" {
				t.Errorf(`payload does not record that the comparison was not made: %s`, refusal.Payload)
			}
		})
	}
}

// A nil gate is ungated, which is what every test written before AJ.1 is. This
// asserts the property those tests rely on rather than leaving it to the fact
// that they still pass.
func TestTierGate_NilIsUngated(t *testing.T) {
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}
	res, err := loop(&stubInfer{answers: []string{`{"a":1}`}}, reg, &recorder{}, 2).
		Run(context.Background(), task())
	if err != nil {
		t.Fatalf("an ungated loop must run: %v", err)
	}
	if res.Outcome != OutcomeSucceeded {
		t.Errorf("outcome = %q, want succeeded", res.Outcome)
	}
}

// The real registry satisfies TierSource, which is the assertion permitTool
// makes at the call site. If DefaultRegistry ever stopped satisfying it, every
// gated loop would fail closed in production and no test in the agent package
// would say why.
func TestDefaultRegistrySatisfiesTierSource(t *testing.T) {
	reg, err := tools.DefaultRegistry()
	if err != nil {
		t.Fatalf("building the default registry: %v", err)
	}
	src, ok := any(reg).(TierSource)
	if !ok {
		t.Fatal("*tools.Registry no longer satisfies TierSource; every gated loop now fails closed")
	}
	tier, err := src.MinTierOf("validate_cpipes_config")
	if err != nil {
		t.Fatalf("MinTierOf: %v", err)
	}
	if _, err := audit.ParseTier(tier); err != nil {
		t.Errorf("the registry reports a tier the vocabulary does not have: %v", err)
	}
	if _, err := src.MinTierOf("no_such_tool"); err == nil {
		t.Error("an unknown tool reported a tier; a gate reading that would fail open")
	}
}

// PgTierGate refuses when it cannot reach agent_run, rather than falling back
// on anything. This is guard.go's rule — a check that cannot reach its source
// must not fail open — arriving at the second check.
type brokenQuerier struct{}

func (brokenQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("connection refused")
}

func TestPgTierGate_UnresolvableRunRefuses(t *testing.T) {
	g := &PgTierGate{DB: brokenQuerier{}, RunId: "run-1"}
	err := g.Permit(context.Background(), "validate_cpipes_config", "T0")
	if err == nil {
		t.Fatal("a gate that cannot read the run record must refuse")
	}
	if !strings.Contains(err.Error(), "refusing to act") {
		t.Errorf("error = %v, want it to say it is refusing", err)
	}
	var low *ErrTierTooLow
	if errors.As(err, &low) {
		t.Error("an unreachable source was reported as an authority failure")
	}
}

func TestPgTierGate_NoRunIdRefuses(t *testing.T) {
	g := &PgTierGate{DB: brokenQuerier{}}
	if err := g.Permit(context.Background(), "validate_cpipes_config", "T0"); err == nil {
		t.Fatal("a gate with no run to check against must refuse")
	}
}
