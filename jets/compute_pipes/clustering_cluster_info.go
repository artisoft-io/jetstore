package compute_pipes

import (
	"sort"
	"strings"
)

// Utility types and functions for clustering operator

type ClusterInfo struct {
	clusterTags             map[string]bool
	membership              map[string]bool
	columnClassificationMap map[string]string
	config                  *ClusteringSpec
}

func NewClusterInfo(classificationMap map[string]string, config *ClusteringSpec) *ClusterInfo {
	return &ClusterInfo{
		clusterTags:             make(map[string]bool),
		membership:              make(map[string]bool),
		columnClassificationMap: classificationMap,
		config:                  config,
	}
}

func (cc *ClusterInfo) AddMember(column string) {
	for _, tag := range cc.config.ClusterDataSubclassification {
		if cc.columnClassificationMap[column] == tag {
			cc.clusterTags[tag] = true
		}
	}
	cc.membership[column] = true
}

func (cc *ClusterInfo) String() string {
	var buf strings.Builder
	buf.WriteString("Cluster Info, tags: ")
	for tag := range cc.clusterTags {
		buf.WriteString(tag)
		buf.WriteString(", ")
	}
	buf.WriteString("membership: ")
	for member := range cc.membership {
		buf.WriteString(member)
		buf.WriteString(", ")
	}
	return buf.String()
}

// edgeWeightFunc yields the undirected association weight of one measured
// column pair. It is a parameter rather than a constant so that the
// measurement in clustering_measure_test.go can hold the partition fixed and
// vary the score, which is what grading the two changes separately requires.
type edgeWeightFunc func(*ColumnCorrelation) float64

// normalisedDependencyWeight is the production weight: the normalised
// association measure of clustering_association.go. A pair measured in both
// directions has both its measurements averaged into one edge below, which is
// where the directional measure stops being used as though it were symmetric.
func normalisedDependencyWeight(cc *ColumnCorrelation) float64 {
	return cc.scores.NormalisedDependency
}

// MakeClusters builds the clusters from the raw column correlation.
//
// Revised 2026-09-05. What it did before: sort the pairs ascending by
// distinct2Count/observationCount and merge them agglomeratively in that
// order, held back from over-merging by a tag heuristic -- canMerge permitted
// a merge only if some cluster without tags survived it -- and abandoning
// every remaining pair the moment that heuristic refused once. Four defects
// were diagnosed in that:
//
//  1. the score is an unnormalised proxy for functional dependency;
//  2. functional dependence is directional and the score was used as though
//     symmetric;
//  3. greedy merging in score order is single-linkage, and single-linkage
//     chains;
//  4. over-merging was held back by a tag heuristic rather than by an
//     objective.
//
// 1 and 2 are fixed in clustering_association.go, which normalises the ratio
// against what column2's own distribution predicts and computes both
// directions. 3 is fixed in clustering_partition.go, which replaces the greedy
// merge with a modularity partition of the weighted graph. 4 is fixed here, by
// deleting the heuristic: with a global objective deciding the merges there is
// nothing for it to hold back, and the tags keep only the job they were
// introduced for, which is labelling a cluster's subclassification through
// AddMember.
//
// Note that config.TransitiveDataClassification is no longer read. It gated
// which columns could bridge two clusters under the greedy merge, and a
// partition has no bridging step to gate.
func MakeClusters(columnsCorrelation []*ColumnCorrelation,
	columnClassificationMap map[string]string, config *ClusteringSpec) []*ClusterInfo {
	return makeClustersWithWeights(columnsCorrelation, columnClassificationMap, config,
		normalisedDependencyWeight)
}

// makeClustersWithWeights is MakeClusters with the edge weight injected.
func makeClustersWithWeights(columnsCorrelation []*ColumnCorrelation,
	columnClassificationMap map[string]string, config *ClusteringSpec,
	weightOf edgeWeightFunc) []*ClusterInfo {

	// Index the columns, in order of first appearance, so that the partition's
	// node ids are stable for a given input ordering.
	index := make(map[string]int)
	names := make([]string, 0)
	addColumn := func(c string) {
		if _, ok := index[c]; !ok {
			index[c] = len(names)
			names = append(names, c)
		}
	}
	for _, cc := range columnsCorrelation {
		addColumn(cc.column1)
		addColumn(cc.column2)
	}
	if len(names) == 0 {
		return make([]*ClusterInfo, 0)
	}

	// Build the undirected graph. A pair measured in both directions
	// contributes both measurements and the edge takes their mean, so the
	// graph is symmetric by construction rather than by whichever direction
	// happened to be sorted on -- which is diagnosis 2.
	sum := make(map[[2]int]float64)
	count := make(map[[2]int]int)
	for _, cc := range columnsCorrelation {
		i, j := index[cc.column1], index[cc.column2]
		if i == j {
			continue
		}
		if i > j {
			i, j = j, i
		}
		key := [2]int{i, j}
		sum[key] += weightOf(cc)
		count[key]++
	}
	keys := make([][2]int, 0, len(sum))
	for k := range sum {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(x, y int) bool {
		if keys[x][0] != keys[y][0] {
			return keys[x][0] < keys[y][0]
		}
		return keys[x][1] < keys[y][1]
	})
	edges := make([]weightedEdge, 0, len(keys))
	for _, k := range keys {
		w := sum[k] / float64(count[k])
		if w > 0 {
			edges = append(edges, weightedEdge{i: k[0], j: k[1], w: w})
		}
	}

	labels := louvainPartition(len(names), edges, clusteringModularityResolution)

	// Materialise the clusters. Every column that appears in any measured pair
	// lands in exactly one cluster -- the greedy merge could leave a column in
	// two, and could abandon the remaining columns altogether.
	byLabel := make(map[int][]string)
	for i, name := range names {
		byLabel[labels[i]] = append(byLabel[labels[i]], name)
	}
	labelList := make([]int, 0, len(byLabel))
	for l := range byLabel {
		labelList = append(labelList, l)
	}
	// Order the clusters by their alphabetically first member, so that the
	// cluster0, cluster1, ... identifiers the pool manager emits do not depend
	// on map iteration order.
	for _, l := range labelList {
		sort.Strings(byLabel[l])
	}
	sort.Slice(labelList, func(x, y int) bool {
		return byLabel[labelList[x]][0] < byLabel[labelList[y]][0]
	})
	clusters := make([]*ClusterInfo, 0, len(labelList))
	for _, l := range labelList {
		cluster := NewClusterInfo(columnClassificationMap, config)
		for _, name := range byLabel[l] {
			cluster.AddMember(name)
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}
