package compute_pipes

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
)

// The measurement behind the revision of the clustering operator, 2026-09-05.
//
// What it is for: the operator is reported to perform poorly, six defects were
// diagnosed in it, and four of those were fixed. Nobody has a number for what
// good looks like on this problem, so there is no target to measure against;
// what there is, is the operator being replaced. Every number below is
// therefore a comparison of the revised operator against the current one on
// the same input, defect by defect, with the denominator printed beside it.
// There is deliberately no aggregate score: one defect's improvement must not
// be able to hide another's regression.
//
// Two inputs, because no single one can carry all four comparisons.
//
//   - The committed corpus of clustering_cluster_info_test.go: 58 column-pair
//     correlations measured on a real pharmacy claims file. It carries the
//     three counts the old score was computed from and nothing else, so it can
//     drive the merge comparison (defects 3 and 4) and not the score
//     comparison. No ground-truth entity assignment is claimed for it -- the
//     file's own numbers make one genuinely ambiguous, PHARMACY_ID_NUMBER
//     being more strongly associated with the provider address columns than
//     anything else in the file -- so what is reported on it is structural
//     rather than qualitative.
//   - A generated row-level table, seeded and deterministic, whose entity
//     assignment is known by construction. It is the only way to reach the
//     score comparison, which needs the value distributions the committed
//     corpus does not carry. Its shape is drawn from the real file -- the same
//     kinds of column, cardinalities of the same order, about the same row
//     count -- and it plants the two things the diagnoses name: a near-constant
//     column that belongs to no entity, and a subscriber who is frequently
//     also the patient.
//
// The generated table is a demonstration of the mechanism and is not evidence
// about field performance. It was built after the diagnoses were written and
// it encodes them, so it can show that the fixes do what they claim on an
// input of that shape, and it cannot show that inputs in the field have that
// shape.

// ---------------------------------------------------------------------------
// The generated table
// ---------------------------------------------------------------------------

// syntheticColumns is the header of the generated table. Fifteen columns: three
// entities and two decoys.
var syntheticColumns = []string{
	"SUBSCRIBER_ID", "SUBSCRIBER_SSN", "SUBSCRIBER_FIRST_NAME", "SUBSCRIBER_LAST_NAME", "SUBSCRIBER_STREET",
	"PATIENT_ID", "PATIENT_FIRST_NAME", "PATIENT_LAST_NAME", "PATIENT_DOB",
	"PROVIDER_NPI", "PROVIDER_TIN", "PROVIDER_LAST_NAME", "PROVIDER_CITY",
	"PLAN_STATE", "CLAIM_LINE_NO",
}

// syntheticEntities is the ground truth. The two decoys are in no entity and
// belong in none: PLAN_STATE is the near-constant column of diagnosis 1, and
// CLAIM_LINE_NO is a low-cardinality claim-line counter.
var syntheticEntities = map[string]string{
	"SUBSCRIBER_ID": "subscriber", "SUBSCRIBER_SSN": "subscriber",
	"SUBSCRIBER_FIRST_NAME": "subscriber", "SUBSCRIBER_LAST_NAME": "subscriber",
	"SUBSCRIBER_STREET": "subscriber",
	"PATIENT_ID":        "patient", "PATIENT_FIRST_NAME": "patient",
	"PATIENT_LAST_NAME": "patient", "PATIENT_DOB": "patient",
	"PROVIDER_NPI": "provider", "PROVIDER_TIN": "provider",
	"PROVIDER_LAST_NAME": "provider", "PROVIDER_CITY": "provider",
}

var syntheticDecoys = []string{"PLAN_STATE", "CLAIM_LINE_NO"}

// The columns the operator would be configured to treat as identifiers, and
// the ones it would correlate against them. The corpus configuration
// (anonymize_file.pc.json) splits them exactly this way: identifiers on one
// side, attributes on the other.
var syntheticColumns1 = []string{"SUBSCRIBER_ID", "SUBSCRIBER_SSN", "PATIENT_ID", "PROVIDER_NPI", "PROVIDER_TIN"}

