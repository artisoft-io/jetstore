// Package feedprofile is the declaration a person makes about one feed: how
// often its deliveries are due, how large one is, which table holds its claims
// and which columns identify them, and the thresholds the feed detectors judge
// it against.
//
// A feed is a source_config row, unique on client, org and object type and on
// table name. Its profile is stored beside it, in the nullable jsonb column
// source_config.feed_profile_json, and edited where the feed is configured. A
// feed with no profile is not judged: a detector reports it as not judged, by
// name, rather than judging it with defaults.
//
// Whether a feed is cumulative is not part of the profile. It is
// source_config.is_cumulative, which the stage reads also use, and Feed carries
// it beside the profile so that the two never disagree.
//
// The package imports nothing outside the standard library and jets/wsvalidate.
// It reaches the database through Querier, which *pgxpool.Pool satisfies.
package feedprofile

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// SchemaJSON is the JSON Schema of a profile document. Validate implements it;
// the package's tests hold the two to the same fields.
//
//go:embed feed_profile.schema.json
var SchemaJSON []byte

// The period types a profile may declare, JetStore's own: the units of
// source_period.
const (
	MonthPeriod = "month_period"
	WeekPeriod  = "week_period"
	DayPeriod   = "day_period"
)

// PeriodTypes lists the accepted period types, in the order the schema gives them.
var PeriodTypes = []string{MonthPeriod, WeekPeriod, DayPeriod}

// Profile is one feed's declaration, as stored in source_config.feed_profile_json.
//
// Every field but DeclaredBy and DeclaredAt is optional. A detector whose
// fields are absent does not judge the feed. Optional numbers are pointers so
// that an absent value and an explicit zero stay distinct.
type Profile struct {
	// PeriodType is the unit in which deliveries are due: month_period,
	// week_period or day_period.
	PeriodType string `json:"period_type,omitempty"`
	// PeriodsPerDelivery is how many periods one delivery covers: 2 for every
	// two weeks with week_period, 3 for quarterly with month_period. Absent is 1.
	PeriodsPerDelivery *int `json:"periods_per_delivery,omitempty"`
	// OverdueAfterDays is the grace, in days, after the period in which a
	// delivery was due.
	OverdueAfterDays *int `json:"overdue_after_days,omitempty"`
	// SizeBytes is the expected size of one delivery. Absent means the
	// detector uses the feed's trailing band, and says so.
	SizeBytes *SizeRange `json:"size_bytes,omitempty"`
	// ClaimSet names the domain table holding the feed's claims and the
	// columns every content check reads.
	ClaimSet *ClaimSet `json:"claim_set,omitempty"`
	// MaxServiceDateLagDays is how far the newest date of service may fall
	// behind the delivery.
	MaxServiceDateLagDays *int `json:"max_service_date_lag_days,omitempty"`
	// MaxServiceDateSpanDays is the widest span of service dates one delivery
	// of an incremental feed may hold: a row whose date of service is more
	// than this many days before the delivery's newest is outside the span.
	MaxServiceDateSpanDays *int `json:"max_service_date_span_days,omitempty"`
	// MaxOutOfSpanShare is the largest share, at least 0 and below 1, of a
	// delivery's rows with a date of service that may be outside the span.
	// It is valid only with MaxServiceDateSpanDays. Absent is 0: any row
	// outside the span fires, which is the same as judging newest less oldest
	// against the span. Declaring it lets a few late rows, reversals of old
	// claims say, through without letting a back-dated delivery through.
	MaxOutOfSpanShare *float64 `json:"max_out_of_span_share,omitempty"`
	// MaxRedeliveredShare is the largest share, 0 to 1, of an incremental
	// delivery's distinct keys that may already have been delivered.
	MaxRedeliveredShare *float64 `json:"max_redelivered_share,omitempty"`
	// RequiredFields are columns that must be present in at least a share of
	// a delivery's rows.
	RequiredFields []RequiredField `json:"required_fields,omitempty"`
	// DeclaredBy is who declared the profile.
	DeclaredBy string `json:"declared_by"`
	// DeclaredAt is the date of the declaration, YYYY-MM-DD.
	DeclaredAt string `json:"declared_at"`
}

