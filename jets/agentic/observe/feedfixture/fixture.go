package feedfixture

import (
	"fmt"
	"strconv"
	"time"
)

// Clock is the detection time the expected result is written for: a daily run
// at 05:00 UTC. A feed detector under test takes it as its "now".
var Clock = time.Date(2026, time.June, 1, 5, 0, 0, 0, time.UTC)

// SyncTime is file_key_staging.last_update on every row: the last sync of the
// file keys, an hour before Clock. A detector that reads it as an arrival sees
// every delivery arrive at once.
var SyncTime = Clock.Add(-time.Hour)

// The canaries. Every member value contains MemberCanary and every business
// key value contains KeyCanary; neither appears in any table, column, feed or
// profile name, so finding either in a written row or a prompt means a
// member-level value leaked.
const (
	MemberCanary = "zq7canarymbr"
	KeyCanary    = "zq7canarykey"
)

// Canaries returns the two canary tokens.
func Canaries() []string { return []string{MemberCanary, KeyCanary} }

// DomainKey is the object type every delivery is registered under, whatever
// its feed: RegisterFileKeys writes the source configuration's domain key into
// input_registry.object_type, not the feed's object type.
const DomainKey = "Member"

// The claim-set columns, the same in every claim table. Service date and member
// are what the profiles declare; line_number is part of the business key of one
// feed only (ShrinkingCumulative); load_batch is the non-business column that
// differs between deliveries carrying the same claims.
const (
	ColClaimID     = "fx:claim_id"
	ColLineNumber  = "fx:line_number"
	ColServiceDate = "fx:service_date"
	ColMember      = "fx:member_id"
	ColPaidAmount  = "fx:paid_amount"
	ColLoadBatch   = "fx:load_batch"
)

// The feeds, by table_name, which is how a delivery is joined to its feed.
const (
	// A cumulative feed whose weekly deliveries repeat the same key set, never
	// byte-identical, with the newest service date not advancing.
	StaleCumulative = "stale_cumulative"
	// A cumulative feed that keeps advancing: the healthy cumulative control.
	AdvancingCumulative = "advancing_cumulative"
	// An incremental weekly feed that stops while its sibling keeps arriving.
	StoppedIncremental = "stopped_incremental"
	// The sibling that keeps arriving: the healthy incremental control.
	SiblingIncremental = "sibling_incremental"
	// An incremental feed whose paid amount is null on every row from one
	// delivery on.
	IncompleteRequiredField = "incomplete_required_field"
	// An incremental monthly feed with seven deliveries in one month, each
	// registered under its own current period, six of them carrying dates of
	// service that reach years back; a small key overlap between deliveries,
	// and rows exceeding distinct keys within a delivery.
	WideSpanIncremental = "wide_span_incremental"
	// An incremental weekly feed with an ordinary history, then one late
	// delivery registered under a period about three months older than the
	// feed's latest. Its dates of service are ordinary: only its registration
	// is out of order.
	LatePeriodIncremental = "late_period_incremental"
	// A cumulative feed one of whose deliveries does not contain its
	// predecessor. Its business key has two columns.
	ShrinkingCumulative = "shrinking_cumulative"
	// A feed whose deliveries restate its history, declared incremental.
	CumulativeDeclaredIncremental = "cumulative_declared_incremental"
	// A profiled feed whose claim-set table holds none of its deliveries, as
	// when the pipeline skips the database load.
	UnloadedClaimSet = "unloaded_claim_set"
	// A feed with no profile, which stopped long before Clock.
	Unprofiled = "unprofiled"
)

// Feed is one feed: its source_config row, its claim-set table and its
// deliveries in registration order.
type Feed struct {
	Client     string
	Org        string
	ObjectType string // source_config.object_type, and file_key_staging's
	TableName  string // source_config.table_name, unique
	Cumulative bool   // source_config.is_cumulative
	Profiled   bool   // has a profile in testdata/feed_profiles.json
	// ClaimTable is the claim-set table in the public schema, or "" when the
	// feed has none. It is created even when it holds no rows.
	ClaimTable string
	Deliveries []Delivery
	// Reregistrations are later registrations of files already delivered:
	// re-processing, not deliveries. Only the re-processing feeds
	// (ReprocessingFeeds) have any.
	Reregistrations []Reregistration
}