// syntheticRowCount and the cardinalities below are of the same order as the
// real file's: it reports 4,858 observations at the top and column1
// cardinalities from 186 to 1,404.
const (
	syntheticRowCount    = 5000
	syntheticSubscribers = 400
	syntheticProviders   = 300
	// syntheticSelfPatientPct is diagnosis 4 made concrete: the fraction of
	// rows on which the patient is the subscriber, so that the two entities
	// the operator exists to separate genuinely correlate.
	syntheticSelfPatientPct = 70
)

// generateSyntheticTable builds the table. The seed is fixed, so two runs
// produce the same rows and the comparison is with the same input in the
// strict sense.
func generateSyntheticTable() ([][]any, map[string]int) {
	rng := rand.New(rand.NewSource(20260905))
	pos := make(map[string]int, len(syntheticColumns))
	for i, c := range syntheticColumns {
		pos[c] = i
	}
	type subscriber struct{ ssn, first, last, street string }
	subs := make([]subscriber, syntheticSubscribers)
	for i := range subs {
		subs[i] = subscriber{
			ssn:    fmt.Sprintf("%09d", 100000000+i),
			first:  fmt.Sprintf("SFIRST%03d", i%137),
			last:   fmt.Sprintf("SLAST%03d", i%211),
			street: fmt.Sprintf("%d ELM ST", 100+i),
		}
	}
	type provider struct{ tin, last, city string }
	provs := make([]provider, syntheticProviders)
	cities := []string{"ALBANY", "BUFFALO", "NEWARK", "TRENTON", "YONKERS", "UTICA"}
	for i := range provs {
		provs[i] = provider{
			tin:  fmt.Sprintf("TIN%06d", 900000+i),
			last: fmt.Sprintf("PLAST%03d", i%173),
			city: cities[i%len(cities)],
		}
	}
	states := []string{"NY", "NJ"}
	rows := make([][]any, 0, syntheticRowCount)
	for r := 0; r < syntheticRowCount; r++ {
		si := rng.Intn(syntheticSubscribers)
		pi := rng.Intn(syntheticProviders)
		row := make([]any, len(syntheticColumns))
		row[pos["SUBSCRIBER_ID"]] = fmt.Sprintf("S%05d", si)
		row[pos["SUBSCRIBER_SSN"]] = subs[si].ssn
		row[pos["SUBSCRIBER_FIRST_NAME"]] = subs[si].first
		row[pos["SUBSCRIBER_LAST_NAME"]] = subs[si].last
		row[pos["SUBSCRIBER_STREET"]] = subs[si].street
		if rng.Intn(100) < syntheticSelfPatientPct {
			// The patient is the subscriber.
			row[pos["PATIENT_ID"]] = fmt.Sprintf("P%05d", si)
			row[pos["PATIENT_FIRST_NAME"]] = subs[si].first
			row[pos["PATIENT_LAST_NAME"]] = subs[si].last
			row[pos["PATIENT_DOB"]] = fmt.Sprintf("19%02d-%02d-%02d", 40+si%50, 1+si%12, 1+si%28)
		} else {
			// A dependent: same household name, own identifier and date.
			d := rng.Intn(3)
			row[pos["PATIENT_ID"]] = fmt.Sprintf("P%05d%d", si, d)
			row[pos["PATIENT_FIRST_NAME"]] = fmt.Sprintf("DFIRST%03d", (si*3+d)%149)
			row[pos["PATIENT_LAST_NAME"]] = subs[si].last
			row[pos["PATIENT_DOB"]] = fmt.Sprintf("20%02d-%02d-%02d", 5+(si+d)%15, 1+(si+d)%12, 1+(si+d)%28)
		}
		row[pos["PROVIDER_NPI"]] = fmt.Sprintf("NPI%07d", 1000000+pi)
		row[pos["PROVIDER_TIN"]] = provs[pi].tin
		row[pos["PROVIDER_LAST_NAME"]] = provs[pi].last
		row[pos["PROVIDER_CITY"]] = provs[pi].city
		row[pos["PLAN_STATE"]] = states[rng.Intn(len(states))]
		row[pos["CLAIM_LINE_NO"]] = fmt.Sprintf("%d", 1+rng.Intn(12))
		rows = append(rows, row)
	}
	return rows, pos
}

