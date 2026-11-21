package feedprofile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

const fullProfile = `{
  "period_type": "week_period",
  "periods_per_delivery": 2,
  "overdue_after_days": 5,
  "size_bytes": {"min": 1000, "max": 900000},
  "claim_set": {
    "table": "hc:Claim",
    "key_columns": ["hc:claim_number", "hc:line_number"],
    "service_date_column": "hc:service_date",
    "member_column": "hc:member_id"
  },
  "max_service_date_lag_days": 45,
  "max_service_date_span_days": 62,
  "max_redelivered_share": 0.05,
  "required_fields": [{"column": "hc:paid_amount", "min_present_share": 0.95}],
  "declared_by": "analyst@example.com",
  "declared_at": "2026-11-21"
}`

func TestParseFullProfile(t *testing.T) {
	p, findings := Parse([]byte(fullProfile))
	if len(findings) != 0 {
		t.Fatalf("findings on a full profile: %v", findings)
	}
	if p.PeriodType != WeekPeriod || p.DeliveryPeriods() != 2 || *p.OverdueAfterDays != 5 ||
		*p.SizeBytes.Max != 900000 || p.ClaimSet.MemberColumn != "hc:member_id" ||
		len(p.ClaimSet.KeyColumns) != 2 || *p.MaxRedeliveredShare != 0.05 ||
		*p.RequiredFields[0].MinPresentShare != 0.95 || p.DeclaredAt != "2026-11-21" {
		t.Errorf("decoded %+v", p)
	}
}

func TestParseMinimalProfileJudgesNothing(t *testing.T) {
	p, findings := Parse([]byte(`{"declared_by": "a", "declared_at": "2026-01-31"}`))
	if len(findings) != 0 {
		t.Fatalf("findings: %v", findings)
	}
	if p.DeliveryPeriods() != 1 || p.ClaimSet != nil || p.OverdueAfterDays != nil {
		t.Errorf("a minimal profile declares nothing beyond who and when: %+v", p)
	}
}

