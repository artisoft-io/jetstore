package feedprofile

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// full declares every field any detector needs.
func full() Profile {
	return Profile{
		PeriodType: WeekPeriod, OverdueAfterDays: ptr(3),
		ClaimSet:              &ClaimSet{Table: "t", KeyColumns: []string{"k"}, ServiceDateColumn: "d", MemberColumn: "m"},
		MaxServiceDateLagDays: ptr(30), MaxServiceDateSpanDays: ptr(120), MaxRedeliveredShare: ptr(0.05),
		RequiredFields: []RequiredField{{Column: "c", MinPresentShare: ptr(0.9)}},
		DeclaredBy:     "someone", DeclaredAt: "2026-03-01",
	}
}

func TestEveryDetectorIsInTheTable(t *testing.T) {
	if len(Detectors) != 8 {
		t.Fatalf("%d detectors, want 8", len(Detectors))
	}
	for _, d := range Detectors {
		if _, ok := required[d]; !ok {
			t.Errorf("%s has no row in the required-field table", d)
		}
	}
	if len(required) != len(Detectors) {
		t.Errorf("the table has %d rows for %d detectors", len(required), len(Detectors))
	}
	if _, err := Missing("no_such_detector", full(), false); err == nil {
		t.Error("an unknown detector is not an error")
	}
}

// Every field the table names is a field of the profile document, so a
// renamed JSON tag cannot leave the table naming a field nothing declares.
func TestTheTableNamesProfileFields(t *testing.T) {
	buf, err := json.Marshal(full())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(buf, &doc); err != nil {
		t.Fatal(err)
	}
	for _, d := range Detectors {
		for _, cumulative := range []bool{false, true} {
			fields, _ := Required(d, cumulative)
			for _, f := range fields {
				if _, ok := doc[f]; !ok {
					t.Errorf("%s requires %q, which is not a profile field", d, f)
				}
			}
		}
	}
}

func TestAFullProfileLacksNothing(t *testing.T) {
	for _, d := range Detectors {
		for _, cumulative := range []bool{false, true} {
			if m, _ := Missing(d, full(), cumulative); len(m) != 0 {
				t.Errorf("%s (cumulative %v) says a full profile lacks %v", d, cumulative, m)
			}
		}
	}
}

func TestWhatAnEmptyProfileLacks(t *testing.T) {
	empty := Profile{DeclaredBy: "someone", DeclaredAt: "2026-03-01"}
	want := map[string][]string{
		FeedOverdue:       {FieldPeriodType, FieldOverdueAfterDays},
		DeliverySize:      nil,
		DeliveryBackdated: {FieldPeriodType},
		ServiceDateStale:  {FieldMaxServiceDateLagDays, FieldClaimSet},
		ServiceDateSpan:   {FieldMaxServiceDateSpanDays, FieldClaimSet},
		FieldCompleteness: {FieldRequiredFields, FieldClaimSet},
		DeliveryOverlap:   {FieldMaxRedeliveredShare, FieldClaimSet},
		DeliveryUnchanged: {FieldClaimSet},
	}
	for d, w := range want {
		got, _ := Missing(d, empty, false)
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s over an empty incremental profile lacks %v, want %v", d, got, w)
		}
	}
	// A cumulative feed's overlap check reads containment, which needs no share.
	if got, _ := Missing(DeliveryOverlap, empty, true); !reflect.DeepEqual(got, []string{FieldClaimSet}) {
		t.Errorf("delivery_overlap over an empty cumulative profile lacks %v", got)
	}
}

// The threshold a detector owns comes before claim_set, so a profile lacking
// both is reported by the field specific to the detector.
func TestTheSpecificFieldComesFirst(t *testing.T) {
	for _, d := range Detectors {
		for _, cumulative := range []bool{false, true} {
			fields, _ := Required(d, cumulative)
			if i := slices.Index(fields, FieldClaimSet); i >= 0 && i != len(fields)-1 {
				t.Errorf("%s names claim_set before %v", d, fields[i+1:])
			}
		}
	}
}

func TestOnlySpanSkipsACumulativeFeed(t *testing.T) {
	for _, d := range Detectors {
		inc, _ := Applies(d, false)
		cum, _ := Applies(d, true)
		if !inc {
			t.Errorf("%s does not judge an incremental feed", d)
		}
		if cum == (d == ServiceDateSpan) {
			t.Errorf("%s judges a cumulative feed: %v", d, cum)
		}
	}
}

// An empty required_fields list is a field not declared, not a declaration
// that nothing is required.
func TestAnEmptyRequiredFieldListIsMissing(t *testing.T) {
	p := full()
	p.RequiredFields = []RequiredField{}
	if got, _ := Missing(FieldCompleteness, p, false); strings.Join(got, ",") != FieldRequiredFields {
		t.Errorf("an empty required_fields lacks %v", got)
	}
}