// Delivery is one file at its first registration: an input_registry row, its
// file_key_staging row and, for a loaded feed, its rows in the claim-set
// table.
type Delivery struct {
	Ordinal    int       // 1-based, in registration order within the feed
	Registered time.Time // the arrival; SessionID is its UnixMilli
	SessionID  string
	Period     time.Time // the date the file key names, its source_period
	FileKey    string
	FileSize   int64
	Claims     []Claim
}

// Reregistration is a later registration of a delivered file, as JetStore
// records a re-processing: an input_registry row with the file's file key and
// period and a session of its own, and, for a loaded feed, the file's claims
// loaded again under that session with a load batch of their own. It adds no
// file_key_staging row, which is unique on file_key.
type Reregistration struct {
	Of         int // the ordinal of the delivery whose file it registers again
	Registered time.Time
	SessionID  string
	Claims     []Claim
}

// Claim is one row of a claim-set table.
type Claim struct {
	ClaimID     string
	LineNumber  int
	ServiceDate time.Time // the zero time is a null date of service
	MemberID    string
	PaidAmount  *float64 // nil is a null paid amount
	LoadBatch   string
}

// bytesPerRow is the nominal size of one row in a delivered file.
const bytesPerRow = 470

// membersPerFeed bounds the distinct members of a feed, so every delivery of
// 120 rows or more carries at least 40 distinct members.
const membersPerFeed = 60

// Feeds returns the fixture's feeds. It is deterministic: two calls return
// equal values, and nothing depends on the clock or on a random source.
func Feeds() []Feed {
	specs := []func(int) Feed{
		staleCumulative, advancingCumulative,
		stoppedIncremental, siblingIncremental, incompleteRequiredField,
		wideSpanIncremental, latePeriodIncremental, shrinkingCumulative,
		cumulativeDeclaredIncremental, unloadedClaimSet, unprofiled,
	}
	feeds := make([]Feed, len(specs))
	for i, spec := range specs {
		feeds[i] = spec(i)
	}
	return feeds
}

// AllFeeds returns the base feeds, then the re-processing feeds, then the
// not-judged feeds.
func AllFeeds() []Feed { return append(append(Feeds(), ReprocessingFeeds()...), NotJudgedFeeds()...) }

// FeedByTable returns the feed of a table_name, among all the fixture's feeds.
func FeedByTable(tableName string) (Feed, bool) {
	for _, f := range AllFeeds() {
		if f.TableName == tableName {
			return f, true
		}
	}
	return Feed{}, false
}

// SessionOf returns the session id of a feed's delivery by ordinal.
func SessionOf(tableName string, ordinal int) (string, bool) {
	f, ok := FeedByTable(tableName)
	if !ok || ordinal < 1 || ordinal > len(f.Deliveries) {
		return "", false
	}
	return f.Deliveries[ordinal-1].SessionID, true
}

// ---------------------------------------------------------------------------
// Generation

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func days(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }

// mondays returns n consecutive weekly dates from a Monday.
func mondays(from time.Time, n int) []time.Time {
	out := make([]time.Time, n)
	for i := range out {
		out[i] = from.AddDate(0, 0, 7*i)
	}
	return out
}

// gen carries one feed's claim numbering.
type gen struct {
	code string // short, used only inside generated values
	seq  int
}

// newClaims returns n claims of line 1 with service dates spread from lo to hi
// inclusive, so the oldest is lo and the newest hi exactly.
func (g *gen) newClaims(n int, lo, hi time.Time) []Claim {
	out := make([]Claim, n)
	spanDays := int(hi.Sub(lo) / days(1))
	for i := range out {
		g.seq++
		offset := 0
		if n > 1 {
			offset = i * spanDays / (n - 1)
		}
		paid := float64((g.seq*37)%500) + 10.25
		out[i] = Claim{
			ClaimID:     fmt.Sprintf("%s-%s-%06d", KeyCanary, g.code, g.seq),
			LineNumber:  1,
			ServiceDate: lo.AddDate(0, 0, offset),
			MemberID:    fmt.Sprintf("%s-%s-%03d", MemberCanary, g.code, (g.seq*7)%membersPerFeed),
			PaidAmount:  &paid,
		}
	}
	return out
}

