package feedfixture

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"time"
)

//go:embed testdata/expected.json testdata/feed_profiles.json testdata/reprocessing_profiles.json
var testdata embed.FS

// Detectors are the eight feed detectors the expected result covers, in the
// order the plan gives them.
var Detectors = []string{
	"feed_overdue", "delivery_size", "delivery_backdated", "service_date_stale",
	"service_date_span", "field_completeness", "delivery_overlap", "delivery_unchanged",
}

// The verdicts a detector may reach on one delivery, or at the clock.
const (
	Fires     = "fires"
	Silent    = "silent"
	NotJudged = "not_judged"
)

// ProfilesPath is testdata/feed_profiles.json in this checkout: one profile per
// profiled feed, keyed by table_name, the shape a profile file is read in.
func ProfilesPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "feed_profiles.json")
}

// ReprocessingProfilesPath is testdata/reprocessing_profiles.json: the
// re-processing feeds' profiles, kept apart from ProfilesPath so a run over
// the base fixture reads no profile whose feed is not installed.
func ReprocessingProfilesPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "reprocessing_profiles.json")
}

// Profiles returns the declared profiles, keyed by table_name, as the JSON
// documents they are. The fixture does not decode them: what a profile means is
// the profile package's, and a detector test reads them through it.
func Profiles() (map[string]json.RawMessage, error) {
	return readProfiles("feed_profiles.json")
}

// ReprocessingProfiles returns the re-processing feeds' profiles, as Profiles
// returns the base feeds'.
func ReprocessingProfiles() (map[string]json.RawMessage, error) {
	return readProfiles("reprocessing_profiles.json")
}

func readProfiles(name string) (map[string]json.RawMessage, error) {
	buf, err := testdata.ReadFile("testdata/" + name)
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	if err = json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", name, err)
	}
	return out, nil
}

// Expected is testdata/expected.json: the written expected result.
type Expected struct {
	Clock            time.Time         `json:"clock"`
	Rules            map[string]string `json:"rules"`
	NotJudgedReasons map[string]string `json:"not_judged_reasons"`
	Install          Counts            `json:"install"`
	Feeds            []ExpectedFeed    `json:"feeds"`
	Verdicts         []ExpectedVerdict `json:"verdicts"`
	// Reprocessing is the expected result over the re-processing feeds, as
	// InstallReprocessing writes them.
	Reprocessing ExpectedSet `json:"reprocessing"`
}

// ExpectedSet is the expected result over a set of feeds installed together.
type ExpectedSet struct {
	Install  Counts            `json:"install"`
	Feeds    []ExpectedFeed    `json:"feeds"`
	Verdicts []ExpectedVerdict `json:"verdicts"`
}

// ExpectedFeed is a feed as the expected result states it. Role is "case",
// "control" or "not_judged".
type ExpectedFeed struct {
	TableName  string `json:"table_name"`
	Client     string `json:"client"`
	Cumulative bool   `json:"cumulative"`
	Deliveries int    `json:"deliveries"`
	Role       string `json:"role"`
	// Reregistrations and ReprocessedFiles are the feed's re-processing as of
	// the clock: later registrations of files already delivered, and the
	// files. Neither is a delivery, and no detector judges either.
	Reregistrations  int `json:"reregistrations,omitempty"`
	ReprocessedFiles int `json:"reprocessed_files,omitempty"`
}

// ExpectedVerdict is one detector over one feed.
//
// For feed_overdue, which judges a feed at the clock, AtClock is "fires" or
// "silent"; when it fires, OnDelivery is the delivery whose session the finding
// hangs on and FirstFiresAt the first instant the rule holds.
//
// For the delivery-grain detectors, FiresOn lists the ordinals that fire,
// NotJudgedOn the ordinals that are not judged and why, and every other
// delivery is silent.
//
// NotJudged, when set, is the reason no delivery of the feed is judged.
type ExpectedVerdict struct {
	Detector     string       `json:"detector"`
	TableName    string       `json:"table_name"`
	NotJudged    string       `json:"not_judged,omitempty"`
	AtClock      string       `json:"at_clock,omitempty"`
	OnDelivery   int          `json:"on_delivery,omitempty"`
	FirstFiresAt *time.Time   `json:"first_fires_at,omitempty"`
	FiresOn      []int        `json:"fires_on,omitempty"`
	NotJudgedOn  []OrdinalWhy `json:"not_judged_on,omitempty"`
}

// OrdinalWhy is one delivery that is not judged, and the reason.
type OrdinalWhy struct {
	Delivery int    `json:"delivery"`
	Reason   string `json:"reason"`
}

// LoadExpected decodes the written expected result.
func LoadExpected() (Expected, error) {
	buf, err := testdata.ReadFile("testdata/expected.json")
	if err != nil {
		return Expected{}, err
	}
	var e Expected
	if err = json.Unmarshal(buf, &e); err != nil {
		return Expected{}, fmt.Errorf("decoding expected.json: %w", err)
	}
	return e, nil
}

// Of returns the expected verdict of a detector over a feed, base or
// re-processing.
func (e Expected) Of(detector, tableName string) (ExpectedVerdict, bool) {
	for _, v := range append(append([]ExpectedVerdict(nil), e.Verdicts...), e.Reprocessing.Verdicts...) {
		if v.Detector == detector && v.TableName == tableName {
			return v, true
		}
	}
	return ExpectedVerdict{}, false
}

// Delivery returns the verdict on one delivery, and the reason when it is not
// judged. It is for the delivery-grain detectors; for feed_overdue use AtClock.
func (v ExpectedVerdict) Delivery(ordinal int) (verdict, reason string) {
	if v.NotJudged != "" {
		return NotJudged, v.NotJudged
	}
	for _, o := range v.NotJudgedOn {
		if o.Delivery == ordinal {
			return NotJudged, o.Reason
		}
	}
	if slices.Contains(v.FiresOn, ordinal) {
		return Fires, ""
	}
	return Silent, ""
}

// Row is one expected verdict on one delivery, with its session id, as a
// detector test compares it.
type Row struct {
	Detector  string
	TableName string
	Ordinal   int
	SessionID string
	Verdict   string
	Reason    string
}

// Rows expands the delivery-grain verdicts into one row per detector and
// delivery, sessions included. feed_overdue is not expanded: it has one verdict
// per feed, at the clock.
func (e Expected) Rows() []Row { return e.rowsOf(Feeds()) }

// ReprocessingRows is Rows over the re-processing feeds: one row per detector
// and delivery. A re-registration is not a delivery and has no row.
func (e Expected) ReprocessingRows() []Row { return e.rowsOf(ReprocessingFeeds()) }

func (e Expected) rowsOf(feeds []Feed) []Row {
	var out []Row
	for _, f := range feeds {
		for _, det := range Detectors {
			if det == "feed_overdue" {
				continue
			}
			v, ok := e.Of(det, f.TableName)
			if !ok {
				continue
			}
			for _, d := range f.Deliveries {
				verdict, reason := v.Delivery(d.Ordinal)
				out = append(out, Row{Detector: det, TableName: f.TableName, Ordinal: d.Ordinal,
					SessionID: d.SessionID, Verdict: verdict, Reason: reason})
			}
		}
	}
	return out
}
