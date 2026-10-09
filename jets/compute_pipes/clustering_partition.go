package compute_pipes

import "sort"

// Graph partitioning for the clustering operator.
//
// Until 2026-09-05 the operator merged column pairs greedily in score order:
// take the strongest remaining edge, fuse the two clusters it joins, repeat.
// That is single-linkage agglomeration, and single-linkage chains -- one
// strong spurious edge fuses two entities irreversibly, with no global
// objective to trade it off against. The motivating case is the one that
// chains worst: subscriber and patient fields genuinely correlate, because the
// subscriber frequently is the patient, so the very columns the operator
// exists to separate are the ones a chaining algorithm joins.
//
// The replacement is a partition of the weighted graph under a global
// objective: modularity, optimised by the Louvain method. A move is accepted
// only when it improves the partition as a whole, so a single strong edge no
// longer decides a merge on its own.
//
// The implementation is deterministic. Louvain's usual node ordering is
// randomised; here nodes are visited in index order and candidate communities
// in sorted order, so the same graph yields the same partition on every run.
// A harness whose output reshuffles between runs cannot be compared with
// itself.

// clusteringModularityResolution is the resolution parameter gamma of the
// modularity objective. 1.0 is standard modularity; a larger value yields more
// and smaller communities. It is a constant rather than a configuration field
// because no configuration in the four workspace repositories instantiates
// this operator, so there is nobody to tune it for yet.
const clusteringModularityResolution = 1.0

// louvainMaxPasses bounds the local-moving phase of one level. Louvain
// converges in a handful of passes on graphs of this size; the bound exists so
// that a floating-point oscillation cannot spin.
const louvainMaxPasses = 32

// weightedEdge is one undirected edge. A self-loop has i == j and arises from
// aggregating a community into a single node.
type weightedEdge struct {
	i, j int
	w    float64
}

// louvainPartition assigns each of the n nodes a community label, maximising
// modularity by the Louvain method. Labels are consecutive from 0.
func louvainPartition(n int, edges []weightedEdge, resolution float64) []int {
	labels := make([]int, n)
	for i := range labels {
		labels[i] = i
	}
	if n == 0 {
		return labels
	}
	curN := n
	curEdges := edges
	for {
		comm, improved := louvainOneLevel(curN, curEdges, resolution)
		if !improved {
			break
		}
		// Renumber the communities compactly, in order of first appearance, so
		// that the next level's node ids are stable.
		remap := make(map[int]int, curN)
		order := make([]int, 0, curN)
		for i := 0; i < curN; i++ {
			if _, ok := remap[comm[i]]; !ok {
				remap[comm[i]] = len(order)
				order = append(order, comm[i])
			}
		}
		newN := len(order)
		for i := range labels {
			labels[i] = remap[comm[labels[i]]]
		}
		if newN == curN {
			// Nothing was actually aggregated; a further level would repeat
			// this one.
			break
		}
		// Aggregate the graph: one node per community, edge weights summed,
		// intra-community weight becoming a self-loop.
		agg := make(map[[2]int]float64, len(curEdges))
		for _, e := range curEdges {
			a, b := remap[comm[e.i]], remap[comm[e.j]]
			if a > b {
				a, b = b, a
			}
			agg[[2]int{a, b}] += e.w
		}
		keys := make([][2]int, 0, len(agg))
		for k := range agg {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(x, y int) bool {
			if keys[x][0] != keys[y][0] {
				return keys[x][0] < keys[y][0]
			}
			return keys[x][1] < keys[y][1]
		})
		next := make([]weightedEdge, 0, len(keys))
		for _, k := range keys {
			next = append(next, weightedEdge{i: k[0], j: k[1], w: agg[k]})
		}
		curEdges = next
		curN = newN
	}
	// Renumber the final labels compactly.
	remap := make(map[int]int, n)
	for i := range labels {
		if _, ok := remap[labels[i]]; !ok {
			remap[labels[i]] = len(remap)
		}
	}
	for i := range labels {
		labels[i] = remap[labels[i]]
	}
	return labels
}

// louvainOneLevel runs the local-moving phase on one graph and reports the
// community of each node, plus whether any node moved.
func louvainOneLevel(n int, edges []weightedEdge, resolution float64) ([]int, bool) {
	comm := make([]int, n)
	for i := range comm {
		comm[i] = i
	}
	adj := make([][]weightedEdge, n)
	degree := make([]float64, n)
	for _, e := range edges {
		if e.w <= 0 {
			continue
		}
		if e.i == e.j {
			// A self-loop contributes twice to the node's weighted degree and
			// is never a candidate move.
			degree[e.i] += 2 * e.w
			continue
		}
		adj[e.i] = append(adj[e.i], weightedEdge{i: e.i, j: e.j, w: e.w})
		adj[e.j] = append(adj[e.j], weightedEdge{i: e.j, j: e.i, w: e.w})
		degree[e.i] += e.w
		degree[e.j] += e.w
	}
	var m2 float64
	for _, d := range degree {
		m2 += d
	}
	if m2 <= 0 {
		return comm, false
	}
	commTot := make([]float64, n)
	copy(commTot, degree)

	movedAny := false
	for pass := 0; pass < louvainMaxPasses; pass++ {
		moved := false
		for i := 0; i < n; i++ {
			if len(adj[i]) == 0 {
				continue
			}
			from := comm[i]
			commTot[from] -= degree[i]
			// Weight from i to each neighbouring community.
			wTo := make(map[int]float64, len(adj[i]))
			wTo[from] = 0
			for _, e := range adj[i] {
				wTo[comm[e.j]] += e.w
			}
			candidates := make([]int, 0, len(wTo))
			for c := range wTo {
				candidates = append(candidates, c)
			}
			sort.Ints(candidates)
			// Modularity gain of moving the isolated node i into community c:
			//   k_i,in - gamma * sum_tot(c) * k_i / 2m
			best, bestGain := from, wTo[from]-resolution*commTot[from]*degree[i]/m2
			for _, c := range candidates {
				gain := wTo[c] - resolution*commTot[c]*degree[i]/m2
				if gain > bestGain+1e-12 {
					best, bestGain = c, gain
				}
			}
			comm[i] = best
			commTot[best] += degree[i]
			if best != from {
				moved = true
				movedAny = true
			}
		}
		if !moved {
			break
		}
	}
	return comm, movedAny
}
