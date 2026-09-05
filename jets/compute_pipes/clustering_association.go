package compute_pipes

import "math"

// Association measures for the clustering operator.
//
// What the operator is for: a wide claim record carries the same field several
// times over for different parties -- a given name appears for the subscriber,
// the patient and the provider -- and nothing in the header says which is
// which. The operator finds the columns that co-vary and groups them into one
// entity, so that automated mapping can say "these seven columns are the
// subscriber". It is column-level entity resolution for schema mapping, not
// clustering in the statistical sense.
//
// The score it used until 2026-09-05 was a single ratio,
// distinct2Count / observationCount, and it had two defects that this file
// exists to correct:
//
//   - The ratio is an unnormalised proxy for functional dependency. It
//     conflates "column 1 determines column 2" with "column 2 has few values
//     anyway", so a near-constant column -- a state code, a mostly-null field
//     -- scores well against everything. The correction is to divide by the
//     column's own marginal entropy, which is what an uncertainty coefficient
//     does.
//   - Functional dependence is directional and the ratio was used as though it
//     were symmetric: member_id -> member_name is strong and the reverse is
//     weak, and only one direction was ever scored before the edge was merged
//     as undirected. Both directions are computed here and the merge consumes
//     a measure that is symmetric by construction.
//
// Everything below is a counting problem over the single pass the operator
// already makes. No new dependency, no model call.

// columnEntropyStats holds the additive sufficient statistics needed to
// compute the entropies of one (column1, column2) pair over the rows where
// both columns are non-empty.
//
// Each statistic is a sum over the column1 value groups -- the clustering pool
// runs one worker per distinct value of column1 -- so the partial statistics
// of two groups add, which is what lets the pool reduce them the way it
// already reduces the counts.
type columnEntropyStats struct {
	// observations is N, the number of rows where both columns are non-empty.
	observations int
	// groups is the number of column1 value groups that contributed.
	groups int
	// jointNLogN is the sum over groups v and column2 values i of
	// n_vi*ln(n_vi). It yields H(column1, column2) exactly.
	jointNLogN float64
	// cells is the number of occupied cells of the joint table: the sum over
	// groups of the group's distinct column2 value count. It is the quantity
	// the operator's original score used as its numerator, and the quantity
	// normalisedDependency normalises.
	cells int
	// groupSizes counts how many groups had each observation count. It is what
	// lets the independence baseline be computed exactly rather than at the
	// mean group size: the baseline is concave in the group size, so a mean
	// would overstate it and inflate every score.
	groupSizes map[int]int
}

// addGroup folds one column1 value group into the statistics. groupJointNLogN
// is the sum of n*ln(n) over that group's column2 value counts.
func (s *columnEntropyStats) addGroup(groupObservations, groupDistinctValues int, groupJointNLogN float64) {
	s.observations += groupObservations
	s.groups++
	s.cells += groupDistinctValues
	s.jointNLogN += groupJointNLogN
	if s.groupSizes == nil {
		s.groupSizes = make(map[int]int)
	}
	s.groupSizes[groupObservations]++
}

// nLogN returns n*ln(n), with the n <= 0 case defined as 0 -- the limit of
// p*ln(p) as p goes to zero, which is the convention Shannon entropy takes for
// an unobserved value.
func nLogN(n int) float64 {
	if n <= 0 {
		return 0
	}
	return float64(n) * math.Log(float64(n))
}

// entropyFromNLogN returns the Shannon entropy in nats of a distribution given
// its total count and the sum of n_i*ln(n_i) over its value counts:
//
//	H = -sum_i (n_i/N) ln(n_i/N) = ln(N) - (1/N) sum_i n_i ln(n_i)
func entropyFromNLogN(total int, sumNLogN float64) float64 {
	if total <= 0 {
		return 0
	}
	h := math.Log(float64(total)) - sumNLogN/float64(total)
	if h < 0 {
		// Only reachable through floating-point error on a degenerate
		// distribution; the true value is 0.
		return 0
	}
	return h
}

// valueHistogram accumulates the value counts of one column over one
// population, and the running sum of n*ln(n) is maintained incrementally so
// that the entropy is available without a second pass over the map.
type valueHistogram struct {
	counts  map[string]int
	total   int
	sumNLog float64
}

func newValueHistogram() *valueHistogram {
	return &valueHistogram{counts: make(map[string]int)}
}

