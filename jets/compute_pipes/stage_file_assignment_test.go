package compute_pipes

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
)

const mib = 1024 * 1024

// stagedFeed is a staged feed over a lookback: three sessions that used four, one
// and two partitions, as mapping sessions of different sizes do.
func stagedFeed() []*awsi.S3Object {
	const p = "jetstore/stage/Client/PharmacyClaim/"
	return []*awsi.S3Object{
		{Key: p + "682M/1791850097705/", Size: 0}, // a folder marker
		{Key: p + "682M/1791850097705/jets_partition=0000P/part0000-0000001.csv", Size: 26 * mib},
		{Key: p + "682M/1791850097705/jets_partition=0001P/part0001-0000001.csv", Size: 28 * mib},
		{Key: p + "682M/1791850097705/jets_partition=0002P/part0002-0000001.csv", Size: 28 * mib},
		{Key: p + "682M/1791850097705/jets_partition=0003P/part0003-0000001.csv", Size: 23 * mib},
		{Key: p + "681M/1790614235917/jets_partition=0000P/part0000-0000001.csv", Size: 31 * mib},
		{Key: p + "680M/1789490789165/jets_partition=0000P/part0000-0000001.csv", Size: 9 * mib},
		{Key: p + "680M/1789490789165/jets_partition=0001P/part0001-0000001.csv", Size: 8 * mib},
	}
}

func assignedKeys(assignment []StageFileKey, channel int) []string {
	var keys []string
	for _, f := range assignment {
		if f.Channel == channel {
			keys = append(keys, f.Key)
		}
	}
	slices.Sort(keys)
	return keys
}

// nodeLoads returns each node's number of files and bytes.
func nodeLoads(assignment []StageFileKey, nbrNodes int) ([]int, []int64) {
	files := make([]int, nbrNodes)
	loads := make([]int64, nbrNodes)
	for _, f := range assignment {
		files[f.Node]++
		loads[f.Node] += f.Size
	}
	return files, loads
}

func dataKeys(objects []*awsi.S3Object) []string {
	var keys []string
	for _, obj := range objects {
		if obj.Size > 0 {
			keys = append(keys, obj.Key)
		}
	}
	slices.Sort(keys)
	return keys
}

func TestAssignStageFilesReadsEveryFileOnce(t *testing.T) {
	feed := stagedFeed()
	for nbrNodes := 1; nbrNodes <= 7; nbrNodes++ {
		assignment := assignStageFiles([][]*awsi.S3Object{feed}, nbrNodes)
		if got, want := assignedKeys(assignment, 0), dataKeys(feed); !slices.Equal(got, want) {
			t.Errorf("%d nodes: read %v, want every data file once: %v", nbrNodes, got, want)
		}
		files, _ := nodeLoads(assignment, nbrNodes)
		for n := range files {
			if files[n] == 0 {
				t.Errorf("%d nodes: node %d was assigned no file", nbrNodes, n)
			}
		}
	}
}

func TestAssignStageFilesBalancesTheNodes(t *testing.T) {
	_, loads := nodeLoads(assignStageFiles([][]*awsi.S3Object{stagedFeed()}, 3), 3)
	// 153 MiB over three nodes: no node more than the largest file above another.
	if spread := slices.Max(loads) - slices.Min(loads); spread > 31*mib {
		t.Errorf("loads %v spread by %d bytes", loads, spread)
	}
}

func TestAssignStageFilesIsDeterministic(t *testing.T) {
	feed := stagedFeed()
	reversed := slices.Clone(feed)
	slices.Reverse(reversed)
	a := assignStageFiles([][]*awsi.S3Object{feed}, 3)
	b := assignStageFiles([][]*awsi.S3Object{reversed}, 3)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("the listing order changed the assignment:\n%v\n%v", a, b)
	}
}

// The defect this replaces: one node per label of the main input, and each node
// reading its own label in every channel. A merge channel whose files carry labels
// the main input does not was read in part.
func TestAssignStageFilesReadsAMergeChannelWhoseLabelsTheMainInputLacks(t *testing.T) {
	const p = "jetstore/stage/Client/"
	main := []*awsi.S3Object{
		{Key: p + "Eligibility/682M/1/jets_partition=0000P/part0000-0000001.csv", Size: 5 * mib},
	}
	merge := []*awsi.S3Object{
		{Key: p + "MedicalClaim/682M/2/jets_partition=0000P/part0000-0000001.csv", Size: 26 * mib},
		{Key: p + "MedicalClaim/682M/2/jets_partition=0001P/part0001-0000001.csv", Size: 28 * mib},
		{Key: p + "MedicalClaim/682M/2/jets_partition=0002P/part0002-0000001.csv", Size: 28 * mib},
		{Key: p + "MedicalClaim/682M/2/jets_partition=0003P/part0003-0000001.csv", Size: 23 * mib},
	}

	// Before: the main input's one label gave one node, which kept one merge file.
	kept, err := objectsOfPartition(merge, "0000P")
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 {
		t.Fatalf("the label filter kept %d merge files, the defect is one", len(kept))
	}

	// Now: every merge file is assigned to a node.
	assignment := assignStageFiles([][]*awsi.S3Object{main, merge}, 1)
	if got, want := assignedKeys(assignment, 1), dataKeys(merge); !slices.Equal(got, want) {
		t.Errorf("merge channel read %v, want %v", got, want)
	}
}

