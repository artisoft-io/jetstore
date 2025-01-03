package compute_pipes

import (
	"slices"
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

func getClusterOf(column string, clusters []*ClusterInfo) int {
	for i, c := range clusters {
		if c.membership[column] {
			return i
		}
	}
	return -1
}

func remove(s []*ClusterInfo, i int) []*ClusterInfo {
	return slices.Delete(s, i, i+1)
}

func merge(c1, c2 *ClusterInfo) *ClusterInfo {
	for k, v := range c2.clusterTags {
		c1.clusterTags[k] = v
	}
	for k, v := range c2.membership {
		c1.membership[k] = v
	}
	return c1
}

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
func canMerge(cluster *ClusterInfo, c2 int, rest []*ClusterInfo) bool {
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

// Function that build the clusters from the raw column correlation
func MakeClusters(columnsCorrelation []*ColumnCorrelation,
	columnClassificationMap map[string]string, config *ClusteringSpec) []*ClusterInfo {

	// Sort the columnsCorrelation result, in decreasing value of probability the columns are correlated
	slices.SortFunc(columnsCorrelation, func(a, b *ColumnCorrelation) int {
		valueA := float64(a.distinct2Count) / float64(a.observationCount)
		valueB := float64(b.distinct2Count) / float64(b.observationCount)
		switch {
		case valueA < valueB:
			return -1
		case valueA > valueB:
			return 1
		default:
			return 0
		}
	})
	// //***
	// for _, cc := range columnsCorrelation {
	//   log.Printf("SORTED COLUMN CORRELATION: %s -> %s: (%v, %v, %v)\n",
	//   cc.column1, cc.column2, cc.distinct1Count, cc.distinct2Count, cc.observationCount)
	// }
	// //***
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
		// log.Printf("Considering (%s, %s)\n", cc.column1, cc.column2)
		c1 := getClusterOf(cc.column1, clusters)
		if c1 < 0 {
			cluster = NewClusterInfo(columnClassificationMap, config)
			cluster.AddMember(cc.column1)
		} else {
			cluster = clusters[c1]
			clusters = remove(clusters, c1)
		}

		c2 := getClusterOf(cc.column2, clusters)
		if c2 < 0 || !transitiveDC[cc.column2] {
			// column2 is not yet in a cluster, put it in the current cluster
			cluster.AddMember(cc.column2)
		} else {
			// Merge c2 into cluster, check if this will breakdown the clusters structure
			if i*10 < count || canMerge(cluster, c2, clusters) {
				cluster = merge(cluster, clusters[c2])
				// Remove c2 from clusters
				clusters = remove(clusters, c2)
			} else {
				// log.Printf("Cannot merge %s with %s\n", cluster, clusters[c2])
				// cluster structure complete
				//*TODO may continue for unseen columns
				// Add cluster into the set of clusters
				clusters = append(clusters, cluster)
				return clusters
			}
		}
		// Add cluster into the set of clusters
		clusters = append(clusters, cluster)
	}
	return clusters
}
