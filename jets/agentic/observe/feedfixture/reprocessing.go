package feedfixture

import (
	"fmt"
	"strconv"
	"time"
)

// The re-processing feeds. JetStore records the re-processing of a file it
// already holds as a new registration of the same file key under the file's
// own period, so a reading that takes every registration for a delivery judges
// re-processing as deliveries. These two feeds carry it in the two shapes a
// client's database shows: a re-map, which registers every earlier file of a
// feed again in one burst, and a backfill, which registers each of a run of
// past-period files two or three times within seconds.
//
// They are installed by InstallReprocessing, on top of Install, and their
// expected verdicts are expected.json's "reprocessing" section, so the base
// fixture's totals, which other packages assert, do not move.
const (
	// An incremental weekly feed whose twelve files are all registered again
	// in one second, newest first, before its thirteenth delivery.
	RemappedIncremental = "remapped_incremental"
	// An incremental weekly feed that misses three weeks, delivers the next
	// on time, and then delivers the three missed weeks' files in a burst,
	// each registered two or three times within seconds.
	BackfilledIncremental = "backfilled_incremental"
)

// ReprocessingClient is the client both re-processing feeds belong to, which
// no base feed uses, so InstallReprocessing replaces only its own rows.
const ReprocessingClient = "client_e"

// ReprocessingFeeds returns the two re-processing feeds. Their feed indices
// follow the base feeds', so no session id is shared with a base feed.
func ReprocessingFeeds() []Feed {
	n := len(Feeds())
	return []Feed{remappedIncremental(n), backfilledIncremental(n + 1)}
}

// The re-map of RemappedIncremental: twelve registrations, one every 60 ms.
var remapAt = time.Date(2026, time.May, 21, 20, 53, 0, 0, time.UTC)

// reregister registers a delivery's file again at an instant: the same file
// key and period, a session of its own, and the file's claims loaded again
// with that session's load batch.
func reregister(d Delivery, at time.Time) Reregistration {
	session := strconv.FormatInt(at.UnixMilli(), 10)
	cs := make([]Claim, len(d.Claims))
	for i, c := range d.Claims {
		c.LoadBatch = fmt.Sprintf("batch-%s", session)
		cs[i] = c
	}
	return Reregistration{Of: d.Ordinal, Registered: at, SessionID: session, Claims: cs}
}

func remappedIncremental(idx int) Feed {
	f := Feed{Client: ReprocessingClient, ObjectType: "PharmacyClaim", TableName: RemappedIncremental,
		Profiled: true, ClaimTable: claimTable("RemappedIncrementalClaim")}
	g := &gen{code: "rm"}
	weeks := mondays(day(2026, time.March, 2), 13)
	for i, r := range weeks[:12] {
		claims := g.newClaims(120, r.Add(-days(28)), r.Add(-days(3)))
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	}
	// The re-map, newest file first: the first re-registration is of the
	// delivery just before it, which a registration reading would compare
	// with its own original.
	for i := range 12 {
		f.Reregistrations = append(f.Reregistrations,
			reregister(f.Deliveries[11-i], remapAt.Add(time.Duration(i)*60*time.Millisecond)))
	}
	r := weeks[12]
	claims := g.newClaims(120, r.Add(-days(28)), r.Add(-days(3)))
	f.Deliveries = append(f.Deliveries, delivery(idx, 13, f.Client, f.ObjectType, f.TableName, r, r, len(claims), claims))
	return f
}

// The backfill of BackfilledIncremental: the three missed weeks' files, from
// this instant, two seconds apart.
var backfillAt = time.Date(2026, time.May, 6, 14, 0, 0, 0, time.UTC)

// backfillTimes is how many times each missed week's file is registered.
var backfillTimes = []int{2, 3, 2}

func backfilledIncremental(idx int) Feed {
	f := Feed{Client: ReprocessingClient, ObjectType: "MedicalClaim", TableName: BackfilledIncremental,
		Profiled: true, ClaimTable: claimTable("BackfilledIncrementalClaim")}
	g := &gen{code: "bf"}
	weeks := mondays(day(2026, time.March, 2), 13)
	claimsOf := map[int][]Claim{}
	for w, r := range weeks {
		claimsOf[w] = g.newClaims(120, r.Add(-days(28)), r.Add(-days(3)))
	}
	add := func(w int) {
		r := weeks[w]
		f.Deliveries = append(f.Deliveries, delivery(idx, len(f.Deliveries)+1, f.Client, f.ObjectType, f.TableName,
			r, r, len(claimsOf[w]), claimsOf[w]))
	}
	// Six weeks on time, three missed, the tenth on time.
	for w := range 6 {
		add(w)
	}
	add(9)
	// The backfill: each missed week's file at its first registration, then
	// registered again, every registration two seconds after the last.
	at := backfillAt
	for k, w := range []int{6, 7, 8} {
		r := weeks[w]
		d := deliveryAt(idx, len(f.Deliveries)+1, f.Client, f.ObjectType, f.TableName, at, r, len(claimsOf[w]), claimsOf[w])
		f.Deliveries = append(f.Deliveries, d)
		for range backfillTimes[k] - 1 {
			at = at.Add(2 * time.Second)
			f.Reregistrations = append(f.Reregistrations, reregister(d, at))
		}
		at = at.Add(2 * time.Second)
	}
	// Weekly again.
	for w := 10; w < 13; w++ {
		add(w)
	}
	return f
}

// Registration is one input_registry row of a feed as the fixture installs
// it: a delivery's first registration or a re-registration.
type Registration struct {
	Registered time.Time
	SessionID  string
	Of         int  // the delivery's ordinal
	Again      bool // a re-registration
}

// Registrations returns every registration of a feed in registration order.
func (f Feed) Registrations() []Registration {
	var out []Registration
	for _, d := range f.Deliveries {
		out = append(out, Registration{Registered: d.Registered, SessionID: d.SessionID, Of: d.Ordinal})
	}
	for _, r := range f.Reregistrations {
		out = append(out, Registration{Registered: r.Registered, SessionID: r.SessionID, Of: r.Of, Again: true})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Registered.Before(out[j-1].Registered); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ReprocessedFiles is how many distinct delivered files a feed registers again.
func (f Feed) ReprocessedFiles() int {
	seen := map[int]bool{}
	for _, r := range f.Reregistrations {
		seen[r.Of] = true
	}
	return len(seen)
}
