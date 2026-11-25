package feedsummary

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// allowedStrings lists every string field of Input and where its value comes
// from. A string field that is not listed fails the test, so adding one is a
// decision someone has to write down.
var allowedStrings = map[string]string{
	"Input.IncidentID":        "the incident's identifier, inc_<session>_<locus>",
	"Input.Locus":             "closed vocabulary: triage.DeliveryLoci",
	"Feed.Table":              "source_config.table_name",
	"Feed.Client":             "source_config.client",
	"Feed.Org":                "source_config.org",
	"Feed.ObjectType":         "source_config.object_type",
	"Feed.RegisteredUnder":    "input_registry.client of a registration",
	"Declaration.DeclaredBy":  "the profile's declared_by",
	"Declaration.PeriodType":  "closed vocabulary: feedprofile.PeriodTypes",
	"Finding.Detector":        "closed vocabulary: feedprofile.Detectors",
	"Finding.Signal":          "closed vocabulary: observe's signal types",
	"Finding.Column":          "a required field the profile names, checked against it on read",
	"Finding.ObservedUnit":    "closed vocabulary: this package's units",
	"Finding.BoundUnit":       "closed vocabulary: this package's units",
	"Reading.Cause":           "closed vocabulary: rca's ten cause classes",
	"Reading.Evidenceability": "closed vocabulary: rca's grades",
	"Condition.End":           "closed vocabulary: this package's ends",
	"Condition.EndedInto":     "an incident identifier at the same locus",
	"Condition.Verdict":       "closed vocabulary: audit.VerdictStatuses",
	"Condition.Others":        "incident identifiers at the same locus",
	"Previous.Kind":           "closed vocabulary: this package's previous kinds",
	"Previous.IncidentID":     "an incident identifier at the same locus",
	"Previous.Verdict":        "closed vocabulary: audit.VerdictStatuses",
}

// TestTheInputHasNoFieldThatCanCarryARow walks Input by reflection. No field is
// an interface, a map or a byte slice, and every string is one of the listed
// sources, none of which is a delivery's row.
func TestTheInputHasNoFieldThatCanCarryARow(t *testing.T) {
	seen := map[string]bool{}
	var walk func(rt reflect.Type, owner string)
	walk = func(rt reflect.Type, owner string) {
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			name := owner + "." + f.Name
			ft := f.Type
			for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice {
				if ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Uint8 {
					t.Errorf("%s is a byte slice, which can carry anything", name)
				}
				ft = ft.Elem()
			}
			switch ft.Kind() {
			case reflect.Interface, reflect.Map, reflect.Chan, reflect.Func:
				t.Errorf("%s is a %s, which can carry a row", name, ft.Kind())
			case reflect.String:
				seen[name] = true
				if _, ok := allowedStrings[name]; !ok {
					t.Errorf("%s is a string with no recorded source; add it to allowedStrings with its source, "+
						"or make it a number or a closed vocabulary", name)
				}
			case reflect.Struct:
				if ft == reflect.TypeOf(time.Time{}) {
					continue
				}
				walk(ft, ft.Name())
			}
		}
	}
	walk(reflect.TypeOf(Input{}), "Input")
	for name := range allowedStrings {
		if !seen[name] {
			t.Errorf("allowedStrings lists %s, which is not a string field of Input", name)
		}
	}
}

func TestTheDocumentCompiles(t *testing.T) {
	tm, err := Template()
	if err != nil {
		t.Fatal(err)
	}
	if tm.Key() != Key {
		t.Fatalf("key %q", tm.Key())
	}
}

