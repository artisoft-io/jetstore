package compute_pipes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func windowPipe(t *testing.T, config *MapRecordSpec, env map[string]any, results *ChannelResults) (*MapRecordTransformationPipe, error) {
	pipe, _, err := windowPipeOut(t, config, env, results)
	return pipe, err
}

func windowPipeOut(t *testing.T, config *MapRecordSpec, env map[string]any, results *ChannelResults) (*MapRecordTransformationPipe, chan []any, error) {
	t.Helper()
	spec := &TransformationSpec{Type: "map_record", MapRecordConfig: config}
	source := &InputChannel{
		Name:    "in",
		Columns: &map[string]int{"key": 0, "dos": 1},
		Config:  &ChannelSpec{Name: "in"},
	}
	out := make(chan []any, 64)
	outputCh := &OutputChannel{
		Name:    "out",
		Channel: out,
		Columns: &map[string]int{"key": 0, "dos": 1},
		Config:  &ChannelSpec{Name: "out", Columns: []string{"key", "dos"}},
	}
	ctx := &BuilderContext{done: make(chan struct{}), chResults: results, env: env}
	pipe, err := ctx.NewMapRecordTransformationPipe(source, outputCh, spec)
	return pipe, out, err
}

func fileDateEnv(date string, more ...any) map[string]any {
	env := map[string]any{"$DATE_FILE_KEY": date}
	for i := 0; i+1 < len(more); i += 2 {
		env[more[i].(string)] = more[i+1]
	}
	return env
}

// A record is kept when its column is dated within the window around the file
// date, bounds included, and dropped otherwise, counted by its reason; the
// dropped records never reach the output channel; the span and the day counts
// see every record, as delivered.
func TestRetainWindowKeepsTheWindowAndCountsTheRest(t *testing.T) {
	results := &ChannelResults{InputSpan: &InputSpan{}, RetainWindow: &RetainWindowDrops{}}
	pipe, out, err := windowPipeOut(t, &MapRecordSpec{
		CalculateSpanOf: "dos",
		RetainWindow:    &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: float64(30), DaysAfterFileDate: float64(2)},
	}, fileDateEnv("2026-03-31"), results)
	if err != nil {
		t.Fatal(err)
	}
	records := [][]any{
		{"in-start", spanDay("2026-03-01")}, // the first day of the window
		{"in-end", "2026-04-02"},            // the last day
		{"in-mid", spanDay("2026-03-15")},
		{"before", spanDay("2026-02-28")},
		{"long-before", "2021-06-01"},
		{"after", spanDay("2026-04-03")},
		{"null", nil},
		{"garbage", "not a date"},
		{"array-inside", []any{spanDay("2025-01-01"), spanDay("2026-03-20")}},
		{"array-before", []any{spanDay("2025-01-01"), spanDay("2025-02-01")}},
	}
	for _, rec := range records {
		if err := pipe.Apply(&rec); err != nil {
			t.Fatalf("Apply(%v): %v", rec, err)
		}
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	close(out)
	var kept []string
	for rec := range out {
		kept = append(kept, rec[0].(string))
	}
	if got := strings.Join(kept, ","); got != "in-start,in-end,in-mid,array-inside" {
		t.Errorf("kept %s", got)
	}
	c, ok := results.RetainWindow.Counts()
	want := RetainWindowCounts{Column: "dos", DaysBefore: 30, DaysAfter: 2, NoDate: 2, Before: 3, After: 1}
	if !ok || c != want {
		t.Errorf("drops = %+v %v, want %+v", c, ok, want)
	}
	// The span is of every record, kept or dropped.
	if _, lo, hi, ok := results.InputSpan.Values(); !ok || !lo.Equal(spanDay("2021-06-01")) || !hi.Equal(spanDay("2026-04-03")) {
		t.Errorf("span = %v %v %v, want 2021-06-01 2026-04-03", lo, hi, ok)
	}
	days := results.InputSpan.DayCounts()
	if days["2025-01-01"] != 2 || days["2021-06-01"] != 1 || days["2026-04-03"] != 1 || len(days) != 9 {
		t.Errorf("day counts %v", days)
	}
}

