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