func f64(x float64) *float64 { return &x }
func i(x int) *int           { return &x }

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func departs() *Input {
	return &Input{
		IncidentID: "inc_1747000000000_delivery_departs_profile",
		Locus:      triage.LocusDeliveryDepartsProfile,
		DetectedAt: time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC),
		Feed: Feed{Table: "acme_pharmacy", Configured: true, Client: "acme", Org: "east",
			ObjectType: "PharmacyClaim", RegisteredUnder: "Any"},
		Declaration: Declaration{Readable: true, DeclaredBy: "ops@example.com", DeclaredAt: day(2026, 1, 15),
			AgeDays: 137, PeriodType: feedprofile.WeekPeriod, PeriodsPerDelivery: 1},
		Findings: []Finding{
			{Detector: feedprofile.DeliverySize, Generation: 1, Signal: observe.SignalVolume,
				Observed: f64(1200), ObservedUnit: UnitBytes, Bound: f64(40000), BoundIsMin: true,
				Upper: f64(90000), BoundUnit: UnitBytes, Magnitude: f64(0.97)},
			{Detector: feedprofile.FieldCompleteness, Generation: 1, Signal: observe.SignalRuleBreach,
				Column: "paid_amount", Observed: f64(0.123), ObservedUnit: UnitShare, Bound: f64(0.95),
				BoundIsMin: true, BoundUnit: UnitShare},
			{Detector: feedprofile.DeliveryUnchanged, Generation: 1, Signal: observe.SignalCardinality,
				Equal: true},
		},
		Readings: []Reading{
			{Rank: 1, Cause: rca.CauseSourceContentChange, Confidence: 0.55, Supporting: 3, Contradicting: 1,
				Evidenceability: string(rca.Conditional)},
			{Rank: 2, Cause: rca.CauseTransformationDefect, Confidence: 0.3, Supporting: 3, Contradicting: 1,
				Evidenceability: string(rca.Coarse)},
			{Rank: 3, Cause: rca.CauseBenignVariation, Confidence: 0.15, Supporting: 0, Contradicting: 1,
				Evidenceability: string(rca.Asymmetric)},
		},
		Condition: &Condition{Held: 3, Total: 3, First: day(2026, 5, 11), Last: day(2026, 5, 25), End: EndOpen,
			Previous: Previous{Kind: PrevCondition, IncidentID: "inc_1740000000000_delivery_departs_profile",
				Verdict: audit.IncidentSuppressedAsBenign, VerdictAt: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
				Deliveries: 2, First: day(2026, 2, 16), Last: day(2026, 2, 23)}},
	}
}

func TestRendersADeparture(t *testing.T) {
	s, err := Render(departs())
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + s.Text)
	for _, want := range []string{
		"A delivery of feed acme_pharmacy departs from the feed's declared profile",
		"client acme, org east, object type PharmacyClaim, an incremental feed",
		"Found on 1 June 2026.",
		"registered under client Any, not the feed's own",
		"The delivery is 1,200 bytes, below the expected minimum of 40,000 bytes.",
		"The required field paid_amount carries a value in 12.3% of the delivery's rows; the declaration requires at least 95.0%.",
		"exactly the same business keys",
		"1. the source changed what it sends (source_content_change), confidence 0.55, three items of evidence for and one against, graded conditional",
		"3. an ordinary variation",
		"declared by ops@example.com on 15 January 2026, one hundred and thirty-seven days before this was found",
		"This incident stands for three deliveries of the feed with a finding at this locus, registered from 11 May 2026 to 25 May 2026.",
		"The findings above are those of the delivery the incident is written on",
		"It is still open: the feed's latest delivery carries the finding.",
		"The feed's previous condition at this locus, two deliveries registered from 16 February 2026 to 23 February 2026, has incident inc_1740000000000_delivery_departs_profile, with a person's verdict, suppressed_as_benign on 2 March 2026.",
		"A condition is one fault that persists, raised as one incident: give it one verdict.",
		"No model wrote it.",
	} {
		if !strings.Contains(strings.Join(strings.Fields(s.Text), " "), want) {
			t.Errorf("the summary does not say %q", want)
		}
	}
	if len(s.Violations) != 0 {
		t.Errorf("violations: %v", s.Violations)
	}
}

// The span check reads as the span in days against the declared span, or, where
// the profile declares max_out_of_span_share, as the share of dated rows
// outside the span against that share.
func TestRendersBothShapesOfTheSpanCheck(t *testing.T) {
	for _, c := range []struct {
		f    Finding
		want string
	}{
		{Finding{Detector: feedprofile.ServiceDateSpan, Generation: 1, Signal: observe.SignalDistribution,
			Observed: f64(412), ObservedUnit: UnitDays, Bound: f64(62), BoundUnit: UnitDays},
			"The dates of service span 412 days, newest less oldest; the declaration allows at most 62 days."},
		{Finding{Detector: feedprofile.ServiceDateSpan, Generation: 1, Signal: observe.SignalDistribution,
			Observed: f64(0.125), ObservedUnit: UnitShare, Bound: f64(0.05), BoundUnit: UnitShare},
			"12.5% of the delivery's rows with a date of service are dated further before its newest date of " +
				"service than the declared span; the declaration allows at most 5.0%."},
	} {
		in := departs()
		in.Findings = []Finding{c.f}
		in.Condition = nil
		s, err := Render(in)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(strings.Fields(s.Text), " "), c.want) {
			t.Errorf("the summary does not say %q:\n%s", c.want, s.Text)
		}
		if strings.Contains(s.Text, "could not be read back") {
			t.Errorf("the %s finding was not read:\n%s", c.f.ObservedUnit, s.Text)
		}
		if n := CountUntrue(s.Text, in); n.Untrue() != 0 {
			t.Errorf("the counter finds untrue statements in the render:\n%s\n%s", n, s.Text)
		}
	}
}