// The day counts take env var substitution, as any pipeline parameter; the
// days after default to 0.
func TestRetainWindowSubstitutesEnvVars(t *testing.T) {
	results := &ChannelResults{RetainWindow: &RetainWindowDrops{}}
	pipe, err := windowPipe(t, &MapRecordSpec{
		RetainWindow: &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: "${RETAIN_WINDOW_DAYS}"},
	}, fileDateEnv("2026-03-31", "${RETAIN_WINDOW_DAYS}", "10"), results)
	if err != nil {
		t.Fatal(err)
	}
	if pipe.window.daysBefore != 10 || pipe.window.daysAfter != 0 ||
		!pipe.window.start.Equal(spanDay("2026-03-21")) || !pipe.window.end.Equal(spanDay("2026-03-31")) {
		t.Errorf("window %+v", pipe.window)
	}
	pipe, err = windowPipe(t, &MapRecordSpec{
		RetainWindow: &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: "$BEFORE", DaysAfterFileDate: " 3 "},
	}, fileDateEnv("2026-03-31", "$BEFORE", 400), results)
	if err != nil {
		t.Fatal(err)
	}
	if pipe.window.daysBefore != 400 || pipe.window.daysAfter != 3 {
		t.Errorf("window %+v", pipe.window)
	}
}

// A window that could not decide is refused when the pipeline is built, not
// after a run that kept or dropped the wrong records.
func TestRetainWindowRefusals(t *testing.T) {
	for _, c := range []struct {
		name string
		spec *RetainWindowSpec
		env  map[string]any
		want string
	}{
		{"a column the output does not have", &RetainWindowSpec{Column: "paid", DaysBeforeFileDate: float64(5)},
			fileDateEnv("2026-03-31"), "is not a column of output channel"},
		{"no column", &RetainWindowSpec{DaysBeforeFileDate: float64(5)}, fileDateEnv("2026-03-31"), "names no column"},
		{"no days before", &RetainWindowSpec{Column: "dos"}, fileDateEnv("2026-03-31"), "no days_before_file_date"},
		{"a fraction", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: 1.5}, fileDateEnv("2026-03-31"), "not an integer"},
		{"negative", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: float64(-1)}, fileDateEnv("2026-03-31"), "negative"},
		{"negative after substitution", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: float64(5), DaysAfterFileDate: "$AFTER"},
			fileDateEnv("2026-03-31", "$AFTER", "-2"), "negative"},
		{"an unresolved env var", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: "${NO_SUCH_VAR}"},
			fileDateEnv("2026-03-31"), "'${NO_SUCH_VAR}' once its env vars are substituted"},
		{"a boolean", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: true}, fileDateEnv("2026-03-31"), "neither an integer nor a string"},
		{"no file date", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: float64(5)}, map[string]any{}, "$DATE_FILE_KEY"},
		{"a file date that is not a date", &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: float64(5)},
			fileDateEnv("someday"), "'someday' is not a date"},
	} {
		_, err := windowPipe(t, &MapRecordSpec{RetainWindow: c.spec}, c.env, &ChannelResults{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error containing %q", c.name, err, c.want)
		}
	}
}

// The configuration reads the day counts as numbers or as strings.
func TestRetainWindowSpecDecodes(t *testing.T) {
	var spec MapRecordSpec
	if err := json.Unmarshal([]byte(`{"retain_window": {"column": "dos", "days_before_file_date": 365,
		"days_after_file_date": "$AFTER"}}`), &spec); err != nil {
		t.Fatal(err)
	}
	w := spec.RetainWindow
	if w == nil || w.Column != "dos" || w.DaysBeforeFileDate != float64(365) || w.DaysAfterFileDate != "$AFTER" {
		t.Errorf("decoded %+v", w)
	}
}

// Two steps' drops add up; the first step's window is the one recorded.
func TestRetainWindowDropsMerge(t *testing.T) {
	r := &RetainWindowDrops{}
	if _, ok := r.Counts(); ok {
		t.Error("an empty node reports a window")
	}
	r.merge(&retainWindow{column: "dos", daysBefore: 30, noDate: 1, before: 2, after: 3})
	r.merge(&retainWindow{column: "dos", daysBefore: 30, noDate: 10, before: 20, after: 30})
	c, ok := r.Counts()
	if want := (RetainWindowCounts{Column: "dos", DaysBefore: 30, NoDate: 11, Before: 22, After: 33}); !ok || c != want {
		t.Errorf("merged %+v, want %+v", c, want)
	}
}

