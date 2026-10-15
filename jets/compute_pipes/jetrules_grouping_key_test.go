package compute_pipes

import (
	"database/sql"
	"testing"
)

// Which domain key gives a jetrules session its grouping key. The values a worker
// then writes are tested on the Go engine in jetrules_error_keys_test.go.
func TestJetrulesGroupingDomainKey(t *testing.T) {
	twoKeys := &DomainKeysSpec{DomainKeys: map[string]*DomainKeyInfo{
		"Claim":  {KeyExpr: []string{"claim_no"}},
		"Member": {KeyExpr: []string{"member_id"}},
	}}
	oneKey := &DomainKeysSpec{DomainKeys: map[string]*DomainKeyInfo{
		"Claim": {KeyExpr: []string{"claim_no"}},
	}}
	writing := func(spec TransformationSpec) *ComputePipesConfig {
		spec.OutputChannel = OutputChannelConfig{Name: "grouped"}
		return &ComputePipesConfig{PipesConfig: []PipeSpec{
			{Apply: []TransformationSpec{{Type: "map_record", OutputChannel: OutputChannelConfig{Name: "mapped"}}}},
			{Apply: []TransformationSpec{spec}},
		}}
	}
	for _, c := range []struct {
		name     string
		cpConfig *ComputePipesConfig
		dk       *DomainKeysSpec
		want     string
	}{
		{"group_by_domain_key", writing(TransformationSpec{Type: "group_by", GroupByConfig: &GroupBySpec{DomainKey: "Member"}}), twoKeys, "Member"},
		{"merge_domain_key", writing(TransformationSpec{Type: "merge", MergeConfig: &MergeSpec{MainGroupBy: GroupBySpec{DomainKey: "Claim"}}}), twoKeys, "Claim"},
		// A group made by column names or by count has no domain key, even when the
		// class has exactly one.
		{"group_by_name", writing(TransformationSpec{Type: "group_by", GroupByConfig: &GroupBySpec{GroupByName: []string{"claim_no"}}}), oneKey, ""},
		{"group_by_count", writing(TransformationSpec{Type: "group_by", GroupByConfig: &GroupBySpec{GroupByCount: 10}}), oneKey, ""},
		// Nothing groups the input: one row per session, keyed by the class's only
		// domain key, and no guess between two.
		{"ungrouped_one_domain_key", writing(TransformationSpec{Type: "map_record"}), oneKey, "Claim"},
		{"ungrouped_two_domain_keys", writing(TransformationSpec{Type: "map_record"}), twoKeys, ""},
		{"no_config_one_domain_key", nil, oneKey, "Claim"},
		{"no_domain_keys", nil, nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, why := jetrulesGroupingDomainKey(c.cpConfig, &InputChannel{Name: "grouped", DomainKeySpec: c.dk})
			if got != c.want {
				t.Errorf("domain key = %q (%s), want %q", got, why, c.want)
			}
			if why == "" {
				t.Errorf("no reason given")
			}
		})
	}
}

// The grouping key is the domain key as compute_domain_key computes it, from the
// first row of the group; row_jets_key is set for a session of one row only.
func TestSetSessionKeys(t *testing.T) {
	columns := map[string]int{"jets:key": 0, "rdf:type": 1, "jets:source_period_sequence": 2, "claim_no": 3, "line": 4}
	for _, c := range []struct {
		name      string
		algo      string
		keyExpr   []string
		grouped   bool
		input     []any
		wantGroup sql.NullString
		wantRow   sql.NullString
	}{
		{"one_column_unhashed", "none", []string{"claim_no"}, true,
			[]any{[]any{"k1", nil, nil, "c-9", "1"}, []any{"k2", nil, nil, "c-9", "2"}},
			sql.NullString{String: "c-9", Valid: true}, sql.NullString{}},
		{"one_row_group", "none", []string{"claim_no"}, true,
			[]any{[]any{"k1", nil, nil, "c-9", "1"}},
			sql.NullString{String: "c-9", Valid: true}, sql.NullString{String: "k1", Valid: true}},
		{"ungrouped_row", "none", []string{"claim_no"}, false,
			[]any{"k1", nil, nil, "c-9", "1"},
			sql.NullString{String: "c-9", Valid: true}, sql.NullString{String: "k1", Valid: true}},
		{"composite_hashed", "sha1", []string{"claim_no", "line"}, false,
			[]any{"k1", nil, nil, "c-9", "1"},
			// computed below by the hash operator's own evaluator
			sql.NullString{}, sql.NullString{String: "k1", Valid: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			source := &InputChannel{
				Name:           "in",
				HasGroupedRows: c.grouped,
				Columns:        &columns,
				Config:         &ChannelSpec{Name: "in"},
				DomainKeySpec: &DomainKeysSpec{HashingOverride: c.algo,
					DomainKeys: map[string]*DomainKeyInfo{"Claim": {KeyExpr: c.keyExpr}}},
			}
			builder := &BuilderContext{}
			worker := &JrPoolWorker{config: &JetrulesSpec{}, source: source, groupingKey: builder.newJetrulesGroupingKey(source)}
			if worker.groupingKey == nil {
				t.Fatalf("no grouping key evaluator")
			}
			worker.setSessionKeys(&c.input)
			want := c.wantGroup
			if c.algo == "sha1" {
				// What a hash column with compute_domain_key over the same columns writes.
				he, err := builder.NewHashEvaluator(source, &HashExpression{CompositeExpr: c.keyExpr, ComputeDomainKey: true})
				if err != nil {
					t.Fatal(err)
				}
				v, err := he.ComputeDomainKey(c.input)
				if err != nil {
					t.Fatal(err)
				}
				want = sql.NullString{String: v.(string), Valid: true}
			}
			if worker.sessionGroupingKey != want {
				t.Errorf("grouping key = %+v, want %+v", worker.sessionGroupingKey, want)
			}
			if worker.sessionRowJetsKey != c.wantRow {
				t.Errorf("row jets key = %+v, want %+v", worker.sessionRowJetsKey, c.wantRow)
			}
		})
	}
}
