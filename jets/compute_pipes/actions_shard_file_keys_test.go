package compute_pipes

import (
	"fmt"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

func TestAssignShardInfo10(t *testing.T) {

	// func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize int64,
	// 	doSplitFiles bool, sessionId string) ([][]any, int) {

	rows, nShards := assignShardInfo(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 100},
		}, 20, 35, 0, true, "012345", 0)

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 5 {
		t.Errorf("error, expecting 5 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(100), int64(0), int64(20), 0, 0},
		{"012345", "file_key1", int64(100), int64(21), int64(41), 1, 0},
		{"012345", "file_key1", int64(100), int64(42), int64(62), 2, 0},
		{"012345", "file_key1", int64(100), int64(63), int64(83), 3, 0},
		{"012345", "file_key1", int64(100), int64(84), int64(100), 4, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfoP10(t *testing.T) {

	rows, nShards := assignShardInfoParquet(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 25},
		}, 20, 35, true, "012345", 0)

	if len(rows) != 1 {
		t.Errorf("Expecting 1 row, got %d", len(rows))
	}
	if nShards != 1 {
		t.Errorf("error, expecting 1 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(0), int64(0), int64(0), 0, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfoP11(t *testing.T) {

	rows, nShards := assignShardInfoParquet(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 100},
		}, 75, 85, true, "012345", 0)

	if len(rows) != 2 {
		t.Errorf("Expecting 2 rows, got %d", len(rows))
	}
	if nShards != 2 {
		t.Errorf("error, expecting 2 shards, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(100), int64(0), int64(2), 0, 0},
		{"012345", "file_key1", int64(100), int64(1), int64(2), 1, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfoP12(t *testing.T) {

	rows, nShards := assignShardInfoParquet(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 100},
		}, 20, 35, true, "012345", 0)

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 5 {
		t.Errorf("error, expecting 5 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(100), int64(0), int64(5), 0, 0},
		{"012345", "file_key1", int64(100), int64(1), int64(5), 1, 0},
		{"012345", "file_key1", int64(100), int64(2), int64(5), 2, 0},
		{"012345", "file_key1", int64(100), int64(3), int64(5), 3, 0},
		{"012345", "file_key1", int64(100), int64(4), int64(5), 4, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfoP13(t *testing.T) {

	rows, nShards := assignShardInfoParquet(
		[]*awsi.S3Object{
			{Key: "file_key40", Size: 40},
			{Key: "file_key10", Size: 10},
			{Key: "file_key30", Size: 30},
			{Key: "file_key11", Size: 11},
		}, 20, 35, true, "012345", 0)

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 4 {
		t.Errorf("error, expecting 4 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key10", int64(10), int64(0), int64(0), 0, 0},
		{"012345", "file_key11", int64(11), int64(0), int64(0), 0, 0},
		{"012345", "file_key30", int64(30), int64(0), int64(0), 1, 0},
		{"012345", "file_key40", int64(40), int64(0), int64(2), 2, 0},
		{"012345", "file_key40", int64(40), int64(1), int64(2), 3, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfo11(t *testing.T) {
  // shard_offset = 10
	rows, nShards := assignShardInfo(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 100},
		}, 20, 35, 10, true, "012345", 0)

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 5 {
		t.Errorf("error, expecting 5 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(100), int64(0), int64(20), 0, 0},
		{"012345", "file_key1", int64(100), int64(11), int64(41), 1, 0},
		{"012345", "file_key1", int64(100), int64(32), int64(62), 2, 0},
		{"012345", "file_key1", int64(100), int64(53), int64(83), 3, 0},
		{"012345", "file_key1", int64(100), int64(74), int64(100), 4, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfo2(t *testing.T) {

	// func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize int64,
	// 	doSplitFiles bool, sessionId string) ([][]any, int) {

	rows, nShards := assignShardInfo(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 20},
			{Key: "file_key2", Size: 20},
			{Key: "file_key3", Size: 20},
		}, 100, 135, 10, true, "012345", 0)

	if len(rows) != 3 {
		t.Errorf("Expecting 3 row, got %d", len(rows))
	}
	if nShards != 1 {
		t.Errorf("error, expecting 1 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(20), int64(0), int64(0), 0, 0},
		{"012345", "file_key2", int64(20), int64(0), int64(0), 0, 0},
		{"012345", "file_key3", int64(20), int64(0), int64(0), 0, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfo3(t *testing.T) {

	// func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize int64,
	// 	doSplitFiles bool, sessionId string) ([][]any, int) {

	rows, nShards := assignShardInfo(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 10},
			{Key: "file_key2", Size: 20},
			{Key: "file_key3", Size: 20},
		}, 20, 35, 0, true, "012345", 0)

	if len(rows) != 3 {
		t.Errorf("Expecting 3 row, got %d", len(rows))
	}
	if nShards != 2 {
		t.Errorf("error, expecting 2 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(10), int64(0), int64(0), 0, 0},
		{"012345", "file_key2", int64(20), int64(0), int64(0), 0, 0},
		{"012345", "file_key3", int64(20), int64(0), int64(0), 1, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

func TestAssignShardInfo4(t *testing.T) {

	// func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize int64,
	// 	doSplitFiles bool, sessionId string) ([][]any, int) {

	rows, nShards := assignShardInfo(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 10},
			{Key: "file_key2", Size: 50},
			{Key: "file_key3", Size: 20},
		}, 20, 35, 0, true, "012345", 0)

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 4 {
		t.Errorf("error, expecting 4 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(10), int64(0), int64(0), 0, 0},
		{"012345", "file_key2", int64(50), int64(0), int64(10), 0, 0},
		{"012345", "file_key2", int64(50), int64(11), int64(31), 1, 0},
		{"012345", "file_key2", int64(50), int64(32), int64(50), 2, 0},
		{"012345", "file_key3", int64(20), int64(0), int64(0), 3, 0},
	}

	for i, row := range rows {
		fmt.Printf("Got row[%d]: %v\n", i, row)
		for j, elm := range row {
			if expected[i][j] != elm {
				t.Errorf("error at (%d, %d), expecting %v got %v", i, j, expected[i][j], elm)
			}
		}
	}
}

// keysOf returns the object keys of s3Objects, for comparing prune results.
func keysOf(s3Objects []*awsi.S3Object) []string {
	keys := make([]string, 0, len(s3Objects))
	for _, obj := range s3Objects {
		keys = append(keys, obj.Key)
	}
	return keys
}

func TestPruneIgnoredFileKeys(t *testing.T) {

	// func pruneIgnoredFileKeys(s3Objects []*awsi.S3Object, ignore string) []*awsi.S3Object

	sentinel := "client/period/_COMPLETE"
	property := "client/period/test_harness_filters.txt"
	part1 := "client/period/claims_part1.csv"
	part2 := "client/period/claims_part2.csv"

	folder := []*awsi.S3Object{
		{Key: sentinel, Size: 0},
		{Key: property, Size: 120},
		{Key: part1, Size: 4000},
		{Key: part2, Size: 5000},
	}

	tests := []struct {
		name     string
		ignore   string
		wantKeys []string
	}{
		{
			name:     "absent, ie the zero value of an env var that is not set",
			ignore:   "",
			wantKeys: []string{sentinel, property, part1, part2},
		},
		{
			name:     "one entry, the case this was built for",
			ignore:   "test_harness_filters.txt",
			wantKeys: []string{sentinel, part1, part2},
		},
		{
			name:     "several entries",
			ignore:   "test_harness_filters.txt|_COMPLETE|claims_part2.csv",
			wantKeys: []string{part1},
		},
		{
			name:     "an entry matching nothing",
			ignore:   "no_such_file.txt",
			wantKeys: []string{sentinel, property, part1, part2},
		},
		{
			name:     "entries matching every object",
			ignore:   "_COMPLETE|test_harness_filters.txt|claims_part1.csv|claims_part2.csv",
			wantKeys: []string{},
		},
		{
			name:     "a folder prefix matches nothing: the match is a suffix, not a prefix",
			ignore:   "client/period/",
			wantKeys: []string{sentinel, property, part1, part2},
		},
		{
			name:     "a path-bearing entry, matched against the whole key",
			ignore:   "period/test_harness_filters.txt",
			wantKeys: []string{sentinel, part1, part2},
		},
		{
			name:     "a path-bearing entry whose path does not match prunes nothing",
			ignore:   "other/test_harness_filters.txt",
			wantKeys: []string{sentinel, property, part1, part2},
		},
		{
			name:     "an empty entry prunes nothing, it does not match every key",
			ignore:   "test_harness_filters.txt||",
			wantKeys: []string{sentinel, part1, part2},
		},
		{
			name:     "only empty entries, same as absent",
			ignore:   "|",
			wantKeys: []string{sentinel, property, part1, part2},
		},
		{
			name:     "the suffix carries no path boundary, so .csv prunes every data file",
			ignore:   ".csv",
			wantKeys: []string{sentinel, property},
		},
	}

	for _, test := range tests {
		got := keysOf(pruneIgnoredFileKeys(folder, test.ignore))
		if !slices.Equal(got, test.wantKeys) {
			t.Errorf("%s: with ignore %q, expecting %v, got %v", test.name, test.ignore,
				test.wantKeys, got)
		}
	}

	// A nil slice is not a special case
	if got := pruneIgnoredFileKeys(nil, "test_harness_filters.txt"); len(got) != 0 {
		t.Errorf("error: expecting an empty result for a nil input, got %v", keysOf(got))
	}
}

// TestPruneIgnoredFileKeysEmptyFolder covers pruning that empties the input folder.
// ShardFileKeys calls S3 (actions_shard_file_keys.go:53) and has no unit test, so this
// asserts the helper's output against the two guards that reach
// "error: input folder contains no data files" rather than calling ShardFileKeys: the
// len(s3Objects) == 0 guard at actions_shard_file_keys.go:85, and the TotalFileSize == 0
// guard at :95, which is the one a folder still holding the 0-byte sentinel reaches.
func TestPruneIgnoredFileKeysEmptyFolder(t *testing.T) {

	// Everything pruned: len(s3Objects) is 0, which is the guard at :85
	folder := []*awsi.S3Object{
		{Key: "client/period/test_harness_filters.txt", Size: 120},
	}
	pruned := pruneIgnoredFileKeys(folder, "test_harness_filters.txt")
	if len(pruned) != 0 {
		t.Errorf("error: expecting an empty folder, so that the guard at :85 is reached, got %v",
			keysOf(pruned))
	}

	// The sentinel survives the prune, being neither the ignored name nor a suffix of it,
	// so the guard at :85 is not reached and the one at :95 is: the accumulation at :93
	// sums to 0 because the sentinel is a 0-byte file.
	folder = []*awsi.S3Object{
		{Key: "client/period/_COMPLETE", Size: 0},
		{Key: "client/period/test_harness_filters.txt", Size: 120},
	}
	pruned = pruneIgnoredFileKeys(folder, "test_harness_filters.txt")
	if len(pruned) != 1 {
		t.Errorf("error: expecting the sentinel to survive the prune, got %v", keysOf(pruned))
	}
	var totalFileSize int64
	for _, obj := range pruned {
		totalFileSize += obj.Size
	}
	if totalFileSize != 0 {
		t.Errorf("error: expecting a total file size of 0, so that the guard at :95 is reached, got %d",
			totalFileSize)
	}
}
