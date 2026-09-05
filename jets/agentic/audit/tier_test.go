package audit

import (
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The tier vocabulary and its ordering, held against the generated SQL rather
// than against a comment (task AJ.1, gap 7b).
//
// These need no database: the schema is checked in beside the package and
// embedded, so the CHECK constraints are readable as text. That is the same
// move TestApprovalStatesMatchTheGeneratedCheck makes one file over, and for
// the same reason — a vocabulary the Go side and the CHECK disagree about fails
// at the INSERT, in production, after something has already decided to act.

// tierCheckConstraints names every CHECK in the generated schema that
// constrains a column to the tier vocabulary. Naming them rather than searching
// for a shape is deliberate: a fifth tier column added without a line here is
// invisible to this test, and a renamed constraint fails it loudly, which is the
// direction to fail in.
var tierCheckConstraints = []string{
	"agent_audit_tier_ck",
	"agent_run_tier_ck",
	"approval_event_tier_ck",
	"remediation_tier_ck",
}

// Criterion 51's first clause rests on the two operands naming the same
// vocabulary. This is what says they do.
func TestTiersMatchTheGeneratedCheck(t *testing.T) {
	sql, err := os.ReadFile("agent_audit.sql")
	if err != nil {
		t.Fatalf("reading agent_audit.sql: %v", err)
	}
	want := make([]string, 0, len(Tiers))
	for _, m := range Tiers {
		want = append(want, string(m))
	}
	for _, name := range tierCheckConstraints {
		// The `IS NULL OR` form on agent_audit and the bare form elsewhere are
		// both covered: everything between the constraint name and `IN (` is
		// skipped, and neither form has a closing paren in between.
		re := regexp.MustCompile(`CONSTRAINT ` + name + `\s+CHECK \([^)]*IN \(([^)]*)\)\)`)
		m := re.FindSubmatch(sql)
		if m == nil {
			t.Errorf("%s is not in the generated schema; this test is stale, or the constraint was renamed", name)
			continue
		}
		var got []string
		for _, lit := range strings.Split(string(m[1]), ",") {
			got = append(got, strings.Trim(strings.TrimSpace(lit), "'"))
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s constrains %v; the Go vocabulary is %v", name, got, want)
		}
	}
}

// The ordering assertion the plan asked for, and the reason this file exists
// rather than a `<` in the gate.
//
// AutonomyTier's docstring says lexicographic order is tier order and asks that
// the members not be relabelled to §9.2's names. This turns the request into a
// test: relabel Observe/Advise/Propose and the declared order stops being the
// sorted order, this fails, and nobody ships a comparison that has silently
// inverted.
func TestTierOrderIsLexicographic(t *testing.T) {
	declared := make([]string, 0, len(Tiers))
	for _, m := range Tiers {
		declared = append(declared, string(m))
	}
	sorted := append([]string(nil), declared...)
	sort.Strings(sorted)
	if strings.Join(declared, ",") != strings.Join(sorted, ",") {
		t.Fatalf("Tiers is declared %v and sorts to %v; model.py's claim that "+
			"lexicographic order is tier order no longer holds, and any comparison "+
			"written on the text is now inverted", declared, sorted)
	}
	// And the ordering Reaches actually uses agrees with it, pair by pair,
	// which is the property the claim is about rather than a restatement of it.
	for i := 0; i+1 < len(Tiers); i++ {
		lo, hi := Tiers[i], Tiers[i+1]
		if ok, err := Reaches(lo, hi); err != nil || ok {
			t.Errorf("%s reaches %s (err %v); the order is wrong", lo, hi, err)
		}
		if ok, err := Reaches(hi, lo); err != nil || !ok {
			t.Errorf("%s does not reach %s (err %v); the order is wrong", hi, lo, err)
		}
	}
}

func TestParseTierRefusesRatherThanDefaulting(t *testing.T) {
	for _, m := range Tiers {
		got, err := ParseTier(string(m))
		if err != nil || got != m {
			t.Errorf("ParseTier(%q) = %q, %v; want %q, nil", m, got, err, m)
		}
	}
	// " T1 " is admitted because a value round-tripped through a text column
	// and a config file picks up whitespace; nothing else is.
	if got, err := ParseTier("  T1  "); err != nil || got != TierT1 {
		t.Errorf(`ParseTier("  T1  ") = %q, %v; want T1, nil`, got, err)
	}
	// The empty string is the one worth naming: it is what an unset struct
	// field carries, and defaulting it to T0 would read as a real answer.
	for _, bad := range []string{"", "t1", "T5", "assisted", "Advise", "T10"} {
		got, err := ParseTier(bad)
		if err == nil {
			t.Errorf("ParseTier(%q) = %q with no error; an unknown tier must be refused", bad, got)
			continue
		}
		var unknown *ErrUnknownTier
		if !errors.As(err, &unknown) {
			t.Errorf("ParseTier(%q) returned %v, which is not an *ErrUnknownTier", bad, err)
		}
	}
}

func TestReaches(t *testing.T) {
	cases := []struct {
		current, required Tier
		want              bool
	}{
		{TierT0, TierT0, true},
		{TierT0, TierT1, false},
		{TierT0, TierT4, false},
		{TierT2, TierT1, true},
		{TierT2, TierT2, true},
		{TierT2, TierT3, false},
		{TierT4, TierT0, true},
		{TierT4, TierT4, true},
	}
	for _, c := range cases {
		got, err := Reaches(c.current, c.required)
		if err != nil {
			t.Errorf("Reaches(%s, %s) errored: %v", c.current, c.required, err)
			continue
		}
		if got != c.want {
			t.Errorf("Reaches(%s, %s) = %v, want %v", c.current, c.required, got, c.want)
		}
	}
}

// An unusable operand is an error rather than a false, so that "not authorised"
// and "this build cannot read that value" are distinguishable in the record.
func TestReachesRefusesAnUnknownOperand(t *testing.T) {
	if _, err := Reaches(Tier("assisted"), TierT1); err == nil {
		t.Error("an unknown current tier compared cleanly")
	}
	if _, err := Reaches(TierT4, Tier("")); err == nil {
		t.Error("an empty required tier compared cleanly")
	}
	// Even the permissive direction refuses: T4 reaches everything in the
	// vocabulary and must not thereby reach something outside it.
	if ok, err := Reaches(TierT4, Tier("T5")); err == nil || ok {
		t.Error("T4 reached a tier that is not in the vocabulary")
	}
}
