package compute_pipes

import (
	"fmt"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

func TestAssignShardInfo10(t *testing.T) {

	// func assignShardInfo(s3Objects []*awsi.S3Object, shardSize, maxShardSize int64,
	// 	doSplitFiles bool, sessionId string) ([][]any, int) {

	rows, nShards := assignShardInfo(
		[]*awsi.S3Object{
			{Key: "file_key1", Size: 100},
		}, 20, 35, 0, true, "012345")

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 5 {
		t.Errorf("error, expecting 5 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(100), int64(0), int64(20), 0},
		{"012345", "file_key1", int64(100), int64(21), int64(41), 1},
		{"012345", "file_key1", int64(100), int64(42), int64(62), 2},
		{"012345", "file_key1", int64(100), int64(63), int64(83), 3},
		{"012345", "file_key1", int64(100), int64(84), int64(100), 4},
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
		}, 20, 35, 10, true, "012345")

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 5 {
		t.Errorf("error, expecting 5 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(100), int64(0), int64(20), 0},
		{"012345", "file_key1", int64(100), int64(11), int64(41), 1},
		{"012345", "file_key1", int64(100), int64(32), int64(62), 2},
		{"012345", "file_key1", int64(100), int64(53), int64(83), 3},
		{"012345", "file_key1", int64(100), int64(74), int64(100), 4},
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
		}, 100, 135, 10, true, "012345")

	if len(rows) != 3 {
		t.Errorf("Expecting 3 row, got %d", len(rows))
	}
	if nShards != 1 {
		t.Errorf("error, expecting 1 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(20), int64(0), int64(0), 0},
		{"012345", "file_key2", int64(20), int64(0), int64(0), 0},
		{"012345", "file_key3", int64(20), int64(0), int64(0), 0},
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
		}, 20, 35, 0, true, "012345")

	if len(rows) != 3 {
		t.Errorf("Expecting 3 row, got %d", len(rows))
	}
	if nShards != 2 {
		t.Errorf("error, expecting 2 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(10), int64(0), int64(0), 0},
		{"012345", "file_key2", int64(20), int64(0), int64(0), 0},
		{"012345", "file_key3", int64(20), int64(0), int64(0), 1},
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
		}, 20, 35, 0, true, "012345")

	if len(rows) != 5 {
		t.Errorf("Expecting 5 row, got %d", len(rows))
	}
	if nShards != 4 {
		t.Errorf("error, expecting 4 shard, got %d", nShards)
	}
	expected := [][]any{
		{"012345", "file_key1", int64(10), int64(0), int64(0), 0},
		{"012345", "file_key2", int64(50), int64(0), int64(10), 0},
		{"012345", "file_key2", int64(50), int64(11), int64(31), 1},
		{"012345", "file_key2", int64(50), int64(32), int64(50), 2},
		{"012345", "file_key3", int64(20), int64(0), int64(0), 3},
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
