package compute_pipes

import "strings"

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
	buf.WriteString("members: ")
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
	s[len(s)-1], s[i] = nil, s[len(s)-1]
	return s[:len(s)-1]
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
//	- if c2 is without tags, return true if there is one other
//    cluster without tags.
//  - if c2 has tags and cluster is without tags, return true if there is one other
//    cluster without tags.
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