package compute_pipes

import (
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

// One session of a staged medical feed, as a client's bucket listed it on
// 2026-10-16: four partitions, about 26 MiB each. Before the fix each of the
// four nodes read all four files, so the session was read four times.
func stagedSession() []*awsi.S3Object {
	const s = "jetstore/stage/Client/MedicalClaim/682M/1791850097705/"
	return []*awsi.S3Object{
		{Key: s, Size: 0}, // a folder marker, which carries no partition
		{Key: s + "jets_partition=0000P/part0000-0000001.csv", Size: 27262976},
		{Key: s + "jets_partition=0001P/part0001-0000001.csv", Size: 29884416},
		{Key: s + "jets_partition=0002P/part0002-0000001.csv", Size: 29779558},
		{Key: s + "jets_partition=0003P/part0003-0000001.csv", Size: 24117248},
	}
}

func TestObjectsOfPartitionKeepsTheNodesOwnPartition(t *testing.T) {
	objects := stagedSession()
	seen := 0
	for _, label := range []string{"0000P", "0001P", "0002P", "0003P"} {
		kept, err := objectsOfPartition(objects, label)
		if err != nil {
			t.Fatal(err)
		}
		if len(kept) != 1 {
			t.Fatalf("partition %s: kept %d objects, want 1", label, len(kept))
		}
		got, _ := ExtractPartitionLabelFromS3Key(kept[0].Key)
		if got != label {
			t.Errorf("partition %s: kept %s", label, kept[0].Key)
		}
		seen += len(kept)
	}
	// Across the four nodes each file is read once, not four times.
	if seen != 4 {
		t.Errorf("the four nodes read %d files in all, want 4", seen)
	}
}

func TestObjectsOfPartitionWithoutALabelKeepsEverything(t *testing.T) {
	objects := stagedSession()
	kept, err := objectsOfPartition(objects, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kept, objects) {
		t.Errorf("an empty label kept %d of %d objects", len(kept), len(objects))
	}
}

func TestObjectsOfPartitionRefusesADataFileWithNoPartition(t *testing.T) {
	objects := []*awsi.S3Object{{Key: "jetstore/stage/Client/MedicalClaim/682M/1/part.csv", Size: 10}}
	if _, err := objectsOfPartition(objects, "0000P"); err == nil {
		t.Error("want an error for a data file outside any jets_partition")
	}
}