// delivery builds a Delivery: the registration at 06:00 UTC plus the feed's
// index in minutes, so no two feeds share a session id, and a file size of
// rows times bytesPerRow plus a small jitter, so no two deliveries are the same
// size. Claims are copied, and stamped with the delivery's load batch.
func delivery(feedIndex, ordinal int, client, objectType, tableName string,
	regDate, period time.Time, rows int, claims []Claim) Delivery {
	return deliveryAt(feedIndex, ordinal, client, objectType, tableName,
		regDate.Add(6*time.Hour+time.Duration(feedIndex)*time.Minute), period, rows, claims)
}

// deliveryAt builds a Delivery registered at an exact instant.
func deliveryAt(feedIndex, ordinal int, client, objectType, tableName string,
	reg, period time.Time, rows int, claims []Claim) Delivery {
	session := strconv.FormatInt(reg.UnixMilli(), 10)
	jitter := int64((ordinal*137+feedIndex*53)%901) - 450
	cs := make([]Claim, len(claims))
	for i, c := range claims {
		c.LoadBatch = fmt.Sprintf("batch-%s", session)
		cs[i] = c
	}
	return Delivery{
		Ordinal:    ordinal,
		Registered: reg,
		SessionID:  session,
		Period:     period,
		FileKey: fmt.Sprintf("fixture/input/client=%s/object_type=%s/year=%d/month=%d/day=%d/%s_%02d.csv",
			client, objectType, period.Year(), int(period.Month()), period.Day(), tableName, ordinal),
		FileSize: int64(rows)*bytesPerRow + jitter,
		Claims:   cs,
	}
}

// claimTable names a feed's claim-set table the way a pipeline names a domain
// table: a class name with a namespace prefix, so it needs quoting.
func claimTable(class string) string { return "fx:" + class }

func staleCumulative(idx int) Feed {
	f := Feed{Client: "client_a", ObjectType: "PharmacyClaim", TableName: StaleCumulative,
		Cumulative: true, Profiled: true, ClaimTable: claimTable("StaleCumulativeClaim")}
	g := &gen{code: "sc"}
	regs := mondays(day(2026, time.April, 6), 8)
	base := g.newClaims(200, day(2025, time.April, 1), day(2026, time.April, 3))
	// The second delivery adds six claims, and from then on the key set and
	// every business column are frozen; only load_batch and the size move.
	frozen := append(append([]Claim{}, base...), g.newClaims(6, day(2026, time.April, 6), day(2026, time.April, 10))...)
	for i, r := range regs {
		claims := frozen
		if i == 0 {
			claims = base
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	}
	return f
}

func advancingCumulative(idx int) Feed {
	f := Feed{Client: "client_a", ObjectType: "MedicalClaim", TableName: AdvancingCumulative,
		Cumulative: true, Profiled: true, ClaimTable: claimTable("AdvancingCumulativeClaim")}
	g := &gen{code: "ac"}
	regs := mondays(day(2026, time.April, 6), 8)
	held := g.newClaims(300, day(2024, time.July, 1), regs[0].Add(-days(3)))
	for i, r := range regs {
		if i > 0 {
			held = append(held, g.newClaims(15, regs[i-1].Add(-days(2)), r.Add(-days(3)))...)
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(held), held))
	}
	return f
}

func stoppedIncremental(idx int) Feed {
	f := Feed{Client: "client_b", ObjectType: "MedicalClaim", TableName: StoppedIncremental,
		Profiled: true, ClaimTable: claimTable("StoppedIncrementalClaim")}
	g := &gen{code: "si"}
	for i, r := range mondays(day(2026, time.March, 2), 7) {
		claims := g.newClaims(120, r.Add(-days(28)), r.Add(-days(3)))
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	}
	return f
}

