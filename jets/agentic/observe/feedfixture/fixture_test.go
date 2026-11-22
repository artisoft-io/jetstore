package feedfixture

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
)

// These tests need no database. They check the fixture against itself: that it
// is reproducible, that the written expectation is complete, and that the
// canaries are where a leak test will look for them.

func TestFeedsAreDeterministic(t *testing.T) {
	if !reflect.DeepEqual(Feeds(), Feeds()) {
		t.Fatal("two calls to Feeds returned different fixtures")
	}
}

func TestPeriodAgreesWithJetStore(t *testing.T) {
	seen := 0
	for _, f := range Feeds() {
		for _, d := range f.Deliveries {
			m, w, dp := Period(d.Period)
			wm, ww, wd := datatable.CalculatePeriod(d.Period.Year(), int(d.Period.Month()), d.Period.Day())
			if m != wm || w != ww || dp != wd {
				t.Errorf("%s: Period gives (%d, %d, %d), CalculatePeriod (%d, %d, %d)",
					d.Period.Format(time.DateOnly), m, w, dp, wm, ww, wd)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("no delivery checked")
	}
}

func TestSessionsAreUniqueAndAreTheArrival(t *testing.T) {
	seen := map[string]string{}
	for _, f := range Feeds() {
		for i, d := range f.Deliveries {
			if d.Ordinal != i+1 {
				t.Errorf("%s: delivery %d has ordinal %d", f.TableName, i+1, d.Ordinal)
			}
			if other, dup := seen[d.SessionID]; dup {
				t.Errorf("session %s is %s and %s #%d", d.SessionID, other, f.TableName, d.Ordinal)
			}
			seen[d.SessionID] = f.TableName
			ms, err := strconv.ParseInt(d.SessionID, 10, 64)
			if err != nil || !time.UnixMilli(ms).UTC().Equal(d.Registered) {
				t.Errorf("%s #%d: session %s is not the registration %s", f.TableName, d.Ordinal, d.SessionID, d.Registered)
			}
			if i > 0 && !d.Registered.After(f.Deliveries[i-1].Registered) {
				t.Errorf("%s #%d is not registered after #%d", f.TableName, d.Ordinal, i)
			}
			if !d.Registered.Before(Clock) {
				t.Errorf("%s #%d is registered after the clock", f.TableName, d.Ordinal)
			}
		}
	}
}

// A stale cumulative feed's deliveries carry the same claims and are never
// byte-identical: the sizes differ, and so does a non-business column.
func TestRepeatedDeliveriesDifferOutsideTheBusinessColumns(t *testing.T) {
	f, _ := FeedByTable(StaleCumulative)
	for i := 2; i < len(f.Deliveries); i++ {
		a, b := f.Deliveries[i-1], f.Deliveries[i]
		if a.FileSize == b.FileSize {
			t.Errorf("#%d and #%d have the same size", a.Ordinal, b.Ordinal)
		}
		if len(a.Claims) != len(b.Claims) {
			t.Fatalf("#%d and #%d differ in rows", a.Ordinal, b.Ordinal)
		}
		for j := range a.Claims {
			x, y := a.Claims[j], b.Claims[j]
			if x.LoadBatch == y.LoadBatch {
				t.Fatalf("#%d and #%d share a load batch", a.Ordinal, b.Ordinal)
			}
			x.LoadBatch, y.LoadBatch = "", ""
			if !reflect.DeepEqual(x, y) {
				t.Fatalf("#%d and #%d differ in a business column at row %d", a.Ordinal, b.Ordinal, j)
			}
		}
	}
}

func TestEveryMemberAndKeyCarriesItsCanary(t *testing.T) {
	n := 0
	for _, f := range Feeds() {
		for _, d := range f.Deliveries {
			for _, c := range d.Claims {
				n++
				if !strings.Contains(c.MemberID, MemberCanary) {
					t.Fatalf("%s: member %q has no canary", f.TableName, c.MemberID)
				}
				if !strings.Contains(c.ClaimID, KeyCanary) {
					t.Fatalf("%s: key %q has no canary", f.TableName, c.ClaimID)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no claim checked")
	}
}

// The canaries must be findable only in the claim values: a canary in a name or
// in the written documents would make a leak test report a leak
// that is not one.
func TestTheCanariesAreNowhereButInTheValues(t *testing.T) {
	var docs []string
	for _, name := range []string{"testdata/expected.json", "testdata/feed_profiles.json"} {
		buf, err := testdata.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, string(buf))
	}
	for _, f := range Feeds() {
		docs = append(docs, f.Client, f.ObjectType, f.TableName, f.ClaimTable)
		for _, d := range f.Deliveries {
			docs = append(docs, d.FileKey, d.SessionID)
		}
	}
	docs = append(docs, ColClaimID, ColLineNumber, ColServiceDate, ColMember, ColPaidAmount, ColLoadBatch, DomainKey)
	for _, doc := range docs {
		for _, c := range Canaries() {
			if strings.Contains(doc, c) {
				t.Errorf("canary %s outside the claim values: %.80q", c, doc)
			}
		}
	}
}

func TestTheExpectationIsComplete(t *testing.T) {
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	if !e.Clock.Equal(Clock) {
		t.Errorf("expected.json's clock %s is not Clock %s", e.Clock, Clock)
	}
	for _, det := range Detectors {
		if e.Rules[det] == "" {
			t.Errorf("no rule written for %s", det)
		}
	}
	feeds := Feeds()
	if len(e.Feeds) != len(feeds) {
		t.Fatalf("expected.json names %d feeds, the fixture has %d", len(e.Feeds), len(feeds))
	}
	for i, f := range feeds {
		ef := e.Feeds[i]
		if ef.TableName != f.TableName || ef.Client != f.Client || ef.Cumulative != f.Cumulative ||
			ef.Deliveries != len(f.Deliveries) {
			t.Errorf("feed %d: expected.json says %+v, the fixture %s/%s cumulative=%v with %d deliveries",
				i, ef, f.Client, f.TableName, f.Cumulative, len(f.Deliveries))
		}
	}
	if want := len(Detectors) * len(feeds); len(e.Verdicts) != want {
		t.Errorf("%d verdicts, want one per detector and feed, %d", len(e.Verdicts), want)
	}
	reasons := func(r string) bool {
		if strings.HasPrefix(r, "field_absent:") {
			return true
		}
		_, ok := e.NotJudgedReasons[r]
		return ok
	}
	for _, det := range Detectors {
		for _, f := range feeds {
			v, ok := e.Of(det, f.TableName)
			if !ok {
				t.Errorf("no verdict for %s over %s", det, f.TableName)
				continue
			}
			if v.NotJudged != "" && !reasons(v.NotJudged) {
				t.Errorf("%s over %s: unknown reason %q", det, f.TableName, v.NotJudged)
			}
			if det == "feed_overdue" {
				if v.NotJudged == "" && v.AtClock != Fires && v.AtClock != Silent {
					t.Errorf("feed_overdue over %s: at_clock %q", f.TableName, v.AtClock)
				}
				if v.AtClock == Fires && (v.OnDelivery != len(f.Deliveries) || v.FirstFiresAt == nil) {
					t.Errorf("feed_overdue over %s fires without its last delivery and first instant", f.TableName)
				}
				continue
			}
			for _, o := range append(append([]int{}, v.FiresOn...), ordinalsOf(v.NotJudgedOn)...) {
				if o < 1 || o > len(f.Deliveries) {
					t.Errorf("%s over %s: delivery %d out of range", det, f.TableName, o)
				}
			}
			for _, o := range v.NotJudgedOn {
				if !reasons(o.Reason) || slices.Contains(v.FiresOn, o.Delivery) {
					t.Errorf("%s over %s: delivery %d is %q and fires=%v", det, f.TableName, o.Delivery, o.Reason,
						slices.Contains(v.FiresOn, o.Delivery))
				}
			}
		}
	}
	if got, want := len(e.Rows()), (len(Detectors)-1)*e.Install.InputRegistry; got != want {
		t.Errorf("Rows expands to %d, want %d", got, want)
	}
}

func ordinalsOf(os []OrdinalWhy) []int {
	out := make([]int, len(os))
	for i, o := range os {
		out[i] = o.Delivery
	}
	return out
}

// Each detector fires on its case and on neither healthy
// control. The controls are judged wherever the detector can judge them, so
// "neither" is a verdict rather than an absence.
func TestEveryDetectorFiresSomewhereAndOnNoControl(t *testing.T) {
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	for _, det := range Detectors {
		fired := false
		for _, ef := range e.Feeds {
			v, _ := e.Of(det, ef.TableName)
			fires := len(v.FiresOn) > 0 || v.AtClock == Fires
			fired = fired || fires
			if ef.Role == "control" && fires {
				t.Errorf("%s fires on the control %s", det, ef.TableName)
			}
		}
		if !fired {
			t.Errorf("%s fires on no feed", det)
		}
	}
}

func TestTheProfilesAreTheProfiledFeeds(t *testing.T) {
	profiles, err := Profiles()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range Feeds() {
		raw, ok := profiles[f.TableName]
		if ok != f.Profiled {
			t.Errorf("%s: profiled=%v, has a profile=%v", f.TableName, f.Profiled, ok)
		}
		if !ok {
			continue
		}
		n++
		var p struct {
			ClaimSet struct {
				Table string `json:"table"`
			} `json:"claim_set"`
			DeclaredBy string `json:"declared_by"`
			DeclaredAt string `json:"declared_at"`
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("%s: %v", f.TableName, err)
		}
		if p.ClaimSet.Table != f.ClaimTable {
			t.Errorf("%s: the profile names claim table %q, the fixture %q", f.TableName, p.ClaimSet.Table, f.ClaimTable)
		}
		if p.DeclaredBy == "" || p.DeclaredAt == "" {
			t.Errorf("%s: the profile has no author or date", f.TableName)
		}
	}
	if n != len(profiles) {
		t.Errorf("%d profiles, %d of them for a fixture feed", len(profiles), n)
	}
}

// The late-period feed's last delivery is registered last, under a week about
// three months before the latest the feed has delivered, and carries dates of
// service as recent as its predecessor's.
func TestTheLateDeliveryIsLastAndOld(t *testing.T) {
	f, _ := FeedByTable(LatePeriodIncremental)
	last, prev := f.Deliveries[len(f.Deliveries)-1], f.Deliveries[len(f.Deliveries)-2]
	_, latest, _ := Period(prev.Period)
	_, late, _ := Period(last.Period)
	if gap := latest - late; gap < 12 || gap > 15 {
		t.Errorf("the late delivery is %d weeks before the latest, want about three months", gap)
	}
	newest := func(d Delivery) time.Time {
		var n time.Time
		for _, c := range d.Claims {
			if c.ServiceDate.After(n) {
				n = c.ServiceDate
			}
		}
		return n
	}
	if !newest(last).After(newest(prev)) {
		t.Errorf("the late delivery's newest service date %s is not after its predecessor's %s",
			newest(last).Format(time.DateOnly), newest(prev).Format(time.DateOnly))
	}
}

// The re-processing section names the re-processing feeds, with a verdict per
// detector and feed, and the feeds have the shapes their comments give.
func TestTheReprocessingExpectationIsComplete(t *testing.T) {
	e, err := LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	feeds := ReprocessingFeeds()
	if len(e.Reprocessing.Feeds) != len(feeds) {
		t.Fatalf("the re-processing section names %d feeds, the fixture has %d", len(e.Reprocessing.Feeds), len(feeds))
	}
	for i, f := range feeds {
		ef := e.Reprocessing.Feeds[i]
		if ef.TableName != f.TableName || ef.Client != f.Client || ef.Cumulative != f.Cumulative ||
			ef.Deliveries != len(f.Deliveries) || ef.Reregistrations != len(f.Reregistrations) ||
			ef.ReprocessedFiles != f.ReprocessedFiles() {
			t.Errorf("feed %d: expected.json says %+v, the fixture %s with %d deliveries and %d re-registrations of %d files",
				i, ef, f.TableName, len(f.Deliveries), len(f.Reregistrations), f.ReprocessedFiles())
		}
		for _, det := range Detectors {
			if _, ok := e.Of(det, f.TableName); !ok {
				t.Errorf("no verdict for %s over %s", det, f.TableName)
			}
		}
	}
	if want := len(Detectors) * len(feeds); len(e.Reprocessing.Verdicts) != want {
		t.Errorf("%d re-processing verdicts, want %d", len(e.Reprocessing.Verdicts), want)
	}
	if e.Rules["reprocessing"] == "" {
		t.Error("no rule written for re-processing")
	}
	if got, want := len(e.ReprocessingRows()), (len(Detectors)-1)*(e.Reprocessing.Install.InputRegistry-
		e.Reprocessing.Install.Reregistrations); got != want {
		t.Errorf("ReprocessingRows expands to %d, want one per detector and delivery, %d", got, want)
	}
	// Sessions are unique across all the fixture's registrations.
	seen := map[string]bool{}
	for _, f := range AllFeeds() {
		for _, r := range f.Registrations() {
			if seen[r.SessionID] {
				t.Errorf("session %s is registered twice", r.SessionID)
			}
			seen[r.SessionID] = true
		}
	}

	remap, _ := FeedByTable(RemappedIncremental)
	burst := remap.Reregistrations
	if span := burst[len(burst)-1].Registered.Sub(burst[0].Registered); len(burst) != 12 || span >= time.Second {
		t.Errorf("the re-map is %d re-registrations over %s, want every earlier file within one second", len(burst), span)
	}
	for _, r := range burst {
		if r.Of == 13 || r.Registered.After(remap.Deliveries[12].Registered) || !r.Registered.After(remap.Deliveries[11].Registered) {
			t.Errorf("re-registration of delivery %d at %s is not between the twelfth and thirteenth deliveries", r.Of, r.Registered)
		}
	}
	if burst[0].Of != 12 {
		t.Errorf("the re-map's first re-registration is of delivery %d, want the twelfth, just before it", burst[0].Of)
	}

	back, _ := FeedByTable(BackfilledIncremental)
	times := map[int]int{}
	for _, r := range back.Registrations() {
		times[r.Of]++
	}
	for _, ord := range []int{8, 9, 10} {
		d := back.Deliveries[ord-1]
		if times[ord] < 2 || times[ord] > 3 || !d.Period.Before(back.Deliveries[6].Period) {
			t.Errorf("backfill delivery %d is registered %d times under %s, want two or three under a week before %s",
				ord, times[ord], d.Period.Format(time.DateOnly), back.Deliveries[6].Period.Format(time.DateOnly))
		}
	}
	regs := back.Registrations()
	first, last := regs[7], regs[7+len(back.Reregistrations)+2]
	if last.Registered.Sub(first.Registered) > 15*time.Second {
		t.Errorf("the backfill spans %s, want seconds", last.Registered.Sub(first.Registered))
	}
}
