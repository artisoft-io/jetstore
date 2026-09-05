package compute_pipes

// Component to determine correlation between columns based on column cardinality
// Nbr of observation is the number of distinct values of column1 that was observed.

type ClusterCorrelation struct {
	column1              string
	column2              string
	minObservationsCount int
	distinctValuesCount  int
	observationsCount    int
	// stats carries the additive entropy sufficient statistics of the pair.
	// They are what the normalised, symmetric association measure is computed
	// from; the two counts above remain because they are what the correlation
	// output channel has always reported.
	stats columnEntropyStats
}

func NewClusterCorrelation(c1, c2 string, minObservationsCount int) *ClusterCorrelation {
	return &ClusterCorrelation{
		column1:              c1,
		column2:              c2,
		minObservationsCount: minObservationsCount,
	}
}

// AddObservation folds one column1 value group into the pair's evidence.
// distinctValues and nbrObservations are the group's distinct column2 value
// count and its non-empty observation count; groupJointNLogN is the sum of
// n*ln(n) over the group's column2 value counts, which is what makes the joint
// entropy reducible across groups.
func (cc *ClusterCorrelation) AddObservation(distinctValues, nbrObservations int, groupJointNLogN float64) {
	cc.distinctValuesCount += distinctValues
	cc.observationsCount += nbrObservations
	cc.stats.addGroup(nbrObservations, distinctValues, groupJointNLogN)
}

// returns commulated counts
// Note: a minimum number of observations for column1 is required, otherwise the function
// returns -1, -1
func (cc *ClusterCorrelation) CumulatedCounts() (int, int) {
	if cc.observationsCount < cc.minObservationsCount {
		return -1, -1
	}
	return cc.distinctValuesCount, cc.observationsCount
}