func (h *valueHistogram) add(value string) {
	n := h.counts[value]
	h.sumNLog += nLogN(n+1) - nLogN(n)
	h.counts[value] = n + 1
	h.total++
}

func (h *valueHistogram) distinctCount() int {
	return len(h.counts)
}

func (h *valueHistogram) marginal() marginalStats {
	if h == nil {
		return marginalStats{}
	}
	counts := make([]int, 0, len(h.counts))
	for _, n := range h.counts {
		counts = append(counts, n)
	}
	return marginalStats{total: h.total, distinct: len(h.counts), sumNLogN: h.sumNLog, counts: counts}
}

// AssociationScores are the association measures of one ordered column pair.
// All four are in nats or in [0,1]; the operator ranks and partitions on
// SymmetricUncertainty.
type AssociationScores struct {
	// MutualInformation is I(column1; column2) in nats.
	MutualInformation float64
	// UncertaintyC2GivenC1 is Theil's U(column2 | column1) = I/H(column2): the
	// fraction of column2's entropy that column1 explains. 1 means column1
	// functionally determines column2.
	UncertaintyC2GivenC1 float64
	// UncertaintyC1GivenC2 is the reverse direction, I/H(column1). The two
	// differ, which is the point of computing both.
	UncertaintyC1GivenC2 float64
	// SymmetricUncertainty is 2I/(H(column1)+H(column2)), in [0,1].
	//
	// It is reported and it is NOT what the partition consumes, which is a
	// departure from the remedy the analysis named and is recorded here rather
	// than left to be rediscovered. The operator partitions its input by the
	// value of column1, so a conditional distribution is estimated from about
	// N/K1 observations -- 3 to 26 rows per group on the real file the
	// operator's own test corpus was measured from. Plug-in mutual information
	// is biased upward at that sample size and the bias grows with
	// cardinality, so two independent high-cardinality columns score as
	// strongly associated: measured at 0.39 and 0.54 on columns generated
	// independently of each other. Subtracting the occupied-cell degrees of
	// freedom removes it for a low-cardinality column and does not for a
	// high-cardinality one, because the joint table is then too sparse for the
	// occupied-cell count to estimate the degrees of freedom at all.
	//
	// It is kept because a human reads the correlation output, and a second
	// statistic computed a different way is worth having in front of them. Do
	// not rank on it.
	SymmetricUncertainty float64
	// NormalisedDependency is the measure the partition consumes, in [0,1].
	// See normalisedDependency.
	NormalisedDependency float64
	// Bias is the finite-sample mutual information subtracted before
	// normalising, in nats. See computeAssociation.
	Bias float64
	// MarginalClamped records that the marginal-population approximation
	// described on computeAssociation put the mutual information outside its
	// admissible range and it was clamped. It is reported rather than hidden
	// because the count of clamped pairs is the instrument for how much the
	// approximation cost on a given input.
	MarginalClamped bool
}

// marginalStats is one column's value distribution over the whole input,
// reduced to what an entropy needs.
type marginalStats struct {
	total    int
	distinct int
	sumNLogN float64
	// counts is the column's value counts. normalisedDependency needs the
	// distribution itself and not a summary of it, because the expected number
	// of distinct values in a small sample depends on the shape.
	counts []int
}

func (m marginalStats) entropy() float64 {
	return entropyFromNLogN(m.total, m.sumNLogN)
}