func TestRendersAnOverdueFeed(t *testing.T) {
	in := &Input{
		IncidentID: "inc_1747000000000_delivery_overdue",
		Locus:      triage.LocusDeliveryOverdue,
		DetectedAt: time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC),
		Feed:       Feed{Table: "acme_medical", Configured: true, Client: "acme", ObjectType: "MedicalClaim"},
		Declaration: Declaration{Readable: true, DeclaredBy: "ops@example.com", DeclaredAt: day(2026, 1, 15),
			AgeDays: 137, PeriodType: feedprofile.WeekPeriod, PeriodsPerDelivery: 1, OverdueAfterDays: i(3)},
		Findings: []Finding{{Detector: feedprofile.FeedOverdue, Generation: 1, Signal: observe.SignalArrival,
			ObservedPeriod: day(2026, 4, 27), OverdueFrom: day(2026, 5, 7), Magnitude: f64(25.2)}},
		Readings: []Reading{{Rank: 1, Cause: rca.CauseSourceDeliveryFailure, Confidence: 0.7, Supporting: 2,
			Contradicting: 1, Evidenceability: string(rca.Conditional)}},
	}
	s, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + s.Text)
	flat := strings.Join(strings.Fields(s.Text), " ")
	for _, want := range []string{
		"Feed acme_medical is overdue (client acme, object type MedicalClaim, an incremental feed).",
		"No delivery has arrived since the one for the period starting 27 April 2026; under the declared cadence the next became overdue on 7 May 2026, twenty-five days before this was found.",
		"The declaration expects a delivery every week, with three days of grace.",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the summary does not say %q", want)
		}
	}
	if strings.Contains(flat, "registered under") || strings.Contains(flat, "This incident stands for") {
		t.Errorf("the summary says something the input does not carry:\n%s", s.Text)
	}
}

func TestSaysWhatItCannotRead(t *testing.T) {
	in := departs()
	in.Declaration = Declaration{}
	in.Findings[0].Observed = nil
	s, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(s.Text), " ")
	for _, want := range []string{
		"The delivery_size check fired; what it measured could not be read back as a number.",
		"the feed's profile can no longer be read",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the summary does not say %q:\n%s", want, s.Text)
		}
	}
}

func TestADeclarationMadeAfterTheFindingIsNamedAsSuch(t *testing.T) {
	in := departs()
	in.Declaration.DeclaredAt = day(2026, 7, 1)
	in.Declaration.AgeDays = -30
	s, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(strings.Fields(s.Text), " "), "after this was found; the profile has been declared again since") {
		t.Errorf("a later declaration is not named as later:\n%s", s.Text)
	}
}

func TestValidateRefusesAValueOutsideItsVocabulary(t *testing.T) {
	for name, mutate := range map[string]func(*Input){
		"locus":              func(in *Input) { in.Locus = "worker_failed" },
		"detector":           func(in *Input) { in.Findings[0].Detector = "free text" },
		"cause":              func(in *Input) { in.Readings[0].Cause = "something" },
		"unit":               func(in *Input) { in.Findings[0].ObservedUnit = "zq7" },
		"column":             func(in *Input) { in.Findings[0].Column = "paid_amount" },
		"condition incident": func(in *Input) { in.Condition.Others = []string{"a value"} },
		"previous incident":  func(in *Input) { in.Condition.Previous.IncidentID = "inc_1_worker_failed" },
		"end":                func(in *Input) { in.Condition.End = "finished" },
		"previous kind":      func(in *Input) { in.Condition.Previous.Kind = "earlier" },
		"verdict":            func(in *Input) { in.Condition.Previous.Verdict = audit.IncidentClosed },
	} {
		in := departs()
		mutate(in)
		if _, err := Render(in); err == nil {
			t.Errorf("%s: a value outside its vocabulary was rendered", name)
		}
	}
}

