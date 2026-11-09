package compute_pipes

import (
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// The stage layout a mapping pipeline writes: $CLIENT/<ObjectType>/${PERIOD_ID}M/$SESSIONID/...
func period(id int, prefix string, keys ...string) lookbackPeriodObjects {
	p := lookbackPeriodObjects{periodId: id, prefix: prefix}
	for _, k := range keys {
		p.objects = append(p.objects, &awsi.S3Object{Key: k, Size: 10})
	}
	return p
}

func TestLatestDeliveryTakesTheLatestPeriodThenTheLatestSession(t *testing.T) {
	periods := []lookbackPeriodObjects{
		period(682, "stage/c/PharmacyClaim/682M",
			"stage/c/PharmacyClaim/682M/1792005309434/part-0001.csv",
			"stage/c/PharmacyClaim/682M/1792005309434/part-0002.csv",
			"stage/c/PharmacyClaim/682M/1791958838089/part-0001.csv"),
		// An older delivery re-mapped later: a newer session id in an older period.
		period(681, "stage/c/PharmacyClaim/681M",
			"stage/c/PharmacyClaim/681M/1792099999999/part-0001.csv"),
	}
	got, err := latestDelivery(periods)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"stage/c/PharmacyClaim/682M/1792005309434/part-0001.csv",
		"stage/c/PharmacyClaim/682M/1792005309434/part-0002.csv",
	}
	if !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
}

func TestLatestDeliverySkipsEmptyPeriodsAndEmptySessions(t *testing.T) {
	empty := period(682, "stage/c/MedicalClaim/682M")
	markerOnly := lookbackPeriodObjects{periodId: 681, prefix: "stage/c/MedicalClaim/681M",
		objects: []*awsi.S3Object{{Key: "stage/c/MedicalClaim/681M/1792000000002/_DONE", Size: 0}}}
	older := period(680, "stage/c/MedicalClaim/680M",
		"stage/c/MedicalClaim/680M/1791000000001/part-0001.csv")
	got, err := latestDelivery([]lookbackPeriodObjects{empty, markerOnly, older})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"stage/c/MedicalClaim/680M/1791000000001/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
	// Within a period, a session holding only an empty object is not a delivery.
	mixed := lookbackPeriodObjects{periodId: 682, prefix: "stage/c/MedicalClaim/682M",
		objects: []*awsi.S3Object{
			{Key: "stage/c/MedicalClaim/682M/1792000000009/_DONE", Size: 0},
			{Key: "stage/c/MedicalClaim/682M/1792000000001/part-0001.csv", Size: 5},
		}}
	got, err = latestDelivery([]lookbackPeriodObjects{mixed})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"stage/c/MedicalClaim/682M/1792000000001/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
	if got, _ := latestDelivery(nil); got != nil {
		t.Errorf("no period: got %v, want nothing", keysOf(got))
	}
}

func TestLatestDeliveryRefusesAKeyWithNoSessionFolder(t *testing.T) {
	_, err := latestDelivery([]lookbackPeriodObjects{
		period(682, "stage/c/PharmacyClaim/682M", "stage/c/PharmacyClaim/682M/part-0001.csv"),
	})
	if err == nil || !strings.Contains(err.Error(), "no session folder") {
		t.Errorf("got %v, want a refusal naming the missing session folder", err)
	}
}

func TestSessionSegment(t *testing.T) {
	cases := []struct {
		prefix, key, want string
		ok                bool
	}{
		{"stage/c/P/682M", "stage/c/P/682M/123/part.csv", "123", true},
		{"stage/c/P/682M/", "stage/c/P/682M/123/a/b.csv", "123", true},
		{"/stage/c/P/682M", "stage/c/P/682M/123/part.csv", "123", true},
		{"stage/c/P/682M", "stage/c/P/682M/part.csv", "", false},
		{"stage/c/P/682M", "stage/other/123/part.csv", "", false},
	}
	for _, c := range cases {
		got, ok := sessionSegment(c.prefix, c.key)
		if got != c.want || ok != c.ok {
			t.Errorf("sessionSegment(%q, %q) = %q, %v; want %q, %v", c.prefix, c.key, got, ok, c.want, c.ok)
		}
	}
}

func TestSessionAfterIsNumericForSessionIds(t *testing.T) {
	// Text order would put "999" after "1000".
	if !sessionAfter("1000", "999") || sessionAfter("999", "1000") {
		t.Error("session ids must compare as numbers")
	}
	if !sessionAfter("b", "a") {
		t.Error("non-numeric session folders compare as text")
	}
}

