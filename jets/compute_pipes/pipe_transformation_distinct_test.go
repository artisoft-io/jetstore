package compute_pipes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// runDistinct feeds the records through a distinct operator over an input
// channel with the given columns, and returns the records it sent on, in order.
func runDistinct(t *testing.T, columns []string, distinctOn []string, records [][]any) [][]any {
	t.Helper()
	spec := &TransformationSpec{
		Type:           "distinct",
		DistinctConfig: &DistinctSpec{DistinctOn: distinctOn},
	}
	colMap := make(map[string]int, len(columns))
	for i, c := range columns {
		colMap[c] = i
	}
	source := &InputChannel{
		Name:    "in",
		Columns: &colMap,
		Config:  &ChannelSpec{Name: "in", Columns: columns},
	}
	outCh := make(chan []any)
	ctx := &BuilderContext{done: make(chan struct{})}
	pipe, err := ctx.NewDistinctTransformationPipe(source, &OutputChannel{Channel: outCh}, spec)
	if err != nil {
		t.Fatal(err)
	}
	var out [][]any
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for row := range outCh {
			out = append(out, row)
		}
	}()
	for _, r := range records {
		if err := pipe.Apply(&r); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	close(outCh)
	wg.Wait()
	return out
}

func assertRows(t *testing.T, got, want [][]any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("distinct sent\n  %v\nwant\n  %v", got, want)
	}
}

// distinct_on a column that is not the channel's first keys on that column.
// Before, the key was read from the first len(distinct_on) positions, so this
// kept one record per value of column a.
func TestDistinctOnNonFirstColumn(t *testing.T) {
	cols := []string{"a", "b", "c"}
	got := runDistinct(t, cols, []string{"c"}, [][]any{
		{"1", "x", "p"},
		{"1", "y", "q"}, // same a, new c: kept
		{"2", "z", "p"}, // new a, repeated c: dropped
		{"3", "w", "q"}, // dropped
	})
	assertRows(t, got, [][]any{{"1", "x", "p"}, {"1", "y", "q"}})
}

// Two keys whose concatenations are equal are two keys.
func TestDistinctConcatenationsDoNotCollide(t *testing.T) {
	cols := []string{"a", "b"}
	got := runDistinct(t, cols, []string{"a", "b"}, [][]any{
		{"ab", "c"},
		{"a", "bc"},
		{"", "abc"},
		{"abc", ""},
		{"ab", "c"}, // a true repeat: dropped
	})
	assertRows(t, got, [][]any{{"ab", "c"}, {"a", "bc"}, {"", "abc"}, {"abc", ""}})
}

// A nil is not the string "<nil>", nor the empty string, and two nils are one key.
func TestDistinctNilIsItsOwnValue(t *testing.T) {
	cols := []string{"a", "b"}
	got := runDistinct(t, cols, []string{"a", "b"}, [][]any{
		{nil, "x"},
		{"<nil>", "x"},
		{"", "x"},
		{"~", "x"},
		{nil, "x"}, // dropped
		{nil, nil},
		{"~", nil},
		{nil, nil}, // dropped
	})
	assertRows(t, got, [][]any{
		{nil, "x"}, {"<nil>", "x"}, {"", "x"}, {"~", "x"}, {nil, nil}, {"~", nil},
	})
}

// Several columns, named in an order other than the channel's, key on those
// columns and no other.
func TestDistinctOnColumnsOutOfChannelOrder(t *testing.T) {
	cols := []string{"a", "b", "c", "d"}
	got := runDistinct(t, cols, []string{"d", "b"}, [][]any{
		{"1", "x", "k", "p"},
		{"2", "x", "l", "p"}, // same (d, b): dropped
		{"1", "y", "k", "p"}, // new b: kept
		{"1", "x", "k", "q"}, // new d: kept
		{"9", "y", "m", "p"}, // repeats (p, y): dropped
	})
	assertRows(t, got, [][]any{
		{"1", "x", "k", "p"}, {"1", "y", "k", "p"}, {"1", "x", "k", "q"},
	})
}

// Values that are not strings key on their text, as fmt's %v prints it, and the
// fast paths for the common types print what %v prints.
func TestDistinctNonStringValues(t *testing.T) {
	d := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	cols := []string{"n", "f", "d"}
	got := runDistinct(t, cols, []string{"n", "f", "d"}, [][]any{
		{1, 1.5, d},
		{int64(1), 1.5, d}, // same text: dropped
		{"1", "1.5", d},    // same text: dropped
		{12, 1.5, d},
		{1, 2.5, d},
		{1, 1.5, d.AddDate(0, 0, 1)},
	})
	assertRows(t, got, [][]any{
		{1, 1.5, d}, {12, 1.5, d}, {1, 2.5, d}, {1, 1.5, d.AddDate(0, 0, 1)},
	})

	for _, v := range []any{0, -7, int64(1) << 40, int32(-3), 1.0, 0.1, 1e21, -2.5e-9} {
		k, _ := appendDistinctKey(nil, nil, v)
		text := fmt.Sprintf("%v", v)
		want := fmt.Sprintf("%d:%s", len(text), text)
		if string(k) != want {
			t.Errorf("key of %T %v = %q, want %q", v, v, k, want)
		}
	}
}

