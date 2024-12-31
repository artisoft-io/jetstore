package compute_pipes

// Component to determine correlation between columns based on column cardinality
// Nbr of observation is the number of distinct values of column1 that was observed.

type ClusterCorrelation struct {
	column1         string
	column2         string
	minObservations int
	cardinalityWA   *WelfordAlgo
	// cardinalityMap  map[int]int
	// nbrObservations int
}

func NewClusterCorrelation(c1, c2 string, minObservations int) *ClusterCorrelation {
	return &ClusterCorrelation{
		column1:         c1,
		column2:         c2,
		minObservations: minObservations,
		cardinalityWA:   NewWelfordAlgo(),
		// cardinalityMap:  make(map[int]int),
	}
}

func (cc *ClusterCorrelation) AddObservation(nbrDistinctValues int) {
	cc.cardinalityWA.Update(float64(nbrDistinctValues))
}

// returns the mean and variance
// Note: a minimum number of observations for column1 is required, otherwise the function
// returns -1, -1
func (cc *ClusterCorrelation) MeanAndVariance() (float64, float64) {
	if cc.cardinalityWA.Count < cc.minObservations {
		return -1, -1
	}
	return cc.cardinalityWA.Finalize()
}

// func (cc *ClusterCorrelation) AddObservation(nbrDistinctValues int) {
// 	cc.cardinalityMap[nbrDistinctValues] += 1
// 	cc.nbrObservations += 1
// }

// Using a modified Cramer's V calculation (https://en.wikipedia.org/wiki/Cram%C3%A9r%27s_V)
// where ClusterCorrelation correspond to the calculation to determine the correlation
// between column1 and column2.
// cardinalityMap is a map where key is the cardinality of column2 (ie nbr of distinct values)
// for a value of column1 and the value is the number of time this cardinality was observed.
// CramerV = sum i,j ((nij - ni*nj/n)**2 / ni*nj/n)
// where nij == 1 since we only have one observation for each distinct value of column1
// ni == 1 since we only have one observation for each distinct value of column1
// nj = cc.cardinalityMap[j]
// n = cc.nbrObservations
// returns cramerV and average cardinality of (column1, column2)
// Note: a minimum of 5 values for column1 (observations) is required, otherwise the function
// returns -1, -1
// func (cc *ClusterCorrelation) CramerV() (float64, float64) {
// 	if cc.nbrObservations < cc.minObservations {
// 		fmt.Println("CramerV: Got only", cc.nbrObservations,
// 			"expecting at least", cc.minObservations)
// 		return -1, -1
// 	}
// 	cramerV := 0.0
// 	n := float64(cc.nbrObservations)
// 	ac := 0
// 	for c, nj := range cc.cardinalityMap {
// 		r := float64(nj) / n
// 		factor := math.Pow((1.0-r), 2.0) / r
// 		cramerV += factor
// 		// fmt.Printf("cramer contrib (%v, %v) c: %d, nj: %d, n: %v :: %v\n", cc.column1, cc.column2, c, nj,n, factor)
// 		ac += c * nj
// 	}
// 	norm := math.Min(n-1, float64(len(cc.cardinalityMap)-1))
// 	if norm < 1.0 {
// 		norm = 1.0
// 	}
// 	result := math.Sqrt(cramerV / n / norm)
// 	avrCardinality := float64(ac) / n
// 	// fmt.Printf("cramerV (%v, %v): %v, norm: %v, result: %v, avrCardinality: %v\n", cc.column1, cc.column2, cramerV, norm, result, avrCardinality)
// 	return result, avrCardinality
// }

func (cc *ClusterCorrelation) ObservationsCount() int {
	return cc.cardinalityWA.Count
}
