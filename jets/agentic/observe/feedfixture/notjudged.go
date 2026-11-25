package feedfixture

import "time"

// The not-judged feeds. Each carries one way a content check meets a delivery
// it cannot evaluate, other than a feed with no profile or a claim-set table
// holding none of its deliveries, which the base feeds carry: a delivery whose
// claims were not loaded, so its successor has nothing to be compared with; a
// delivery none of whose rows carries a date of service; a profile naming a
// column the claim-set table does not have, once for a column every check reads
// and once for a required field only field_completeness reads. The last also
// declares no size range, so delivery_size judges it against its trailing
// band.
//
// They are installed by InstallNotJudged, alone or beside Install, and their
// expected verdicts are expected.json's "not_judged" section, so the base
// fixture's totals, which other packages assert, do not move.
const (
	// An incremental weekly feed whose fourth delivery's claims were not
	// loaded, as when the pipeline skips the database load for one file. The
	// file was delivered at its usual size, and the deliveries before and
	// after it were loaded.
	PartlyLoadedIncremental = "partly_loaded_incremental"
	// An incremental weekly feed whose fourth delivery carries no date of
	// service on any row, as when a mapping leaves the column empty.
	UndatedDeliveryIncremental = "undated_delivery_incremental"
	// An incremental weekly feed whose profile names a member column the
	// claim-set table does not have, as when the profile was written against
	// a property that was later renamed.
	AbsentMemberColumn = "absent_member_column"
	// An incremental weekly feed whose profile names two required fields, one
	// of which the claim-set table does not have, and declares no size range.
	// Its last delivery is a small file.
	AbsentRequiredField = "absent_required_field"
)

// NotJudgedClient is the client the not-judged feeds belong to, which no other
// feed uses, so InstallNotJudged replaces only its own rows.
const NotJudgedClient = "client_f"

// The deliveries that carry a not-judged case: the fourth of
// PartlyLoadedIncremental holds no claim rows, and the fourth of
// UndatedDeliveryIncremental no date of service.
const (
	unloadedAt = 4
	undatedAt  = 4
)

// The columns the not-judged profiles name that the claim-set tables do not
// have.
const (
	ColAbsentMember   = "fx:member_key"
	ColAbsentRequired = "fx:allowed_amount"
)

// smallFileRows is the row count of AbsentRequiredField's last delivery, a
// file about a third of the feed's usual size.
const smallFileRows = 40

// NotJudgedFeeds returns the four not-judged feeds. Their feed indices follow
// the re-processing feeds', so no session id is shared with another feed.
func NotJudgedFeeds() []Feed {
	n := len(Feeds()) + len(ReprocessingFeeds())
	return []Feed{partlyLoadedIncremental(n), undatedDeliveryIncremental(n + 1), absentMemberColumn(n + 2),
		absentRequiredField(n + 3)}
}

// notJudgedWeeks are the weeks every not-judged feed delivers on, the last a
// week before the clock, so none is overdue.
func notJudgedWeeks() []time.Time { return mondays(day(2026, time.April, 13), 7) }

// weekly builds an ordinary incremental weekly feed of 120 claims a delivery,
// dated from four weeks to three days before the arrival; rows, when it
// returns a count other than 120 for an ordinal, sets that delivery's size and
// claim count, and claims may change a delivery's claims after they are made.
func weekly(idx int, f Feed, code string, rows func(ordinal int) int, claims func(ordinal int, cs []Claim) []Claim) Feed {
	g := &gen{code: code}
	for i, r := range notJudgedWeeks() {
		n := 120
		if rows != nil {
			n = rows(i + 1)
		}
		cs := g.newClaims(n, r.Add(-days(28)), r.Add(-days(3)))
		if claims != nil {
			cs = claims(i+1, cs)
		}
		f.Deliveries = append(f.Deliveries, delivery(idx, i+1, f.Client, f.ObjectType, f.TableName, r, r, n, cs))
	}
	return f
}

func partlyLoadedIncremental(idx int) Feed {
	f := Feed{Client: NotJudgedClient, ObjectType: "MedicalClaim", TableName: PartlyLoadedIncremental,
		Profiled: true, ClaimTable: claimTable("PartlyLoadedIncrementalClaim")}
	return weekly(idx, f, "pl", nil, func(ordinal int, cs []Claim) []Claim {
		if ordinal == unloadedAt {
			return nil
		}
		return cs
	})
}

func undatedDeliveryIncremental(idx int) Feed {
	f := Feed{Client: NotJudgedClient, ObjectType: "PharmacyClaim", TableName: UndatedDeliveryIncremental,
		Profiled: true, ClaimTable: claimTable("UndatedDeliveryIncrementalClaim")}
	return weekly(idx, f, "ud", nil, func(ordinal int, cs []Claim) []Claim {
		if ordinal == undatedAt {
			for i := range cs {
				cs[i].ServiceDate = time.Time{}
			}
		}
		return cs
	})
}

func absentMemberColumn(idx int) Feed {
	f := Feed{Client: NotJudgedClient, ObjectType: "DentalClaim", TableName: AbsentMemberColumn,
		Profiled: true, ClaimTable: claimTable("AbsentMemberColumnClaim")}
	return weekly(idx, f, "am", nil, nil)
}

func absentRequiredField(idx int) Feed {
	f := Feed{Client: NotJudgedClient, ObjectType: "VisionClaim", TableName: AbsentRequiredField,
		Profiled: true, ClaimTable: claimTable("AbsentRequiredFieldClaim")}
	last := len(notJudgedWeeks())
	return weekly(idx, f, "ar", func(ordinal int) int {
		if ordinal == last {
			return smallFileRows
		}
		return 120
	}, nil)
}