func siblingIncremental(idx int) Feed {
	f := Feed{Client: "client_b", ObjectType: "DentalClaim", TableName: SiblingIncremental,
		Profiled: true, ClaimTable: claimTable("SiblingIncrementalClaim")}
	g := &gen{code: "sb"}
	var prevNew []Claim
	for i, r := range mondays(day(2026, time.March, 2), 13) {
		var claims []Claim
		if i == 0 {
			prevNew = g.newClaims(250, r.Add(-days(28)), r.Add(-days(3)))
			claims = prevNew
		} else {
			// Three claims of the previous delivery are delivered again.
			again := append([]Claim{}, prevNew[:3]...)
			prevNew = g.newClaims(247, r.Add(-days(28)), r.Add(-days(3)))
			claims = append(again, prevNew...)
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	}
	return f
}

// incompleteFrom is the ordinal from which IncompleteRequiredField carries no
// paid amount on any row.
const incompleteFrom = 7

func incompleteRequiredField(idx int) Feed {
	f := Feed{Client: "client_b", ObjectType: "PharmacyClaim", TableName: IncompleteRequiredField,
		Profiled: true, ClaimTable: claimTable("IncompleteRequiredFieldClaim")}
	g := &gen{code: "ir"}
	for i, r := range mondays(day(2026, time.March, 2), 13) {
		claims := g.newClaims(200, r.Add(-days(21)), r.Add(-days(2)))
		for j := range claims {
			// Before the change, one row in a hundred has no paid amount; from
			// it on, none has one.
			if i+1 >= incompleteFrom || j%100 == 0 {
				claims[j].PaidAmount = nil
			}
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	}
	return f
}

func wideSpanIncremental(idx int) Feed {
	f := Feed{Client: "client_c", ObjectType: "PharmacyClaim", TableName: WideSpanIncremental,
		Profiled: true, ClaimTable: claimTable("WideSpanIncrementalClaim")}
	g := &gen{code: "ws"}
	// Every delivery is registered under its own current period. reach, when
	// set, is how far back its dates of service go: it covers the year before
	// reach and the month after, where an ordinary delivery covers the seven
	// weeks before its arrival.
	type reg struct{ at, reach time.Time }
	regs := []reg{
		// Four monthly deliveries.
		{day(2026, time.January, 5), time.Time{}},
		{day(2026, time.February, 2), time.Time{}},
		{day(2026, time.March, 2), time.Time{}},
		{day(2026, time.April, 6), time.Time{}},
		// Seven in one month: three reaching years back, an ordinary one,
		// three more reaching back.
		{day(2026, time.May, 4), day(2021, time.June, 1)},
		{day(2026, time.May, 7), day(2022, time.March, 1)},
		{day(2026, time.May, 12), day(2023, time.January, 1)},
		{day(2026, time.May, 14), time.Time{}},
		{day(2026, time.May, 19), day(2024, time.February, 1)},
		{day(2026, time.May, 21), day(2025, time.January, 1)},
		{day(2026, time.May, 26), day(2025, time.September, 1)},
	}
	const keysPerDelivery, shared, doubled = 300, 10, 33
	var prevNew []Claim
	for i, r := range regs {
		lo, hi := r.at.Add(-days(50)), r.at.Add(-days(2))
		if !r.reach.IsZero() {
			lo, hi = r.reach.Add(-days(365)), r.reach.Add(days(30))
		}
		// Ten claims of the previous delivery are delivered again when their
		// service dates fall in this delivery's range. They are taken from its
		// new claims, so no claim is delivered more than twice.
		var again []Claim
		if n := len(prevNew); n >= shared {
			tail := prevNew[n-shared:]
			if !tail[0].ServiceDate.Before(lo) && !tail[shared-1].ServiceDate.After(hi) {
				again = append(again, tail...)
			}
		}
		prevNew = g.newClaims(keysPerDelivery-len(again), lo, hi)
		keys := append(again, prevNew...)
		// Thirty-three keys carry a second row, so rows exceed distinct keys by
		// 11%: the business key is the claim, and the rows are its lines.
		claims := append([]Claim{}, keys...)
		for j := 0; j < doubled; j++ {
			c := keys[j*9]
			c.LineNumber = 2
			claims = append(claims, c)
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r.at, r.at, len(claims), claims))
	}
	return f
}

// The late delivery of LatePeriodIncremental: registered after the weekly
// history, under the period of a week about three months before the latest.
var (
	lateRegistered = day(2026, time.May, 27)
	latePeriod     = day(2026, time.February, 9)
)

func latePeriodIncremental(idx int) Feed {
	f := Feed{Client: "client_c", ObjectType: "DentalClaim", TableName: LatePeriodIncremental,
		Profiled: true, ClaimTable: claimTable("LatePeriodIncrementalClaim")}
	g := &gen{code: "lp"}
	for i, r := range mondays(day(2026, time.March, 2), 13) {
		claims := g.newClaims(150, r.Add(-days(28)), r.Add(-days(3)))
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	}
	// The late delivery is the last registered, so a detector reading the last
	// registration's period as the latest delivered judges the feed overdue.
	// Its dates of service are as recent as any other delivery's, so no
	// content detector fires on it, whether a lag is read from its arrival or
	// from its period.
	claims := g.newClaims(150, lateRegistered.Add(-days(28)), lateRegistered.Add(-days(3)))
	f.Deliveries = append(f.Deliveries, delivery(idx, 14, f.Client, f.ObjectType, f.TableName,
		lateRegistered, latePeriod, len(claims), claims))
	return f
}

// shrinkingAt is the ordinal of ShrinkingCumulative's delivery that does not
// contain its predecessor.
const shrinkingAt = 5

func shrinkingCumulative(idx int) Feed {
	f := Feed{Client: "client_c", ObjectType: "MedicalClaim", TableName: ShrinkingCumulative,
		Cumulative: true, Profiled: true, ClaimTable: claimTable("ShrinkingCumulativeClaim")}
	g := &gen{code: "sh"}
	regs := mondays(day(2026, time.April, 6), 8)
	// Every claim has two lines, and the business key is (claim, line).
	twoLines := func(cs []Claim) []Claim {
		out := make([]Claim, 0, 2*len(cs))
		for _, c := range cs {
			out = append(out, c)
			c.LineNumber = 2
			out = append(out, c)
		}
		return out
	}
	held := twoLines(g.newClaims(75, day(2025, time.January, 1), regs[0].Add(-days(3))))
	for i, r := range regs {
		if i > 0 {
			held = append(held, twoLines(g.newClaims(3, regs[i-1].Add(-days(2)), r.Add(-days(3))))...)
		}
		if i+1 == shrinkingAt {
			// Line 2 of the first ten claims is dropped, and stays dropped.
			// Each of those claims keeps its line 1, so the claim ids alone are
			// still contained and only the declared two-column key sees it.
			kept := held[:0:0]
			for j, c := range held {
				if c.LineNumber == 2 && j < 20 {
					continue
				}
				kept = append(kept, c)
			}
			held = kept
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(held), held))
	}
	return f
}

