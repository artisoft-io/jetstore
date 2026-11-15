package compute_pipes

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// completedAmong is a completedSessionsFn over a fixed set of completed sessions,
// recording the ids it was asked about.
func completedAmong(asked *[][]string, completed ...string) completedSessionsFn {
	return func(ids []string) (map[string]bool, error) {
		if asked != nil {
			*asked = append(*asked, slices.Sorted(slices.Values(ids)))
		}
		done := make(map[string]bool)
		for _, id := range ids {
			if slices.Contains(completed, id) {
				done[id] = true
			}
		}
		return done, nil
	}
}

// A week folder as a mapping pipeline writes it: one folder per session.
const week = "stage/c/ClaimDetails/2900W"

func TestLatestSessionKeepsTheLatestCompletedSessionOfEachPeriod(t *testing.T) {
	periods := []lookbackPeriodObjects{
		period(2900, week,
			week+"/1790000000000/jets_partition=0000P/part-0001.csv", // mapped before the model change
			week+"/1795000000000/jets_partition=0000P/part-0001.csv", // the remap
			week+"/1795000000000/jets_partition=0001P/part-0001.csv",
			week+"/1793000000000/jets_partition=0000P/part-0001.csv"), // a duplicate
		period(2899, "stage/c/ClaimDetails/2899W",
			"stage/c/ClaimDetails/2899W/1789000000000/part-0001.csv"),
	}
	var asked [][]string
	got, err := keepLatestCompletedSessions(periods,
		completedAmong(&asked, "1790000000000", "1793000000000", "1795000000000", "1789000000000"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		week + "/1795000000000/jets_partition=0000P/part-0001.csv",
		week + "/1795000000000/jets_partition=0001P/part-0001.csv",
	}
	if !slices.Equal(keysOf(got[0].objects), want) {
		t.Errorf("period 2900: got %v, want %v", keysOf(got[0].objects), want)
	}
	if len(got[1].objects) != 1 {
		t.Errorf("period 2899: got %v, want its one session", keysOf(got[1].objects))
	}
	// One registry read per period, with that period's sessions only.
	if len(asked) != 2 || len(asked[0]) != 3 || len(asked[1]) != 1 {
		t.Errorf("registry asked %v, want 3 sessions then 1", asked)
	}
	// The input is not modified.
	if len(periods[0].objects) != 4 {
		t.Errorf("input period modified: %v", keysOf(periods[0].objects))
	}
}

func TestLatestSessionSkipsANewerRunThatDidNotComplete(t *testing.T) {
	periods := []lookbackPeriodObjects{period(2900, week,
		week+"/1795000000000/part-0001.csv", // completed
		week+"/1796000000000/part-0001.csv", // failed, or still running
	)}
	got, err := keepLatestCompletedSessions(periods, completedAmong(nil, "1795000000000"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{week + "/1795000000000/part-0001.csv"}; !slices.Equal(keysOf(got[0].objects), want) {
		t.Errorf("got %v, want %v", keysOf(got[0].objects), want)
	}
}

func TestLatestSessionRefusesAPeriodWithNoCompletedSession(t *testing.T) {
	periods := []lookbackPeriodObjects{period(2900, week,
		week+"/1795000000000/part-0001.csv",
		week+"/1796000000000/part-0001.csv")}
	_, err := keepLatestCompletedSessions(periods, completedAmong(nil))
	if err == nil {
		t.Fatal("want an error when no session of a period with data completed")
	}
	for _, s := range []string{week, "1796000000000, 1795000000000", "resubmit"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not name %q", err, s)
		}
	}
}

func TestLatestSessionLeavesAPeriodWithNoDataEmpty(t *testing.T) {
	marker := lookbackPeriodObjects{periodId: 2900, prefix: week,
		objects: []*awsi.S3Object{{Key: week + "/1795000000000/_DONE", Size: 0}}}
	got, err := keepLatestCompletedSessions([]lookbackPeriodObjects{period(2901, "stage/c/X/2901W"), marker},
		func([]string) (map[string]bool, error) {
			t.Error("the registry must not be read for a period with no data")
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].objects) != 0 || len(got[1].objects) != 0 {
		t.Errorf("got %v and %v, want both empty", keysOf(got[0].objects), keysOf(got[1].objects))
	}
}

func TestLatestSessionIgnoresASessionHoldingNoData(t *testing.T) {
	periods := []lookbackPeriodObjects{{periodId: 2900, prefix: week, objects: []*awsi.S3Object{
		{Key: week + "/1795000000000/part-0001.csv", Size: 10},
		{Key: week + "/1796000000000/_DONE", Size: 0}, // newer, completed, empty
	}}}
	got, err := keepLatestCompletedSessions(periods, completedAmong(nil, "1795000000000", "1796000000000"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{week + "/1795000000000/part-0001.csv"}; !slices.Equal(keysOf(got[0].objects), want) {
		t.Errorf("got %v, want %v", keysOf(got[0].objects), want)
	}
}

func TestLatestSessionRefusesAKeyWithNoSessionFolder(t *testing.T) {
	periods := []lookbackPeriodObjects{period(2900, week, week+"/part-0001.csv")}
	if _, err := keepLatestCompletedSessions(periods, completedAmong(nil)); err == nil {
		t.Error("want an error for an object directly in the period folder")
	}
}

func TestLatestSessionPassesTheRegistryError(t *testing.T) {
	periods := []lookbackPeriodObjects{period(2900, week, week+"/1795000000000/part-0001.csv")}
	boom := errors.New("boom")
	_, err := keepLatestCompletedSessions(periods, func([]string) (map[string]bool, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want %v", err, boom)
	}
}

// With a cumulative object type, the latest session of each period is kept, then
// the latest period's: one session in all, and a completed one.
func TestLatestSessionThenLatestDelivery(t *testing.T) {
	periods := []lookbackPeriodObjects{
		period(682, "stage/c/PharmacyClaim/682M",
			"stage/c/PharmacyClaim/682M/1792005309434/part-0001.csv",
			"stage/c/PharmacyClaim/682M/1792100000000/part-0001.csv"), // failed run
		period(681, "stage/c/PharmacyClaim/681M",
			"stage/c/PharmacyClaim/681M/1791000000000/part-0001.csv"),
	}
	kept, err := keepLatestCompletedSessions(periods, completedAmong(nil, "1792005309434", "1791000000000"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := latestDelivery(kept)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"stage/c/PharmacyClaim/682M/1792005309434/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
}

func TestLatestSessionOfAFolderListedDirectly(t *testing.T) {
	objects := period(0, week, week+"/1790000000000/part-0001.csv", week+"/1795000000000/part-0001.csv").objects
	// No option: unchanged.
	got, err := keepLatestCompletedSessionOfFolder(week, objects, stageReadOptions{})
	if err != nil || len(got) != 2 {
		t.Fatalf("no option: got %v, %v; want both objects", keysOf(got), err)
	}
	got, err = keepLatestCompletedSessionOfFolder(week, objects,
		stageReadOptions{completed: completedAmong(nil, "1790000000000", "1795000000000")})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{week + "/1795000000000/part-0001.csv"}; !slices.Equal(keysOf(got), want) {
		t.Errorf("got %v, want %v", keysOf(got), want)
	}
}

func TestStageReadOptionsNeedTheDatabaseForLatestSessionOnly(t *testing.T) {
	ic := &InputChannelConfig{Name: "ClaimDetails", Type: "stage", SourceObjectType: "X"}
	opts, err := stageReadOptionsOf(context.Background(), nil, ic)
	if err != nil || opts.completed != nil || opts.sourceObjectType != "X" {
		t.Errorf("without the option: got %+v, %v", opts, err)
	}
	ic.LatestSessionOnly = true
	if _, err = stageReadOptionsOf(context.Background(), nil, ic); err == nil ||
		!strings.Contains(err.Error(), "ClaimDetails") {
		t.Errorf("with the option and no database: got %v, want an error naming the channel", err)
	}
}

func TestValidateLatestSessionOnly(t *testing.T) {
	stage := InputChannelConfig{Name: "d", Type: "stage", LatestSessionOnly: true}
	stage.FileKey = "$CLIENT/D/${WEEK_PERIOD}W"
	cases := []struct {
		name string
		ic   InputChannelConfig
		ok   bool
	}{
		{"stage with file_key", stage, true},
		{"stage by read_step_id", InputChannelConfig{Name: "d", Type: "stage", ReadStepId: "s1", LatestSessionOnly: true}, false},
		{"memory", InputChannelConfig{Name: "d", Type: "memory", LatestSessionOnly: true}, false},
		{"merge channel of a stage read", InputChannelConfig{Name: "m", Type: "stage", ReadStepId: "s1",
			MergeChannels: []InputChannelConfig{{Name: "x", Type: "stage", LatestSessionOnly: true}}}, false},
		{"not set", InputChannelConfig{Name: "d", Type: "memory"}, true},
	}
	for _, c := range cases {
		cfg := &ComputePipesConfig{ReducingPipesConfig: [][]PipeSpec{{{InputChannel: c.ic}}}}
		err := validateLatestSessionOnly(cfg)
		if (err == nil) != c.ok {
			t.Errorf("%s: got %v, want ok=%v", c.name, err, c.ok)
		}
	}
}