// A node with no window writes nothing; one with a window writes it and its
// drops, zeros included, to its own detail row.
func TestRecordRetainWindow(t *testing.T) {
	if err := RecordRetainWindow(context.Background(), failingExecer{}, 7, &RetainWindowDrops{}); err != nil {
		t.Errorf("a node with no window wrote: %v", err)
	}
	if err := RecordRetainWindow(context.Background(), failingExecer{}, 7, nil); err != nil {
		t.Errorf("a node with no window collector wrote: %v", err)
	}
	r := &RetainWindowDrops{}
	r.merge(&retainWindow{column: "dos", daysBefore: 30, daysAfter: 2})
	rec := &recordingExecer{}
	if err := RecordRetainWindow(context.Background(), rec, 7, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.sql, "dropped_before_window_count") || len(rec.args) != 7 ||
		rec.args[0] != "dos" || rec.args[1] != 30 || rec.args[2] != 2 || rec.args[3] != int64(0) || rec.args[6] != 7 {
		t.Errorf("RecordRetainWindow wrote %q with %v", rec.sql, rec.args)
	}
}

// The day counts are keyed by date, also before 1970.
func TestInputSpanDayCounts(t *testing.T) {
	tr := &spanTracker{column: "dos"}
	for _, d := range []string{"1969-12-31", "1969-12-31", "1970-01-01", "2026-02-28"} {
		tr.add(spanDay(d))
	}
	s := &InputSpan{}
	s.merge(tr)
	s.merge(&spanTracker{column: "dos", min: spanDay("2026-02-28"), max: spanDay("2026-02-28"), seen: true,
		days: map[int64]int64{spanDay("2026-02-28").Unix() / secondsPerDay: 5}})
	got := s.DayCounts()
	if len(got) != 3 || got["1969-12-31"] != 2 || got["1970-01-01"] != 1 || got["2026-02-28"] != 6 {
		t.Errorf("day counts %v", got)
	}
	if (&InputSpan{}).DayCounts() != nil {
		t.Error("a node with no span has day counts")
	}
}

// One feed overrides the window a pipeline defaults, with no mechanism of the
// window's own. The pipeline configuration gives the window's days as an env
// var and defaults it with a context entry of type default_value; a feed's
// source_config.schema_provider_json sets the same key in its env, which is
// merged into the main input's env before the context entries run, and
// default_value fills only a key still unset. Both arms through the run's own
// code: the merge, prepareCpipesEnv, and the window's builder.
func TestRetainWindowDefaultAndPerFeedOverride(t *testing.T) {
	context := []ContextSpec{{Type: "default_value", Key: "${RETAIN_WINDOW_DAYS}", Expr: "180"}}
	spec := &MapRecordSpec{RetainWindow: &RetainWindowSpec{Column: "dos", DaysBeforeFileDate: "${RETAIN_WINDOW_DAYS}"}}
	for _, c := range []struct {
		name         string
		sourceConfig string // the feed's schema_provider_json, "" for none
		want         int
	}{
		{"the pipeline's default", "", 180},
		{"a feed with a schema provider that does not set it", `{"trim_columns": true}`, 180},
		{"a feed that overrides it", `{"env": {"${RETAIN_WINDOW_DAYS}": "270"}}`, 270},
	} {
		// The run's start sets $DATE_FILE_KEY on the main input's env, then
		// merges the feed's schema provider into it.
		provider := map[string]any{"$DATE_FILE_KEY": "2024-08-02"}
		if c.sourceConfig != "" {
			sp := &SchemaProviderSpec{Env: provider}
			if err := mergeSourceConfigSchemaProvider(c.sourceConfig, sp); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			provider = sp.Env
		}
		env := envFor(t, provider, context)
		pipe, err := windowPipe(t, spec, env, &ChannelResults{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if pipe.window.daysBefore != c.want ||
			!pipe.window.start.Equal(spanDay("2024-08-02").AddDate(0, 0, -c.want)) {
			t.Errorf("%s: window %d days from %v, want %d days", c.name, pipe.window.daysBefore, pipe.window.start, c.want)
		}
	}
}

// The anchor reads $DATE_FILE_KEY as the run's start sets it, a YYYY-MM-DD
// string, and as a date, which prepareCpipesEnv leaves when nothing set it.
func TestRetainWindowAnchorReadsAStringOrADate(t *testing.T) {
	for _, v := range []any{"2024-08-02", " 2024-08-02 ", time.Date(2024, 8, 2, 14, 0, 0, 0, time.UTC)} {
		d, err := retainWindowAnchor(map[string]any{"$DATE_FILE_KEY": v})
		if err != nil || !d.Equal(spanDay("2024-08-02")) {
			t.Errorf("%#v: %v %v", v, d, err)
		}
	}
	for _, v := range []any{nil, "", time.Time{}, 20240802} {
		if _, err := retainWindowAnchor(map[string]any{"$DATE_FILE_KEY": v}); err == nil {
			t.Errorf("%#v: accepted", v)
		}
	}
}
