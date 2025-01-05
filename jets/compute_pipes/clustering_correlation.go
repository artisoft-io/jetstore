package compute_pipes

// Component to determine correlation between columns based on column cardinality
// Nbr of observation is the number of distinct values of column1 that was observed.

type ClusterCorrelation struct {
	column1              string
	column2              string
	minObservationsCount int
	distinctValuesCount  int
	observationsCount    int
}

func NewClusterCorrelation(c1, c2 string, minObservationsCount int) *ClusterCorrelation {
	return &ClusterCorrelation{
		column1:              c1,
		column2:              c2,
		minObservationsCount: minObservationsCount,
	}
}

func (cc *ClusterCorrelation) AddObservation(distinctValues, nbrObservations int) {
	cc.distinctValuesCount += distinctValues
	cc.observationsCount += nbrObservations
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