func TestParsesWhatEachDetectorWrites(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, c := range []struct {
		a     observe.Anomaly
		check func(Finding) bool
	}{
		{observe.Anomaly{DetectorRef: "delivery_size@2", SignalType: observe.SignalVolume, ObservedValue: "1200",
			ExpectedMin: str("40000"), ExpectedMax: str("90000")},
			func(f Finding) bool {
				return *f.Observed == 1200 && *f.Bound == 40000 && *f.Upper == 90000 && f.Generation == 2
			}},
		{observe.Anomaly{DetectorRef: "service_date_stale@1", ObservedValue: "3 calendar months", ExpectedMax: str("45 days")},
			func(f Finding) bool {
				return *f.Observed == 3 && f.ObservedUnit == UnitCalendarMonths && *f.Bound == 45
			}},
		{observe.Anomaly{DetectorRef: "delivery_overlap@1", ObservedValue: "12 keys absent", ExpectedMax: str("0 keys absent")},
			func(f Finding) bool { return *f.Observed == 12 && f.ObservedUnit == UnitKeysAbsent && f.Bound == nil }},
		{observe.Anomaly{DetectorRef: "feed_overdue@1", ObservedValue: "2026-04-27", ExpectedMax: str("2026-05-07T00:00:00Z")},
			func(f Finding) bool {
				return f.ObservedPeriod.Equal(day(2026, 4, 27)) && f.OverdueFrom.Equal(day(2026, 5, 7))
			}},
		{observe.Anomaly{DetectorRef: "delivery_unchanged@1", ObservedValue: "equal"},
			func(f Finding) bool { return f.Equal }},
		{observe.Anomaly{DetectorRef: "service_date_span@1", ObservedValue: "412 days", ExpectedMax: str("62 days")},
			func(f Finding) bool {
				return *f.Observed == 412 && f.ObservedUnit == UnitDays && *f.Bound == 62 && f.BoundUnit == UnitDays
			}},
		{observe.Anomaly{DetectorRef: "service_date_span@1", ObservedValue: "0.1250", ExpectedMax: str("0.0500")},
			func(f Finding) bool {
				return *f.Observed == 0.125 && f.ObservedUnit == UnitShare && *f.Bound == 0.05 && f.BoundUnit == UnitShare
			}},
		// A span in days against a share is not one comparison; it is left unread.
		{observe.Anomaly{DetectorRef: "service_date_span@1", ObservedValue: "412 days", ExpectedMax: str("0.0500")},
			func(f Finding) bool { return f.Observed == nil && f.Bound == nil }},
	} {
		f, ok := parseFinding(c.a, nil)
		if !ok || !c.check(f) {
			t.Errorf("%s %q parsed as %+v", c.a.DetectorRef, c.a.ObservedValue, f)
		}
	}
	// A completeness finding on a column the profile does not name is dropped.
	if _, ok := parseFinding(observe.Anomaly{DetectorRef: "field_completeness@1", AnomalyId: "x/t/s/zq7canarykey",
		ObservedValue: "0.1"}, []string{"paid_amount"}); ok {
		t.Error("a completeness finding on an undeclared column was kept")
	}
	// A value of any other shape keeps nothing.
	f, _ := parseFinding(observe.Anomaly{DetectorRef: "service_date_span@1", ObservedValue: "zq7canarymbr"}, nil)
	if f.Observed != nil {
		t.Error("a value that is not a quantity was kept")
	}
}

// TestTheCounterFindsNothingInTheRender is the negative control on hand-built
// inputs; the fixture test repeats it on every fixture incident.
func TestTheCounterFindsNothingInTheRender(t *testing.T) {
	for _, in := range []*Input{departs()} {
		s, err := Render(in)
		if err != nil {
			t.Fatal(err)
		}
		if c := CountUntrue(s.Text, in); c.Untrue() != 0 {
			t.Errorf("the counter finds untrue statements in the render:\n%s\n%s", c, s.Text)
		}
	}
}