// SizeRange is an expected size, in bytes, of one delivery. Either bound may
// be absent, not both.
type SizeRange struct {
	Min *int64 `json:"min,omitempty"`
	Max *int64 `json:"max,omitempty"`
}

// ClaimSet is where a feed's claims are, once loaded, and the columns that
// identify a claim, date it and name its member. The key is declared, never
// inferred, and a detector checks it against the data before trusting it.
type ClaimSet struct {
	Table             string   `json:"table"`
	KeyColumns        []string `json:"key_columns"`
	ServiceDateColumn string   `json:"service_date_column"`
	MemberColumn      string   `json:"member_column"`
}

// RequiredField is a column and the smallest share, above 0 and at most 1, of
// a delivery's rows in which it must be present.
type RequiredField struct {
	Column          string   `json:"column"`
	MinPresentShare *float64 `json:"min_present_share"`
}

// DeliveryPeriods is PeriodsPerDelivery, or 1 when it is absent.
func (p Profile) DeliveryPeriods() int {
	if p.PeriodsPerDelivery == nil {
		return 1
	}
	return *p.PeriodsPerDelivery
}

// Feed is a source_config row as the feed detectors read it: its identity, the
// cumulative switch, and its profile.
type Feed struct {
	Client     string
	Org        string
	ObjectType string
	// TableName joins the feed to its deliveries: input_registry.table_name.
	TableName string
	// Cumulative is source_config.is_cumulative: each delivery holds the whole
	// feed. The profile does not declare it.
	Cumulative bool
	// Profile is the feed's declaration. It is the zero Profile on a feed
	// returned as unprofiled.
	Profile Profile
	// Findings say why a stored profile was not used: it did not decode, or
	// Validate refused it. Empty on a profiled feed, and on a feed that has no
	// profile at all.
	Findings []wsvalidate.Finding
}

// IsEmpty reports whether text holds no profile: empty, blank or the JSON null.
// That is what clears source_config.feed_profile_json.
func IsEmpty(text string) bool {
	t := strings.TrimSpace(text)
	return t == "" || t == "null"
}

// Parse decodes one profile document and validates it. A document that does
// not decode is one finding; a document that decodes returns what Validate
// finds. The profile is usable when no finding is an error.
//
// Decoding refuses a field the profile does not have, a value of the wrong
// type, anything after the document, and the JSON null: an empty profile is
// IsEmpty's case, and the caller's to handle.
func Parse(data []byte) (Profile, []wsvalidate.Finding) {
	var p Profile
	if IsEmpty(string(data)) {
		return p, []wsvalidate.Finding{{
			Severity: wsvalidate.Error, Code: "feed_profile.empty",
			Message: "the feed profile is empty",
		}}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Profile{}, []wsvalidate.Finding{decodeFinding(err)}
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Profile{}, []wsvalidate.Finding{{
			Severity: wsvalidate.Error, Code: "feed_profile.decode",
			Message: "the feed profile must be one JSON object, with nothing after it",
		}}
	}
	return p, Validate(p)
}

func decodeFinding(err error) wsvalidate.Finding {
	f := wsvalidate.Finding{Severity: wsvalidate.Error, Code: "feed_profile.decode",
		Message: "the feed profile is not a valid document: " + err.Error()}
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) && te.Field != "" {
		f.Path = "/" + strings.ReplaceAll(te.Field, ".", "/")
	}
	return f
}

// Errors returns the error findings as one error, or nil when there is none.
// Each finding reads as its path and its message, so the error says where.
func Errors(findings []wsvalidate.Finding) error {
	errs := wsvalidate.ErrorsOnly(findings)
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, len(errs))
	for i, f := range errs {
		if f.Path != "" {
			parts[i] = fmt.Sprintf("%s: %s", f.Path, f.Message)
		} else {
			parts[i] = f.Message
		}
	}
	return errors.New("invalid feed profile: " + strings.Join(parts, "; "))
}