func TestIsCumulativeRead(t *testing.T) {
	env := map[string]any{
		cumulativeObjectTypesEnv:      "MedicalClaim,PharmacyClaim",
		mixedCumulativeObjectTypesEnv: "Eligibility",
		"$OT":                         "PharmacyClaim",
	}
	for _, c := range []struct {
		objectType string
		want       bool
	}{
		{"PharmacyClaim", true},
		{"MedicalClaim", true},
		{"$OT", true},
		{"Pharmacy", false},
		{"", false},
	} {
		got, err := isCumulativeRead(c.objectType, env)
		if err != nil || got != c.want {
			t.Errorf("isCumulativeRead(%q) = %v, %v; want %v", c.objectType, got, err, c.want)
		}
	}
	if _, err := isCumulativeRead("Eligibility", env); err == nil {
		t.Error("an object type cumulative for some orgs only must be refused")
	}
	// Not loaded (a run started before the flag existed): never cumulative.
	if got, err := isCumulativeRead("PharmacyClaim", map[string]any{}); got || err != nil {
		t.Errorf("empty env: got %v, %v; want false", got, err)
	}
}

// Three months of a stage folder, the most recent first. Session 1792000000002
// has a folder in two periods, so that dropping it is shown to be by session
// rather than by period.
func deletedSessionsPeriods() []lookbackPeriodObjects {
	return []lookbackPeriodObjects{
		period(682, "stage/c/Claim/682M",
			"stage/c/Claim/682M/1792000000002/part-0001.csv",
			"stage/c/Claim/682M/1792000000002/part-0002.csv",
			"stage/c/Claim/682M/1792000000001/part-0001.csv",
			"stage/c/Claim/682M/part-0001.csv"), // directly in the period folder
		period(681, "stage/c/Claim/681M",
			"stage/c/Claim/681M/1792000000002/part-0001.csv",
			"stage/c/Claim/681M/1791000000001/part-0001.csv"),
		period(680, "stage/c/Claim/680M",
			"stage/c/Claim/680M/1790000000001/part-0001.csv"),
	}
}

func TestDropDeletedSessionsDropsTheSessionInEveryPeriod(t *testing.T) {
	periods := deletedSessionsPeriods()
	before := keysOf(allLookbackObjects(periods))
	got := dropDeletedSessions(periods, map[string]any{deletedSessionsEnv: "1700000000000,1792000000002"})
	want := []string{
		"stage/c/Claim/682M/1792000000001/part-0001.csv",
		"stage/c/Claim/682M/part-0001.csv",
		"stage/c/Claim/681M/1791000000001/part-0001.csv",
		"stage/c/Claim/680M/1790000000001/part-0001.csv",
	}
	if !slices.Equal(keysOf(allLookbackObjects(got)), want) {
		t.Errorf("got %v, want %v", keysOf(allLookbackObjects(got)), want)
	}
	for i := range got {
		if got[i].periodId != periods[i].periodId || got[i].prefix != periods[i].prefix {
			t.Errorf("period %d: got %d %s, want %d %s", i, got[i].periodId, got[i].prefix, periods[i].periodId, periods[i].prefix)
		}
	}
	// The periods passed in are left as they were.
	if after := keysOf(allLookbackObjects(periods)); !slices.Equal(after, before) {
		t.Errorf("input modified: got %v, want %v", after, before)
	}
}

func TestDropDeletedSessionsKeepsObjectsDirectlyInThePeriodFolder(t *testing.T) {
	periods := []lookbackPeriodObjects{
		period(682, "stage/c/Claim/682M",
			"stage/c/Claim/682M/part-0001.csv",
			"stage/c/Claim/682M/1792000000002/part-0001.csv"),
	}
	// A deleted id that is also an object's name directly in the folder: the
	// object has no session folder, so it is kept.
	periods[0].objects = append(periods[0].objects, &awsi.S3Object{Key: "stage/c/Claim/682M/1792000000002", Size: 10})
	got := dropDeletedSessions(periods, map[string]any{deletedSessionsEnv: "1792000000002"})
	want := []string{"stage/c/Claim/682M/part-0001.csv", "stage/c/Claim/682M/1792000000002"}
	if !slices.Equal(keysOf(allLookbackObjects(got)), want) {
		t.Errorf("got %v, want %v", keysOf(allLookbackObjects(got)), want)
	}
}

func TestDropDeletedSessionsMakesTheDeliveryBeforeTheLatest(t *testing.T) {
	// A cumulative read refuses an object directly in a period folder, so this
	// case has none.
	periods := deletedSessionsPeriods()
	periods[0].objects = periods[0].objects[:3]
	// The latest session of the latest period deleted: the session before it.
	got, err := latestDelivery(dropDeletedSessions(periods, map[string]any{deletedSessionsEnv: "1792000000002"}))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"stage/c/Claim/682M/1792000000001/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
}

