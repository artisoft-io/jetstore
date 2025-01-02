package compute_pipes

import (
	"testing"
)

func TestClusterInfo01(t *testing.T) {
	// 2 clusters without tags should be able to merge
	cluster := NewClusterInfo(nil, nil)
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil)}
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
	// Only one cluster without tags should not be able to merge with cluster
	// having tags
	cluster.clusterTags["tag"] = true
	if canMerge(cluster, 0, clusters) {
		t.Errorf("NOT expecting to be able to merge")
	}
	// 2 clusters with tags should be able to merge
	clusters[0].clusterTags["tag"] = true
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
}

func TestClusterInfo02(t *testing.T) {
	// 2 clusters with tags, with another one without tags should be able to merge
	cluster := NewClusterInfo(nil, nil)
	cluster.clusterTags["tag"] = true
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil), NewClusterInfo(nil, nil)}
	clusters[0].clusterTags["tag"] = true
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
}

func TestClusterInfo03(t *testing.T) {
	// 2 clusters 1 with tags and 1 without, and another one without tags should be able to merge
	cluster := NewClusterInfo(nil, nil)
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil), NewClusterInfo(nil, nil)}
	clusters[0].clusterTags["tag"] = true
	if !canMerge(cluster, 0, clusters) {
		t.Errorf("Expecting to be able to merge")
	}
}

func TestClusterInfo04(t *testing.T) {
	// 2 clusters 1 with tags and 1 without, and another one with tags should not be able to merge
	cluster := NewClusterInfo(nil, nil)
	clusters := []*ClusterInfo{NewClusterInfo(nil, nil), NewClusterInfo(nil, nil)}
	clusters[0].clusterTags["tag"] = true
	clusters[1].clusterTags["tag"] = true
	if canMerge(cluster, 0, clusters) {
		t.Errorf("NOT expecting to be able to merge")
	}
}
