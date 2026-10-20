package jetrules_go_adaptor

import (
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// The Go engine's counters, read by the rules_bench command
// (jets/cmds/rules_bench) through compute_pipes.GraphSizer and
// compute_pipes.ReteCounterReader. Nothing in a pipeline calls them. Each reads
// fields the engine keeps anyway, so reading costs a walk over the network's
// vertices and nothing while the rules run.

// GraphSizes implements compute_pipes.GraphSizer.
func (ses *JetRdfSessionGo) GraphSizes() compute_pipes.GraphSizes {
	rs := ses.rdfSession
	return compute_pipes.GraphSizes{
		Meta:     rs.MetaGraph.Size(),
		Asserted: rs.AssertedGraph.Size(),
		Inferred: rs.InferredGraph.Size(),
	}
}

// ReteCounters implements compute_pipes.ReteCounterReader. A released rete
// session has no counters, and returns the zero value.
func (ses *JetReteSessionGo) ReteCounters() compute_pipes.ReteCounters {
	var c compute_pipes.ReteCounters
	rs := ses.reteSession
	if rs == nil {
		return c
	}
	c.Vertices = len(rs.VertexVisits)
	for i := range rs.VertexVisits {
		v := &rs.VertexVisits[i]
		c.VertexVisits += int64(v.InferCount)
		c.Retractions += int64(v.RetractCount)
		if int64(v.InferCount) > c.BusiestVertexVisits {
			c.BusiestVertexVisits = int64(v.InferCount)
			c.BusiestVertexRules = v.Label
		}
		if br := rs.GetBetaRelation(i); br != nil && br.AllRows != nil {
			c.BetaRows += int64(br.AllRows.Size())
		}
	}
	return c
}