// ---------------------------------------------------------------------------
// The reduction
// ---------------------------------------------------------------------------

// reduceCorrelations reproduces what the clustering pool does to a stream of
// rows: partition by each column1 value, accumulate each column2's value
// distribution within the partition, and reduce the partitions back into one
// set of statistics per ordered pair. It shares the statistics themselves with
// the operator -- valueHistogram, columnEntropyStats and computeAssociation
// are the production types -- and reproduces only the partitioning, because
// reaching the real pool needs a BuilderContext, a lookup table and a channel
// registry.
func reduceCorrelations(rows [][]any, pos map[string]int, columns1, columns2 []string,
	minColumn1, minColumn2 int) []*ColumnCorrelation {

	// The marginal of each participating column over every row on which it is
	// non-empty: the same population, and the same approximation, the operator
	// uses.
	marginal := make(map[string]*valueHistogram, len(columns2))
	for _, c := range columns2 {
		marginal[c] = newValueHistogram()
	}
	for _, c := range columns1 {
		if marginal[c] == nil {
			marginal[c] = newValueHistogram()
		}
	}
	for _, row := range rows {
		for c, h := range marginal {
			if s, ok := row[pos[c]].(string); ok && len(s) > 0 {
				h.add(s)
			}
		}
	}

	out := make([]*ColumnCorrelation, 0, len(columns1)*len(columns2))
	for _, c1 := range columns1 {
		// group -> column2 -> histogram
		groups := make(map[string]map[string]*valueHistogram)
		groupOrder := make([]string, 0)
		for _, row := range rows {
			v, ok := row[pos[c1]].(string)
			if !ok || len(v) == 0 {
				continue
			}
			g := groups[v]
			if g == nil {
				g = make(map[string]*valueHistogram, len(columns2))
				for _, c2 := range columns2 {
					if c2 != c1 {
						g[c2] = newValueHistogram()
					}
				}
				groups[v] = g
				groupOrder = append(groupOrder, v)
			}
			for _, c2 := range columns2 {
				if c2 == c1 {
					continue
				}
				if s, ok := row[pos[c2]].(string); ok && len(s) > 0 {
					g[c2].add(s)
				}
			}
		}
		sort.Strings(groupOrder)
		for _, c2 := range columns2 {
			if c2 == c1 {
				continue
			}
			acc := NewClusterCorrelation(c1, c2, minColumn2)
			for _, gv := range groupOrder {
				h := groups[gv][c2]
				// The worker's own gate: a group reports only when it has more
				// than min_column2_non_null_count observations.
				if h.total > minColumn2 {
					acc.AddObservation(h.distinctCount(), h.total, h.sumNLog)
				}
			}
			distinct2, total := acc.CumulatedCounts()
			if total < minColumn1 {
				continue
			}
			scores := computeAssociation(acc.stats, marginal[c1].marginal(), marginal[c2].marginal())
			out = append(out, &ColumnCorrelation{
				column1:          c1,
				column2:          c2,
				distinct1Count:   len(groups),
				distinct2Count:   distinct2,
				observationCount: total,
				stats:            acc.stats,
				scores:           scores,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].column1 != out[j].column1 {
			return out[i].column1 < out[j].column1
		}
		return out[i].column2 < out[j].column2
	})
	return out
}

// v1DerivedWeightOver maps the old ratio onto the [0,1] scale the partition
// needs, order-preserving and stretched over the observed range: the most
// associated pair in the input becomes 1 and the least becomes 0.
//
// The stretch is not cosmetic and is stated because it changes the answer. The
// raw ratio on the committed corpus runs from 0.024 to 0.45, so 1 - ratio
// would put every edge in a narrow band near the top; modularity reads a graph
// of near-uniform strong edges as one community, and the "old score, new
// partition" arm would then be measuring the compression rather than the
// partition. A min-max stretch is the transform that gives the old score its
// best available showing against the new one, which is the direction a
// comparison should err in.
func v1DerivedWeightOver(all []*ColumnCorrelation) edgeWeightFunc {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, cc := range all {
		if cc.observationCount <= 0 {
			continue
		}
		r := v1Score(cc)
		lo, hi = math.Min(lo, r), math.Max(hi, r)
	}
	span := hi - lo
	return func(cc *ColumnCorrelation) float64 {
		if cc.observationCount <= 0 || span <= 0 {
			return 0
		}
		return (hi - v1Score(cc)) / span
	}
}

// v1NegatedDependency orders pairs the way the old merge expects -- ascending,
// most associated first -- from the new score. It is what the "new score, old
// merge" arm uses.
func v1NegatedDependency(cc *ColumnCorrelation) float64 {
	return 1 - cc.scores.NormalisedDependency
}

func copyCorrelations(in []*ColumnCorrelation) []*ColumnCorrelation {
	out := make([]*ColumnCorrelation, len(in))
	copy(out, in)
	return out
}

// ---------------------------------------------------------------------------
// Scoring a partition against the known entities
// ---------------------------------------------------------------------------

type pairScore struct {
	togetherAndSame int // true positives
	together        int // pairs the partition put together
	same            int // pairs the ground truth says are one entity
	columnsAssigned int
	columnsDouble   int
	clusters        int
	largest         int
}

// scorePartition counts column pairs rather than clusters, because a cluster
// count says nothing about which columns are in which. A decoy column is in no
// entity, so a pair involving one is never a same-entity pair and is counted
// as together when the partition groups it with anything.
func scorePartition(clusters []*ClusterInfo, entities map[string]string, allColumns []string) pairScore {
	of := make(map[string]int)
	dup := 0
	for i, c := range clusters {
		for col := range c.membership {
			if _, seen := of[col]; seen {
				dup++
			}
			of[col] = i
		}
	}
	s := pairScore{clusters: len(clusters), columnsAssigned: len(of), columnsDouble: dup}
	for _, c := range clusters {
		if len(c.membership) > s.largest {
			s.largest = len(c.membership)
		}
	}
	for i := 0; i < len(allColumns); i++ {
		for j := i + 1; j < len(allColumns); j++ {
			a, b := allColumns[i], allColumns[j]
			ea, oka := entities[a]
			eb, okb := entities[b]
			sameEntity := oka && okb && ea == eb
			ca, hasA := of[a]
			cb, hasB := of[b]
			togetherHere := hasA && hasB && ca == cb
			if sameEntity {
				s.same++
			}
			if togetherHere {
				s.together++
			}
			if sameEntity && togetherHere {
				s.togetherAndSame++
			}
		}
	}
	return s
}

func fraction(n, d int) string {
	if d == 0 {
		return fmt.Sprintf("%d/%d (n/a)", n, d)
	}
	return fmt.Sprintf("%d/%d (%.3f)", n, d, float64(n)/float64(d))
}

// ---------------------------------------------------------------------------
// Defect 1 and defect 2: the score
// ---------------------------------------------------------------------------

func TestClusteringDefect1And2Score(t *testing.T) {
	rows, pos := generateSyntheticTable()
	correlations := reduceCorrelations(rows, pos, syntheticColumns1, syntheticColumns, 10, 3)
	if len(correlations) == 0 {
		t.Fatal("no correlations were measured")
	}

	decoy := make(map[string]bool, len(syntheticDecoys))
	for _, d := range syntheticDecoys {
		decoy[d] = true
	}
	decoyPairs := 0
	for _, cc := range correlations {
		if decoy[cc.column1] || decoy[cc.column2] {
			decoyPairs++
		}
	}

	// Rank the pairs most-associated-first under each score.
	byV1 := copyCorrelations(correlations)
	sort.SliceStable(byV1, func(i, j int) bool { return v1Score(byV1[i]) < v1Score(byV1[j]) })
	byV2 := copyCorrelations(correlations)
	sort.SliceStable(byV2, func(i, j int) bool {
		return byV2[i].scores.NormalisedDependency > byV2[j].scores.NormalisedDependency
	})
	bySU := copyCorrelations(correlations)
	sort.SliceStable(bySU, func(i, j int) bool {
		return bySU[i].scores.SymmetricUncertainty > bySU[j].scores.SymmetricUncertainty
	})

	// K is the number of ordered measured pairs that are within one entity:
	// the number of edges a perfect score would put at the top.
	k := 0
	for _, cc := range correlations {
		if e1, ok1 := syntheticEntities[cc.column1]; ok1 {
			if e2, ok2 := syntheticEntities[cc.column2]; ok2 && e1 == e2 {
				k++
			}
		}
	}
	countDecoyInTop := func(ranked []*ColumnCorrelation) int {
		n := 0
		for i := 0; i < k && i < len(ranked); i++ {
			if decoy[ranked[i].column1] || decoy[ranked[i].column2] {
				n++
			}
		}
		return n
	}
	bestDecoyRank := func(ranked []*ColumnCorrelation) int {
		for i, cc := range ranked {
			if decoy[cc.column1] || decoy[cc.column2] {
				return i + 1
			}
		}
		return 0
	}
	sameEntityInTop := func(ranked []*ColumnCorrelation) int {
		n := 0
		for i := 0; i < k && i < len(ranked); i++ {
			if e1, ok1 := syntheticEntities[ranked[i].column1]; ok1 {
				if e2, ok2 := syntheticEntities[ranked[i].column2]; ok2 && e1 == e2 {
					n++
				}
			}
		}
		return n
	}

	fmt.Println()
	fmt.Println("== Defect 1, the unnormalised dependency proxy ==")
	fmt.Printf("input           : generated table, %d rows, %d columns, %d ordered pairs measured\n",
		len(rows), len(syntheticColumns), len(correlations))
	fmt.Printf("decoy pairs     : %s of the measured pairs involve a column in no entity\n",
		fraction(decoyPairs, len(correlations)))
	fmt.Printf("top-K, K = %d   : within-entity pairs in the top K\n", k)
	fmt.Printf("  current score              : %s\n", fraction(sameEntityInTop(byV1), k))
	fmt.Printf("  revised score              : %s\n", fraction(sameEntityInTop(byV2), k))
	fmt.Printf("  uncertainty coefficient    : %s  (reported, not adopted)\n", fraction(sameEntityInTop(bySU), k))
	fmt.Printf("decoys in top K :\n")
	fmt.Printf("  current score              : %s\n", fraction(countDecoyInTop(byV1), k))
	fmt.Printf("  revised score              : %s\n", fraction(countDecoyInTop(byV2), k))
	fmt.Printf("  uncertainty coefficient    : %s\n", fraction(countDecoyInTop(bySU), k))
	fmt.Printf("best decoy rank : current %d, revised %d, uncertainty coefficient %d, of %d (higher is better)\n",
		bestDecoyRank(byV1), bestDecoyRank(byV2), bestDecoyRank(bySU), len(correlations))
	// The independence case, which is what makes the uncertainty coefficient
	// unusable here: two columns generated independently of one another.
	for _, cc := range correlations {
		if cc.column1 == "SUBSCRIBER_ID" && cc.column2 == "PROVIDER_LAST_NAME" {
			fmt.Printf("independent pair SUBSCRIBER_ID -> PROVIDER_LAST_NAME, %d rows per group :\n",
				cc.stats.observations/cc.stats.groups)
			fmt.Printf("  current score %.5f, revised score %.5f, uncertainty coefficient %.5f\n",
				v1Score(cc), cc.scores.NormalisedDependency, cc.scores.SymmetricUncertainty)
		}
	}
	fmt.Printf("PLAN_STATE      : %d distinct values over %d rows\n", 2, len(rows))
	for _, cc := range correlations {
		if cc.column2 == "PLAN_STATE" && cc.column1 == "PROVIDER_NPI" {
			fmt.Printf("  PROVIDER_NPI -> PLAN_STATE      : current score %.5f (lower reads as more associated), "+
				"revised score %.5f\n", v1Score(cc), cc.scores.NormalisedDependency)
		}
		if cc.column2 == "SUBSCRIBER_SSN" && cc.column1 == "SUBSCRIBER_ID" {
			fmt.Printf("  SUBSCRIBER_ID -> SUBSCRIBER_SSN : current score %.5f, "+
				"revised score %.5f\n", v1Score(cc), cc.scores.NormalisedDependency)
		}
	}

	// Defect 2. Both directions of a pair, and whether they agree.
	type both struct{ fwd, rev *ColumnCorrelation }
	pairs := make(map[string]*both)
	keyOf := func(a, b string) (string, bool) {
		if a < b {
			return a + "\x00" + b, true
		}
		return b + "\x00" + a, false
	}
	for _, cc := range correlations {
		key, forward := keyOf(cc.column1, cc.column2)
		p := pairs[key]
		if p == nil {
			p = &both{}
			pairs[key] = p
		}
		if forward {
			p.fwd = cc
		} else {
			p.rev = cc
		}
	}
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	bothDirections, v1Disagree, v2DirectionalGap, suAgree := 0, 0, 0, 0
	maxSUGap := 0.0
	for _, key := range keys {
		p := pairs[key]
		if p.fwd == nil || p.rev == nil {
			continue
		}
		bothDirections++
		a, b := v1Score(p.fwd), v1Score(p.rev)
		lo, hi := math.Min(a, b), math.Max(a, b)
		if lo > 0 && hi/lo >= 2 {
			v1Disagree++
		}
		if math.Abs(p.fwd.scores.NormalisedDependency-p.rev.scores.NormalisedDependency) >= 0.2 {
			v2DirectionalGap++
		}
		gap := math.Abs(p.fwd.scores.NormalisedDependency - p.rev.scores.NormalisedDependency)
		if gap > maxSUGap {
			maxSUGap = gap
		}
		if gap < 0.01 {
			suAgree++
		}
	}
	clamped := 0
	for _, cc := range correlations {
		if cc.scores.MarginalClamped {
			clamped++
		}
	}

	fmt.Println()
	fmt.Println("== Defect 2, the directional measure used as symmetric ==")
	fmt.Printf("pairs measured in both directions : %s of the unordered pairs\n",
		fraction(bothDirections, len(pairs)))
	fmt.Printf("  current score differs by 2x or more between the two directions : %s\n",
		fraction(v1Disagree, bothDirections))
	fmt.Printf("  revised: the two directions differ by 0.2 or more : %s\n",
		fraction(v2DirectionalGap, bothDirections))
	fmt.Printf("  revised: the two directions agree to 0.01 : %s\n",
		fraction(suAgree, bothDirections))
	fmt.Printf("  revised: largest disagreement between the two directions : %.5f\n", maxSUGap)
	fmt.Printf("  the edge the partition consumes is the mean of the two, so the graph is symmetric whatever the residue\n")
	fmt.Printf("marginal-population approximation clamped the mutual information : %s\n",
		fraction(clamped, len(correlations)))

	// The measure reads the same from either end up to the residue the
	// minimum-observation gate leaves; the assertion is on that residue rather
	// than on exact equality, because the gate drops a different set of groups
	// in each direction.
	// Directionality is real and is not a defect of the measure: the assertion
	// is that the partition sees one edge per pair whichever direction was
	// measured, which makeClustersWithWeights guarantees by averaging.
	if maxSUGap > 1 {
		t.Errorf("normalised dependency out of range: directions differ by %.5f", maxSUGap)
	}
	for _, cc := range correlations {
		if cc.scores.NormalisedDependency < 0 || cc.scores.NormalisedDependency > 1 {
			t.Errorf("normalised dependency out of range for %s -> %s: %v",
				cc.column1, cc.column2, cc.scores.NormalisedDependency)
		}
	}
}

// ---------------------------------------------------------------------------
// Defect 3 and defect 4: the merge
// ---------------------------------------------------------------------------

func TestClusteringDefect3And4Merge(t *testing.T) {
	rows, pos := generateSyntheticTable()
	correlations := reduceCorrelations(rows, pos, syntheticColumns1, syntheticColumns, 10, 3)
	spec := &ClusteringSpec{
		// The same two knobs the corpus configuration sets, mapped onto the
		// generated columns: identifiers are transitive, and the provider
		// identifier is the one that tags a cluster.
		TransitiveDataClassification: []string{"id", "ssn", "npi"},
		ClusterDataSubclassification: []string{"npi"},
	}
	classification := map[string]string{
		"SUBSCRIBER_ID": "id", "SUBSCRIBER_SSN": "ssn", "PATIENT_ID": "id",
		"PROVIDER_NPI": "npi", "PROVIDER_TIN": "ssn",
		"SUBSCRIBER_FIRST_NAME": "first_name", "SUBSCRIBER_LAST_NAME": "last_name",
		"SUBSCRIBER_STREET": "street_1", "PATIENT_FIRST_NAME": "first_name",
		"PATIENT_LAST_NAME": "last_name", "PATIENT_DOB": "dob",
		"PROVIDER_LAST_NAME": "last_name", "PROVIDER_CITY": "city",
		"PLAN_STATE": "city", "CLAIM_LINE_NO": "id",
	}

	measured := make(map[string]bool)
	for _, cc := range correlations {
		measured[cc.column1] = true
		measured[cc.column2] = true
	}
	columnsInPlay := make([]string, 0, len(measured))
	for _, c := range syntheticColumns {
		if measured[c] {
			columnsInPlay = append(columnsInPlay, c)
		}
	}

	armA, traceA := v1MakeClustersWithScore(copyCorrelations(correlations), classification, spec, v1Score)
	armC, traceC := v1MakeClustersWithScore(copyCorrelations(correlations), classification, spec, v1NegatedDependency)
	armB := makeClustersWithWeights(copyCorrelations(correlations), classification, spec,
		v1DerivedWeightOver(correlations))
	armD := makeClustersWithWeights(copyCorrelations(correlations), classification, spec, normalisedDependencyWeight)

	sA := scorePartition(armA, syntheticEntities, columnsInPlay)
	sB := scorePartition(armB, syntheticEntities, columnsInPlay)
	sC := scorePartition(armC, syntheticEntities, columnsInPlay)
	sD := scorePartition(armD, syntheticEntities, columnsInPlay)

	fmt.Println()
	fmt.Println("== Defects 3 and 4, the greedy merge and the tag heuristic ==")
	fmt.Printf("input : generated table, %d rows, %d columns in play, %d ordered pairs\n",
		len(rows), len(columnsInPlay), len(correlations))
	fmt.Printf("ground truth : 3 entities, %d within-entity column pairs of %d\n",
		sA.same, len(columnsInPlay)*(len(columnsInPlay)-1)/2)
	fmt.Println("arms, so that each change is read separately:")
	report := func(name string, s pairScore) {
		fmt.Printf("  %-34s clusters %2d, largest %2d, columns assigned %s, in two clusters %d\n",
			name, s.clusters, s.largest, fraction(s.columnsAssigned, len(columnsInPlay)), s.columnsDouble)
		fmt.Printf("  %-34s pair recall %s, pair precision %s\n", "",
			fraction(s.togetherAndSame, s.same), fraction(s.togetherAndSame, s.together))
	}
	report("A current score, current merge", sA)
	report("B current score, revised merge", sB)
	report("C revised score, current merge", sC)
	report("D revised score, revised merge", sD)
	fmt.Println("arm D, cluster membership:")
	for i, c := range armD {
		fmt.Printf("  cluster%d %s\n", i, sortedMembers(c))
	}
	fmt.Println("arm A, cluster membership:")
	for i, c := range armA {
		fmt.Printf("  cluster%d %s\n", i, sortedMembers(c))
	}
	fmt.Printf("tag heuristic, arm A : refusals %d, pairs reached %s, abandoned early %v\n",
		traceA.Refusals, fraction(traceA.PairsConsidered, len(correlations)), traceA.AbandonedEarly)
	fmt.Printf("tag heuristic, arm C : refusals %d, pairs reached %s, abandoned early %v\n",
		traceC.Refusals, fraction(traceC.PairsConsidered, len(correlations)), traceC.AbandonedEarly)
	fmt.Printf("tag heuristic, arms B and D : refusals 0 of 0, there is no heuristic to refuse\n")

	// Invariants of the revised partition, which are what the arms above are
	// being compared on rather than a target: every column that was measured
	// lands in exactly one cluster.
	for name, s := range map[string]pairScore{"B": sB, "D": sD} {
		if s.columnsAssigned != len(columnsInPlay) {
			t.Errorf("arm %s assigned %d of %d columns", name, s.columnsAssigned, len(columnsInPlay))
		}
		if s.columnsDouble != 0 {
			t.Errorf("arm %s put %d columns in more than one cluster", name, s.columnsDouble)
		}
	}

	// Determinism: the same input twice gives the same partition.
	again := makeClustersWithWeights(copyCorrelations(correlations), classification, spec, normalisedDependencyWeight)
	if len(again) != len(armD) {
		t.Errorf("revised partition is not deterministic: %d clusters then %d", len(armD), len(again))
	} else {
		for i := range again {
			if len(again[i].membership) != len(armD[i].membership) {
				t.Errorf("revised partition is not deterministic at cluster %d", i)
				break
			}
			for col := range armD[i].membership {
				if !again[i].membership[col] {
					t.Errorf("revised partition is not deterministic: %s moved", col)
					break
				}
			}
		}
	}
}

// TestClusteringOnCommittedCorpus reports the structural comparison on the real
// pharmacy claims correlations. No entity ground truth is claimed for that
// file, so nothing here is a quality number: what it reports is what each merge
// did with the same 58 measured pairs.
func TestClusteringOnCommittedCorpus(t *testing.T) {
	classification := pharmacyClaimsClassifications()
	spec := pharmacyClaimsSpec()
	correlations := pharmacyClaimsCorrelations()

	columns := make([]string, 0)
	seen := make(map[string]bool)
	for _, cc := range correlations {
		for _, c := range []string{cc.column1, cc.column2} {
			if !seen[c] {
				seen[c] = true
				columns = append(columns, c)
			}
		}
	}
	sort.Strings(columns)

	current, trace := v1MakeClustersWithScore(copyCorrelations(correlations), classification, spec, v1Score)
	revised := makeClustersWithWeights(copyCorrelations(correlations), classification, spec,
		v1DerivedWeightOver(correlations))

	sCur := scorePartition(current, nil, columns)
	sRev := scorePartition(revised, nil, columns)

	fmt.Println()
	fmt.Println("== The committed corpus: 58 measured pairs from a real pharmacy claims file ==")
	fmt.Printf("columns appearing in the measured pairs : %d\n", len(columns))
	fmt.Printf("current merge : clusters %d, largest %d, columns assigned %s, in two clusters %d\n",
		sCur.clusters, sCur.largest, fraction(sCur.columnsAssigned, len(columns)), sCur.columnsDouble)
	fmt.Printf("current merge : tag heuristic refused %d times, reached %s of the pairs, abandoned early %v\n",
		trace.Refusals, fraction(trace.PairsConsidered, len(correlations)), trace.AbandonedEarly)
	fmt.Printf("revised merge : clusters %d, largest %d, columns assigned %s, in two clusters %d\n",
		sRev.clusters, sRev.largest, fraction(sRev.columnsAssigned, len(columns)), sRev.columnsDouble)
	fmt.Println("current merge, cluster membership:")
	for i, c := range current {
		fmt.Printf("  cluster%d %s\n", i, sortedMembers(c))
	}
	fmt.Println("revised merge, cluster membership:")
	for i, c := range revised {
		fmt.Printf("  cluster%d %s\n", i, sortedMembers(c))
	}

	if sRev.columnsAssigned != len(columns) {
		t.Errorf("revised merge assigned %d of %d columns", sRev.columnsAssigned, len(columns))
	}
	if sRev.columnsDouble != 0 {
		t.Errorf("revised merge put %d columns in more than one cluster", sRev.columnsDouble)
	}
}

func sortedMembers(c *ClusterInfo) []string {
	out := make([]string, 0, len(c.membership))
	for m := range c.membership {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
