package compute_pipes

import "slices"

// A frozen copy of the clustering operator's association score and merge as
// they stood at commit 6551a20e, before the 2026-09-05 revision.
//
// It lives in a _test.go file on purpose. The revised operator has to be
// measured against the one it replaces on the same input, and a baseline that
// is a live call into production code stops being a baseline the moment
// somebody edits that code. Frozen here, the comparison in
// clustering_measure_test.go means the same thing next year as it does today.
//
// Nothing outside the test binary calls any of this. The four canMerge unit
// tests that shipped with the operator now exercise v1CanMerge, which is the
// same function under a different name, so they still assert what they were
// written to assert.

// v1Score is the association score MakeClusters sorted on: the summed distinct
// count of column2 within the column1 value groups, over the summed
// observation count. Lower means more correlated.
func v1Score(cc *ColumnCorrelation) float64 {
	return float64(cc.distinct2Count) / float64(cc.observationCount)
}

func v1GetClusterOf(column string, clusters []*ClusterInfo) int {
	for i, c := range clusters {
		if c.membership[column] {
			return i
		}
	}
	return -1
}

func v1Remove(s []*ClusterInfo, i int) []*ClusterInfo {
	return slices.Delete(s, i, i+1)
}

func v1Merge(c1, c2 *ClusterInfo) *ClusterInfo {
	for k, v := range c2.clusterTags {
		c1.clusterTags[k] = v
	}
	for k, v := range c2.membership {
		c1.membership[k] = v
	}
	return c1
}

// v1CanMerge is the tag heuristic that held over-merging back in place of an
// objective.
//
// Check if cluster and rest[c2] can merge.
// Rules:
// If after the merge there is no cluster without tags, return false
// unless there were no cluster without tags before the merge.
//
// Essentially this means:
//   - if c2 is without tags, return true if there is one other
//     cluster without tags.
//   - if c2 has tags and cluster is without tags, return true if there is one other
//     cluster without tags.
func v1CanMerge(cluster *ClusterInfo, c2 int, rest []*ClusterInfo) bool {
	if len(rest[c2].clusterTags) == 0 {
		// c2 is without tags, check if there is one other without tags
		if len(cluster.clusterTags) == 0 {
			return true
		}
		for j, c := range rest {
			if j != c2 {
				if len(c.clusterTags) == 0 {
					return true
				}
			}
		}
		return false
	}
	// c2 has tags, check if cluster is without tags
	if len(cluster.clusterTags) == 0 {
		// return true if there is one other cluster without tags.
		for j, c := range rest {
			if j != c2 {
				if len(c.clusterTags) == 0 {
					return true
				}
			}
		}
		return false
	}
	return true
}

// v1MergeTrace records what the greedy merge did, so the measurement can count
// the two things the merge could not report about itself: how often the tag
// heuristic refused, and whether it abandoned the remaining pairs.
type v1MergeTrace struct {
	// PairsConsidered is how many of the input pairs the loop reached before
	// it returned.
	PairsConsidered int
	// Refusals is how many times v1CanMerge said no.
	Refusals int
	// AbandonedEarly is true when a refusal ended the loop with pairs left.
	AbandonedEarly bool
	// DoubleAssignments counts the columns that ended up a member of more than
	// one cluster.
	DoubleAssignments int
}

// v1MakeClustersWithScore is MakeClusters as it stood at 6551a20e, plus a
// trace and one deviation: the sort comparator calls scoreOf rather than
// computing the ratio inline. Everything else is the original, character for
// character.
//
// The deviation is what makes the comparison separable. Two changes were made
// to this operator -- the score and the merge -- and a comparison that can
// only run them together cannot say which one moved a number. Passing v1Score
// gives the original behaviour exactly.
func v1MakeClustersWithScore(columnsCorrelation []*ColumnCorrelation,
	columnClassificationMap map[string]string, config *ClusteringSpec,
	scoreOf func(*ColumnCorrelation) float64) ([]*ClusterInfo, v1MergeTrace) {

	trace := v1MergeTrace{}
	// Sort the columnsCorrelation result, in decreasing value of probability the columns are correlated
	slices.SortFunc(columnsCorrelation, func(a, b *ColumnCorrelation) int {
		valueA := scoreOf(a)
		valueB := scoreOf(b)
		switch {
		case valueA < valueB:
			return -1
		case valueA > valueB:
			return 1
		default:
			return 0
		}
	})
	// Determine the clusters
	// make a lookup of the columns that have a transitive data classification
	transitiveDC := make(map[string]bool)
	for _, dc := range config.TransitiveDataClassification {
		for column, tag := range columnClassificationMap {
			if tag == dc {
				transitiveDC[column] = true
			}
		}
		transitiveDC[dc] = true
	}
	// make the clusters
	clusters := make([]*ClusterInfo, 0)
	var cluster *ClusterInfo
	count := len(columnsCorrelation)
	for i, cc := range columnsCorrelation {
		trace.PairsConsidered = i + 1
		c1 := v1GetClusterOf(cc.column1, clusters)
		if c1 < 0 {
			cluster = NewClusterInfo(columnClassificationMap, config)
			cluster.AddMember(cc.column1)
		} else {
			cluster = clusters[c1]
			clusters = v1Remove(clusters, c1)
		}

		c2 := v1GetClusterOf(cc.column2, clusters)
		if c2 < 0 || !transitiveDC[cc.column2] {
			// column2 is not yet in a cluster, put it in the current cluster
			cluster.AddMember(cc.column2)
		} else {
			// Merge c2 into cluster, check if this will breakdown the clusters structure
			if i*10 < count || v1CanMerge(cluster, c2, clusters) {
				cluster = v1Merge(cluster, clusters[c2])
				// Remove c2 from clusters
				clusters = v1Remove(clusters, c2)
			} else {
				trace.Refusals++
				// cluster structure complete
				clusters = append(clusters, cluster)
				trace.AbandonedEarly = i+1 < count
				trace.DoubleAssignments = v1CountDoubleAssignments(clusters)
				return clusters, trace
			}
		}
		// Add cluster into the set of clusters
		clusters = append(clusters, cluster)
	}
	trace.DoubleAssignments = v1CountDoubleAssignments(clusters)
	return clusters, trace
}

// v1CountDoubleAssignments counts the columns that are a member of more than
// one cluster. The greedy merge can produce them: when column2 already belongs
// to a cluster but is not transitively classified, the branch that adds it to
// the current cluster runs without removing it from the one it is in.
func v1CountDoubleAssignments(clusters []*ClusterInfo) int {
	seen := make(map[string]int)
	for _, c := range clusters {
		for column := range c.membership {
			seen[column]++
		}
	}
	n := 0
	for _, c := range seen {
		if c > 1 {
			n++
		}
	}
	return n
}