// A record whose length is not the input channel's is dropped, as before.
func TestDistinctSkipsMisshapenRecords(t *testing.T) {
	cols := []string{"a", "b"}
	got := runDistinct(t, cols, []string{"b"}, [][]any{
		{"1"},
		{"1", "x", "extra"},
		{"1", "x"},
	})
	assertRows(t, got, [][]any{{"1", "x"}})
}

func TestDistinctUnknownColumnIsRefused(t *testing.T) {
	colMap := map[string]int{"a": 0}
	ctx := &BuilderContext{done: make(chan struct{})}
	_, err := ctx.NewDistinctTransformationPipe(
		&InputChannel{Columns: &colMap, Config: &ChannelSpec{Columns: []string{"a"}}},
		&OutputChannel{},
		&TransformationSpec{Type: "distinct", DistinctConfig: &DistinctSpec{DistinctOn: []string{"z"}}})
	if err == nil {
		t.Fatal("distinct_on naming a column the input channel does not have was accepted")
	}
}

// With no distinct_on every record has the same key and only the first would
// be kept, so the builder refuses it, naming the channel it reads.
func TestDistinctEmptyKeyIsRefused(t *testing.T) {
	colMap := map[string]int{"a": 0}
	ctx := &BuilderContext{done: make(chan struct{})}
	for _, config := range []*DistinctSpec{{}, {DistinctOn: []string{}}} {
		_, err := ctx.NewDistinctTransformationPipe(
			&InputChannel{Name: "in", Columns: &colMap, Config: &ChannelSpec{Columns: []string{"a"}}},
			&OutputChannel{},
			&TransformationSpec{Type: "distinct", DistinctConfig: config})
		if err == nil || !strings.Contains(err.Error(), "'in'") || !strings.Contains(err.Error(), "distinct_on") {
			t.Errorf("an empty distinct_on: %v", err)
		}
	}
}

// The start refuses a distinct step that is certain to have no distinct_on,
// naming the step, and reads a conditional_config override with its host as
// MergeTransformationSpec applies it.
func TestValidateDistinctOn(t *testing.T) {
	for what, c := range map[string]struct {
		apply  string
		refuse string // the site the message names, or "" when accepted
	}{
		"a distinct_on":        {`{"type": "distinct", "distinct_config": {"distinct_on": ["a"]}}`, ""},
		"an empty distinct_on": {`{"type": "distinct", "distinct_config": {"distinct_on": []}}`, `(step_name "s"), pipe 0, apply 0 is`},
		"no distinct_config":   {`{"type": "distinct"}`, `(step_name "s"), pipe 0, apply 0 is`},
		"not a distinct step":  {`{"type": "map_record", "distinct_config": {}}`, ""},
		"supplied by an override": {`{"type": "distinct", "conditional_config": [{"when": {"type": "value", "expr": "1"},
		  "then": {"distinct_config": {"distinct_on": ["a"]}}}]}`, ""},
		"an override with an empty distinct_on": {`{"type": "distinct", "distinct_config": {"distinct_on": ["a"]},
		  "conditional_config": [{"then": {"distinct_config": {"distinct_on": []}}}]}`, "apply 0, conditional_config[0] is"},
		"an override that becomes a distinct with none": {`{"type": "map_record",
		  "conditional_config": [{"then": {"type": "distinct"}}]}`, "apply 0, conditional_config[0] is"},
		"a host with none, overridden only by another operator": {`{"type": "distinct",
		  "conditional_config": [{"then": {"type": "map_record"}}]}`, `(step_name "s"), pipe 0, apply 0 is`},
		"an override of another operator's config": {`{"type": "map_record",
		  "conditional_config": [{"then": {"distinct_config": {}}}]}`, ""},
	} {
		content := `{"cluster_config": {}, "conditional_pipes_config": [{"step_name": "s", "pipes_config": [{"type": "fan_out",
		  "input_channel": {"type": "input", "name": "input_row"}, "apply": [` + c.apply + `]}]}]}`
		var cfg ComputePipesConfig
		if err := json.Unmarshal([]byte(content), &cfg); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		err := validateDistinctOn(&cfg)
		switch {
		case c.refuse == "" && err != nil:
			t.Errorf("%s: refused: %v", what, err)
		case c.refuse != "" && (err == nil || !strings.Contains(err.Error(), c.refuse)):
			t.Errorf("%s: %v, want a refusal naming %q", what, err, c.refuse)
		}
	}
}

func BenchmarkDistinctApply(b *testing.B) {
	cols := []string{"a", "b", "c", "d"}
	colMap := map[string]int{"a": 0, "b": 1, "c": 2, "d": 3}
	outCh := make(chan []any, 1024)
	ctx := &BuilderContext{done: make(chan struct{})}
	pipe, err := ctx.NewDistinctTransformationPipe(
		&InputChannel{Columns: &colMap, Config: &ChannelSpec{Columns: cols}},
		&OutputChannel{Channel: outCh},
		&TransformationSpec{Type: "distinct", DistinctConfig: &DistinctSpec{DistinctOn: []string{"c", "a"}}})
	if err != nil {
		b.Fatal(err)
	}
	go func() {
		for range outCh {
		}
	}()
	rows := make([][]any, 1000)
	for i := range rows {
		rows[i] = []any{fmt.Sprintf("member-%d", i%300), "x", fmt.Sprintf("2026-01-%02d", i%28), 42}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := rows[i%len(rows)]
		if err := pipe.Apply(&r); err != nil {
			b.Fatal(err)
		}
	}
}