func cumulativeDeclaredIncremental(idx int) Feed {
	f := Feed{Client: "client_d", ObjectType: "PharmacyClaim", TableName: CumulativeDeclaredIncremental,
		Profiled: true, ClaimTable: claimTable("CumulativeDeclaredIncrementalClaim")}
	g := &gen{code: "cd"}
	regs := mondays(day(2026, time.April, 6), 8)
	held := g.newClaims(400, day(2024, time.June, 1), regs[0].Add(-days(4)))
	for i, r := range regs {
		if i > 0 {
			held = append(held, g.newClaims(12, regs[i-1].Add(-days(3)), r.Add(-days(4)))...)
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(held), held))
	}
	return f
}

func unloadedClaimSet(idx int) Feed {
	f := Feed{Client: "client_d", ObjectType: "MedicalClaim", TableName: UnloadedClaimSet,
		Profiled: true, ClaimTable: claimTable("UnloadedClaim")}
	for i, r := range mondays(day(2026, time.April, 6), 8) {
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, 200, nil))
	}
	return f
}

func unprofiled(idx int) Feed {
	f := Feed{Client: "client_d", ObjectType: "VisionClaim", TableName: Unprofiled}
	for i, r := range mondays(day(2026, time.January, 5), 6) {
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, 100, nil))
	}
	return f
}