func TestTheCounterFindsWhatIsUntrue(t *testing.T) {
	in := departs()
	draft := "Feed acme_pharmacy was found on 2 June 2026 to be overdue; it is 1,300 bytes. " +
		"The likeliest reading is source_delivery_failure, from the feed_overdue check. " +
		"The profile was declared by someone@example.com. See inc_999_delivery_departs_profile. " +
		"Paid amount is present in 12.3% of rows."
	c := CountUntrue(draft, in)
	want := map[string]int{CatDate: 1, CatOverdue: 1, CatNumber: 1, CatCause: 1, CatDetector: 1, CatDeclarer: 1,
		CatIdentifier: 1, CatOrder: 0, CatDeliveries: 0, CatOpen: 0, CatVerdict: 0}
	for _, cat := range c.Categories {
		if len(cat.Findings) != want[cat.Name] {
			t.Errorf("%s: %d findings %v, want %d", cat.Name, len(cat.Findings), cat.Findings, want[cat.Name])
		}
	}
	if len(c.Categories) != len(want) {
		t.Errorf("%d categories, want %d", len(c.Categories), len(want))
	}
}

// TestARestatedThresholdIsNotAnOverdueClaim: a draft that restates the
// declared overdue threshold says what the declaration says, and is not a
// claim that the feed is overdue. The restatements are phrasings a model
// produced when drafting a summary for a delivery the overdue check did not
// fire on; a claim beside one is still found.
func TestARestatedThresholdIsNotAnOverdueClaim(t *testing.T) {
	declared := departs()
	declared.Declaration.OverdueAfterDays = i(3)
	none := departs()
	fired := departs()
	fired.Declaration.OverdueAfterDays = i(3)
	fired.Findings = append(fired.Findings, Finding{Detector: feedprofile.FeedOverdue, Generation: 1,
		Signal: observe.SignalArrival, ObservedPeriod: day(2026, 4, 27), OverdueFrom: day(2026, 5, 7)})
	for _, tc := range []struct {
		name  string
		in    *Input
		draft string
		want  int
	}{
		// The restatements, as a model wrote them.
		{"per period", declared, "Locus is delivery_departs_profile. Overdue after 3 days per period (week).", 0},
		{"per week", declared, "Declaration authored by ops@example.com. Overdue after 3 days per week.", 0},
		{"in a list", declared, "Period: weekly, overdue after 3 days.", 0},
		{"with the cadence", declared, "Overdue after 3 days, with 1 period per delivery.", 0},
		{"attached to a finding", declared, "Declaration configured but paid_amount had overdue after 3 days.", 0},
		// Other ways of stating the same threshold.
		{"threshold of", declared, "The declaration sets an overdue threshold of 3 days.", 0},
		{"threshold is", declared, "The overdue threshold is 3 days.", 0},
		{"grace", declared, "The overdue grace is 3 days.", 0},
		{"n-day", declared, "The feed has a 3-day overdue threshold.", 0},
		{"one day", func() *Input { in := departs(); in.Declaration.OverdueAfterDays = i(1); return in }(),
			"Overdue after 1 day.", 0},
		// Claims that the feed is overdue, which it is not.
		{"has been overdue", declared, "It was declared on 15 January 2026 and has been overdue for 137 days.", 1},
		{"is overdue", declared, "Feed acme_pharmacy is overdue.", 1},
		{"became overdue after", declared, "The feed became overdue after 3 days.", 1},
		{"has been overdue after", declared, "The feed has been overdue after 3 days without a delivery.", 1},
		{"claim beside a restatement", declared,
			"It has been overdue for 137 days. Overdue after 3 days, with 1 period per delivery.", 1},
		// A threshold that is not the declared one is not a restatement.
		{"another threshold", declared, "Overdue after 5 days per week.", 1},
		{"none declared", none, "Overdue after 3 days per week.", 1},
		// The check fired: every mention is true.
		{"fired, claim", fired, "Feed acme_pharmacy is overdue and has been overdue for 25 days.", 0},
		{"fired, restatement", fired, "Overdue after 3 days per week.", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, cat := range CountUntrue(tc.draft, tc.in).Categories {
				if cat.Name == CatOverdue && len(cat.Findings) != tc.want {
					t.Errorf("%q: %d findings %v, want %d", tc.draft, len(cat.Findings), cat.Findings, tc.want)
				}
			}
		})
	}
}

func TestThePromptIsTheDocument(t *testing.T) {
	in := departs()
	_, user, err := Prompt(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"Feed_Table": "acme_pharmacy"`, `"Declared_By": "ops@example.com"`} {
		if !strings.Contains(user, s) {
			t.Errorf("the prompt does not carry %s", s)
		}
	}
}