// computeAssociation turns the pair's joint statistics plus the two columns'
// marginal distributions into the association measures.
//
// Both marginals are taken over every input row on which that column is
// non-empty, whereas the joint statistic is over the rows on which both are
// non-empty and the reporting group met the operator's minimum-observation
// gate. The populations coincide when both columns are fully populated and
// diverge by the fraction of rows on which either is empty.
//
// The approximation is deliberate: an exact per-pair marginal would need one
// merged value histogram per (column1, column2) pair rather than one per
// column, which is the memory the pool already spends on its workers spent a
// second time. What it costs is that I can come out above its ceiling of
// min(H1,H2) or below zero; both are clamped, and the clamp is reported on the
// score so a caller can count how often it fired rather than take the number
// on trust.
//
// Taking both entropies from the marginals rather than one from the marginal
// and one from the column1 groups is what makes the measure read the same from
// either end of the pair. The residual asymmetry is the joint statistic's,
// through that minimum-observation gate, and the partition removes even that
// by averaging the two directions into one edge.
func computeAssociation(s columnEntropyStats, marginal1, marginal2 marginalStats) AssociationScores {
	h12 := entropyFromNLogN(s.observations, s.jointNLogN)
	h1 := marginal1.entropy()
	h2 := marginal2.entropy()

	mi := h1 + h2 - h12
	ceiling := math.Min(h1, h2)
	clamped := false
	if mi < 0 {
		mi, clamped = 0, true
	}
	if mi > ceiling {
		mi, clamped = ceiling, true
	}

	// Subtract the mutual information two independent columns would show at
	// this sample size. Estimated mutual information is biased upward by
	// roughly df/(2N) nats, where df is the degrees of freedom of the joint
	// table; the classic (K1-1)(K2-1) overstates df badly for a pair whose
	// joint table is mostly empty, so the occupied-cell count is used instead.
	//
	// This is where diagnosis 1 is answered at the root rather than by a
	// threshold. A near-constant column against a high-cardinality one fills
	// about K1*K2 cells with about N/(K1*K2) observations each, which is the
	// regime where the bias is the whole of the apparent association; a column
	// that genuinely determines another fills about K1 cells, so its df is
	// zero or negative and nothing is taken off it.
	//
	// K2 is the column's distinct count over the whole input rather than over
	// the pair's population, so df is understated and the correction is
	// conservative: it errs toward keeping an association rather than
	// discarding one.
	df := s.cells - s.groups - marginal2.distinct + 1
	bias := 0.0
	if df > 0 && s.observations > 0 {
		bias = float64(df) / (2 * float64(s.observations))
		mi -= bias
		if mi < 0 {
			mi = 0
		}
	}

	scores := AssociationScores{
		MutualInformation:    mi,
		Bias:                 bias,
		MarginalClamped:      clamped,
		NormalisedDependency: normalisedDependency(s, marginal2),
	}
	if h2 > 0 {
		scores.UncertaintyC2GivenC1 = mi / h2
	}
	if h1 > 0 {
		scores.UncertaintyC1GivenC2 = mi / h1
	}
	if h1+h2 > 0 {
		scores.SymmetricUncertainty = 2 * mi / (h1 + h2)
	}
	return scores
}

// expectedDistinct returns the expected number of distinct values in a sample
// of n drawn with replacement from the distribution the counts describe.
func expectedDistinct(counts []int, total, n int) float64 {
	if total <= 0 || n <= 0 {
		return 0
	}
	e := 0.0
	for _, c := range counts {
		if c <= 0 {
			continue
		}
		q := float64(c) / float64(total)
		e += 1 - math.Pow(1-q, float64(n))
	}
	return e
}

// normalisedDependency is the association measure the partition ranks and
// merges on. It is in [0,1]: 1 when column1 determines column2 outright, 0
// when knowing column1 tells you no more about column2 than its own
// distribution already does.
//
// It normalises the operator's own statistic rather than replacing it. That
// statistic is `cells`, the summed count of distinct column2 values within the
// column1 value groups; the original score divided it by the observation count,
// which is the defect -- the divisor says nothing about how many distinct
// values column2 has to offer, so a near-constant column scores well against
// everything.
//
// The two ends of the scale are what the divisor should be:
//
//   - the floor is the number of groups, reached when every group holds one
//     distinct column2 value, which is exactly functional dependency;
//   - the ceiling is what independence predicts, the expected number of
//     distinct values a sample of that group's size would show if it were
//     drawn from column2's own distribution.
//
// So the measure is (ceiling - observed) / (ceiling - floor). A near-constant
// column now sits at the ceiling by construction: two values in a group of
// twelve is what two values in the whole file predicts.
//
// The ceiling is computed per distinct group size and summed, which is exact
// and costs one pass over column2's value counts per distinct group size
// rather than per group.
func normalisedDependency(s columnEntropyStats, marginal2 marginalStats) float64 {
	if s.groups == 0 || marginal2.total == 0 {
		return 0
	}
	ceiling := 0.0
	for size, n := range s.groupSizes {
		ceiling += float64(n) * expectedDistinct(marginal2.counts, marginal2.total, size)
	}
	floor := float64(s.groups)
	if ceiling <= floor {
		// Column2 cannot show more distinct values than one per group even
		// under independence, so the statistic cannot discriminate.
		return 0
	}
	d := (ceiling - float64(s.cells)) / (ceiling - floor)
	if d < 0 {
		return 0
	}
	if d > 1 {
		return 1
	}
	return d
}