func TestDropDeletedSessionsFallsBackToThePreviousPeriod(t *testing.T) {
	periods := []lookbackPeriodObjects{
		period(682, "stage/c/Claim/682M",
			"stage/c/Claim/682M/1792000000002/part-0001.csv",
			"stage/c/Claim/682M/1792000000002/part-0002.csv"),
		period(681, "stage/c/Claim/681M",
			"stage/c/Claim/681M/1791000000002/part-0001.csv",
			"stage/c/Claim/681M/1791000000001/part-0001.csv"),
	}
	// The latest period's only session deleted: the latest session of the period before.
	got, err := latestDelivery(dropDeletedSessions(periods, map[string]any{deletedSessionsEnv: "1792000000002"}))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"stage/c/Claim/681M/1791000000002/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
	// And that one deleted too: the session before it in the same period.
	got, err = latestDelivery(dropDeletedSessions(periods,
		map[string]any{deletedSessionsEnv: "1791000000002,1792000000002"}))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"stage/c/Claim/681M/1791000000001/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
	// Every session deleted: nothing to read.
	got, err = latestDelivery(dropDeletedSessions(periods,
		map[string]any{deletedSessionsEnv: "1791000000001,1791000000002,1792000000002"}))
	if err != nil || got != nil {
		t.Errorf("got %v, %v; want nothing", keysOf(got), err)
	}
}

func TestDropDeletedSessionsWithNoListChangesNothing(t *testing.T) {
	want := keysOf(allLookbackObjects(deletedSessionsPeriods()))
	for name, env := range map[string]map[string]any{
		"absent (a run started by an older build)": {},
		"empty (no user deletion)":                 {deletedSessionsEnv: ""},
		"nil env":                                  nil,
	} {
		got := dropDeletedSessions(deletedSessionsPeriods(), env)
		if !slices.Equal(keysOf(allLookbackObjects(got)), want) {
			t.Errorf("%s: got %v, want %v", name, keysOf(allLookbackObjects(got)), want)
		}
	}
}

// A stage folder listed directly (a stage channel with a file_key and no
// lookback_periods): one period folder holding several sessions' folders.
func TestDropDeletedSessionObjectsInADirectlyListedFolder(t *testing.T) {
	folder := period(682, "stage/c/Eligibility/682M",
		"stage/c/Eligibility/682M/1792000000002/jets_partition=0p/part-0001.csv",
		"stage/c/Eligibility/682M/1792000000002/jets_partition=1p/part-0001.csv",
		"stage/c/Eligibility/682M/1792000000001/jets_partition=0p/part-0001.csv",
		"stage/c/Eligibility/682M/part-0001.csv")
	before := keysOf(folder.objects)
	env := map[string]any{deletedSessionsEnv: "1792000000002"}
	got := dropDeletedSessionObjects(folder.prefix, folder.objects, env)
	want := []string{
		"stage/c/Eligibility/682M/1792000000001/jets_partition=0p/part-0001.csv",
		"stage/c/Eligibility/682M/part-0001.csv",
	}
	if !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
	if after := keysOf(folder.objects); !slices.Equal(after, before) {
		t.Errorf("input modified: got %v, want %v", after, before)
	}
	// The prefix as ListS3Objects is given it, with a leading '/'.
	if got := dropDeletedSessionObjects("/"+folder.prefix, folder.objects, env); !slices.Equal(keysOf(got), want) {
		t.Errorf("leading '/': got %v, want %v", keysOf(got), want)
	}
	for name, env := range map[string]map[string]any{
		"absent": {}, "empty": {deletedSessionsEnv: ""}, "only commas": {deletedSessionsEnv: ", ,"}, "nil env": nil,
	} {
		if got := dropDeletedSessionObjects(folder.prefix, folder.objects, env); !slices.Equal(keysOf(got), before) {
			t.Errorf("%s: got %v, want %v", name, keysOf(got), before)
		}
	}
}

// When the prefix is a session's own folder, the segment below it is a partition
// label (or a file name), never a session id: nothing is dropped, even with the
// session itself, or a label-like id, in the list.
func TestDropDeletedSessionObjectsNeverMatchesBelowASessionFolder(t *testing.T) {
	folder := period(682, "stage/c/Eligibility/682M/1792000000002",
		"stage/c/Eligibility/682M/1792000000002/jets_partition=0p/part-0001.csv",
		"stage/c/Eligibility/682M/1792000000002/jets_partition=1p/part-0001.csv",
		"stage/c/Eligibility/682M/1792000000002/part-0001.csv")
	env := map[string]any{deletedSessionsEnv: "1792000000002,0p,1p"}
	if got := dropDeletedSessionObjects(folder.prefix, folder.objects, env); !slices.Equal(keysOf(got), keysOf(folder.objects)) {
		t.Errorf("got %v, want every object kept", keysOf(got))
	}
	// The segment really is the partition label, which no session id equals.
	if seg, ok := sessionSegment(folder.prefix, folder.objects[0].Key); !ok || seg != "jets_partition=0p" {
		t.Errorf("segment below a session folder: got %q, %v", seg, ok)
	}
	// An explicit-session read (process_name=…/session_id=…/step_id=…/) likewise.
	prefix := "stage/process_name=p/session_id=1792000000002/step_id=reducing01/"
	objs := period(0, prefix, prefix+"jets_partition=0p/part-0001.csv").objects
	if got := dropDeletedSessionObjects(prefix, objs, env); len(got) != 1 {
		t.Errorf("explicit-session read: got %v, want the object kept", keysOf(got))
	}
}
