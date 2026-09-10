package jetrules_go_adaptor

// Does an unresolvable code get flagged before the prose is written?
//
// **The guard is on the input, not the output, and that is the point.** The
// briefing's prose -- template or model -- is a function of the entity it is
// given, and a function of incomplete input is confidently wrong. With no drug
// name and no class the best either renderer can do is print the NDC, which is
// noise to a call-centre representative.
//
// **The 22-member unit-test set cannot exercise this.** Its NDCs were curated to
// resolve, because a missing one was known to impair prose generation — 0 misses
// across 257 fills against a 182,280-row lookup. So the case that occurs in
// production is the case that population excludes, and this file supplies it.
//
// Run as the sibling tests are run:
//
//	WORKSPACES_HOME=<...>/workspaces WORKSPACE=jets_ws \
//	  go test -count=1 ./jets/compute_pipes/jetrules_go_adaptor/

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ndcNotInTheLookup is 11 digits of the right shape and is absent from
// drug_info_lookup.csv. Checked rather than assumed: see the assertion in
// TestTheFixtureNdcReallyIsUnresolvable below, which fails if a future lookup
// refresh happens to add it.
const ndcNotInTheLookup = "99999999999"

func aFillWithAnUnresolvableNdc(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) {
	t.Helper()
	c := claims(t, s, rm)
	c.medical("mc-1", "81", 2025, 8, 14, "B182")
	c.pharmacy("pc-1", ndcTramadol, 2025, 7, 2)
	c.pharmacy("pc-x", ndcNotInTheLookup, 2025, 7, 20)
}

func briefingFlags(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) []string {
	t.Helper()
	b := objectOfSomeBriefing(t, s, rm)
	return values(s, rm, b, "cintel:Briefing_Data_Flags")
}

// The guard fires, and it names the code it could not resolve.
//
// Naming it is what separates a formatting problem from a coverage gap: a client
// whose codes arrive in an unexpected shape misses on nearly every row, and one
// whose data is fine misses on the few codes the reference table lacks. Both look
// like a thin briefing.
func TestAnUnresolvableNdcIsFlaggedOnTheBriefing(t *testing.T) {
	s, rm := ruleSessionOver(t, aFillWithAnUnresolvableNdc)

	flags := briefingFlags(t, s, rm)
	if len(flags) == 0 {
		t.Fatal("cintel:Briefing_Data_Flags is empty; the unresolvable NDC was not flagged, " +
			"so the prose maker would be handed a medication it cannot describe")
	}
	var named bool
	for _, f := range flags {
		if strings.Contains(f, ndcNotInTheLookup) {
			named = true
		}
	}
	if !named {
		t.Errorf("flags = %v, none naming the unresolved NDC %s", flags, ndcNotInTheLookup)
	}
}

// The control, and it is the half that catches a guard which flags everything.
//
// A guard that fires on a resolvable code is worse than no guard: it trains
// whoever reads the flags to ignore them.
func TestAResolvableNdcIsNotFlagged(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)

	for _, f := range briefingFlags(t, s, rm) {
		if strings.Contains(f, "Unresolved NDC") {
			t.Errorf("the fixture's NDCs all resolve, and one was flagged anyway: %s", f)
		}
	}
}

// And the fixture's premise, asserted rather than trusted.
//
// If a lookup refresh ever adds this NDC, the two tests above start passing and
// failing for reasons that have nothing to do with the guard. This one says so.
func TestTheFixtureNdcReallyIsUnresolvable(t *testing.T) {
	s, rm := ruleSessionOver(t, aFillWithAnUnresolvableNdc)

	root := oneSubjectOfType(t, s, rm, "am:AnalysisRoot")
	profile := object(t, s, rm, root, "am:hasPatientProfile")
	for _, ev := range objects(s, rm, profile, "cintel:has_Pharmacy_Events") {
		if got := values(s, rm, ev, "hc:NDC"); len(got) == 1 && got[0] == ndcNotInTheLookup {
			if names := values(s, rm, ev, "hc:Drug_Name"); len(names) != 0 {
				t.Fatalf("NDC %s now resolves to %v in drug_info_lookup.csv, so it is no "+
					"longer a fixture for an unresolvable code", ndcNotInTheLookup, names)
			}
			return
		}
	}
	t.Fatalf("no pharmacy event carries NDC %s; the fixture is not reaching the rules",
		ndcNotInTheLookup)
}