func TestNbrStageNodes(t *testing.T) {
	cluster := &ClusterSpec{DefaultShardSizeMb: 10, DefaultShardMaxSizeMb: 15}
	feed := stagedFeed()
	cases := []struct {
		name          string
		objects       []*awsi.S3Object
		cluster       *ClusterSpec
		singleNode    bool
		fallbackNodes int
		want          int
	}{
		// 153 MiB at 10 MiB a shard is 16 nodes, and there are 7 files, none split.
		{"as many nodes as files", feed, cluster, false, 2, 7},
		{"by shard size", feed, &ClusterSpec{DefaultShardSizeMb: 60}, false, 2, 3},
		{"a merge_files step", feed, cluster, true, 2, 1},
		{"no shard size: this session's partitions", feed, &ClusterSpec{}, false, 2, 2},
		{"no cluster config", feed, nil, false, 0, 1},
		{"nothing staged", []*awsi.S3Object{{Key: "x/", Size: 0}}, cluster, false, 2, 0},
	}
	for _, c := range cases {
		if got := nbrStageNodes(c.objects, "csv", c.cluster, c.singleNode, c.fallbackNodes); got != c.want {
			t.Errorf("%s: %d nodes, want %d", c.name, got, c.want)
		}
	}
}

func TestStageFileKeysOfNode(t *testing.T) {
	assignment := assignStageFiles([][]*awsi.S3Object{stagedFeed(), {}}, 2)
	var all []string
	for n := range 2 {
		fileKeys, err := stageFileKeysOfNode(assignment, n, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(fileKeys) != 2 {
			t.Fatalf("node %d: %d channels", n, len(fileKeys))
		}
		for _, k := range fileKeys[0] {
			if k.size <= 0 {
				t.Errorf("node %d: %s without its size", n, k.key)
			}
			all = append(all, k.key)
		}
	}
	slices.Sort(all)
	if want := dataKeys(stagedFeed()); !slices.Equal(all, want) {
		t.Errorf("the nodes read %v, want %v", all, want)
	}
	if _, err := stageFileKeysOfNode(assignment, 2, 2); err == nil {
		t.Error("want an error for a node the starter did not assign")
	}
	merged := assignStageFiles([][]*awsi.S3Object{stagedFeed(), stagedFeed()}, 2)
	if _, err := stageFileKeysOfNode(merged, 0, 1); err == nil {
		t.Error("want an error when the step reads fewer channels than were assigned")
	}
}

// A node knows its files only from the reducing config the starter stores, so the
// assignment must survive the config's JSON round trip.
func TestStageFileKeysReachAWorkerNode(t *testing.T) {
	assignment := assignStageFiles([][]*awsi.S3Object{stagedFeed()}, 3)
	config := ComputePipesConfig{CommonRuntimeArgs: &ComputePipesCommonArgs{StageFileKeys: assignment}}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var node ComputePipesConfig
	if err := json.Unmarshal(b, &node); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(node.CommonRuntimeArgs.StageFileKeys, assignment) {
		t.Errorf("the assignment did not survive the config: %s", fmt.Sprint(node.CommonRuntimeArgs.StageFileKeys))
	}
}

func TestReadsStageByFileKey(t *testing.T) {
	cases := []struct {
		channel InputChannelConfig
		want    bool
	}{
		{InputChannelConfig{Type: "stage", FileConfig: FileConfig{FileKey: "$CLIENT/PharmacyClaim/${PERIOD_ID}M"}}, true},
		{InputChannelConfig{Type: "stage", ReadStepId: "PharmacyClaim.sharded"}, false},
		{InputChannelConfig{Type: "generator"}, false},
	}
	for _, c := range cases {
		if got := readsStageByFileKey(&c.channel); got != c.want {
			t.Errorf("%+v: %v, want %v", c.channel, got, c.want)
		}
	}
}