func TestParseRefuses(t *testing.T) {
	cases := []struct {
		name, doc, path, code string
	}{
		{"empty", ``, "", "feed_profile.empty"},
		{"null", `null`, "", "feed_profile.empty"},
		{"not json", `{"declared_by": }`, "", "feed_profile.decode"},
		{"array", `[]`, "", "feed_profile.decode"},
		{"unknown field", `{"declared_by":"a","declared_at":"2026-01-01","delivery_mode":"cumulative"}`, "", "feed_profile.decode"},
		{"trailing", `{"declared_by":"a","declared_at":"2026-01-01"} {}`, "", "feed_profile.decode"},
		{"wrong type", `{"declared_by":"a","declared_at":"2026-01-01","overdue_after_days":"5"}`, "/overdue_after_days", "feed_profile.decode"},
		{"no declarer", `{"declared_at":"2026-01-01"}`, "/declared_by", "feed_profile.invalid"},
		{"no date", `{"declared_by":"a"}`, "/declared_at", "feed_profile.invalid"},
		{"bad date", `{"declared_by":"a","declared_at":"21/11/2026"}`, "/declared_at", "feed_profile.invalid"},
		{"period type", `{"declared_by":"a","declared_at":"2026-01-01","period_type":"quarter_period"}`, "/period_type", "feed_profile.invalid"},
		{"zero periods", `{"declared_by":"a","declared_at":"2026-01-01","periods_per_delivery":0}`, "/periods_per_delivery", "feed_profile.invalid"},
		{"negative grace", `{"declared_by":"a","declared_at":"2026-01-01","overdue_after_days":-1}`, "/overdue_after_days", "feed_profile.invalid"},
		{"negative lag", `{"declared_by":"a","declared_at":"2026-01-01","max_service_date_lag_days":-1}`, "/max_service_date_lag_days", "feed_profile.invalid"},
		{"negative span", `{"declared_by":"a","declared_at":"2026-01-01","max_service_date_span_days":-3}`, "/max_service_date_span_days", "feed_profile.invalid"},
		{"share above one", `{"declared_by":"a","declared_at":"2026-01-01","max_redelivered_share":1.5}`, "/max_redelivered_share", "feed_profile.invalid"},
		{"empty size", `{"declared_by":"a","declared_at":"2026-01-01","size_bytes":{}}`, "/size_bytes", "feed_profile.invalid"},
		{"size inverted", `{"declared_by":"a","declared_at":"2026-01-01","size_bytes":{"min":10,"max":5}}`, "/size_bytes", "feed_profile.invalid"},
		{"negative size", `{"declared_by":"a","declared_at":"2026-01-01","size_bytes":{"min":-1}}`, "/size_bytes/min", "feed_profile.invalid"},
		{"no key", `{"declared_by":"a","declared_at":"2026-01-01","claim_set":{"table":"t","key_columns":[],"service_date_column":"d","member_column":"m"}}`, "/claim_set/key_columns", "feed_profile.invalid"},
		{"key twice", `{"declared_by":"a","declared_at":"2026-01-01","claim_set":{"table":"t","key_columns":["k","k"],"service_date_column":"d","member_column":"m"}}`, "/claim_set/key_columns/1", "feed_profile.invalid"},
		{"no member", `{"declared_by":"a","declared_at":"2026-01-01","claim_set":{"table":"t","key_columns":["k"],"service_date_column":"d"}}`, "/claim_set/member_column", "feed_profile.invalid"},
		{"padded table", `{"declared_by":"a","declared_at":"2026-01-01","claim_set":{"table":" t","key_columns":["k"],"service_date_column":"d","member_column":"m"}}`, "/claim_set/table", "feed_profile.invalid"},
		{"no share", `{"declared_by":"a","declared_at":"2026-01-01","required_fields":[{"column":"c"}]}`, "/required_fields/0/min_present_share", "feed_profile.invalid"},
		{"zero share", `{"declared_by":"a","declared_at":"2026-01-01","required_fields":[{"column":"c","min_present_share":0}]}`, "/required_fields/0/min_present_share", "feed_profile.invalid"},
		{"field twice", `{"declared_by":"a","declared_at":"2026-01-01","required_fields":[{"column":"c","min_present_share":1},{"column":"c","min_present_share":0.5}]}`, "/required_fields/1/column", "feed_profile.invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, findings := Parse([]byte(c.doc))
			if len(findings) == 0 {
				t.Fatal("no finding")
			}
			f := findings[0]
			if f.Code != c.code || f.Path != c.path || f.Severity != "error" {
				t.Errorf("first finding %+v, want code %s at %q", f, c.code, c.path)
			}
			if err := Errors(findings); err == nil || (c.path != "" && !strings.Contains(err.Error(), c.path)) {
				t.Errorf("Errors = %v, want it to name %q", err, c.path)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	for _, s := range []string{"", "  ", "null", " null\n"} {
		if !IsEmpty(s) {
			t.Errorf("IsEmpty(%q) = false", s)
		}
	}
	if IsEmpty("{}") {
		t.Error(`IsEmpty("{}") = true`)
	}
}

// The schema and Validate state one contract. These hold them to the same
// fields, the same required lists and the same period types, so a field added
// to one and not the other fails here rather than in a detector.
func TestSchemaAgreesWithTheTypes(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(SchemaJSON, &schema); err != nil {
		t.Fatalf("the embedded schema is not JSON: %v", err)
	}
	props := schema["properties"].(map[string]any)

	check := func(where string, node map[string]any, typ reflect.Type) {
		t.Helper()
		if node["additionalProperties"] != false {
			t.Errorf("%s: the schema must close the object, as decoding does", where)
		}
		var fromSchema, fromType, reqSchema, reqType []string
		for k := range node["properties"].(map[string]any) {
			fromSchema = append(fromSchema, k)
		}
		if r, ok := node["required"].([]any); ok {
			for _, k := range r {
				reqSchema = append(reqSchema, k.(string))
			}
		}
		for i := 0; i < typ.NumField(); i++ {
			name, opts, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			fromType = append(fromType, name)
			// A required field is one decoding does not omit when empty.
			if opts != "omitempty" {
				reqType = append(reqType, name)
			}
		}
		for _, s := range [][]string{fromSchema, fromType, reqSchema, reqType} {
			sort.Strings(s)
		}
		if !slices.Equal(fromSchema, fromType) {
			t.Errorf("%s: schema properties %v, type fields %v", where, fromSchema, fromType)
		}
		if !slices.Equal(reqSchema, reqType) {
			t.Errorf("%s: schema requires %v, type requires %v", where, reqSchema, reqType)
		}
	}
	check("profile", schema, reflect.TypeOf(Profile{}))
	check("size_bytes", props["size_bytes"].(map[string]any), reflect.TypeOf(SizeRange{}))
	check("claim_set", props["claim_set"].(map[string]any), reflect.TypeOf(ClaimSet{}))
	check("required_fields", props["required_fields"].(map[string]any)["items"].(map[string]any), reflect.TypeOf(RequiredField{}))

	var enum []string
	for _, v := range props["period_type"].(map[string]any)["enum"].([]any) {
		enum = append(enum, v.(string))
	}
	if !slices.Equal(enum, PeriodTypes) {
		t.Errorf("period_type enum %v, PeriodTypes %v", enum, PeriodTypes)
	}
	if _, ok := props["delivery_mode"]; ok {
		t.Error("delivery_mode is source_config.is_cumulative, not a profile field")
	}
}

// fakeRows and fakeDB stand in for pgx.Rows and *pgxpool.Pool.
type fakeRows struct {
	rows [][]any
	i    int
	err  error
}

func (r *fakeRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *fakeRows) Close()     {}
func (r *fakeRows) Err() error { return r.err }
func (r *fakeRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	for i, d := range dest {
		switch d := d.(type) {
		case *string:
			*d = row[i].(string)
		case *int:
			*d = row[i].(int)
		case **string:
			if row[i] == nil {
				*d = nil
			} else {
				s := row[i].(string)
				*d = &s
			}
		}
	}
	return nil
}

type fakeDB struct {
	rows   *fakeRows
	sql    string
	client any
	err    error
}

func (db *fakeDB) Query(_ context.Context, sql string, args ...any) (*fakeRows, error) {
	db.sql, db.client = sql, args[0]
	return db.rows, db.err
}

func TestLoadSplitsProfiledFromUnprofiled(t *testing.T) {
	db := &fakeDB{rows: &fakeRows{rows: [][]any{
		{"c", "o", "hc:Claim", "c_o_claim", 1, fullProfile},
		{"c", "o", "hc:Eligibility", "c_o_elig", 0, nil},
		{"c", "", "hc:Pharmacy", "c_pharmacy", 0, "null"},
		{"c", "o", "hc:Provider", "c_o_provider", 0, `{"declared_by":"a"}`},
	}}}
	profiled, unprofiled, err := Load(context.Background(), db, "c")
	if err != nil {
		t.Fatal(err)
	}
	if db.client != "c" || !strings.Contains(db.sql, "to_jsonb(sc) ->> 'feed_profile_json'") {
		t.Errorf("query %q with %v", db.sql, db.client)
	}
	if len(profiled) != 1 || profiled[0].TableName != "c_o_claim" || !profiled[0].Cumulative ||
		profiled[0].Profile.ClaimSet == nil || len(profiled[0].Findings) != 0 {
		t.Errorf("profiled %+v", profiled)
	}
	if len(unprofiled) != 3 {
		t.Fatalf("unprofiled %+v", unprofiled)
	}
	if len(unprofiled[0].Findings)+len(unprofiled[1].Findings) != 0 {
		t.Error("a feed with no profile carries no findings")
	}
	if f := unprofiled[2]; f.TableName != "c_o_provider" || len(f.Findings) == 0 || f.Findings[0].Path != "/declared_at" {
		t.Errorf("a refused stored profile says why: %+v", f)
	}
}

func TestLoadReportsTheQueryError(t *testing.T) {
	db := &fakeDB{err: errors.New("relation does not exist")}
	if _, _, err := Load(context.Background(), db, "c"); err == nil || !strings.Contains(err.Error(), "client c") {
		t.Errorf("err = %v", err)
	}
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileAndAttach(t *testing.T) {
	path := writeFile(t, `{"c_o_claim": `+fullProfile+`, "c_o_missing": {"declared_by":"a","declared_at":"2026-01-01"}}`)
	profiles, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	feeds := []Feed{
		{Client: "c", TableName: "c_o_claim", Cumulative: true},
		{Client: "c", TableName: "c_o_elig"},
	}
	profiled, unprofiled, unmatched := Attach(feeds, profiles)
	if len(profiled) != 1 || profiled[0].TableName != "c_o_claim" || !profiled[0].Cumulative ||
		profiled[0].Profile.PeriodType != WeekPeriod {
		t.Errorf("profiled %+v", profiled)
	}
	if len(unprofiled) != 1 || unprofiled[0].TableName != "c_o_elig" {
		t.Errorf("unprofiled %+v", unprofiled)
	}
	if !slices.Equal(unmatched, []string{"c_o_missing"}) {
		t.Errorf("unmatched %v", unmatched)
	}
}

func TestLoadFileRefuses(t *testing.T) {
	cases := map[string]string{
		"bad profile": `{"t1": {"declared_by":"a","declared_at":"2026-01-01"}, "t2": {"declared_by":"a"}}`,
		"null":        `null`,
		"array":       `[]`,
		"twice":       `{"t1": {"declared_by":"a","declared_at":"2026-01-01"}, "t1": {"declared_by":"b","declared_at":"2026-01-01"}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadFile(writeFile(t, content))
			if err == nil {
				t.Fatal("accepted")
			}
			if name == "bad profile" && !strings.Contains(err.Error(), "t2: invalid feed profile: /declared_at") {
				t.Errorf("err %v does not name the table and the path", err)
			}
		})
	}
}
